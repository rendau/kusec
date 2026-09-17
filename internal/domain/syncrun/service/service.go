package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rendau/kusec/internal/domain/syncrun/model"
	"github.com/rendau/kusec/internal/errs"
)

// Service — журнал запусков sync. Запись создаётся в начале запуска,
// финализируется в конце; редактирование задним числом и удаление через API
// невозможны, чистка — только DeleteOlderThan из фонового ретеншна.
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

// Get возвращает запуск вместе с затронутыми объектами.
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

	result.Objects, err = s.repoDb.ListObjects(ctx, []string{id})
	if err != nil {
		return nil, false, fmt.Errorf("repoDb.ListObjects: %w", err)
	}

	return result, true, nil
}

// Start создаёт запись запуска (status=running) и возвращает её id.
func (s *Service) Start(ctx context.Context, obj *model.Main) (string, error) {
	obj.Status = model.StatusRunning

	newId, err := s.repoDb.Create(ctx, obj)
	if err != nil {
		return "", fmt.Errorf("repoDb.Create: %w", err)
	}
	return newId, nil
}

// Finish финализирует запуск.
func (s *Service) Finish(ctx context.Context, id string, obj *model.Edit) error {
	if err := s.repoDb.Update(ctx, id, obj); err != nil {
		return fmt.Errorf("repoDb.Update: %w", err)
	}
	return nil
}

// AddObjects добавляет затронутые k8s-объекты запуска.
func (s *Service) AddObjects(ctx context.Context, runId string, objects []*model.Object) error {
	if err := s.repoDb.CreateObjects(ctx, runId, objects); err != nil {
		return fmt.Errorf("repoDb.CreateObjects: %w", err)
	}
	return nil
}

// ListObjects возвращает объекты набора запусков.
func (s *Service) ListObjects(ctx context.Context, runIds []string) ([]*model.Object, error) {
	objects, err := s.repoDb.ListObjects(ctx, runIds)
	if err != nil {
		return nil, fmt.Errorf("repoDb.ListObjects: %w", err)
	}
	return objects, nil
}

// DeleteOlderThan удаляет запуски старше границы (фоновый ретеншн).
func (s *Service) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	count, err := s.repoDb.DeleteOlderThan(ctx, before)
	if err != nil {
		return 0, fmt.Errorf("repoDb.DeleteOlderThan: %w", err)
	}
	return count, nil
}
