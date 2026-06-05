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
	uritemplate "github.com/yosida95/uritemplate/v3"
)

const (
	stubAlpha            = "alpha"
	stubBeta             = "beta"
	stubGreetURI         = "mem://greet"
	stubGreetPromptName  = "greet-prompt"
	stubPromptArgSubject = "subject"
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

// stubCompleter returns fixed completion values for any prompt or
// resource argument, allowing completion relay tests to verify
// end-to-end forwarding without real upstream logic.
type stubCompleter struct{}

func (s *stubCompleter) CompletePromptArgument(_ context.Context, _ string, _ mcp.CompleteArgument, _ mcp.CompleteContext) (*mcp.Completion, error) {
	return &mcp.Completion{Values: []string{stubAlpha, stubBeta}}, nil
}

func (s *stubCompleter) CompleteResourceArgument(_ context.Context, _ string, _ mcp.CompleteArgument, _ mcp.CompleteContext) (*mcp.Completion, error) {
	return &mcp.Completion{Values: []string{stubAlpha, stubBeta}}, nil
}

// runStubServer serves tools, a resource, a resource template, and
// a prompt over stdio. Used as a real upstream subprocess in tests.
func runStubServer() {
	srv := server.NewMCPServer("stub", "0",
		server.WithResourceCapabilities(false, false),
		server.WithPromptCapabilities(false),
		server.WithCompletions(),
		server.WithPromptCompletionProvider(&stubCompleter{}),
		server.WithResourceCompletionProvider(&stubCompleter{}),
	)

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

	srv.AddResource(mcp.Resource{
		URI:         stubGreetURI,
		Name:        "greet",
		Description: "stub resource",
	}, func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return []mcp.ResourceContents{
			mcp.TextResourceContents{URI: stubGreetURI, MIMEType: "text/plain", Text: "hello-resource"},
		}, nil
	})

	uriTmpl, _ := uritemplate.New("mem://greet/{name}")
	srv.AddResourceTemplate(mcp.ResourceTemplate{
		Name:        "greet-template",
		URITemplate: &mcp.URITemplate{Template: uriTmpl},
	}, func(_ context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return []mcp.ResourceContents{
			mcp.TextResourceContents{URI: req.Params.URI, MIMEType: "text/plain", Text: req.Params.URI},
		}, nil
	})

	srv.AddPrompt(mcp.Prompt{
		Name:        stubGreetPromptName,
		Description: "stub prompt",
		Arguments:   []mcp.PromptArgument{{Name: stubPromptArgSubject}},
	}, func(_ context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		subject := req.Params.Arguments["subject"]
		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				{
					Role:    mcp.RoleUser,
					Content: mcp.TextContent{Type: "text", Text: "hello " + subject},
				},
			},
		}, nil
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

// dialUnix returns an http.Client whose transport dials the given
// unix socket path, used to reach the aggregator's HTTP listener.
func dialUnix(socketPath string) *http.Client {
	return &http.Client{Transport: &http.Transport{
		DialContext: func(dialCtx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(dialCtx, "unix", socketPath)
		},
	}}
}

// connectToUpstream creates, starts, and initializes an MCP HTTP
// client connected to /{serverWorkflowy}/mcp via the aggregator socket.
func connectToUpstream(t *testing.T, ctx context.Context, socketPath string) *client.Client {
	t.Helper()
	httpClient, err := client.NewStreamableHttpClient(
		"http://demesne-mcp/"+serverWorkflowy+"/mcp",
		transport.WithHTTPBasicClient(dialUnix(socketPath)),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = httpClient.Close() })
	require.NoError(t, httpClient.Start(ctx))
	initReq := mcp.InitializeRequest{}
	initReq.Params.ClientInfo = mcp.Implementation{Name: "test", Version: "0"}
	_, err = httpClient.Initialize(ctx, initReq)
	require.NoError(t, err)
	return httpClient
}

func TestAggregator_RoundTrip(t *testing.T) {
	cfgPath := writeStubConfig(t)
	socketPath := filepath.Join(t.TempDir(), testAggSock)
	agg, err := NewAggregator(Config{
		ClaudeMCPConfigPath: cfgPath,
		SocketPath:          socketPath,
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
	httpClient := connectToUpstream(t, ctx, socketPath)

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
		ClaudeMCPConfigPath: cfgPath,
		SocketPath:          socketPath,
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
		ClaudeMCPConfigPath: cfgPath,
		SocketPath:          filepath.Join(t.TempDir(), testAggSock),
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
		ClaudeMCPConfigPath: path,
		SocketPath:          filepath.Join(t.TempDir(), testAggSock),
	})
	require.NoError(t, err)
	err = agg.Start(context.Background())
	assert.ErrorContains(t, err, "no upstreams")
}

func TestNewAggregator_RequiresSocketPath(t *testing.T) {
	_, err := NewAggregator(Config{ClaudeMCPConfigPath: writeStubConfig(t)})
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

func TestAggregator_RelaysResources(t *testing.T) {
	cfgPath := writeStubConfig(t)
	socketPath := filepath.Join(t.TempDir(), testAggSock)
	agg, err := NewAggregator(Config{
		ClaudeMCPConfigPath: cfgPath,
		SocketPath:          socketPath,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, agg.Start(ctx))
	defer func() { _ = agg.Shutdown(context.Background()) }()

	httpClient := connectToUpstream(t, ctx, socketPath)

	listed, err := httpClient.ListResources(ctx, mcp.ListResourcesRequest{})
	require.NoError(t, err)
	require.Len(t, listed.Resources, 1)
	assert.Equal(t, "greet", listed.Resources[0].Name)
	assert.Equal(t, stubGreetURI, listed.Resources[0].URI)

	readReq := mcp.ReadResourceRequest{}
	readReq.Params.URI = stubGreetURI
	readResult, err := httpClient.ReadResource(ctx, readReq)
	require.NoError(t, err)
	require.Len(t, readResult.Contents, 1)
	textContent, ok := readResult.Contents[0].(mcp.TextResourceContents)
	require.True(t, ok)
	assert.Equal(t, "hello-resource", textContent.Text)
}

func TestAggregator_RelaysResourceTemplates(t *testing.T) {
	cfgPath := writeStubConfig(t)
	socketPath := filepath.Join(t.TempDir(), testAggSock)
	agg, err := NewAggregator(Config{
		ClaudeMCPConfigPath: cfgPath,
		SocketPath:          socketPath,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, agg.Start(ctx))
	defer func() { _ = agg.Shutdown(context.Background()) }()

	httpClient := connectToUpstream(t, ctx, socketPath)

	listed, err := httpClient.ListResourceTemplates(ctx, mcp.ListResourceTemplatesRequest{})
	require.NoError(t, err)
	require.Len(t, listed.ResourceTemplates, 1)
	tmpl := listed.ResourceTemplates[0]
	assert.Equal(t, "greet-template", tmpl.Name)
	require.NotNil(t, tmpl.URITemplate)
	assert.Equal(t, "mem://greet/{name}", tmpl.URITemplate.Raw())
}

func TestAggregator_RelaysPrompts(t *testing.T) {
	cfgPath := writeStubConfig(t)
	socketPath := filepath.Join(t.TempDir(), testAggSock)
	agg, err := NewAggregator(Config{
		ClaudeMCPConfigPath: cfgPath,
		SocketPath:          socketPath,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, agg.Start(ctx))
	defer func() { _ = agg.Shutdown(context.Background()) }()

	httpClient := connectToUpstream(t, ctx, socketPath)

	listedPrompts, err := httpClient.ListPrompts(ctx, mcp.ListPromptsRequest{})
	require.NoError(t, err)
	require.Len(t, listedPrompts.Prompts, 1)
	assert.Equal(t, stubGreetPromptName, listedPrompts.Prompts[0].Name)

	getReq := mcp.GetPromptRequest{}
	getReq.Params.Name = stubGreetPromptName
	getReq.Params.Arguments = map[string]string{stubPromptArgSubject: "world"}
	result, err := httpClient.GetPrompt(ctx, getReq)
	require.NoError(t, err)
	require.Len(t, result.Messages, 1)
	msg := result.Messages[0]
	assert.Equal(t, mcp.RoleUser, msg.Role)
	textContent, ok := msg.Content.(mcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, "hello world", textContent.Text)
}

func TestAggregator_RelaysCompletion(t *testing.T) {
	cfgPath := writeStubConfig(t)
	socketPath := filepath.Join(t.TempDir(), testAggSock)
	agg, err := NewAggregator(Config{
		ClaudeMCPConfigPath: cfgPath,
		SocketPath:          socketPath,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, agg.Start(ctx))
	defer func() { _ = agg.Shutdown(context.Background()) }()

	httpClient := connectToUpstream(t, ctx, socketPath)

	completeReq := mcp.CompleteRequest{}
	completeReq.Params.Ref = mcp.PromptReference{Type: "ref/prompt", Name: stubGreetPromptName}
	completeReq.Params.Argument = mcp.CompleteArgument{Name: stubPromptArgSubject, Value: "a"}
	result, err := httpClient.Complete(ctx, completeReq)
	require.NoError(t, err)
	assert.Contains(t, result.Completion.Values, stubAlpha)
	assert.Contains(t, result.Completion.Values, stubBeta)
}

// TestAggregator_CatalogueRemainsToolsOnly verifies that resources and
// prompts relayed from an upstream do not bleed into the tool catalogue
// — it must contain only allowlisted tools.
func TestAggregator_CatalogueRemainsToolsOnly(t *testing.T) {
	cfgPath := writeStubConfig(t)
	socketPath := filepath.Join(t.TempDir(), testAggSock)
	agg, err := NewAggregator(Config{
		ClaudeMCPConfigPath: cfgPath,
		SocketPath:          socketPath,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, agg.Start(ctx))
	defer func() { _ = agg.Shutdown(context.Background()) }()

	cat := agg.Catalogue()
	require.Contains(t, cat, serverWorkflowy)
	require.Len(t, cat[serverWorkflowy], 1)
	assert.Equal(t, toolSearchNodes, cat[serverWorkflowy][0].Name)
}

// TestAggregator_ResourcesOnlyUpstreamMounted verifies that a server
// with no allowlisted tools (but with advertised resources) is still
// mounted and reachable, with an empty tools catalogue entry.
//
// The allowlist override uses ["nonexistent_tool"] so NewAggregator
// keeps the spec in the pool (len(Tools)=1 passes the empty-set
// guard), but filterTools produces an empty slice at Start time
// because the stub has no tool named "nonexistent_tool".
func TestAggregator_ResourcesOnlyUpstreamMounted(t *testing.T) {
	cfgPath := writeStubConfig(t)
	socketPath := filepath.Join(t.TempDir(), testAggSock)

	allowPath := filepath.Join(t.TempDir(), "allowlist.json")
	require.NoError(t, os.WriteFile(allowPath, []byte(`{"workflowy":["nonexistent_tool"]}`), 0o600))

	agg, err := NewAggregator(Config{
		ClaudeMCPConfigPath: cfgPath,
		SocketPath:          socketPath,
		AllowlistFilePath:   allowPath,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, agg.Start(ctx))
	defer func() { _ = agg.Shutdown(context.Background()) }()

	require.Contains(t, agg.Servers(), serverWorkflowy)
	cat := agg.Catalogue()
	require.Contains(t, cat, serverWorkflowy)
	assert.Empty(t, cat[serverWorkflowy]) // no allowlisted tools, but server is mounted

	httpClient := connectToUpstream(t, ctx, socketPath)

	listed, err := httpClient.ListResources(ctx, mcp.ListResourcesRequest{})
	require.NoError(t, err)
	require.Len(t, listed.Resources, 1)
	assert.Equal(t, "greet", listed.Resources[0].Name)
}
