package kube

import (
	"context"
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
	configitemModel "github.com/rendau/kusec/internal/domain/configitem/model"
	configmapModel "github.com/rendau/kusec/internal/domain/configmap/model"
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
)

type configMapSvcStub struct {
	listFn func(_ context.Context, req *configmapModel.ListReq) ([]*configmapModel.Main, int64, error)
}

func (s configMapSvcStub) List(ctx context.Context, req *configmapModel.ListReq) ([]*configmapModel.Main, int64, error) {
	return s.listFn(ctx, req)
}

type configItemSvcStub struct {
	listFn func(_ context.Context, req *configitemModel.ListReq) ([]*configitemModel.Main, int64, error)
}

func (s configItemSvcStub) List(ctx context.Context, req *configitemModel.ListReq) ([]*configitemModel.Main, int64, error) {
	return s.listFn(ctx, req)
}

func TestBuildConfigMapData_OK(t *testing.T) {
	t.Parallel()

	svc := &Service{
		configItemSvc: configItemSvcStub{
			listFn: func(_ context.Context, req *configitemModel.ListReq) ([]*configitemModel.Main, int64, error) {
				if req.ConfigMapId == nil || *req.ConfigMapId != "cm-1" {
					t.Fatalf("unexpected ConfigMapId: %+v", req.ConfigMapId)
				}
				return []*configitemModel.Main{
					{Key: "PLAIN", Value: "value"},
					{Key: "BIN", Value: " aGVsbG8= \n", Encoding: "base64"},
				}, 2, nil
			},
		},
	}

	data, binaryData, err := svc.buildConfigMapData(context.Background(), "cm-1")
	if err != nil {
		t.Fatalf("buildConfigMapData() error = %v", err)
	}

	if data["PLAIN"] != "value" {
		t.Fatalf("unexpected PLAIN value: %q", data["PLAIN"])
	}
	if _, ok := data["BIN"]; ok {
		t.Fatalf("BIN must not land in Data (text), got: %q", data["BIN"])
	}
	if string(binaryData["BIN"]) != "hello" {
		t.Fatalf("unexpected BIN value: %q", string(binaryData["BIN"]))
	}
}

func TestSync_ReconcilesSecretsAndConfigMaps(t *testing.T) {
	t.Parallel()

	client := k8sfake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}},
	)

	svc := &Service{
		client: client,
		appSvc: appSvcStub{
			listFn: func(_ context.Context, _ *appModel.ListReq) ([]*appModel.Main, int64, error) {
				return []*appModel.Main{{Id: "app-1", Namespace: "team-a", SlugName: "web"}}, 1, nil
			},
		},
		secretSvc: secretSvcStub{
			listFn: func(_ context.Context, _ *secretModel.ListReq) ([]*secretModel.Main, int64, error) {
				return []*secretModel.Main{{Id: "sec-1", SlugName: "db"}}, 1, nil
			},
		},
		itemSvc: itemSvcStub{
			listFn: func(_ context.Context, _ *itemModel.ListReq) ([]*itemModel.Main, int64, error) {
				return []*itemModel.Main{{Key: "A", Value: "1"}}, 1, nil
			},
		},
		configMapSvc: configMapSvcStub{
			listFn: func(_ context.Context, _ *configmapModel.ListReq) ([]*configmapModel.Main, int64, error) {
				return []*configmapModel.Main{{Id: "cm-1", SlugName: "app-config"}}, 1, nil
			},
		},
		configItemSvc: configItemSvcStub{
			listFn: func(_ context.Context, _ *configitemModel.ListReq) ([]*configitemModel.Main, int64, error) {
				return []*configitemModel.Main{{Key: "B", Value: "2"}}, 1, nil
			},
		},
	}

	secrets, configMaps, err := svc.Sync(context.Background(), []string{"app-1"})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	wantSecret := "team-a/" + SecretName("web", "db", false)
	if !slices.Equal(secrets.Created, []string{wantSecret}) {
		t.Fatalf("unexpected secrets.Created: %#v", secrets.Created)
	}
	wantConfigMap := "team-a/" + ConfigMapName("web", "app-config", false)
	if !slices.Equal(configMaps.Created, []string{wantConfigMap}) {
		t.Fatalf("unexpected configMaps.Created: %#v", configMaps.Created)
	}

	if _, err = client.CoreV1().Secrets("team-a").Get(context.Background(), SecretName("web", "db", false), metav1.GetOptions{}); err != nil {
		t.Fatalf("synced secret must exist: %v", err)
	}
	if _, err = client.CoreV1().ConfigMaps("team-a").Get(context.Background(), ConfigMapName("web", "app-config", false), metav1.GetOptions{}); err != nil {
		t.Fatalf("synced configmap must exist: %v", err)
	}
}

func TestBuildConfigMapData_Errors(t *testing.T) {
	t.Parallel()

	t.Run("invalid key", func(t *testing.T) {
		t.Parallel()

		svc := &Service{
			configItemSvc: configItemSvcStub{
				listFn: func(_ context.Context, _ *configitemModel.ListReq) ([]*configitemModel.Main, int64, error) {
					return []*configitemModel.Main{{Key: "bad key", Value: "v"}}, 1, nil
				},
			},
		}

		_, _, err := svc.buildConfigMapData(context.Background(), "cm-1")
		if err == nil || !strings.Contains(err.Error(), "invalid key") {
			t.Fatalf("expected invalid key error, got: %v", err)
		}
	})

	t.Run("invalid base64", func(t *testing.T) {
		t.Parallel()

		svc := &Service{
			configItemSvc: configItemSvcStub{
				listFn: func(_ context.Context, _ *configitemModel.ListReq) ([]*configitemModel.Main, int64, error) {
					return []*configitemModel.Main{{Key: "BIN", Value: "%%%invalid%%%", Encoding: "base64"}}, 1, nil
				},
			},
		}

		_, _, err := svc.buildConfigMapData(context.Background(), "cm-1")
		if err == nil || !strings.Contains(err.Error(), "invalid base64 value") {
			t.Fatalf("expected invalid base64 error, got: %v", err)
		}
	})
}
