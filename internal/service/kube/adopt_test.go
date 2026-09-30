package kube

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
	configitemModel "github.com/rendau/kusec/internal/domain/configitem/model"
	configmapModel "github.com/rendau/kusec/internal/domain/configmap/model"
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
)

func TestAdoptionRefusal(t *testing.T) {
	t.Parallel()

	b := func(s string) []byte { return []byte(s) }
	managed := map[string]string{managedByLabelKey: managedByLabelValue}
	helm := map[string]string{managedByLabelKey: "Helm"}

	// данные совпадают — усыновление без потерь
	assert.Empty(t, adoptionRefusal(nil, map[string][]byte{"A": b("v")}, map[string][]byte{"A": b("v")}))

	// изменённое непустое значение, пустое с обеих сторон и новый пустой ключ — не потеря
	assert.Empty(t, adoptionRefusal(helm,
		map[string][]byte{"A": b("old"), "E": b("")},
		map[string][]byte{"A": b("new"), "E": b(""), "NEW": b("")}))

	// ключа нет в kusec и непустое значение затёрлось бы пустым — отказ с именами ключей
	reason := adoptionRefusal(helm,
		map[string][]byte{"A": b("live-a"), "B": b("live-b"), "Z": b("live-z"), "OK": b("same")},
		map[string][]byte{"A": b(""), "OK": b("same")})
	assert.Contains(t, reason, "keys missing in kusec: B, Z")
	assert.Contains(t, reason, "keys with empty value in kusec: A")
	for _, value := range []string{"live-a", "live-b", "live-z", "same"} {
		assert.NotContains(t, reason, value, "значение утекло в причину отказа")
	}

	// объект уже наш — проверка не применяется
	assert.Empty(t, adoptionRefusal(managed, map[string][]byte{"GONE": b("v")}, map[string][]byte{}))
}

// adoptSecretSvc — сервис с одним приложением web (namespace team-a) и одним
// секретом billing-db с exact_slug: имя k8s-секрета совпадает с живым объектом.
func adoptSecretSvc(client kubernetes.Interface, items []*itemModel.Main) *Service {
	return &Service{
		client:     client,
		txm:        txmStub{},
		auditRec:   auditRecStub{},
		syncRunSvc: syncRunSvcStub{},
		appSvc: appSvcStub{
			listFn: func(_ context.Context, _ *appModel.ListReq) ([]*appModel.Main, int64, error) {
				return []*appModel.Main{{Id: "app-1", Namespace: "team-a", SlugName: "web"}}, 1, nil
			},
		},
		secretSvc: secretSvcStub{
			listFn: func(_ context.Context, _ *secretModel.ListReq) ([]*secretModel.Main, int64, error) {
				return []*secretModel.Main{{Id: "sec-1", SlugName: "billing-db", ExactSlug: true}}, 1, nil
			},
		},
		itemSvc: itemSvcStub{
			listFn: func(_ context.Context, _ *itemModel.ListReq) ([]*itemModel.Main, int64, error) {
				return items, int64(len(items)), nil
			},
		},
	}
}

func TestSyncSecrets_AdoptRefusedWhenLiveDataWouldBeLost(t *testing.T) {
	t.Parallel()

	liveData := map[string][]byte{"A": []byte("live-a"), "B": []byte("live-b"), "C": []byte("live-c")}
	client := k8sfake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "billing-db",
				Namespace: "team-a",
				Labels:    map[string]string{managedByLabelKey: "Helm"},
			},
			Type: corev1.SecretTypeOpaque,
			Data: liveData,
		},
	)
	// в kusec: A заведён пустым, B не заведён вовсе, C с новым значением
	svc := adoptSecretSvc(client, []*itemModel.Main{
		{Key: "A", Value: ""},
		{Key: "C", Value: "new-c"},
	})

	result, err := svc.SyncSecrets(context.Background(), nil)
	require.NoError(t, err)

	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "team-a/billing-db: adopt: refused")
	assert.Contains(t, result.Errors[0], "keys missing in kusec: B")
	assert.Contains(t, result.Errors[0], "keys with empty value in kusec: A")
	for _, value := range []string{"live-a", "live-b", "live-c", "new-c"} {
		assert.NotContains(t, result.Errors[0], value, "значение утекло в ошибку sync")
	}
	assert.Empty(t, result.Created)
	assert.Empty(t, result.Updated)

	// живой объект не тронут: данные и владелец прежние
	live, err := client.CoreV1().Secrets("team-a").Get(context.Background(), "billing-db", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, liveData, live.Data)
	assert.Equal(t, "Helm", live.Labels[managedByLabelKey])
	assert.Empty(t, live.Annotations[secretIdAnnotation])
}

func TestSyncSecrets_AdoptAllowedWhenNoLiveDataIsLost(t *testing.T) {
	t.Parallel()

	client := k8sfake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "billing-db", Namespace: "team-a"},
			Type:       corev1.SecretTypeOpaque,
			Data:       map[string][]byte{"A": []byte("old"), "E": []byte("")},
		},
	)
	// изменённое значение, пустое с обеих сторон, новый ключ — потерь нет
	svc := adoptSecretSvc(client, []*itemModel.Main{
		{Key: "A", Value: "new"},
		{Key: "E", Value: ""},
		{Key: "NEW", Value: "added"},
	})

	result, err := svc.SyncSecrets(context.Background(), nil)
	require.NoError(t, err)

	assert.Empty(t, result.Errors)
	assert.Equal(t, []string{"team-a/billing-db"}, result.Updated)

	live, err := client.CoreV1().Secrets("team-a").Get(context.Background(), "billing-db", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, managedByLabelValue, live.Labels[managedByLabelKey])
	assert.Equal(t, "sec-1", live.Annotations[secretIdAnnotation])
	assert.Equal(t, map[string][]byte{"A": []byte("new"), "E": []byte(""), "NEW": []byte("added")}, live.Data)
}

func TestSyncSecrets_AdoptOwnObjectFromAnotherAppIsNotGuarded(t *testing.T) {
	t.Parallel()

	// объект уже помечен kusec, но аннотирован другим app: при sync по app-1
	// он не попадает в выборку управляемых и идёт через Create → AlreadyExists
	client := k8sfake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "billing-db",
				Namespace:   "team-a",
				Labels:      map[string]string{managedByLabelKey: managedByLabelValue},
				Annotations: map[string]string{appIdAnnotation: "app-0"},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"A": []byte("x"), "GONE": []byte("y")},
		},
	)
	svc := adoptSecretSvc(client, []*itemModel.Main{{Key: "A", Value: "x"}})

	result, err := svc.SyncSecrets(context.Background(), []string{"app-1"})
	require.NoError(t, err)

	assert.Empty(t, result.Errors)
	assert.Equal(t, []string{"team-a/billing-db"}, result.Updated)

	live, err := client.CoreV1().Secrets("team-a").Get(context.Background(), "billing-db", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, map[string][]byte{"A": []byte("x")}, live.Data)
	assert.Equal(t, "app-1", live.Annotations[appIdAnnotation])
}

// adoptConfigMapSvc — сервис с одним приложением и одним configmap-ом
// billing-config с exact_slug.
func adoptConfigMapSvc(client kubernetes.Interface, items []*configitemModel.Main) *Service {
	return &Service{
		client:     client,
		txm:        txmStub{},
		auditRec:   auditRecStub{},
		syncRunSvc: syncRunSvcStub{},
		appSvc: appSvcStub{
			listFn: func(_ context.Context, _ *appModel.ListReq) ([]*appModel.Main, int64, error) {
				return []*appModel.Main{{Id: "app-1", Namespace: "team-a", SlugName: "web"}}, 1, nil
			},
		},
		configMapSvc: configMapSvcStub{
			listFn: func(_ context.Context, _ *configmapModel.ListReq) ([]*configmapModel.Main, int64, error) {
				return []*configmapModel.Main{{Id: "cm-1", SlugName: "billing-config", ExactSlug: true}}, 1, nil
			},
		},
		configItemSvc: configItemSvcStub{
			listFn: func(_ context.Context, _ *configitemModel.ListReq) ([]*configitemModel.Main, int64, error) {
				return items, int64(len(items)), nil
			},
		},
	}
}

func TestSyncConfigMaps_AdoptRefusedWhenLiveDataWouldBeLost(t *testing.T) {
	t.Parallel()

	client := k8sfake.NewSimpleClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "billing-config", Namespace: "team-a"},
			Data:       map[string]string{"PG_HOST": "pg.internal", "PG_PORT": "5432"},
			BinaryData: map[string][]byte{"ca.crt": {0x00, 0x01}},
		},
	)
	// в kusec: PG_HOST пустой, бинарный ca.crt не заведён
	svc := adoptConfigMapSvc(client, []*configitemModel.Main{
		{Key: "PG_HOST", Value: ""},
		{Key: "PG_PORT", Value: "5432"},
	})

	result, err := svc.SyncConfigMaps(context.Background(), nil)
	require.NoError(t, err)

	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "team-a/billing-config: adopt: refused")
	assert.Contains(t, result.Errors[0], "keys missing in kusec: ca.crt")
	assert.Contains(t, result.Errors[0], "keys with empty value in kusec: PG_HOST")
	assert.NotContains(t, result.Errors[0], "pg.internal")
	assert.Empty(t, result.Updated)

	live, err := client.CoreV1().ConfigMaps("team-a").Get(context.Background(), "billing-config", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"PG_HOST": "pg.internal", "PG_PORT": "5432"}, live.Data)
	assert.Equal(t, map[string][]byte{"ca.crt": {0x00, 0x01}}, live.BinaryData)
	assert.Empty(t, live.Labels[managedByLabelKey])
}

func TestSyncConfigMaps_AdoptAllowedWhenNoLiveDataIsLost(t *testing.T) {
	t.Parallel()

	client := k8sfake.NewSimpleClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "billing-config", Namespace: "team-a"},
			Data:       map[string]string{"PG_HOST": "pg.internal"},
			BinaryData: map[string][]byte{"ca.crt": {0x00, 0x01}},
		},
	)
	svc := adoptConfigMapSvc(client, []*configitemModel.Main{
		{Key: "PG_HOST", Value: "pg.new"},
		{Key: "ca.crt", Value: "AAE=", Encoding: "base64"},
		{Key: "TZ", Value: "Asia/Almaty"},
	})

	result, err := svc.SyncConfigMaps(context.Background(), nil)
	require.NoError(t, err)

	assert.Empty(t, result.Errors)
	assert.Equal(t, []string{"team-a/billing-config"}, result.Updated)

	live, err := client.CoreV1().ConfigMaps("team-a").Get(context.Background(), "billing-config", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, managedByLabelValue, live.Labels[managedByLabelKey])
	assert.Equal(t, map[string]string{"PG_HOST": "pg.new", "TZ": "Asia/Almaty"}, live.Data)
	assert.Equal(t, map[string][]byte{"ca.crt": {0x00, 0x01}}, live.BinaryData)
}
