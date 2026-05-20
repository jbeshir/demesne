package mcpproxy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveAllowlist_NoOverride(t *testing.T) {
	got, err := ResolveAllowlist("")
	require.NoError(t, err)
	assert.Contains(t, got, "workflowy")
	wf := got["workflowy"]
	assert.False(t, wf.AllowAll)
	assert.True(t, wf.Allowed("search_nodes"))
	assert.False(t, wf.Allowed("delete_node"))
}

func TestResolveAllowlist_OverrideMissingFile(t *testing.T) {
	got, err := ResolveAllowlist("/nonexistent/path/mcp-allowlist.json")
	require.NoError(t, err)
	assert.Contains(t, got, "workflowy")
}

func TestApplyOverride(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		serverCheck func(t *testing.T, got map[string]ServerAllowlist)
	}{
		{
			name:  "explicit list narrows the default",
			input: `{"workflowy": ["search_nodes"]}`,
			serverCheck: func(t *testing.T, got map[string]ServerAllowlist) {
				wf := got["workflowy"]
				assert.True(t, wf.Allowed("search_nodes"))
				assert.False(t, wf.Allowed("get_node"))
			},
		},
		{
			name:  "asterisk means AllowAll",
			input: `{"alignment": "*"}`,
			serverCheck: func(t *testing.T, got map[string]ServerAllowlist) {
				assert.True(t, got["alignment"].AllowAll)
				assert.True(t, got["alignment"].Allowed("anything"))
			},
		},
		{
			name:  "default string keeps built-in",
			input: `{"bunpro": "default"}`,
			serverCheck: func(t *testing.T, got map[string]ServerAllowlist) {
				assert.True(t, got["bunpro"].Allowed("get_decks"))
			},
		},
		{
			name:  "empty list disables the server",
			input: `{"amazon": []}`,
			serverCheck: func(t *testing.T, got map[string]ServerAllowlist) {
				_, ok := got["amazon"]
				assert.False(t, ok)
			},
		},
		{
			name:  "doc metadata key is ignored",
			input: `{"_doc": "blah", "workflowy": "default"}`,
			serverCheck: func(t *testing.T, got map[string]ServerAllowlist) {
				_, ok := got["_doc"]
				assert.False(t, ok)
				assert.True(t, got["workflowy"].Allowed("search_nodes"))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := make(map[string]ServerAllowlist, len(defaultAllowlist))
			for name, set := range defaultAllowlist {
				base[name] = ServerAllowlist{Tools: cloneSet(set)}
			}
			got, err := applyOverride(base, []byte(tt.input))
			require.NoError(t, err)
			tt.serverCheck(t, got)
		})
	}
}

func TestApplyOverride_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unknown string sentinel", `{"workflowy": "yes"}`},
		{"default for unknown server", `{"never-heard-of-it": "default"}`},
		{"non-string non-list value", `{"workflowy": 5}`},
		{"empty tool name in list", `{"workflowy": [""]}`},
		{"malformed JSON", `{not json}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := map[string]ServerAllowlist{}
			_, err := applyOverride(base, []byte(tt.input))
			assert.Error(t, err)
		})
	}
}

func TestSeedOverrideFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp-allowlist.json")

	require.NoError(t, SeedOverrideFile(path))

	data, err := os.ReadFile(path) //nolint:gosec // path is a test temp file
	require.NoError(t, err)
	body := string(data)
	assert.Contains(t, body, "_doc")
	assert.Contains(t, body, `"workflowy": "default"`)
	assert.Contains(t, body, `"wanikani": "default"`)

	// Re-seeding must not overwrite an existing file.
	require.NoError(t, os.WriteFile(path, []byte("custom"), 0o600))
	require.NoError(t, SeedOverrideFile(path))
	data, err = os.ReadFile(path) //nolint:gosec // path is a test temp file
	require.NoError(t, err)
	assert.Equal(t, "custom", string(data))
}

func TestServerAllowlist_Allowed(t *testing.T) {
	a := ServerAllowlist{Tools: map[string]struct{}{"x": {}}}
	assert.True(t, a.Allowed("x"))
	assert.False(t, a.Allowed("y"))

	b := ServerAllowlist{AllowAll: true}
	assert.True(t, b.Allowed("anything"))
}
