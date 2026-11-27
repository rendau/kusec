package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/rendau/kusec/internal/infra/secretbox"
)

// valueEncryptionApplierI — доменный сервис, значения которого шифруются в базе.
type valueEncryptionApplierI interface {
	ApplyValueEncryption(ctx context.Context) (int64, error)
}

// applyValueEncryption приводит значения в базе к режиму шифрования — после
// миграций и до запуска серверов. target — что переводим ("item.value").
//
// Ключ задан: ещё не зашифрованные значения шифруются (один раз, повторный
// запуск ничего не меняет). Ключ не задан, а в базе зашифрованные значения —
// ошибка, сервис стартовать не должен: читать значения ему нечем.
func applyValueEncryption(ctx context.Context, box *secretbox.Box, target string, applier valueEncryptionApplierI) error {
	sealedCount, err := applier.ApplyValueEncryption(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", target, err)
	}

	if !box.Enabled() {
		slog.Info("data encryption is disabled (DATA_ENCRYPTION_KEY is not set): values are stored in plain text",
			slog.String("target", target))
		return nil
	}

	slog.Info("data encryption is enabled",
		slog.String("target", target),
		slog.Int64("encrypted_now", sealedCount),
	)
	return nil
}
