package shield

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"

	ort "github.com/shota3506/onnxruntime-purego/onnxruntime"
	"github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/pretrained"
)

const maxSeqLength = 512

// LocalOnnxClient runs the ONNX classifier in-process using onnxruntime-purego.
// It loads the DeBERTa model and tokenizer from ~/.openparallax/models/prompt-injection/.
type LocalOnnxClient struct {
	ortRuntime *ort.Runtime
	session    *ort.Session
	env        *ort.Env
	tk         *tokenizer.Tokenizer
	threshold  float64
	available  bool
}

// NewLocalOnnxClient attempts to load the ONNX model from the default path.
// Returns a client with available=false if the model or runtime library is missing.
func NewLocalOnnxClient(threshold float64) *LocalOnnxClient {
	client := &LocalOnnxClient{threshold: threshold}

	home, err := os.UserHomeDir()
	if err != nil {
		return client
	}

	modelDir := filepath.Join(home, ".openparallax", "models", "prompt-injection")
	modelPath := filepath.Join(modelDir, "model.onnx")
	tokenizerPath := filepath.Join(modelDir, "tokenizer.json")
	libPath := filepath.Join(modelDir, onnxLibName())

	// Check all required files exist.
	for _, path := range []string{modelPath, tokenizerPath, libPath} {
		if _, statErr := os.Stat(path); statErr != nil {
			return client
		}
	}

	// Load ONNX Runtime.
	ortRT, rtErr := ort.NewRuntime(libPath, 23)
	if rtErr != nil {
		return client
	}
	client.ortRuntime = ortRT

	// Create environment.
	ortEnv, envErr := ortRT.NewEnv("openparallax-shield", ort.LoggingLevelWarning)
	if envErr != nil {
		_ = ortRT.Close()
		return client
	}
	client.env = ortEnv

	// Load model.
	sess, sessErr := ortRT.NewSession(ortEnv, modelPath, &ort.SessionOptions{
		IntraOpNumThreads: 1,
	})
	if sessErr != nil {
		ortEnv.Close()
		_ = ortRT.Close()
		return client
	}
	client.session = sess

	// Load tokenizer.
	tk, tkErr := pretrained.FromFile(tokenizerPath)
	if tkErr != nil {
		sess.Close()
		ortEnv.Close()
		_ = ortRT.Close()
		return client
	}
	client.tk = tk
	client.available = true

	return client
}

// Classify runs inference on the action text and returns the classification result.
func (c *LocalOnnxClient) Classify(_ context.Context, action *ActionRequest) (*ClassifierResult, error) {
	if !c.available {
		return nil, fmt.Errorf("local ONNX classifier not available")
	}

	text := fmt.Sprintf("%s: %v", action.Type, action.Payload)

	// Tokenize.
	encoding, err := c.tk.EncodeSingle(text)
	if err != nil {
		return nil, fmt.Errorf("tokenize: %w", err)
	}

	ids := encoding.GetIds()
	attMask := encoding.GetAttentionMask()

	// Truncate/pad to maxSeqLength.
	if len(ids) > maxSeqLength {
		ids = ids[:maxSeqLength]
		attMask = attMask[:maxSeqLength]
	}
	for len(ids) < maxSeqLength {
		ids = append(ids, 0)
		attMask = append(attMask, 0)
	}

	// Convert to int64 slices.
	inputIDs := make([]int64, maxSeqLength)
	attention := make([]int64, maxSeqLength)
	tokenTypeIDs := make([]int64, maxSeqLength) // all zeros
	for i := range ids {
		inputIDs[i] = int64(ids[i])
		attention[i] = int64(attMask[i])
	}

	shape := []int64{1, int64(maxSeqLength)}

	inputIDsTensor, err := ort.NewTensorValue(c.ortRuntime, inputIDs, shape)
	if err != nil {
		return nil, fmt.Errorf("create input_ids tensor: %w", err)
	}
	defer func() { inputIDsTensor.Close() }()

	attMaskTensor, err := ort.NewTensorValue(c.ortRuntime, attention, shape)
	if err != nil {
		return nil, fmt.Errorf("create attention_mask tensor: %w", err)
	}
	defer func() { attMaskTensor.Close() }()

	ttidsTensor, err := ort.NewTensorValue(c.ortRuntime, tokenTypeIDs, shape)
	if err != nil {
		return nil, fmt.Errorf("create token_type_ids tensor: %w", err)
	}
	defer func() { ttidsTensor.Close() }()

	// Run inference.
	inputs := map[string]*ort.Value{
		"input_ids":      inputIDsTensor,
		"attention_mask": attMaskTensor,
		"token_type_ids": ttidsTensor,
	}
	outputs, runErr := c.session.Run(context.Background(), inputs)
	if runErr != nil {
		return nil, fmt.Errorf("onnx inference: %w", runErr)
	}

	// Extract logits.
	logits, _, err := ort.GetTensorData[float32](outputs["logits"])
	if err != nil {
		return nil, fmt.Errorf("extract logits: %w", err)
	}

	// Apply softmax to get probabilities.
	// logits[0] = SAFE score, logits[1] = INJECTION score
	probs := softmax(logits)

	label := "SAFE"
	confidence := float64(probs[0])
	if len(probs) > 1 && probs[1] > probs[0] {
		label = "INJECTION"
		confidence = float64(probs[1])
	}

	var decision VerdictDecision
	switch {
	case label == "INJECTION" && confidence >= c.threshold:
		decision = VerdictBlock
	case label == "INJECTION":
		decision = VerdictEscalate
	default:
		decision = VerdictAllow
	}

	return &ClassifierResult{
		Decision:   decision,
		Confidence: confidence,
		Reason:     fmt.Sprintf("classifier [%s, conf %.2f]: prompt-injection ONNX model flagged this input — paraphrasing the request will not bypass it", label, confidence),
		Source:     "onnx-local",
	}, nil
}

// IsAvailable returns true if the model was loaded successfully.
func (c *LocalOnnxClient) IsAvailable() bool { return c.available }

// Close releases ONNX Runtime resources.
func (c *LocalOnnxClient) Close() {
	if c.session != nil {
		c.session.Close()
	}
	if c.env != nil {
		c.env.Close()
	}
	if c.ortRuntime != nil {
		_ = c.ortRuntime.Close()
	}
}

// softmax applies the softmax function to a slice of float32 logits.
func softmax(logits []float32) []float32 {
	maxVal := logits[0]
	for _, v := range logits[1:] {
		if v > maxVal {
			maxVal = v
		}
	}
	sum := float32(0)
	result := make([]float32, len(logits))
	for i, v := range logits {
		result[i] = float32(math.Exp(float64(v - maxVal)))
		sum += result[i]
	}
	for i := range result {
		result[i] /= sum
	}
	return result
}

// onnxLibName returns the platform-specific ONNX Runtime library filename.
func onnxLibName() string {
	switch runtime.GOOS {
	case "darwin":
		return "libonnxruntime.dylib"
	case "windows":
		return "onnxruntime.dll"
	default:
		return "libonnxruntime.so"
	}
}
