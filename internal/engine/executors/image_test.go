package executors

import (
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestImageExecutorNilWhenNoProvider(t *testing.T) {
	e := NewImageExecutor(types.GenProviderConfig{}, "/tmp/ws", nil)
	assert.Nil(t, e)
}

func TestImageExecutorNilWhenNone(t *testing.T) {
	e := NewImageExecutor(types.GenProviderConfig{Provider: "none"}, "/tmp/ws", nil)
	assert.Nil(t, e)
}

func TestVideoExecutorNilWhenNoProvider(t *testing.T) {
	e := NewVideoExecutor(types.GenProviderConfig{}, "/tmp/ws", nil)
	assert.Nil(t, e)
}

func TestVideoExecutorNilWhenNone(t *testing.T) {
	e := NewVideoExecutor(types.GenProviderConfig{Provider: "none"}, "/tmp/ws", nil)
	assert.Nil(t, e)
}
