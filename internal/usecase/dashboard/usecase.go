package dashboard

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	appModel "github.com/mechta-market/kusec/internal/domain/app/model"
	commonModel "github.com/mechta-market/kusec/internal/domain/common/model"
	itemModel "github.com/mechta-market/kusec/internal/domain/item/model"
	secretModel "github.com/mechta-market/kusec/internal/domain/secret/model"
	usrModel "github.com/mechta-market/kusec/internal/domain/usr/model"
	"github.com/mechta-market/kusec/internal/errs"
)

const recentSecretsLimit = 5

type Usecase struct {
	appSvc     AppServiceI
	secretSvc  SecretServiceI
	itemSvc    ItemServiceI
	usrSvc     UsrServiceI
	sessionSvc SessionServiceI
}

func New(
	appSvc AppServiceI,
	secretSvc SecretServiceI,
	itemSvc ItemServiceI,
	usrSvc UsrServiceI,
	sessionSvc SessionServiceI,
) *Usecase {
	return &Usecase{
		appSvc:     appSvc,
		secretSvc:  secretSvc,
		itemSvc:    itemSvc,
		usrSvc:     usrSvc,
		sessionSvc: sessionSvc,
	}
}

func countParams() commonModel.ListParams {
	return commonModel.ListParams{OnlyCount: true, WithTotalCount: true}
}

func (u *Usecase) Get(ctx context.Context) (*Summary, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}

	appScope, all := u.sessionSvc.FromContext(ctx).AccessibleAppIds()

	var err error
	var secretScope []string
	if !all {
		secretScope, err = u.accessibleSecretIds(ctx, appScope)
		if err != nil {
			return nil, err
		}
	}

	summary := &Summary{}

	if summary.App, err = u.appCount(ctx, appScope); err != nil {
		return nil, err
	}
	if summary.Secret, err = u.secretCount(ctx, appScope); err != nil {
		return nil, err
	}
	if summary.Item, err = u.itemCount(ctx, !all, secretScope); err != nil {
		return nil, err
	}
	if summary.Usr, err = u.usrCount(ctx); err != nil {
		return nil, err
	}
	if summary.RecentSecrets, err = u.recentSecrets(ctx, appScope); err != nil {
		return nil, err
	}

	return summary, nil
}

func (u *Usecase) accessibleSecretIds(ctx context.Context, appScope []string) ([]string, error) {
	if len(appScope) == 0 {
		return nil, nil
	}
	secrets, _, err := u.secretSvc.List(ctx, &secretModel.ListReq{AppIds: appScope})
	if err != nil {
		return nil, fmt.Errorf("secretSvc.List(ids): %w", err)
	}
	return lo.Map(secrets, func(s *secretModel.Main, _ int) string { return s.Id }), nil
}

func (u *Usecase) appCount(ctx context.Context, appScope []string) (Count, error) {
	_, total, err := u.appSvc.List(ctx, &appModel.ListReq{ListParams: countParams(), Ids: appScope})
	if err != nil {
		return Count{}, fmt.Errorf("appSvc.List: %w", err)
	}
	_, active, err := u.appSvc.List(ctx, &appModel.ListReq{
		ListParams: countParams(),
		Ids:        appScope,
		Active:     lo.ToPtr(true),
	})
	if err != nil {
		return Count{}, fmt.Errorf("appSvc.List(active): %w", err)
	}
	return Count{Total: total, Active: active}, nil
}

func (u *Usecase) secretCount(ctx context.Context, appScope []string) (Count, error) {
	_, total, err := u.secretSvc.List(ctx, &secretModel.ListReq{ListParams: countParams(), AppIds: appScope})
	if err != nil {
		return Count{}, fmt.Errorf("secretSvc.List: %w", err)
	}
	_, active, err := u.secretSvc.List(ctx, &secretModel.ListReq{
		ListParams: countParams(),
		AppIds:     appScope,
		Active:     lo.ToPtr(true),
	})
	if err != nil {
		return Count{}, fmt.Errorf("secretSvc.List(active): %w", err)
	}
	return Count{Total: total, Active: active}, nil
}

func (u *Usecase) itemCount(ctx context.Context, scoped bool, secretScope []string) (Count, error) {
	if scoped && len(secretScope) == 0 {
		return Count{}, nil
	}
	_, total, err := u.itemSvc.List(ctx, &itemModel.ListReq{ListParams: countParams(), SecretIds: secretScope})
	if err != nil {
		return Count{}, fmt.Errorf("itemSvc.List: %w", err)
	}
	_, active, err := u.itemSvc.List(ctx, &itemModel.ListReq{
		ListParams: countParams(),
		SecretIds:  secretScope,
		Active:     lo.ToPtr(true),
	})
	if err != nil {
		return Count{}, fmt.Errorf("itemSvc.List(active): %w", err)
	}
	return Count{Total: total, Active: active}, nil
}

func (u *Usecase) usrCount(ctx context.Context) (Count, error) {
	_, total, err := u.usrSvc.List(ctx, &usrModel.ListReq{ListParams: countParams()})
	if err != nil {
		return Count{}, fmt.Errorf("usrSvc.List: %w", err)
	}
	_, active, err := u.usrSvc.List(ctx, &usrModel.ListReq{
		ListParams: countParams(),
		Active:     lo.ToPtr(true),
	})
	if err != nil {
		return Count{}, fmt.Errorf("usrSvc.List(active): %w", err)
	}
	return Count{Total: total, Active: active}, nil
}

func (u *Usecase) recentSecrets(ctx context.Context, appScope []string) ([]*RecentSecret, error) {
	secrets, _, err := u.secretSvc.List(ctx, &secretModel.ListReq{
		ListParams: commonModel.ListParams{
			Page:     0,
			PageSize: recentSecretsLimit,
			Sort:     []string{"-updated_at"},
		},
		AppIds: appScope,
	})
	if err != nil {
		return nil, fmt.Errorf("secretSvc.List(recent): %w", err)
	}

	// Кэш имён приложений: несколько секретов часто из одного приложения.
	appNames := make(map[string]string, len(secrets))

	result := make([]*RecentSecret, 0, len(secrets))
	for _, secret := range secrets {
		appName, ok := appNames[secret.AppId]
		if !ok {
			app, found, err := u.appSvc.Get(ctx, secret.AppId, false)
			if err != nil {
				return nil, fmt.Errorf("appSvc.Get: %w", err)
			}
			if found {
				appName = app.Name
			}
			appNames[secret.AppId] = appName
		}

		_, itemCount, err := u.itemSvc.List(ctx, &itemModel.ListReq{
			ListParams: countParams(),
			SecretId:   lo.ToPtr(secret.Id),
		})
		if err != nil {
			return nil, fmt.Errorf("itemSvc.List(count): %w", err)
		}

		result = append(result, &RecentSecret{
			Id:          secret.Id,
			AppId:       secret.AppId,
			AppName:     appName,
			SlugName:    secret.SlugName,
			Description: secret.Description,
			Active:      secret.Active,
			UpdatedAt:   secret.UpdatedAt,
			ItemCount:   itemCount,
		})
	}

	return result, nil
}
