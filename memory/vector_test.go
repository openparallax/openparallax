package memory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCosineSimilarity_IdenticalVectors(t *testing.T) {
	v := []float32{1.0, 2.0, 3.0}
	score := CosineSimilarity(v, v)
	assert.InDelta(t, 1.0, score, 0.001)
}

func TestCosineSimilarity_OrthogonalVectors(t *testing.T) {
	a := []float32{1.0, 0.0}
	b := []float32{0.0, 1.0}
	score := CosineSimilarity(a, b)
	assert.InDelta(t, 0.0, score, 0.001)
}

func TestCosineSimilarity_DifferentLength(t *testing.T) {
	a := []float32{1.0, 2.0}
	b := []float32{1.0}
	score := CosineSimilarity(a, b)
	assert.Equal(t, 0.0, score)
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := []float32{0.0, 0.0}
	b := []float32{1.0, 2.0}
	score := CosineSimilarity(a, b)
	assert.Equal(t, 0.0, score)
}

func TestBuiltinVectorSearcher_InsertAndSearch(t *testing.T) {
	vs := NewBuiltinVectorSearcher()

	require.NoError(t, vs.Insert("a", []float32{1.0, 0.0, 0.0}))
	require.NoError(t, vs.Insert("b", []float32{0.0, 1.0, 0.0}))
	require.NoError(t, vs.Insert("c", []float32{0.9, 0.1, 0.0}))

	results, err := vs.Search([]float32{1.0, 0.0, 0.0}, 2)
	require.NoError(t, err)
	require.Len(t, results, 2)

	assert.Equal(t, "a", results[0].ID, "most similar should be 'a'")
	assert.Equal(t, "c", results[1].ID, "second most similar should be 'c'")
}

func TestBuiltinVectorSearcher_Delete(t *testing.T) {
	vs := NewBuiltinVectorSearcher()

	require.NoError(t, vs.Insert("a", []float32{1.0, 0.0}))
	require.NoError(t, vs.Insert("b", []float32{0.0, 1.0}))
	assert.Equal(t, 2, vs.Count())

	require.NoError(t, vs.Delete("a"))
	assert.Equal(t, 1, vs.Count())

	results, err := vs.Search([]float32{1.0, 0.0}, 5)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "b", results[0].ID)
}

func TestBuiltinVectorSearcher_SearchEmpty(t *testing.T) {
	vs := NewBuiltinVectorSearcher()
	results, err := vs.Search([]float32{1.0, 0.0}, 5)
	require.NoError(t, err)
	assert.Empty(t, results)
}
