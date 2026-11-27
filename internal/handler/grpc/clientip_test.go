package grpc

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

func ctxWithForwardedFor(values ...string) context.Context {
	md := metadata.MD{}
	md.Append("x-forwarded-for", values...)
	return metadata.NewIncomingContext(context.Background(), md)
}

// Адрес клиента для журнала — хвост цепочки x-forwarded-for: последние два
// адреса дописывают наши прокси. Начало цепочки присылает сам клиент, и по
// нему подбирающий пароль записал бы в журнал чужой адрес вместо своего.
func TestClientIp_TakesChainTail(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		values []string
		want   string
	}{
		{
			name:   "один прокси: адрес клиента и адрес прокси",
			values: []string{"203.0.113.7, 10.42.0.5"},
			want:   "203.0.113.7, 10.42.0.5",
		},
		{
			name:   "клиент подставил в начало чужие адреса — в журнал они не попадают",
			values: []string{"1.1.1.1, 8.8.8.8, 203.0.113.7, 10.42.0.5"},
			want:   "203.0.113.7, 10.42.0.5",
		},
		{
			name:   "клиент прислал длинную цепочку, чтобы вытеснить свой адрес за предел длины",
			values: []string{strings.Repeat("9.9.9.9, ", 200) + "203.0.113.7, 10.42.0.5"},
			want:   "203.0.113.7, 10.42.0.5",
		},
		{
			name:   "цепочка пришла несколькими значениями заголовка",
			values: []string{"1.1.1.1, 8.8.8.8", "203.0.113.7", "10.42.0.5"},
			want:   "203.0.113.7, 10.42.0.5",
		},
		{
			name:   "без прокси: единственный адрес — тот, с которого пришли",
			values: []string{"198.51.100.20"},
			want:   "198.51.100.20",
		},
		{
			name:   "пробелы и пустые элементы пропускаются",
			values: []string{" 1.1.1.1 ,, 203.0.113.7 ,  ,10.42.0.5 , "},
			want:   "203.0.113.7, 10.42.0.5",
		},
		{
			name:   "IPv6",
			values: []string{"2001:db8::1, 2001:db8::7, fd00::5"},
			want:   "2001:db8::7, fd00::5",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, clientIp(ctxWithForwardedFor(tc.values...)))
		})
	}
}

// Отдельный адрес в журнале ограничен по длине: без прокси перед сервисом
// предпоследний элемент цепочки присылает клиент.
func TestClientIp_AddressLengthLimited(t *testing.T) {
	t.Parallel()

	got := clientIp(ctxWithForwardedFor(strings.Repeat("x", 10_000) + ", 198.51.100.20"))

	assert.Equal(t, strings.Repeat("x", clientIpAddrMaxLen)+", 198.51.100.20", got)
	assert.LessOrEqual(t, len(got), clientIpChainTail*(clientIpAddrMaxLen+2))
}

// Без x-forwarded-for (прямой gRPC-вызов) — адрес соединения; нет и его —
// пусто.
func TestClientIp_FallsBackToPeer(t *testing.T) {
	t.Parallel()

	addr := &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 4321}
	peerCtx := peer.NewContext(context.Background(), &peer.Peer{Addr: addr})

	assert.Equal(t, "192.0.2.10:4321", clientIp(peerCtx))

	// пустой заголовок — как его отсутствие
	md := metadata.MD{}
	md.Append("x-forwarded-for", " , ")
	assert.Equal(t, "192.0.2.10:4321", clientIp(metadata.NewIncomingContext(peerCtx, md)))

	// заголовок есть — он важнее адреса соединения (тот принадлежит шлюзу)
	md = metadata.MD{}
	md.Append("x-forwarded-for", "203.0.113.7, 10.42.0.5")
	assert.Equal(t, "203.0.113.7, 10.42.0.5", clientIp(metadata.NewIncomingContext(peerCtx, md)))

	assert.Empty(t, clientIp(context.Background()))
}
