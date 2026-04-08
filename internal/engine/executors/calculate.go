package executors

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/openparallax/openparallax/internal/types"
)

// CalculateExecutor evaluates mathematical expressions.
type CalculateExecutor struct{}

// NewCalculateExecutor creates a new calculate executor.
func NewCalculateExecutor() *CalculateExecutor { return &CalculateExecutor{} }

// WorkspaceScope reports that calculate does not touch the filesystem.
func (c *CalculateExecutor) WorkspaceScope() WorkspaceScope { return ScopeNoFilesystem }

// SupportedActions returns the calculate action type.
func (c *CalculateExecutor) SupportedActions() []types.ActionType {
	return []types.ActionType{types.ActionCalculate}
}

// ToolSchemas returns the calculate tool definition.
func (c *CalculateExecutor) ToolSchemas() []ToolSchema {
	return []ToolSchema{{
		ActionType:  types.ActionCalculate,
		Name:        "calculate",
		Description: "Evaluate a mathematical expression with precision. Supports arithmetic (+, -, *, /, ^, %), sqrt, sin, cos, tan, log, ln, abs, and constants (pi, e). Examples: '2^10', 'sqrt(144)', 'sin(90 deg)', '23.7% of 145892'.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"expression": map[string]any{"type": "string", "description": "Mathematical expression to evaluate."},
			},
			"required": []string{"expression"},
		},
	}}
}

// Execute evaluates the expression.
func (c *CalculateExecutor) Execute(_ context.Context, action *types.ActionRequest) *types.ActionResult {
	expr, _ := action.Payload["expression"].(string)
	if expr == "" {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "expression is required"}
	}

	result, err := Evaluate(expr)
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "calculation error"}
	}

	formatted := formatResult(result)
	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output: fmt.Sprintf("%s = %s", expr, formatted), Summary: formatted,
	}
}

// Evaluate parses and evaluates a math expression string.
func Evaluate(expr string) (float64, error) {
	// Pre-process: handle "X% of Y" syntax.
	expr = preprocessPercent(expr)

	p := &parser{input: expr, pos: 0}
	result, err := p.parseExpr(0)
	if err != nil {
		return 0, err
	}
	// Verify all input consumed.
	p.skipSpaces()
	if p.pos < len(p.input) {
		return 0, fmt.Errorf("unexpected character at position %d: %q", p.pos, string(p.input[p.pos]))
	}
	return result, nil
}

func preprocessPercent(expr string) string {
	// Handle "X% of Y" → "(X/100)*Y"
	lower := strings.ToLower(expr)
	if idx := strings.Index(lower, "% of "); idx > 0 {
		pct := strings.TrimSpace(expr[:idx])
		val := strings.TrimSpace(expr[idx+5:])
		return fmt.Sprintf("(%s/100)*(%s)", pct, val)
	}
	return expr
}

// --- Pratt parser ---

type parser struct {
	input string
	pos   int
}

func (p *parser) parseExpr(minPrec int) (float64, error) {
	left, err := p.parseUnary()
	if err != nil {
		return 0, err
	}

	for {
		p.skipSpaces()
		if p.pos >= len(p.input) {
			break
		}

		op := p.input[p.pos]
		prec := precedence(op)
		if prec < minPrec {
			break
		}

		p.pos++
		right, parseErr := p.parseExpr(prec + 1)
		if parseErr != nil {
			return 0, parseErr
		}

		switch op {
		case '+':
			left += right
		case '-':
			left -= right
		case '*':
			left *= right
		case '/':
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			left /= right
		case '^':
			left = math.Pow(left, right)
		case '%':
			if right == 0 {
				return 0, fmt.Errorf("modulo by zero")
			}
			left = math.Mod(left, right)
		default:
			return 0, fmt.Errorf("unknown operator: %c", op)
		}
	}
	return left, nil
}

func (p *parser) parseUnary() (float64, error) {
	p.skipSpaces()
	if p.pos >= len(p.input) {
		return 0, fmt.Errorf("unexpected end of expression")
	}

	// Unary minus.
	if p.input[p.pos] == '-' {
		p.pos++
		val, err := p.parseUnary()
		if err != nil {
			return 0, err
		}
		return -val, nil
	}

	// Unary plus.
	if p.input[p.pos] == '+' {
		p.pos++
		return p.parseUnary()
	}

	return p.parsePrimary()
}

func (p *parser) parsePrimary() (float64, error) {
	p.skipSpaces()
	if p.pos >= len(p.input) {
		return 0, fmt.Errorf("unexpected end of expression")
	}

	// Parentheses.
	if p.input[p.pos] == '(' {
		p.pos++
		val, err := p.parseExpr(0)
		if err != nil {
			return 0, err
		}
		p.skipSpaces()
		if p.pos >= len(p.input) || p.input[p.pos] != ')' {
			return 0, fmt.Errorf("unmatched parenthesis")
		}
		p.pos++
		return val, nil
	}

	// Number.
	if p.input[p.pos] == '.' || (p.input[p.pos] >= '0' && p.input[p.pos] <= '9') {
		return p.parseNumber()
	}

	// Named function or constant.
	if unicode.IsLetter(rune(p.input[p.pos])) {
		return p.parseFuncOrConst()
	}

	return 0, fmt.Errorf("unexpected character: %q", string(p.input[p.pos]))
}

func (p *parser) parseNumber() (float64, error) {
	start := p.pos
	for p.pos < len(p.input) && (p.input[p.pos] >= '0' && p.input[p.pos] <= '9' || p.input[p.pos] == '.') {
		p.pos++
	}
	val, err := strconv.ParseFloat(p.input[start:p.pos], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number: %s", p.input[start:p.pos])
	}
	return val, nil
}

func (p *parser) parseFuncOrConst() (float64, error) {
	start := p.pos
	for p.pos < len(p.input) && (unicode.IsLetter(rune(p.input[p.pos])) || unicode.IsDigit(rune(p.input[p.pos]))) {
		p.pos++
	}
	name := strings.ToLower(p.input[start:p.pos])

	// Check for "deg" suffix after a number (handled in trig functions).
	p.skipSpaces()

	// Constants.
	switch name {
	case "pi":
		return math.Pi, nil
	case "e":
		return math.E, nil
	}

	// Functions require parentheses.
	if p.pos >= len(p.input) || p.input[p.pos] != '(' {
		return 0, fmt.Errorf("unknown identifier: %s", name)
	}
	p.pos++ // skip '('

	arg, err := p.parseExpr(0)
	if err != nil {
		return 0, err
	}

	// Check for "deg" modifier inside trig functions.
	p.skipSpaces()

	// Check for optional "deg" before closing paren.
	isDeg := false
	if p.pos+3 <= len(p.input) && strings.ToLower(p.input[p.pos:p.pos+3]) == "deg" {
		isDeg = true
		p.pos += 3
		p.skipSpaces()
	}

	if p.pos >= len(p.input) || p.input[p.pos] != ')' {
		return 0, fmt.Errorf("unmatched parenthesis in function %s", name)
	}
	p.pos++ // skip ')'

	if isDeg {
		arg = arg * math.Pi / 180
	}

	switch name {
	case "sqrt":
		if arg < 0 {
			return 0, fmt.Errorf("sqrt of negative number")
		}
		return math.Sqrt(arg), nil
	case "abs":
		return math.Abs(arg), nil
	case "sin":
		return math.Sin(arg), nil
	case "cos":
		return math.Cos(arg), nil
	case "tan":
		return math.Tan(arg), nil
	case "log", "log10":
		if arg <= 0 {
			return 0, fmt.Errorf("log of non-positive number")
		}
		return math.Log10(arg), nil
	case "log2":
		if arg <= 0 {
			return 0, fmt.Errorf("log of non-positive number")
		}
		return math.Log2(arg), nil
	case "ln":
		if arg <= 0 {
			return 0, fmt.Errorf("log of non-positive number")
		}
		return math.Log(arg), nil
	case "ceil":
		return math.Ceil(arg), nil
	case "floor":
		return math.Floor(arg), nil
	case "round":
		return math.Round(arg), nil
	default:
		return 0, fmt.Errorf("unknown function: %s", name)
	}
}

func (p *parser) skipSpaces() {
	for p.pos < len(p.input) && (p.input[p.pos] == ' ' || p.input[p.pos] == '\t') {
		p.pos++
	}
}

func precedence(op byte) int {
	switch op {
	case '+', '-':
		return 1
	case '*', '/', '%':
		return 2
	case '^':
		return 3
	default:
		return -1
	}
}

func formatResult(v float64) string {
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatFloat(v, 'f', 0, 64)
	}
	s := strconv.FormatFloat(v, 'f', -1, 64)
	// Clean up trailing zeros after decimal.
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}
