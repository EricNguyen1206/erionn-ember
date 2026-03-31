package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gomemkv/internal/pubsub"
	"gomemkv/internal/server"
	"gomemkv/internal/store"
)

const shutdownTimeout = 5 * time.Second

type Config struct {
	svrPort string
}

func main() {
	if err := run(); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := loadConfig()
	svrAddr := ":" + cfg.svrPort

	kvStore := store.New()
	hub := pubsub.New(16)

	slog.Info("starting gomemkv",
		"version", "4.0.0",
		"mode", "svr-data-cache",
	)

	cacheSvr, err := server.NewServer(svrAddr, kvStore, hub)
	if err != nil {
		return fmt.Errorf("create svr server: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)

	go func() {
		slog.Info("svr server ready", "addr", cacheSvr.Addr().String())
		if err := cacheSvr.Serve(); err != nil {
			errCh <- fmt.Errorf("svr server: %w", err)
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

	svrStopped := make(chan struct{})
	go func() {
		cacheSvr.GracefulStop()
		close(svrStopped)
	}()

	select {
	case <-svrStopped:
	case <-shutdownCtx.Done():
		cacheSvr.Stop()
	}

	var shutdownErr error

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
		svrPort: getEnv("PORT", "9090"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
