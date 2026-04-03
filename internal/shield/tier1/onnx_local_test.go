package tier1

import (
	"context"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestLocalOnnxClientUnavailable(t *testing.T) {
	// Without a downloaded model, the client should be unavailable.
	client := NewLocalOnnxClient(0.85)
	assert.False(t, client.IsAvailable())

	_, err := client.Classify(context.Background(), &types.ActionRequest{
		Type:    types.ActionExecCommand,
		Payload: map[string]any{"command": "ls"},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not available")
}

func TestLocalOnnxClientCloseNilSafe(t *testing.T) {
	// Close should not panic when client was never initialized.
	client := &LocalOnnxClient{}
	client.Close()
}

func TestSoftmax(t *testing.T) {
	result := softmax([]float32{1.0, 2.0, 3.0})
	assert.Len(t, result, 3)

	// Values should sum to 1.
	sum := float32(0)
	for _, v := range result {
		sum += v
		assert.True(t, v > 0, "softmax values must be positive")
	}
	assert.InDelta(t, 1.0, float64(sum), 0.001)

	// Highest input should have highest probability.
	assert.True(t, result[2] > result[1])
	assert.True(t, result[1] > result[0])
}

func TestSoftmaxSingleValue(t *testing.T) {
	result := softmax([]float32{5.0})
	assert.Len(t, result, 1)
	assert.InDelta(t, 1.0, float64(result[0]), 0.001)
}

func TestSoftmaxNegativeValues(t *testing.T) {
	result := softmax([]float32{-1.0, -2.0})
	sum := float32(0)
	for _, v := range result {
		sum += v
	}
	assert.InDelta(t, 1.0, float64(sum), 0.001)
	assert.True(t, result[0] > result[1])
}

func TestOnnxLibName(t *testing.T) {
	name := onnxLibName()
	assert.NotEmpty(t, name)
	// Should be a shared library name.
	assert.True(t,
		name == "libonnxruntime.so" ||
			name == "libonnxruntime.dylib" ||
			name == "onnxruntime.dll",
		"unexpected lib name: %s", name)
}
