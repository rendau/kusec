package db

import (
	"context"
	"fmt"

	"github.com/rendau/kusec/internal/infra/secretbox"
)

// ApplyValueEncryption приводит хранимые ключи 2FA (usr.totp_secret) к
// текущему режиму шифрования. Шаг запуска сервиса: после миграций, до приёма
// запросов. Возвращает число строк, зашифрованных этим вызовом. Устроен так
// же, как перевод значений секретов (item/repo/db/encryption.go).
//
// Шифрование выключено: в базе не должно быть зашифрованных ключей — иначе
// ошибка (secretbox.ErrKeyRequired): проверять коды 2FA сервису было бы нечем.
//
// Шифрование включено: сначала проверяем, что ключ подходит к уже
// зашифрованным строкам, затем шифруем все непустые открытые ключи. Повторный
// вызов ничего не меняет.
func (r *Repo) ApplyValueEncryption(ctx context.Context) (int64, error) {
	if !r.box.Enabled() {
		sealed, err := r.loadTotpSecrets(ctx,
			`select id, totp_secret from usr where starts_with(totp_secret, $1) order by id`,
			secretbox.Prefix,
		)
		if err != nil {
			return 0, fmt.Errorf("loadTotpSecrets: %w", err)
		}
		if len(sealed) > 0 {
			return 0, fmt.Errorf("%d usr totp secrets are encrypted: %w", len(sealed), secretbox.ErrKeyRequired)
		}
		return 0, nil
	}

	// С чужим ключом (опечатка, ключ другого стенда) сервис иначе запустился
	// бы и стал писать новые ключи 2FA вторым ключом. Пользователей мало,
	// поэтому проверяем все зашифрованные строки, а не выборку.
	sealed, err := r.loadTotpSecrets(ctx,
		`select id, totp_secret from usr where starts_with(totp_secret, $1) order by id`,
		secretbox.Prefix,
	)
	if err != nil {
		return 0, fmt.Errorf("loadTotpSecrets: %w", err)
	}
	for _, row := range sealed {
		if _, err = r.box.Open(totpSecretScope, row.secret); err != nil {
			return 0, fmt.Errorf("DATA_ENCRYPTION_KEY does not match the stored data (usr %d): %w", row.id, err)
		}
	}

	plain, err := r.loadTotpSecrets(ctx, `
		select id, totp_secret
		from usr
		where totp_secret <> '' and not starts_with(totp_secret, $1)
		order by id
	`, secretbox.Prefix)
	if err != nil {
		return 0, fmt.Errorf("loadTotpSecrets: %w", err)
	}

	var sealedCount int64
	for _, row := range plain {
		sealedSecret, err := r.box.Seal(totpSecretScope, row.secret)
		if err != nil {
			return sealedCount, fmt.Errorf("seal usr %d totp secret: %w", row.id, err)
		}
		// «and totp_secret = старое значение»: если ключ успели сменить (второй
		// экземпляр сервиса), не затираем новый шифртекстом старого
		tag, err := r.TxM.GetConnection(ctx).Exec(ctx,
			`update usr set totp_secret = $1 where id = $2 and totp_secret = $3`,
			sealedSecret, row.id, row.secret,
		)
		if err != nil {
			return sealedCount, fmt.Errorf("update usr %d: %w", row.id, err)
		}
		sealedCount += tag.RowsAffected()
	}

	return sealedCount, nil
}

type totpSecretRow struct {
	id     int64
	secret string
}

func (r *Repo) loadTotpSecrets(ctx context.Context, query string, args ...any) ([]totpSecretRow, error) {
	rows, err := r.TxM.GetConnection(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	result := make([]totpSecretRow, 0)
	for rows.Next() {
		var row totpSecretRow
		if err = rows.Scan(&row.id, &row.secret); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		result = append(result, row)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return result, nil
}
