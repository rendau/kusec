package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/mechta-market/kusec/internal/constant"
	"github.com/mechta-market/kusec/internal/domain/apikey/model"
	"github.com/mechta-market/kusec/internal/errs"
)

const (
	// keyRandomBytes — энтропия ключа (256 бит).
	keyRandomBytes = 32

	// keyPrefixLen — сколько первых символов ключа сохраняем для опознания.
	keyPrefixLen = 12
)

type Service struct {
	repoDb RepoDbI
}

func New(repoDb RepoDbI) *Service {
	return &Service{repoDb: repoDb}
}

// ── Ключи ───────────────────────────────────────────────

// GenerateKey возвращает новый ключ (показывается пользователю один раз),
// его sha256-хэш для хранения и префикс для опознания в списках.
func GenerateKey() (key, hash, prefix string, err error) {
	raw := make([]byte, keyRandomBytes)
	if _, err = rand.Read(raw); err != nil {
		return "", "", "", fmt.Errorf("rand read: %w", err)
	}

	key = constant.ApiKeyPrefix + hex.EncodeToString(raw)

	return key, HashKey(key), key[:keyPrefixLen], nil
}

// HashKey — sha256-хэш ключа: ключи высокоэнтропийные, соль не требуется.
func HashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// ── CRUD ────────────────────────────────────────────────

func (s *Service) List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error) {
	items, tCount, err := s.repoDb.List(ctx, pars)
	if err != nil {
		return nil, 0, fmt.Errorf("repoDb.List: %w", err)
	}
	return items, tCount, nil
}

func (s *Service) Get(ctx context.Context, id string, errNE bool) (*model.Main, bool, error) {
	result, found, err := s.repoDb.Get(ctx, id)
	if err != nil {
		return nil, false, fmt.Errorf("repoDb.Get: %w", err)
	}
	if !found {
		if errNE {
			return nil, false, errs.ObjectNotFound
		}
		return nil, false, nil
	}
	return result, found, nil
}

// GetByKeyHash находит ключ по хэшу (аутентификация машинных клиентов).
func (s *Service) GetByKeyHash(ctx context.Context, keyHash string) (*model.Main, bool, error) {
	items, _, err := s.repoDb.List(ctx, &model.ListReq{KeyHash: &keyHash})
	if err != nil {
		return nil, false, fmt.Errorf("repoDb.List: %w", err)
	}
	if len(items) == 0 {
		return nil, false, nil
	}
	return items[0], true, nil
}

func (s *Service) Create(ctx context.Context, obj *model.Edit) (string, error) {
	obj.UpdatedAt = nil // DB-default устанавливает значение

	newId, err := s.repoDb.Create(ctx, obj)
	if err != nil {
		return "", fmt.Errorf("repoDb.Create: %w", err)
	}
	return newId, nil
}

func (s *Service) Update(ctx context.Context, id string, obj *model.Edit) error {
	obj.UpdatedAt = new(time.Now())

	if err := s.repoDb.Update(ctx, id, obj); err != nil {
		return fmt.Errorf("repoDb.Update: %w", err)
	}
	return nil
}

// TouchLastUsed отмечает использование ключа, не трогая updated_at.
func (s *Service) TouchLastUsed(ctx context.Context, id string) error {
	if err := s.repoDb.Update(ctx, id, &model.Edit{LastUsedAt: new(time.Now())}); err != nil {
		return fmt.Errorf("repoDb.Update: %w", err)
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.repoDb.Delete(ctx, id); err != nil {
		return fmt.Errorf("repoDb.Delete: %w", err)
	}
	return nil
}
