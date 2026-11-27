package mcp

import (
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"
)

// ── Ограничитель записи известных агенту значений в секрет ──
//
// Агент не видит значения секретов, но видит их отпечатки (value_hash). Если
// он запишет в item секрета значение, которое знает сам, и сравнит отпечаток с
// чужим — он проверит догадку «значение равно X». Без ограничения скорости это
// перебор: короткий PIN или словарный пароль находится за минуты.
//
// Известное агенту значение получается двумя способами:
//   - literal — значение прислано целиком;
//   - template — текст шаблона пишет агент, а переменные он может взять
//     известные: пустой item (их заводят заранее под ручное заполнение) через
//     copy_item даёт «догадка{{пусто}}» = догадка без единого literal; десять
//     item-ов с цифрами дают любой PIN склейкой {{a}}{{b}}{{c}}{{d}}.
//
// Поэтому считается каждая запись в секрет через literal и через template —
// с любыми переменными, а не только с literal в vars: иначе ограничение
// обходится шаблоном. generate, reuse и copy_item не считаются: их значение
// агенту неизвестно, а копия ничего нового не проверяет. Конфигмапы не
// ограничиваются: их значения не секретны и отпечатками не сравниваются —
// поэтому ни один инструмент не отдаёт агенту отпечаток значения конфигмапа
// или содержимого целого объекта (см. SyncRunObjectOut и auditToOut в
// tools_monitoring.go). Новое поле с таким отпечатком открыло бы обход.
//
// Счёт — на подключение (principal: id api-ключа или OAuth-подключения), а не
// на MCP-сессию: сессий одним ключом можно открыть сколько угодно.
const (
	// knownValueWriteBurst — запас записей: обычная настройка приложения
	// (несколько несекретных значений и DSN в секрете) в него укладывается.
	knownValueWriteBurst = 20

	// knownValueWriteRefill — за сколько запас пополняется на одну запись.
	// В установившемся режиме это 10 проверок в минуту вместо тысяч.
	knownValueWriteRefill = 6 * time.Second
)

// writeLimiter — запас записей на каждого principal с равномерным пополнением
// (token bucket). Хранит по одному моменту времени на principal: когда его
// запас снова станет полным.
type writeLimiter struct {
	mu     sync.Mutex
	burst  int
	refill time.Duration
	now    func() time.Time

	// fullAt — когда запас principal снова полон; прошедший момент или
	// отсутствие записи — запас полон
	fullAt  map[string]time.Time
	sweptAt time.Time
}

func newWriteLimiter(burst int, refill time.Duration) *writeLimiter {
	return &writeLimiter{
		burst:  burst,
		refill: refill,
		now:    time.Now,
		fullAt: map[string]time.Time{},
	}
}

// take списывает одну запись у principal. Если запас пуст — ничего не
// списывает и возвращает, через сколько появится следующая запись.
func (l *writeLimiter) take(principal string) (wait time.Duration, ok bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweep(now)

	fullAt := l.fullAt[principal]
	if fullAt.Before(now) {
		fullAt = now
	}

	// до полного запаса осталось debt; каждая запись добавляет к нему refill,
	// пустой запас — это debt = burst * refill
	debt := fullAt.Sub(now)
	if limit := time.Duration(l.burst-1) * l.refill; debt > limit {
		return debt - limit, false
	}

	l.fullAt[principal] = fullAt.Add(l.refill)

	return 0, true
}

// sweep забывает principal-ов с полным запасом (ключ отозван, подключение
// удалено или просто давно не писал), чтобы карта не росла.
func (l *writeLimiter) sweep(now time.Time) {
	if now.Sub(l.sweptAt) < time.Duration(l.burst)*l.refill {
		return
	}
	l.sweptAt = now

	for principal, fullAt := range l.fullAt {
		if !fullAt.After(now) {
			delete(l.fullAt, principal)
		}
	}
}

// writesKnownValue — значение из этого источника агент знает или может
// собрать из известных частей (см. комментарий к ограничителю).
func writesKnownValue(src ValueSourceIn) bool {
	return src.Kind == "literal" || src.Kind == "template"
}

// limitKnownValueWrite вызывается перед записью значения в item секрета:
// списывает запись из запаса подключения, если значение известно агенту.
func (s *sessionServer) limitKnownValueWrite(src ValueSourceIn) error {
	if !writesKnownValue(src) {
		return nil
	}

	wait, ok := s.h.knownValueWrites.take(s.principal)
	if ok {
		return nil
	}

	retryAfter := int(math.Ceil(wait.Seconds()))

	// src.Kind здесь — одно из двух известных значений, не произвольный ввод
	slog.Warn("mcp: known value write rate limit exceeded",
		slog.String("principal", s.principal),
		slog.String("kind", src.Kind),
		slog.Int("retry_after_s", retryAfter))

	return agentErr(fmt.Sprintf(
		"слишком частая запись в секрет значений, заданных текстом (literal или template): запас — %d записей, пополняется по одной каждые %d с; повтори через %d с. "+
			"literal предназначен для несекретных значений (хосты, порты, url, имена БД), секретные задавай через generate, reuse или copy_item — они не ограничены; "+
			"несекретные настройки удобнее держать в configmap (create_config_item — без ограничений)",
		knownValueWriteBurst, int(knownValueWriteRefill.Seconds()), retryAfter,
	))
}
