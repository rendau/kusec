package mcp

import (
	"context"
	"time"
)

// authFailMinDuration — отказ в проверке ключа или токена отдаётся не раньше
// этого времени от начала проверки, по какой бы причине ни был: строка не
// нашего формата (отсекается до базы), ключа нет в базе, ключ отключён,
// владелец неактивен, неверный код OAuth. По времени ответа причину не
// различить, а перебор с одного соединения идёт не быстрее 4 попыток в секунду.
//
// Блокировки по адресу нет намеренно: адрес клиента берётся из заголовков
// посредников, в обход Cloudflare его можно подделать и заблокировать чужой
// адрес (например, общий адрес коннекторов claude.ai). Подбор и так
// бесполезен: ключи и токены — 32 случайных байта.
const authFailMinDuration = 250 * time.Millisecond

// authGuard выравнивает время отказа. Общий для Bearer-входа MCP и
// OAuth-адресов /token, /revoke.
type authGuard struct {
	minFailDuration time.Duration
}

func newAuthGuard() *authGuard {
	return &authGuard{minFailDuration: authFailMinDuration}
}

// slowDown ждёт, пока с начала проверки не пройдёт minFailDuration.
func (g *authGuard) slowDown(ctx context.Context, started time.Time) {
	wait := g.minFailDuration - time.Since(started)
	if wait <= 0 {
		return
	}

	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}
