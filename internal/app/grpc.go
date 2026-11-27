package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"os"
	"runtime/debug"
	"strings"
	"time"

	otgrpc "github.com/opentracing-contrib/go-grpc"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"github.com/rendau/kusec/internal/config"
	"github.com/rendau/kusec/internal/constant"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	sessionService "github.com/rendau/kusec/internal/domain/session/service"
	"github.com/rendau/kusec/internal/errs"
	"github.com/rendau/kusec/internal/infra/metrics"
	"github.com/rendau/kusec/internal/util"
	proto "github.com/rendau/kusec/pkg/proto/kusec_v1"
)

// grpcMaxRecvMsgBytes — предел входящего gRPC-сообщения (было 4 ГБ: один
// запрос без входа мог занять всю память пода). Запросы админки и API идут
// через HTTP-шлюз, где тело режется ещё раньше (httpMaxBodyBytes); вдвое
// больший предел здесь — запас для прямых gRPC-клиентов. Предел на отправку
// (MaxSendMsgSize) не трогаем: размер ответа задаёт сам сервис.
const grpcMaxRecvMsgBytes = 16 << 20

type GrpcServer struct {
	name   string
	server *grpc.Server
}

func NewGrpcServer(
	name string,
	sessionSvc *sessionService.Service,
	apiKeyAuth ApiKeyAuthI,
	usrAuth UsrAuthI,
	register func(*grpc.Server),
) *GrpcServer {
	interceptors := grpcInterceptors(sessionSvc, apiKeyAuth, usrAuth)

	// server
	server := grpc.NewServer(grpcServerOptions(interceptors)...)

	// register handlers
	if register != nil {
		register(server)
	}

	// register grpc reflection (только при DEBUG)
	registerGrpcReflection(server, config.Conf.Debug)

	return &GrpcServer{
		name:   name,
		server: server,
	}
}

// grpcInterceptors — цепочка интерсепторов gRPC-сервера, от внешнего к
// внутреннему. Порядок важен: всё, что может ответить ошибкой само (сессия,
// read_only), стоит после error-интерсептора — иначе отказ ушёл бы клиенту
// мимо обычного формата ошибок API, а вместе с ним и сырой текст ошибки.
func grpcInterceptors(
	sessionSvc *sessionService.Service,
	apiKeyAuth ApiKeyAuthI,
	usrAuth UsrAuthI,
) []grpc.UnaryServerInterceptor {
	interceptors := make([]grpc.UnaryServerInterceptor, 0, 8)

	// ctx without cancel
	interceptors = append(interceptors, GrpcInterceptorCtxWithoutCancel())

	// request id (for audit)
	interceptors = append(interceptors, GrpcInterceptorRequestId())

	// recovery
	interceptors = append(interceptors, GrpcInterceptorRecovery())

	// error
	interceptors = append(interceptors, GrpcInterceptorError())

	// session (extract bearer token -> session in context); после
	// error-интерсептора: сбой проверки токена — обычная ошибка API
	interceptors = append(interceptors, GrpcInterceptorSession(sessionSvc, apiKeyAuth, usrAuth))

	// read_only scope (после error-интерсептора, чтобы отказ форматировался
	// как обычная ошибка API)
	interceptors = append(interceptors, GrpcInterceptorReadOnlyScope(sessionSvc))

	// tracing
	if config.Conf.WithTracing {
		interceptors = append(interceptors, GrpcInterceptorTracing())
	}

	// metrics
	if metrics.Enabled {
		interceptors = append(interceptors, GrpcInterceptorMetrics())
	}

	return interceptors
}

// grpcServerOptions — параметры gRPC-сервера: пределы размера сообщений и
// цепочка интерсепторов.
func grpcServerOptions(interceptors []grpc.UnaryServerInterceptor) []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.MaxSendMsgSize(math.MaxUint32),
		grpc.MaxRecvMsgSize(grpcMaxRecvMsgBytes),
		grpc.ChainUnaryInterceptor(interceptors...),
	}
}

// registerGrpcReflection включает grpc reflection только для отладки: он
// отдаёт любому подключившемуся список сервисов, методов и схемы сообщений.
func registerGrpcReflection(server *grpc.Server, debug bool) {
	if debug {
		reflection.Register(server)
	}
}

func (s *GrpcServer) Start() error {
	lis, err := net.Listen("tcp", ":"+config.Conf.GrpcPort)
	if err != nil {
		return fmt.Errorf("failed to listen grpc: %w", err)
	}
	go func() {
		err = s.server.Serve(lis)
		if err != nil {
			slog.Error(s.name+"-grpc-server stopped", "error", err)
			os.Exit(1)
		}
	}()
	slog.Info(s.name + "-grpc-server started " + lis.Addr().String())
	return nil
}

func (s *GrpcServer) Stop() {
	s.server.GracefulStop()
}

func GrpcInterceptorCtxWithoutCancel() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		return handler(context.WithoutCancel(ctx), req)
	}
}

// GrpcInterceptorRequestId кладёт request id в контекст: из metadata
// x-request-id (через gateway — заголовок Grpc-Metadata-X-Request-Id) либо
// генерирует новый. Попадает в записи аудита.
func GrpcInterceptorRequestId() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		id := ""
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if values := md.Get("x-request-id"); len(values) > 0 {
				id = strings.TrimSpace(values[0])
			}
		}
		if id == "" {
			id = util.NewRequestId()
		}
		return handler(util.RequestIdToContext(ctx, id), req)
	}
}

// ApiKeyAuthI — аутентификация по API-ключу (реализуется apikey usecase).
type ApiKeyAuthI interface {
	SessionFromKey(ctx context.Context, key string) (*sessionModel.Session, error)
}

// UsrAuthI — аутентификация по access-токену входа (реализуется usr
// usecase): сессия строится по живой записи пользователя, поэтому
// выключенный пользователь и отозванный токен отвергаются сразу, а права
// всегда актуальны.
type UsrAuthI interface {
	SessionFromToken(ctx context.Context, token string) (*sessionModel.Session, error)
}

// GrpcInterceptorSession кладёт в контекст сессию по bearer-токену: API-ключ
// (по префиксу) либо access-токен входа. И то и другое проверяется по БД на
// каждый запрос.
//
// Исходов три:
//   - токена нет либо он не принят (не тот, отозван, пользователь выключен) —
//     запрос идёт как неавторизованный, решение об отказе принимает usecase;
//   - токен принят — запрос идёт с сессией;
//   - проверка не удалась (БД недоступна) — запрос обрывается ошибкой
//     service_not_available. Продолжать как неавторизованный здесь нельзя:
//     клиент получил бы not_authorized, выбросил рабочий токен, и короткая
//     недоступность Postgres разлогинила бы всех.
//
// Должен стоять после GrpcInterceptorError: тот приводит ошибку к обычному
// ответу API.
func GrpcInterceptorSession(sessionSvc *sessionService.Service, apiKeyAuth ApiKeyAuthI, usrAuth UsrAuthI) grpc.UnaryServerInterceptor {
	// accept отличает отказ от сбоя проверки. Наружу сбой уходит без
	// подробностей — ровно errs.ServiceNA: в исходной ошибке бывает и текст
	// Postgres, и чужой код из internal/errs. Причина остаётся в журнале.
	accept := func(ctx context.Context, parsed *sessionModel.Session, authErr error, kind string) (*sessionModel.Session, error) {
		if authErr != nil {
			if errors.Is(authErr, errs.NotAuthorized) {
				return nil, nil
			}
			slog.Error("session check failed",
				"kind", kind,
				"error", authErr,
				"request_id", util.RequestIdFromContext(ctx),
			)
			return nil, errs.ServiceNA
		}
		if !parsed.IsAuthorized() {
			return nil, nil
		}
		return parsed, nil
	}

	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		var session *sessionModel.Session

		token := grpcExtractBearerToken(ctx)
		switch {
		case token == "":
		case strings.HasPrefix(token, constant.ApiKeyPrefix):
			if apiKeyAuth != nil {
				parsedSession, keyErr := apiKeyAuth.SessionFromKey(ctx, token)
				if session, err = accept(ctx, parsedSession, keyErr, "api_key"); err != nil {
					return nil, err
				}
			}
		default:
			if usrAuth != nil {
				parsedSession, tokenErr := usrAuth.SessionFromToken(ctx, token)
				if session, err = accept(ctx, parsedSession, tokenErr, "token"); err != nil {
					return nil, err
				}
			}
		}

		return handler(sessionSvc.WithContext(ctx, session), req)
	}
}

func grpcExtractBearerToken(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	values := md.Get("authorization")
	if len(values) == 0 {
		return ""
	}

	value := strings.TrimSpace(values[0])
	if value == "" {
		return ""
	}

	parts := strings.Fields(value)
	if len(parts) == 1 {
		return parts[0]
	}
	if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
		return parts[1]
	}

	return ""
}

// readOnlyMethodWhitelist — методы, разрешённые сессиям scope=read_only.
// Именно белый список, а не «всё, кроме мутаций»: новый метод по умолчанию
// закрыт. Значения item/config_item в ответах разрешённых методов
// дополнительно маскируются в usecase-слое. Kube.* закрыт целиком, включая
// GetClusterSecret/GetClusterConfigMap (отдают значения из кластера).
var readOnlyMethodWhitelist = map[string]bool{
	"/kusec_v1.App/List":        true,
	"/kusec_v1.App/Get":         true,
	"/kusec_v1.App/Resolve":     true,
	"/kusec_v1.App/Keys":        true,
	"/kusec_v1.App/Drift":       true,
	"/kusec_v1.Secret/List":     true,
	"/kusec_v1.Secret/Get":      true,
	"/kusec_v1.ConfigMap/List":  true,
	"/kusec_v1.ConfigMap/Get":   true,
	"/kusec_v1.Item/List":       true,
	"/kusec_v1.Item/Get":        true,
	"/kusec_v1.ConfigItem/List": true,
	"/kusec_v1.ConfigItem/Get":  true,
	"/kusec_v1.Audit/List":      true,
	"/kusec_v1.SyncRun/List":    true,
	"/kusec_v1.SyncRun/Get":     true,
	"/kusec_v1.Transfer/Tree":   true,
}

// GrpcInterceptorReadOnlyScope пропускает read_only-сессии только к методам
// из белого списка.
func GrpcInterceptorReadOnlyScope(sessionSvc *sessionService.Service) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if sessionSvc.FromContext(ctx).IsReadOnly() && !readOnlyMethodWhitelist[info.FullMethod] {
			return nil, errs.NoPermission
		}
		return handler(ctx, req)
	}
}

func GrpcInterceptorTracing() grpc.UnaryServerInterceptor {
	tracer := opentracing.GlobalTracer()

	return otgrpc.OpenTracingServerInterceptor(
		tracer,
		otgrpc.SpanDecorator(func(_ context.Context, span opentracing.Span, method string, req, resp any, err error) {
			if err != nil {
				span.SetTag("error", true)
			}
		}),
	)
}

func GrpcInterceptorRecovery() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error(
					"Recovered from grpc panic",
					slog.Any("error", recovered),
					slog.String("fullMethod", info.FullMethod),
					slog.Any("recovery_stacktrace", string(debug.Stack())),
				)
				err = status.Error(codes.Internal, "internal server error")
			}
		}()

		return handler(ctx, req)
	}
}

func GrpcInterceptorMetrics() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		start := time.Now()

		h, err := handler(ctx, req)

		responseStatus := "ok"
		if err != nil {
			responseStatus = "error"
		}

		metricRequestCounter.WithLabelValues("grpc", info.FullMethod, responseStatus).Inc()
		metricResponseDuration.WithLabelValues("grpc", info.FullMethod).Observe(time.Since(start).Seconds())

		return h, err
	}
}

// GrpcInterceptorError приводит ошибку handler-а к ответу API (ErrorRep в
// деталях статуса). Клиенту уходит только то, что написано для него:
//   - errs.ErrFull — код, Desc и Fields;
//   - errs.Err (в том числе обёрнутая) — только код, без цепочки обёрток;
//   - всё остальное (Postgres, Kubernetes, сеть) — service_not_available и
//     request id. Полный текст такой ошибки пишется только в лог: в нём
//     бывают имена таблиц и ограничений, адреса, куски запросов.
func GrpcInterceptorError() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		h, err := handler(ctx, req)
		if err == nil {
			return h, nil
		}

		requestId := util.RequestIdFromContext(ctx)
		ei, internal := grpcErrorRep(err, requestId)

		logAttrs := []any{
			slog.String("error", err.Error()),
			slog.String("method", info.FullMethod),
			slog.String("request_id", requestId),
		}
		if internal {
			slog.Error("GRPC handler internal error", logAttrs...)
		} else {
			slog.Info("GRPC handler error", logAttrs...)
		}

		// текст статуса видят прямые gRPC-клиенты — он тоже без внутренностей
		st := status.New(codes.InvalidArgument, grpcErrorStatusText(ei))
		if stWithDetails, detailsErr := st.WithDetails(ei); detailsErr != nil {
			slog.Error(
				"error while creating status with details",
				slog.String("error", detailsErr.Error()),
				slog.String("method", info.FullMethod),
				slog.String("request_id", requestId),
			)
		} else {
			st = stWithDetails
		}

		return h, st.Err()
	}
}

// grpcErrorRep — что из ошибки можно показать клиенту. internal=true —
// ошибка не из internal/errs: её текст клиенту не отдаётся совсем.
func grpcErrorRep(err error, requestId string) (rep *proto.ErrorRep, internal bool) {
	if fullErr, ok := errors.AsType[errs.ErrFull](err); ok {
		// код — только из internal/errs: вложенная в ErrFull произвольная
		// ошибка наружу тоже не уходит
		code := errs.ServiceNA
		if baseErr, ok := errors.AsType[errs.Err](fullErr.Err); ok {
			code = baseErr
		}
		return &proto.ErrorRep{
			Code:    code.Error(),
			Message: fullErr.Desc,
			Fields:  fullErr.Fields,
		}, false
	}

	if baseErr, ok := errors.AsType[errs.Err](err); ok {
		return &proto.ErrorRep{
			Code:    baseErr.Error(),
			Message: baseErr.Error(),
		}, false
	}

	message := "internal error"
	if requestId != "" {
		// по request id полная ошибка находится в логе сервера
		message += " (request id: " + requestId + ")"
	}
	return &proto.ErrorRep{
		Code:    errs.ServiceNA.Error(),
		Message: message,
	}, true
}

// grpcErrorStatusText — текст gRPC-статуса из уже очищенного ответа.
func grpcErrorStatusText(rep *proto.ErrorRep) string {
	if rep.Message == "" || rep.Message == rep.Code {
		return rep.Code
	}
	return rep.Code + ": " + rep.Message
}
