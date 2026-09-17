package common

import (
	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendau/mobone/v2"
)

type Base struct {
	Con *pgxpool.Pool
	QB  squirrel.StatementBuilderType
	// TxM подключает repo к транзакции из контекста (mobone TxFn):
	// каждый ModelStore должен получать его в поле TransactionManager.
	TxM *mobone.TransactionManager
}

func NewBase(con *pgxpool.Pool) *Base {
	return &Base{
		Con: con,
		QB:  squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
		TxM: mobone.NewTransactionManager(con),
	}
}
