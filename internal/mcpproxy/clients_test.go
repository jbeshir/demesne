package mcpproxy

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	tests := []struct {
		name string
		env  map[string]string
		want []string
	}{
		{
			name: "nil map",
			env:  nil,
			want: nil,
		},
		{
			name: "empty map",
			env:  map[string]string{},
			want: nil,
		},
		{
			name: "multiple values",
			env:  map[string]string{"A": "1", "B": "2"},
			want: []string{"A=1", "B=2"},
		},
		{
			name: "value may contain equals",
			env:  map[string]string{testEnvTokenName: "a=b=c"},
			want: []string{testEnvTokenName + "=a=b=c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatch(t, tt.want, envSlice(tt.env))
		})
	}
}

func TestPool_AcquireUnknownServer_NewMethods(t *testing.T) {
	p := NewPool([]UpstreamSpec{{Name: "known", Command: "/bin/true"}})
	ctx := context.Background()

	t.Run("ListUpstreamResources", func(t *testing.T) {
		_, err := p.ListUpstreamResources(ctx, "unknown")
		assert.ErrorIs(t, err, ErrNotRegistered)
	})

	t.Run("ListUpstreamResourceTemplates", func(t *testing.T) {
		_, err := p.ListUpstreamResourceTemplates(ctx, "unknown")
		assert.ErrorIs(t, err, ErrNotRegistered)
	})

	t.Run("ListUpstreamPrompts", func(t *testing.T) {
		_, err := p.ListUpstreamPrompts(ctx, "unknown")
		assert.ErrorIs(t, err, ErrNotRegistered)
	})

	t.Run("ReadResource", func(t *testing.T) {
		_, err := p.ReadResource(ctx, "unknown", mcp.ReadResourceRequest{})
		assert.ErrorIs(t, err, ErrNotRegistered)
	})

	t.Run("GetPrompt", func(t *testing.T) {
		_, err := p.GetPrompt(ctx, "unknown", mcp.GetPromptRequest{})
		assert.ErrorIs(t, err, ErrNotRegistered)
	})

	t.Run("Complete", func(t *testing.T) {
		_, err := p.Complete(ctx, "unknown", mcp.CompleteRequest{})
		assert.ErrorIs(t, err, ErrNotRegistered)
	})
}

// TestPool_EvictGenerationGuard verifies that a queued evict goroutine
// carrying a stale generation is a no-op, and that an evict with the
// current generation removes the entry.
func TestPool_EvictGenerationGuard(t *testing.T) {
	// NewInProcessClient gives us a *client.Client whose Close() is safe
	// to call without spawning a real subprocess.
	stubSrv := server.NewMCPServer("stub", "0")

	t.Run("stale generation is a no-op", func(t *testing.T) {
		c, err := client.NewInProcessClient(stubSrv)
		require.NoError(t, err)

		p := &Pool{
			specs:   map[string]UpstreamSpec{"s": {Name: "s"}},
			clients: make(map[string]*upstreamClient),
			timeout: idleTimeout,
		}
		uc := &upstreamClient{c: c, generation: 5}
		p.clients["s"] = uc

		p.evict("s", 4) // stale: generation mismatch → must be a no-op

		got, ok := p.clients["s"]
		require.True(t, ok, "evict with stale generation must not remove the entry")
		assert.Same(t, uc, got)
	})

	t.Run("matching generation removes entry", func(t *testing.T) {
		c, err := client.NewInProcessClient(stubSrv)
		require.NoError(t, err)

		p := &Pool{
			specs:   map[string]UpstreamSpec{"s": {Name: "s"}},
			clients: make(map[string]*upstreamClient),
			timeout: idleTimeout,
		}
		uc := &upstreamClient{c: c, generation: 5}
		p.clients["s"] = uc

		p.evict("s", 5) // matching: client is closed and entry removed

		_, ok := p.clients["s"]
		assert.False(t, ok, "evict with matching generation must remove the entry")
	})
}

func TestPool_KnownSpecsSortedAndDuplicateNames(t *testing.T) {
	p := NewPool([]UpstreamSpec{
		{Name: "zeta", Command: "/bin/zeta"},
		{Name: stubAlpha, Command: "/bin/old-alpha"},
		{Name: stubAlpha, Command: "/bin/new-alpha"},
	})

	got := p.knownSpecs()
	require.Len(t, got, 2)
	assert.Equal(t, stubAlpha, got[0].Name)
	assert.Equal(t, "/bin/new-alpha", got[0].Command)
	assert.Equal(t, "zeta", got[1].Name)
}
