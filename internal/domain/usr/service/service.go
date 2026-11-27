package service

import (
	"context"
	"fmt"

	"github.com/rendau/kusec/internal/domain/usr/model"
	"github.com/rendau/kusec/internal/errs"
)

type Service struct {
	repoDb RepoDbI
}

func New(repoDb RepoDbI) *Service {
	// хэш для холостого сравнения считаем заранее, а не на первом запросе
	dummyPasswordHash()

	return &Service{
		repoDb: repoDb,
	}
}

func (s *Service) List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error) {
	items, tCount, err := s.repoDb.List(ctx, pars)
	if err != nil {
		return nil, 0, fmt.Errorf("repoDb.List: %w", err)
	}
	return items, tCount, nil
}

func (s *Service) Get(ctx context.Context, id int64, errNE bool) (*model.Main, bool, error) {
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

// GetByUsername — пользователь по логину. Логин сравнивается так же, как в
// базе: точно, с учётом регистра (admin и Admin — разные учётки).
func (s *Service) GetByUsername(ctx context.Context, username string) (*model.Main, bool, error) {
	result, found, err := s.repoDb.GetByUsername(ctx, username)
	if err != nil {
		return nil, false, fmt.Errorf("repoDb.GetByUsername: %w", err)
	}
	if !found {
		return nil, false, nil
	}
	return result, true, nil
}

// BurnPasswordCheck — «холостое» сравнение пароля для логина, которого нет:
// выполняется такое же bcrypt-сравнение (с фиктивным хэшем), что и для
// существующего пользователя, — по времени ответа их не отличить.
func (s *Service) BurnPasswordCheck(password string) {
	_, _ = comparePassword(dummyPasswordHash(), password)
}

// PasswordMatches сверяет пароль с сохранённым хэшем (вход, подтверждение
// текущим паролем смены пароля/логина, выпуска ключа и согласия OAuth).
func (s *Service) PasswordMatches(passwordHash, password string) (bool, error) {
	ok, err := comparePassword(passwordHash, password)
	if err != nil {
		return false, fmt.Errorf("comparePassword: %w", err)
	}
	return ok, nil
}

func (s *Service) HasAny(ctx context.Context) (bool, error) {
	result, err := s.repoDb.HasAny(ctx)
	if err != nil {
		return false, fmt.Errorf("repoDb.HasAny: %w", err)
	}
	return result, nil
}

func (s *Service) Create(ctx context.Context, obj *model.Edit) (int64, error) {
	if obj != nil && obj.Password != nil {
		passwordHash, err := hashPassword(*obj.Password)
		if err != nil {
			return 0, fmt.Errorf("hashPassword: %w", err)
		}
		obj.Password = &passwordHash
	}

	newId, err := s.repoDb.Create(ctx, obj)
	if err != nil {
		return 0, fmt.Errorf("repoDb.Create: %w", err)
	}
	return newId, nil
}

// Update меняет пользователя. Смена пароля и выключение пользователя
// отзывают все выданные ему токены (session_version + 1): чужой, знавший
// старый пароль, теряет сессию сразу, а повторное включение не оживляет
// старые refresh-токены.
func (s *Service) Update(ctx context.Context, id int64, obj *model.Edit) error {
	if obj != nil && obj.Password != nil {
		passwordHash, err := hashPassword(*obj.Password)
		if err != nil {
			return fmt.Errorf("hashPassword: %w", err)
		}
		obj.Password = &passwordHash
		obj.SessionVersionInc = true
	}
	if obj != nil && obj.Active != nil && !*obj.Active {
		obj.SessionVersionInc = true
	}

	if err := s.repoDb.Update(ctx, id, obj); err != nil {
		return fmt.Errorf("repoDb.Update: %w", err)
	}
	return nil
}

// RevokeSessions отзывает все выданные пользователю токены (выход):
// session_version + 1, access- и refresh-токены с прежним номером больше не
// принимаются.
func (s *Service) RevokeSessions(ctx context.Context, id int64) error {
	if err := s.repoDb.Update(ctx, id, &model.Edit{SessionVersionInc: true}); err != nil {
		return fmt.Errorf("repoDb.Update: %w", err)
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if err := s.repoDb.Delete(ctx, id); err != nil {
		return fmt.Errorf("repoDb.Delete: %w", err)
	}
	return nil
}
