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

// ImportResult — итог импорта одного секрета.
type ImportResult struct {
	// Запись secret (созданная или дозаполненная).
	SecretId   string
	SecretSlug string
	// false — секрет уже существовал, выполнено дозаполнение.
	SecretCreated bool
	// Сколько item-ов создано (новые ключи).
	CreatedItems int64
	// Сколько item-ов обновлено (совпавшие ключи — значение перезаписано).
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
