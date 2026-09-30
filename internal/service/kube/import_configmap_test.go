package kube

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
	configitemModel "github.com/rendau/kusec/internal/domain/configitem/model"
	configmapModel "github.com/rendau/kusec/internal/domain/configmap/model"
)

func TestListClusterConfigMaps_FiltersAndSorts(t *testing.T) {
	t.Parallel()

	client := k8sfake.NewSimpleClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "b-cm", Namespace: "team-a"},
			Data:       map[string]string{"Z": "1", "A": "2"},
			BinaryData: map[string][]byte{"M": {0x00}},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "a-cm",
				Namespace: "team-a",
				Labels:    map[string]string{managedByLabelKey: managedByLabelValue},
			},
			Data: map[string]string{"HOST": "pg"},
		},
		// Служебный configmap k8s — отфильтровывается.
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "kube-root-ca.crt", Namespace: "team-a"}},
		// Системный namespace — отфильтровывается.
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "sys", Namespace: "kube-system"}},
	)
	svc := &Service{client: client}

	got, inCluster, err := svc.ListClusterConfigMaps(context.Background(), "")
	require.NoError(t, err)
	assert.True(t, inCluster)
	require.Len(t, got, 2)

	// Отсортированы по namespace, затем по имени; ключи data и binaryData слиты.
	assert.Equal(t, "a-cm", got[0].Name)
	assert.True(t, got[0].Managed)
	assert.Equal(t, "b-cm", got[1].Name)
	assert.False(t, got[1].Managed)
	assert.Equal(t, []string{"A", "M", "Z"}, got[1].Keys)
}

// configMapImportFixture — configmap-источник с тремя ключами (NEW и FULL —
// текст, EMPTY — бинарный) и посадочный configmap "cfg", в котором NEW
// отсутствует, EMPTY существует с пустым значением, FULL — с непустым.
type configMapImportFixture struct {
	svc     *Service
	spy     *auditRecSpy
	created map[string][2]string // item key -> {value, encoding}
	updated map[string][2]string // item id -> {value, encoding}
}

func newConfigMapImportFixture(t *testing.T, existing bool) *configMapImportFixture {
	t.Helper()

	client := k8sfake.NewSimpleClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "app-config", Namespace: "team-a"},
			Data:       map[string]string{"NEW": "new-value", "FULL": "cluster-value"},
			BinaryData: map[string][]byte{"EMPTY": {0x00, 0xff, 0x10}},
		},
	)

	fx := &configMapImportFixture{
		spy:     &auditRecSpy{},
		created: map[string][2]string{},
		updated: map[string][2]string{},
	}
	fx.svc = &Service{
		client:   client,
		txm:      txmStub{},
		auditRec: fx.spy,
		appSvc: appSvcStub{
			getFn: func(_ context.Context, _ string, _ bool) (*appModel.Main, bool, error) {
				return &appModel.Main{Id: "app-1", Namespace: "web", SlugName: "web"}, true, nil
			},
		},
		configMapSvc: configMapSvcStub{
			listFn: func(_ context.Context, req *configmapModel.ListReq) ([]*configmapModel.Main, int64, error) {
				require.Equal(t, "app-1", *req.AppId)
				if !existing {
					return nil, 0, nil
				}
				return []*configmapModel.Main{{Id: "cm-1", SlugName: "cfg"}}, 1, nil
			},
			createFn: func(_ context.Context, obj *configmapModel.Edit) (string, error) {
				require.False(t, existing, "Create must not be called for an existing configmap")
				require.Equal(t, "app-1", *obj.AppId)
				return "cm-" + *obj.SlugName, nil
			},
		},
		configItemSvc: configItemSvcStub{
			listFn: func(_ context.Context, _ *configitemModel.ListReq) ([]*configitemModel.Main, int64, error) {
				return []*configitemModel.Main{
					{Id: "i-empty", Key: "EMPTY", Value: ""},
					{Id: "i-full", Key: "FULL", Value: "old-value"},
				}, 2, nil
			},
			createFn: func(_ context.Context, obj *configitemModel.Edit) (string, error) {
				fx.created[*obj.Key] = [2]string{*obj.Value, *obj.Encoding}
				return "item-" + *obj.Key, nil
			},
			updateFn: func(_ context.Context, id string, obj *configitemModel.Edit) error {
				fx.updated[id] = [2]string{*obj.Value, *obj.Encoding}
				return nil
			},
		},
	}
	return fx
}

func TestImportConfigMap_CreatesAndEncodes(t *testing.T) {
	t.Parallel()

	fx := newConfigMapImportFixture(t, false)

	result, err := fx.svc.ImportConfigMap(context.Background(), "app-1",
		ImportRef{Namespace: "team-a", Name: "app-config"}, "cfg", ImportOptions{})
	require.NoError(t, err)

	assert.Equal(t, "cm-cfg", result.ObjectId)
	assert.Equal(t, "cfg", result.Slug)
	assert.True(t, result.ObjectCreated)
	assert.Empty(t, result.KubeType)
	assert.Equal(t, ConfigMapName("web", "cfg", false), result.KubeName)
	assert.Equal(t, []string{"EMPTY", "FULL", "NEW"}, result.CreatedKeys)
	assert.EqualValues(t, 3, result.CreatedItems)

	// data — текст как есть, binaryData — base64
	assert.Equal(t, map[string][2]string{
		"NEW":   {"new-value", "plain"},
		"FULL":  {"cluster-value", "plain"},
		"EMPTY": {"AP8Q", "base64"},
	}, fx.created)

	// аудит: configmap + 3 item-а под одним batch_id
	assert.Len(t, fx.spy.recorded, 4)
	assert.Len(t, lo.Uniq(fx.spy.recorded), 1)
}

func TestImportConfigMap_NoOverwriteAndOverwrite(t *testing.T) {
	t.Parallel()

	// overwrite=false: новый создан, пустой заполнен, непустой не тронут
	fx := newConfigMapImportFixture(t, true)
	result, err := fx.svc.ImportConfigMap(context.Background(), "app-1",
		ImportRef{Namespace: "team-a", Name: "app-config"}, "cfg", ImportOptions{})
	require.NoError(t, err)

	assert.Equal(t, "cm-1", result.ObjectId)
	assert.False(t, result.ObjectCreated)
	assert.Equal(t, []string{"NEW"}, result.CreatedKeys)
	assert.Equal(t, []string{"EMPTY"}, result.FilledKeys)
	assert.Empty(t, result.UpdatedKeys)
	assert.Equal(t, []string{"FULL"}, result.SkippedKeys)
	assert.Equal(t, map[string][2]string{"NEW": {"new-value", "plain"}}, fx.created)
	assert.Equal(t, map[string][2]string{"i-empty": {"AP8Q", "base64"}}, fx.updated)

	// overwrite=true: непустой перезаписан значением из кластера
	fx = newConfigMapImportFixture(t, true)
	result, err = fx.svc.ImportConfigMap(context.Background(), "app-1",
		ImportRef{Namespace: "team-a", Name: "app-config"}, "cfg", ImportOptions{Overwrite: true})
	require.NoError(t, err)

	assert.Equal(t, []string{"EMPTY"}, result.FilledKeys)
	assert.Equal(t, []string{"FULL"}, result.UpdatedKeys)
	assert.Empty(t, result.SkippedKeys)
	assert.EqualValues(t, 2, result.UpdatedItems)
	assert.Equal(t, [2]string{"cluster-value", "plain"}, fx.updated["i-full"])
}

func TestImportConfigMaps_Batch_ContinuesOnError(t *testing.T) {
	t.Parallel()

	fx := newConfigMapImportFixture(t, false)

	items, err := fx.svc.ImportConfigMaps(context.Background(), "app-1", []ImportSpec{
		{Ref: ImportRef{Namespace: "team-a", Name: "missing"}, Slug: "a"},
		{Ref: ImportRef{Namespace: "team-a", Name: "app-config"}, Slug: "Bad_Slug"},
		{Ref: ImportRef{Namespace: "team-a", Name: "app-config"}, Slug: "cfg", Options: ImportOptions{Keys: map[string]string{"NEW": "RENAMED"}}},
	})
	require.NoError(t, err)
	require.Len(t, items, 3)

	require.Error(t, items[0].Err)
	assert.Contains(t, items[0].Err.Error(), "team-a/missing")
	assert.Nil(t, items[0].Result)

	require.Error(t, items[1].Err)
	assert.Contains(t, items[1].Err.Error(), "invalid configmap slug")

	require.NoError(t, items[2].Err)
	assert.Equal(t, []string{"RENAMED"}, items[2].Result.CreatedKeys)
	assert.Equal(t, map[string][2]string{"RENAMED": {"new-value", "plain"}}, fx.created)

	// один batch_id на пакет
	assert.Equal(t, 1, fx.spy.newBatchIds)
}
