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

	"gomemkv/internal/config"
	"gomemkv/internal/pubsub"
	"gomemkv/internal/server"
	"gomemkv/internal/store"
)

const shutdownTimeout = 5 * time.Second

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Default()
	cfg.OverrideFromEnv()
	svrAddr := ":" + cfg.Port

	kvStore := store.New()
	hub := pubsub.New(16)

	slog.Info("starting gomemkv",
		"version", version,
		"commit", commit,
		"built", date,
	)

	cacheSvr, err := server.NewServer(svrAddr, kvStore, hub, server.ServerConfig{
		MaxConns:    cfg.MaxConns,
		IdleTimeout: cfg.IdleTimeout,
	})
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)

	go func() {
		slog.Info("server ready", "addr", cacheSvr.Addr().String())
		if err := cacheSvr.Serve(); err != nil {
			errCh <- fmt.Errorf("server: %w", err)
		}
	}()

	var serveErr error
	select {
	case <-ctx.Done():
		slog.Info("shutting down gracefully...")
	case serveErr = <-errCh:
		stop()
		slog.Error("runtime error", "err", serveErr)
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
