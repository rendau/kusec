package kube

type SyncResult struct {
	// Списки в формате "namespace/name".
	Created []string
	Updated []string
	Deleted []string

	Unchanged int64

	// Ошибки по отдельным секретам: sync не прерывается, проблемные
	// секреты пропускаются и попадают сюда.
	Errors []string
}

// ClusterSecret — сводка о секрете кластера для выбора при импорте.
type ClusterSecret struct {
	Namespace string
	Name      string
	// Тип k8s-секрета (пусто = Opaque).
	Type string
	// Ключи data (отсортированы), значения не отдаются.
	Keys []string
	// true — секрет уже под управлением kusec (лейбл managed-by=kusec).
	Managed bool
}

// ClusterConfigMap — сводка о configmap-е кластера для выбора при импорте.
type ClusterConfigMap struct {
	Namespace string
	Name      string
	// Ключи data и binaryData (отсортированы), значения не отдаются.
	Keys []string
	// true — configmap уже под управлением kusec (лейбл managed-by=kusec).
	Managed bool
}

// ClusterResource — живой k8s-объект (secret или configmap) из кластера,
// прочитанный для сверки с записью kusec.
type ClusterResource struct {
	Namespace string
	Name      string
	// Тип k8s-секрета (для secret; пусто для configmap и для Opaque).
	Type string
	// true — объект под управлением kusec (лейбл managed-by=kusec).
	Managed bool
	// Ключи data (отсортированы) со значениями.
	Items []ClusterResourceItem
}

// ClusterResourceItem — пара ключ/значение из живого k8s-объекта. Значение —
// текст (encoding=plain) либо base64 для бинарных (encoding=base64),
// зеркально encodeImportValue.
type ClusterResourceItem struct {
	Key      string
	Value    string
	Encoding string
}

// ImportRef — ссылка на импортируемый секрет кластера.
type ImportRef struct {
	Namespace string
	Name      string
}

// ImportOptions — параметры импорта. Нулевое значение — консервативный
// режим: все ключи источника 1:1, существующие непустые значения не трогаются.
type ImportOptions struct {
	// Keys — какие ключи источника переносить и под какими именами item-ов:
	// ключ в кластере → имя item-а (пустое имя = оставить как в кластере).
	// Пусто — все ключи источника под своими именами. Ключ, которого нет в
	// источнике, — ошибка.
	Keys map[string]string
	// Overwrite — перезаписывать значения совпавших непустых item-ов
	// значением из кластера. false — заполняются только пустые item-ы,
	// непустые попадают в SkippedKeys. REST-импорт передаёт true (перезапись
	// совпавших — поведение кнопки «Import from cluster»).
	Overwrite bool
}

// ImportSpec — один объект кластера (секрет или configmap) в импорте.
type ImportSpec struct {
	Ref ImportRef
	// Slug посадочной записи kusec (secret или configmap).
	Slug    string
	Options ImportOptions
}

// ImportBatchItem — итог по одному объекту пакета: Result при успехе, Err при
// ошибке (валидация или сбой импорта этого объекта). Остальные объекты пакета
// на него не влияют.
type ImportBatchItem struct {
	Ref    ImportRef
	Slug   string
	Result *ImportResult
	Err    error
}

// ImportResult — итог импорта одного объекта кластера (секрета или
// configmap-а). Содержит только имена ключей и метаданные: значения из
// кластера в результат не попадают.
type ImportResult struct {
	// Запись kusec (secret или configmap): созданная или дозаполненная.
	ObjectId string
	Slug     string
	// false — запись уже существовала, выполнено дозаполнение.
	ObjectCreated bool
	// Тип k8s-секрета целевой записи (пусто = Opaque): у созданного —
	// скопирован из источника, у существующего — прежний. Для configmap пусто.
	KubeType string
	// Имя k8s-объекта, под которым запись уйдёт в кластер при sync.
	KubeName string

	// Имена item-ов по итогу (отсортированы, без значений):
	// CreatedKeys — созданы (ключей не было);
	// FilledKeys  — существовали с пустым значением, заполнены;
	// UpdatedKeys — существовали с непустым значением, перезаписаны (Overwrite);
	// SkippedKeys — существовали с непустым значением, не тронуты (!Overwrite).
	CreatedKeys []string
	FilledKeys  []string
	UpdatedKeys []string
	SkippedKeys []string

	// Счётчики для REST-ответа: CreatedItems = len(CreatedKeys),
	// UpdatedItems = len(FilledKeys) + len(UpdatedKeys).
	CreatedItems int64
	UpdatedItems int64
}

type desiredSecret struct {
	namespace string
	name      string
	appId     string
	secretId  string
	// Тип k8s-секрета (пусто = Opaque).
	kubeType string
	data     map[string][]byte
}

type desiredConfigMap struct {
	namespace   string
	name        string
	appId       string
	configMapId string
	// data — текстовые значения (encoding=plain), binaryData — бинарные
	// (encoding=base64, декодированы в байты).
	data       map[string]string
	binaryData map[string][]byte
}
