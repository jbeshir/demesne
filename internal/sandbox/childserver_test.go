package sandbox

import (
	"context"
	"net/http"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	proxymcp "github.com/jbeshir/demesne/internal/proxies/mcp"
)

func TestValidateChildName(t *testing.T) {
	valid := []string{"a", "probe-1", "my_child.v2", "ABC"}
	for _, n := range valid {
		require.NoError(t, validateChildName(n), n)
	}
	bad := []string{"", "..", ".", "a/b", "a b", "a:b", "../escape"}
	for _, n := range bad {
		require.Error(t, validateChildName(n), n)
	}
}

func TestReserveName_Unique(t *testing.T) {
	c := &childContext{usedNames: map[string]bool{}}
	require.NoError(t, c.reserveName("alpha"))
	require.Error(t, c.reserveName("alpha"), "duplicate must be rejected")
	require.NoError(t, c.reserveName("beta"))
}

func TestChildMCPServer_Catalogue(t *testing.T) {
	r := NewRunner(Config{})
	name, tools, handler := r.ChildMCPServer()
	assert.Equal(t, DemesneServerName, name)
	require.NotNil(t, handler)

	got := map[string]bool{}
	for _, tl := range tools {
		got[tl.Name] = true
	}
	for _, want := range []string{
		"sandbox_script", "sandbox_agent", "sandbox_research",
		"sandbox_create", "sandbox_exec", "sandbox_destroy",
	} {
		assert.True(t, got[want], "missing tool %q", want)
	}
	// upload/download are intentionally not exposed in-sandbox.
	assert.False(t, got["sandbox_upload"])
	assert.False(t, got["sandbox_download"])
}

func TestParentFor(t *testing.T) {
	r := NewRunner(Config{})
	parent := &childContext{usedNames: map[string]bool{}}
	r.registerChild("job-7", parent)

	// Header → context → lookup.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://x/demesne/mcp", nil)
	require.NoError(t, err)
	req.Header.Set(proxymcp.ParentHeader, "job-7")
	ctx := parentFromRequest(context.Background(), req)

	got, err := r.parentFor(ctx)
	require.NoError(t, err)
	assert.Same(t, parent, got)

	// No header → error.
	_, err = r.parentFor(context.Background())
	require.Error(t, err)

	// Unknown jobID → error.
	bad, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://x/demesne/mcp", nil)
	require.NoError(t, err)
	bad.Header.Set(proxymcp.ParentHeader, "nope")
	_, err = r.parentFor(parentFromRequest(context.Background(), bad))
	require.Error(t, err)
}

func TestHandleChildAgent_RejectsOpenEgress(t *testing.T) {
	r := NewRunner(Config{})
	parent := &childContext{usedNames: map[string]bool{}}
	r.registerChild("job-9", parent)
	ctx := context.WithValue(context.Background(), parentKey, "job-9")

	req := mcp.CallToolRequest{}
	req.Params.Name = "sandbox_agent"
	req.Params.Arguments = map[string]any{
		"name":   "child",
		"prompt": "do a thing",
		"egress": "open",
	}
	res, err := r.handleChildAgent(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.IsError)
}

func TestHandleChildScript_NoParentIdentity(t *testing.T) {
	r := NewRunner(Config{})
	req := mcp.CallToolRequest{}
	req.Params.Name = "sandbox_script"
	req.Params.Arguments = map[string]any{"name": "x", "command": "echo hi"}
	res, err := r.handleChildScript(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, res.IsError)
}
