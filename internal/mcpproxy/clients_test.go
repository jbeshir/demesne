package mcpproxy

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestPool_ShutdownEmpty(t *testing.T) {
	p := NewPool(nil)
	// Should be a no-op, not a panic.
	p.Shutdown()
}

func TestPool_AcquireUnknownServer(t *testing.T) {
	p := NewPool([]UpstreamSpec{{Name: "known", Command: "/bin/true"}})
	_, err := p.CallTool(context.Background(), "unknown", mcp.CallToolRequest{})
	assert.ErrorIs(t, err, ErrNotRegistered)
}

func TestEnvSlice(t *testing.T) {
	got := envSlice(map[string]string{"A": "1", "B": "2"})
	assert.ElementsMatch(t, []string{"A=1", "B=2"}, got)
	assert.Nil(t, envSlice(nil))
}
