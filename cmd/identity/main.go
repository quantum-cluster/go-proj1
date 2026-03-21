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

	// Load Configuration
	cfg := config.Load()

	// Initialize Interceptors
	interceptors := []grpc.UnaryServerInterceptor{
		interceptor.UnaryLoggingInterceptor(logger),
		interceptor.AuthInterceptor,
	}
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(interceptors...),
	)

	// Initialize Database Pool
	ctxInit, cancelInit := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelInit()

	pool, err := pgxpool.New(ctxInit, cfg.DatabaseURL)
	if err != nil {
		logger.Error("Can't connect to DB", slog.Any("error", err))
		os.Exit(1)
	}
	defer pool.Close()

	// Initialize Repository
	userRepo := repository.NewPgUserRepository(pool)

	// Initialize Service
	identityService := identity.NewService(userRepo, cfg, logger)

	// Register Service
	pb.RegisterIdentityServiceServer(grpcServer, identityService)

	// Start Listening
	listener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		logger.Error("Can't listen", slog.Any("error", err))
		os.Exit(1)
	}

	// Handle Graceful Shutdown Context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
}
