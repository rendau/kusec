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

// ImportRef — ссылка на импортируемый секрет кластера.
type ImportRef struct {
	Namespace string
	Name      string
}

type ImportResult struct {
	// Импортированные секреты в формате "namespace/name".
	Imported []string
	// Пропущены — kusec-секрет с таким slug уже существует.
	Skipped []string
	// Ошибки по отдельным секретам: импорт не прерывается, проблемные
	// секреты пропускаются и попадают сюда.
	Errors []string

	CreatedSecrets int64
	CreatedItems   int64
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
