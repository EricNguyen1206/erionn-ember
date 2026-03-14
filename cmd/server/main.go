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
	"strings"
	"syscall"
	"time"

	"github.com/EricNguyen1206/erion-ember/internal/cache"
	"github.com/EricNguyen1206/erion-ember/internal/server"
)

const shutdownTimeout = 5 * time.Second

var newONNXEmbedder = func(cfg cache.ONNXEmbedderConfig) (cache.Embedder, error) {
	return cache.NewONNXEmbedder(cfg)
}

type runtimeConfig struct {
	Engine string
	ONNX   *cache.ONNXEmbedderConfig
}

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
	sc, engine, err := newSemanticCache(cfg)
	if err != nil {
		return err
	}

	slog.Info("starting erion-ember",
		"version", "3.0.0",
		"http_addr", httpAddr,
		"grpc_addr", grpcAddr,
		"similarity_threshold", cfg.SimilarityThreshold,
		"max_elements", cfg.MaxElements,
		"engine", engine,
	)

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

func loadRuntimeConfig() (runtimeConfig, error) {
	modelPath := strings.TrimSpace(getEnv("CACHE_EMBEDDING_MODEL_PATH", ""))
	backend := strings.ToLower(strings.TrimSpace(getEnv("CACHE_EMBEDDER_BACKEND", "")))
	hasEmbeddingConfig := backend != "" || hasAnyEnv(
		"CACHE_EMBEDDING_TOKENIZER_PATH",
		"CACHE_ONNXRUNTIME_SHARED_LIBRARY_PATH",
		"CACHE_EMBEDDING_MAX_LENGTH",
		"CACHE_EMBEDDING_DIMENSION",
		"CACHE_EMBEDDING_OUTPUT_NAME",
		"CACHE_EMBEDDING_POOLING",
		"CACHE_EMBEDDING_NORMALIZE",
		"CACHE_EMBEDDING_INTRA_OP_THREADS",
		"CACHE_EMBEDDING_INTER_OP_THREADS",
	)
	if modelPath == "" {
		if hasEmbeddingConfig {
			return runtimeConfig{}, fmt.Errorf("CACHE_EMBEDDING_MODEL_PATH is required when embedder configuration is provided")
		}
		return runtimeConfig{Engine: "exact-only"}, nil
	}

	if backend != "" && backend != "onnx" {
		return runtimeConfig{}, fmt.Errorf("unsupported embedder backend %q", backend)
	}

	maxLength, err := parseEnvInt("CACHE_EMBEDDING_MAX_LENGTH", 512)
	if err != nil {
		return runtimeConfig{}, err
	}
	dimension, err := parseEnvInt("CACHE_EMBEDDING_DIMENSION", 384)
	if err != nil {
		return runtimeConfig{}, err
	}
	intraOpThreads, err := parseEnvInt("CACHE_EMBEDDING_INTRA_OP_THREADS", 0)
	if err != nil {
		return runtimeConfig{}, err
	}
	interOpThreads, err := parseEnvInt("CACHE_EMBEDDING_INTER_OP_THREADS", 0)
	if err != nil {
		return runtimeConfig{}, err
	}
	normalize, err := parseEnvBool("CACHE_EMBEDDING_NORMALIZE", true)
	if err != nil {
		return runtimeConfig{}, err
	}

	onnxCfg := cache.ONNXEmbedderConfig{
		ModelPath:         modelPath,
		TokenizerPath:     strings.TrimSpace(getEnv("CACHE_EMBEDDING_TOKENIZER_PATH", "")),
		SharedLibraryPath: strings.TrimSpace(getEnv("CACHE_ONNXRUNTIME_SHARED_LIBRARY_PATH", "")),
		MaxLength:         maxLength,
		Dimension:         dimension,
		OutputName:        strings.TrimSpace(getEnv("CACHE_EMBEDDING_OUTPUT_NAME", "")),
		Pooling:           strings.TrimSpace(getEnv("CACHE_EMBEDDING_POOLING", "mean")),
		Normalize:         normalize,
		IntraOpThreads:    intraOpThreads,
		InterOpThreads:    interOpThreads,
	}
	if err := onnxCfg.Validate(); err != nil {
		return runtimeConfig{}, err
	}

	return runtimeConfig{
		Engine: "onnx-cpu",
		ONNX:   &onnxCfg,
	}, nil
}

func newSemanticCache(cfg cache.Config) (*cache.SemanticCache, string, error) {
	runtimeCfg, err := loadRuntimeConfig()
	if err != nil {
		return nil, "", fmt.Errorf("load runtime config: %w", err)
	}
	if runtimeCfg.ONNX == nil {
		return cache.New(cfg), runtimeCfg.Engine, nil
	}

	embedder, err := newONNXEmbedder(*runtimeCfg.ONNX)
	if err != nil {
		return nil, "", fmt.Errorf("initialize ONNX embedder: %w", err)
	}

	return cache.NewWithDependencies(cfg, embedder, cache.NewFlatIndex()), runtimeCfg.Engine, nil
}

func parseEnvInt(key string, fallback int) (int, error) {
	v := strings.TrimSpace(getEnv(key, ""))
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return n, nil
}

func parseEnvBool(key string, fallback bool) (bool, error) {
	v := strings.TrimSpace(getEnv(key, ""))
	if v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", key, err)
	}
	return b, nil
}

func hasAnyEnv(keys ...string) bool {
	for _, key := range keys {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return true
		}
	}
	return false
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
