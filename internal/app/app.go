package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/mechta-market/kusec/internal/config"
	"github.com/mechta-market/kusec/internal/constant"

	apikeyDb "github.com/mechta-market/kusec/internal/domain/apikey/repo/db"
	apikeyService "github.com/mechta-market/kusec/internal/domain/apikey/service"
	appDb "github.com/mechta-market/kusec/internal/domain/app/repo/db"
	appService "github.com/mechta-market/kusec/internal/domain/app/service"
	configitemDb "github.com/mechta-market/kusec/internal/domain/configitem/repo/db"
	configitemService "github.com/mechta-market/kusec/internal/domain/configitem/service"
	configmapDb "github.com/mechta-market/kusec/internal/domain/configmap/repo/db"
	configmapService "github.com/mechta-market/kusec/internal/domain/configmap/service"
	itemDb "github.com/mechta-market/kusec/internal/domain/item/repo/db"
	itemService "github.com/mechta-market/kusec/internal/domain/item/service"
	secretDb "github.com/mechta-market/kusec/internal/domain/secret/repo/db"
	secretService "github.com/mechta-market/kusec/internal/domain/secret/service"
	sessionService "github.com/mechta-market/kusec/internal/domain/session/service"
	usrDb "github.com/mechta-market/kusec/internal/domain/usr/repo/db"
	usrService "github.com/mechta-market/kusec/internal/domain/usr/service"

	grpcHandler "github.com/mechta-market/kusec/internal/handler/grpc"
	mcpHandler "github.com/mechta-market/kusec/internal/handler/mcp"

	kubeService "github.com/mechta-market/kusec/internal/service/kube"

	apikeyUsc "github.com/mechta-market/kusec/internal/usecase/apikey"
	appUsc "github.com/mechta-market/kusec/internal/usecase/app"
	configitemUsc "github.com/mechta-market/kusec/internal/usecase/configitem"
	configmapUsc "github.com/mechta-market/kusec/internal/usecase/configmap"
	dashboardUsc "github.com/mechta-market/kusec/internal/usecase/dashboard"
	itemUsc "github.com/mechta-market/kusec/internal/usecase/item"
	kubeUsc "github.com/mechta-market/kusec/internal/usecase/kube"
	secretUsc "github.com/mechta-market/kusec/internal/usecase/secret"
	transferUsc "github.com/mechta-market/kusec/internal/usecase/transfer"
	usrUsc "github.com/mechta-market/kusec/internal/usecase/usr"

	proto "github.com/mechta-market/kusec/pkg/proto/kusec_v1"
)

type App struct {
	globalTracerCloser io.Closer

	pgpool *pgxpool.Pool

	grpcServer       *GrpcServer
	httpServer       *http.Server
	mcpHttpServer    *http.Server
	systemHttpServer *http.Server

	ctx       context.Context
	ctxCancel context.CancelFunc

	usrSvc *usrService.Service

	exitCode int
}

func (a *App) Init() {
	var err error

	a.ctx, a.ctxCancel = context.WithCancel(context.Background())

	// logger
	initLogger(config.Conf.Debug, config.Conf.LogLevel)

	// globalTracer
	{
		if config.Conf.WithTracing && config.Conf.JaegerAddress != "" {
			slog.Info("tracing enabled")
			_, a.globalTracerCloser, err = tracerInitGlobal(config.Conf.JaegerAddress, constant.ServiceName)
			errCheck(err, "tracerInitGlobal")
		}
	}

	// pgpool
	a.pgpool, err = initPgPool(config.Conf.PgDsn)
	errCheck(err, "pgpool init")

	// migrations
	{
		runMigrations()
		slog.Info("PG-migrations have been successfully applied")
	}

	// session service (stateless HS256 JWT)
	sessionSvc := sessionService.New(config.Conf.JWTSecret)

	// dependency graph
	usrSvc := usrService.New(usrDb.New(a.pgpool))
	a.usrSvc = usrSvc
	appSvc := appService.New(appDb.New(a.pgpool))
	secretSvc := secretService.New(secretDb.New(a.pgpool))
	itemSvc := itemService.New(itemDb.New(a.pgpool))
	configMapSvc := configmapService.New(configmapDb.New(a.pgpool))
	configItemSvc := configitemService.New(configitemDb.New(a.pgpool))
	apikeySvc := apikeyService.New(apikeyDb.New(a.pgpool))

	// api-key usecase используется и хендлером, и session-интерсептором
	apikeyUsecase := apikeyUsc.New(apikeySvc, usrSvc, sessionSvc)

	usrHandler := grpcHandler.NewUsr(usrUsc.New(usrSvc, sessionSvc))
	appHandler := grpcHandler.NewApp(appUsc.New(appSvc, sessionSvc))
	secretHandler := grpcHandler.NewSecret(secretUsc.New(secretSvc, appSvc, sessionSvc))
	itemHandler := grpcHandler.NewItem(itemUsc.New(itemSvc, secretSvc, sessionSvc))
	configMapHandler := grpcHandler.NewConfigMap(configmapUsc.New(configMapSvc, appSvc, sessionSvc))
	configItemHandler := grpcHandler.NewConfigItem(configitemUsc.New(configItemSvc, configMapSvc, sessionSvc))
	dashboardHandler := grpcHandler.NewDashboard(
		dashboardUsc.New(appSvc, secretSvc, itemSvc, configMapSvc, configItemSvc, usrSvc, sessionSvc),
	)
	kubeHandler := grpcHandler.NewKube(
		kubeUsc.New(
			kubeService.New(appSvc, secretSvc, itemSvc, configMapSvc, configItemSvc),
			appSvc,
			secretSvc,
			configMapSvc,
			sessionSvc,
		),
	)
	transferHandler := grpcHandler.NewTransfer(
		transferUsc.New(appSvc, secretSvc, itemSvc, configMapSvc, configItemSvc, sessionSvc),
	)
	apikeyHandler := grpcHandler.NewApiKey(apikeyUsecase)

	// grpc server
	{
		a.grpcServer = NewGrpcServer("main", sessionSvc, apikeyUsecase, func(server *grpc.Server) {
			proto.RegisterUsrServer(server, usrHandler)
			proto.RegisterAppServer(server, appHandler)
			proto.RegisterSecretServer(server, secretHandler)
			proto.RegisterItemServer(server, itemHandler)
			proto.RegisterConfigMapServer(server, configMapHandler)
			proto.RegisterConfigItemServer(server, configItemHandler)
			proto.RegisterDashboardServer(server, dashboardHandler)
			proto.RegisterKubeServer(server, kubeHandler)
			proto.RegisterTransferServer(server, transferHandler)
			proto.RegisterApiKeyServer(server, apikeyHandler)
		})
	}

	// http-gw server
	{
		var grpcGwHandler http.Handler

		grpcGwHandler, err = GrpcGatewayCreateHandler(func(mux *runtime.ServeMux) error {
			opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

			var conn *grpc.ClientConn
			conn, err = grpc.NewClient("localhost:"+config.Conf.GrpcPort, opts...)
			errCheck(err, "grpc.Dial")

			// register grpc handlers
			handlers := []func(context.Context, *runtime.ServeMux, *grpc.ClientConn) error{
				proto.RegisterUsrHandler,
				proto.RegisterAppHandler,
				proto.RegisterSecretHandler,
				proto.RegisterItemHandler,
				proto.RegisterConfigMapHandler,
				proto.RegisterConfigItemHandler,
				proto.RegisterDashboardHandler,
				proto.RegisterKubeHandler,
				proto.RegisterTransferHandler,
				proto.RegisterApiKeyHandler,
			}
			for _, h := range handlers {
				err = h(context.Background(), mux, conn)
				if err != nil {
					return fmt.Errorf("grpc-gateway: register grpc-handler: %w", err)
				}
			}

			// custom http handlers
			httpHandlers := []struct {
				method  string
				path    string
				handler runtime.HandlerFunc
			}{
				{
					"GET", "/tst",
					func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
						slog.Info("test error", "error", errors.New("test error"))
					},
				},
			}
			for _, h := range httpHandlers {
				err = mux.HandlePath(h.method, h.path, h.handler)
				if err != nil {
					return fmt.Errorf("grpc-gateway: register http-handler: %w", err)
				}
			}

			return nil
		})
		errCheck(err, "grpcGatewayCreateHandler")

		handler := http.NewServeMux()
		handler.Handle("/api", http.RedirectHandler("/api/", http.StatusMovedPermanently))
		handler.Handle("/api/", http.StripPrefix("/api", grpcGwHandler))
		handler.Handle("/", NewAdminSPAHandler())

		// server
		a.httpServer = &http.Server{
			Addr:              ":" + config.Conf.HttpPort,
			Handler:           handler,
			ReadHeaderTimeout: 2 * time.Second,
			ReadTimeout:       time.Minute,
			MaxHeaderBytes:    300 * 1024,
		}
	}

	// встроенный MCP-сервер для AI-агентов (отдельный порт, api-key auth)
	if config.Conf.McpEnabled {
		mcpH := mcpHandler.New(
			sessionSvc,
			apikeyUsecase,
			appUsc.New(appSvc, sessionSvc),
			secretUsc.New(secretSvc, appSvc, sessionSvc),
			itemUsc.New(itemSvc, secretSvc, sessionSvc),
			configmapUsc.New(configMapSvc, appSvc, sessionSvc),
			configitemUsc.New(configItemSvc, configMapSvc, sessionSvc),
		)

		a.mcpHttpServer = &http.Server{
			Addr:              ":" + config.Conf.McpPort,
			Handler:           mcpH.HTTPHandler(),
			ReadHeaderTimeout: 2 * time.Second,
			MaxHeaderBytes:    300 * 1024,
		}
	}

	// system http server (healthcheck, docs, metrics)
	{
		a.systemHttpServer = SystemHttpServerCreate()
	}
}

func (a *App) PreStartHook() {
	slog.Info("PreStartHook")
}

func (a *App) Start() {
	slog.Info("Starting")

	// grpc server
	{
		err := a.grpcServer.Start()
		errCheck(err, "grpcServer.Start")
	}

	// http-gw server
	{
		go func() {
			err := a.httpServer.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				// errCheck(err, "http-server stopped")
			}
		}()
		slog.Info("http-server started " + a.httpServer.Addr)
	}

	// mcp server
	if a.mcpHttpServer != nil {
		go func() {
			err := a.mcpHttpServer.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				// errCheck(err, "mcp-http-server stopped")
			}
		}()
		slog.Info("mcp-http-server started " + a.mcpHttpServer.Addr)
	}

	// system http server
	{
		go func() {
			err := a.systemHttpServer.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				// errCheck(err, "system-http-server stopped")
			}
		}()
		slog.Info("system-http-server started " + a.systemHttpServer.Addr)
	}
}

func (a *App) Listen() {
	signalCtx, signalCtxCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer signalCtxCancel()

	// wait signal
	<-signalCtx.Done()
}

func (a *App) Stop() {
	slog.Info("Shutting down...")

	// stop context
	a.ctxCancel()

	// http-gw server
	{
		ctx, ctxCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer ctxCancel()

		if err := a.httpServer.Shutdown(ctx); err != nil {
			slog.Error("http-server shutdown error", "error", err)
			a.exitCode = 1
		}
	}

	// mcp server
	if a.mcpHttpServer != nil {
		ctx, ctxCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer ctxCancel()

		if err := a.mcpHttpServer.Shutdown(ctx); err != nil {
			slog.Error("mcp-http-server shutdown error", "error", err)
			a.exitCode = 1
		}
	}

	// system http server
	{
		ctx, ctxCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer ctxCancel()

		if err := a.systemHttpServer.Shutdown(ctx); err != nil {
			slog.Error("system-http-server shutdown error", "error", err)
			a.exitCode = 1
		}
	}

	// grpc server
	a.grpcServer.Stop()
}

func (a *App) WaitJobs() {
	slog.Info("waiting jobs")
}

func (a *App) Exit() {
	slog.Info("Exit")

	if a.globalTracerCloser != nil {
		_ = a.globalTracerCloser.Close()
	}

	a.pgpool.Close()

	// flush stdout

	os.Exit(a.exitCode)
}

func errCheck(err error, msg string) {
	if err != nil {
		if msg != "" {
			err = fmt.Errorf("%s: %w", msg, err)
		}
		slog.Error(err.Error())
		os.Exit(1)
	}
}
