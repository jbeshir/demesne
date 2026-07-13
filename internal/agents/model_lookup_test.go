package agents

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These white-box tests read the unexported registry directly, so they
// cannot blank-import the vendor packages (anthropic/codex import this
// package, which would form a test import cycle). The vendor providers are
// registered for this package's test binary by the side-effect imports in
// agent_test.go (external agents_test package), which compiles into the
// same binary — so the registry is populated before these tests run.

// TestModelAliasUniqueness asserts that no model alias is claimed by
// more than one registered provider.
func TestModelAliasUniqueness(t *testing.T) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	seen := map[string]string{} // model alias → agent name
	for _, a := range registry {
		for _, m := range a.Models() {
			alias := string(m)
			if prev, exists := seen[alias]; exists {
				t.Errorf("model alias %q claimed by both %q and %q", alias, prev, a.Name())
			}
			seen[alias] = a.Name()
		}
	}
}

// TestLookupByModel verifies that known model aliases resolve to the
// correct provider and that an unknown alias returns ErrUnknownModel.
func TestLookupByModel(t *testing.T) {
	a, err := LookupByModel("sonnet")
	require.NoError(t, err)
	assert.Equal(t, "claude-code", a.Name())

	a, err = LookupByModel("gpt-5.6-sol")
	require.NoError(t, err)
	assert.Equal(t, "codex", a.Name())

	_, err = LookupByModel("nope")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnknownModel)
}
