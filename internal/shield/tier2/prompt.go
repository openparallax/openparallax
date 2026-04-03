// Package tier2 implements the LLM-based evaluator for Shield's third evaluation
// tier. It uses an independent LLM with canary token verification to detect
// prompt injection of the evaluator itself.
package tier2

import (
	"fmt"
	"os"
	"strings"

	"github.com/openparallax/openparallax/crypto"
)

// LoadPrompt reads the evaluator prompt from disk, verifies its integrity,
// and injects the canary token at the {{CANARY_TOKEN}} marker.
func LoadPrompt(promptPath string, canaryToken string) (string, string, error) {
	data, err := os.ReadFile(promptPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to load evaluator prompt: %w", err)
	}

	promptHash := crypto.SHA256Hex(data)
	prompt := strings.ReplaceAll(string(data), "{{CANARY_TOKEN}}", canaryToken)

	return prompt, promptHash, nil
}
