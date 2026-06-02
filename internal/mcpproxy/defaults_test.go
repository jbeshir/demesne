package mcpproxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDefaultAllowlist_WorkflowyPins is a regression ratchet: it pins
// representative read-only tools as present and obviously-mutating names as
// absent. Any future change to the default allowlist that removes a read-only
// tool or adds a write tool will break this test intentionally.
func TestDefaultAllowlist_WorkflowyPins(t *testing.T) {
	wf := defaultAllowlist[serverWorkflowy]

	// Must be present (confirmed read-only).
	_, ok := wf[toolSearchNodes]
	assert.True(t, ok, "workflowy: search_nodes should be in default allowlist")
	_, ok = wf["list_targets"]
	assert.True(t, ok, "workflowy: list_targets should be in default allowlist")

	// Must be absent (write/mutating operations — not defined upstream today,
	// but pinned here to guard against accidental future addition).
	_, ok = wf["create_node"]
	assert.False(t, ok, "workflowy: create_node must not be in default allowlist")
	_, ok = wf["update_node"]
	assert.False(t, ok, "workflowy: update_node must not be in default allowlist")
	_, ok = wf["delete_node"]
	assert.False(t, ok, "workflowy: delete_node must not be in default allowlist")
}

// TestDefaultAllowlist_AnkiPins pins read-only anki query tools as present
// and write operations as absent.
func TestDefaultAllowlist_AnkiPins(t *testing.T) {
	anki := defaultAllowlist["anki"]

	// Must be present (confirmed read-only).
	_, ok := anki["findNotes"]
	assert.True(t, ok, "anki: findNotes should be in default allowlist")
	_, ok = anki["getTags"]
	assert.True(t, ok, "anki: getTags should be in default allowlist")

	// Must be absent (write/mutating operations).
	_, ok = anki["addNote"]
	assert.False(t, ok, "anki: addNote must not be in default allowlist")
	_, ok = anki["deleteCards"]
	assert.False(t, ok, "anki: deleteCards must not be in default allowlist")
}
