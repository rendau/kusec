package kube

import (
	"context"
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
)

type appSvcStub struct {
	listFn func(_ context.Context, req *appModel.ListReq) ([]*appModel.Main, int64, error)
	getFn  func(_ context.Context, id string, errNE bool) (*appModel.Main, bool, error)
}

func (s appSvcStub) List(ctx context.Context, req *appModel.ListReq) ([]*appModel.Main, int64, error) {
	return s.listFn(ctx, req)
}

func (s appSvcStub) Get(ctx context.Context, id string, errNE bool) (*appModel.Main, bool, error) {
	return s.getFn(ctx, id, errNE)
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
