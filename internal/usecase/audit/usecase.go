package audit

import (
	"context"
	"fmt"

	"github.com/rendau/kusec/internal/config"
	auditModel "github.com/rendau/kusec/internal/domain/audit/model"
	"github.com/rendau/kusec/internal/errs"
	"github.com/rendau/kusec/internal/util"
)

// configValueField — поле записи аудита со значением config_item
// (configItemChanges в internal/service/audit).
const configValueField = "value"

// Usecase — чтение аудита. Записи через API не редактируются и не
// удаляются: мутирующих методов нет намеренно.
type Usecase struct {
	svc        ServiceI
	sessionSvc SessionServiceI
}

func New(svc ServiceI, sessionSvc SessionServiceI) *Usecase {
	return &Usecase{
		svc:        svc,
		sessionSvc: sessionSvc,
	}
}

func (u *Usecase) List(ctx context.Context, pars *auditModel.ListReq) ([]*auditModel.Main, int64, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, 0, errs.NotAuthorized
	}
	if err := util.RequirePageSize(pars.ListParams, 0); err != nil {
		return nil, 0, err
	}

	// Не-админ видит только записи приложений — и только своих. Записи без
	// app_id (учётки, API-ключи, OAuth-подключения, общий sync) — только
	// админу: в них логины, флаги админа, имена ключей. Это касается и
	// не-админа с all_apps: «все приложения» — не «весь аудит». Условия
	// клиента сюда не входят: OnlyWithApp и AppIds выставляются только здесь.
	session := u.sessionSvc.FromContext(ctx)
	pars.OnlyWithApp = false
	pars.AppIds = nil
	if !session.IsAdmin() {
		appIds, all := session.AccessibleAppIds()
		switch {
		case pars.AppId != nil:
			if !session.HasAppAccess(*pars.AppId) {
				return nil, 0, errs.NoPermission
			}
		case all:
			pars.OnlyWithApp = true
		case len(appIds) == 0:
			// ни одного доступного приложения; репозиторий пустой срез
			// фильтром не считает и вернул бы весь аудит
			return []*auditModel.Main{}, 0, nil
		default:
			pars.AppIds = appIds
		}
	}

	items, tCount, err := u.svc.List(ctx, pars)
	if err != nil {
		return nil, 0, fmt.Errorf("svc.List: %w", err)
	}

	// значения config_item лежат в записях аудита целиком; сессии, которой
	// они не показываются (read_only-ключ), отдаём вместо них размер и
	// отпечаток — иначе аудит был бы обходом маскирования ConfigItem/List
	if u.sessionSvc.FromContext(ctx).ConfigValuesHidden() {
		hideConfigValues(items, config.Conf.AuditHashKey)
	}

	return items, tCount, nil
}

// hideConfigValues заменяет в записях аудита значения config_item на то же,
// что хранится вместо значений секретов (см. fingerprintChange в
// internal/service/audit): отпечаток и размер. Изменения, уже сведённые к
// отпечатку (значение длиннее лимита), остаются как есть.
func hideConfigValues(items []*auditModel.Main, hashKey string) {
	for _, item := range items {
		if item.EntityType != auditModel.EntityConfigItem {
			continue
		}
		for i := range item.Changes {
			change := &item.Changes[i]
			if change.Field != configValueField {
				continue
			}
			if change.Old != nil {
				change.OldHash = util.ValueFingerprint(hashKey, *change.Old)
				change.OldSize = new(int64(len(*change.Old)))
				change.Old = nil
			}
			if change.New != nil {
				change.NewHash = util.ValueFingerprint(hashKey, *change.New)
				change.NewSize = new(int64(len(*change.New)))
				change.New = nil
			}
		}
	}
}
