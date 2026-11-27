package usr

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rendau/kusec/internal/errs"
)

const (
	// attemptMaxFailures — столько неудач подряд ведут к первой блокировке.
	attemptMaxFailures = 5
	// attemptBaseLock — срок первой блокировки; каждая следующая неудача
	// после её истечения удваивает срок (2, 4, 8… минут).
	attemptBaseLock = time.Minute
	// attemptMaxLock — предел срока блокировки.
	attemptMaxLock = 15 * time.Minute
	// attemptEntryTTL — столько запись живёт без активности; потом счётчик
	// начинается с нуля.
	attemptEntryTTL = time.Hour
	// attemptMaxUnknown — предел числа записей несуществующих логинов:
	// перебор случайных логинов не должен раздувать память процесса.
	attemptMaxUnknown = 10_000
	// attemptUserSweepInterval — как часто из записей существующих
	// пользователей выметаются истёкшие.
	attemptUserSweepInterval = 10 * time.Minute
	// attemptBusyRetry — «повторите через» для отказа из-за слишком многих
	// одновременных попыток (блокировки по времени ещё нет).
	attemptBusyRetry = time.Second
)

// attemptKey — ключ счётчика неудач.
type attemptKey struct {
	// id — вид проверки и тот, кого проверяют: пользователь либо логин.
	id string
	// user — счётчик существующего пользователя (вход по его id, текущий
	// пароль). Такие записи хранятся отдельно от записей
	// несуществующих логинов и чужими ключами не вытесняются.
	user bool
}

// attemptEntry — счётчик неудач одного ключа.
type attemptEntry struct {
	failures    int       // неудач подряд
	inflight    int       // попыток, которые проверяются прямо сейчас
	lockedUntil time.Time // до этого момента попытки не принимаются
	lastSeen    time.Time // последняя активность (для TTL и вытеснения)
}

// attemptLimiter — ограничитель перебора в памяти процесса. Считает неудачи
// подряд по ключу: после attemptMaxFailures — блокировка, срок которой
// удваивается с каждой следующей неудачей; успех сбрасывает счётчик.
//
// Попытка сначала резервируется (begin), потом получает исход (fail /
// succeed / release). Резерв нужен против параллельного перебора: пока
// первые запросы ещё считают bcrypt, остальные не проходят «в долг» —
// одновременно проверяется не больше попыток, чем осталось до блокировки.
//
// Записи лежат в двух картах:
//   - users — счётчики существующих пользователей. Их не больше двух на
//     учётку (вход, текущий пароль), поэтому места им хватает
//     всегда: запись уходит только после успеха либо по сроку
//     (attemptEntryTTL). Иначе четыре попытки по жертве можно было бы «смыть»
//     потоком случайных логинов и подбирать пароль без блокировки;
//   - unknown — счётчики входа по несуществующим логинам. Такие логины шлёт
//     кто угодно и в любом количестве, поэтому карта ограничена maxUnknown,
//     лишние записи вытесняются.
type attemptLimiter struct {
	mu sync.Mutex

	users        map[string]*attemptEntry
	usersSweptAt time.Time

	unknown    map[string]*attemptEntry
	maxUnknown int

	now func() time.Time // часы; подменяются в тестах
}

func newAttemptLimiter() *attemptLimiter {
	return &attemptLimiter{
		users:      map[string]*attemptEntry{},
		unknown:    map[string]*attemptEntry{},
		maxUnknown: attemptMaxUnknown,
		now:        time.Now,
	}
}

// table — карта, в которой живёт запись ключа.
func (l *attemptLimiter) table(key attemptKey) map[string]*attemptEntry {
	if key.user {
		return l.users
	}
	return l.unknown
}

// attemptLockDuration — срок блокировки после failures неудач подряд:
// 5-я — 1 минута, 6-я — 2, 7-я — 4, 8-я — 8, дальше 15 минут.
func attemptLockDuration(failures int) time.Duration {
	d := attemptBaseLock
	for i := attemptMaxFailures; i < failures && d < attemptMaxLock; i++ {
		d *= 2
	}
	return min(d, attemptMaxLock)
}

func (l *attemptLimiter) expired(e *attemptEntry, now time.Time) bool {
	return e.inflight == 0 && now.Sub(e.lastSeen) > attemptEntryTTL
}

// begin резервирует попытку. ok=false — попытка не принимается: действует
// блокировка либо слишком много попыток проверяется одновременно;
// retryAfter — через сколько повторить.
func (l *attemptLimiter) begin(key attemptKey) (a *attempt, retryAfter time.Duration, ok bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	entries := l.table(key)

	e := entries[key.id]
	if e != nil && l.expired(e, now) {
		delete(entries, key.id)
		e = nil
	}

	if e == nil {
		l.makeRoom(key, now)
		e = &attemptEntry{}
		entries[key.id] = e
	} else {
		if now.Before(e.lockedUntil) {
			return nil, e.lockedUntil.Sub(now), false
		}
		// до первой блокировки одновременно проверяется столько попыток,
		// сколько осталось до порога; после — по одной
		allowed := 1
		if e.failures < attemptMaxFailures {
			allowed = attemptMaxFailures - e.failures
		}
		if e.inflight >= allowed {
			return nil, attemptBusyRetry, false
		}
	}

	e.inflight++
	e.lastSeen = now

	return &attempt{limiter: l, key: key}, 0, true
}

type attemptOutcome int8

const (
	// attemptNeutral — попытка не дошла до проверки секрета либо её исход
	// не говорит ни о подборе, ни об успехе (например, ошибка БД).
	attemptNeutral attemptOutcome = iota
	attemptFailed
	attemptSucceeded
)

func (l *attemptLimiter) finish(key attemptKey, outcome attemptOutcome) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	entries := l.table(key)

	e := entries[key.id]
	if e != nil && e.inflight > 0 {
		e.inflight--
	}

	switch outcome {
	case attemptSucceeded:
		delete(entries, key.id)
	case attemptFailed:
		if e == nil {
			// запись вытеснена или сброшена успехом параллельной попытки
			l.makeRoom(key, now)
			e = &attemptEntry{}
			entries[key.id] = e
		}
		e.failures++
		e.lastSeen = now
		if e.failures >= attemptMaxFailures {
			e.lockedUntil = now.Add(attemptLockDuration(e.failures))
		}
	default:
		if e != nil && e.failures == 0 && e.inflight == 0 {
			delete(entries, key.id)
		}
	}
}

// makeRoom готовит место под новую запись ключа.
//
// Запись существующего пользователя места не ждёт и никого не вытесняет:
// здесь только изредка выметаются истёкшие записи (удалённые учётки,
// давние попытки), чтобы карта не копила мусор.
//
// Под запись несуществующего логина место освобождается, когда карта
// заполнена: сначала уходят истёкшие записи, затем самая старая. Записи, уже
// дошедшие до блокировки, вытесняются в последнюю очередь.
func (l *attemptLimiter) makeRoom(key attemptKey, now time.Time) {
	if key.user {
		if now.Sub(l.usersSweptAt) >= attemptUserSweepInterval {
			l.usersSweptAt = now
			l.sweepExpired(l.users, now)
		}
		return
	}

	if len(l.unknown) < l.maxUnknown {
		return
	}

	l.sweepExpired(l.unknown, now)

	for len(l.unknown) >= l.maxUnknown {
		var (
			oldestKey, oldestColdKey string
			oldest, oldestCold       *attemptEntry
		)
		for id, e := range l.unknown {
			if oldest == nil || e.lastSeen.Before(oldest.lastSeen) {
				oldestKey, oldest = id, e
			}
			if e.failures < attemptMaxFailures && (oldestCold == nil || e.lastSeen.Before(oldestCold.lastSeen)) {
				oldestColdKey, oldestCold = id, e
			}
		}
		switch {
		case oldestCold != nil:
			delete(l.unknown, oldestColdKey)
		case oldest != nil:
			delete(l.unknown, oldestKey)
		default:
			return
		}
	}
}

// sweepExpired удаляет из карты записи без активности дольше attemptEntryTTL.
func (l *attemptLimiter) sweepExpired(entries map[string]*attemptEntry, now time.Time) {
	for id, e := range entries {
		if l.expired(e, now) {
			delete(entries, id)
		}
	}
}

// attempt — зарезервированная попытка. Исход сообщается один раз: fail,
// succeed либо release (вызывается через defer и ничего не делает, если
// исход уже сообщён).
type attempt struct {
	limiter *attemptLimiter
	key     attemptKey
	done    bool
}

func (a *attempt) report(outcome attemptOutcome) {
	if a == nil || a.done {
		return
	}
	a.done = true
	a.limiter.finish(a.key, outcome)
}

// fail — секрет не подошёл: неудача засчитана.
func (a *attempt) fail() { a.report(attemptFailed) }

// succeed — проверка пройдена: счётчик сброшен.
func (a *attempt) succeed() { a.report(attemptSucceeded) }

// release снимает резерв без изменения счётчика.
func (a *attempt) release() { a.report(attemptNeutral) }

// tooManyAttemptsErr — отказ too_many_attempts с подсказкой, через сколько
// секунд повторить.
func tooManyAttemptsErr(retryAfter time.Duration) error {
	sec := max(int64(math.Ceil(retryAfter.Seconds())), 1)
	return errs.ErrFull{
		Err:    errs.TooManyAttempts,
		Desc:   fmt.Sprintf("too many failed attempts, try again in %d seconds", sec),
		Fields: map[string]string{"retry_after": strconv.FormatInt(sec, 10)},
	}
}

// Ключи ограничителя. У каждого вида проверки свой счётчик: иначе успех
// одной проверки сбрасывал бы счётчик подбора другой.

// loginAttemptKey — вход существующего пользователя: счётчик ведётся по его
// id, а не по логину. Логин меняется (и самим пользователем), а база
// различает регистр: по ключу «логин в нижнем регистре» пользователь,
// переименовавшийся в Admin, своим успешным входом сбрасывал бы счётчик
// подбора пароля учётки admin.
func loginAttemptKey(usrId int64) attemptKey {
	return attemptKey{id: "uid:" + strconv.FormatInt(usrId, 10), user: true}
}

// unknownLoginAttemptKey — вход по логину, которого нет: счётчик по логину
// ровно в том виде, в каком его сравнивает база (без пробелов по краям, с
// учётом регистра). Хэш даёт ключ фиксированной длины — логин любой длины не
// раздувает карту. Снаружи такие логины ведут себя как существующие: по
// блокировке не видно, есть ли учётка.
func unknownLoginAttemptKey(username string) attemptKey {
	sum := sha256.Sum256([]byte(strings.TrimSpace(username)))
	return attemptKey{id: "login:" + hex.EncodeToString(sum[:16])}
}

// passwordAttemptKey — проверки текущего пароля вошедшего пользователя
// (UpdateProfile, ConfirmPassword).
func passwordAttemptKey(usrId int64) attemptKey {
	return attemptKey{id: "pwd:" + strconv.FormatInt(usrId, 10), user: true}
}
