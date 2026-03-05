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

	// ── Embedder selection (priority order) ─────────────────────────────────
	//
	//  1. MODEL_DIR set → ONNX in-process (hugot, fastest, no HTTP, ~2ms)
	//  2. OLLAMA_URL set → Ollama HTTP (pure Go, ~15ms per embed)
	//  3. Neither set   → ZeroEmbedder (exact-match only, 0ms)
	//
	var embedder embedding.Embedder

	switch {
	case getEnv("MODEL_DIR", "") != "":
		modelDir := getEnv("MODEL_DIR", "")
		slog.Info("loading ONNX embedder (in-process)", "dir", modelDir)
		onnx, err := embedding.NewONNXEmbedder(modelDir)
		if err != nil {
			slog.Error("ONNX embedder failed, falling back to ZeroEmbedder", "err", err)
			embedder = embedding.NewZeroEmbedder(cfg.Dim)
		} else {
			slog.Info("ONNX ready", "model", "all-MiniLM-L6-v2", "dim", onnx.Dim())
			cfg.Dim = onnx.Dim()
			embedder = onnx
			defer onnx.Close()
		}

	case getEnv("OLLAMA_URL", "") != "":
		ollamaURL := getEnv("OLLAMA_URL", "")
		model := getEnv("OLLAMA_MODEL", "nomic-embed-text")
		slog.Info("using Ollama HTTP embedder", "url", ollamaURL, "model", model)
		ollama, err := embedding.NewOllamaEmbedder(ollamaURL, model)
		if err != nil {
			slog.Error("Ollama unavailable, falling back to ZeroEmbedder", "err", err)
			embedder = embedding.NewZeroEmbedder(cfg.Dim)
		} else {
			slog.Info("Ollama ready", "dim", ollama.Dim())
			cfg.Dim = ollama.Dim()
			embedder = ollama
		}

	default:
		slog.Warn("no embedder configured — semantic similarity disabled (exact-match only)",
			"hint", "set MODEL_DIR or OLLAMA_URL in .env to enable")
		embedder = embedding.NewZeroEmbedder(cfg.Dim)
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
