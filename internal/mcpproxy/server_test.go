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
		Name:        toolSearchNodes,
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

const (
	testSandboxScript = "sandbox_script"
	testAggSock       = "agg.sock"
	testClaudeJSON    = "claude.json"
)

// writeStubConfig writes a Claude-Code-shaped MCP config that points
// the "workflowy" server at this test binary running as the stub.
func writeStubConfig(t *testing.T) string {
	t.Helper()
	self, err := os.Executable()
	require.NoError(t, err)
	cfg := map[string]any{
		"mcpServers": map[string]any{
			serverWorkflowy: map[string]any{
				"type":    "stdio",
				"command": self,
				"env":     map[string]string{"DEMESNE_TEST_STUB_MCP": "1"},
			},
		},
	}
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), testClaudeJSON)
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}

func TestAggregator_RoundTrip(t *testing.T) {
	cfgPath := writeStubConfig(t)
	socketPath := filepath.Join(t.TempDir(), testAggSock)
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
	require.Equal(t, []string{serverWorkflowy}, servers)
	assert.Equal(t, socketPath, agg.SocketPath())

	// Catalogue must contain only the allowlisted tool.
	cat := agg.Catalogue()
	require.Contains(t, cat, serverWorkflowy)
	require.Len(t, cat[serverWorkflowy], 1)
	assert.Equal(t, toolSearchNodes, cat[serverWorkflowy][0].Name)

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
	assert.Equal(t, toolSearchNodes, listed.Tools[0].Name)

	callReq := mcp.CallToolRequest{}
	callReq.Params.Name = toolSearchNodes
	callReq.Params.Arguments = map[string]any{"query": DemesneServerName}
	res, err := httpClient.CallTool(ctx, callReq)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.NotEmpty(t, res.Content)
	textContent, ok := res.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, "found:"+DemesneServerName, textContent.Text)
}

func TestAggregator_ExtraServer(t *testing.T) {
	// No stdio upstreams; an extra in-process server alone must bring
	// the aggregator up and appear in Servers()/Catalogue().
	cfgPath := filepath.Join(t.TempDir(), testClaudeJSON)
	require.NoError(t, os.WriteFile(cfgPath, []byte(`{"mcpServers":{}}`), 0o600))
	socketPath := filepath.Join(t.TempDir(), testAggSock)

	extraSrv := server.NewMCPServer(DemesneServerName, "0")
	extraSrv.AddTool(mcp.Tool{Name: testSandboxScript, Description: "spawn"},
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return mcp.NewToolResultText("ok"), nil
		})

	agg, err := NewAggregator(Config{
		HostMCPConfigPath: cfgPath,
		SocketPath:        socketPath,
		ExtraServers: []ExtraServer{{
			Name:    DemesneServerName,
			Tools:   []mcp.Tool{{Name: testSandboxScript, Description: "spawn"}},
			Handler: server.NewStreamableHTTPServer(extraSrv),
		}},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, agg.Start(ctx))
	defer func() { _ = agg.Shutdown(context.Background()) }()

	require.Equal(t, []string{DemesneServerName}, agg.Servers())
	cat := agg.Catalogue()
	require.Contains(t, cat, DemesneServerName)
	require.Len(t, cat[DemesneServerName], 1)
	assert.Equal(t, testSandboxScript, cat[DemesneServerName][0].Name)
}

func TestAggregator_DoubleStart(t *testing.T) {
	cfgPath := writeStubConfig(t)
	agg, err := NewAggregator(Config{
		HostMCPConfigPath: cfgPath,
		SocketPath:        filepath.Join(t.TempDir(), testAggSock),
	})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, agg.Start(ctx))
	defer func() { _ = agg.Shutdown(context.Background()) }()
	err = agg.Start(ctx)
	assert.ErrorIs(t, err, ErrAlreadyStarted)
}

func TestAggregator_NoUpstreams(t *testing.T) {
	path := filepath.Join(t.TempDir(), testClaudeJSON)
	require.NoError(t, os.WriteFile(path, []byte(`{"mcpServers":{}}`), 0o600))
	agg, err := NewAggregator(Config{
		HostMCPConfigPath: path,
		SocketPath:        filepath.Join(t.TempDir(), testAggSock),
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
	got := filterTools(tools, ServerAllowlist{Tools: map[ToolName]struct{}{"a": {}, "c": {}}})
	require.Len(t, got, 2)
	assert.Equal(t, "a", got[0].Name)
	assert.Equal(t, "c", got[1].Name)

	all := filterTools(tools, ServerAllowlist{AllowAll: true})
	assert.Len(t, all, 3)
}
