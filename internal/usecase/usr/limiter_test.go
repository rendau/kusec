package usr

import (
	"errors"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rendau/kusec/internal/errs"
)

// newTestLimiter — ограничитель с ручными часами.
func newTestLimiter() (*attemptLimiter, *time.Time) {
	now := time.Unix(1_800_000_000, 0)
	l := newAttemptLimiter()
	l.now = func() time.Time { return now }
	return l, &now
}

// userKey — ключ счётчика существующего пользователя (как uid:/pwd:).
func userKey(id string) attemptKey { return attemptKey{id: id, user: true} }

// unknownKey — ключ счётчика несуществующего логина.
func unknownKey(id string) attemptKey { return attemptKey{id: id} }

// failOnce — одна неудачная попытка; false, если попытка не принята.
func failOnce(l *attemptLimiter, key attemptKey) bool {
	a, _, ok := l.begin(key)
	if !ok {
		return false
	}
	a.fail()
	return true
}

// До порога попытки принимаются, пятая неудача подряд блокирует на минуту;
// пока действует блокировка, попытка не принимается вовсе.
func TestAttemptLimiter_LocksAfterFiveFailures(t *testing.T) {
	t.Parallel()
	l, now := newTestLimiter()

	for i := range attemptMaxFailures {
		require.True(t, failOnce(l, userKey("k")), "попытка %d должна быть принята", i+1)
	}

	_, retryAfter, ok := l.begin(userKey("k"))
	require.False(t, ok, "после пятой неудачи — блокировка")
	assert.Equal(t, time.Minute, retryAfter)

	// блокировка считается от момента неудачи и убывает со временем
	*now = now.Add(40 * time.Second)
	_, retryAfter, ok = l.begin(userKey("k"))
	require.False(t, ok)
	assert.Equal(t, 20*time.Second, retryAfter)

	// другой ключ не затронут
	_, _, ok = l.begin(userKey("other"))
	assert.True(t, ok)
}

// Каждая следующая неудача после истечения блокировки удваивает срок:
// 1, 2, 4, 8 минут и дальше не больше 15.
func TestAttemptLimiter_LockDoubles(t *testing.T) {
	t.Parallel()
	l, now := newTestLimiter()

	for range attemptMaxFailures - 1 {
		require.True(t, failOnce(l, userKey("k")))
	}

	for _, want := range []time.Duration{
		time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute,
		15 * time.Minute, 15 * time.Minute,
	} {
		require.True(t, failOnce(l, userKey("k")), "после истечения блокировки попытка принимается")

		_, retryAfter, ok := l.begin(userKey("k"))
		require.False(t, ok)
		assert.Equal(t, want, retryAfter)

		// за секунду до конца блокировка ещё действует
		*now = now.Add(want - time.Second)
		_, _, ok = l.begin(userKey("k"))
		require.False(t, ok)

		*now = now.Add(time.Second)
	}
}

func TestAttemptLockDuration(t *testing.T) {
	t.Parallel()

	assert.Equal(t, time.Minute, attemptLockDuration(5))
	assert.Equal(t, 2*time.Minute, attemptLockDuration(6))
	assert.Equal(t, 4*time.Minute, attemptLockDuration(7))
	assert.Equal(t, 8*time.Minute, attemptLockDuration(8))
	assert.Equal(t, 15*time.Minute, attemptLockDuration(9))
	assert.Equal(t, 15*time.Minute, attemptLockDuration(1000))
}

// Успех сбрасывает счётчик: дальше снова пять попыток до блокировки.
func TestAttemptLimiter_SuccessResets(t *testing.T) {
	t.Parallel()
	l, _ := newTestLimiter()

	for range attemptMaxFailures - 1 {
		require.True(t, failOnce(l, userKey("k")))
	}

	a, _, ok := l.begin(userKey("k"))
	require.True(t, ok)
	a.succeed()
	assert.Empty(t, l.users, "успех удаляет запись")

	for i := range attemptMaxFailures {
		require.True(t, failOnce(l, userKey("k")), "попытка %d после сброса", i+1)
	}
	_, _, ok = l.begin(userKey("k"))
	assert.False(t, ok)
}

// Попытка без исхода (например, ошибка БД) счётчик не меняет: не
// добавляет неудачу и не сбрасывает уже набранные.
func TestAttemptLimiter_ReleaseKeepsCounter(t *testing.T) {
	t.Parallel()
	l, _ := newTestLimiter()

	a, _, ok := l.begin(userKey("fresh"))
	require.True(t, ok)
	a.release()
	assert.Empty(t, l.users, "пустая запись не остаётся в карте")

	for range attemptMaxFailures - 1 {
		require.True(t, failOnce(l, userKey("k")))
	}
	for range 10 {
		a, _, ok = l.begin(userKey("k"))
		require.True(t, ok)
		a.release()
	}
	assert.Equal(t, attemptMaxFailures-1, l.users["k"].failures)

	// исход сообщается один раз: release после fail ничего не меняет
	a, _, ok = l.begin(userKey("k"))
	require.True(t, ok)
	a.fail()
	a.release()
	a.succeed()
	assert.Equal(t, attemptMaxFailures, l.users["k"].failures)
	_, _, ok = l.begin(userKey("k"))
	assert.False(t, ok)
}

// Параллельный перебор: пока попытки ещё проверяются, «в долг» проходит не
// больше, чем осталось до блокировки, а после порога — по одной.
func TestAttemptLimiter_ParallelAttemptsBounded(t *testing.T) {
	t.Parallel()
	l, now := newTestLimiter()

	inflight := make([]*attempt, 0, attemptMaxFailures)
	for range attemptMaxFailures {
		a, _, ok := l.begin(userKey("k"))
		require.True(t, ok)
		inflight = append(inflight, a)
	}

	_, retryAfter, ok := l.begin(userKey("k"))
	require.False(t, ok, "шестая одновременная попытка не принимается")
	assert.Equal(t, attemptBusyRetry, retryAfter)

	for _, a := range inflight {
		a.fail()
	}
	_, retryAfter, ok = l.begin(userKey("k"))
	require.False(t, ok)
	assert.Equal(t, time.Minute, retryAfter)

	// после блокировки — строго по одной попытке
	*now = now.Add(time.Minute)
	a, _, ok := l.begin(userKey("k"))
	require.True(t, ok)
	_, _, ok = l.begin(userKey("k"))
	require.False(t, ok, "вторая одновременная попытка после порога")
	a.fail()

	_, retryAfter, ok = l.begin(userKey("k"))
	require.False(t, ok)
	assert.Equal(t, 2*time.Minute, retryAfter)
}

// Настоящие одновременные запросы: сколько бы их ни пришло разом, до
// блокировки принимается ровно пять — остальные получают отказ сразу.
func TestAttemptLimiter_ConcurrentBurst(t *testing.T) {
	t.Parallel()
	l, _ := newTestLimiter()

	var (
		wg       sync.WaitGroup
		accepted atomic.Int64
		start    = make(chan struct{})
	)
	for range 200 {
		wg.Go(func() {
			<-start
			a, _, ok := l.begin(userKey("k"))
			if !ok {
				return
			}
			accepted.Add(1)
			runtime.Gosched() // «проверка пароля» идёт, пока приходят остальные
			a.fail()
		})
	}
	close(start)
	wg.Wait()

	assert.EqualValues(t, attemptMaxFailures, accepted.Load())
	_, retryAfter, ok := l.begin(userKey("k"))
	require.False(t, ok)
	assert.Equal(t, time.Minute, retryAfter)
}

// Запись живёт час без активности: потом счётчик начинается с нуля.
func TestAttemptLimiter_EntryExpires(t *testing.T) {
	t.Parallel()
	l, now := newTestLimiter()

	for range attemptMaxFailures - 1 {
		require.True(t, failOnce(l, userKey("k")))
	}

	*now = now.Add(attemptEntryTTL + time.Second)

	for i := range attemptMaxFailures {
		require.True(t, failOnce(l, userKey("k")), "попытка %d после истечения записи", i+1)
	}
	_, _, ok := l.begin(userKey("k"))
	assert.False(t, ok)
}

// Число записей несуществующих логинов ограничено: поток случайных логинов не
// раздувает память, вытесняются самые старые «холодные» записи, а логин,
// дошедший до блокировки, переживает вытеснение.
func TestAttemptLimiter_UnknownSizeBounded(t *testing.T) {
	t.Parallel()
	l, now := newTestLimiter()
	l.maxUnknown = 100

	for range attemptMaxFailures {
		require.True(t, failOnce(l, unknownKey("locked")))
	}

	for i := range 1000 {
		*now = now.Add(time.Millisecond)
		require.True(t, failOnce(l, unknownKey("random-"+strconv.Itoa(i))))
		require.LessOrEqual(t, len(l.unknown), l.maxUnknown)
	}
	assert.Len(t, l.unknown, l.maxUnknown)

	// вытеснялись самые старые из «холодных» записей
	assert.NotContains(t, l.unknown, "random-0")
	assert.Contains(t, l.unknown, "random-999")

	// заблокированная запись на месте, блокировка действует
	_, _, ok := l.begin(unknownKey("locked"))
	assert.False(t, ok)
}

// Запись существующего пользователя чужими ключами не вытесняется. Атака:
// четыре попытки по жертве, затем поток случайных логинов выталкивает её
// запись, счётчик обнуляется — и так без конца, блокировка не наступает
// никогда. Записи пользователей лежат отдельно от ограниченной карты
// несуществующих логинов: пятая неудача блокирует, сколько бы чужих ключей
// ни прошло между попытками.
func TestAttemptLimiter_UserEntryNotEvictedByUnknownLogins(t *testing.T) {
	t.Parallel()
	l, now := newTestLimiter()
	l.maxUnknown = 100

	victim := loginAttemptKey(7)

	for range attemptMaxFailures - 1 {
		require.True(t, failOnce(l, victim))
	}

	// поток случайных логинов — в десять раз больше, чем вмещает карта
	for i := range 10 * l.maxUnknown {
		*now = now.Add(time.Millisecond)
		require.True(t, failOnce(l, unknownLoginAttemptKey("random-"+strconv.Itoa(i))))
	}
	require.Len(t, l.unknown, l.maxUnknown)

	require.Equal(t, attemptMaxFailures-1, l.users[victim.id].failures, "счётчик жертвы на месте")

	// пятая неудача — блокировка
	require.True(t, failOnce(l, victim))
	_, retryAfter, ok := l.begin(victim)
	require.False(t, ok, "после пятой неудачи — блокировка")
	assert.Equal(t, time.Minute, retryAfter)

	// то же — для счётчика текущего пароля вошедшего пользователя
	for _, key := range []attemptKey{passwordAttemptKey(7)} {
		for range attemptMaxFailures - 1 {
			require.True(t, failOnce(l, key))
		}
		for i := range 2 * l.maxUnknown {
			*now = now.Add(time.Millisecond)
			require.True(t, failOnce(l, unknownLoginAttemptKey(key.id+"-random-"+strconv.Itoa(i))))
		}
		require.True(t, failOnce(l, key))
		_, _, ok = l.begin(key)
		require.False(t, ok, key.id)
	}
}

// Записей пользователей сколько угодно — они не упираются в предел карты
// несуществующих логинов и друг друга не вытесняют.
func TestAttemptLimiter_UserEntriesNotBounded(t *testing.T) {
	t.Parallel()
	l, now := newTestLimiter()
	l.maxUnknown = 10

	for id := range int64(50) {
		*now = now.Add(time.Millisecond)
		require.True(t, failOnce(l, loginAttemptKey(id)))
	}
	assert.Len(t, l.users, 50)
	assert.Empty(t, l.unknown)
}

// Истечение по времени у записей пользователей остаётся: без активности
// дольше часа запись выметается (при появлении новых записей), карта не
// копит записи удалённых учёток.
func TestAttemptLimiter_UserEntriesExpire(t *testing.T) {
	t.Parallel()
	l, now := newTestLimiter()

	for id := range int64(20) {
		require.True(t, failOnce(l, loginAttemptKey(id)))
	}
	require.Len(t, l.users, 20)

	*now = now.Add(attemptEntryTTL + time.Second)
	require.True(t, failOnce(l, loginAttemptKey(100)))

	assert.Len(t, l.users, 1)
	assert.Contains(t, l.users, loginAttemptKey(100).id)
}

// Когда карта несуществующих логинов заполнена, первыми уходят истёкшие записи.
func TestAttemptLimiter_EvictsExpiredFirst(t *testing.T) {
	t.Parallel()
	l, now := newTestLimiter()
	l.maxUnknown = 10

	for i := range 10 {
		require.True(t, failOnce(l, unknownKey("old-"+strconv.Itoa(i))))
	}

	*now = now.Add(attemptEntryTTL + time.Second)
	require.True(t, failOnce(l, unknownKey("new")))

	assert.Len(t, l.unknown, 1)
	assert.Contains(t, l.unknown, "new")
}

// Если «холодных» записей нет (все дошли до блокировки), вытесняется самая
// старая из всех — карта несуществующих логинов всё равно не растёт.
func TestAttemptLimiter_EvictsOldestWhenAllHot(t *testing.T) {
	t.Parallel()
	l, now := newTestLimiter()
	l.maxUnknown = 3

	for i := range 3 {
		*now = now.Add(time.Second)
		for range attemptMaxFailures {
			require.True(t, failOnce(l, unknownKey("hot-"+strconv.Itoa(i))))
		}
	}

	*now = now.Add(time.Second)
	require.True(t, failOnce(l, unknownKey("new")))

	assert.Len(t, l.unknown, 3)
	assert.NotContains(t, l.unknown, "hot-0")
	assert.Contains(t, l.unknown, "new")
}

// Несуществующий логин снаружи ведёт себя как существующий: те же пять
// неудач до блокировки и тот же срок.
func TestAttemptLimiter_UnknownLoginLocksLikeUser(t *testing.T) {
	t.Parallel()
	l, _ := newTestLimiter()

	for _, key := range []attemptKey{loginAttemptKey(7), unknownLoginAttemptKey("no-such-user")} {
		for i := range attemptMaxFailures {
			require.True(t, failOnce(l, key), "%s: попытка %d", key.id, i+1)
		}
		_, retryAfter, ok := l.begin(key)
		require.False(t, ok, key.id)
		assert.Equal(t, time.Minute, retryAfter, key.id)
	}
}

func TestTooManyAttemptsErr(t *testing.T) {
	t.Parallel()

	for retryAfter, want := range map[time.Duration]string{
		time.Minute:             "60",
		59*time.Second + 1:      "60", // округление вверх
		500 * time.Millisecond:  "1",
		0:                       "1",
		15 * time.Minute:        "900",
		90*time.Second + 1*1000: "91",
	} {
		fullErr, ok := errors.AsType[errs.ErrFull](tooManyAttemptsErr(retryAfter))
		require.True(t, ok)
		assert.Equal(t, errs.TooManyAttempts, fullErr.Err)
		assert.Equal(t, want, fullErr.Fields["retry_after"], "retryAfter=%s", retryAfter)
		assert.NotEmpty(t, fullErr.Desc)
	}
}

// Ключи входа. Существующий пользователь считается по id: логин (и его
// регистр) на счётчик не влияет. Несуществующий логин — как его сравнивает
// база: пробелы по краям не важны, регистр важен; длина ключа не зависит от
// длины логина.
func TestLoginAttemptKeys(t *testing.T) {
	t.Parallel()

	assert.Equal(t, attemptKey{id: "uid:7", user: true}, loginAttemptKey(7))
	assert.NotEqual(t, loginAttemptKey(7), loginAttemptKey(8))

	assert.Equal(t, unknownLoginAttemptKey("admin"), unknownLoginAttemptKey("  admin "))
	assert.NotEqual(t, unknownLoginAttemptKey("admin"), unknownLoginAttemptKey("Admin"),
		"admin и Admin — разные логины: база сравнивает с учётом регистра")
	assert.NotEqual(t, unknownLoginAttemptKey("admin"), unknownLoginAttemptKey("admin2"))
	assert.False(t, unknownLoginAttemptKey("admin").user)
	assert.Len(t, unknownLoginAttemptKey(string(make([]byte, 1<<20))).id, len(unknownLoginAttemptKey("a").id))

	// несуществующий логин не попадает в чужой счётчик, даже если назван как ключ
	assert.NotEqual(t, loginAttemptKey(7).id, unknownLoginAttemptKey("uid:7").id)

	// счётчики разных проверок одного пользователя не пересекаются
	assert.NotEqual(t, loginAttemptKey(7), passwordAttemptKey(7))
	assert.True(t, passwordAttemptKey(7).user)
}
