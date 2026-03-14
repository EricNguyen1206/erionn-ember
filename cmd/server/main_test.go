package main

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/EricNguyen1206/erion-ember/internal/cache"
)

type startupTestEmbedder struct {
	vector []float32
}

func (e startupTestEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return append([]float32(nil), e.vector...), nil
}

func (e startupTestEmbedder) Dimension() int {
	return len(e.vector)
}

func TestLoadRuntimeConfigDefaultsToExactOnly(t *testing.T) {
	t.Setenv("CACHE_EMBEDDER_BACKEND", "")
	t.Setenv("CACHE_EMBEDDING_MODEL_PATH", "")
	t.Setenv("CACHE_EMBEDDING_TOKENIZER_PATH", "")
	t.Setenv("CACHE_ONNXRUNTIME_SHARED_LIBRARY_PATH", "")

	runtimeCfg, err := loadRuntimeConfig()
	if err != nil {
		t.Fatalf("loadRuntimeConfig() error = %v", err)
	}

	if runtimeCfg.Engine != "exact-only" {
		t.Fatalf("Engine = %q, want %q", runtimeCfg.Engine, "exact-only")
	}

	if runtimeCfg.ONNX != nil {
		t.Fatalf("ONNX config = %#v, want nil", runtimeCfg.ONNX)
	}
}

func TestLoadRuntimeConfigRequiresCompleteONNXConfig(t *testing.T) {
	t.Setenv("CACHE_EMBEDDER_BACKEND", "onnx")
	t.Setenv("CACHE_EMBEDDING_MODEL_PATH", "/models/embedder.onnx")
	t.Setenv("CACHE_EMBEDDING_TOKENIZER_PATH", "")
	t.Setenv("CACHE_ONNXRUNTIME_SHARED_LIBRARY_PATH", "")

	_, err := loadRuntimeConfig()
	if err == nil {
		t.Fatal("loadRuntimeConfig() error = nil, want non-nil")
	}
}

func TestLoadRuntimeConfigRejectsExplicitONNXWithoutModelPath(t *testing.T) {
	t.Setenv("CACHE_EMBEDDER_BACKEND", "onnx")
	t.Setenv("CACHE_EMBEDDING_MODEL_PATH", "")
	t.Setenv("CACHE_EMBEDDING_TOKENIZER_PATH", "")
	t.Setenv("CACHE_ONNXRUNTIME_SHARED_LIBRARY_PATH", "")

	_, err := loadRuntimeConfig()
	if err == nil {
		t.Fatal("loadRuntimeConfig() error = nil, want non-nil")
	}
}

func TestLoadRuntimeConfigRejectsEmbeddingKnobsWithoutModelPath(t *testing.T) {
	t.Setenv("CACHE_EMBEDDER_BACKEND", "")
	t.Setenv("CACHE_EMBEDDING_MODEL_PATH", "")
	t.Setenv("CACHE_EMBEDDING_MAX_LENGTH", "256")
	t.Setenv("CACHE_EMBEDDING_TOKENIZER_PATH", "")
	t.Setenv("CACHE_ONNXRUNTIME_SHARED_LIBRARY_PATH", "")

	_, err := loadRuntimeConfig()
	if err == nil {
		t.Fatal("loadRuntimeConfig() error = nil, want non-nil")
	}
}

func TestNewSemanticCacheUsesInjectedONNXEmbedderFactory(t *testing.T) {
	t.Setenv("CACHE_EMBEDDER_BACKEND", "onnx")
	t.Setenv("CACHE_EMBEDDING_MODEL_PATH", "/models/embedder.onnx")
	t.Setenv("CACHE_EMBEDDING_TOKENIZER_PATH", "/models/tokenizer.json")
	t.Setenv("CACHE_ONNXRUNTIME_SHARED_LIBRARY_PATH", "/opt/onnxruntime/libonnxruntime.dylib")
	t.Setenv("CACHE_EMBEDDING_MAX_LENGTH", "128")
	t.Setenv("CACHE_EMBEDDING_DIMENSION", "384")
	t.Setenv("CACHE_EMBEDDING_OUTPUT_NAME", "sentence_embedding")
	t.Setenv("CACHE_EMBEDDING_POOLING", "mean")
	t.Setenv("CACHE_EMBEDDING_NORMALIZE", "true")
	t.Setenv("CACHE_EMBEDDING_INTRA_OP_THREADS", "2")
	t.Setenv("CACHE_EMBEDDING_INTER_OP_THREADS", "1")

	var got cache.ONNXEmbedderConfig
	originalFactory := newONNXEmbedder
	newONNXEmbedder = func(cfg cache.ONNXEmbedderConfig) (cache.Embedder, error) {
		got = cfg
		return startupTestEmbedder{vector: []float32{1, 0}}, nil
	}
	defer func() {
		newONNXEmbedder = originalFactory
	}()

	sc, engine, err := newSemanticCache(cache.DefaultConfig())
	if err != nil {
		t.Fatalf("newSemanticCache() error = %v", err)
	}

	if engine != "onnx-cpu" {
		t.Fatalf("engine = %q, want %q", engine, "onnx-cpu")
	}

	wantCfg := cache.ONNXEmbedderConfig{
		ModelPath:         "/models/embedder.onnx",
		TokenizerPath:     "/models/tokenizer.json",
		SharedLibraryPath: "/opt/onnxruntime/libonnxruntime.dylib",
		MaxLength:         128,
		Dimension:         384,
		OutputName:        "sentence_embedding",
		Pooling:           cache.ONNXPoolingMean,
		Normalize:         true,
		IntraOpThreads:    2,
		InterOpThreads:    1,
	}

	if !reflect.DeepEqual(got, wantCfg) {
		t.Fatalf("factory config = %#v, want %#v", got, wantCfg)
	}

	if _, err := sc.Set(context.Background(), "hello", "world", time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	result, ok := sc.Get(context.Background(), "different prompt", 0.1)
	if !ok {
		t.Fatal("Get() miss, want semantic hit")
	}

	if result.ExactMatch {
		t.Fatal("Get() returned exact match, want semantic hit")
	}
}
