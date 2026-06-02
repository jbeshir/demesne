package agents_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jbeshir/demesne/internal/agents"
	// Trigger the anthropic init so "claude-code" is registered before any test runs.
	_ "github.com/jbeshir/demesne/internal/agents/anthropic"
)

func TestLookup_Default(t *testing.T) {
	a, err := agents.Lookup("")
	require.NoError(t, err)
	assert.Equal(t, agents.DefaultAgent, a.Name())
}

func TestLookup_Unknown(t *testing.T) {
	_, err := agents.Lookup("unknown-thing-not-registered")
	require.Error(t, err)
	assert.ErrorIs(t, err, agents.ErrUnknownAgent)
}

func TestRegister_DuplicatePanics(t *testing.T) {
	// "claude-code" is registered by the anthropic init import above.
	// Registering the same name again must panic. The global registry is
	// permanently mutated by the first Register call, so this test relies
	// on an already-registered name rather than introducing a new one that
	// would pollute the registry for other tests.
	assert.Panics(t, func() {
		agents.Register(agents.DefaultAgent, nil)
	})
}
