package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/EricNguyen1206/erion-ember/internal/pubsub"
	"github.com/EricNguyen1206/erion-ember/internal/server"
	"github.com/EricNguyen1206/erion-ember/internal/store"
)

const shutdownTimeout = 5 * time.Second

type Config struct {
	HTTPPort string
	GRPCPort string
}

func main() {
	if err := run(); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := loadConfig()
	httpAddr := ":" + cfg.HTTPPort
	grpcAddr := ":" + cfg.GRPCPort

	kvStore := store.New()
	hub := pubsub.New(16)

	slog.Info("starting erion-ember",
		"version", "4.0.0",
		"http_addr", httpAddr,
		"grpc_addr", grpcAddr,
		"http_metrics_path", "/metrics",
		"mode", "grpc-data-cache",
	)

	httpSrv := server.NewHTTPServer(httpAddr, kvStore, hub)
	grpcSrv, err := server.NewGRPCServer(grpcAddr, kvStore, hub)
	if err != nil {
		return fmt.Errorf("create gRPC server: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 2)

	go func() {
		slog.Info("HTTP server ready", "addr", httpAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http server: %w", err)
		}
	}()

	go func() {
		slog.Info("gRPC server ready", "addr", grpcSrv.Addr().String())
		if err := grpcSrv.Serve(); err != nil {
			errCh <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	var serveErr error
	select {
	case <-ctx.Done():
		slog.Info("shutting down gracefully...")
	case serveErr = <-errCh:
		stop()
		slog.Error("runtime server error", "err", serveErr)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	grpcStopped := make(chan struct{})
	go func() {
		grpcSrv.GracefulStop()
		close(grpcStopped)
	}()

	select {
	case <-grpcStopped:
	case <-shutdownCtx.Done():
		grpcSrv.Stop()
	}

	var shutdownErr error
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		shutdownErr = fmt.Errorf("shutdown http server: %w", err)
	}

	storeStats := kvStore.Stats()
	hubStats := hub.Stats()
	slog.Info("cache stats",
		"total_keys", storeStats.TotalKeys,
		"string_keys", storeStats.StringKeys,
		"hash_keys", storeStats.HashKeys,
		"list_keys", storeStats.ListKeys,
		"set_keys", storeStats.SetKeys,
		"pubsub_channels", hubStats.Channels,
		"pubsub_subscribers", hubStats.Subscribers,
	)

	return errors.Join(serveErr, shutdownErr)
}

func loadConfig() Config {
	return Config{
		HTTPPort: getEnv("HTTP_PORT", "8080"),
		GRPCPort: getEnv("GRPC_PORT", "9090"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
