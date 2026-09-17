package audit

import (
	"context"
	"strconv"

	apikeyModel "github.com/rendau/kusec/internal/domain/apikey/model"
	appModel "github.com/rendau/kusec/internal/domain/app/model"
	auditModel "github.com/rendau/kusec/internal/domain/audit/model"
	configitemModel "github.com/rendau/kusec/internal/domain/configitem/model"
	configmapModel "github.com/rendau/kusec/internal/domain/configmap/model"
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
	syncrunModel "github.com/rendau/kusec/internal/domain/syncrun/model"
	usrModel "github.com/rendau/kusec/internal/domain/usr/model"
	kubeService "github.com/rendau/kusec/internal/service/kube"
	"github.com/rendau/kusec/internal/util"
)

// Во всех Record* old=nil означает создание, cur=nil — удаление, обе стороны —
// обновление (пустой дифф не пишется). action переопределяет выведенное
// действие (например import); пустая строка — вывести автоматически.

func (s *Service) RecordApp(ctx context.Context, old, cur *appModel.Main, action string, batchId *string) error {
	side := pick(old, cur)
	changes := appChanges(old, cur)

	entry := &auditModel.Main{
		EntityType: auditModel.EntityApp,
		EntityId:   side.Id,
		AppId:      new(side.Id),
		Namespace:  side.Namespace,
		AppSlug:    side.SlugName,
		Action:     actionOr(action, old != nil, cur != nil, changes),
		Changes:    changes,
		BatchId:    batchId,
	}
	return s.record(ctx, entry)
}

func (s *Service) RecordSecret(ctx context.Context, old, cur *secretModel.Main, action string, batchId *string) error {
	side := pick(old, cur)
	changes := secretChanges(old, cur)

	entry := &auditModel.Main{
		EntityType: auditModel.EntitySecret,
		EntityId:   side.Id,
		KubeKind:   auditModel.KubeKindSecret,
		Action:     actionOr(action, old != nil, cur != nil, changes),
		Changes:    changes,
		BatchId:    batchId,
	}
	if app := s.appContext(ctx, side.AppId, entry); app != nil {
		entry.KubeName = kubeService.SecretName(app.SlugName, side.SlugName, side.ExactSlug)
	}
	return s.record(ctx, entry)
}

func (s *Service) RecordItem(ctx context.Context, old, cur *itemModel.Main, action string, batchId *string) error {
	side := pick(old, cur)
	changes := s.itemChanges(old, cur)

	entry := &auditModel.Main{
		EntityType: auditModel.EntityItem,
		EntityId:   side.Id,
		KubeKind:   auditModel.KubeKindSecret,
		Key:        side.Key,
		Action:     actionOr(action, old != nil, cur != nil, changes),
		Changes:    changes,
		BatchId:    batchId,
	}
	if secret, found, err := s.secretSvc.Get(ctx, side.SecretId, false); err == nil && found {
		if app := s.appContext(ctx, secret.AppId, entry); app != nil {
			entry.KubeName = kubeService.SecretName(app.SlugName, secret.SlugName, secret.ExactSlug)
		}
	}
	return s.record(ctx, entry)
}

func (s *Service) RecordConfigMap(ctx context.Context, old, cur *configmapModel.Main, action string, batchId *string) error {
	side := pick(old, cur)
	changes := configMapChanges(old, cur)

	entry := &auditModel.Main{
		EntityType: auditModel.EntityConfigMap,
		EntityId:   side.Id,
		KubeKind:   auditModel.KubeKindConfigMap,
		Action:     actionOr(action, old != nil, cur != nil, changes),
		Changes:    changes,
		BatchId:    batchId,
	}
	if app := s.appContext(ctx, side.AppId, entry); app != nil {
		entry.KubeName = kubeService.ConfigMapName(app.SlugName, side.SlugName, side.ExactSlug)
	}
	return s.record(ctx, entry)
}

func (s *Service) RecordConfigItem(ctx context.Context, old, cur *configitemModel.Main, action string, batchId *string) error {
	side := pick(old, cur)
	changes := s.configItemChanges(old, cur)

	entry := &auditModel.Main{
		EntityType: auditModel.EntityConfigItem,
		EntityId:   side.Id,
		KubeKind:   auditModel.KubeKindConfigMap,
		Key:        side.Key,
		Action:     actionOr(action, old != nil, cur != nil, changes),
		Changes:    changes,
		BatchId:    batchId,
	}
	if configMap, found, err := s.configMapSvc.Get(ctx, side.ConfigMapId, false); err == nil && found {
		if app := s.appContext(ctx, configMap.AppId, entry); app != nil {
			entry.KubeName = kubeService.ConfigMapName(app.SlugName, configMap.SlugName, configMap.ExactSlug)
		}
	}
	return s.record(ctx, entry)
}

func (s *Service) RecordApiKey(ctx context.Context, old, cur *apikeyModel.Main, batchId *string) error {
	side := old
	if cur != nil {
		side = cur
	}
	changes := apiKeyChanges(old, cur)

	entry := &auditModel.Main{
		EntityType: auditModel.EntityApiKey,
		EntityId:   side.Id,
		Action:     deriveAction(old != nil, cur != nil, changes),
		Changes:    changes,
		BatchId:    batchId,
	}
	return s.record(ctx, entry)
}

func (s *Service) RecordUsr(ctx context.Context, old, cur *usrModel.Main, batchId *string) error {
	side := old
	if cur != nil {
		side = cur
	}
	changes := usrChanges(old, cur)

	entry := &auditModel.Main{
		EntityType: auditModel.EntityUsr,
		EntityId:   formatUsrId(side.Id),
		Action:     deriveAction(old != nil, cur != nil, changes),
		Changes:    changes,
		BatchId:    batchId,
	}
	return s.record(ctx, entry)
}

// NewSyncRun готовит запись запуска sync с актором из сессии (для журнала
// sync_run; сам запуск создаёт kube-сервис через syncrun domain service).
func (s *Service) NewSyncRun(ctx context.Context, appId *string) *syncrunModel.Main {
	session := s.sessionSvc.FromContext(ctx)

	run := &syncrunModel.Main{
		ActorName: s.actorName(ctx, session),
		Source:    session.Source,
		RequestId: util.RequestIdFromContext(ctx),
		AppId:     appId,
	}
	if run.Source == "" {
		run.Source = "system"
	}
	if session.IsAuthorized() {
		run.ActorUsrId = new(session.Id)
	}
	if session.ApiKeyId != "" {
		run.ActorApiKeyId = new(session.ApiKeyId)
	}
	return run
}

// RecordSyncRun — одна запись на запуск sync (детали — в журнале sync_run,
// batch_id связывает записи).
func (s *Service) RecordSyncRun(ctx context.Context, runId string, appId *string) error {
	entry := &auditModel.Main{
		EntityType: auditModel.EntitySyncRun,
		EntityId:   runId,
		Action:     auditModel.ActionSync,
		BatchId:    new(runId),
	}
	if appId != nil {
		s.appContext(ctx, *appId, entry)
	}
	return s.record(ctx, entry)
}

// apiKeyChanges — хэш ключа не пишется никогда; префикс не секретен.
func apiKeyChanges(old, cur *apikeyModel.Main) []auditModel.Change {
	c := newChangeSet(old != nil, cur != nil)
	var o, n apikeyModel.Main
	if old != nil {
		o = *old
	}
	if cur != nil {
		n = *cur
	}
	c.boolF("active", o.Active, n.Active)
	c.boolF("mcp_only", o.McpOnly, n.McpOnly)
	c.str("name", o.Name, n.Name)
	c.int64F("usr_id", o.UsrId, n.UsrId)
	c.str("key_prefix", o.KeyPrefix, n.KeyPrefix)
	return c.changes
}

// actionOr — явное действие либо выведенное из сторон и изменений.
func actionOr(action string, hasOld, hasNew bool, changes []auditModel.Change) string {
	if action != "" {
		return action
	}
	return deriveAction(hasOld, hasNew, changes)
}

// formatUsrId — entity_id для usr (числовой id в текстовой колонке).
func formatUsrId(id int64) string {
	return strconv.FormatInt(id, 10)
}

// pick — сторона, описывающая объект: текущая, а для удаления — старая.
func pick[T any](old, cur *T) *T {
	if cur != nil {
		return cur
	}
	return old
}
