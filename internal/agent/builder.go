package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/openparallax/openparallax/internal/crypto"
	"github.com/openparallax/openparallax/internal/types"
)

// ActionBuilder parses the LLM's planning output into ActionRequests.
type ActionBuilder struct{}

// NewActionBuilder creates an ActionBuilder.
func NewActionBuilder() *ActionBuilder {
	return &ActionBuilder{}
}

var (
	actionPattern = regexp.MustCompile(`(?m)^ACTION:\s*(.+)$`)
	paramsPattern = regexp.MustCompile(`(?m)^PARAMS:\s*(.+)$`)
)

// Build parses the raw LLM plan into ActionRequests.
// Each action gets a unique request ID and a SHA-256 hash for integrity.
func (b *ActionBuilder) Build(rawPlan string) ([]*types.ActionRequest, error) {
	if strings.Contains(rawPlan, "ACTION: none") {
		return nil, nil
	}

	blocks := splitActionBlocks(rawPlan)

	actions := make([]*types.ActionRequest, 0, len(blocks))
	for _, block := range blocks {
		action, err := b.parseBlock(block)
		if err != nil {
			continue
		}
		actions = append(actions, action)
	}

	return actions, nil
}

func (b *ActionBuilder) parseBlock(block string) (*types.ActionRequest, error) {
	actionMatch := actionPattern.FindStringSubmatch(block)
	if len(actionMatch) < 2 {
		return nil, fmt.Errorf("no ACTION found in block")
	}
	actionType := types.ActionType(strings.TrimSpace(strings.ToLower(actionMatch[1])))

	payload := make(map[string]any)
	paramsMatch := paramsPattern.FindStringSubmatch(block)
	if len(paramsMatch) >= 2 {
		raw := strings.TrimSpace(paramsMatch[1])
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			payload["raw"] = raw
		}
	}

	hash, err := crypto.HashAction(string(actionType), payload)
	if err != nil {
		return nil, fmt.Errorf("hash computation failed: %w", err)
	}

	return &types.ActionRequest{
		RequestID: crypto.NewID(),
		Type:      actionType,
		Payload:   payload,
		Hash:      hash,
		Timestamp: time.Now(),
	}, nil
}

// splitActionBlocks splits the LLM plan into individual action blocks.
// Each block starts with an "ACTION:" line.
func splitActionBlocks(plan string) []string {
	lines := strings.Split(plan, "\n")
	var blocks []string
	var current strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "ACTION:") && current.Len() > 0 {
			blocks = append(blocks, current.String())
			current.Reset()
		}
		current.WriteString(line)
		current.WriteString("\n")
	}
	if current.Len() > 0 {
		blocks = append(blocks, current.String())
	}

	return blocks
}
