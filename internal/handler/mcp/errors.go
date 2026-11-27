package mcp

import (
	"errors"
	"log/slog"

	"github.com/rendau/kusec/internal/errs"
)

// ── Тексты ошибок для агента ────────────────────────────
//
// Текст ошибки — канал, по которому агент мог бы подобрать значение секрета.
// Если сервер возвращает в ошибке присланное агентом (неизвестный kind, id не
// в формате uuid из текста Postgres, имя переменной шаблона), а потом
// вычищает из текста известные значения секретов, то место вычистки
// показывает, какой из присланных вариантов совпал со значением: агент шлёт
// «000000 000001 … 999999» и находит шестизначный PIN за один вызов.
//
// Поэтому агенту уходит только:
//   - текст ошибок MCP-слоя (agentError) — сервер собирает его сам, без
//     ввода агента;
//   - коды семантических ошибок (errs.Err) и их описания (errs.ErrFull) —
//     в описаниях бывают ключи, slug-и и имена, но никогда значения;
//   - вместо остального (ошибки БД, k8s, сети) — общий текст, подробности —
//     в логе сервера.
//
// Тексты не проверяются на «увиденные» значения и не заменяются при
// совпадении: замена сама — ответ «угадал / не угадал». Агент кладёт в ключ
// item-а тысячи вариантов PIN, ловит ошибку «ключ уже есть» (в ней повторён
// ключ) и по тому, скрыт ли текст, делит варианты пополам. Поэтому значения
// не попадают в тексты по построению: описания ErrFull их не содержат, а
// ошибки sync сведены к значение-независимому виду (kube.kubeErrText, ошибка
// base64 без позиции).

// agentError — ошибка MCP-слоя с текстом, который можно показать агенту.
// В msg не подставляется ничего из запроса агента.
type agentError struct {
	msg string
	err error
}

func (e *agentError) Error() string {
	if e.err == nil {
		return e.msg
	}
	return e.msg + ": " + e.err.Error()
}

func (e *agentError) Unwrap() error { return e.err }

// agentErr — ошибка MCP-слоя без причины.
func agentErr(msg string) error { return &agentError{msg: msg} }

// agentWrap — ошибка MCP-слоя с причиной; причина показывается агенту по
// правилам publicErrText.
func agentWrap(msg string, err error) error { return &agentError{msg: msg, err: err} }

const internalErrText = "internal_error: внутренняя ошибка kusec (подробности — в логе сервера)"

// publicErrText — текст ошибки, который можно показать агенту (см. выше).
func publicErrText(err error) string {
	if err == nil {
		return ""
	}
	if ae, ok := errors.AsType[*agentError](err); ok {
		if ae.err == nil {
			return ae.msg
		}
		return ae.msg + ": " + publicErrText(ae.err)
	}
	if full, ok := errors.AsType[errs.ErrFull](err); ok {
		code := errs.ServiceNA.Error()
		if full.Err != nil {
			code = full.Err.Error()
		}
		if full.Desc == "" {
			return code
		}
		return code + ": " + full.Desc
	}
	if code, ok := errors.AsType[errs.Err](err); ok {
		return code.Error()
	}
	return internalErrText
}

// toolErr готовит ошибку к выдаче агенту: только безопасный текст (см.
// выше). Полный текст внутренних ошибок (с вычисткой значений) — в лог.
func (s *sessionServer) toolErr(err error) error {
	text := publicErrText(err)
	if text == internalErrText {
		slog.Warn("mcp tool error",
			slog.String("principal", s.principal),
			slog.String("error", s.vault.scrub(err.Error())))
	}
	return errors.New(text)
}
