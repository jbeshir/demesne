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

// TestDefaultAllowlist_AssetsPins is a regression ratchet: it pins all seven
// read-only assets tools as present and hypothetical mutating/privileged names
// as absent. Every assets tool only reads embedded data and writes into a local
// output dir, so all seven are safe to expose by default.
func TestDefaultAllowlist_AssetsPins(t *testing.T) {
	as := defaultAllowlist[serverAssets]

	// Must be present (all read-only: search/list and the file-producing gets).
	for _, tool := range []ToolName{
		"list_asset_sources",
		"search_icons",
		"get_icon",
		"search_illustrations",
		"get_illustration",
		"search_fonts",
		"get_font",
	} {
		_, ok := as[tool]
		assert.True(t, ok, "assets: %s should be in default allowlist", tool)
	}

	// Must be absent (mutating/privileged operations — guard against future addition).
	_, ok := as["delete_asset"]
	assert.False(t, ok, "assets: delete_asset must not be in default allowlist")
	_, ok = as["set_output_dir"]
	assert.False(t, ok, "assets: set_output_dir must not be in default allowlist")
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
