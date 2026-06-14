package kube

import (
	"context"
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	appModel "github.com/mechta-market/kusec/internal/domain/app/model"
	itemModel "github.com/mechta-market/kusec/internal/domain/item/model"
	secretModel "github.com/mechta-market/kusec/internal/domain/secret/model"
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
		client: client,
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
		ImportRef{Namespace: "team-a", Name: "app-creds"}, "db")
	if err != nil {
		t.Fatalf("ImportSecret() error = %v", err)
	}

	if result.SecretId != "sec-1" || result.SecretSlug != "db" {
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
		client: client,
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
				return []*itemModel.Main{{Id: "i1", Key: "USER"}}, 1, nil
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

	result, err := svc.ImportSecret(context.Background(), "app-1",
		ImportRef{Namespace: "team-a", Name: "app-creds"}, "db")
	if err != nil {
		t.Fatalf("ImportSecret() error = %v", err)
	}
	if result.SecretId != "sec-1" || result.SecretCreated {
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
