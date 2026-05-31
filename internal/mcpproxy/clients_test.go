package mcpproxy

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
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
			env:  map[string]string{"TOKEN": "a=b=c"},
			want: []string{"TOKEN=a=b=c"},
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
