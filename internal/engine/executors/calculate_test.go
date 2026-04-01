package executors

import (
	"context"
	"math"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluateBasicArithmetic(t *testing.T) {
	tests := []struct {
		expr     string
		expected float64
	}{
		{"2 + 3", 5},
		{"10 - 4", 6},
		{"3 * 7", 21},
		{"20 / 4", 5},
		{"2 + 3 * 4", 14},   // operator precedence
		{"(2 + 3) * 4", 20}, // parentheses
		{"10 % 3", 1},       // modulo
		{"-5 + 3", -2},      // unary minus
		{"2 ^ 10", 1024},    // exponentiation
		{"(1 + 2) * (3 + 4)", 21},
	}

	for _, tt := range tests {
		result, err := Evaluate(tt.expr)
		require.NoError(t, err, "expression: %s", tt.expr)
		assert.InDelta(t, tt.expected, result, 1e-9, "expression: %s", tt.expr)
	}
}

func TestEvaluatePercentage(t *testing.T) {
	result, err := Evaluate("23.7% of 145892")
	require.NoError(t, err)
	assert.InDelta(t, 34576.404, result, 0.001)
}

func TestEvaluateSquareRoot(t *testing.T) {
	result, err := Evaluate("sqrt(144)")
	require.NoError(t, err)
	assert.Equal(t, 12.0, result)
}

func TestEvaluateTrigDegrees(t *testing.T) {
	result, err := Evaluate("sin(90 deg)")
	require.NoError(t, err)
	assert.InDelta(t, 1.0, result, 1e-9)
}

func TestEvaluateTrigRadians(t *testing.T) {
	result, err := Evaluate("cos(0)")
	require.NoError(t, err)
	assert.InDelta(t, 1.0, result, 1e-9)
}

func TestEvaluateLog2(t *testing.T) {
	result, err := Evaluate("log2(1024)")
	require.NoError(t, err)
	assert.InDelta(t, 10.0, result, 1e-9)
}

func TestEvaluateLog10(t *testing.T) {
	result, err := Evaluate("log10(100)")
	require.NoError(t, err)
	assert.InDelta(t, 2.0, result, 1e-9)
}

func TestEvaluateLn(t *testing.T) {
	result, err := Evaluate("ln(e)")
	require.NoError(t, err)
	assert.InDelta(t, 1.0, result, 1e-9)
}

func TestEvaluateConstants(t *testing.T) {
	result, err := Evaluate("pi * 2")
	require.NoError(t, err)
	assert.InDelta(t, 2*math.Pi, result, 1e-9)

	result, err = Evaluate("e ^ 1")
	require.NoError(t, err)
	assert.InDelta(t, math.E, result, 1e-9)
}

func TestEvaluateAbs(t *testing.T) {
	result, err := Evaluate("abs(-42)")
	require.NoError(t, err)
	assert.Equal(t, 42.0, result)
}

func TestEvaluateNestedParentheses(t *testing.T) {
	result, err := Evaluate("((2 + 3) * (4 - 1)) / 5")
	require.NoError(t, err)
	assert.Equal(t, 3.0, result)
}

func TestEvaluateDivisionByZero(t *testing.T) {
	_, err := Evaluate("10 / 0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "division by zero")
}

func TestEvaluateInvalidExpression(t *testing.T) {
	_, err := Evaluate("2 @ 3")
	assert.Error(t, err)
}

func TestEvaluateUnmatchedParenthesis(t *testing.T) {
	_, err := Evaluate("(2 + 3")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parenthesis")
}

func TestEvaluateUnknownFunction(t *testing.T) {
	_, err := Evaluate("tangent(45)")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown function")
}

func TestEvaluateSqrtNegative(t *testing.T) {
	_, err := Evaluate("sqrt(-1)")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "negative")
}

func TestFormatResult(t *testing.T) {
	assert.Equal(t, "42", formatResult(42.0))
	assert.Equal(t, "3.14", formatResult(3.14))
	assert.Equal(t, "0.3", formatResult(0.3))
	assert.Equal(t, "1024", formatResult(1024.0))
}

func TestCalculateExecutorExecution(t *testing.T) {
	exec := NewCalculateExecutor()
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionCalculate, Payload: map[string]any{"expression": "2 ^ 10"},
	})
	require.True(t, result.Success)
	assert.Contains(t, result.Output, "1024")
}

func TestCalculateExecutorEmptyExpression(t *testing.T) {
	exec := NewCalculateExecutor()
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionCalculate, Payload: map[string]any{},
	})
	assert.False(t, result.Success)
}

func TestCalculateToolSchema(t *testing.T) {
	exec := NewCalculateExecutor()
	schemas := exec.ToolSchemas()
	assert.Len(t, schemas, 1)
	assert.Equal(t, "calculate", schemas[0].Name)
}
