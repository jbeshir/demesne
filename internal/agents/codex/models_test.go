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
	for _, model := range Models {
		t.Run(string(model), func(t *testing.T) {
			got, err := ResolveModel(string(model))
			require.NoError(t, err)
			assert.Equal(t, model, got)
		})
	}
}

func TestResolveModel_UnknownError(t *testing.T) {
	tests := []string{
		"nonexistent-model",
		" ",
		"\t",
		" gpt-5.5",
		"gpt-5.5 ",
	}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ResolveModel(name)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrUnknownModel)
		})
	}
}
