package common

import (
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rendau/kusec/internal/errs"
)

// pgCodeUniqueViolation — SQLSTATE нарушения уникального индекса/ограничения.
const pgCodeUniqueViolation = "23505"

// IsUniqueViolation сообщает, вызвана ли ошибка нарушением уникального
// индекса constraint. Repo подменяет такую ошибку семантической из
// internal/errs, чтобы наружу не уходил сырой текст Postgres.
func IsUniqueViolation(err error, constraint string) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	return ok && pgErr.Code == pgCodeUniqueViolation && pgErr.ConstraintName == constraint
}

// ExistsErr — семантическая ошибка code вместо нарушения уникального индекса.
// subject — что занято ("item key"), scope — в пределах чего оно уникально
// ("in the secret"). value — занятое значение; nil, если запрос его не менял
// (конфликт вызван сменой родителя) — тогда оно в текст не попадает.
func ExistsErr(code errs.Err, subject string, value *string, scope string) error {
	desc := subject
	if value != nil {
		desc += " " + strconv.Quote(*value)
	}
	return errs.ErrFull{Err: code, Desc: desc + " already exists " + scope}
}
