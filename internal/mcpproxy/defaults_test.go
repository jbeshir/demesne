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

// TestDefaultAllowlist_ImageGenMcpPins is a regression ratchet: it pins
// the read-only image-gen-mcp tools as present and mutating/privileged
// names as absent.
func TestDefaultAllowlist_ImageGenMcpPins(t *testing.T) {
	ig := defaultAllowlist["image-gen-mcp"]

	// Must be present (confirmed read/generate-only).
	_, ok := ig["generate_image"]
	assert.True(t, ok, "image-gen-mcp: generate_image should be in default allowlist")
	_, ok = ig["edit_image"]
	assert.True(t, ok, "image-gen-mcp: edit_image should be in default allowlist")
	_, ok = ig["list_available_models"]
	assert.True(t, ok, "image-gen-mcp: list_available_models should be in default allowlist")

	// Must be absent (write/privileged operations).
	_, ok = ig["delete_image"]
	assert.False(t, ok, "image-gen-mcp: delete_image must not be in default allowlist")
	_, ok = ig["set_api_key"]
	assert.False(t, ok, "image-gen-mcp: set_api_key must not be in default allowlist")
}

// TestDefaultAllowlist_MermaidPins is a regression ratchet: it pins
// the read-only mermaid tool as present and hypothetical mutating names
// as absent.
func TestDefaultAllowlist_MermaidPins(t *testing.T) {
	mm := defaultAllowlist["mermaid"]

	// Must be present (confirmed read/generate-only).
	_, ok := mm["generate"]
	assert.True(t, ok, "mermaid: generate should be in default allowlist")

	// Must be absent (mutating/hypothetical operations).
	_, ok = mm["delete"]
	assert.False(t, ok, "mermaid: delete must not be in default allowlist")
	_, ok = mm["render_inline"]
	assert.False(t, ok, "mermaid: render_inline must not be in default allowlist")
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
