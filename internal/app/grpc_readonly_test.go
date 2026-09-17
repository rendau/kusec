package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/rendau/kusec/internal/constant"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	sessionService "github.com/rendau/kusec/internal/domain/session/service"
	"github.com/rendau/kusec/internal/errs"
)

// Белый список read_only: разрешено только чтение, любая мутация и чтение
// значений из кластера — отказ; новый (не перечисленный) метод закрыт по
// умолчанию.
func TestGrpcInterceptorReadOnlyScope(t *testing.T) {
	t.Parallel()

	sessionSvc := sessionService.New("test-secret")
	interceptor := GrpcInterceptorReadOnlyScope(sessionSvc)

	roCtx := sessionSvc.WithContext(context.Background(), &sessionModel.Session{
		Id: 1, Scope: constant.ApiKeyScopeReadOnly,
	})
	fullCtx := sessionSvc.WithContext(context.Background(), &sessionModel.Session{Id: 1})

	call := func(ctx context.Context, method string) (bool, error) {
		called := false
		_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) {
			called = true
			return nil, nil
		})
		return called, err
	}

	// чтение из белого списка проходит
	for _, method := range []string{
		"/kusec_v1.App/List",
		"/kusec_v1.Item/Get",
		"/kusec_v1.Audit/List",
		"/kusec_v1.SyncRun/Get",
		"/kusec_v1.Transfer/Tree",
	} {
		called, err := call(roCtx, method)
		require.NoError(t, err, method)
		assert.True(t, called, method)
	}

	// мутации, значения из кластера и неизвестные методы — отказ
	for _, method := range []string{
		"/kusec_v1.Item/Create",
		"/kusec_v1.Item/Update",
		"/kusec_v1.Item/Delete",
		"/kusec_v1.Secret/Create",
		"/kusec_v1.App/Delete",
		"/kusec_v1.Kube/Sync",
		"/kusec_v1.Kube/GetClusterSecret",
		"/kusec_v1.Kube/GetClusterConfigMap",
		"/kusec_v1.ApiKey/Create",
		"/kusec_v1.Usr/List",
		"/kusec_v1.Dashboard/Get",
		"/kusec_v1.Future/NewMethod", // новый метод по умолчанию закрыт
	} {
		called, err := call(roCtx, method)
		assert.ErrorIs(t, err, errs.NoPermission, method)
		assert.False(t, called, method)
	}

	// full-сессия ограничений не имеет
	called, err := call(fullCtx, "/kusec_v1.Item/Create")
	require.NoError(t, err)
	assert.True(t, called)
}
