package cache

import (
	"math"
	"reflect"
	"testing"
)

func TestONNXEmbedderConfigValidate(t *testing.T) {
	valid := ONNXEmbedderConfig{
		ModelPath:         "/models/embedder.onnx",
		TokenizerPath:     "/models/tokenizer.json",
		SharedLibraryPath: "/opt/onnxruntime/libonnxruntime.so",
		Dimension:         384,
		MaxLength:         256,
		OutputName:        "sentence_embedding",
		Pooling:           ONNXPoolingMean,
	}

	tests := []struct {
		name string
		cfg  ONNXEmbedderConfig
	}{
		{name: "valid", cfg: valid},
		{name: "missing model path", cfg: ONNXEmbedderConfig{}},
		{name: "missing tokenizer path", cfg: func() ONNXEmbedderConfig {
			cfg := valid
			cfg.TokenizerPath = ""
			return cfg
		}()},
		{name: "missing shared library path", cfg: func() ONNXEmbedderConfig {
			cfg := valid
			cfg.SharedLibraryPath = ""
			return cfg
		}()},
		{name: "invalid dimension", cfg: func() ONNXEmbedderConfig {
			cfg := valid
			cfg.Dimension = 0
			return cfg
		}()},
		{name: "invalid max length", cfg: func() ONNXEmbedderConfig {
			cfg := valid
			cfg.MaxLength = 0
			return cfg
		}()},
		{name: "invalid pooling", cfg: func() ONNXEmbedderConfig {
			cfg := valid
			cfg.Pooling = "cls"
			return cfg
		}()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.name == "valid" && err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
			if tt.name != "valid" && err == nil {
				t.Fatal("Validate() error = nil, want non-nil")
			}
		})
	}
}

func TestMeanPoolEmbeddings(t *testing.T) {
	hiddenState := []float32{
		1, 2,
		3, 4,
		100, 200,
	}

	pooled, err := meanPoolEmbeddings(hiddenState, []int64{1, 1, 0}, 3, 2)
	if err != nil {
		t.Fatalf("meanPoolEmbeddings() error = %v", err)
	}

	want := []float32{2, 3}
	if !reflect.DeepEqual(pooled, want) {
		t.Fatalf("meanPoolEmbeddings() = %v, want %v", pooled, want)
	}
}

func TestMeanPoolEmbeddingsRejectsShapeMismatch(t *testing.T) {
	_, err := meanPoolEmbeddings([]float32{1, 2, 3, 4}, []int64{1}, 2, 2)
	if err == nil {
		t.Fatal("meanPoolEmbeddings() error = nil, want non-nil")
	}
}

func TestNormalizeEmbeddingVector(t *testing.T) {
	vector := []float32{3, 4}

	normalized := normalizeEmbeddingVector(vector)
	if len(normalized) != 2 {
		t.Fatalf("len(normalizeEmbeddingVector()) = %d, want 2", len(normalized))
	}

	if math.Abs(float64(normalized[0]-0.6)) > 1e-6 {
		t.Fatalf("normalized[0] = %v, want 0.6", normalized[0])
	}
	if math.Abs(float64(normalized[1]-0.8)) > 1e-6 {
		t.Fatalf("normalized[1] = %v, want 0.8", normalized[1])
	}
}

func TestValidateEmbeddingDimension(t *testing.T) {
	if err := validateEmbeddingDimension([]float32{1, 2}, 3); err == nil {
		t.Fatal("validateEmbeddingDimension() error = nil, want non-nil")
	}

	if err := validateEmbeddingDimension([]float32{1, 2, 3}, 3); err != nil {
		t.Fatalf("validateEmbeddingDimension() error = %v, want nil", err)
	}
}
