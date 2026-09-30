package kube

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
	"github.com/rendau/kusec/internal/errs"
)

func TestEncodeImportValue(t *testing.T) {
	t.Parallel()

	if v, enc := encodeImportValue([]byte("hello")); v != "hello" || enc != "plain" {
		t.Fatalf("plain: got (%q, %q)", v, enc)
	}
	if v, enc := encodeImportValue([]byte{0x00, 0xff, 0x10}); enc != "base64" || v != "AP8Q" {
		t.Fatalf("binary: got (%q, %q)", v, enc)
	}
}

func TestListClusterSecrets_FiltersAndSorts(t *testing.T) {
	t.Parallel()

	client := k8sfake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "b-secret", Namespace: "team-a"},
			Type:       corev1.SecretTypeOpaque,
			Data:       map[string][]byte{"Z": []byte("1"), "A": []byte("2")},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "a-secret",
				Namespace: "team-a",
				Labels:    map[string]string{managedByLabelKey: managedByLabelValue},
			},
			Type: corev1.SecretType("kubernetes.io/basic-auth"),
			Data: map[string][]byte{"username": []byte("u")},
		},
		// Служебный токен — отфильтровывается.
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "sa-token", Namespace: "team-a"},
			Type:       corev1.SecretTypeServiceAccountToken,
		},
		// Системный namespace — отфильтровывается.
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "sys", Namespace: "kube-system"},
			Type:       corev1.SecretTypeOpaque,
		},
	)
	svc := &Service{client: client}

	got, inCluster, err := svc.ListClusterSecrets(context.Background(), "")
	if err != nil {
		t.Fatalf("ListClusterSecrets() error = %v", err)
	}
	if !inCluster {
		t.Fatal("expected inCluster=true")
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 secrets, got %d: %#v", len(got), got)
	}
	// Отсортированы по namespace, затем по имени.
	if got[0].Name != "a-secret" || got[1].Name != "b-secret" {
		t.Fatalf("unexpected order: %q, %q", got[0].Name, got[1].Name)
	}
	if !got[0].Managed || got[0].Type != "kubernetes.io/basic-auth" {
		t.Fatalf("a-secret: managed/type mismatch: %#v", got[0])
	}
	if got[1].Type != "" {
		t.Fatalf("b-secret: Opaque must map to empty type, got %q", got[1].Type)
	}
	if !slices.Equal(got[1].Keys, []string{"A", "Z"}) {
		t.Fatalf("b-secret: keys must be sorted, got %#v", got[1].Keys)
	}
}

func TestImportSecret_CreatesSecretItemsInTargetApp(t *testing.T) {
	t.Parallel()

	client := k8sfake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "app-creds", Namespace: "team-a"},
			Type:       corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"USER": []byte("admin"),
				"BIN":  {0x00, 0x01},
			},
		},
	)

	var createdSecretApp, createdSlug string
	createdItems := map[string]string{} // key -> encoding

	svc := &Service{
		client:   client,
		txm:      txmStub{},
		auditRec: auditRecStub{},
		appSvc: appSvcStub{
			getFn: func(_ context.Context, id string, _ bool) (*appModel.Main, bool, error) {
				if id != "app-1" {
					t.Fatalf("unexpected target app id: %q", id)
				}
				return &appModel.Main{Id: "app-1", Namespace: "web", SlugName: "web"}, true, nil
			},
		},
		secretSvc: secretSvcStub{
			listFn: func(_ context.Context, _ *secretModel.ListReq) ([]*secretModel.Main, int64, error) {
				return nil, 0, nil // существующего секрета нет
			},
			createFn: func(_ context.Context, obj *secretModel.Edit) (string, error) {
				createdSecretApp = *obj.AppId
				createdSlug = *obj.SlugName
				return "sec-1", nil
			},
		},
		itemSvc: itemSvcStub{
			createFn: func(_ context.Context, obj *itemModel.Edit) (string, error) {
				createdItems[*obj.Key] = *obj.Encoding
				return "item", nil
			},
		},
	}

	// secretSlug задан пользователем — он и становится slug посадочного секрета.
	result, err := svc.ImportSecret(context.Background(), "app-1",
		ImportRef{Namespace: "team-a", Name: "app-creds"}, "db", ImportOptions{})
	if err != nil {
		t.Fatalf("ImportSecret() error = %v", err)
	}

	if result.ObjectId != "sec-1" || result.Slug != "db" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.CreatedItems != 2 {
		t.Fatalf("unexpected created items: %+v", result)
	}
	if createdSecretApp != "app-1" {
		t.Fatalf("secret must be created in the target app, got %q", createdSecretApp)
	}
	if createdSlug != "db" {
		t.Fatalf("landing slug must be the user-provided one, got %q", createdSlug)
	}
	if createdItems["USER"] != "plain" || createdItems["BIN"] != "base64" {
		t.Fatalf("unexpected item encodings: %#v", createdItems)
	}
}

func TestImportSecret_TopsUpAndOverrides(t *testing.T) {
	t.Parallel()

	client := k8sfake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "app-creds", Namespace: "team-a"},
			Type:       corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"USER": []byte("admin"),  // уже есть в kusec — значение перезаписывается
				"PASS": []byte("secret"), // недостающий — дозаполняется
			},
		},
	)

	createdItems := map[string]string{} // key -> encoding
	updatedItems := map[string]string{} // id -> value

	svc := &Service{
		client:   client,
		txm:      txmStub{},
		auditRec: auditRecStub{},
		appSvc: appSvcStub{
			getFn: func(_ context.Context, _ string, _ bool) (*appModel.Main, bool, error) {
				return &appModel.Main{Id: "app-1", Namespace: "web", SlugName: "web"}, true, nil
			},
		},
		secretSvc: secretSvcStub{
			listFn: func(_ context.Context, _ *secretModel.ListReq) ([]*secretModel.Main, int64, error) {
				return []*secretModel.Main{{Id: "sec-1", SlugName: "db"}}, 1, nil
			},
			createFn: func(_ context.Context, _ *secretModel.Edit) (string, error) {
				t.Fatal("Create must not be called for an existing secret")
				return "", nil
			},
		},
		itemSvc: itemSvcStub{
			listFn: func(_ context.Context, _ *itemModel.ListReq) ([]*itemModel.Main, int64, error) {
				return []*itemModel.Main{{Id: "i1", Key: "USER", Value: "old-admin"}}, 1, nil
			},
			createFn: func(_ context.Context, obj *itemModel.Edit) (string, error) {
				createdItems[*obj.Key] = *obj.Encoding
				return "item", nil
			},
			updateFn: func(_ context.Context, id string, obj *itemModel.Edit) error {
				updatedItems[id] = *obj.Value
				return nil
			},
		},
	}

	// REST-режим: все ключи, совпавшие перезаписываются.
	result, err := svc.ImportSecret(context.Background(), "app-1",
		ImportRef{Namespace: "team-a", Name: "app-creds"}, "db", ImportOptions{Overwrite: true})
	if err != nil {
		t.Fatalf("ImportSecret() error = %v", err)
	}
	if result.ObjectId != "sec-1" || result.ObjectCreated {
		t.Fatalf("must reuse existing secret: %+v", result)
	}
	if result.CreatedItems != 1 || result.UpdatedItems != 1 {
		t.Fatalf("unexpected counts: %+v", result)
	}
	if _, has := createdItems["USER"]; has {
		t.Fatalf("existing key USER must be updated, not recreated: %#v", createdItems)
	}
	if updatedItems["i1"] != "admin" {
		t.Fatalf("existing key USER must be overridden with cluster value: %#v", updatedItems)
	}
	if createdItems["PASS"] == "" {
		t.Fatalf("missing key PASS must be created: %#v", createdItems)
	}
}

// ── Опции импорта: overwrite и keys ─────────────────────

// importFixture — секрет-источник с тремя ключами и посадочный секрет "db", в
// котором NEW отсутствует, EMPTY существует с пустым значением, FULL — с
// непустым. Значения источника намеренно похожи на секреты: тесты проверяют,
// что они не всплывают ни в результате, ни в ошибках.
const (
	fxNewValue   = "new-Value-7f3a"
	fxEmptyValue = "empty-Value-9c1d"
	fxFullValue  = "full-Value-2b8e"
	fxFullOld    = "old-kusec-Value"
)

type importFixture struct {
	svc     *Service
	created map[string]string // item key -> value
	updated map[string]string // item id -> value
}

func newImportFixture(t *testing.T) *importFixture {
	t.Helper()

	client := k8sfake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "app-creds", Namespace: "team-a"},
			Type:       corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"NEW":   []byte(fxNewValue),
				"EMPTY": []byte(fxEmptyValue),
				"FULL":  []byte(fxFullValue),
			},
		},
	)

	fx := &importFixture{
		created: map[string]string{},
		updated: map[string]string{},
	}
	fx.svc = &Service{
		client:   client,
		txm:      txmStub{},
		auditRec: auditRecStub{},
		appSvc: appSvcStub{
			getFn: func(_ context.Context, _ string, _ bool) (*appModel.Main, bool, error) {
				return &appModel.Main{Id: "app-1", Namespace: "web", SlugName: "web"}, true, nil
			},
		},
		secretSvc: secretSvcStub{
			listFn: func(_ context.Context, _ *secretModel.ListReq) ([]*secretModel.Main, int64, error) {
				return []*secretModel.Main{{Id: "sec-1", SlugName: "db", KubeType: "kubernetes.io/basic-auth"}}, 1, nil
			},
			createFn: func(_ context.Context, _ *secretModel.Edit) (string, error) {
				t.Fatal("Create must not be called for an existing secret")
				return "", nil
			},
		},
		itemSvc: itemSvcStub{
			listFn: func(_ context.Context, _ *itemModel.ListReq) ([]*itemModel.Main, int64, error) {
				return []*itemModel.Main{
					{Id: "i-empty", Key: "EMPTY", Value: ""},
					{Id: "i-full", Key: "FULL", Value: fxFullOld},
				}, 2, nil
			},
			createFn: func(_ context.Context, obj *itemModel.Edit) (string, error) {
				fx.created[*obj.Key] = *obj.Value
				return "item-" + *obj.Key, nil
			},
			updateFn: func(_ context.Context, id string, obj *itemModel.Edit) error {
				fx.updated[id] = *obj.Value
				return nil
			},
		},
	}
	return fx
}

func (fx *importFixture) run(t *testing.T, opts ImportOptions) (*ImportResult, error) {
	t.Helper()
	return fx.svc.ImportSecret(context.Background(), "app-1",
		ImportRef{Namespace: "team-a", Name: "app-creds"}, "db", opts)
}

func TestImportSecret_NoOverwrite_CreatesFillsSkips(t *testing.T) {
	t.Parallel()

	fx := newImportFixture(t)

	result, err := fx.run(t, ImportOptions{})
	require.NoError(t, err)

	assert.Equal(t, []string{"NEW"}, result.CreatedKeys)
	assert.Equal(t, []string{"EMPTY"}, result.FilledKeys)
	assert.Empty(t, result.UpdatedKeys)
	assert.Equal(t, []string{"FULL"}, result.SkippedKeys)
	assert.EqualValues(t, 1, result.CreatedItems)
	assert.EqualValues(t, 1, result.UpdatedItems)

	// метаданные существующего секрета: kube_type прежний, имя k8s-секрета с префиксом
	assert.False(t, result.ObjectCreated)
	assert.Equal(t, "kubernetes.io/basic-auth", result.KubeType)
	assert.Equal(t, SecretName("web", "db", false), result.KubeName)

	// мутации: новый создан, пустой заполнен, непустой не тронут
	assert.Equal(t, map[string]string{"NEW": fxNewValue}, fx.created)
	assert.Equal(t, map[string]string{"i-empty": fxEmptyValue}, fx.updated)
}

func TestImportSecret_Overwrite_UpdatesNonEmpty(t *testing.T) {
	t.Parallel()

	fx := newImportFixture(t)

	result, err := fx.run(t, ImportOptions{Overwrite: true})
	require.NoError(t, err)

	assert.Equal(t, []string{"NEW"}, result.CreatedKeys)
	assert.Equal(t, []string{"EMPTY"}, result.FilledKeys)
	assert.Equal(t, []string{"FULL"}, result.UpdatedKeys)
	assert.Empty(t, result.SkippedKeys)
	assert.EqualValues(t, 1, result.CreatedItems)
	assert.EqualValues(t, 2, result.UpdatedItems)

	assert.Equal(t, map[string]string{"NEW": fxNewValue}, fx.created)
	assert.Equal(t, map[string]string{"i-empty": fxEmptyValue, "i-full": fxFullValue}, fx.updated)
}

func TestImportSecret_KeysSubsetAndRename(t *testing.T) {
	t.Parallel()

	fx := newImportFixture(t)

	// NEW переименовывается в DB_NEW, EMPTY — под своим именем (пустая строка),
	// FULL не переносится вовсе.
	result, err := fx.run(t, ImportOptions{Keys: map[string]string{"NEW": "DB_NEW", "EMPTY": ""}})
	require.NoError(t, err)

	assert.Equal(t, []string{"DB_NEW"}, result.CreatedKeys)
	assert.Equal(t, []string{"EMPTY"}, result.FilledKeys)
	assert.Empty(t, result.UpdatedKeys)
	assert.Empty(t, result.SkippedKeys, "невыбранный ключ не должен попадать и в skipped")

	assert.Equal(t, map[string]string{"DB_NEW": fxNewValue}, fx.created)
	assert.Equal(t, map[string]string{"i-empty": fxEmptyValue}, fx.updated)
}

func TestImportSecret_KeysMissing_ErrorWithoutValues(t *testing.T) {
	t.Parallel()

	fx := newImportFixture(t)

	_, err := fx.run(t, ImportOptions{Keys: map[string]string{"NEW": "", "NOPE": "", "ALSO_NOPE": "X"}})
	require.Error(t, err)

	full, ok := errors.AsType[errs.ErrFull](err)
	require.True(t, ok, "expected errs.ErrFull, got %T: %v", err, err)
	assert.ErrorIs(t, full.Err, errs.InvalidRequest)
	assert.Contains(t, full.Desc, "ALSO_NOPE, NOPE")
	assert.NotContains(t, full.Desc, "NEW,", "существующий ключ не должен числиться недостающим")

	for _, value := range []string{fxNewValue, fxEmptyValue, fxFullValue, fxFullOld} {
		assert.NotContains(t, err.Error(), value, "значение утекло в текст ошибки")
	}

	// проверка до любых мутаций
	assert.Empty(t, fx.created)
	assert.Empty(t, fx.updated)
}

func TestImportKeyPlan_ValidatesItemKeys(t *testing.T) {
	t.Parallel()

	data := map[string][]byte{"A": []byte("1"), "B": []byte("2")}

	// невалидное имя item-а
	_, err := importKeyPlan(sortedDataKeys(data), map[string]string{"A": "bad key!"})
	require.Error(t, err)
	full, ok := errors.AsType[errs.ErrFull](err)
	require.True(t, ok)
	assert.ErrorIs(t, full.Err, errs.InvalidRequest)
	assert.Contains(t, full.Desc, `"bad key!"`)

	// два ключа источника на одно имя item-а
	_, err = importKeyPlan(sortedDataKeys(data), map[string]string{"A": "SAME", "B": "SAME"})
	require.Error(t, err)
	full, ok = errors.AsType[errs.ErrFull](err)
	require.True(t, ok)
	assert.Contains(t, full.Desc, `"SAME"`)

	// без keys — все ключи 1:1 в отсортированном порядке
	plan, err := importKeyPlan(sortedDataKeys(data), nil)
	require.NoError(t, err)
	assert.Equal(t, []importKeyEntry{{"A", "A"}, {"B", "B"}}, plan)
}

// ── Пакетный импорт ─────────────────────────────────────

// auditRecSpy — регистратор аудита, запоминающий batch_id всех записей и
// число выданных batch_id.
type auditRecSpy struct {
	auditRecStub
	newBatchIds int
	recorded    []string
}

func (a *auditRecSpy) NewBatchId() string {
	a.newBatchIds++
	return fmt.Sprintf("batch-%d", a.newBatchIds)
}

func (a *auditRecSpy) RecordSecret(_ context.Context, _, _ *secretModel.Main, _ string, batchId *string) error {
	a.recorded = append(a.recorded, *batchId)
	return nil
}

func (a *auditRecSpy) RecordItem(_ context.Context, _, _ *itemModel.Main, _ string, batchId *string) error {
	a.recorded = append(a.recorded, *batchId)
	return nil
}

func (a *auditRecSpy) RecordConfigMap(_ context.Context, _, _ *configmapModel.Main, _ string, batchId *string) error {
	a.recorded = append(a.recorded, *batchId)
	return nil
}

func (a *auditRecSpy) RecordConfigItem(_ context.Context, _, _ *configitemModel.Main, _ string, batchId *string) error {
	a.recorded = append(a.recorded, *batchId)
	return nil
}

func TestImportSecrets_Batch_ContinuesOnErrorSharesBatchId(t *testing.T) {
	t.Parallel()

	const (
		aUser  = "a-user-Value"
		aPass  = "a-pass-Value"
		cToken = "c-token-Value"
	)

	client := k8sfake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "a-creds", Namespace: "team-a"},
			Type:       corev1.SecretTypeOpaque,
			Data:       map[string][]byte{"USER": []byte(aUser), "PASS": []byte(aPass)},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "c-creds", Namespace: "team-a"},
			Type:       corev1.SecretType("kubernetes.io/basic-auth"),
			Data:       map[string][]byte{"TOKEN": []byte(cToken)},
		},
	)

	spy := &auditRecSpy{}
	createdSlugs := []string{}
	createdItems := map[string][]string{} // secret id -> keys

	svc := &Service{
		client:   client,
		txm:      txmStub{},
		auditRec: spy,
		appSvc: appSvcStub{
			getFn: func(_ context.Context, _ string, _ bool) (*appModel.Main, bool, error) {
				return &appModel.Main{Id: "app-1", Namespace: "web", SlugName: "web"}, true, nil
			},
		},
		secretSvc: secretSvcStub{
			listFn: func(_ context.Context, _ *secretModel.ListReq) ([]*secretModel.Main, int64, error) {
				return nil, 0, nil
			},
			createFn: func(_ context.Context, obj *secretModel.Edit) (string, error) {
				createdSlugs = append(createdSlugs, *obj.SlugName)
				return "sec-" + *obj.SlugName, nil
			},
		},
		itemSvc: itemSvcStub{
			createFn: func(_ context.Context, obj *itemModel.Edit) (string, error) {
				createdItems[*obj.SecretId] = append(createdItems[*obj.SecretId], *obj.Key)
				return "item-" + *obj.Key, nil
			},
		},
	}

	items, err := svc.ImportSecrets(context.Background(), "app-1", []ImportSpec{
		{Ref: ImportRef{Namespace: "team-a", Name: "a-creds"}, Slug: "a"},
		// нет в кластере — ошибка только у этого секрета
		{Ref: ImportRef{Namespace: "team-a", Name: "missing"}, Slug: "b"},
		// лишний ключ в keys — ошибка валидации, мутаций нет
		{Ref: ImportRef{Namespace: "team-a", Name: "c-creds"}, Slug: "c", Options: ImportOptions{Keys: map[string]string{"NOPE": ""}}},
		{Ref: ImportRef{Namespace: "team-a", Name: "c-creds"}, Slug: "c"},
	})
	require.NoError(t, err)
	require.Len(t, items, 4)

	// [0] успех
	require.NoError(t, items[0].Err)
	require.NotNil(t, items[0].Result)
	assert.Equal(t, "sec-a", items[0].Result.ObjectId)
	assert.Equal(t, []string{"PASS", "USER"}, items[0].Result.CreatedKeys)

	// [1] секрет не найден в кластере
	require.Error(t, items[1].Err)
	assert.Nil(t, items[1].Result)
	assert.Contains(t, items[1].Err.Error(), "team-a/missing")
	assert.Equal(t, ImportRef{Namespace: "team-a", Name: "missing"}, items[1].Ref)
	assert.Equal(t, "b", items[1].Slug)

	// [2] лишний ключ
	require.Error(t, items[2].Err)
	full, ok := errors.AsType[errs.ErrFull](items[2].Err)
	require.True(t, ok)
	assert.ErrorIs(t, full.Err, errs.InvalidRequest)
	assert.Contains(t, full.Desc, "NOPE")

	// [3] успех после ошибок соседей, kube_type скопирован из источника
	require.NoError(t, items[3].Err)
	assert.Equal(t, []string{"TOKEN"}, items[3].Result.CreatedKeys)
	assert.Equal(t, "kubernetes.io/basic-auth", items[3].Result.KubeType)

	// мутации только у успешных
	assert.Equal(t, []string{"a", "c"}, createdSlugs)
	assert.Equal(t, map[string][]string{"sec-a": {"PASS", "USER"}, "sec-c": {"TOKEN"}}, createdItems)

	// ни одно значение не всплыло в ошибках
	for _, item := range items {
		if item.Err == nil {
			continue
		}
		for _, value := range []string{aUser, aPass, cToken} {
			assert.NotContains(t, item.Err.Error(), value)
		}
	}

	// один batch_id на весь пакет
	assert.Equal(t, 1, spy.newBatchIds)
	assert.Len(t, spy.recorded, 5, "2 secret + 3 item")
	assert.Equal(t, []string{"batch-1"}, lo.Uniq(spy.recorded))
}
