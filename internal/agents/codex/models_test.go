package codex

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	proxyopenai "github.com/jbeshir/demesne/internal/proxies/openai"
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
		" gpt-5.6-sol",
		"gpt-5.6-sol ",
	}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ResolveModel(name)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrUnknownModel)
		})
	}
}

func TestResolveModel_RejectsRemoved(t *testing.T) {
	removed := []string{"gpt-5.4", "gpt-5.3-codex", "gpt-5.2"}
	for _, name := range removed {
		t.Run(name, func(t *testing.T) {
			_, err := ResolveModel(name)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrUnknownModel)
		})
	}
}

func TestModels_MatchCatalog(t *testing.T) {
	assert.Len(t, Models, 5)
	assert.Equal(t, ModelGPT56Sol, Models[0])
	assert.Equal(t, ModelGPT56Terra, Models[1])
	assert.Equal(t, ModelGPT56Luna, Models[2])
	assert.Equal(t, ModelGPT55, Models[3])
	assert.Equal(t, ModelGPT54Mini, Models[4])
	assert.Equal(t, string(ModelGPT56Sol), proxyopenai.Aliases()[0])
}

func TestAgent_Models(t *testing.T) {
	got := codexAgent{}.Models()
	assert.Equal(t, Models, got)
}
