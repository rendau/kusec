package dashboard

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
	commonModel "github.com/rendau/kusec/internal/domain/common/model"
	configitemModel "github.com/rendau/kusec/internal/domain/configitem/model"
	configmapModel "github.com/rendau/kusec/internal/domain/configmap/model"
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
	usrModel "github.com/rendau/kusec/internal/domain/usr/model"
	"github.com/rendau/kusec/internal/errs"
)

const recentSecretsLimit = 5

type Usecase struct {
	appSvc        AppServiceI
	secretSvc     SecretServiceI
	itemSvc       ItemServiceI
	configMapSvc  ConfigMapServiceI
	configItemSvc ConfigItemServiceI
	usrSvc        UsrServiceI
	sessionSvc    SessionServiceI
}

func New(
	appSvc AppServiceI,
	secretSvc SecretServiceI,
	itemSvc ItemServiceI,
	configMapSvc ConfigMapServiceI,
	configItemSvc ConfigItemServiceI,
	usrSvc UsrServiceI,
	sessionSvc SessionServiceI,
) *Usecase {
	return &Usecase{
		appSvc:        appSvc,
		secretSvc:     secretSvc,
		itemSvc:       itemSvc,
		configMapSvc:  configMapSvc,
		configItemSvc: configItemSvc,
		usrSvc:        usrSvc,
		sessionSvc:    sessionSvc,
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
	var secretScope, configMapScope []string
	if !all {
		secretScope, err = u.accessibleSecretIds(ctx, appScope)
		if err != nil {
			return nil, err
		}
		configMapScope, err = u.accessibleConfigMapIds(ctx, appScope)
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
	if summary.ConfigMap, err = u.configMapCount(ctx, appScope); err != nil {
		return nil, err
	}
	if summary.ConfigItem, err = u.configItemCount(ctx, !all, configMapScope); err != nil {
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
		Active:     new(true),
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
		Active:     new(true),
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
		Active:     new(true),
	})
	if err != nil {
		return Count{}, fmt.Errorf("itemSvc.List(active): %w", err)
	}
	return Count{Total: total, Active: active}, nil
}

func (u *Usecase) accessibleConfigMapIds(ctx context.Context, appScope []string) ([]string, error) {
	if len(appScope) == 0 {
		return nil, nil
	}
	configMaps, _, err := u.configMapSvc.List(ctx, &configmapModel.ListReq{AppIds: appScope})
	if err != nil {
		return nil, fmt.Errorf("configMapSvc.List(ids): %w", err)
	}
	return lo.Map(configMaps, func(c *configmapModel.Main, _ int) string { return c.Id }), nil
}

func (u *Usecase) configMapCount(ctx context.Context, appScope []string) (Count, error) {
	_, total, err := u.configMapSvc.List(ctx, &configmapModel.ListReq{ListParams: countParams(), AppIds: appScope})
	if err != nil {
		return Count{}, fmt.Errorf("configMapSvc.List: %w", err)
	}
	_, active, err := u.configMapSvc.List(ctx, &configmapModel.ListReq{
		ListParams: countParams(),
		AppIds:     appScope,
		Active:     new(true),
	})
	if err != nil {
		return Count{}, fmt.Errorf("configMapSvc.List(active): %w", err)
	}
	return Count{Total: total, Active: active}, nil
}

func (u *Usecase) configItemCount(ctx context.Context, scoped bool, configMapScope []string) (Count, error) {
	if scoped && len(configMapScope) == 0 {
		return Count{}, nil
	}
	_, total, err := u.configItemSvc.List(ctx, &configitemModel.ListReq{ListParams: countParams(), ConfigMapIds: configMapScope})
	if err != nil {
		return Count{}, fmt.Errorf("configItemSvc.List: %w", err)
	}
	_, active, err := u.configItemSvc.List(ctx, &configitemModel.ListReq{
		ListParams:   countParams(),
		ConfigMapIds: configMapScope,
		Active:       new(true),
	})
	if err != nil {
		return Count{}, fmt.Errorf("configItemSvc.List(active): %w", err)
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
		Active:     new(true),
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
			SecretId:   new(secret.Id),
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
