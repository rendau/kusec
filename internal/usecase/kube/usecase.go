package kube

import (
	"context"
	"fmt"

	"github.com/mechta-market/kusec/internal/errs"
	kubeService "github.com/mechta-market/kusec/internal/service/kube"
)

type Usecase struct {
	svc        KubeServiceI
	sessionSvc SessionServiceI
}

func New(svc KubeServiceI, sessionSvc SessionServiceI) *Usecase {
	return &Usecase{
		svc:        svc,
		sessionSvc: sessionSvc,
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

// ImportSecrets импортирует выбранные секреты кластера в указанное приложение
// (создание secret/item) — операция администратора.
func (u *Usecase) ImportSecrets(ctx context.Context, appId string, refs []kubeService.ImportRef) (*kubeService.ImportResult, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}
	if !u.sessionSvc.CtxIsAdmin(ctx) {
		return nil, errs.NoPermission
	}
	if appId == "" {
		return nil, errs.IdRequired
	}
	if len(refs) == 0 {
		return nil, errs.InvalidRequest
	}

	result, err := u.svc.ImportSecrets(ctx, appId, refs)
	if err != nil {
		// Сентинельные коды (not_in_cluster, object_not_found) пробрасываем как есть.
		if _, ok := err.(errs.Err); ok {
			return nil, err
		}
		return nil, fmt.Errorf("svc.ImportSecrets: %w", err)
	}

	return result, nil
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
		if _, ok := err.(errs.Err); ok {
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
		if _, ok := err.(errs.Err); ok {
			return nil, err
		}
		return nil, fmt.Errorf("svc.SyncConfigMaps: %w", err)
	}

	return result, nil
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
