package mcp

import (
	"context"
	"errors"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/samber/lo"

	kubeService "github.com/rendau/kusec/internal/service/kube"
)

// ── Регистрация ─────────────────────────────────────────

func (s *sessionServer) registerKubeTools(srv *mcpsdk.Server) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "sync",
		Description: "Синхронизировать секреты и конфигмапы из kusec в Kubernetes-кластер (создать/обновить/удалить управляемые k8s-объекты). Укажи app (id, slug_name или имя) либо all_apps=true — все доступные app. Работает, только когда kusec запущен внутри кластера.",
	}, s.sync)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_cluster_secret",
		Description: "Секреты, уже существующие в Kubernetes-кластере (для выбора источника import_secret): namespace, имя, тип, имена ключей и признак managed (уже под управлением kusec). Без системных namespace-ов kube-* и служебных типов (токены SA, helm-релизы). Значения не отдаются. Только для админа; вне кластера — in_cluster=false и пустой список.",
	}, s.listClusterSecret)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "import_secret",
		Description: "Перенести существующий секрет кластера в kusec: значения читаются из k8s и пишутся сразу в базу, тебе не показываются. Целевой секрет secret_slug в app создаётся или дозаполняется: новые ключи создаются, пустые item-ы заполняются; непустые по умолчанию не трогаются (skipped), с overwrite=true — перезаписываются. keys — подмножество и переименование (ключ в кластере → имя item-а), без keys — все ключи 1:1. Источник в кластере не меняется. Только для админа.",
	}, s.importSecret)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "import_secret_batch",
		Description: "Перенести несколько секретов кластера в один app за один вызов. secrets — список: у каждого name, при необходимости свои secret_slug (по умолчанию = name), keys и overwrite, иначе действуют общие namespace и overwrite пакета; семантика каждого — как у import_secret. Секреты импортируются по порядку, каждый в своей транзакции: ошибка одного (нет в кластере, лишний ключ в keys, невалидный slug) попадает в его error, остальные переносятся; повторный запуск идемпотентен. Значения не показываются. Только для админа.",
	}, s.importSecretBatch)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_cluster_configmap",
		Description: "Configmap-ы, уже существующие в Kubernetes-кластере (для выбора источника import_configmap): namespace, имя, имена ключей (data и binaryData) и признак managed (уже под управлением kusec). Без системных namespace-ов kube-* и служебного kube-root-ca.crt. Значения не отдаются. Только для админа; вне кластера — in_cluster=false и пустой список.",
	}, s.listClusterConfigMap)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "import_configmap",
		Description: "Перенести существующий configmap кластера в kusec: значения читаются из k8s и пишутся сразу в базу (data — текст, binaryData — base64). Целевой configmap configmap_slug в app создаётся или дозаполняется по тем же правилам, что import_secret: новые ключи создаются, пустые item-ы заполняются; непустые по умолчанию не трогаются (skipped), с overwrite=true — перезаписываются. keys — подмножество и переименование. Источник в кластере не меняется. Только для админа.",
	}, s.importConfigMap)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "import_configmap_batch",
		Description: "Перенести несколько configmap-ов кластера в один app за один вызов. configmaps — список: у каждого name, при необходимости свои configmap_slug (по умолчанию = name), keys и overwrite, иначе действуют общие namespace и overwrite пакета. Семантика и обработка ошибок — как у import_secret_batch: каждый configmap в своей транзакции, ошибка одного — в его error, остальные переносятся; повторный запуск идемпотентен. Только для админа.",
	}, s.importConfigMapBatch)
}

// ── Sync ────────────────────────────────────────────────

type SyncIn struct {
	App     string `json:"app,omitempty" jsonschema:"id, slug_name или имя app для синхронизации"`
	AllApps bool   `json:"all_apps,omitempty" jsonschema:"true — синхронизировать все доступные app"`
}

// SyncResultOut — итог синхронизации одного вида объектов; списки в формате "namespace/name".
type SyncResultOut struct {
	Created   []string `json:"created"`
	Updated   []string `json:"updated"`
	Deleted   []string `json:"deleted"`
	Unchanged int64    `json:"unchanged"`
	Errors    []string `json:"errors,omitempty" jsonschema:"ошибки по отдельным объектам: sync не прерывается, проблемные объекты пропускаются"`
}

type SyncOut struct {
	Secrets    SyncResultOut `json:"secrets"`
	Configmaps SyncResultOut `json:"configmaps"`
}

func (s *sessionServer) sync(ctx context.Context, req *mcpsdk.CallToolRequest, in SyncIn) (*mcpsdk.CallToolResult, SyncOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, SyncOut{}, err
	}

	var appId *string
	if !in.AllApps {
		if in.App == "" {
			return nil, SyncOut{}, errors.New("укажи app (id, slug_name или имя) либо all_apps=true")
		}
		app, rerr := s.resolveApp(ctx, in.App)
		if rerr != nil {
			return nil, SyncOut{}, s.toolErr(rerr)
		}
		appId = &app.Id
	}

	secrets, configMaps, err := s.h.kubeUsecase.Sync(ctx, appId)
	if err != nil {
		return nil, SyncOut{}, s.toolErr(err)
	}

	return nil, SyncOut{
		Secrets:    s.syncResultOut(secrets),
		Configmaps: s.syncResultOut(configMaps),
	}, nil
}

// syncResultOut конвертирует итог синхронизации, вычищая значения секретов из
// текстов ошибок по отдельным объектам.
func (s *sessionServer) syncResultOut(v *kubeService.SyncResult) SyncResultOut {
	if v == nil {
		return SyncResultOut{Created: []string{}, Updated: []string{}, Deleted: []string{}}
	}

	return SyncResultOut{
		Created:   v.Created,
		Updated:   v.Updated,
		Deleted:   v.Deleted,
		Unchanged: v.Unchanged,
		Errors:    lo.Map(v.Errors, func(e string, _ int) string { return s.vault.scrub(e) }),
	}
}

// ── Секреты кластера ────────────────────────────────────

type ListClusterSecretIn struct {
	Namespace string `json:"namespace,omitempty" jsonschema:"namespace для выборки; пусто — все namespace-ы без системных kube-*"`
}

// ClusterSecretOut — сводка о секрете кластера: только имена ключей, без значений.
type ClusterSecretOut struct {
	Namespace string   `json:"namespace"`
	Name      string   `json:"name"`
	Type      string   `json:"type" jsonschema:"тип k8s-секрета, пусто = Opaque"`
	Keys      []string `json:"keys" jsonschema:"имена ключей data (отсортированы), значения не отдаются"`
	Managed   bool     `json:"managed" jsonschema:"true — секрет уже под управлением kusec (лейбл managed-by=kusec)"`
}

type ListClusterSecretOut struct {
	InCluster bool               `json:"in_cluster" jsonschema:"false — kusec запущен вне кластера, список пуст"`
	Secrets   []ClusterSecretOut `json:"secrets"`
}

func (s *sessionServer) listClusterSecret(ctx context.Context, req *mcpsdk.CallToolRequest, in ListClusterSecretIn) (*mcpsdk.CallToolResult, ListClusterSecretOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, ListClusterSecretOut{}, err
	}

	secrets, inCluster, err := s.h.kubeUsecase.ListClusterSecrets(ctx, in.Namespace)
	if err != nil {
		return nil, ListClusterSecretOut{}, s.toolErr(err)
	}

	return nil, ListClusterSecretOut{
		InCluster: inCluster,
		Secrets: lo.Map(secrets, func(v *kubeService.ClusterSecret, _ int) ClusterSecretOut {
			return ClusterSecretOut{
				Namespace: v.Namespace,
				Name:      v.Name,
				Type:      v.Type,
				Keys:      emptyIfNil(v.Keys),
				Managed:   v.Managed,
			}
		}),
	}, nil
}

// ── Импорт секрета из кластера ──────────────────────────

type ImportSecretIn struct {
	App        string            `json:"app" jsonschema:"id, slug_name или имя app, в который импортировать"`
	Namespace  string            `json:"namespace" jsonschema:"namespace секрета-источника в кластере"`
	Name       string            `json:"name" jsonschema:"имя секрета-источника в кластере"`
	SecretSlug string            `json:"secret_slug" jsonschema:"slug целевого секрета kusec: новый (создаётся, kube_type копируется из источника) или существующий (дозаполняется)"`
	Keys       map[string]string `json:"keys,omitempty" jsonschema:"подмножество и переименование: ключ в кластере → имя item-а (пустое имя = оставить как в кластере); не задано — все ключи 1:1. Ключ, которого нет в источнике, — ошибка"`
	Overwrite  bool              `json:"overwrite,omitempty" jsonschema:"true — перезаписать значения совпавших непустых item-ов значениями из кластера; false (по умолчанию) — создать отсутствующие и заполнить пустые, непустые не трогать (skipped)"`
}

// ImportSecretOut — итог импорта: метаданные секрета и имена ключей по
// категориям. Значений здесь нет и быть не может.
type ImportSecretOut struct {
	SecretId       string   `json:"secret_id"`
	SecretSlug     string   `json:"secret_slug"`
	SecretCreated  bool     `json:"secret_created" jsonschema:"false — секрет уже существовал, выполнено дозаполнение"`
	KubeType       string   `json:"kube_type" jsonschema:"тип k8s-секрета целевой записи, пусто = Opaque"`
	KubeSecretName string   `json:"kube_secret_name" jsonschema:"имя k8s-секрета, под которым запись уйдёт в кластер при sync"`
	Created        []string `json:"created" jsonschema:"имена созданных item-ов (ключей не было)"`
	Filled         []string `json:"filled" jsonschema:"имена item-ов, существовавших с пустым значением и заполненных"`
	Updated        []string `json:"updated" jsonschema:"имена непустых item-ов, перезаписанных значением из кластера (overwrite=true)"`
	Skipped        []string `json:"skipped" jsonschema:"имена непустых item-ов, оставленных как есть (overwrite=false)"`
}

func (s *sessionServer) importSecret(ctx context.Context, req *mcpsdk.CallToolRequest, in ImportSecretIn) (*mcpsdk.CallToolResult, ImportSecretOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, ImportSecretOut{}, err
	}

	if in.App == "" {
		return nil, ImportSecretOut{}, errors.New("укажи app (id, slug_name или имя)")
	}
	if in.Namespace == "" || in.Name == "" {
		return nil, ImportSecretOut{}, errors.New("укажи namespace и name секрета-источника в кластере")
	}
	if in.SecretSlug == "" {
		return nil, ImportSecretOut{}, errors.New("укажи secret_slug целевого секрета")
	}

	app, err := s.resolveApp(ctx, in.App)
	if err != nil {
		return nil, ImportSecretOut{}, s.toolErr(err)
	}

	// Значения из кластера проходят service → БД и в эту сессию не попадают:
	// ни в реестр значений, ни в ответ. Ошибки сервиса содержат только имена
	// ключей.
	result, err := s.h.kubeUsecase.ImportSecret(ctx,
		app.Id,
		kubeService.ImportRef{Namespace: in.Namespace, Name: in.Name},
		in.SecretSlug,
		kubeService.ImportOptions{Keys: in.Keys, Overwrite: in.Overwrite},
	)
	if err != nil {
		return nil, ImportSecretOut{}, s.toolErr(err)
	}

	return nil, importResultOut(result), nil
}

// importResultOut конвертирует итог импорта; nil — пустой итог со стабильными
// пустыми списками (для секретов пакета, упавших с ошибкой).
func importResultOut(v *kubeService.ImportResult) ImportSecretOut {
	if v == nil {
		v = &kubeService.ImportResult{}
	}
	return ImportSecretOut{
		SecretId:       v.ObjectId,
		SecretSlug:     v.Slug,
		SecretCreated:  v.ObjectCreated,
		KubeType:       v.KubeType,
		KubeSecretName: v.KubeName,
		Created:        emptyIfNil(v.CreatedKeys),
		Filled:         emptyIfNil(v.FilledKeys),
		Updated:        emptyIfNil(v.UpdatedKeys),
		Skipped:        emptyIfNil(v.SkippedKeys),
	}
}

// ── Пакетный импорт ─────────────────────────────────────

type ImportSecretBatchIn struct {
	App       string                   `json:"app" jsonschema:"id, slug_name или имя app, в который импортировать все секреты пакета"`
	Namespace string                   `json:"namespace,omitempty" jsonschema:"namespace секретов-источников по умолчанию; у секрета может быть свой"`
	Overwrite bool                     `json:"overwrite,omitempty" jsonschema:"режим перезаписи по умолчанию для всех секретов пакета (семантика как в import_secret); у секрета может быть свой"`
	Secrets   []ImportSecretBatchEntry `json:"secrets" jsonschema:"секреты кластера для переноса; обрабатываются по порядку"`
}

type ImportSecretBatchEntry struct {
	Namespace  string            `json:"namespace,omitempty" jsonschema:"namespace секрета-источника; пусто — общий namespace пакета"`
	Name       string            `json:"name" jsonschema:"имя секрета-источника в кластере"`
	SecretSlug string            `json:"secret_slug,omitempty" jsonschema:"slug целевого секрета kusec; пусто — равен name"`
	Keys       map[string]string `json:"keys,omitempty" jsonschema:"как в import_secret: ключ в кластере → имя item-а (пустое имя = оставить), пусто — все ключи 1:1"`
	Overwrite  *bool             `json:"overwrite,omitempty" jsonschema:"переопределяет overwrite пакета для этого секрета"`
}

// ImportSecretBatchResult — итог по одному секрету пакета: источник, error
// (пусто = успех) и поля import_secret. У упавшего секрета заполнены только
// namespace, name, secret_slug и error, списки ключей пустые.
type ImportSecretBatchResult struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Error     string `json:"error" jsonschema:"пусто — секрет импортирован; иначе причина (остальные секреты пакета не затронуты)"`
	ImportSecretOut
}

type ImportSecretBatchOut struct {
	Ok      int                       `json:"ok" jsonschema:"сколько секретов импортировано"`
	Failed  int                       `json:"failed" jsonschema:"сколько секретов завершилось ошибкой (см. results[].error)"`
	Results []ImportSecretBatchResult `json:"results" jsonschema:"по одному на каждый секрет пакета, в порядке запроса"`
}

func (s *sessionServer) importSecretBatch(ctx context.Context, req *mcpsdk.CallToolRequest, in ImportSecretBatchIn) (*mcpsdk.CallToolResult, ImportSecretBatchOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, ImportSecretBatchOut{}, err
	}

	if in.App == "" {
		return nil, ImportSecretBatchOut{}, errors.New("укажи app (id, slug_name или имя)")
	}

	app, err := s.resolveApp(ctx, in.App)
	if err != nil {
		return nil, ImportSecretBatchOut{}, s.toolErr(err)
	}

	// Дефолты пакета: namespace, overwrite и secret_slug = name.
	specs := lo.Map(in.Secrets, func(e ImportSecretBatchEntry, _ int) kubeService.ImportSpec {
		return batchImportSpec(e.Namespace, e.Name, e.SecretSlug, e.Keys, e.Overwrite, in.Namespace, in.Overwrite)
	})

	items, err := s.h.kubeUsecase.ImportSecrets(ctx, app.Id, specs)
	if err != nil {
		return nil, ImportSecretBatchOut{}, s.toolErr(err)
	}

	out := ImportSecretBatchOut{}
	out.Ok, out.Failed, out.Results = batchImportResults(s, items, func(item kubeService.ImportBatchItem, errText string) ImportSecretBatchResult {
		res := ImportSecretBatchResult{
			Namespace:       item.Ref.Namespace,
			Name:            item.Ref.Name,
			Error:           errText,
			ImportSecretOut: importResultOut(item.Result),
		}
		if errText != "" {
			res.SecretSlug = item.Slug
		}
		return res
	})

	return nil, out, nil
}

// batchImportSpec — спека одного объекта пакета с дефолтами пакета:
// namespace и overwrite — общие, slug посадочной записи = name.
func batchImportSpec(namespace, name, slug string, keys map[string]string, overwrite *bool, defNamespace string, defOverwrite bool) kubeService.ImportSpec {
	return kubeService.ImportSpec{
		Ref: kubeService.ImportRef{
			Namespace: lo.CoalesceOrEmpty(namespace, defNamespace),
			Name:      name,
		},
		Slug: lo.CoalesceOrEmpty(slug, name),
		Options: kubeService.ImportOptions{
			Keys:      keys,
			Overwrite: lo.FromPtrOr(overwrite, defOverwrite),
		},
	}
}

// batchImportResults раскладывает итоги пакета в элементы ответа и считает
// успешные/упавшие. toResult получает итог и текст ошибки ("" — успех); текст
// проходит скраб значений секретов.
func batchImportResults[R any](s *sessionServer, items []kubeService.ImportBatchItem, toResult func(item kubeService.ImportBatchItem, errText string) R) (ok, failed int, results []R) {
	results = make([]R, 0, len(items))
	for _, item := range items {
		errText := ""
		if item.Err != nil {
			errText = s.toolErr(item.Err).Error()
			failed++
		} else {
			ok++
		}
		results = append(results, toResult(item, errText))
	}
	return ok, failed, results
}

// ── Configmap-ы кластера ────────────────────────────────

type ListClusterConfigMapIn struct {
	Namespace string `json:"namespace,omitempty" jsonschema:"namespace для выборки; пусто — все namespace-ы без системных kube-*"`
}

// ClusterConfigMapOut — сводка о configmap-е кластера: только имена ключей.
type ClusterConfigMapOut struct {
	Namespace string   `json:"namespace"`
	Name      string   `json:"name"`
	Keys      []string `json:"keys" jsonschema:"имена ключей data и binaryData (отсортированы), значения не отдаются"`
	Managed   bool     `json:"managed" jsonschema:"true — configmap уже под управлением kusec (лейбл managed-by=kusec)"`
}

type ListClusterConfigMapOut struct {
	InCluster  bool                  `json:"in_cluster" jsonschema:"false — kusec запущен вне кластера, список пуст"`
	Configmaps []ClusterConfigMapOut `json:"configmaps"`
}

func (s *sessionServer) listClusterConfigMap(ctx context.Context, req *mcpsdk.CallToolRequest, in ListClusterConfigMapIn) (*mcpsdk.CallToolResult, ListClusterConfigMapOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, ListClusterConfigMapOut{}, err
	}

	configMaps, inCluster, err := s.h.kubeUsecase.ListClusterConfigMaps(ctx, in.Namespace)
	if err != nil {
		return nil, ListClusterConfigMapOut{}, s.toolErr(err)
	}

	return nil, ListClusterConfigMapOut{
		InCluster: inCluster,
		Configmaps: lo.Map(configMaps, func(v *kubeService.ClusterConfigMap, _ int) ClusterConfigMapOut {
			return ClusterConfigMapOut{
				Namespace: v.Namespace,
				Name:      v.Name,
				Keys:      emptyIfNil(v.Keys),
				Managed:   v.Managed,
			}
		}),
	}, nil
}

// ── Импорт configmap-а из кластера ──────────────────────

type ImportConfigMapIn struct {
	App           string            `json:"app" jsonschema:"id, slug_name или имя app, в который импортировать"`
	Namespace     string            `json:"namespace" jsonschema:"namespace configmap-а-источника в кластере"`
	Name          string            `json:"name" jsonschema:"имя configmap-а-источника в кластере"`
	ConfigmapSlug string            `json:"configmap_slug" jsonschema:"slug целевого configmap-а kusec: новый (создаётся) или существующий (дозаполняется)"`
	Keys          map[string]string `json:"keys,omitempty" jsonschema:"подмножество и переименование: ключ в кластере → имя item-а (пустое имя = оставить как в кластере); не задано — все ключи 1:1. Ключ, которого нет в источнике, — ошибка"`
	Overwrite     bool              `json:"overwrite,omitempty" jsonschema:"true — перезаписать значения совпавших непустых item-ов значениями из кластера; false (по умолчанию) — создать отсутствующие и заполнить пустые, непустые не трогать (skipped)"`
}

// ImportConfigMapOut — итог импорта configmap-а: метаданные и имена ключей
// по категориям.
type ImportConfigMapOut struct {
	ConfigmapId       string   `json:"configmap_id"`
	ConfigmapSlug     string   `json:"configmap_slug"`
	ConfigmapCreated  bool     `json:"configmap_created" jsonschema:"false — configmap уже существовал, выполнено дозаполнение"`
	KubeConfigmapName string   `json:"kube_configmap_name" jsonschema:"имя k8s-configmap, под которым запись уйдёт в кластер при sync"`
	Created           []string `json:"created" jsonschema:"имена созданных item-ов (ключей не было)"`
	Filled            []string `json:"filled" jsonschema:"имена item-ов, существовавших с пустым значением и заполненных"`
	Updated           []string `json:"updated" jsonschema:"имена непустых item-ов, перезаписанных значением из кластера (overwrite=true)"`
	Skipped           []string `json:"skipped" jsonschema:"имена непустых item-ов, оставленных как есть (overwrite=false)"`
}

// importConfigMapResultOut конвертирует итог импорта configmap-а; nil — пустой
// итог со стабильными пустыми списками.
func importConfigMapResultOut(v *kubeService.ImportResult) ImportConfigMapOut {
	if v == nil {
		v = &kubeService.ImportResult{}
	}
	return ImportConfigMapOut{
		ConfigmapId:       v.ObjectId,
		ConfigmapSlug:     v.Slug,
		ConfigmapCreated:  v.ObjectCreated,
		KubeConfigmapName: v.KubeName,
		Created:           emptyIfNil(v.CreatedKeys),
		Filled:            emptyIfNil(v.FilledKeys),
		Updated:           emptyIfNil(v.UpdatedKeys),
		Skipped:           emptyIfNil(v.SkippedKeys),
	}
}

func (s *sessionServer) importConfigMap(ctx context.Context, req *mcpsdk.CallToolRequest, in ImportConfigMapIn) (*mcpsdk.CallToolResult, ImportConfigMapOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, ImportConfigMapOut{}, err
	}

	if in.App == "" {
		return nil, ImportConfigMapOut{}, errors.New("укажи app (id, slug_name или имя)")
	}
	if in.Namespace == "" || in.Name == "" {
		return nil, ImportConfigMapOut{}, errors.New("укажи namespace и name configmap-а-источника в кластере")
	}
	if in.ConfigmapSlug == "" {
		return nil, ImportConfigMapOut{}, errors.New("укажи configmap_slug целевого configmap-а")
	}

	app, err := s.resolveApp(ctx, in.App)
	if err != nil {
		return nil, ImportConfigMapOut{}, s.toolErr(err)
	}

	result, err := s.h.kubeUsecase.ImportConfigMap(ctx,
		app.Id,
		kubeService.ImportRef{Namespace: in.Namespace, Name: in.Name},
		in.ConfigmapSlug,
		kubeService.ImportOptions{Keys: in.Keys, Overwrite: in.Overwrite},
	)
	if err != nil {
		return nil, ImportConfigMapOut{}, s.toolErr(err)
	}

	return nil, importConfigMapResultOut(result), nil
}

// ── Пакетный импорт configmap-ов ────────────────────────

type ImportConfigMapBatchIn struct {
	App        string                      `json:"app" jsonschema:"id, slug_name или имя app, в который импортировать все configmap-ы пакета"`
	Namespace  string                      `json:"namespace,omitempty" jsonschema:"namespace configmap-ов-источников по умолчанию; у configmap-а может быть свой"`
	Overwrite  bool                        `json:"overwrite,omitempty" jsonschema:"режим перезаписи по умолчанию для всех configmap-ов пакета (семантика как в import_configmap); у configmap-а может быть свой"`
	Configmaps []ImportConfigMapBatchEntry `json:"configmaps" jsonschema:"configmap-ы кластера для переноса; обрабатываются по порядку"`
}

type ImportConfigMapBatchEntry struct {
	Namespace     string            `json:"namespace,omitempty" jsonschema:"namespace configmap-а-источника; пусто — общий namespace пакета"`
	Name          string            `json:"name" jsonschema:"имя configmap-а-источника в кластере"`
	ConfigmapSlug string            `json:"configmap_slug,omitempty" jsonschema:"slug целевого configmap-а kusec; пусто — равен name"`
	Keys          map[string]string `json:"keys,omitempty" jsonschema:"как в import_configmap: ключ в кластере → имя item-а (пустое имя = оставить), пусто — все ключи 1:1"`
	Overwrite     *bool             `json:"overwrite,omitempty" jsonschema:"переопределяет overwrite пакета для этого configmap-а"`
}

// ImportConfigMapBatchResult — итог по одному configmap-у пакета: источник,
// error (пусто = успех) и поля import_configmap. У упавшего заполнены только
// namespace, name, configmap_slug и error, списки ключей пустые.
type ImportConfigMapBatchResult struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Error     string `json:"error" jsonschema:"пусто — configmap импортирован; иначе причина (остальные configmap-ы пакета не затронуты)"`
	ImportConfigMapOut
}

type ImportConfigMapBatchOut struct {
	Ok      int                          `json:"ok" jsonschema:"сколько configmap-ов импортировано"`
	Failed  int                          `json:"failed" jsonschema:"сколько configmap-ов завершилось ошибкой (см. results[].error)"`
	Results []ImportConfigMapBatchResult `json:"results" jsonschema:"по одному на каждый configmap пакета, в порядке запроса"`
}

func (s *sessionServer) importConfigMapBatch(ctx context.Context, req *mcpsdk.CallToolRequest, in ImportConfigMapBatchIn) (*mcpsdk.CallToolResult, ImportConfigMapBatchOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, ImportConfigMapBatchOut{}, err
	}

	if in.App == "" {
		return nil, ImportConfigMapBatchOut{}, errors.New("укажи app (id, slug_name или имя)")
	}

	app, err := s.resolveApp(ctx, in.App)
	if err != nil {
		return nil, ImportConfigMapBatchOut{}, s.toolErr(err)
	}

	// Дефолты пакета: namespace, overwrite и configmap_slug = name.
	specs := lo.Map(in.Configmaps, func(e ImportConfigMapBatchEntry, _ int) kubeService.ImportSpec {
		return batchImportSpec(e.Namespace, e.Name, e.ConfigmapSlug, e.Keys, e.Overwrite, in.Namespace, in.Overwrite)
	})

	items, err := s.h.kubeUsecase.ImportConfigMaps(ctx, app.Id, specs)
	if err != nil {
		return nil, ImportConfigMapBatchOut{}, s.toolErr(err)
	}

	out := ImportConfigMapBatchOut{}
	out.Ok, out.Failed, out.Results = batchImportResults(s, items, func(item kubeService.ImportBatchItem, errText string) ImportConfigMapBatchResult {
		res := ImportConfigMapBatchResult{
			Namespace:          item.Ref.Namespace,
			Name:               item.Ref.Name,
			Error:              errText,
			ImportConfigMapOut: importConfigMapResultOut(item.Result),
		}
		if errText != "" {
			res.ConfigmapSlug = item.Slug
		}
		return res
	})

	return nil, out, nil
}

// emptyIfNil — стабильный JSON: пустой список вместо null.
func emptyIfNil(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}
