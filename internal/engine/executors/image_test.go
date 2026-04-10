package executors

import (
	"testing"

	"github.com/openparallax/openparallax/internal/generation"
	"github.com/stretchr/testify/assert"
)

func TestImageExecutorNilWhenNoProvider(t *testing.T) {
	e := NewImageExecutor(generation.ProviderConfig{}, "/tmp/ws", nil)
	assert.Nil(t, e)
}

func TestImageExecutorNilWhenNone(t *testing.T) {
	e := NewImageExecutor(generation.ProviderConfig{Provider: "none"}, "/tmp/ws", nil)
	assert.Nil(t, e)
}

func TestVideoExecutorNilWhenNoProvider(t *testing.T) {
	e := NewVideoExecutor(generation.ProviderConfig{}, "/tmp/ws", nil)
	assert.Nil(t, e)
}

func TestVideoExecutorNilWhenNone(t *testing.T) {
	e := NewVideoExecutor(generation.ProviderConfig{Provider: "none"}, "/tmp/ws", nil)
	assert.Nil(t, e)
}
