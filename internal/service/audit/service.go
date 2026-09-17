// Package audit — регистратор аудита: дополняет событие актором (сессия из
// контекста), контекстом объекта (app, namespace, имя k8s-объекта), строит
// список изменённых полей и пишет запись через доменный сервис. Значения
// item-ов (секретов) в запись не попадают никогда — только HMAC-отпечаток
// и размер (см. changes.go).
//
// Вызывается из usecase-слоя внутри той же транзакции, что и мутация
// (mobone TxFn): изменение без записи аудита или запись без изменения
// невозможны.
package audit

import (
	"context"
	"fmt"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
	auditModel "github.com/rendau/kusec/internal/domain/audit/model"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	"github.com/rendau/kusec/internal/util"
)

type Service struct {
	auditSvc     AuditServiceI
	usrSvc       UsrServiceI
	appSvc       AppServiceI
	secretSvc    SecretServiceI
	configMapSvc ConfigMapServiceI
	sessionSvc   SessionServiceI

	hashKey string
}

func New(
	auditSvc AuditServiceI,
	usrSvc UsrServiceI,
	appSvc AppServiceI,
	secretSvc SecretServiceI,
	configMapSvc ConfigMapServiceI,
	sessionSvc SessionServiceI,
	hashKey string,
) *Service {
	return &Service{
		auditSvc:     auditSvc,
		usrSvc:       usrSvc,
		appSvc:       appSvc,
		secretSvc:    secretSvc,
		configMapSvc: configMapSvc,
		sessionSvc:   sessionSvc,
		hashKey:      hashKey,
	}
}

// NewBatchId — общий идентификатор для группы записей одной массовой
// операции (каскадное удаление, импорт, sync).
func (s *Service) NewBatchId() string {
	return util.NewRequestId()
}

// record дополняет запись актором и request id и сохраняет её.
// Пустой update (ни одного изменённого поля) не пишется.
func (s *Service) record(ctx context.Context, entry *auditModel.Main) error {
	if entry.Action == auditModel.ActionUpdate && len(entry.Changes) == 0 {
		return nil
	}

	session := s.sessionSvc.FromContext(ctx)

	entry.Source = session.Source
	if entry.Source == "" {
		entry.Source = sourceOf(session)
	}
	entry.RequestId = util.RequestIdFromContext(ctx)
	if session.IsAuthorized() {
		entry.ActorUsrId = new(session.Id)
	}
	if session.ApiKeyId != "" {
		entry.ActorApiKeyId = new(session.ApiKeyId)
	}
	entry.ActorName = s.actorName(ctx, session)

	observeChange(entry)

	if err := s.auditSvc.Create(ctx, entry); err != nil {
		return fmt.Errorf("auditSvc.Create: %w", err)
	}
	return nil
}

// sourceOf — источник для сессий без проставленного Source: фоновые
// процессы (нет пользователя) считаются system.
func sourceOf(session *sessionModel.Session) string {
	if session.IsAuthorized() {
		return ""
	}
	return "system"
}

// actorName — имя актора: из сессии (api-ключ заполняет её при
// аутентификации) либо по usr_id (JWT-сессии несут только id).
func (s *Service) actorName(ctx context.Context, session *sessionModel.Session) string {
	name := session.Name
	if name == "" && session.IsAuthorized() {
		usr, found, err := s.usrSvc.Get(ctx, session.Id, false)
		if err == nil && found {
			name = usr.Name
		} else {
			name = fmt.Sprintf("usr:%d", session.Id)
		}
	}
	if name == "" {
		return "system"
	}
	if session.ApiKeyName != "" {
		return name + " (" + session.ApiKeyName + ")"
	}
	return name
}

// deriveAction — действие по наличию сторон и составу изменений: смена
// только поля active трактуется как activate/deactivate.
func deriveAction(hasOld, hasNew bool, changes []auditModel.Change) string {
	switch {
	case !hasOld:
		return auditModel.ActionCreate
	case !hasNew:
		return auditModel.ActionDelete
	}

	if len(changes) == 1 && changes[0].Field == "active" && changes[0].New != nil {
		if *changes[0].New == "true" {
			return auditModel.ActionActivate
		}
		return auditModel.ActionDeactivate
	}

	return auditModel.ActionUpdate
}

// appContext — контекст объекта по приложению; отсутствие app (не должно
// случаться внутри транзакции мутации) не роняет запись аудита.
func (s *Service) appContext(ctx context.Context, appId string, entry *auditModel.Main) *appModel.Main {
	if appId == "" {
		return nil
	}
	entry.AppId = new(appId)

	app, found, err := s.appSvc.Get(ctx, appId, false)
	if err != nil || !found {
		return nil
	}
	entry.Namespace = app.Namespace
	entry.AppSlug = app.SlugName
	return app
}
