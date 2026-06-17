package kube

import (
	"context"
	"errors"
	"fmt"

	"github.com/mechta-market/kusec/internal/errs"
	kubeService "github.com/mechta-market/kusec/internal/service/kube"
)

type Usecase struct {
	svc          KubeServiceI
	appSvc       AppServiceI
	secretSvc    SecretServiceI
	configMapSvc ConfigMapServiceI
	sessionSvc   SessionServiceI
}

func New(
	svc KubeServiceI,
	appSvc AppServiceI,
	secretSvc SecretServiceI,
	configMapSvc ConfigMapServiceI,
	sessionSvc SessionServiceI,
) *Usecase {
	return &Usecase{
		svc:          svc,
		appSvc:       appSvc,
		secretSvc:    secretSvc,
		configMapSvc: configMapSvc,
		sessionSvc:   sessionSvc,
	}
}

// ListNamespaces возвращает namespace-ы кластера для выбора в форме
// приложения (создание/изменение app — операции администратора).
// Вне кластера возвращает inCluster=false и пустой список.
func (u *Usecase) ListNamespaces(ctx context.Context) ([]string, bool, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, false, errs.NotAuthorized
	}
	if !u.sessionSvc.CtxIsAdmin(ctx) {
		return nil, false, errs.NoPermission
	}

	namespaces, inCluster, err := u.svc.ListNamespaces(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("svc.ListNamespaces: %w", err)
	}

	return namespaces, inCluster, nil
}

// ListClusterSecrets возвращает секреты кластера для выбора при импорте.
// Импорт создаёт приложения, поэтому операция доступна только администратору.
// Вне кластера возвращает inCluster=false и пустой список.
func (u *Usecase) ListClusterSecrets(ctx context.Context, namespace string) ([]*kubeService.ClusterSecret, bool, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, false, errs.NotAuthorized
	}
	if !u.sessionSvc.CtxIsAdmin(ctx) {
		return nil, false, errs.NoPermission
	}

	secrets, inCluster, err := u.svc.ListClusterSecrets(ctx, namespace)
	if err != nil {
		return nil, false, fmt.Errorf("svc.ListClusterSecrets: %w", err)
	}

	return secrets, inCluster, nil
}

// ImportSecret импортирует один секрет кластера в указанное приложение
// (создание secret/item) — операция администратора.
func (u *Usecase) ImportSecret(ctx context.Context, appId string, ref kubeService.ImportRef, secretSlug string) (*kubeService.ImportResult, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}
	if !u.sessionSvc.CtxIsAdmin(ctx) {
		return nil, errs.NoPermission
	}
	if appId == "" {
		return nil, errs.IdRequired
	}
	if ref.Namespace == "" || ref.Name == "" {
		return nil, errs.InvalidRequest
	}
	if secretSlug == "" {
		return nil, errs.NameRequired
	}

	result, err := u.svc.ImportSecret(ctx, appId, ref, secretSlug)
	if err != nil {
		// Сентинельные коды (not_in_cluster, object_not_found) и ErrFull
		// (невалидный slug) пробрасываем как есть.
		if _, ok := errors.AsType[errs.Err](err); ok {
			return nil, err
		}
		if _, ok := errors.AsType[errs.ErrFull](err); ok {
			return nil, err
		}
		return nil, fmt.Errorf("svc.ImportSecret: %w", err)
	}

	return result, nil
}

// GetClusterSecret отдаёт живой k8s-secret из кластера для сверки с записью
// kusec. Доступ — по своим app (HasAppAccess на app секрета), не только админ.
func (u *Usecase) GetClusterSecret(ctx context.Context, secretId string) (*kubeService.ClusterResource, bool, bool, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, false, false, errs.NotAuthorized
	}
	if secretId == "" {
		return nil, false, false, errs.IdRequired
	}

	secret, _, err := u.secretSvc.Get(ctx, secretId, true)
	if err != nil {
		// ObjectNotFound пробрасываем как сентинель.
		if _, ok := errors.AsType[errs.Err](err); ok {
			return nil, false, false, err
		}
		return nil, false, false, fmt.Errorf("secretSvc.Get: %w", err)
	}

	if !u.sessionSvc.FromContext(ctx).HasAppAccess(secret.AppId) {
		return nil, false, false, errs.NoPermission
	}

	app, _, err := u.appSvc.Get(ctx, secret.AppId, true)
	if err != nil {
		if _, ok := errors.AsType[errs.Err](err); ok {
			return nil, false, false, err
		}
		return nil, false, false, fmt.Errorf("appSvc.Get: %w", err)
	}

	name := kubeService.SecretName(app.SlugName, secret.SlugName, secret.ExactSlug)

	resource, inCluster, found, err := u.svc.GetClusterSecret(ctx, app.Namespace, name)
	if err != nil {
		return nil, false, false, fmt.Errorf("svc.GetClusterSecret: %w", err)
	}

	return resource, inCluster, found, nil
}

// GetClusterConfigMap отдаёт живой k8s-configmap из кластера для сверки.
// Доступ — по своим app (HasAppAccess на app configmap-а).
func (u *Usecase) GetClusterConfigMap(ctx context.Context, configMapId string) (*kubeService.ClusterResource, bool, bool, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, false, false, errs.NotAuthorized
	}
	if configMapId == "" {
		return nil, false, false, errs.IdRequired
	}

	configMap, _, err := u.configMapSvc.Get(ctx, configMapId, true)
	if err != nil {
		if _, ok := errors.AsType[errs.Err](err); ok {
			return nil, false, false, err
		}
		return nil, false, false, fmt.Errorf("configMapSvc.Get: %w", err)
	}

	if !u.sessionSvc.FromContext(ctx).HasAppAccess(configMap.AppId) {
		return nil, false, false, errs.NoPermission
	}

	app, _, err := u.appSvc.Get(ctx, configMap.AppId, true)
	if err != nil {
		if _, ok := errors.AsType[errs.Err](err); ok {
			return nil, false, false, err
		}
		return nil, false, false, fmt.Errorf("appSvc.Get: %w", err)
	}

	name := kubeService.ConfigMapName(app.SlugName, configMap.SlugName, configMap.ExactSlug)

	resource, inCluster, found, err := u.svc.GetClusterConfigMap(ctx, app.Namespace, name)
	if err != nil {
		return nil, false, false, fmt.Errorf("svc.GetClusterConfigMap: %w", err)
	}

	return resource, inCluster, found, nil
}

func (u *Usecase) SyncSecrets(ctx context.Context, appId *string) (*kubeService.SyncResult, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}

	scope, err := u.syncScope(ctx, appId)
	if err != nil {
		return nil, err
	}

	result, err := u.svc.SyncSecrets(ctx, scope)
	if err != nil {
		// Сентинельные коды (not_in_cluster, sync_in_progress) пробрасываем
		// как есть — интерцептор превратит их в осмысленный ответ клиенту.
		if _, ok := errors.AsType[errs.Err](err); ok {
			return nil, err
		}
		return nil, fmt.Errorf("svc.SyncSecrets: %w", err)
	}

	return result, nil
}

func (u *Usecase) SyncConfigMaps(ctx context.Context, appId *string) (*kubeService.SyncResult, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}

	scope, err := u.syncScope(ctx, appId)
	if err != nil {
		return nil, err
	}

	result, err := u.svc.SyncConfigMaps(ctx, scope)
	if err != nil {
		// Сентинельные коды (not_in_cluster, sync_in_progress) пробрасываем
		// как есть — интерцептор превратит их в осмысленный ответ клиенту.
		if _, ok := errors.AsType[errs.Err](err); ok {
			return nil, err
		}
		return nil, fmt.Errorf("svc.SyncConfigMaps: %w", err)
	}

	return result, nil
}

// Sync выполняет общую синхронизацию секретов и configmap-ов за один вызов.
func (u *Usecase) Sync(ctx context.Context, appId *string) (*kubeService.SyncResult, *kubeService.SyncResult, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, nil, errs.NotAuthorized
	}

	scope, err := u.syncScope(ctx, appId)
	if err != nil {
		return nil, nil, err
	}

	secrets, configMaps, err := u.svc.Sync(ctx, scope)
	if err != nil {
		// Сентинельные коды (not_in_cluster, sync_in_progress) пробрасываем
		// как есть — интерцептор превратит их в осмысленный ответ клиенту.
		if _, ok := errors.AsType[errs.Err](err); ok {
			return nil, nil, err
		}
		return nil, nil, fmt.Errorf("svc.Sync: %w", err)
	}

	return secrets, configMaps, nil
}

// syncScope определяет область синхронизации: при заданном appId — один app
// (с проверкой доступа), иначе — все доступные app (или nil, если доступны все).
func (u *Usecase) syncScope(ctx context.Context, appId *string) ([]string, error) {
	session := u.sessionSvc.FromContext(ctx)
	accessibleAppIds, all := session.AccessibleAppIds()

	if appId != nil && *appId != "" {
		if !session.HasAppAccess(*appId) {
			return nil, errs.NoPermission
		}
		return []string{*appId}, nil
	}
	if !all {
		return accessibleAppIds, nil
	}
	return nil, nil
}
