package monitoring

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	kubeService "github.com/rendau/kusec/internal/service/kube"
)

// Объект с совпавшим именем, записанный не для этой записи (секрет "main"
// app "airflow-dags" под именем kusec-airflow-dags-main у секрета
// "dags-main" app "airflow"), не-админу не сравнивается: иначе он видел бы
// чужие ключи и проверял догадку о значении по value_differs.
func TestDriftObject_ForeignObject(t *testing.T) {
	t.Parallel()

	desired := map[string][]byte{"PASSWORD": []byte("guess")}
	foreign := &kubeService.ClusterResource{
		Managed:       true,
		OwnerAppId:    "app-dags",
		OwnerRecordId: "sec-dags-main",
		Items: []kubeService.ClusterResourceItem{
			{Key: "PASSWORD", Value: "guess"},
			{Key: "OTHER", Value: "x"},
		},
	}
	now := time.Now()

	obj := driftObject("Secret", "kusec-airflow-dags-main", "airflow", "app-airflow", "sec-mine",
		true, &now, desired, now, foreign, true, false)
	assert.True(t, obj.ExistsInCluster)
	assert.True(t, obj.Foreign)
	assert.Empty(t, obj.MissingInCluster)
	assert.Empty(t, obj.ExtraInCluster)
	assert.Empty(t, obj.ValueDiffers)

	// админ видит сравнение и признак foreign
	obj = driftObject("Secret", "kusec-airflow-dags-main", "airflow", "app-airflow", "sec-mine",
		true, &now, desired, now, foreign, true, true)
	assert.True(t, obj.Foreign)
	assert.Equal(t, []string{"OTHER"}, obj.ExtraInCluster)

	// свой объект — обычное сравнение
	own := &kubeService.ClusterResource{
		Managed:       true,
		OwnerAppId:    "app-airflow",
		OwnerRecordId: "sec-mine",
		Items:         []kubeService.ClusterResourceItem{{Key: "PASSWORD", Value: "old"}},
	}
	obj = driftObject("Secret", "kusec-airflow-mine", "airflow", "app-airflow", "sec-mine",
		true, &now, desired, now, own, true, false)
	assert.False(t, obj.Foreign)
	assert.Equal(t, []string{"PASSWORD"}, obj.ValueDiffers)

	// объект вне kusec (без лейбла) — тоже чужой
	unmanaged := &kubeService.ClusterResource{Items: []kubeService.ClusterResourceItem{{Key: "K", Value: "v"}}}
	obj = driftObject("Secret", "pg-credentials", "airflow", "app-airflow", "sec-mine",
		true, &now, desired, now, unmanaged, true, false)
	assert.True(t, obj.Foreign)
	assert.Empty(t, obj.ExtraInCluster)

	// объекта нет — не чужой, все ключи отсутствуют
	obj = driftObject("Secret", "kusec-airflow-mine", "airflow", "app-airflow", "sec-mine",
		true, &now, desired, now, nil, false, false)
	assert.False(t, obj.Foreign)
	assert.Equal(t, []string{"PASSWORD"}, obj.MissingInCluster)
}
