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
		" gpt-6-sol",
		"gpt-6-sol ",
	}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ResolveModel(name)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrUnknownModel)
		})
	}
}

func TestResolveModel_RejectsUnlisted(t *testing.T) {
	unlisted := []string{"gpt-5.4", "gpt-5.4-mini", "gpt-5.3-codex", "gpt-5.2"}
	for _, name := range unlisted {
		t.Run(name, func(t *testing.T) {
			_, err := ResolveModel(name)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrUnknownModel)
		})
	}
}

func TestModels_MatchCatalog(t *testing.T) {
	assert.Equal(t, []ModelName{
		ModelGPT61Sol,
		ModelGPT6Astra,
		ModelGPT6Sol,
		ModelGPT6Luna,
		ModelGPT56Sol,
		ModelGPT56Terra,
		ModelGPT56Luna,
		ModelGPT55,
	}, Models)
	assert.Equal(t, ModelGPT61Sol, DefaultModel)
	assert.Equal(t, DefaultModel, Models[0])
	assert.Equal(t, string(DefaultModel), proxyopenai.Aliases()[0])
}

func TestAgent_Models(t *testing.T) {
	got := codexAgent{}.Models()
	assert.Equal(t, Models, got)
}
