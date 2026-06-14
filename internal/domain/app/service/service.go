package service

import (
	"context"
	"fmt"
	"time"

	"github.com/mechta-market/kusec/internal/domain/app/model"
	"github.com/mechta-market/kusec/internal/errs"
)

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

func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.repoDb.Delete(ctx, id); err != nil {
		return fmt.Errorf("repoDb.Delete: %w", err)
	}
	return nil
}
