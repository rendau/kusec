package usr

import (
	"context"
	"runtime"
	"time"
)

const (
	// loginCheckMinSlots — меньше двух одновременных проверок пароля на входе
	// не бывает, даже на одном ядре.
	loginCheckMinSlots = 2
	// loginCheckWait — столько вход ждёт свободного места среди одновременных
	// проверок пароля; не дождался — отказ service_not_available.
	loginCheckWait = 3 * time.Second
)

// checkGate — предел одновременных проверок пароля на входе.
//
// bcrypt занимает ядро на десятки миллисекунд, а вход доступен без
// авторизации и считает bcrypt даже для несуществующего логина (чтобы по
// времени ответа не отличить его от существующего). Без предела поток
// запросов на вход со случайными логинами занял бы весь процессор и
// остановил остальной API; счётчики по логину от этого не спасают — логины
// каждый раз новые.
type checkGate struct {
	slots chan struct{}
	wait  time.Duration
}

func newCheckGate(slots int, wait time.Duration) *checkGate {
	return &checkGate{
		slots: make(chan struct{}, max(slots, 1)),
		wait:  wait,
	}
}

// newLoginCheckGate — по числу ядер, доступных процессу, но не меньше
// loginCheckMinSlots.
func newLoginCheckGate() *checkGate {
	return newCheckGate(max(runtime.GOMAXPROCS(0), loginCheckMinSlots), loginCheckWait)
}

// acquire занимает место под проверку. Свободного нет — ждёт не дольше wait;
// false — места не дождались, проверку выполнять нельзя. После true
// обязателен release.
func (g *checkGate) acquire(ctx context.Context) bool {
	select {
	case g.slots <- struct{}{}:
		return true
	default:
	}

	timer := time.NewTimer(g.wait)
	defer timer.Stop()

	select {
	case g.slots <- struct{}{}:
		return true
	case <-timer.C:
		return false
	case <-ctx.Done():
		return false
	}
}

func (g *checkGate) release() {
	<-g.slots
}
