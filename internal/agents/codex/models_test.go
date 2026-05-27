package codex

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveModel_EmptyReturnsDefault(t *testing.T) {
	got, err := ResolveModel("")
	require.NoError(t, err)
	assert.Equal(t, DefaultModel, got)
}

func TestResolveModel_ValidPassthrough(t *testing.T) {
	got, err := ResolveModel("gpt-5.5")
	require.NoError(t, err)
	assert.Equal(t, ModelGPT55, got)
}

func TestResolveModel_UnknownError(t *testing.T) {
	_, err := ResolveModel("nonexistent-model")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnknownModel)
}
