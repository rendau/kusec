package kube

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/rendau/kusec/internal/config"
	syncrunModel "github.com/rendau/kusec/internal/domain/syncrun/model"
	"github.com/rendau/kusec/internal/util"
)

// Аннотации, проставляемые на синхронизированные k8s-объекты: по ним pulse
// связывает объект кластера с записью журнала sync и рестартом от reloader.
// Ставятся только при create/update (data изменилась) — на unchanged-объекты
// не пишутся, чтобы каждый запуск sync не «трогал» все объекты кластера.
const (
	syncedAtAnnotation    = "kusec.io/synced-at"
	syncRunIdAnnotation   = "kusec.io/sync-run-id"
	contentHashAnnotation = "kusec.io/content-hash"
)

const (
	kubeKindSecret    = "Secret"
	kubeKindConfigMap = "ConfigMap"
)

// runMeta — контекст текущего запуска sync для аннотаций объектов.
// nil (в тестах реконсиляции) — аннотации запуска не проставляются.
type runMeta struct {
	id string
	at string // RFC3339
}

// annotate проставляет аннотации запуска sync и отпечатка содержимого.
func (m *runMeta) annotate(annotations map[string]string, contentHash string) {
	if m == nil {
		return
	}
	annotations[syncedAtAnnotation] = m.at
	annotations[syncRunIdAnnotation] = m.id
	annotations[contentHashAnnotation] = contentHash
}

// syncJournal накапливает записи по объектам одного запуска sync.
// nil-безопасен: без журнала (в тестах) записи просто не собираются.
type syncJournal struct {
	objects   []*syncrunModel.Object
	hasErrors bool
}

func (j *syncJournal) add(kind, namespace, name, op, contentHash string, changedKeys []string) {
	if j == nil {
		return
	}
	sort.Strings(changedKeys)
	j.objects = append(j.objects, &syncrunModel.Object{
		Namespace:   namespace,
		KubeKind:    kind,
		KubeName:    name,
		Op:          op,
		ContentHash: contentHash,
		ChangedKeys: changedKeys,
	})
}

func (j *syncJournal) addError(kind, namespace, name, msg string) {
	if j == nil {
		return
	}
	j.hasErrors = true
	j.objects = append(j.objects, &syncrunModel.Object{
		Namespace: namespace,
		KubeKind:  kind,
		KubeName:  name,
		Op:        syncrunModel.OpError,
		Error:     msg,
	})
}

// ── Отпечатки содержимого и изменившиеся ключи ──────────

// contentFingerprint — HMAC-отпечаток канонизированного содержимого объекта
// (отсортированные пары ключ/значение); значения наружу не выходят.
func contentFingerprint(data map[string][]byte) string {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	canon := make([]byte, 0, 256)
	for _, key := range keys {
		canon = append(canon, key...)
		canon = append(canon, 0)
		canon = append(canon, data[key]...)
		canon = append(canon, 1)
	}

	return util.Fingerprint(config.Conf.AuditHashKey, canon)
}

// configMapContent объединяет текстовые и бинарные значения configmap-а в
// одно представление для отпечатка и сравнения ключей.
func configMapContent(data map[string]string, binaryData map[string][]byte) map[string][]byte {
	merged := make(map[string][]byte, len(data)+len(binaryData))
	for key, value := range data {
		merged[key] = []byte(value)
	}
	for key, value := range binaryData {
		merged[key] = value
	}
	return merged
}

// secretContent — содержимое живого k8s-секрета.
func secretContent(secret *corev1.Secret) map[string][]byte {
	return secret.Data
}

// liveConfigMapContent — содержимое живого k8s-configmap.
func liveConfigMapContent(configMap *corev1.ConfigMap) map[string][]byte {
	return configMapContent(configMap.Data, configMap.BinaryData)
}

// changedKeys — имена ключей, у которых значение добавлено, удалено или
// изменилось (сами значения не сохраняются).
func changedKeys(current, want map[string][]byte) []string {
	keys := make([]string, 0)
	for key, wantValue := range want {
		currentValue, ok := current[key]
		if !ok || string(currentValue) != string(wantValue) {
			keys = append(keys, key)
		}
	}
	for key := range current {
		if _, ok := want[key]; !ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

// allKeys — все ключи содержимого (для created/deleted-объектов).
func allKeys(data map[string][]byte) []string {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// ── Жизненный цикл запуска ──────────────────────────────

// startSyncRun создаёт запись запуска в журнале sync (status=running).
func (s *Service) startSyncRun(ctx context.Context, appIds []string) (string, *runMeta, error) {
	run := s.auditRec.NewSyncRun(ctx, runScopeAppId(appIds))

	runId, err := s.syncRunSvc.Start(ctx, run)
	if err != nil {
		return "", nil, fmt.Errorf("syncRunSvc.Start: %w", err)
	}

	return runId, &runMeta{id: runId, at: time.Now().UTC().Format(time.RFC3339)}, nil
}

// finishSyncRun финализирует запуск: объекты журнала, статус и запись аудита
// сохраняются одной транзакцией. Ошибка финализации sync не роняет — итог
// применения в кластере важнее записи о нём.
func (s *Service) finishSyncRun(
	ctx context.Context,
	runId string,
	startedAt time.Time,
	appIds []string,
	journal *syncJournal,
	resultErrors int,
	syncErr error,
) {
	status := syncrunModel.StatusOk
	errText := ""
	switch {
	case syncErr != nil:
		status = syncrunModel.StatusError
		errText = syncErr.Error()
	case journal.hasErrors || resultErrors > 0:
		status = syncrunModel.StatusPartial
	}

	duration := time.Since(startedAt)

	err := s.txm.TxFn(ctx, func(ctx context.Context) error {
		if err := s.syncRunSvc.AddObjects(ctx, runId, journal.objects); err != nil {
			return fmt.Errorf("syncRunSvc.AddObjects: %w", err)
		}
		err := s.syncRunSvc.Finish(ctx, runId, &syncrunModel.Edit{
			FinishedAt: new(time.Now()),
			Status:     &status,
			Error:      &errText,
			DurationMs: new(duration.Milliseconds()),
		})
		if err != nil {
			return fmt.Errorf("syncRunSvc.Finish: %w", err)
		}
		if err = s.auditRec.RecordSyncRun(ctx, runId, runScopeAppId(appIds)); err != nil {
			return fmt.Errorf("auditRec.RecordSyncRun: %w", err)
		}
		return nil
	})
	if err != nil {
		slog.Error("kube: finish sync run", "error", err, "run_id", runId)
	}

	observeSyncRun(status, duration)
}

// runScopeAppId — app_id запуска: один app в области — он, иначе nil (все).
func runScopeAppId(appIds []string) *string {
	if len(appIds) == 1 {
		return new(appIds[0])
	}
	return nil
}

// touchSynced отмечает последний применённый снимок записи secret/configmap.
// Ошибка не прерывает sync: снимок догонится следующим запуском.
func touchSynced(ctx context.Context, touch func(context.Context, string, time.Time, string) error, id, hash, key string) {
	if id == "" {
		return
	}
	if err := touch(ctx, id, time.Now(), hash); err != nil {
		slog.Warn("kube: touch last_synced", "error", err, "id", id, "object", key)
	}
}
