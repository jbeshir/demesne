package mcpproxy

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	uritemplate "github.com/yosida95/uritemplate/v3"

	proxymcp "github.com/jbeshir/demesne/internal/proxies/mcp"
)

const (
	stubAlpha            = "alpha"
	stubBeta             = "beta"
	stubGreetURI         = "mem://greet"
	stubGreetPromptName  = "greet-prompt"
	stubPromptArgSubject = "subject"
	testMCPServersKey    = "mcpServers"
	testTypeKey          = "type"
	testTransportStdio   = "stdio"
	testCommandKey       = "command"
	testEnvKey           = "env"
	testClientName       = "test"
)

// TestMain lets this test binary double as a stub stdio MCP server
// when DEMESNE_TEST_STUB_MCP=1, so aggregator tests can spawn a
// real upstream subprocess without a separate binary.
func TestMain(m *testing.M) {
	switch {
	case os.Getenv("DEMESNE_TEST_STUB_MCP") == "1":
		runStubServer()
		return
	case os.Getenv("DEMESNE_TEST_STUB_IMAGEGEN") == "1":
		runImagegenStub()
		return
	case os.Getenv("DEMESNE_TEST_STUB_ASSETS") == "1":
		runAssetsStub()
		return
	}
	os.Exit(m.Run())
}

// runAssetsStub serves every assets file-producing tool. The requested shape
// lets one compact handler test exercise both forms emitted by manifest tools.
func runAssetsStub() {
	srv := server.NewMCPServer("stub-assets", "0")
	for _, name := range (assetsAdapter{}).Tools() {
		toolName := name
		srv.AddTool(mcp.Tool{Name: toolName}, func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			hostPath := "/host/assets/" + toolName + ".asset"
			manifest := map[string]any{
				filesKey: []any{map[string]any{pathKey: hostPath}},
				countKey: 1,
			}
			if req.GetString("shape", "") == textType {
				data, _ := json.Marshal(manifest)
				return mcp.NewToolResultText(string(data)), nil
			}
			return &mcp.CallToolResult{StructuredContent: manifest}, nil
		})
	}
	_ = server.ServeStdio(srv)
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

// runImagegenStub serves a generate_image tool that returns a
// structured result with a file:// image URL. Used as a real upstream
// subprocess in file-gen handler tests.
func runImagegenStub() {
	srv := server.NewMCPServer("stub-imagegen", "0")
	srv.AddTool(mcp.Tool{
		Name:        toolGenerateImg,
		Description: "generate image stub",
	}, func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content:           []mcp.Content{mcp.TextContent{Type: textType, Text: `{"image_url":"file:///tmp/fake/img.png"}`}},
			StructuredContent: map[string]any{imageURLKey: imageGenStubURL},
		}, nil
	})
	_ = server.ServeStdio(srv)
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
					Content: mcp.TextContent{Type: textType, Text: "hello " + subject},
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
	imageGenStubURL   = "file:///tmp/fake/img.png"
	fakeHostDir       = "/host/delivery"
	fakeSandboxDir    = "/workspace/generated"
)

// writeStubConfig writes a Claude-Code-shaped MCP config that points
// the "workflowy" server at this test binary running as the stub.
func writeStubConfig(t *testing.T) string {
	t.Helper()
	self, err := os.Executable()
	require.NoError(t, err)
	cfg := map[string]any{
		testMCPServersKey: map[string]any{
			serverWorkflowy: map[string]any{
				testTypeKey:    testTransportStdio,
				testCommandKey: self,
				testEnvKey:     map[string]string{"DEMESNE_TEST_STUB_MCP": "1"},
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
	initReq.Params.ClientInfo = mcp.Implementation{Name: testClientName, Version: "0"}
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

// writeImagegenStubConfig writes a Claude-Code-shaped MCP config pointing
// "image-gen-mcp" at the test binary running as the imagegen stub, and
// an allowlist file granting generate_image. Returns (cfgPath, allowPath).
func writeImagegenStubConfig(t *testing.T) (string, string) {
	t.Helper()
	self, err := os.Executable()
	require.NoError(t, err)
	cfg := map[string]any{
		testMCPServersKey: map[string]any{
			serverImageGen: map[string]any{
				testTypeKey:    testTransportStdio,
				testCommandKey: self,
				testEnvKey:     map[string]string{"DEMESNE_TEST_STUB_IMAGEGEN": "1"},
			},
		},
	}
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	cfgPath := filepath.Join(t.TempDir(), testClaudeJSON)
	require.NoError(t, os.WriteFile(cfgPath, data, 0o600))

	allowPath := filepath.Join(t.TempDir(), "allowlist.json")
	require.NoError(t, os.WriteFile(allowPath, []byte(`{"image-gen-mcp":["generate_image"]}`), 0o600))

	return cfgPath, allowPath
}

// connectToImagegenUpstream creates, starts, and initializes an MCP HTTP
// client connected to /image-gen-mcp/mcp via the aggregator socket using
// the provided http.Client.
func connectToImagegenUpstream(t *testing.T, ctx context.Context, httpClient *http.Client) *client.Client {
	t.Helper()
	mcpClient, err := client.NewStreamableHttpClient(
		"http://demesne-mcp/image-gen-mcp/mcp",
		transport.WithHTTPBasicClient(httpClient),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mcpClient.Close() })
	require.NoError(t, mcpClient.Start(ctx))
	initReq := mcp.InitializeRequest{}
	initReq.Params.ClientInfo = mcp.Implementation{Name: testClientName, Version: "0"}
	_, err = mcpClient.Initialize(ctx, initReq)
	require.NoError(t, err)
	return mcpClient
}

// headerSettingTransport wraps an http.RoundTripper and injects fixed
// headers on every request, used to supply the parent-job header in
// file-gen tests without per-request client API.
type headerSettingTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerSettingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	return t.base.RoundTrip(req)
}

type fakeDeliverer struct {
	hostDir          string
	sbDir            string
	err              error
	deliverErr       error
	mapping          map[string]string
	deliveryDirCalls atomic.Int32
	deliverCalls     atomic.Int32
	lastJobID        string
	deliveredPaths   [][]string
}

func (f *fakeDeliverer) DeliveryDir(parent string) (string, string, error) {
	f.deliveryDirCalls.Add(1)
	f.lastJobID = parent
	return f.hostDir, f.sbDir, f.err
}

func (f *fakeDeliverer) Deliver(_ string, paths []string) (map[string]string, error) {
	f.deliverCalls.Add(1)
	f.deliveredPaths = append(f.deliveredPaths, append([]string(nil), paths...))
	return f.mapping, f.deliverErr
}

func TestAggregator_AssetsFileProducersDeliverAndRewrite(t *testing.T) {
	self, err := os.Executable()
	require.NoError(t, err)
	tools := (assetsAdapter{}).Tools()
	cfg := map[string]any{testMCPServersKey: map[string]any{
		serverAssets: map[string]any{
			testTypeKey: testTransportStdio, testCommandKey: self,
			testEnvKey: map[string]string{"DEMESNE_TEST_STUB_ASSETS": "1"},
		},
	}}
	cfgData, err := json.Marshal(cfg)
	require.NoError(t, err)
	cfgPath := filepath.Join(t.TempDir(), testClaudeJSON)
	require.NoError(t, os.WriteFile(cfgPath, cfgData, 0o600))
	allowData, err := json.Marshal(map[string][]string{serverAssets: tools})
	require.NoError(t, err)
	allowPath := filepath.Join(t.TempDir(), "allowlist.json")
	require.NoError(t, os.WriteFile(allowPath, allowData, 0o600))

	mapping := make(map[string]string, len(tools))
	for _, tool := range tools {
		mapping["/host/assets/"+tool+".asset"] = fakeSandboxDir + "/" + tool + ".asset"
	}
	fd := &fakeDeliverer{hostDir: fakeHostDir, sbDir: fakeSandboxDir, mapping: mapping}
	socketPath := filepath.Join(t.TempDir(), testAggSock)
	agg, err := NewAggregator(Config{
		ClaudeMCPConfigPath: cfgPath, AllowlistFilePath: allowPath,
		SocketPath: socketPath, FileDeliverer: fd,
	})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, agg.Start(ctx))
	defer func() { _ = agg.Shutdown(context.Background()) }()

	httpClient := &http.Client{Transport: &headerSettingTransport{
		base:    dialUnix(socketPath).Transport,
		headers: map[string]string{proxymcp.ParentHeader: "assets-parent"},
	}}
	mcpClient, err := client.NewStreamableHttpClient(
		"http://demesne-mcp/assets/mcp", transport.WithHTTPBasicClient(httpClient),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mcpClient.Close() })
	require.NoError(t, mcpClient.Start(ctx))
	initReq := mcp.InitializeRequest{}
	initReq.Params.ClientInfo = mcp.Implementation{Name: testClientName, Version: "0"}
	_, err = mcpClient.Initialize(ctx, initReq)
	require.NoError(t, err)

	for _, tool := range tools {
		for _, shape := range []string{"structured", textType} {
			t.Run(tool+"/"+shape, func(t *testing.T) {
				hostPath := "/host/assets/" + tool + ".asset"
				sandboxPath := fakeSandboxDir + "/" + tool + ".asset"
				before := len(fd.deliveredPaths)
				req := mcp.CallToolRequest{}
				req.Params.Name = tool
				req.Params.Arguments = map[string]any{"shape": shape}
				result, callErr := mcpClient.CallTool(ctx, req)
				require.NoError(t, callErr)
				require.False(t, result.IsError)
				require.Len(t, fd.deliveredPaths, before+1)
				assert.Equal(t, []string{hostPath}, fd.deliveredPaths[before])

				if shape == "structured" {
					manifest, ok := result.StructuredContent.(map[string]any)
					require.True(t, ok)
					files, ok := manifest[filesKey].([]any)
					require.True(t, ok)
					entry, ok := files[0].(map[string]any)
					require.True(t, ok)
					assert.Equal(t, sandboxPath, entry[pathKey])
					assert.NotEqual(t, hostPath, entry[pathKey])
					return
				}

				require.Len(t, result.Content, 1)
				textResult, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok)
				assert.Contains(t, textResult.Text, sandboxPath)
				assert.NotContains(t, textResult.Text, hostPath)
			})
		}
	}
	const expectedDeliverCalls int32 = 18
	assert.Equal(t, expectedDeliverCalls, fd.deliverCalls.Load())
}

func TestAggregator_FileGenPassthrough_NoParent(t *testing.T) {
	cfgPath, allowPath := writeImagegenStubConfig(t)
	socketPath := filepath.Join(t.TempDir(), testAggSock)

	fd := &fakeDeliverer{
		hostDir: fakeHostDir,
		sbDir:   fakeSandboxDir,
		mapping: map[string]string{"/tmp/fake/img.png": "/workspace/generated/img.png"},
	}

	agg, err := NewAggregator(Config{
		ClaudeMCPConfigPath: cfgPath,
		AllowlistFilePath:   allowPath,
		SocketPath:          socketPath,
		FileDeliverer:       fd,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, agg.Start(ctx))
	defer func() { _ = agg.Shutdown(context.Background()) }()

	mcpClient := connectToImagegenUpstream(t, ctx, dialUnix(socketPath))

	callReq := mcp.CallToolRequest{}
	callReq.Params.Name = toolGenerateImg
	callReq.Params.Arguments = map[string]any{}
	res, err := mcpClient.CallTool(ctx, callReq)
	require.NoError(t, err)
	require.False(t, res.IsError)

	sc, ok := res.StructuredContent.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, imageGenStubURL, sc[imageURLKey])

	assert.Equal(t, int32(0), fd.deliveryDirCalls.Load())
	assert.Equal(t, int32(0), fd.deliverCalls.Load())
}

func TestAggregator_FileGenRewrites_WithParent(t *testing.T) {
	cfgPath, allowPath := writeImagegenStubConfig(t)
	socketPath := filepath.Join(t.TempDir(), testAggSock)

	fd := &fakeDeliverer{
		hostDir: fakeHostDir,
		sbDir:   fakeSandboxDir,
		mapping: map[string]string{"/tmp/fake/img.png": "/workspace/generated/img.png"},
	}

	agg, err := NewAggregator(Config{
		ClaudeMCPConfigPath: cfgPath,
		AllowlistFilePath:   allowPath,
		SocketPath:          socketPath,
		FileDeliverer:       fd,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, agg.Start(ctx))
	defer func() { _ = agg.Shutdown(context.Background()) }()

	httpClient := &http.Client{
		Transport: &headerSettingTransport{
			base: dialUnix(socketPath).Transport,
			headers: map[string]string{
				proxymcp.ParentHeader: "test-parent-job-123",
			},
		},
	}
	mcpClient := connectToImagegenUpstream(t, ctx, httpClient)

	callReq := mcp.CallToolRequest{}
	callReq.Params.Name = toolGenerateImg
	callReq.Params.Arguments = map[string]any{}
	res, err := mcpClient.CallTool(ctx, callReq)
	require.NoError(t, err)
	require.False(t, res.IsError)

	sc, ok := res.StructuredContent.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "/workspace/generated/img.png", sc[imageURLKey])

	require.NotEmpty(t, res.Content)
	textContent, ok := res.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "/workspace/generated/img.png")
	assert.NotContains(t, textContent.Text, "file://")

	assert.Equal(t, int32(1), fd.deliveryDirCalls.Load())
	assert.Equal(t, int32(1), fd.deliverCalls.Load())
}

func TestAggregator_FileGenDeliveryDirError_ReturnsToolError(t *testing.T) {
	cfgPath, allowPath := writeImagegenStubConfig(t)
	socketPath := filepath.Join(t.TempDir(), testAggSock)

	fd := &fakeDeliverer{
		err: errors.New("delivery dir failed"),
	}

	agg, err := NewAggregator(Config{
		ClaudeMCPConfigPath: cfgPath,
		AllowlistFilePath:   allowPath,
		SocketPath:          socketPath,
		FileDeliverer:       fd,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, agg.Start(ctx))
	defer func() { _ = agg.Shutdown(context.Background()) }()

	httpClient := &http.Client{
		Transport: &headerSettingTransport{
			base: dialUnix(socketPath).Transport,
			headers: map[string]string{
				proxymcp.ParentHeader: "test-parent-job-456",
			},
		},
	}
	mcpClient := connectToImagegenUpstream(t, ctx, httpClient)

	callReq := mcp.CallToolRequest{}
	callReq.Params.Name = toolGenerateImg
	callReq.Params.Arguments = map[string]any{}
	res, err := mcpClient.CallTool(ctx, callReq)
	require.NoError(t, err)
	require.True(t, res.IsError)

	require.NotEmpty(t, res.Content)
	textContent, ok := res.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "file-gen")
}

func TestAggregator_FileGenDeliverError_ReturnsToolError(t *testing.T) {
	cfgPath, allowPath := writeImagegenStubConfig(t)
	socketPath := filepath.Join(t.TempDir(), testAggSock)

	fd := &fakeDeliverer{
		hostDir:    fakeHostDir,
		sbDir:      fakeSandboxDir,
		deliverErr: errors.New("copy failed"),
	}

	agg, err := NewAggregator(Config{
		ClaudeMCPConfigPath: cfgPath,
		AllowlistFilePath:   allowPath,
		SocketPath:          socketPath,
		FileDeliverer:       fd,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, agg.Start(ctx))
	defer func() { _ = agg.Shutdown(context.Background()) }()

	httpClient := &http.Client{
		Transport: &headerSettingTransport{
			base: dialUnix(socketPath).Transport,
			headers: map[string]string{
				proxymcp.ParentHeader: "test-parent-job-789",
			},
		},
	}
	mcpClient := connectToImagegenUpstream(t, ctx, httpClient)

	callReq := mcp.CallToolRequest{}
	callReq.Params.Name = toolGenerateImg
	callReq.Params.Arguments = map[string]any{}
	res, err := mcpClient.CallTool(ctx, callReq)
	require.NoError(t, err)
	require.True(t, res.IsError)

	require.NotEmpty(t, res.Content)
	textContent, ok := res.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "deliver")
	assert.Equal(t, int32(1), fd.deliveryDirCalls.Load())
	assert.Equal(t, int32(1), fd.deliverCalls.Load())
}
