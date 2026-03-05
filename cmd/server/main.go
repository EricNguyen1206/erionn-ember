package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/EricNguyen1206/erion-ember/internal/cache"
	"github.com/EricNguyen1206/erion-ember/internal/embedding"
	"github.com/EricNguyen1206/erion-ember/internal/index"
	"github.com/EricNguyen1206/erion-ember/internal/server"
)

func main() {
	cfg := loadConfig()
	httpAddr := ":" + getEnv("HTTP_PORT", "8080")

	slog.Info("starting erion-ember",
		"version", "3.0.0",
		"addr", httpAddr,
		"dim", cfg.Dim,
		"threshold", cfg.SimilarityThreshold,
		"max_elements", cfg.MaxElements,
	)

	// Embedding: ZeroEmbedder until ONNX is configured via MODEL_DIR.
	// Exact hash-match path (fast path) works fully without ONNX.
	// Semantic similarity search requires ONNX model.
	var embedder embedding.Embedder = embedding.NewZeroEmbedder(cfg.Dim)
	if modelDir := os.Getenv("MODEL_DIR"); modelDir != "" {
		slog.Info("MODEL_DIR set — plug in hugot ONNX embedder here", "dir", modelDir)
		// TODO: embedder = embedding.NewONNXEmbedder(modelDir)
	} else {
		slog.Warn("MODEL_DIR not set — semantic similarity search disabled, exact-match only")
	}

	vidx := index.NewFlatIndex(cfg.Dim)
	sc := cache.New(cfg, embedder, vidx)
	defer sc.Close()

	srv := server.NewHTTPServer(httpAddr, sc)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("HTTP server ready", "addr", httpAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "err", err)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down gracefully...")
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
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
	if v := getEnv("EMBED_WORKERS", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.EmbedWorkers = n
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
