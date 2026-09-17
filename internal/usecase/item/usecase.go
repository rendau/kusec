package item

import (
	"context"
	"fmt"

	"github.com/rendau/kusec/internal/config"
	"github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
	"github.com/rendau/kusec/internal/errs"
	"github.com/rendau/kusec/internal/util"
)

type Usecase struct {
	svc        ServiceI
	secretSvc  SecretServiceI
	sessionSvc SessionServiceI
	txm        TransactionManagerI
	auditRec   AuditRecorderI
}

func New(
	svc ServiceI,
	secretSvc SecretServiceI,
	sessionSvc SessionServiceI,
	txm TransactionManagerI,
	auditRec AuditRecorderI,
) *Usecase {
	return &Usecase{
		svc:        svc,
		secretSvc:  secretSvc,
		sessionSvc: sessionSvc,
		txm:        txm,
		auditRec:   auditRec,
	}
}

func (u *Usecase) requireSecretAccess(ctx context.Context, secretId string) error {
	session := u.sessionSvc.FromContext(ctx)
	if _, all := session.AccessibleAppIds(); all {
		return nil
	}

	secret, _, err := u.secretSvc.Get(ctx, secretId, true)
	if err != nil {
		return fmt.Errorf("secretSvc.Get: %w", err)
	}
	if !session.HasAppAccess(secret.AppId) {
		return errs.NoPermission
	}
	return nil
}

// requireSecretsAccess проверяет доступ ко всем секретам одним запросом:
// каждый запрошенный id должен существовать и принадлежать доступному app.
func (u *Usecase) requireSecretsAccess(ctx context.Context, secretIds []string) error {
	session := u.sessionSvc.FromContext(ctx)
	if _, all := session.AccessibleAppIds(); all {
		return nil
	}

	secrets, _, err := u.secretSvc.List(ctx, &secretModel.ListReq{Ids: secretIds})
	if err != nil {
		return fmt.Errorf("secretSvc.List: %w", err)
	}

	appBySecret := make(map[string]string, len(secrets))
	for _, secret := range secrets {
		appBySecret[secret.Id] = secret.AppId
	}

	for _, id := range secretIds {
		appId, ok := appBySecret[id]
		if !ok || !session.HasAppAccess(appId) {
			return errs.NoPermission
		}
	}
	return nil
}

func (u *Usecase) validateEdit(obj *model.Edit, forCreate bool) error {
	if forCreate {
		if obj.SecretId == nil || *obj.SecretId == "" {
			return errs.InvalidRequest
		}
		if obj.Key == nil || *obj.Key == "" {
			return errs.InvalidRequest
		}
	}
	if obj.SecretId != nil && *obj.SecretId == "" {
		return errs.InvalidRequest
	}
	if obj.Key != nil && *obj.Key == "" {
		return errs.InvalidRequest
	}
	return nil
}

func (u *Usecase) List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, 0, errs.NotAuthorized
	}
	// Запрос считается узким (без обязательной пагинации), если ограничен
	// одним секретом, набором секретов или приложением.
	scopedBySecret := pars.SecretId != nil || len(pars.SecretIds) > 0 || pars.AppId != nil
	if !scopedBySecret {
		if err := util.RequirePageSize(pars.ListParams, 0); err != nil {
			return nil, 0, err
		}
	}

	session := u.sessionSvc.FromContext(ctx)
	if _, all := session.AccessibleAppIds(); !all {
		if !scopedBySecret {
			return nil, 0, errs.NoPermission
		}
		if pars.SecretId != nil {
			if err := u.requireSecretAccess(ctx, *pars.SecretId); err != nil {
				return nil, 0, err
			}
		}
		if len(pars.SecretIds) > 0 {
			if err := u.requireSecretsAccess(ctx, pars.SecretIds); err != nil {
				return nil, 0, err
			}
		}
		if pars.AppId != nil && !session.HasAppAccess(*pars.AppId) {
			return nil, 0, errs.NoPermission
		}
	}

	items, tCount, err := u.svc.List(ctx, pars)
	if err != nil {
		return nil, 0, fmt.Errorf("svc.List: %w", err)
	}
	u.applyValuePolicy(ctx, items)
	return items, tCount, nil
}

func (u *Usecase) Get(ctx context.Context, id string) (*model.Main, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}
	result, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}
	if err = u.requireSecretAccess(ctx, result.SecretId); err != nil {
		return nil, err
	}
	u.applyValuePolicy(ctx, []*model.Main{result})
	return result, nil
}

// applyValuePolicy заполняет вычисляемые value_size/value_hash и прячет
// значения от read_only-сессий (им отдаются только размер и отпечаток).
func (u *Usecase) applyValuePolicy(ctx context.Context, items []*model.Main) {
	readOnly := u.sessionSvc.FromContext(ctx).IsReadOnly()
	for _, item := range items {
		item.ValueSize = int64(len(item.Value))
		item.ValueHash = util.ValueFingerprint(config.Conf.AuditHashKey, item.Value)
		if readOnly {
			item.Value = ""
		}
	}
}

func (u *Usecase) Create(ctx context.Context, obj *model.Edit) (string, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return "", errs.NotAuthorized
	}
	if err := u.validateEdit(obj, true); err != nil {
		return "", err
	}
	if err := u.requireSecretAccess(ctx, *obj.SecretId); err != nil {
		return "", err
	}

	var newId string
	err := u.txm.TxFn(ctx, func(ctx context.Context) error {
		var err error
		newId, err = u.svc.Create(ctx, obj)
		if err != nil {
			return fmt.Errorf("svc.Create: %w", err)
		}

		created, _, err := u.svc.Get(ctx, newId, true)
		if err != nil {
			return fmt.Errorf("svc.Get: %w", err)
		}
		if err = u.auditRec.RecordItem(ctx, nil, created, "", nil); err != nil {
			return fmt.Errorf("auditRec.RecordItem: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return newId, nil
}

func (u *Usecase) Update(ctx context.Context, id string, obj *model.Edit) error {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return errs.NotAuthorized
	}
	if id == "" {
		return errs.IdRequired
	}
	if err := u.validateEdit(obj, false); err != nil {
		return err
	}

	current, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return fmt.Errorf("svc.Get: %w", err)
	}
	if err = u.requireSecretAccess(ctx, current.SecretId); err != nil {
		return err
	}
	if obj.SecretId != nil && *obj.SecretId != current.SecretId {
		if err = u.requireSecretAccess(ctx, *obj.SecretId); err != nil {
			return err
		}
	}

	return u.txm.TxFn(ctx, func(ctx context.Context) error {
		if err = u.svc.Update(ctx, id, obj); err != nil {
			return fmt.Errorf("svc.Update: %w", err)
		}

		updated, _, err := u.svc.Get(ctx, id, true)
		if err != nil {
			return fmt.Errorf("svc.Get: %w", err)
		}
		if err = u.auditRec.RecordItem(ctx, current, updated, "", nil); err != nil {
			return fmt.Errorf("auditRec.RecordItem: %w", err)
		}
		return nil
	})
}

func (u *Usecase) Delete(ctx context.Context, id string) error {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return errs.NotAuthorized
	}
	if id == "" {
		return errs.IdRequired
	}

	current, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return fmt.Errorf("svc.Get: %w", err)
	}
	if err = u.requireSecretAccess(ctx, current.SecretId); err != nil {
		return err
	}

	return u.txm.TxFn(ctx, func(ctx context.Context) error {
		// запись аудита — до удаления, пока резолвится контекст secret/app
		if err = u.auditRec.RecordItem(ctx, current, nil, "", nil); err != nil {
			return fmt.Errorf("auditRec.RecordItem: %w", err)
		}
		if err = u.svc.Delete(ctx, id); err != nil {
			return fmt.Errorf("svc.Delete: %w", err)
		}
		return nil
	})
}
