package mcpproxy

import (
	"context"
	"net/http"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	proxymcp "github.com/jbeshir/demesne/internal/proxies/mcp"
)

const (
	textType         = "text"
	imageDataAbc     = "abc"
	imgFileURL       = "file:///tmp/img.png"
	imgHostPath      = "/tmp/img.png"
	imgSandboxPath   = "/workspace/generated/img.png"
	assetHostPath    = "/tmp/assets-mcp/icon.svg"
	assetSandboxPath = "/workspace/generated/icon.svg"
	assetPathA       = "/tmp/assets-mcp/a.svg"
	countKey         = "count"
)

func TestParentFromHTTP_LiftsHeader(t *testing.T) {
	ctx := t.Context()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	require.NoError(t, err)
	req.Header.Set(proxymcp.ParentHeader, "job-123")

	got := parentFromHTTP(ctx, req)
	assert.Equal(t, "job-123", ParentJobIDFromContext(got))

	reqNoHeader, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	require.NoError(t, err)
	gotEmpty := parentFromHTTP(ctx, reqNoHeader)
	assert.Empty(t, ParentJobIDFromContext(gotEmpty))
}

func TestIsFileGenServer(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{serverMermaid, true},
		{serverImageGen, true},
		{serverAssets, true},
		{serverWorkflowy, false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsFileGenServer(tc.name))
		})
	}
}

func TestFileGenAdapterFor(t *testing.T) {
	cases := []struct {
		server  string
		tool    string
		wantNil bool
	}{
		{serverMermaid, toolGenerate, false},
		{serverImageGen, toolGenerateImg, false},
		{serverImageGen, toolEditImage, false},
		{serverAssets, toolGetIcon, false},
		{serverAssets, toolGetIllustration, false},
		{serverAssets, toolGetFont, false},
		{serverWorkflowy, toolSearchNodes, true},
		{serverMermaid, toolEditImage, true},
		{serverImageGen, toolGenerate, true},
		{serverAssets, "search_icons", true},
		{serverAssets, "list_asset_sources", true},
		{"", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.server+"/"+tc.tool, func(t *testing.T) {
			a := FileGenAdapterFor(tc.server, tc.tool)
			if tc.wantNil {
				assert.Nil(t, a)
			} else {
				assert.NotNil(t, a)
			}
		})
	}
}

func TestMermaidAdapter_PrepareArgs(t *testing.T) {
	a := mermaidAdapter{}
	const hostDir = "/tmp/delivery"

	t.Run("nil args", func(t *testing.T) {
		out := a.PrepareArgs(nil, hostDir)
		require.NotNil(t, out)
		assert.Equal(t, hostDir, out["folder"])
		assert.NotEmpty(t, out["name"])
	})

	t.Run("pre-existing key preserved", func(t *testing.T) {
		in := map[string]any{"format": "svg"}
		out := a.PrepareArgs(in, hostDir)
		assert.Equal(t, "svg", out["format"])
		assert.Equal(t, hostDir, out["folder"])
	})

	t.Run("mutation isolation", func(t *testing.T) {
		in := map[string]any{"k": "v"}
		out := a.PrepareArgs(in, hostDir)
		out["injected"] = "x"
		_, found := in["injected"]
		assert.False(t, found)
	})

	t.Run("unique names across calls", func(t *testing.T) {
		n1 := a.PrepareArgs(nil, hostDir)["name"]
		n2 := a.PrepareArgs(nil, hostDir)["name"]
		assert.NotEqual(t, n1, n2)
	})
}

func TestMermaidAdapter_ExtractHostPaths_Various(t *testing.T) {
	a := mermaidAdapter{}

	cases := []struct {
		name    string
		content []mcp.Content
		want    []string
	}{
		{
			name: "also saved to",
			content: []mcp.Content{
				mcp.TextContent{Type: textType, Text: "PNG also saved to: /tmp/x.png"},
			},
			want: []string{"/tmp/x.png"},
		},
		{
			name: "saved to without also",
			content: []mcp.Content{
				mcp.TextContent{Type: textType, Text: "SVG saved to: /tmp/y.svg"},
			},
			want: []string{"/tmp/y.svg"},
		},
		{
			name: "image content ignored for path extraction",
			content: []mcp.Content{
				mcp.ImageContent{Type: "image", Data: imageDataAbc, MIMEType: "image/png"},
				mcp.TextContent{Type: textType, Text: "PNG also saved to: /tmp/z.png"},
			},
			want: []string{"/tmp/z.png"},
		},
		{
			name:    "empty content",
			content: nil,
			want:    nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := &mcp.CallToolResult{Content: tc.content}
			got := a.ExtractHostPaths(result)
			if len(tc.want) == 0 {
				assert.Empty(t, got)
			} else {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestMermaidAdapter_RewriteResult(t *testing.T) {
	a := mermaidAdapter{}
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.ImageContent{Type: "image", Data: imageDataAbc, MIMEType: "image/png"},
			mcp.TextContent{Type: textType, Text: "PNG also saved to: /tmp/x.png"},
		},
	}
	mapping := map[string]string{"/tmp/x.png": "/workspace/generated/x.png"}

	a.RewriteResult(result, mapping)

	require.Len(t, result.Content, 1)
	tc, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, "PNG also saved to: /workspace/generated/x.png", tc.Text)
}

func TestImageGenAdapter_ExtractHostPaths(t *testing.T) {
	a := imageGenAdapter{}

	t.Run("structured content only", func(t *testing.T) {
		result := &mcp.CallToolResult{
			StructuredContent: map[string]any{imageURLKey: imgFileURL},
		}
		got := a.ExtractHostPaths(result)
		assert.Equal(t, []string{imgHostPath}, got)
	})

	t.Run("json text block", func(t *testing.T) {
		result := &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: textType, Text: `{"image_url":"file:///tmp/img.png","seed":1}`},
			},
		}
		got := a.ExtractHostPaths(result)
		assert.Equal(t, []string{imgHostPath}, got)
	})

	t.Run("both deduped", func(t *testing.T) {
		result := &mcp.CallToolResult{
			StructuredContent: map[string]any{imageURLKey: imgFileURL},
			Content: []mcp.Content{
				mcp.TextContent{Type: textType, Text: `{"image_url":"file:///tmp/img.png","seed":1}`},
			},
		}
		got := a.ExtractHostPaths(result)
		assert.Equal(t, []string{imgHostPath}, got)
	})

	t.Run("non-file scheme skipped", func(t *testing.T) {
		result := &mcp.CallToolResult{
			StructuredContent: map[string]any{imageURLKey: "https://example.com/img.png"},
		}
		got := a.ExtractHostPaths(result)
		assert.Empty(t, got)
	})

	t.Run("structured content not a map", func(t *testing.T) {
		result := &mcp.CallToolResult{
			StructuredContent: "just a string",
		}
		got := a.ExtractHostPaths(result)
		assert.Empty(t, got)
	})
}

func TestImageGenAdapter_RewriteResult(t *testing.T) {
	a := imageGenAdapter{}
	sc := map[string]any{imageURLKey: imgFileURL}
	result := &mcp.CallToolResult{
		StructuredContent: sc,
		Content: []mcp.Content{
			mcp.TextContent{Type: textType, Text: `{"image_url":"file:///tmp/img.png"}`},
		},
	}
	mapping := map[string]string{imgHostPath: imgSandboxPath}

	a.RewriteResult(result, mapping)

	gotSC, ok := result.StructuredContent.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, imgSandboxPath, gotSC[imageURLKey])

	tc, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, tc.Text, imgSandboxPath)
	assert.NotContains(t, tc.Text, "file://")
}

func TestAssetsAdapter_ServerAndTools(t *testing.T) {
	a := assetsAdapter{}
	assert.Equal(t, serverAssets, a.Server())
	assert.ElementsMatch(t, []string{toolGetIcon, toolGetIllustration, toolGetFont}, a.Tools())
}

func TestAssetsAdapter_PrepareArgs_NoOp(t *testing.T) {
	a := assetsAdapter{}
	in := map[string]any{"set": "lucide", "name": "camera"}
	out := a.PrepareArgs(in, "/tmp/delivery")
	assert.Equal(t, in, out)
}

func TestAssetsAdapter_ExtractHostPaths(t *testing.T) {
	a := assetsAdapter{}

	t.Run("single file", func(t *testing.T) {
		result := &mcp.CallToolResult{
			StructuredContent: map[string]any{
				filesKey: []any{
					map[string]any{pathKey: assetHostPath, "kind": "icon"},
				},
				countKey: 1,
			},
		}
		got := a.ExtractHostPaths(result)
		assert.Equal(t, []string{assetHostPath}, got)
	})

	t.Run("multiple files deduped", func(t *testing.T) {
		result := &mcp.CallToolResult{
			StructuredContent: map[string]any{
				filesKey: []any{
					map[string]any{pathKey: assetPathA},
					map[string]any{pathKey: "/tmp/assets-mcp/b.woff2"},
					map[string]any{pathKey: assetPathA},
				},
				countKey: 2,
			},
		}
		got := a.ExtractHostPaths(result)
		assert.Equal(t, []string{assetPathA, "/tmp/assets-mcp/b.woff2"}, got)
	})

	t.Run("no files", func(t *testing.T) {
		result := &mcp.CallToolResult{
			StructuredContent: map[string]any{filesKey: []any{}, countKey: 0},
		}
		got := a.ExtractHostPaths(result)
		assert.Empty(t, got)
	})

	t.Run("structured content not a map", func(t *testing.T) {
		result := &mcp.CallToolResult{StructuredContent: "just a string"}
		got := a.ExtractHostPaths(result)
		assert.Empty(t, got)
	})

	t.Run("no structured content", func(t *testing.T) {
		result := &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: textType, Text: "Wrote icon to " + assetHostPath},
			},
		}
		got := a.ExtractHostPaths(result)
		assert.Empty(t, got)
	})
}

func TestAssetsAdapter_RewriteResult(t *testing.T) {
	a := assetsAdapter{}
	sc := map[string]any{
		filesKey: []any{
			map[string]any{pathKey: assetHostPath, "kind": "icon"},
		},
		countKey: 1,
	}
	result := &mcp.CallToolResult{
		StructuredContent: sc,
		Content: []mcp.Content{
			mcp.TextContent{Type: textType, Text: "Wrote icon to " + assetHostPath},
		},
	}
	mapping := map[string]string{assetHostPath: assetSandboxPath}

	a.RewriteResult(result, mapping)

	files, ok := sc[filesKey].([]any)
	require.True(t, ok)
	entry, ok := files[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, assetSandboxPath, entry[pathKey])

	tc, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, tc.Text, assetSandboxPath)
	assert.NotContains(t, tc.Text, assetHostPath)
}
