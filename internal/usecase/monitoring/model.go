package monitoring

import (
	"time"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
)

// ResolveResult — app и его secret/configmap, найденные по имени k8s-объекта.
type ResolveResult struct {
	App *appModel.Main
	// Secret | ConfigMap
	KubeKind string
	// id записи kusec (secret или configmap)
	ObjectId   string
	ObjectSlug string
}

// AppKey — один ключ app без значения: только размер и HMAC-отпечаток.
type AppKey struct {
	Key      string
	KubeKind string
	KubeName string
	IsSecret bool
	Active   bool

	ValueSize int64
	ValueHash string

	UpdatedAt time.Time
	// последний актор из аудита; пусто — изменений после включения аудита
	// не было
	UpdatedBy string
	// ключ применён в кластер: updated_at не позже последнего sync объекта
	Synced bool

	ItemId   string
	ParentId string
}

// DriftObject — расхождения kusec ↔ кластер одного k8s-объекта
// (только имена ключей, без значений).
type DriftObject struct {
	KubeKind  string
	KubeName  string
	Namespace string
	ObjectId  string

	ExistsInCluster bool
	Managed         bool

	MissingInCluster []string
	ExtraInCluster   []string
	ValueDiffers     []string

	// время последнего изменения в kusec, ещё не применённого в кластер
	NotSyncedSince *time.Time
}
