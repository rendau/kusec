package kube

import (
	"context"
	"strings"
	"testing"

	configitemModel "github.com/mechta-market/kusec/internal/domain/configitem/model"
)

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
