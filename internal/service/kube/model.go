package kube

// SyncResult — итог синхронизации секретов в кластер.
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

// desiredSecret — желаемое состояние одного k8s-секрета, собранное из базы.
type desiredSecret struct {
	namespace string
	name      string
	appId     string
	secretId  string
	data      map[string][]byte
}
