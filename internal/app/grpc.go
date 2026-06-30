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
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	sessionService "github.com/rendau/kusec/internal/domain/session/service"
	"github.com/rendau/kusec/internal/errs"
	"github.com/rendau/kusec/internal/infra/metrics"
	proto "github.com/rendau/kusec/pkg/proto/kusec_v1"
)

type GrpcServer struct {
	name   string
	server *grpc.Server
}

func NewGrpcServer(name string, sessionSvc *sessionService.Service, register func(*grpc.Server)) *GrpcServer {
	interceptors := make([]grpc.UnaryServerInterceptor, 0, 6)

	// ctx without cancel
	interceptors = append(interceptors, GrpcInterceptorCtxWithoutCancel())

	// session (extract bearer token -> session in context)
	interceptors = append(interceptors, GrpcInterceptorSession(sessionSvc))

	// recovery
	interceptors = append(interceptors, GrpcInterceptorRecovery())

	// error
	interceptors = append(interceptors, GrpcInterceptorError())

	// tracing
	if config.Conf.WithTracing {
		interceptors = append(interceptors, GrpcInterceptorTracing())
	}

	// metrics
	if metrics.Enabled {
		interceptors = append(interceptors, GrpcInterceptorMetrics())
	}

	// server
	server := grpc.NewServer(
		grpc.MaxSendMsgSize(math.MaxUint32),
		grpc.MaxRecvMsgSize(math.MaxUint32),
		grpc.ChainUnaryInterceptor(interceptors...),
	)

	// register handlers
	if register != nil {
		register(server)
	}

	// register grpc reflection
	reflection.Register(server)

	return &GrpcServer{
		name:   name,
		server: server,
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

func GrpcInterceptorSession(sessionSvc *sessionService.Service) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		var session *sessionModel.Session

		token := grpcExtractBearerToken(ctx)
		if token != "" {
			parsedSession, parseErr := sessionSvc.FromToken(token)
			if parseErr == nil && parsedSession != nil && parsedSession.Id != 0 {
				session = parsedSession
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

func GrpcInterceptorError() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		h, err := handler(ctx, req)
		if err == nil {
			return h, nil
		}

		var ei *proto.ErrorRep
		errStr := err.Error()

		if fullErr, ok := errors.AsType[errs.ErrFull](err); ok {
			errCode := errs.ServiceNA.Error()
			if fullErr.Err != nil {
				errCode = fullErr.Err.Error()
			}
			ei = &proto.ErrorRep{
				Code:    errCode,
				Message: fullErr.Desc,
				Fields:  fullErr.Fields,
			}
		} else if baseErr, ok := errors.AsType[errs.Err](err); ok {
			ei = &proto.ErrorRep{
				Code:    baseErr.Error(),
				Message: errStr,
			}
		} else {
			ei = &proto.ErrorRep{
				Code:    errs.ServiceNA.Error(),
				Message: errStr,
			}
		}

		slog.Info(
			"GRPC handler error",
			slog.String("error", errStr),
			slog.String("method", info.FullMethod),
		)

		st := status.New(codes.InvalidArgument, errStr)
		st, err = st.WithDetails(ei)
		if err != nil {
			slog.Error(
				"error while creating status with details",
				slog.String("error", errStr),
				slog.String("method", info.FullMethod),
			)
			st = status.New(codes.InvalidArgument, errStr)
		}

		return h, st.Err()
	}
}
