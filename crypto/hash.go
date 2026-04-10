// Package crypto provides cryptographic utilities for hashing, canonicalization,
// canary token management, and secure random generation.
package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// SHA256Hex computes the SHA-256 hash of the input and returns it as a hex string.
func SHA256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// Canonicalize produces a deterministic JSON representation of any value.
// Keys are sorted alphabetically at every nesting level. This ensures the
// same logical content always produces the same hash regardless of key ordering.
func Canonicalize(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var obj any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}

	return marshalSorted(obj)
}

// HashAction computes the SHA-256 hash of a canonicalized ActionRequest.
// This hash is computed at proposal time and verified before execution
// to prevent TOCTOU (time-of-check-to-time-of-use) attacks.
func HashAction(actionType string, payload map[string]any) (string, error) {
	obj := map[string]any{
		"type":    actionType,
		"payload": payload,
	}
	canonical, err := Canonicalize(obj)
	if err != nil {
		return "", err
	}
	return SHA256Hex(canonical), nil
}

// marshalSorted recursively serializes a value to JSON with sorted map keys.
func marshalSorted(v any) ([]byte, error) {
	switch val := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		result := []byte("{")
		for i, k := range keys {
			if i > 0 {
				result = append(result, ',')
			}
			keyBytes, err := json.Marshal(k)
			if err != nil {
				return nil, err
			}
			result = append(result, keyBytes...)
			result = append(result, ':')
			valBytes, err := marshalSorted(val[k])
			if err != nil {
				return nil, err
			}
			result = append(result, valBytes...)
		}
		result = append(result, '}')
		return result, nil

	case []any:
		result := []byte("[")
		for i, item := range val {
			if i > 0 {
				result = append(result, ',')
			}
			itemBytes, err := marshalSorted(item)
			if err != nil {
				return nil, err
			}
			result = append(result, itemBytes...)
		}
		result = append(result, ']')
		return result, nil

	default:
		return json.Marshal(v)
	}
}
