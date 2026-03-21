package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quantum-cluster/go-proj1/internal/config"
	"github.com/quantum-cluster/go-proj1/internal/interceptor"
	"github.com/quantum-cluster/go-proj1/internal/repository"
	"github.com/quantum-cluster/go-proj1/internal/service/identity"
	pb "github.com/quantum-cluster/go-proj1/protos/identity/v1"
)

func main() {
	// Initialize Logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Handle Graceful Shutdown Context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, logger); err != nil {
		logger.Error("Application failed", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	// Load Configuration
	cfg := config.Load()

	// Initialize Interceptors
	interceptors := []grpc.UnaryServerInterceptor{
		interceptor.UnaryRecoveryInterceptor(logger),
		interceptor.UnaryLoggingInterceptor(logger),
		interceptor.AuthInterceptor(logger),
	}
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(interceptors...),
	)

	// Initialize Database Pool
	ctxInit, cancelInit := context.WithTimeout(ctx, 5*time.Second)
	defer cancelInit()

	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	poolConfig.MaxConns = 50
	poolConfig.MinConns = 10
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctxInit, poolConfig)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Initialize Repository
	userRepo := repository.NewPgUserRepository(pool)

	// Initialize Service
	identityService := identity.NewService(userRepo, cfg, logger)

	// Register Service
	pb.RegisterIdentityServiceServer(grpcServer, identityService)

	// Initialize Health Server
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("IdentityService", grpc_health_v1.HealthCheckResponse_SERVING)

	// Start Listening
	listener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	wg.Go(func() {
		logger.Info("Starting server on port :" + cfg.GRPCPort)
		if err := grpcServer.Serve(listener); err != nil {
			logger.Error("Terminating server", slog.Any("error", err))
		}
	})

	<-ctx.Done()
	logger.Info("Context expired. Exiting gracefully...")

	// Create a timeout context for the graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Use a goroutine and a channel to enforce the shutdown timeout
	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-shutdownCtx.Done():
		logger.Warn("Graceful shutdown timed out, forcing stop")
		grpcServer.Stop()
	case <-stopped:
		logger.Info("Graceful shutdown complete")
	}

	wg.Wait()
	logger.Info("Process complete")
	return nil
}
