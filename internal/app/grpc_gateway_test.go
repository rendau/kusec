package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/rendau/kusec/internal/constant"
	sessionService "github.com/rendau/kusec/internal/domain/session/service"
	"github.com/rendau/kusec/internal/errs"
	proto "github.com/rendau/kusec/pkg/proto/kusec_v1"
)

// usrStub — gRPC-сервис Usr с одним методом Login: возвращает заданную
// ошибку либо успех.
type usrStub struct {
	proto.UnimplementedUsrServer
	loginErr error
}

func (s *usrStub) Login(context.Context, *proto.UsrLoginReq) (*proto.UsrLoginRep, error) {
	if s.loginErr != nil {
		return nil, s.loginErr
	}
	return &proto.UsrLoginRep{Jwt: "ok"}, nil
}

// newTestGateway собирает путь запроса как в проде: HTTP-шлюз → gRPC-сервер
// с интерсепторами request id и ошибок (соединение — в памяти).
func newTestGateway(t *testing.T, usr proto.UsrServer) http.Handler {
	t.Helper()

	return newTestGatewayWith(t, usr, []grpc.UnaryServerInterceptor{
		GrpcInterceptorRequestId(),
		GrpcInterceptorError(),
	})
}

// newTestGatewayWith — то же с заданной цепочкой интерсепторов.
func newTestGatewayWith(t *testing.T, usr proto.UsrServer, interceptors []grpc.UnaryServerInterceptor) http.Handler {
	t.Helper()

	lis := bufconn.Listen(1 << 20)

	grpcSrv := grpc.NewServer(grpc.ChainUnaryInterceptor(interceptors...))
	proto.RegisterUsrServer(grpcSrv, usr)
	go func() { _ = grpcSrv.Serve(lis) }()
	t.Cleanup(grpcSrv.Stop)

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	handler, err := GrpcGatewayCreateHandler(func(mux *runtime.ServeMux) error {
		return proto.RegisterUsrHandler(context.Background(), mux, conn)
	})
	require.NoError(t, err)

	return handler
}

type gatewayErrorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields"`
}

func postLogin(t *testing.T, handler http.Handler, body io.Reader, headers map[string]string) (*httptest.ResponseRecorder, gatewayErrorBody) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/usr/login", body)
	req.Header.Set("Content-Type", "application/json")
	for name, value := range headers {
		req.Header.Set(name, value)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var parsed gatewayErrorBody
	if rec.Code != http.StatusOK {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &parsed), rec.Body.String())
	}

	return rec, parsed
}

// Текст ошибок Postgres и Kubernetes клиенту не уходит: вместо него — общий
// код и request id, по которому ошибка находится в логе.
func TestGateway_InternalErrorNotLeaked(t *testing.T) {
	const pgText = `ERROR: relation "usr" does not exist (SQLSTATE 42P01)`

	handler := newTestGateway(t, &usrStub{
		loginErr: fmt.Errorf("svc.GetByUsername: %w", fmt.Errorf("repoDb.GetByUsername: %w", errors.New(pgText))),
	})

	// request id сгенерирован сервером
	rec, body := postLogin(t, handler, strings.NewReader(`{"username":"a","password":"b"}`), nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, errs.ServiceNA.Error(), body.Code)
	assert.Regexp(t, `^internal error \(request id: [0-9a-f-]{36}\)$`, body.Message)
	for _, leak := range []string{"relation", "SQLSTATE", "repoDb", "svc."} {
		assert.NotContains(t, rec.Body.String(), leak)
	}

	// request id прислан клиентом — он же в ответе
	rec, body = postLogin(t, handler, strings.NewReader(`{}`), map[string]string{
		"Grpc-Metadata-X-Request-Id": "req-123",
	})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "internal error (request id: req-123)", body.Message)
}

// Сбой проверки токена (база недоступна) доходит до клиента обычной ошибкой
// API — service_not_available, без текста сбоя, — а не как not_authorized:
// иначе клиент выбросил бы рабочий токен. Проверяется настоящая цепочка
// интерсепторов сервера (grpcInterceptors): сессия в ней стоит после
// интерсептора ошибок, поэтому отказ форматируется как любой другой.
func TestGateway_SessionCheckFailure(t *testing.T) {
	const pgText = `failed to connect to host=pg user=kusec: connection refused (SQLSTATE 08006)`

	sessionSvc := sessionService.New("test-secret")
	usr := &usrStub{}

	gateway := func(usrAuth, keyAuth *authMock) http.Handler {
		return newTestGatewayWith(t, usr, grpcInterceptors(sessionSvc, keyAuth, usrAuth))
	}

	for name, tc := range map[string]struct {
		token   string
		usrAuth *authMock
		keyAuth *authMock
	}{
		"access-токен": {
			token:   "jwt-token",
			usrAuth: &authMock{err: fmt.Errorf("svc.Get: %w", errors.New(pgText))},
			keyAuth: &authMock{},
		},
		"api-ключ": {
			token:   constant.ApiKeyPrefix + "abc",
			usrAuth: &authMock{},
			keyAuth: &authMock{err: fmt.Errorf("svc.GetByKeyHash: %w", errors.New(pgText))},
		},
	} {
		t.Run(name, func(t *testing.T) {
			handler := gateway(tc.usrAuth, tc.keyAuth)

			rec, body := postLogin(t, handler, strings.NewReader(`{}`), map[string]string{
				"Authorization": "Bearer " + tc.token,
			})
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Equal(t, "service_not_available", body.Code)
			// обычный формат ошибки API: текст — тот же код, а не «rpc error:
			// code = Unknown…», каким он был бы мимо интерсептора ошибок
			assert.Equal(t, "service_not_available", body.Message)
			assert.Empty(t, body.Fields)
			for _, leak := range []string{"connect", "SQLSTATE", "svc.", "host=pg"} {
				assert.NotContains(t, rec.Body.String(), leak)
			}
		})
	}

	// без токена и с непринятым токеном запрос идёт дальше, как раньше
	handler := gateway(&authMock{err: errs.NotAuthorized}, &authMock{err: errs.NotAuthorized})

	rec, _ := postLogin(t, handler, strings.NewReader(`{}`), nil)
	assert.Equal(t, http.StatusOK, rec.Code, "без токена")

	rec, _ = postLogin(t, handler, strings.NewReader(`{}`), map[string]string{"Authorization": "Bearer stale-token"})
	assert.Equal(t, http.StatusOK, rec.Code, "токен не принят — запрос идёт как неавторизованный")
}

// Код из internal/errs доходит до клиента без цепочки обёрток; ErrFull — с
// описанием и полями, которые написаны для клиента.
func TestGateway_SemanticErrors(t *testing.T) {
	t.Run("sentinel", func(t *testing.T) {
		handler := newTestGateway(t, &usrStub{
			loginErr: fmt.Errorf("svc.Get: %w", fmt.Errorf("repo.Get: %w", errs.ObjectNotFound)),
		})

		rec, body := postLogin(t, handler, strings.NewReader(`{}`), nil)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, "object_not_found", body.Code)
		assert.Equal(t, "object_not_found", body.Message)
		assert.NotContains(t, rec.Body.String(), "svc.Get")
	})

	t.Run("full", func(t *testing.T) {
		handler := newTestGateway(t, &usrStub{
			loginErr: fmt.Errorf("svc.Create: %w", errs.ErrFull{
				Err:    errs.UsernameExists,
				Desc:   `username "bob" is already taken`,
				Fields: map[string]string{"username": "taken"},
			}),
		})

		rec, body := postLogin(t, handler, strings.NewReader(`{}`), nil)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, "username_exists", body.Code)
		assert.Equal(t, `username "bob" is already taken`, body.Message)
		assert.Equal(t, map[string]string{"username": "taken"}, body.Fields)
		assert.NotContains(t, rec.Body.String(), "svc.Create")
	})
}

// Прямой gRPC-клиент видит текст статуса — он тоже без внутренностей.
func TestGrpcInterceptorError_StatusText(t *testing.T) {
	interceptor := GrpcInterceptorError()
	info := &grpc.UnaryServerInfo{FullMethod: "/kusec_v1.Usr/Login"}

	call := func(handlerErr error) (*status.Status, *proto.ErrorRep) {
		_, err := interceptor(context.Background(), nil, info, func(context.Context, any) (any, error) {
			return nil, handlerErr
		})
		require.Error(t, err)

		st, ok := status.FromError(err)
		require.True(t, ok)
		require.Equal(t, codes.InvalidArgument, st.Code())
		require.Len(t, st.Details(), 1)

		rep, ok := st.Details()[0].(*proto.ErrorRep)
		require.True(t, ok)

		return st, rep
	}

	// неизвестная ошибка: ни в статусе, ни в деталях нет её текста
	st, rep := call(fmt.Errorf("kube.Apply: %w", errors.New(`secrets "db" is forbidden: User "system:serviceaccount:kusec:kusec" cannot patch`)))
	assert.Equal(t, "service_not_available: internal error", st.Message())
	assert.Equal(t, "service_not_available", rep.Code)
	assert.Equal(t, "internal error", rep.Message)

	// код из internal/errs
	st, rep = call(fmt.Errorf("usc: %w", errs.NoPermission))
	assert.Equal(t, "no_permission", st.Message())
	assert.Equal(t, "no_permission", rep.Code)
	assert.Equal(t, "no_permission", rep.Message)

	// ErrFull: описание — для клиента
	st, rep = call(errs.ErrFull{Err: errs.InvalidRequest, Desc: "slug is required"})
	assert.Equal(t, "invalid_request: slug is required", st.Message())
	assert.Equal(t, "invalid_request", rep.Code)
	assert.Equal(t, "slug is required", rep.Message)

	// в ErrFull вложена не ошибка из internal/errs — её текст кодом не становится
	_, rep = call(errs.ErrFull{Err: errors.New("pq: connection refused 10.0.0.5:5432"), Desc: "try later"})
	assert.Equal(t, "service_not_available", rep.Code)
	assert.Equal(t, "try later", rep.Message)

	// без ошибки интерсептор ничего не меняет
	resp, err := interceptor(context.Background(), nil, info, func(context.Context, any) (any, error) {
		return "ok", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

// Предел входящего gRPC-сообщения: больше прежнего умолчания в 4 МиБ
// проходит, больше 16 МиБ — отказ ещё до handler-а.
func TestGrpcServerOptions_MaxRecvMsgSize(t *testing.T) {
	lis := bufconn.Listen(1 << 20)

	stub := &usrStub{}
	grpcSrv := grpc.NewServer(grpcServerOptions(nil)...)
	proto.RegisterUsrServer(grpcSrv, stub)
	go func() { _ = grpcSrv.Serve(lis) }()
	t.Cleanup(grpcSrv.Stop)

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	client := proto.NewUsrClient(conn)

	_, err = client.Login(t.Context(), &proto.UsrLoginReq{Username: strings.Repeat("a", 9<<20)})
	require.NoError(t, err)

	_, err = client.Login(t.Context(), &proto.UsrLoginReq{Username: strings.Repeat("a", grpcMaxRecvMsgBytes+1)})
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
}

// grpc reflection (список сервисов и схемы сообщений) — только при DEBUG.
func TestRegisterGrpcReflection(t *testing.T) {
	hasReflection := func(debug bool) bool {
		srv := grpc.NewServer()
		registerGrpcReflection(srv, debug)

		for name := range srv.GetServiceInfo() {
			if strings.HasPrefix(name, "grpc.reflection.") {
				return true
			}
		}
		return false
	}

	assert.False(t, hasReflection(false))
	assert.True(t, hasReflection(true))
}

// repeatReader отдаёт n одинаковых байт, не держа их в памяти.
type repeatReader struct {
	b byte
	n int64
}

func (r *repeatReader) Read(p []byte) (int, error) {
	if r.n <= 0 {
		return 0, io.EOF
	}
	n := int(min(int64(len(p)), r.n))
	for i := range n {
		p[i] = r.b
	}
	r.n -= int64(n)
	return n, nil
}

// loginBody — тело логина с username длиной usernameLen; длина тела не
// заявлена (как при chunked-передаче).
func loginBody(usernameLen int64) io.Reader {
	return io.MultiReader(
		strings.NewReader(`{"username":"`),
		&repeatReader{b: 'a', n: usernameLen},
		strings.NewReader(`"}`),
	)
}

func TestGateway_BodyLimit(t *testing.T) {
	handler := newTestGateway(t, &usrStub{})

	assertTooLarge := func(t *testing.T, rec *httptest.ResponseRecorder, body gatewayErrorBody) {
		t.Helper()
		assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		assert.Equal(t, "invalid_request", body.Code)
		assert.Equal(t, "request body is too large: the limit is 8 MiB", body.Message)
	}

	t.Run("в пределах — проходит", func(t *testing.T) {
		rec, _ := postLogin(t, handler, loginBody(1<<20), nil)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"jwt":"ok"`)
	})

	t.Run("заявленная длина больше предела — отказ до чтения", func(t *testing.T) {
		body := &repeatReader{b: 'a', n: 16}

		req := httptest.NewRequest(http.MethodPost, "/usr/login", body)
		req.ContentLength = httpMaxBodyBytes + 1
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		var parsed gatewayErrorBody
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &parsed), rec.Body.String())
		assertTooLarge(t, rec, parsed)
		assert.Equal(t, int64(16), body.n, "тело не должно читаться")
	})

	t.Run("длина не заявлена — обрыв на пределе", func(t *testing.T) {
		rec, body := postLogin(t, handler, loginBody(httpMaxBodyBytes+1024), nil)
		assertTooLarge(t, rec, body)
	})

	t.Run("запрос без тела не затрагивается", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/usr/bootstrap/status", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		// метод в заглушке не реализован — важно, что это не 413
		assert.NotEqual(t, http.StatusRequestEntityTooLarge, rec.Code)
	})
}

func TestWithBodyLimit_ReadStopsAtLimit(t *testing.T) {
	var (
		read    int
		readErr error
		tooBig  bool
	)
	handler := withBodyLimit(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		read, readErr = len(body), err
		tooBig = bodyLimitExceeded(r)
	}), 16)

	// длина не заявлена, тело больше предела: читается не больше предела
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", &repeatReader{b: 'x', n: 1000}))
	assert.Equal(t, 16, read)
	_, isMaxBytes := errors.AsType[*http.MaxBytesError](readErr)
	assert.True(t, isMaxBytes)
	assert.True(t, tooBig)

	// тело в пределах: читается целиком, отметки нет
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", &repeatReader{b: 'x', n: 16}))
	assert.Equal(t, 16, read)
	require.NoError(t, readErr)
	assert.False(t, tooBig)
}
