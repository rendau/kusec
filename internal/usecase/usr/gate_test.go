package usr

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Мест столько, сколько задано: следующий запрос ждёт и, не дождавшись,
// получает отказ; освободившееся место достаётся ждущему.
func TestCheckGate_WaitsThenRefuses(t *testing.T) {
	t.Parallel()

	g := newCheckGate(2, 30*time.Millisecond)

	require.True(t, g.acquire(t.Context()))
	require.True(t, g.acquire(t.Context()))

	started := time.Now()
	require.False(t, g.acquire(t.Context()), "мест нет — отказ после ожидания")
	assert.GreaterOrEqual(t, time.Since(started), 30*time.Millisecond, "перед отказом запрос ждал")

	// место освободилось, пока запрос ждал, — он его получает
	g = newCheckGate(1, 5*time.Second)
	require.True(t, g.acquire(t.Context()))

	got := make(chan bool, 1)
	go func() { got <- g.acquire(t.Context()) }()

	time.Sleep(20 * time.Millisecond)
	g.release()

	select {
	case ok := <-got:
		assert.True(t, ok)
	case <-time.After(2 * time.Second):
		t.Fatal("ждущий запрос не получил освободившееся место")
	}
}

// Сколько бы запросов ни пришло разом, одновременно проверяется не больше,
// чем мест.
func TestCheckGate_BoundsConcurrency(t *testing.T) {
	t.Parallel()

	const slots = 3
	g := newCheckGate(slots, 5*time.Second)

	var (
		wg           sync.WaitGroup
		running, top atomic.Int64
		passed       atomic.Int64
	)
	for range 60 {
		wg.Go(func() {
			if !g.acquire(t.Context()) {
				return
			}
			defer g.release()

			cur := running.Add(1)
			for {
				seen := top.Load()
				if cur <= seen || top.CompareAndSwap(seen, cur) {
					break
				}
			}
			time.Sleep(time.Millisecond) // «проверка пароля»
			running.Add(-1)
			passed.Add(1)
		})
	}
	wg.Wait()

	assert.EqualValues(t, 60, passed.Load(), "при достаточном ожидании проходят все")
	assert.LessOrEqual(t, top.Load(), int64(slots))
}

// На входе мест не меньше двух, даже на одном ядре.
func TestNewLoginCheckGate(t *testing.T) {
	t.Parallel()

	g := newLoginCheckGate()
	assert.GreaterOrEqual(t, cap(g.slots), loginCheckMinSlots)
	assert.Equal(t, loginCheckWait, g.wait)
}
