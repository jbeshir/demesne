package mcpproxy

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain lets this test binary double as a stub stdio MCP server
// when DEMESNE_TEST_STUB_MCP=1, so aggregator tests can spawn a
// real upstream subprocess without a separate binary.
func TestMain(m *testing.M) {
	if os.Getenv("DEMESNE_TEST_STUB_MCP") == "1" {
		runStubServer()
		return
	}
	os.Exit(m.Run())
}

// runStubServer serves two tools over stdio: "search_nodes"
// (allowlisted for workflowy) and "delete_node" (not allowlisted).
func runStubServer() {
	srv := server.NewMCPServer("stub", "0")
	srv.AddTool(mcp.Tool{
		Name:        "search_nodes",
		Description: "search stub",
	}, func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := req.GetString("query", "")
		return mcp.NewToolResultText("found:" + q), nil
	})
	srv.AddTool(mcp.Tool{
		Name:        "delete_node",
		Description: "delete stub",
	}, func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("deleted"), nil
	})
	_ = server.ServeStdio(srv)
}

// writeStubConfig writes a Claude-Code-shaped MCP config that points
// the "workflowy" server at this test binary running as the stub.
func writeStubConfig(t *testing.T) string {
	t.Helper()
	self, err := os.Executable()
	require.NoError(t, err)
	cfg := map[string]any{
		"mcpServers": map[string]any{
			"workflowy": map[string]any{
				"type":    "stdio",
				"command": self,
				"env":     map[string]string{"DEMESNE_TEST_STUB_MCP": "1"},
			},
		},
	}
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "claude.json")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}

func TestAggregator_RoundTrip(t *testing.T) {
	cfgPath := writeStubConfig(t)
	socketPath := filepath.Join(t.TempDir(), "agg.sock")
	agg, err := NewAggregator(Config{
		HostMCPConfigPath: cfgPath,
		SocketPath:        socketPath,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, agg.Start(ctx))
	defer func() { _ = agg.Shutdown(context.Background()) }()

	servers := agg.Servers()
	require.Equal(t, []string{"workflowy"}, servers)
	assert.Equal(t, socketPath, agg.SocketPath())

	// Catalogue must contain only the allowlisted tool.
	cat := agg.Catalogue()
	require.Contains(t, cat, "workflowy")
	require.Len(t, cat["workflowy"], 1)
	assert.Equal(t, "search_nodes", cat["workflowy"][0].Name)

	// Round-trip a real tools/call through the unix socket endpoint.
	unixClient := &http.Client{Transport: &http.Transport{
		DialContext: func(dialCtx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(dialCtx, "unix", socketPath)
		},
	}}
	httpClient, err := client.NewStreamableHttpClient(
		"http://demesne-mcp/workflowy/mcp",
		transport.WithHTTPBasicClient(unixClient),
	)
	require.NoError(t, err)
	defer func() { _ = httpClient.Close() }()
	require.NoError(t, httpClient.Start(ctx))

	initReq := mcp.InitializeRequest{}
	initReq.Params.ClientInfo = mcp.Implementation{Name: "test", Version: "0"}
	_, err = httpClient.Initialize(ctx, initReq)
	require.NoError(t, err)

	listed, err := httpClient.ListTools(ctx, mcp.ListToolsRequest{})
	require.NoError(t, err)
	require.Len(t, listed.Tools, 1)
	assert.Equal(t, "search_nodes", listed.Tools[0].Name)

	callReq := mcp.CallToolRequest{}
	callReq.Params.Name = "search_nodes"
	callReq.Params.Arguments = map[string]any{"query": "demesne"}
	res, err := httpClient.CallTool(ctx, callReq)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.NotEmpty(t, res.Content)
	textContent, ok := res.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, "found:demesne", textContent.Text)
}

func TestAggregator_DoubleStart(t *testing.T) {
	cfgPath := writeStubConfig(t)
	agg, err := NewAggregator(Config{
		HostMCPConfigPath: cfgPath,
		SocketPath:        filepath.Join(t.TempDir(), "agg.sock"),
	})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, agg.Start(ctx))
	defer func() { _ = agg.Shutdown(context.Background()) }()
	err = agg.Start(ctx)
	assert.ErrorContains(t, err, "already started")
}

func TestAggregator_NoUpstreams(t *testing.T) {
	path := filepath.Join(t.TempDir(), "claude.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"mcpServers":{}}`), 0o600))
	agg, err := NewAggregator(Config{
		HostMCPConfigPath: path,
		SocketPath:        filepath.Join(t.TempDir(), "agg.sock"),
	})
	require.NoError(t, err)
	err = agg.Start(context.Background())
	assert.ErrorContains(t, err, "no upstreams")
}

func TestNewAggregator_RequiresSocketPath(t *testing.T) {
	_, err := NewAggregator(Config{HostMCPConfigPath: writeStubConfig(t)})
	assert.ErrorContains(t, err, "SocketPath is required")
}

func TestFilterTools(t *testing.T) {
	tools := []mcp.Tool{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	got := filterTools(tools, ServerAllowlist{Tools: map[string]struct{}{"a": {}, "c": {}}})
	require.Len(t, got, 2)
	assert.Equal(t, "a", got[0].Name)
	assert.Equal(t, "c", got[1].Name)

	all := filterTools(tools, ServerAllowlist{AllowAll: true})
	assert.Len(t, all, 3)
}
