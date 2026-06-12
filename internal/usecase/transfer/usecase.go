package transfer

import (
	"context"
	"fmt"
	"sort"

	"github.com/samber/lo"

	appModel "github.com/mechta-market/kusec/internal/domain/app/model"
	itemModel "github.com/mechta-market/kusec/internal/domain/item/model"
	secretModel "github.com/mechta-market/kusec/internal/domain/secret/model"
	"github.com/mechta-market/kusec/internal/errs"
)

type Usecase struct {
	appSvc     AppServiceI
	secretSvc  SecretServiceI
	itemSvc    ItemServiceI
	sessionSvc SessionServiceI
}

func New(
	appSvc AppServiceI,
	secretSvc SecretServiceI,
	itemSvc ItemServiceI,
	sessionSvc SessionServiceI,
) *Usecase {
	return &Usecase{
		appSvc:     appSvc,
		secretSvc:  secretSvc,
		itemSvc:    itemSvc,
		sessionSvc: sessionSvc,
	}
}

// Import массово заливает дерево app → secret → item. Upsert по натуральным
// ключам: app — (namespace, slug_name), secret — slug_name внутри app,
// item — key внутри secret. Существующие записи обновляются только при
// реальных отличиях; записи вне запроса не удаляются. Ошибки отдельных
// записей собираются в результат, не прерывая импорт остальных.
func (u *Usecase) Import(ctx context.Context, req *ImportReq) (*ImportResult, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}
	if !u.sessionSvc.CtxIsAdmin(ctx) {
		return nil, errs.NoPermission
	}
	if req == nil || len(req.Apps) == 0 {
		return nil, errs.InvalidRequest
	}

	result := &ImportResult{}

	existingApps, _, err := u.appSvc.List(ctx, &appModel.ListReq{})
	if err != nil {
		return nil, fmt.Errorf("appSvc.List: %w", err)
	}
	appByKey := make(map[string]*appModel.Main, len(existingApps))
	for _, app := range existingApps {
		appByKey[app.Namespace+"/"+app.SlugName] = app
	}

	for _, impApp := range req.Apps {
		appPath := impApp.Namespace + "/" + impApp.SlugName
		if impApp.Namespace == "" || impApp.SlugName == "" || impApp.Name == "" {
			result.Errors = append(result.Errors,
				fmt.Sprintf("app %q: namespace, slug_name and name are required", appPath))
			continue
		}

		appId, err := u.upsertApp(ctx, appByKey[appPath], impApp, result)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("app %q: %v", appPath, err))
			continue
		}

		if len(impApp.Secrets) == 0 {
			continue
		}
		if err = u.importSecrets(ctx, appId, appPath, impApp.Secrets, result); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (u *Usecase) upsertApp(
	ctx context.Context,
	existing *appModel.Main,
	imp *ImportApp,
	result *ImportResult,
) (string, error) {
	if existing == nil {
		id, err := u.appSvc.Create(ctx, &appModel.Edit{
			Namespace:   &imp.Namespace,
			Name:        &imp.Name,
			SlugName:    &imp.SlugName,
			Description: &imp.Description,
			Active:      lo.ToPtr(imp.Active == nil || *imp.Active),
		})
		if err != nil {
			return "", fmt.Errorf("create: %w", err)
		}
		result.AppsCreated++
		return id, nil
	}

	edit := &appModel.Edit{}
	changed := false
	if existing.Name != imp.Name {
		edit.Name = &imp.Name
		changed = true
	}
	if existing.Description != imp.Description {
		edit.Description = &imp.Description
		changed = true
	}
	if imp.Active != nil && existing.Active != *imp.Active {
		edit.Active = imp.Active
		changed = true
	}
	if !changed {
		result.Unchanged++
		return existing.Id, nil
	}

	if err := u.appSvc.Update(ctx, existing.Id, edit); err != nil {
		return "", fmt.Errorf("update: %w", err)
	}
	result.AppsUpdated++
	return existing.Id, nil
}

func (u *Usecase) importSecrets(
	ctx context.Context,
	appId, appPath string,
	secrets []*ImportSecret,
	result *ImportResult,
) error {
	existing, _, err := u.secretSvc.List(ctx, &secretModel.ListReq{AppId: &appId})
	if err != nil {
		return fmt.Errorf("secretSvc.List: %w", err)
	}
	bySlug := make(map[string]*secretModel.Main, len(existing))
	for _, secret := range existing {
		bySlug[secret.SlugName] = secret
	}

	for _, impSecret := range secrets {
		secretPath := appPath + "/" + impSecret.SlugName
		if impSecret.SlugName == "" {
			result.Errors = append(result.Errors,
				fmt.Sprintf("secret %q: slug_name is required", secretPath))
			continue
		}

		secretId, err := u.upsertSecret(ctx, appId, bySlug[impSecret.SlugName], impSecret, result)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("secret %q: %v", secretPath, err))
			continue
		}

		if len(impSecret.Items) == 0 {
			continue
		}
		if err = u.importItems(ctx, secretId, secretPath, impSecret.Items, result); err != nil {
			return err
		}
	}

	return nil
}

func (u *Usecase) upsertSecret(
	ctx context.Context,
	appId string,
	existing *secretModel.Main,
	imp *ImportSecret,
	result *ImportResult,
) (string, error) {
	if existing == nil {
		id, err := u.secretSvc.Create(ctx, &secretModel.Edit{
			AppId:       &appId,
			SlugName:    &imp.SlugName,
			Description: &imp.Description,
			KubeType:    &imp.KubeType,
			Active:      lo.ToPtr(imp.Active == nil || *imp.Active),
		})
		if err != nil {
			return "", fmt.Errorf("create: %w", err)
		}
		result.SecretsCreated++
		return id, nil
	}

	edit := &secretModel.Edit{}
	changed := false
	if existing.Description != imp.Description {
		edit.Description = &imp.Description
		changed = true
	}
	if existing.KubeType != imp.KubeType {
		edit.KubeType = &imp.KubeType
		changed = true
	}
	if imp.Active != nil && existing.Active != *imp.Active {
		edit.Active = imp.Active
		changed = true
	}
	if !changed {
		result.Unchanged++
		return existing.Id, nil
	}

	if err := u.secretSvc.Update(ctx, existing.Id, edit); err != nil {
		return "", fmt.Errorf("update: %w", err)
	}
	result.SecretsUpdated++
	return existing.Id, nil
}

func (u *Usecase) importItems(
	ctx context.Context,
	secretId, secretPath string,
	items []*ImportItem,
	result *ImportResult,
) error {
	existing, _, err := u.itemSvc.List(ctx, &itemModel.ListReq{SecretId: &secretId})
	if err != nil {
		return fmt.Errorf("itemSvc.List: %w", err)
	}
	byKey := make(map[string]*itemModel.Main, len(existing))
	for _, item := range existing {
		byKey[item.Key] = item
	}

	for _, impItem := range items {
		itemPath := secretPath + "/" + impItem.Key
		if impItem.Key == "" {
			result.Errors = append(result.Errors,
				fmt.Sprintf("item %q: key is required", itemPath))
			continue
		}
		if err := validateItemEnums(impItem); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("item %q: %v", itemPath, err))
			continue
		}

		if err := u.upsertItem(ctx, secretId, byKey[impItem.Key], impItem, result); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("item %q: %v", itemPath, err))
		}
	}

	return nil
}

func validateItemEnums(imp *ImportItem) error {
	switch imp.ValueFormat {
	case "", "text", "yaml", "json":
	default:
		return fmt.Errorf("invalid value_format %q", imp.ValueFormat)
	}
	switch imp.Encoding {
	case "", "plain", "base64":
	default:
		return fmt.Errorf("invalid encoding %q", imp.Encoding)
	}
	return nil
}

func (u *Usecase) upsertItem(
	ctx context.Context,
	secretId string,
	existing *itemModel.Main,
	imp *ImportItem,
	result *ImportResult,
) error {
	valueFormat := imp.ValueFormat
	if valueFormat == "" {
		valueFormat = "text"
	}
	encoding := imp.Encoding
	if encoding == "" {
		encoding = "plain"
	}

	if existing == nil {
		_, err := u.itemSvc.Create(ctx, &itemModel.Edit{
			SecretId:    &secretId,
			Key:         &imp.Key,
			Value:       &imp.Value,
			ValueFormat: &valueFormat,
			Encoding:    &encoding,
			FileName:    &imp.FileName,
			ContentType: &imp.ContentType,
			Description: &imp.Description,
			Active:      lo.ToPtr(imp.Active == nil || *imp.Active),
		})
		if err != nil {
			return fmt.Errorf("create: %w", err)
		}
		result.ItemsCreated++
		return nil
	}

	edit := &itemModel.Edit{}
	changed := false
	if existing.Value != imp.Value {
		edit.Value = &imp.Value
		changed = true
	}
	if existing.ValueFormat != valueFormat {
		edit.ValueFormat = &valueFormat
		changed = true
	}
	if existing.Encoding != encoding {
		edit.Encoding = &encoding
		changed = true
	}
	if existing.FileName != imp.FileName {
		edit.FileName = &imp.FileName
		changed = true
	}
	if existing.ContentType != imp.ContentType {
		edit.ContentType = &imp.ContentType
		changed = true
	}
	if existing.Description != imp.Description {
		edit.Description = &imp.Description
		changed = true
	}
	if imp.Active != nil && existing.Active != *imp.Active {
		edit.Active = imp.Active
		changed = true
	}
	if !changed {
		result.Unchanged++
		return nil
	}

	if err := u.itemSvc.Update(ctx, existing.Id, edit); err != nil {
		return fmt.Errorf("update: %w", err)
	}
	result.ItemsUpdated++
	return nil
}

// Tree возвращает все записи деревом app → secret → item без значений
// item-ов (вместо value — его размер). Требуется аутентификация (любой
// пользователь, права администратора не нужны).
func (u *Usecase) Tree(ctx context.Context) ([]*TreeApp, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}

	apps, _, err := u.appSvc.List(ctx, &appModel.ListReq{})
	if err != nil {
		return nil, fmt.Errorf("appSvc.List: %w", err)
	}
	sort.Slice(apps, func(i, j int) bool {
		if apps[i].Namespace != apps[j].Namespace {
			return apps[i].Namespace < apps[j].Namespace
		}
		return apps[i].SlugName < apps[j].SlugName
	})

	tree := make([]*TreeApp, 0, len(apps))
	for _, app := range apps {
		secrets, _, err := u.secretSvc.List(ctx, &secretModel.ListReq{AppId: lo.ToPtr(app.Id)})
		if err != nil {
			return nil, fmt.Errorf("secretSvc.List: %w", err)
		}
		sort.Slice(secrets, func(i, j int) bool {
			return secrets[i].SlugName < secrets[j].SlugName
		})

		treeApp := &TreeApp{
			Id:          app.Id,
			Namespace:   app.Namespace,
			Name:        app.Name,
			SlugName:    app.SlugName,
			Description: app.Description,
			Active:      app.Active,
			UpdatedAt:   app.UpdatedAt,
			Secrets:     make([]*TreeSecret, 0, len(secrets)),
		}

		for _, secret := range secrets {
			items, _, err := u.itemSvc.List(ctx, &itemModel.ListReq{SecretId: lo.ToPtr(secret.Id)})
			if err != nil {
				return nil, fmt.Errorf("itemSvc.List: %w", err)
			}
			sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })

			treeSecret := &TreeSecret{
				Id:          secret.Id,
				SlugName:    secret.SlugName,
				Description: secret.Description,
				Active:      secret.Active,
				KubeType:    secret.KubeType,
				UpdatedAt:   secret.UpdatedAt,
				Items:       make([]*TreeItem, 0, len(items)),
			}
			for _, item := range items {
				treeSecret.Items = append(treeSecret.Items, &TreeItem{
					Id:          item.Id,
					Key:         item.Key,
					ValueFormat: item.ValueFormat,
					Encoding:    item.Encoding,
					FileName:    item.FileName,
					ContentType: item.ContentType,
					Description: item.Description,
					Active:      item.Active,
					UpdatedAt:   item.UpdatedAt,
					// Значение скрыто; размер хранимой строки в байтах.
					ValueSize: int64(len(item.Value)),
				})
			}
			treeApp.Secrets = append(treeApp.Secrets, treeSecret)
		}

		tree = append(tree, treeApp)
	}

	return tree, nil
}
