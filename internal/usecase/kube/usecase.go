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

func (u *Usecase) SyncSecrets(ctx context.Context, appId *string) (*kubeService.SyncResult, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}

	session := u.sessionSvc.FromContext(ctx)
	accessibleAppIds, all := session.AccessibleAppIds()

	var scope []string
	if appId != nil && *appId != "" {
		if !session.HasAppAccess(*appId) {
			return nil, errs.NoPermission
		}
		scope = []string{*appId}
	} else if !all {
		scope = accessibleAppIds
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
