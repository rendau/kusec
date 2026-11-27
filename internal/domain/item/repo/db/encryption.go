package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/rendau/kusec/internal/infra/secretbox"
)

const (
	// sealBatchSize — сколько строк переводится на шифрование одной пачкой.
	sealBatchSize = 500
	// keyCheckSample — сколько уже зашифрованных строк пробуем расшифровать,
	// чтобы убедиться, что ключ тот же, которым шифровали базу.
	keyCheckSample = 20
)

// ApplyValueEncryption приводит хранимые значения к текущему режиму
// шифрования. Шаг запуска сервиса: после миграций, до приёма запросов.
// Возвращает число строк, зашифрованных этим вызовом.
//
// Шифрование выключено: в базе не должно быть зашифрованных значений — иначе
// ошибка (secretbox.ErrKeyRequired). С такой базой сервис без ключа работать
// не может: читать значения нечем.
//
// Шифрование включено: сначала проверяем, что ключ подходит к уже
// зашифрованным строкам, затем шифруем все непустые открытые значения.
// Повторный вызов ничего не меняет.
func (r *Repo) ApplyValueEncryption(ctx context.Context) (int64, error) {
	if !r.box.Enabled() {
		sealedCount, err := r.countSealedValues(ctx)
		if err != nil {
			return 0, fmt.Errorf("countSealedValues: %w", err)
		}
		if sealedCount > 0 {
			return 0, fmt.Errorf("%d item values are encrypted: %w", sealedCount, secretbox.ErrKeyRequired)
		}
		return 0, nil
	}

	if err := r.checkEncryptionKey(ctx); err != nil {
		return 0, err
	}

	sealedCount, err := r.sealPlainValues(ctx)
	if err != nil {
		return sealedCount, fmt.Errorf("sealPlainValues: %w", err)
	}
	return sealedCount, nil
}

func (r *Repo) countSealedValues(ctx context.Context) (int64, error) {
	var count int64
	err := r.TxM.GetConnection(ctx).QueryRow(ctx,
		`select count(*) from item where starts_with(value, $1)`, secretbox.Prefix,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("query: %w", err)
	}
	return count, nil
}

// checkEncryptionKey расшифровывает несколько уже зашифрованных строк. С чужим
// ключом (опечатка, ключ другого стенда) сервис иначе запустился бы и стал
// писать новые значения вторым ключом — в базе оказались бы два ключа вперемешку.
func (r *Repo) checkEncryptionKey(ctx context.Context) error {
	rows, err := r.loadValues(ctx,
		`select id, value from item where starts_with(value, $1) order by id limit $2`,
		secretbox.Prefix, keyCheckSample,
	)
	if err != nil {
		return fmt.Errorf("loadValues: %w", err)
	}
	for _, row := range rows {
		if _, err = r.box.Open(valueScope, row.value); err != nil {
			return fmt.Errorf("DATA_ENCRYPTION_KEY does not match the stored data (item %s): %w", row.id, err)
		}
	}
	return nil
}

// sealPlainValues шифрует непустые открытые значения пачками по id.
// updated_at не трогаем и аудит не пишем: содержимое не меняется, меняется
// только способ хранения.
func (r *Repo) sealPlainValues(ctx context.Context) (int64, error) {
	var sealedCount int64
	lastId := ""

	for {
		rows, err := r.loadValues(ctx, `
			select id, value
			from item
			where value <> '' and not starts_with(value, $1) and id > $2
			order by id
			limit $3
		`, secretbox.Prefix, lastId, sealBatchSize)
		if err != nil {
			return sealedCount, fmt.Errorf("loadValues: %w", err)
		}
		if len(rows) == 0 {
			return sealedCount, nil
		}

		batch := &pgx.Batch{}
		for _, row := range rows {
			sealed, err := r.box.Seal(valueScope, row.value)
			if err != nil {
				return sealedCount, fmt.Errorf("seal item %s value: %w", row.id, err)
			}
			// «and value = старое значение»: если строку успели изменить (второй
			// экземпляр сервиса), не затираем новое значение шифртекстом старого
			batch.Queue(`update item set value = $1 where id = $2 and value = $3`, sealed, row.id, row.value)
		}

		affected, err := r.execBatch(ctx, batch)
		if err != nil {
			return sealedCount, fmt.Errorf("execBatch: %w", err)
		}
		sealedCount += affected

		lastId = rows[len(rows)-1].id
	}
}

type valueRow struct {
	id    string
	value string
}

func (r *Repo) loadValues(ctx context.Context, query string, args ...any) ([]valueRow, error) {
	rows, err := r.TxM.GetConnection(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	result := make([]valueRow, 0, sealBatchSize)
	for rows.Next() {
		var row valueRow
		if err = rows.Scan(&row.id, &row.value); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		result = append(result, row)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return result, nil
}

// execBatch выполняет пачку запросов одним обращением к базе (вне транзакции
// пачка применяется целиком либо не применяется) и возвращает число
// изменённых строк.
func (r *Repo) execBatch(ctx context.Context, batch *pgx.Batch) (affected int64, finalError error) {
	results := r.TxM.GetConnection(ctx).SendBatch(ctx, batch)
	defer func() {
		if err := results.Close(); err != nil && finalError == nil {
			finalError = fmt.Errorf("fail to close batch results: %w", err)
		}
	}()

	for i := range batch.Len() {
		tag, err := results.Exec()
		if err != nil {
			return affected, fmt.Errorf("fail to exec (item %d): %w", i, err)
		}
		affected += tag.RowsAffected()
	}
	return affected, nil
}
