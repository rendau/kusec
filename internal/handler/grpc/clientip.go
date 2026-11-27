package grpc

import (
	"context"
	"strings"

	"github.com/samber/lo"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

const (
	// clientIpChainTail — сколько последних адресов цепочки x-forwarded-for
	// попадает в журнал: их дописывают наши прокси (ingress — адрес клиента,
	// grpc-gateway — адрес ingress-а).
	clientIpChainTail = 2
	// clientIpAddrMaxLen — предел длины одного адреса в журнале. Без прокси
	// перед сервисом предпоследний элемент цепочки присылает сам клиент, и
	// он может быть любой длины.
	clientIpAddrMaxLen = 64
)

// clientIp — адрес клиента для журнала: хвост цепочки x-forwarded-for либо,
// при прямом gRPC-вызове, адрес соединения.
//
// Берётся именно хвост. Каждый прокси дописывает адрес, с которого к нему
// пришли, в конец цепочки, а её начало присылает сам клиент: по первым
// адресам подбирающий пароль записал бы в журнал «login failed» чужой адрес
// и вытеснил бы из него свой. Значение годится только для журнала — решений
// по нему не принимаем.
func clientIp(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if chain := forwardedChain(md.Get("x-forwarded-for")); len(chain) > 0 {
			tail := chain[max(len(chain)-clientIpChainTail, 0):]
			return strings.Join(lo.Map(tail, func(addr string, _ int) string {
				return lo.Substring(addr, 0, clientIpAddrMaxLen)
			}), ", ")
		}
	}
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		return p.Addr.String()
	}
	return ""
}

// forwardedChain разбирает значения x-forwarded-for в один список адресов в
// порядке прохождения запроса (пустые элементы пропускаются).
func forwardedChain(values []string) []string {
	chain := make([]string, 0, len(values))
	for _, value := range values {
		for addr := range strings.SplitSeq(value, ",") {
			if addr = strings.TrimSpace(addr); addr != "" {
				chain = append(chain, addr)
			}
		}
	}
	return chain
}
