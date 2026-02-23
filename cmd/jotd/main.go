package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	pagesgrpc "github.com/reggieanim/jot/internal/modules/pages/adapters/grpc"
	pageshttp "github.com/reggieanim/jot/internal/modules/pages/adapters/http"
	pagespostgres "github.com/reggieanim/jot/internal/modules/pages/adapters/postgres"
	pageapp "github.com/reggieanim/jot/internal/modules/pages/app"
	"github.com/reggieanim/jot/internal/platform/config"
	platformpostgres "github.com/reggieanim/jot/internal/platform/db/postgres"
	platformnats "github.com/reggieanim/jot/internal/platform/eventbus/nats"
	"github.com/reggieanim/jot/internal/platform/httputil"
	"github.com/reggieanim/jot/internal/platform/observability"
	platformgrpc "github.com/reggieanim/jot/internal/platform/realtime/grpc"
	platformstorage "github.com/reggieanim/jot/internal/platform/storage"
	"github.com/reggieanim/jot/internal/shared/clock"
	"go.uber.org/zap"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	logger, err := observability.NewLogger(cfg.LogLevel)
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	tracerProvider, err := observability.SetupTracer(ctx, cfg.AppName, cfg.OTLPEndpoint)
	if err != nil {
		logger.Fatal("setup tracer", zap.Error(err))
	}
	defer observability.ShutdownTracer(context.Background(), tracerProvider)

	pool, err := platformpostgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("connect postgres", zap.Error(err))
	}
	defer pool.Close()

	migrationsDir, err := platformpostgres.ResolveMigrationsDir(cfg.MigrationsDir)
	if err != nil {
		logger.Fatal("resolve migrations dir", zap.Error(err))
	}

	logger.Info("running database migrations")
	if err := platformpostgres.RunMigrations(ctx, pool.Pool, migrationsDir); err != nil {
		logger.Fatal("run migrations", zap.Error(err))
	}
	logger.Info("database migrations complete", zap.String("dir", migrationsDir))

	natsConn, jetstream, err := platformnats.Connect(cfg.NATSURL)
	if err != nil {
		logger.Fatal("connect nats", zap.Error(err))
	}
	defer natsConn.Close()

	if err := platformnats.EnsureStream(jetstream, cfg.NATSStream, cfg.NATSSubject); err != nil {
		logger.Fatal("ensure stream", zap.Error(err))
	}

	repo := pagespostgres.NewRepository(pool.Pool)
	events := platformnats.NewPageEventsPublisher(jetstream, cfg.NATSSubject)
	pagesService := pageapp.NewService(repo, events, clock.SystemClock{})
	mediaStore, err := platformstorage.NewS3MediaStore(cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Bucket, cfg.S3UseSSL, cfg.S3PublicURL)
	if err != nil {
		logger.Fatal("setup media store", zap.Error(err))
	}

	router := httputil.NewRouter(cfg.CORSOrigins)
	pageshttp.RegisterRoutes(router, pagesService, natsConn, cfg.NATSSubject, logger, mediaStore)

	httpServer := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	grpcServer := platformgrpc.NewServer()
	pagesgrpc.Register(grpcServer, pagesService, natsConn, cfg.NATSSubject, logger)
	grpcListener, err := platformgrpc.Listen(cfg.GRPCAddr)
	if err != nil {
		logger.Fatal("listen grpc", zap.Error(err))
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		logger.Info("http server started", zap.String("addr", cfg.HTTPAddr))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server error", zap.Error(err))
			stop()
		}
	}()

	go func() {
		defer wg.Done()
		logger.Info("grpc server started", zap.String("addr", cfg.GRPCAddr))
		if err := grpcServer.Serve(grpcListener); err != nil {
			logger.Error("grpc server error", zap.Error(err))
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutdown initiated")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	grpcServer.GracefulStop()
	wg.Wait()
	os.Exit(0)
}
