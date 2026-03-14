package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/EricNguyen1206/erion-ember/internal/cache"
	"github.com/EricNguyen1206/erion-ember/internal/server"
)

const shutdownTimeout = 5 * time.Second

func main() {
	if err := run(); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := loadConfig()
	httpAddr := ":" + getEnv("HTTP_PORT", "8080")
	grpcAddr := ":" + getEnv("GRPC_PORT", "9090")

	slog.Info("starting erion-ember",
		"version", "3.0.0",
		"http_addr", httpAddr,
		"grpc_addr", grpcAddr,
		"http_metrics_path", "/metrics",
		"similarity_threshold", cfg.SimilarityThreshold,
		"max_elements", cfg.MaxElements,
		"engine", "bm25-jaccard",
	)

	sc := cache.New(cfg)
	httpSrv := server.NewHTTPServer(httpAddr, sc)
	grpcSrv, err := server.NewGRPCServer(grpcAddr, sc)
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

	stats := sc.Stats()
	slog.Info("cache stats",
		"total_entries", stats.TotalEntries,
		"cache_hits", stats.CacheHits,
		"cache_misses", stats.CacheMisses,
		"total_queries", stats.TotalQueries,
		"hit_rate", stats.HitRate,
	)

	return errors.Join(serveErr, shutdownErr)
}

func loadConfig() cache.Config {
	cfg := cache.DefaultConfig()
	if v := getEnv("CACHE_SIMILARITY_THRESHOLD", ""); v != "" {
		if f, err := strconv.ParseFloat(v, 32); err == nil {
			cfg.SimilarityThreshold = float32(f)
		}
	}
	if v := getEnv("CACHE_MAX_ELEMENTS", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxElements = n
		}
	}
	if v := getEnv("CACHE_DEFAULT_TTL", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.DefaultTTL = time.Duration(n) * time.Second
		}
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
