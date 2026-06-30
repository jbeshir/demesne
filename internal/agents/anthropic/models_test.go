package anthropic

import (
	"testing"

	proxyanthropic "github.com/jbeshir/demesne/internal/proxies/anthropic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveModel_EmptyReturnsDefault(t *testing.T) {
	got, err := ResolveModel("")
	require.NoError(t, err)
	assert.Equal(t, DefaultModel, got)
}

func TestResolveModel_ValidPassthrough(t *testing.T) {
	for _, m := range Models {
		t.Run(string(m), func(t *testing.T) {
			got, err := ResolveModel(string(m))
			require.NoError(t, err)
			assert.Equal(t, m, got)
		})
	}
}

func TestResolveModel_RejectsRemoved(t *testing.T) {
	for _, name := range []string{"claude-opus-4-7", "claude-opus-4-8", "claude-sonnet-5"} {
		t.Run(name, func(t *testing.T) {
			_, err := ResolveModel(name)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrUnknownModel)
		})
	}
}

// TestModels_MatchCatalog asserts the agent allowlist stays in sync
// with the proxy catalog: every alias in Models has a non-empty
// pricing entry, and DefaultModel sits at catalog index 0.
func TestModels_MatchCatalog(t *testing.T) {
	aliases := proxyanthropic.Aliases()
	require.Len(t, Models, len(aliases))
	for i, a := range aliases {
		assert.Equal(t, ModelName(a), Models[i])
	}
	assert.Equal(t, string(DefaultModel), aliases[0])
}

func TestAgent_Models(t *testing.T) {
	got := claudeCodeAgent{}.Models()
	assert.Equal(t, Models, got)
}
