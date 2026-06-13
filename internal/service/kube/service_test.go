package kube

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	appModel "github.com/mechta-market/kusec/internal/domain/app/model"
	itemModel "github.com/mechta-market/kusec/internal/domain/item/model"
	secretModel "github.com/mechta-market/kusec/internal/domain/secret/model"
)

type appSvcStub struct {
	listFn func(_ context.Context, req *appModel.ListReq) ([]*appModel.Main, int64, error)
}

func (s appSvcStub) List(ctx context.Context, req *appModel.ListReq) ([]*appModel.Main, int64, error) {
	return s.listFn(ctx, req)
}

type secretSvcStub struct {
	listFn func(_ context.Context, req *secretModel.ListReq) ([]*secretModel.Main, int64, error)
}

func (s secretSvcStub) List(ctx context.Context, req *secretModel.ListReq) ([]*secretModel.Main, int64, error) {
	return s.listFn(ctx, req)
}

type itemSvcStub struct {
	listFn func(_ context.Context, req *itemModel.ListReq) ([]*itemModel.Main, int64, error)
}

func (s itemSvcStub) List(ctx context.Context, req *itemModel.ListReq) ([]*itemModel.Main, int64, error) {
	return s.listFn(ctx, req)
}

func TestBuildSecretData_OK(t *testing.T) {
	t.Parallel()

	svc := &Service{
		itemSvc: itemSvcStub{
			listFn: func(_ context.Context, req *itemModel.ListReq) ([]*itemModel.Main, int64, error) {
				if req.SecretId == nil || *req.SecretId != "secret-1" {
					t.Fatalf("unexpected SecretId: %+v", req.SecretId)
				}
				return []*itemModel.Main{
					{Key: "PLAIN", Value: "value"},
					{Key: "BIN", Value: " aGVsbG8= \n", Encoding: "base64"},
				}, 2, nil
			},
		},
	}

	data, err := svc.buildSecretData(context.Background(), "secret-1")
	if err != nil {
		t.Fatalf("buildSecretData() error = %v", err)
	}

	if string(data["PLAIN"]) != "value" {
		t.Fatalf("unexpected PLAIN value: %q", string(data["PLAIN"]))
	}
	if string(data["BIN"]) != "hello" {
		t.Fatalf("unexpected BIN value: %q", string(data["BIN"]))
	}
}

func TestBuildSecretData_Errors(t *testing.T) {
	t.Parallel()

	t.Run("invalid key", func(t *testing.T) {
		t.Parallel()

		svc := &Service{
			itemSvc: itemSvcStub{
				listFn: func(_ context.Context, _ *itemModel.ListReq) ([]*itemModel.Main, int64, error) {
					return []*itemModel.Main{{Key: "bad key", Value: "v"}}, 1, nil
				},
			},
		}

		_, err := svc.buildSecretData(context.Background(), "secret-1")
		if err == nil || !strings.Contains(err.Error(), "invalid key") {
			t.Fatalf("expected invalid key error, got: %v", err)
		}
	})

	t.Run("invalid base64", func(t *testing.T) {
		t.Parallel()

		svc := &Service{
			itemSvc: itemSvcStub{
				listFn: func(_ context.Context, _ *itemModel.ListReq) ([]*itemModel.Main, int64, error) {
					return []*itemModel.Main{{Key: "BIN", Value: "%%%invalid%%%", Encoding: "base64"}}, 1, nil
				},
			},
		}

		_, err := svc.buildSecretData(context.Background(), "secret-1")
		if err == nil || !strings.Contains(err.Error(), "invalid base64 value") {
			t.Fatalf("expected invalid base64 error, got: %v", err)
		}
	})
}

func TestBuildDesired_NameCollisionAndInvalidNamespace(t *testing.T) {
	t.Parallel()

	svc := &Service{
		appSvc: appSvcStub{
			listFn: func(_ context.Context, _ *appModel.ListReq) ([]*appModel.Main, int64, error) {
				return []*appModel.Main{
					{Id: "app-1", Namespace: "team-a", SlugName: "web"},
					{Id: "app-2", Namespace: "team-a", SlugName: "web"},
					{Id: "app-3", Namespace: "Bad_NS", SlugName: "bad"},
				}, 3, nil
			},
		},
		secretSvc: secretSvcStub{
			listFn: func(_ context.Context, req *secretModel.ListReq) ([]*secretModel.Main, int64, error) {
				if req.AppId == nil {
					t.Fatal("AppId is required")
				}
				switch *req.AppId {
				case "app-1", "app-2":
					return []*secretModel.Main{{Id: "sec-1", SlugName: "db"}}, 1, nil
				case "app-3":
					return []*secretModel.Main{{Id: "sec-2", SlugName: "cfg"}}, 1, nil
				default:
					return nil, 0, nil
				}
			},
		},
		itemSvc: itemSvcStub{
			listFn: func(_ context.Context, _ *itemModel.ListReq) ([]*itemModel.Main, int64, error) {
				return []*itemModel.Main{{Key: "A", Value: "1"}}, 1, nil
			},
		},
	}

	result := &SyncResult{}
	desired, err := svc.buildDesired(context.Background(), result, nil)
	if err != nil {
		t.Fatalf("buildDesired() error = %v", err)
	}

	if len(desired) != 1 {
		t.Fatalf("unexpected desired size: %d", len(desired))
	}
	if _, ok := desired["team-a/"+SecretName("web", "db")]; !ok {
		t.Fatalf("expected desired secret key missing")
	}

	if !containsSubstr(result.Errors, "name collision") {
		t.Fatalf("expected collision error, got: %#v", result.Errors)
	}
	if !containsSubstr(result.Errors, "invalid namespace") {
		t.Fatalf("expected invalid namespace error, got: %#v", result.Errors)
	}
}

func TestListNamespaces_FiltersSystemAndSorts(t *testing.T) {
	t.Parallel()

	client := k8sfake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "z-prod"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "a-dev"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-public"}},
	)
	svc := &Service{client: client}

	got, inCluster, err := svc.ListNamespaces(context.Background())
	if err != nil {
		t.Fatalf("ListNamespaces() error = %v", err)
	}
	if !inCluster {
		t.Fatal("expected inCluster=true")
	}
	want := []string{"a-dev", "z-prod"}
	if !slices.Equal(got, want) {
		t.Fatalf("ListNamespaces() = %v, want %v", got, want)
	}
}

func TestSyncSecrets_ScopedSyncUpdatesAndDeletesOnlyScopedApp(t *testing.T) {
	t.Parallel()

	existingScopedName := SecretName("app1", "main")
	client := k8sfake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      existingScopedName,
				Namespace: "team-a",
				Labels: map[string]string{
					managedByLabelKey: managedByLabelValue,
				},
				Annotations: map[string]string{
					appIdAnnotation:    "app-1",
					secretIdAnnotation: "sec-main",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"A": []byte("old")},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stale-app1",
				Namespace: "team-a",
				Labels: map[string]string{
					managedByLabelKey: managedByLabelValue,
				},
				Annotations: map[string]string{
					appIdAnnotation:    "app-1",
					secretIdAnnotation: "sec-stale",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"A": []byte("stale")},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "other-app2",
				Namespace: "team-a",
				Labels: map[string]string{
					managedByLabelKey: managedByLabelValue,
				},
				Annotations: map[string]string{
					appIdAnnotation:    "app-2",
					secretIdAnnotation: "sec-other",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"A": []byte("keep")},
		},
	)

	svc := &Service{
		client: client,
		appSvc: appSvcStub{
			listFn: func(_ context.Context, req *appModel.ListReq) ([]*appModel.Main, int64, error) {
				if len(req.Ids) != 1 || req.Ids[0] != "app-1" {
					t.Fatalf("unexpected app scope: %#v", req.Ids)
				}
				return []*appModel.Main{{Id: "app-1", Namespace: "team-a", SlugName: "app1"}}, 1, nil
			},
		},
		secretSvc: secretSvcStub{
			listFn: func(_ context.Context, req *secretModel.ListReq) ([]*secretModel.Main, int64, error) {
				if req.AppId == nil || *req.AppId != "app-1" {
					t.Fatalf("unexpected AppId: %+v", req.AppId)
				}
				return []*secretModel.Main{{Id: "sec-main", SlugName: "main"}}, 1, nil
			},
		},
		itemSvc: itemSvcStub{
			listFn: func(_ context.Context, req *itemModel.ListReq) ([]*itemModel.Main, int64, error) {
				if req.SecretId == nil || *req.SecretId != "sec-main" {
					t.Fatalf("unexpected SecretId: %+v", req.SecretId)
				}
				return []*itemModel.Main{{Key: "A", Value: "new"}}, 1, nil
			},
		},
	}

	result, err := svc.SyncSecrets(context.Background(), []string{"app-1"})
	if err != nil {
		t.Fatalf("SyncSecrets() error = %v", err)
	}

	if !slices.Equal(result.Updated, []string{"team-a/" + existingScopedName}) {
		t.Fatalf("unexpected updated list: %#v", result.Updated)
	}
	if !slices.Equal(result.Deleted, []string{"team-a/stale-app1"}) {
		t.Fatalf("unexpected deleted list: %#v", result.Deleted)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected sync errors: %#v", result.Errors)
	}

	if _, err = client.CoreV1().Secrets("team-a").Get(context.Background(), "other-app2", metav1.GetOptions{}); err != nil {
		t.Fatalf("secret for other app should not be touched: %v", err)
	}
	scopedSecret, err := client.CoreV1().Secrets("team-a").Get(context.Background(), existingScopedName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("scoped secret must exist: %v", err)
	}
	if string(scopedSecret.Data["A"]) != "new" {
		t.Fatalf("scoped secret must be updated, got: %q", string(scopedSecret.Data["A"]))
	}
}

func TestSecretUpToDate(t *testing.T) {
	t.Parallel()

	want := &desiredSecret{
		name:      "sec",
		namespace: "ns",
		appId:     "app-1",
		secretId:  "sec-1",
		data:      map[string][]byte{"A": []byte("v")},
	}
	current := buildSecret(want)
	if !secretUpToDate(current, want) {
		t.Fatal("expected up-to-date secret")
	}

	current = current.DeepCopy()
	current.Annotations[secretIdAnnotation] = "other"
	if secretUpToDate(current, want) {
		t.Fatal("expected not up-to-date due to annotation mismatch")
	}
}

func TestBuildDesired_PropagatesServiceError(t *testing.T) {
	t.Parallel()

	svc := &Service{
		appSvc: appSvcStub{
			listFn: func(_ context.Context, _ *appModel.ListReq) ([]*appModel.Main, int64, error) {
				return nil, 0, errors.New("db down")
			},
		},
	}

	_, err := svc.buildDesired(context.Background(), &SyncResult{}, nil)
	if err == nil || !strings.Contains(err.Error(), "appSvc.List") {
		t.Fatalf("expected wrapped appSvc.List error, got: %v", err)
	}
}

func containsSubstr(values []string, sub string) bool {
	for _, v := range values {
		if strings.Contains(v, sub) {
			return true
		}
	}
	return false
}
