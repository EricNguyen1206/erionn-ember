package cache

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"

	"github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/pretrained"
	ort "github.com/yalue/onnxruntime_go"
)

const (
	ONNXPoolingMean = "mean"

	defaultONNXOutputName = "sentence_embedding"
	inputNameIDs          = "input_ids"
	inputNameMask         = "attention_mask"
	inputNameTokenTypes   = "token_type_ids"
)

var (
	ErrInvalidONNXEmbedderConfig = errors.New("invalid onnx embedder config")
	ErrUnsupportedONNXPooling    = errors.New("unsupported onnx pooling strategy")
	ErrInvalidEmbeddingDimension = errors.New("invalid embedding dimension")

	ortInitMu            sync.Mutex
	ortSharedLibraryPath string
	ortEnvironmentReady  bool
)

// ONNXEmbedderConfig configures the embedded ONNX sentence encoder runtime.
type ONNXEmbedderConfig struct {
	ModelPath         string
	TokenizerPath     string
	SharedLibraryPath string
	MaxLength         int
	Dimension         int
	OutputName        string
	Pooling           string
	Normalize         bool
	IntraOpThreads    int
	InterOpThreads    int
}

// Validate verifies that the ONNX embedder can be initialized safely.
func (c ONNXEmbedderConfig) Validate() error {
	if strings.TrimSpace(c.ModelPath) == "" {
		return fmt.Errorf("%w: model path is required", ErrInvalidONNXEmbedderConfig)
	}
	if strings.TrimSpace(c.TokenizerPath) == "" {
		return fmt.Errorf("%w: tokenizer path is required", ErrInvalidONNXEmbedderConfig)
	}
	if strings.TrimSpace(c.SharedLibraryPath) == "" {
		return fmt.Errorf("%w: onnxruntime shared library path is required", ErrInvalidONNXEmbedderConfig)
	}
	if c.MaxLength <= 0 {
		return fmt.Errorf("%w: max length must be greater than zero", ErrInvalidONNXEmbedderConfig)
	}
	if c.Dimension <= 0 {
		return fmt.Errorf("%w: dimension must be greater than zero", ErrInvalidONNXEmbedderConfig)
	}
	if c.IntraOpThreads < 0 {
		return fmt.Errorf("%w: intra-op threads must be zero or greater", ErrInvalidONNXEmbedderConfig)
	}
	if c.InterOpThreads < 0 {
		return fmt.Errorf("%w: inter-op threads must be zero or greater", ErrInvalidONNXEmbedderConfig)
	}

	pooling := normalizeONNXPooling(c.Pooling)
	if pooling != ONNXPoolingMean {
		return fmt.Errorf("%w: %q", ErrUnsupportedONNXPooling, c.Pooling)
	}

	return nil
}

// ONNXEmbedder embeds prompt text with a local CPU ONNX model.
type ONNXEmbedder struct {
	cfg           ONNXEmbedderConfig
	tokenizer     *tokenizer.Tokenizer
	session       *ort.DynamicAdvancedSession
	inputNames    []string
	outputName    string
	outputPooling string
	mu            sync.Mutex
}

// NewONNXEmbedder creates an Embedder backed by local tokenizer + ONNX Runtime assets.
func NewONNXEmbedder(cfg ONNXEmbedderConfig) (*ONNXEmbedder, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if err := ensureONNXRuntimeInitialized(cfg.SharedLibraryPath); err != nil {
		return nil, err
	}

	tk, err := pretrained.FromFile(cfg.TokenizerPath)
	if err != nil {
		return nil, fmt.Errorf("load tokenizer: %w", err)
	}
	tk.WithPadding(nil)
	tk.WithTruncation(nil)

	inputInfo, outputInfo, err := ort.GetInputOutputInfo(cfg.ModelPath)
	if err != nil {
		return nil, fmt.Errorf("inspect ONNX model IO: %w", err)
	}

	inputNames, err := resolveONNXInputNames(inputInfo)
	if err != nil {
		return nil, err
	}

	outputName, err := resolveONNXOutputName(cfg, outputInfo)
	if err != nil {
		return nil, err
	}

	sessionOptions, err := ort.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("create ONNX session options: %w", err)
	}
	defer func() {
		_ = sessionOptions.Destroy()
	}()

	if cfg.IntraOpThreads > 0 {
		if err := sessionOptions.SetIntraOpNumThreads(cfg.IntraOpThreads); err != nil {
			return nil, fmt.Errorf("configure intra-op threads: %w", err)
		}
	}
	if cfg.InterOpThreads > 0 {
		if err := sessionOptions.SetInterOpNumThreads(cfg.InterOpThreads); err != nil {
			return nil, fmt.Errorf("configure inter-op threads: %w", err)
		}
	}
	if err := sessionOptions.SetGraphOptimizationLevel(ort.GraphOptimizationLevelEnableAll); err != nil {
		return nil, fmt.Errorf("configure graph optimization: %w", err)
	}

	session, err := ort.NewDynamicAdvancedSession(cfg.ModelPath, inputNames, []string{outputName}, sessionOptions)
	if err != nil {
		return nil, fmt.Errorf("create ONNX session: %w", err)
	}

	return &ONNXEmbedder{
		cfg:           cfg,
		tokenizer:     tk,
		session:       session,
		inputNames:    inputNames,
		outputName:    outputName,
		outputPooling: normalizeONNXPooling(cfg.Pooling),
	}, nil
}

// Dimension returns the configured embedding size.
func (e *ONNXEmbedder) Dimension() int {
	return e.cfg.Dimension
}

// Embed tokenizes text, runs ONNX inference, optionally pools, normalizes, and returns the vector.
func (e *ONNXEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	encoding, err := e.tokenizer.EncodeSingle(text, true)
	if err != nil {
		return nil, fmt.Errorf("tokenize text: %w", err)
	}

	inputIDs, attentionMask, tokenTypeIDs := prepareTokenizerInputs(encoding, e.cfg.MaxLength)
	inputs, cleanup, err := buildONNXInputs(e.inputNames, inputIDs, attentionMask, tokenTypeIDs)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	outputs := make([]ort.Value, 1)

	e.mu.Lock()
	err = e.session.Run(inputs, outputs)
	e.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("run ONNX inference: %w", err)
	}
	defer destroyONNXValues(outputs)

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	tensor, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("output %q is %T, want float32 tensor", e.outputName, outputs[0])
	}

	vector, err := extractEmbeddingVector(tensor.GetData(), tensor.GetShape(), attentionMask, e.cfg.Dimension, e.outputPooling)
	if err != nil {
		return nil, err
	}
	if e.cfg.Normalize {
		vector = normalizeEmbeddingVector(vector)
	}
	if err := validateEmbeddingDimension(vector, e.cfg.Dimension); err != nil {
		return nil, err
	}

	return vector, nil
}

func ensureONNXRuntimeInitialized(sharedLibraryPath string) error {
	ortInitMu.Lock()
	defer ortInitMu.Unlock()

	if ortEnvironmentReady {
		if ortSharedLibraryPath != sharedLibraryPath {
			return fmt.Errorf("onnxruntime already initialized with %q, cannot reuse with %q", ortSharedLibraryPath, sharedLibraryPath)
		}
		return nil
	}

	ort.SetSharedLibraryPath(sharedLibraryPath)
	if err := ort.InitializeEnvironment(ort.WithLogLevelError()); err != nil {
		return fmt.Errorf("initialize onnxruntime environment: %w", err)
	}

	ortSharedLibraryPath = sharedLibraryPath
	ortEnvironmentReady = true
	return nil
}

func resolveONNXInputNames(inputInfo []ort.InputOutputInfo) ([]string, error) {
	available := make(map[string]string, len(inputInfo))
	for _, info := range inputInfo {
		available[strings.ToLower(info.Name)] = info.Name
	}

	inputNames := make([]string, 0, 3)
	for _, wanted := range []string{inputNameIDs, inputNameMask, inputNameTokenTypes} {
		if actual, ok := available[wanted]; ok {
			inputNames = append(inputNames, actual)
		}
	}

	if len(inputNames) < 2 || !slices.Contains(inputNames, available[inputNameIDs]) || !slices.Contains(inputNames, available[inputNameMask]) {
		return nil, fmt.Errorf("inspect ONNX model IO: required inputs %q and %q were not found", inputNameIDs, inputNameMask)
	}

	return inputNames, nil
}

func resolveONNXOutputName(cfg ONNXEmbedderConfig, outputInfo []ort.InputOutputInfo) (string, error) {
	requested := strings.TrimSpace(cfg.OutputName)
	if requested == "" {
		requested = defaultONNXOutputName
	}

	for _, info := range outputInfo {
		if info.Name == requested {
			return requested, nil
		}
	}

	if cfg.OutputName == "" && len(outputInfo) > 0 {
		return outputInfo[0].Name, nil
	}

	return "", fmt.Errorf("inspect ONNX model IO: output %q was not found", requested)
}

func prepareTokenizerInputs(encoding *tokenizer.Encoding, maxLength int) ([]int64, []int64, []int64) {
	inputIDs := truncateInts(encoding.Ids, maxLength)
	attentionMask := truncateInts(encoding.AttentionMask, maxLength)
	typeIDs := truncateInts(encoding.TypeIds, maxLength)

	if len(attentionMask) == 0 {
		attentionMask = make([]int64, len(inputIDs))
		for i := range attentionMask {
			attentionMask[i] = 1
		}
	}
	if len(typeIDs) == 0 {
		typeIDs = make([]int64, len(inputIDs))
	}

	return inputIDs, attentionMask, typeIDs
}

func truncateInts(values []int, maxLength int) []int64 {
	if len(values) > maxLength {
		values = values[:maxLength]
	}

	truncated := make([]int64, len(values))
	for i, value := range values {
		truncated[i] = int64(value)
	}
	return truncated
}

func buildONNXInputs(inputNames []string, inputIDs, attentionMask, tokenTypeIDs []int64) ([]ort.Value, func(), error) {
	shape := ort.NewShape(1, int64(len(inputIDs)))
	if len(inputIDs) == 0 {
		return nil, nil, fmt.Errorf("tokenize text: no tokens produced")
	}

	created := make([]ort.Value, 0, len(inputNames))
	cleanup := func() {
		destroyONNXValues(created)
	}

	inputs := make([]ort.Value, 0, len(inputNames))
	for _, name := range inputNames {
		var data []int64
		switch strings.ToLower(name) {
		case inputNameIDs:
			data = inputIDs
		case inputNameMask:
			data = attentionMask
		case inputNameTokenTypes:
			data = tokenTypeIDs
		default:
			cleanup()
			return nil, nil, fmt.Errorf("unsupported ONNX input %q", name)
		}

		tensor, err := ort.NewTensor(shape, data)
		if err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("create tensor for %q: %w", name, err)
		}
		created = append(created, tensor)
		inputs = append(inputs, tensor)
	}

	return inputs, cleanup, nil
}

func destroyONNXValues(values []ort.Value) {
	for _, value := range values {
		if value != nil {
			_ = value.Destroy()
		}
	}
}

func extractEmbeddingVector(data []float32, shape ort.Shape, attentionMask []int64, dimension int, pooling string) ([]float32, error) {
	switch len(shape) {
	case 2:
		vector := append([]float32(nil), data...)
		if shape[0] == 1 && int(shape[1]) == len(vector) {
			if err := validateEmbeddingDimension(vector, dimension); err != nil {
				return nil, err
			}
			return vector, nil
		}
	case 3:
		if shape[0] != 1 {
			return nil, fmt.Errorf("unsupported ONNX output batch size %d", shape[0])
		}
		return meanPoolEmbeddings(data, attentionMask, int(shape[1]), int(shape[2]))
	}

	return nil, fmt.Errorf("unsupported ONNX output shape %v", shape)
}

func meanPoolEmbeddings(hiddenState []float32, attentionMask []int64, tokenCount, dimension int) ([]float32, error) {
	if tokenCount <= 0 || dimension <= 0 {
		return nil, fmt.Errorf("invalid hidden state shape: tokens=%d dimension=%d", tokenCount, dimension)
	}
	if len(hiddenState) != tokenCount*dimension {
		return nil, fmt.Errorf("invalid hidden state length %d for shape [%d %d]", len(hiddenState), tokenCount, dimension)
	}
	if len(attentionMask) != tokenCount {
		return nil, fmt.Errorf("attention mask length %d does not match token count %d", len(attentionMask), tokenCount)
	}

	pooled := make([]float32, dimension)
	var count float32
	for token := 0; token < tokenCount; token++ {
		if attentionMask[token] == 0 {
			continue
		}
		count++
		base := token * dimension
		for i := 0; i < dimension; i++ {
			pooled[i] += hiddenState[base+i]
		}
	}

	if count == 0 {
		return nil, fmt.Errorf("attention mask excluded every token")
	}

	for i := range pooled {
		pooled[i] /= count
	}
	return pooled, nil
}

func normalizeEmbeddingVector(vector []float32) []float32 {
	normalized := append([]float32(nil), vector...)

	var sum float64
	for _, value := range normalized {
		sum += float64(value * value)
	}
	if sum == 0 {
		return normalized
	}

	norm := float32(math.Sqrt(sum))
	for i := range normalized {
		normalized[i] /= norm
	}
	return normalized
}

func validateEmbeddingDimension(vector []float32, dimension int) error {
	if len(vector) != dimension {
		return fmt.Errorf("%w: got %d want %d", ErrInvalidEmbeddingDimension, len(vector), dimension)
	}
	return nil
}

func normalizeONNXPooling(pooling string) string {
	if strings.TrimSpace(pooling) == "" {
		return ONNXPoolingMean
	}
	return strings.ToLower(strings.TrimSpace(pooling))
}
