package embedding

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/pipelines"
)

const (
	// HuggingFace model ID — same weights as mahonzhan/all-MiniLM-L6-v2 on Ollama,
	// but in ONNX format (required for in-process inference).
	DefaultHFModel = "sentence-transformers/all-MiniLM-L6-v2"
)

// ONNXEmbedder runs all-MiniLM-L6-v2 in-process via ONNX Runtime (hugot).
// No HTTP calls at inference time — the model file is loaded directly.
//
// On first run with an empty MODEL_DIR, hugot downloads the ONNX weights
// from HuggingFace into MODEL_DIR automatically.
//
// Prerequisites (build-time):
//
//	CGO_ENABLED=1 (hugot wraps ONNX Runtime C library)
//
// Prerequisites (runtime):
//
//	MODEL_DIR=/path/to/models   → auto-downloaded on first start
type ONNXEmbedder struct {
	session  *hugot.Session
	pipeline *pipelines.FeatureExtractionPipeline
	dim      int
}

// NewONNXEmbedder initialises hugot, downloads the model if needed, and probes the dimension.
func NewONNXEmbedder(modelDir string) (*ONNXEmbedder, error) {
	session, err := hugot.NewSession(
		hugot.WithTelemetry(false),
	)
	if err != nil {
		return nil, fmt.Errorf("hugot session: %w", err)
	}

	// Resolve model path inside modelDir.
	// hugot stores models as modelDir/orgName/modelName/.
	modelName := "sentence-transformers_all-MiniLM-L6-v2"
	modelPath := filepath.Join(modelDir, modelName)

	// Auto-download from HuggingFace if model directory is missing.
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		fmt.Printf("⬇ Downloading %s → %s (first run, ~22MB)\n", DefaultHFModel, modelDir)
		downloaded, dlErr := hugot.DownloadModel(
			session, DefaultHFModel, modelDir, hugot.NewDownloadOptions(),
		)
		if dlErr != nil {
			_ = session.Destroy()
			return nil, fmt.Errorf("downloading %s: %w", DefaultHFModel, dlErr)
		}
		modelPath = downloaded
	}

	cfg := hugot.FeatureExtractionConfig{
		ModelPath:    modelPath,
		Name:         "erion-embedder",
		OnnxFilename: "model.onnx",
	}
	pipe, err := hugot.NewPipeline[pipelines.FeatureExtractionPipeline](session, cfg)
	if err != nil {
		_ = session.Destroy()
		return nil, fmt.Errorf("creating pipeline: %w", err)
	}

	// Probe to confirm the model works and detect the embedding dimension.
	probe, err := pipe.RunPipeline([]string{"test"})
	if err != nil || len(probe.Embeddings) == 0 {
		_ = session.Destroy()
		return nil, fmt.Errorf("model probe failed: %w", err)
	}

	return &ONNXEmbedder{
		session:  session,
		pipeline: pipe,
		dim:      len(probe.Embeddings[0]),
	}, nil
}

// Embed returns a 384-dim L2-normalised float32 vector for the input text.
func (e *ONNXEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	result, err := e.pipeline.RunPipeline([]string{text})
	if err != nil {
		return nil, fmt.Errorf("onnx embed: %w", err)
	}
	if len(result.Embeddings) == 0 {
		return nil, fmt.Errorf("onnx returned empty embedding")
	}
	return result.Embeddings[0], nil
}

// Dim returns the embedding dimension (384 for all-MiniLM-L6-v2).
func (e *ONNXEmbedder) Dim() int { return e.dim }

// Close releases the ONNX session and associated C memory.
func (e *ONNXEmbedder) Close() error {
	if e.session != nil {
		return e.session.Destroy()
	}
	return nil
}
