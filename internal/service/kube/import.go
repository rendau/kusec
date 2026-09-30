package kube

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
	"github.com/rendau/kusec/internal/errs"
)

// importAction — действие записей аудита, порождённых импортом
// (auditModel.ActionImport; литерал — чтобы не тянуть домен audit сюда).
const importAction = "import"

// importSkippedTypes — типы секретов, не предназначенные для импорта:
// служебные токены ServiceAccount и хранилища релизов helm.
var importSkippedTypes = map[corev1.SecretType]bool{
	corev1.SecretTypeServiceAccountToken: true,
	"helm.sh/release.v1":                 true,
}

// ListClusterSecrets возвращает секреты кластера для выбора при импорте:
// без системных namespace-ов (kube-*) и без служебных типов (токены SA,
// helm-релизы). Значения не отдаются — только ключи data. Вне кластера это
// штатная ситуация: inCluster=false и пустой список без ошибки.
func (s *Service) ListClusterSecrets(ctx context.Context, namespace string) ([]*ClusterSecret, bool, error) {
	client, err := s.getClient()
	if err != nil {
		if errors.Is(err, errs.NotInCluster) {
			return nil, false, nil
		}
		return nil, false, err
	}

	ns := metav1.NamespaceAll
	if namespace != "" {
		ns = namespace
	}

	list, err := client.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, false, fmt.Errorf("k8s: list secrets: %w", err)
	}

	result := make([]*ClusterSecret, 0, len(list.Items))
	for i := range list.Items {
		sec := &list.Items[i]
		if systemNamespaces[sec.Namespace] || importSkippedTypes[sec.Type] {
			continue
		}

		result = append(result, &ClusterSecret{
			Namespace: sec.Namespace,
			Name:      sec.Name,
			Type:      displaySecretType(sec.Type),
			Keys:      sortedDataKeys(sec.Data),
			Managed:   sec.Labels[managedByLabelKey] == managedByLabelValue,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Namespace != result[j].Namespace {
			return result[i].Namespace < result[j].Namespace
		}
		return result[i].Name < result[j].Name
	})

	return result, true, nil
}

// ImportSecret переносит один секрет кластера в указанное приложение:
// секрет становится записью secret в appId с item-ами по ключам data.
// secretSlug задаёт имя посадочного секрета (обязателен, валидируется в
// usecase). Если секрет с таким slug в приложении уже есть — выполняется
// дозаполнение: недостающие ключи создаются, пустые item-ы заполняются,
// непустые перезаписываются только при opts.Overwrite (иначе пропускаются).
// opts.Keys ограничивает и переименовывает переносимые ключи. Источник в
// кластере не изменяется. Значения из кластера идут только в базу: в
// результат и тексты ошибок попадают лишь имена ключей.
func (s *Service) ImportSecret(ctx context.Context, appId string, ref ImportRef, secretSlug string, opts ImportOptions) (*ImportResult, error) {
	return s.importSingle(ctx, appId, ImportSpec{Ref: ref, Slug: secretSlug, Options: opts}, s.importSecretOne)
}

// ImportSecrets — пакетный импорт: несколько секретов кластера в одно
// приложение за один вызов (см. importBatch).
func (s *Service) ImportSecrets(ctx context.Context, appId string, specs []ImportSpec) ([]ImportBatchItem, error) {
	return s.importBatch(ctx, appId, specs, s.importSecretOne)
}

// ── Общий движок импорта (секреты и configmap-ы) ────────

// importOneFn — импорт одного объекта кластера: чтение источника, валидация
// slug и keys, затем мутации и аудит в одной транзакции. batchId общий для
// всех записей аудита (action=import) вызова.
type importOneFn func(ctx context.Context, client kubernetes.Interface, app *appModel.Main, spec ImportSpec, batchId *string) (*ImportResult, error)

// importSingle выполняет импорт одного объекта: проверяет общие предусловия
// (kusec в кластере, приложение существует) и вызывает one.
func (s *Service) importSingle(ctx context.Context, appId string, spec ImportSpec, one importOneFn) (*ImportResult, error) {
	client, err := s.getClient()
	if err != nil {
		return nil, err
	}

	// Проверяем существование целевого приложения. errNE → ObjectNotFound
	// возвращаем как есть (сентинель), чтобы usecase пробросил его клиенту.
	app, _, err := s.appSvc.Get(ctx, appId, true)
	if err != nil {
		return nil, err
	}

	return one(ctx, client, app, spec, new(s.auditRec.NewBatchId()))
}

// importBatch — пакетный импорт в одно приложение. Каждый объект готовится
// (чтение из кластера, валидация slug и keys) и импортируется в своей
// транзакции; ошибка одного объекта попадает в его ImportBatchItem.Err и не
// мешает остальным, поэтому повторный запуск с теми же спеками идемпотентен.
// Спеки обрабатываются по порядку: один и тот же Slug может встретиться
// несколько раз — каждый следующий дозаполняет результат предыдущего. Все
// записи аудита пакета получают общий batch_id. Ошибка возвращается только по
// общим предусловиям (kusec вне кластера, приложение не найдено).
func (s *Service) importBatch(ctx context.Context, appId string, specs []ImportSpec, one importOneFn) ([]ImportBatchItem, error) {
	client, err := s.getClient()
	if err != nil {
		return nil, err
	}

	app, _, err := s.appSvc.Get(ctx, appId, true)
	if err != nil {
		return nil, err
	}

	batchId := new(s.auditRec.NewBatchId())

	return lo.Map(specs, func(spec ImportSpec, _ int) ImportBatchItem {
		result, err := one(ctx, client, app, spec, batchId)
		return ImportBatchItem{Ref: spec.Ref, Slug: spec.Slug, Result: result, Err: err}
	}), nil
}

// importSlug валидирует slug посадочной записи: он входит в имя k8s-объекта,
// поэтому обязан быть DNS1123-subdomain. kind — для текста ошибки.
func importSlug(slug, kind string) (string, error) {
	slug = strings.TrimSpace(slug)
	if errMsgs := validation.IsDNS1123Subdomain(slug); len(errMsgs) > 0 {
		return "", errs.ErrFull{Err: errs.InvalidRequest, Desc: "invalid " + kind + " slug: " + strings.Join(errMsgs, "; ")}
	}
	return slug, nil
}

// newImportResult — пустой итог со стабильными (не nil) списками ключей.
func newImportResult(slug string) *ImportResult {
	return &ImportResult{
		Slug:        slug,
		CreatedKeys: []string{},
		FilledKeys:  []string{},
		UpdatedKeys: []string{},
		SkippedKeys: []string{},
	}
}

// importKeyEntry — один переносимый ключ: имя в источнике и имя item-а.
type importKeyEntry struct {
	sourceKey string
	itemKey   string
}

// importKeyPlan строит план переноса ключей по ключам источника (sourceKeys,
// отсортированы) и карте keys (ключ в кластере → имя item-а; пустое имя =
// оставить как есть). Без keys — все ключи источника 1:1. Ошибки — только с
// именами ключей: ключи, которых нет в источнике, невалидные имена item-ов,
// дубли имён. Порядок — по ключам источника.
func importKeyPlan(sourceKeys []string, keys map[string]string) ([]importKeyEntry, error) {
	if len(keys) == 0 {
		return lo.Map(sourceKeys, func(key string, _ int) importKeyEntry {
			return importKeyEntry{sourceKey: key, itemKey: key}
		}), nil
	}

	wanted := lo.Keys(keys)
	sort.Strings(wanted)

	if missing := lo.Without(wanted, sourceKeys...); len(missing) > 0 {
		return nil, errs.ErrFull{
			Err:  errs.InvalidRequest,
			Desc: "keys not found in the cluster source: " + strings.Join(missing, ", "),
		}
	}

	plan := make([]importKeyEntry, 0, len(wanted))
	seen := map[string]string{} // имя item-а → ключ источника
	for _, sourceKey := range wanted {
		itemKey := strings.TrimSpace(keys[sourceKey])
		if itemKey == "" {
			itemKey = sourceKey
		}
		if errMsgs := validation.IsConfigMapKey(itemKey); len(errMsgs) > 0 {
			return nil, errs.ErrFull{
				Err:  errs.InvalidRequest,
				Desc: fmt.Sprintf("invalid item key %q for cluster key %q: %s", itemKey, sourceKey, strings.Join(errMsgs, "; ")),
			}
		}
		if prev, dup := seen[itemKey]; dup {
			return nil, errs.ErrFull{
				Err:  errs.InvalidRequest,
				Desc: fmt.Sprintf("item key %q is mapped from both %q and %q", itemKey, prev, sourceKey),
			}
		}
		seen[itemKey] = sourceKey
		plan = append(plan, importKeyEntry{sourceKey: sourceKey, itemKey: itemKey})
	}

	return plan, nil
}

// applyImportPlan переносит ключи по плану и раскладывает их имена по
// категориям результата. existingValue отдаёт текущее значение item-а с таким
// именем (ok=false — item-а нет). Отсутствующий ключ создаётся; пустой item
// заполняется всегда; непустой перезаписывается только при overwrite, иначе
// пропускается. create/update выполняют запись (и аудит) для своего вида.
func applyImportPlan(
	result *ImportResult,
	plan []importKeyEntry,
	overwrite bool,
	existingValue func(itemKey string) (string, bool),
	create, update func(entry importKeyEntry) error,
) error {
	for _, entry := range plan {
		current, exists := existingValue(entry.itemKey)
		if !exists {
			if err := create(entry); err != nil {
				return err
			}
			result.CreatedKeys = append(result.CreatedKeys, entry.itemKey)
			continue
		}

		switch {
		case current == "":
			result.FilledKeys = append(result.FilledKeys, entry.itemKey)
		case overwrite:
			result.UpdatedKeys = append(result.UpdatedKeys, entry.itemKey)
		default:
			result.SkippedKeys = append(result.SkippedKeys, entry.itemKey)
			continue
		}

		if err := update(entry); err != nil {
			return err
		}
	}

	// при переименовании порядок плана — по ключам источника; списки отдаём
	// отсортированными по именам item-ов
	for _, keys := range [][]string{result.CreatedKeys, result.FilledKeys, result.UpdatedKeys, result.SkippedKeys} {
		sort.Strings(keys)
	}
	result.CreatedItems = int64(len(result.CreatedKeys))
	result.UpdatedItems = int64(len(result.FilledKeys) + len(result.UpdatedKeys))

	return nil
}

// ── Секреты ─────────────────────────────────────────────

// importSecretOne — importOneFn для секрета: находит или создаёт посадочный
// secret (kube_type копируется из источника) и переносит ключи data.
func (s *Service) importSecretOne(ctx context.Context, client kubernetes.Interface, app *appModel.Main, spec ImportSpec, batchId *string) (*ImportResult, error) {
	ksec, err := client.CoreV1().Secrets(spec.Ref.Namespace).Get(ctx, spec.Ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get cluster secret %s/%s: %w", spec.Ref.Namespace, spec.Ref.Name, err)
	}

	slug, err := importSlug(spec.Slug, "secret")
	if err != nil {
		return nil, err
	}

	// План проверяется до любых мутаций.
	plan, err := importKeyPlan(sortedDataKeys(ksec.Data), spec.Options.Keys)
	if err != nil {
		return nil, err
	}

	existing, err := s.findSecretBySlug(ctx, app.Id, slug)
	if err != nil {
		return nil, err
	}

	result := newImportResult(slug)

	// Мутации и записи аудита — в одной транзакции: частичный импорт не
	// оставит следов ни в базе, ни в аудите.
	err = s.txm.TxFn(ctx, func(ctx context.Context) error {
		// Существующий секрет переиспользуем (дозаполнение), иначе создаём
		// новый. existingItems: ключ → item (active и неактивные — чтобы найти
		// любой совпавший ключ и не плодить дубли).
		existingItems := map[string]*itemModel.Main{}
		if existing != nil {
			result.ObjectId = existing.Id
			result.KubeType = existing.KubeType
			result.KubeName = SecretName(app.SlugName, existing.SlugName, existing.ExactSlug)

			items, _, err := s.itemSvc.List(ctx, &itemModel.ListReq{SecretId: new(existing.Id)})
			if err != nil {
				return fmt.Errorf("list existing items: %w", err)
			}
			for _, it := range items {
				existingItems[it.Key] = it
			}
		} else {
			kubeType := displaySecretType(ksec.Type)

			secretId, err := s.secretSvc.Create(ctx, &secretModel.Edit{
				AppId:       new(app.Id),
				Active:      new(true),
				SlugName:    new(slug),
				Description: new(fmt.Sprintf("Imported from %s/%s", spec.Ref.Namespace, spec.Ref.Name)),
				KubeType:    new(kubeType),
			})
			if err != nil {
				return fmt.Errorf("create secret: %w", err)
			}
			result.ObjectId = secretId
			result.ObjectCreated = true
			result.KubeType = kubeType
			result.KubeName = SecretName(app.SlugName, slug, false)

			created, _, err := s.secretSvc.Get(ctx, secretId, true)
			if err != nil {
				return fmt.Errorf("get created secret: %w", err)
			}
			if err = s.auditRec.RecordSecret(ctx, nil, created, importAction, batchId); err != nil {
				return fmt.Errorf("auditRec.RecordSecret: %w", err)
			}
		}

		return applyImportPlan(result, plan, spec.Options.Overwrite,
			func(itemKey string) (string, bool) {
				item, ok := existingItems[itemKey]
				if !ok {
					return "", false
				}
				return item.Value, true
			},
			func(entry importKeyEntry) error {
				value, encoding := encodeImportValue(ksec.Data[entry.sourceKey])
				return s.importCreateItem(ctx, result.ObjectId, entry.itemKey, value, encoding, batchId)
			},
			func(entry importKeyEntry) error {
				value, encoding := encodeImportValue(ksec.Data[entry.sourceKey])
				return s.importUpdateItem(ctx, existingItems[entry.itemKey], value, encoding, batchId)
			},
		)
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// importCreateItem создаёт item по ключу источника и пишет запись аудита.
func (s *Service) importCreateItem(ctx context.Context, secretId, key, value, encoding string, batchId *string) error {
	itemId, err := s.itemSvc.Create(ctx, &itemModel.Edit{
		SecretId: new(secretId),
		Active:   new(true),
		Key:      new(key),
		Value:    new(value),
		Encoding: new(encoding),
	})
	if err != nil {
		return fmt.Errorf("key %q: create item: %w", key, err)
	}
	created, _, err := s.itemSvc.Get(ctx, itemId, true)
	if err != nil {
		return fmt.Errorf("key %q: get created item: %w", key, err)
	}
	if err = s.auditRec.RecordItem(ctx, nil, created, importAction, batchId); err != nil {
		return fmt.Errorf("auditRec.RecordItem: %w", err)
	}
	return nil
}

// importUpdateItem перезаписывает значение существующего item-а значением из
// кластера и пишет запись аудита.
func (s *Service) importUpdateItem(ctx context.Context, old *itemModel.Main, value, encoding string, batchId *string) error {
	err := s.itemSvc.Update(ctx, old.Id, &itemModel.Edit{
		Value:    new(value),
		Encoding: new(encoding),
	})
	if err != nil {
		return fmt.Errorf("key %q: update item: %w", old.Key, err)
	}
	updated, _, err := s.itemSvc.Get(ctx, old.Id, true)
	if err != nil {
		return fmt.Errorf("key %q: get updated item: %w", old.Key, err)
	}
	if err = s.auditRec.RecordItem(ctx, old, updated, importAction, batchId); err != nil {
		return fmt.Errorf("auditRec.RecordItem: %w", err)
	}
	return nil
}

func (s *Service) findSecretBySlug(ctx context.Context, appId, slug string) (*secretModel.Main, error) {
	secrets, _, err := s.secretSvc.List(ctx, &secretModel.ListReq{AppId: new(appId)})
	if err != nil {
		return nil, fmt.Errorf("secretSvc.List: %w", err)
	}
	for _, secret := range secrets {
		if secret.SlugName == slug {
			return secret, nil
		}
	}
	return nil, nil
}

// displaySecretType приводит тип k8s-секрета к виду базы: Opaque хранится как
// пустая строка.
func displaySecretType(t corev1.SecretType) string {
	if t == corev1.SecretTypeOpaque {
		return ""
	}
	return string(t)
}

func sortedDataKeys(data map[string][]byte) []string {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// encodeImportValue выбирает способ хранения значения: текст как есть
// (encoding=plain) либо base64 для бинарных данных (encoding=base64).
// Зеркально buildSecretData, которая раскодирует base64 обратно при sync.
func encodeImportValue(raw []byte) (value string, encoding string) {
	if utf8.Valid(raw) && !bytes.ContainsRune(raw, 0) {
		return string(raw), "plain"
	}
	return base64.StdEncoding.EncodeToString(raw), "base64"
}
