package agent

import (
	"encoding/json"
	"fmt"
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
	actionType := extractField(block, "ACTION:")
	if actionType == "" {
		return nil, fmt.Errorf("no ACTION found in block")
	}
	at := types.ActionType(strings.TrimSpace(strings.ToLower(actionType)))

	payload := make(map[string]any)
	paramsRaw := extractField(block, "PARAMS:")
	if paramsRaw != "" {
		if err := json.Unmarshal([]byte(paramsRaw), &payload); err != nil {
			payload["raw"] = paramsRaw
		}
	}

	hash, err := crypto.HashAction(string(at), payload)
	if err != nil {
		return nil, fmt.Errorf("hash computation failed: %w", err)
	}

	return &types.ActionRequest{
		RequestID: crypto.NewID(),
		Type:      at,
		Payload:   payload,
		Hash:      hash,
		Timestamp: time.Now(),
	}, nil
}

// extractField extracts the content after a field marker (e.g., "PARAMS:").
// For single-line JSON, returns the rest of the line.
// For multiline JSON, captures everything until the next field marker or end of block.
func extractField(block, marker string) string {
	lines := strings.Split(block, "\n")

	// Known field markers that terminate a value.
	markers := []string{"ACTION:", "PARAMS:", "REASONING:"}

	capturing := false
	var captured strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, marker) {
			// Start capturing: take everything after the marker on this line.
			rest := strings.TrimSpace(trimmed[len(marker):])
			captured.WriteString(rest)
			capturing = true
			continue
		}

		if capturing {
			// Stop if we hit another field marker.
			isMarker := false
			for _, m := range markers {
				if strings.HasPrefix(trimmed, m) {
					isMarker = true
					break
				}
			}
			if isMarker {
				break
			}
			if captured.Len() > 0 {
				captured.WriteString("\n")
			}
			captured.WriteString(line)
		}
	}

	return strings.TrimSpace(captured.String())
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
