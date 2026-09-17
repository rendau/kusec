package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rendau/kusec/internal/domain/audit/model"
)

// Service — хранение аудита. Записи только создаются: ни Update, ни Delete
// нет намеренно; чистка — только DeleteOlderThan из фонового ретеншна.
type Service struct {
	repoDb RepoDbI
}

func New(repoDb RepoDbI) *Service {
	return &Service{repoDb: repoDb}
}

func (s *Service) List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error) {
	items, tCount, err := s.repoDb.List(ctx, pars)
	if err != nil {
		return nil, 0, fmt.Errorf("repoDb.List: %w", err)
	}
	return items, tCount, nil
}

func (s *Service) Create(ctx context.Context, obj *model.Main) error {
	if err := s.repoDb.Create(ctx, obj); err != nil {
		return fmt.Errorf("repoDb.Create: %w", err)
	}
	return nil
}

// LastEditors — имя последнего актора по каждому item/config_item приложения.
func (s *Service) LastEditors(ctx context.Context, appId string) (map[string]string, error) {
	result, err := s.repoDb.LastEditors(ctx, appId)
	if err != nil {
		return nil, fmt.Errorf("repoDb.LastEditors: %w", err)
	}
	return result, nil
}

// DeleteOlderThan удаляет записи старше границы (фоновый ретеншн).
func (s *Service) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	count, err := s.repoDb.DeleteOlderThan(ctx, before)
	if err != nil {
		return 0, fmt.Errorf("repoDb.DeleteOlderThan: %w", err)
	}
	return count, nil
}
