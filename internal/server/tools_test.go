package server

import (
	"context"
	"errors"
	"testing"

	"github.com/jbeshir/demesne/internal/sandbox"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSandboxID    = "sbx-1"
	testError        = "boom"
	msgUnexpectedErr = "unexpected error result: %s"
	msgIsError       = "case %d: expected IsError=true for %v"
	msgNoCall        = "case %d: runner should not be called"
	testCmdEcho      = "echo hello"
	testCmdTrue      = "true"
	testEgressOpen   = "open"
	testStdoutHello  = "hello\n"
	testFile         = "/some/file.txt"
	testDir          = "/some/dir"
	msgExitCodeZero  = "exit_code: 0"
	doneStdout       = "DONE\n"
	// Model alias literals used across multiple test cases; named here
	// so the goconst linter doesn't repeat-flag them as they spread.
	testModelHaiku  = "haiku"
	testModelSonnet = "sonnet"
)

type fakeRunner struct {
	scriptCalls    int
	gotScriptReq   sandbox.ScriptRequest
	scriptRes      sandbox.ScriptResult
	scriptErr      error
	createCalls    int
	gotCreateReq   sandbox.CreateRequest
	createRes      sandbox.CreateResult
	createErr      error
	execCalls      int
	gotExecReq     sandbox.ExecRequest
	execRes        sandbox.ExecResult
	execErr        error
	uploadCalls    int
	gotUploadReq   sandbox.UploadRequest
	uploadErr      error
	downloadCalls  int
	gotDownloadReq sandbox.DownloadRequest
	downloadRes    sandbox.DownloadResult
	downloadErr    error
	destroyCalls   int
	gotDestroyReq  sandbox.DestroyRequest
	destroyErr     error
	agentCalls     int
	gotAgentReq    sandbox.AgentRequest
	agentRes       sandbox.AgentResult
	agentErr       error
	researchCalls  int
	gotResearchReq sandbox.ResearchRequest
	researchRes    sandbox.AgentResult
	researchErr    error
	available      []sandbox.AgentOption
	allowedPaths   []string
}

func (f *fakeRunner) RunScript(_ context.Context, req sandbox.ScriptRequest) (sandbox.ScriptResult, error) {
	f.scriptCalls++
	f.gotScriptReq = req
	return f.scriptRes, f.scriptErr
}

func (f *fakeRunner) Create(_ context.Context, req sandbox.CreateRequest) (sandbox.CreateResult, error) {
	f.createCalls++
	f.gotCreateReq = req
	return f.createRes, f.createErr
}

func (f *fakeRunner) Exec(_ context.Context, req sandbox.ExecRequest) (sandbox.ExecResult, error) {
	f.execCalls++
	f.gotExecReq = req
	return f.execRes, f.execErr
}

func (f *fakeRunner) Upload(_ context.Context, req sandbox.UploadRequest) error {
	f.uploadCalls++
	f.gotUploadReq = req
	return f.uploadErr
}

func (f *fakeRunner) Download(_ context.Context, req sandbox.DownloadRequest) (sandbox.DownloadResult, error) {
	f.downloadCalls++
	f.gotDownloadReq = req
	return f.downloadRes, f.downloadErr
}

func (f *fakeRunner) Destroy(_ context.Context, req sandbox.DestroyRequest) error {
	f.destroyCalls++
	f.gotDestroyReq = req
	return f.destroyErr
}

func (f *fakeRunner) Agent(_ context.Context, req sandbox.AgentRequest) (sandbox.AgentResult, error) {
	f.agentCalls++
	f.gotAgentReq = req
	return f.agentRes, f.agentErr
}

func (f *fakeRunner) Research(_ context.Context, req sandbox.ResearchRequest) (sandbox.AgentResult, error) {
	f.researchCalls++
	f.gotResearchReq = req
	return f.researchRes, f.researchErr
}

func (f *fakeRunner) AvailableAgents() []sandbox.AgentOption { return f.available }

func (f *fakeRunner) AllowedMountPaths() []string { return f.allowedPaths }

func newRequest(args map[string]any) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	req.Params.Name = "sandbox_script"
	req.Params.Arguments = args
	return req
}

func resultText(t *testing.T, r *mcp.CallToolResult) string {
	t.Helper()
	require.NotNil(t, r)
	require.NotEmpty(t, r.Content)
	tc, ok := r.Content[0].(mcp.TextContent)
	require.True(t, ok, "content is %T, want TextContent", r.Content[0])
	return tc.Text
}

// resultStructured returns the result's structuredContent typed as T, failing
// if it is absent or of another type.
func resultStructured[T any](t *testing.T, r *mcp.CallToolResult) T {
	t.Helper()
	require.NotNil(t, r)
	v, ok := r.StructuredContent.(T)
	require.True(t, ok, "structuredContent is %T, want %T", r.StructuredContent, *new(T))
	return v
}

func TestHandleSandboxScript_MissingCommand(t *testing.T) {
	r := &fakeRunner{}
	s := NewServer(r)
	got, err := s.handleSandboxScript(context.Background(), newRequest(map[string]any{}))
	require.NoError(t, err)
	assert.True(t, got.IsError, "expected IsError=true for missing command")
	assert.Zero(t, r.scriptCalls, "runner should not be called")
}

func TestHandleSandboxScript_InvalidFiles(t *testing.T) {
	r := &fakeRunner{}
	s := NewServer(r)
	got, err := s.handleSandboxScript(context.Background(), newRequest(map[string]any{
		paramCommand: testCmdTrue,
		paramFiles:   "not-an-array",
	}))
	require.NoError(t, err)
	assert.True(t, got.IsError, "expected IsError=true for non-array files")
	assert.Zero(t, r.scriptCalls, "runner should not be called when args invalid")
}

func TestHandleSandboxScript_RunnerErrorSurfaced(t *testing.T) {
	r := &fakeRunner{scriptErr: errors.New(testError)}
	s := NewServer(r)
	got, err := s.handleSandboxScript(context.Background(), newRequest(map[string]any{
		paramCommand: testCmdTrue,
	}))
	require.NoError(t, err)
	assert.True(t, got.IsError, "expected IsError=true when runner fails")
	assert.Contains(t, resultText(t, got), testError)
}

func TestHandleSandboxScript_HappyPath(t *testing.T) {
	r := &fakeRunner{
		scriptRes: sandbox.ScriptResult{
			JobID:      sandbox.JobID("abc-123"),
			OutputPath: "/tmp/demesne/out/abc-123",
			Stdout:     testStdoutHello,
			ExitCode:   0,
		},
	}
	s := NewServer(r)
	got, err := s.handleSandboxScript(context.Background(), newRequest(map[string]any{
		paramCommand:     testCmdEcho,
		paramImage:       "anaconda",
		paramEgress:      "none",
		paramFiles:       []any{testFile},
		paramDirectories: []any{testDir},
	}))
	require.NoError(t, err)
	require.False(t, got.IsError, msgUnexpectedErr, resultText(t, got))
	assert.Equal(t, 1, r.scriptCalls)
	assert.Equal(t, sandbox.ScriptRequest{
		Command:     testCmdEcho,
		Image:       "anaconda",
		Egress:      sandbox.EgressNone,
		Files:       []string{testFile},
		Directories: []string{testDir},
	}, r.gotScriptReq)

	text := resultText(t, got)
	for _, want := range []string{msgExitCodeZero, "output_dir: /tmp/demesne/out/abc-123", "job_id: abc-123", "hello"} {
		assert.Contains(t, text, want)
	}
	assert.Equal(t, scriptOutput{
		ExitCode:  0,
		OutputDir: "/tmp/demesne/out/abc-123",
		JobID:     "abc-123",
		Stdout:    testStdoutHello,
	}, resultStructured[scriptOutput](t, got))
}

func TestHandleSandboxScript_DefaultEgress(t *testing.T) {
	r := &fakeRunner{}
	s := NewServer(r)
	_, err := s.handleSandboxScript(context.Background(), newRequest(map[string]any{
		paramCommand: testCmdTrue,
	}))
	require.NoError(t, err)
	assert.Equal(t, sandbox.EgressPackageManagers, r.gotScriptReq.Egress)
}

func TestHandleSandboxCreate_HappyPath(t *testing.T) {
	r := &fakeRunner{
		createRes: sandbox.CreateResult{
			SandboxID:  sandbox.SandboxID("sbx-1"),
			OutputPath: "/tmp/demesne/out/job-1",
		},
	}
	s := NewServer(r)
	got, err := s.handleSandboxCreate(context.Background(), newRequest(map[string]any{
		paramImage:  "python",
		paramEgress: "none",
	}))
	require.NoError(t, err)
	require.False(t, got.IsError, msgUnexpectedErr, resultText(t, got))
	assert.Equal(t, 1, r.createCalls)
	assert.Equal(t, sandbox.CreateRequest{Image: "python", Egress: sandbox.EgressNone}, r.gotCreateReq)
	text := resultText(t, got)
	for _, want := range []string{"sandbox_id: sbx-1", "output_dir: /tmp/demesne/out/job-1"} {
		assert.Contains(t, text, want)
	}
	assert.Equal(t, createOutput{
		SandboxID: "sbx-1",
		OutputDir: "/tmp/demesne/out/job-1",
	}, resultStructured[createOutput](t, got))
}

func TestHandleSandboxCreate_DefaultEgress(t *testing.T) {
	r := &fakeRunner{}
	s := NewServer(r)
	_, err := s.handleSandboxCreate(context.Background(), newRequest(map[string]any{}))
	require.NoError(t, err)
	assert.Equal(t, sandbox.EgressPackageManagers, r.gotCreateReq.Egress)
}

func TestHandleSandboxExec_MissingParams(t *testing.T) {
	cases := []map[string]any{
		{},
		{paramSandboxID: testSandboxID},
		{paramCommand: "echo hi"},
		{paramSandboxID: "", paramCommand: "echo hi"},
	}
	for i, args := range cases {
		r := &fakeRunner{}
		s := NewServer(r)
		got, err := s.handleSandboxExec(context.Background(), newRequest(args))
		require.NoError(t, err)
		assert.True(t, got.IsError, msgIsError, i, args)
		assert.Zero(t, r.execCalls, msgNoCall, i)
	}
}

func TestHandleSandboxExec_HappyPath(t *testing.T) {
	r := &fakeRunner{
		execRes: sandbox.ExecResult{Stdout: testStdoutHello, ExitCode: 0},
	}
	s := NewServer(r)
	got, err := s.handleSandboxExec(context.Background(), newRequest(map[string]any{
		paramSandboxID: testSandboxID,
		paramCommand:   testCmdEcho,
	}))
	require.NoError(t, err)
	require.False(t, got.IsError, msgUnexpectedErr, resultText(t, got))
	assert.Equal(t, sandbox.ExecRequest{SandboxID: sandbox.SandboxID(testSandboxID), Command: testCmdEcho}, r.gotExecReq)
	text := resultText(t, got)
	for _, want := range []string{msgExitCodeZero, "hello"} {
		assert.Contains(t, text, want)
	}
	assert.Equal(t, execOutput{ExitCode: 0, Stdout: testStdoutHello}, resultStructured[execOutput](t, got))
}

func TestHandleSandboxExec_RunnerErrorSurfaced(t *testing.T) {
	r := &fakeRunner{execErr: errors.New(testError)}
	s := NewServer(r)
	got, err := s.handleSandboxExec(context.Background(), newRequest(map[string]any{
		paramSandboxID: testSandboxID,
		paramCommand:   testCmdTrue,
	}))
	require.NoError(t, err)
	assert.True(t, got.IsError, "expected IsError=true when runner fails")
	assert.Contains(t, resultText(t, got), testError)
}

func TestHandleSandboxUpload_MissingParams(t *testing.T) {
	cases := []map[string]any{
		{},
		{paramSandboxID: testSandboxID},
		{paramSandboxID: testSandboxID, paramSrc: "/a"},
		{paramSrc: "/a", paramDst: "/b"},
		{paramSandboxID: testSandboxID, paramSrc: "", paramDst: "/b"},
	}
	for i, args := range cases {
		r := &fakeRunner{}
		s := NewServer(r)
		got, err := s.handleSandboxUpload(context.Background(), newRequest(args))
		require.NoError(t, err)
		assert.True(t, got.IsError, msgIsError, i, args)
		assert.Zero(t, r.uploadCalls, msgNoCall, i)
	}
}

func TestHandleSandboxUpload_HappyPath(t *testing.T) {
	r := &fakeRunner{}
	s := NewServer(r)
	got, err := s.handleSandboxUpload(context.Background(), newRequest(map[string]any{
		paramSandboxID: testSandboxID,
		paramSrc:       "/host/data.txt",
		paramDst:       "/tmp/data.txt",
	}))
	require.NoError(t, err)
	require.False(t, got.IsError, msgUnexpectedErr, resultText(t, got))
	assert.Equal(t, sandbox.UploadRequest{SandboxID: sandbox.SandboxID(testSandboxID), HostSrc: "/host/data.txt", SandboxDst: "/tmp/data.txt"}, r.gotUploadReq)
	assert.Contains(t, resultText(t, got), "uploaded: data.txt -> /tmp/data.txt")
}

func TestHandleSandboxDownload_MissingParams(t *testing.T) {
	cases := []map[string]any{
		{},
		{paramSandboxID: testSandboxID},
		{paramSrc: "/in/a"},
		{paramSandboxID: "", paramSrc: "/in/a"},
	}
	for i, args := range cases {
		r := &fakeRunner{}
		s := NewServer(r)
		got, err := s.handleSandboxDownload(context.Background(), newRequest(args))
		require.NoError(t, err)
		assert.True(t, got.IsError, msgIsError, i, args)
		assert.Zero(t, r.downloadCalls, msgNoCall, i)
	}
}

func TestHandleSandboxDownload_HappyPath(t *testing.T) {
	r := &fakeRunner{
		downloadRes: sandbox.DownloadResult{HostPath: "/host/out/job-1/downloads/a.txt"},
	}
	s := NewServer(r)
	got, err := s.handleSandboxDownload(context.Background(), newRequest(map[string]any{
		paramSandboxID: testSandboxID,
		paramSrc:       "/sandbox/a.txt",
	}))
	require.NoError(t, err)
	require.False(t, got.IsError, msgUnexpectedErr, resultText(t, got))
	assert.Equal(t, sandbox.DownloadRequest{SandboxID: sandbox.SandboxID(testSandboxID), SandboxSrc: "/sandbox/a.txt"}, r.gotDownloadReq)
	assert.Contains(t, resultText(t, got), "downloaded: /sandbox/a.txt -> /host/out/job-1/downloads/a.txt")
}

func TestHandleSandboxDestroy_MissingParam(t *testing.T) {
	r := &fakeRunner{}
	s := NewServer(r)
	got, err := s.handleSandboxDestroy(context.Background(), newRequest(map[string]any{}))
	require.NoError(t, err)
	assert.True(t, got.IsError, "expected IsError=true for missing sandbox_id")
	assert.Zero(t, r.destroyCalls, "runner should not be called")
}

func TestHandleSandboxDestroy_HappyPath(t *testing.T) {
	r := &fakeRunner{}
	s := NewServer(r)
	got, err := s.handleSandboxDestroy(context.Background(), newRequest(map[string]any{
		paramSandboxID: testSandboxID,
	}))
	require.NoError(t, err)
	require.False(t, got.IsError, msgUnexpectedErr, resultText(t, got))
	assert.Equal(t, sandbox.SandboxID(testSandboxID), r.gotDestroyReq.SandboxID)
	assert.Contains(t, resultText(t, got), "destroyed: "+testSandboxID)
}

func TestHandleSandboxAgent_MissingPrompt(t *testing.T) {
	cases := []map[string]any{{}, {paramPrompt: ""}}
	for i, args := range cases {
		r := &fakeRunner{}
		s := NewServer(r)
		got, err := s.handleSandboxAgent(context.Background(), newRequest(args))
		require.NoError(t, err)
		assert.True(t, got.IsError, msgIsError, i, args)
		assert.Zero(t, r.agentCalls, msgNoCall, i)
	}
}

func TestHandleSandboxAgent_HappyPath(t *testing.T) {
	r := &fakeRunner{
		agentRes: sandbox.AgentResult{
			JobID:      sandbox.JobID("abc"),
			OutputPath: "/tmp/demesne-out/abc",
			Stdout:     "PONG\n",
			ExitCode:   0,
			CostUSD:    0.0123,
		},
	}
	s := NewServer(r)
	got, err := s.handleSandboxAgent(context.Background(), newRequest(map[string]any{
		paramPrompt:      "reply PONG",
		paramModel:       testModelHaiku,
		paramPreamble:    "say only the word",
		paramFiles:       []any{testFile},
		paramDirectories: []any{testDir},
		paramEgress:      "package-managers",
	}))
	require.NoError(t, err)
	require.False(t, got.IsError, msgUnexpectedErr, resultText(t, got))
	assert.Equal(t, sandbox.AgentRequest{
		Agent:       "",
		Model:       testModelHaiku,
		Prompt:      "reply PONG",
		Preamble:    "say only the word",
		Files:       []string{testFile},
		Directories: []string{testDir},
		Egress:      sandbox.EgressPackageManagers,
	}, r.gotAgentReq)
	text := resultText(t, got)
	for _, want := range []string{
		msgExitCodeZero, "output_dir: /tmp/demesne-out/abc", "job_id: abc",
		"cost_usd: 0.0123", "PONG",
	} {
		assert.Contains(t, text, want)
	}
	assert.Equal(t, agentRunOutput{
		ExitCode:  0,
		OutputDir: "/tmp/demesne-out/abc",
		JobID:     "abc",
		CostUSD:   0.0123,
		Stdout:    "PONG\n",
	}, resultStructured[agentRunOutput](t, got))
}

func TestHandleSandboxAgent_OutputContractWired(t *testing.T) {
	r := &fakeRunner{agentRes: sandbox.AgentResult{JobID: "x", OutputPath: "/tmp/x"}}
	s := NewServer(r)
	got, err := s.handleSandboxAgent(context.Background(), newRequest(map[string]any{
		paramPrompt:          "do something",
		paramOutputPath:      "/out/result.md",
		paramOutputFormat:    "Markdown report",
		paramSuccessCriteria: []any{"section A present", "no errors"},
	}))
	require.NoError(t, err)
	require.False(t, got.IsError, msgUnexpectedErr, resultText(t, got))
	assert.Equal(t, "/out/result.md", r.gotAgentReq.OutputPath)
	assert.Equal(t, "Markdown report", r.gotAgentReq.OutputFormat)
	assert.Equal(t, []string{"section A present", "no errors"}, r.gotAgentReq.SuccessCriteria)
}

func TestHandleSandboxResearch_OutputContractWired(t *testing.T) {
	r := &fakeRunner{researchRes: sandbox.AgentResult{JobID: "y", OutputPath: "/tmp/y"}}
	s := NewServer(r)
	got, err := s.handleSandboxResearch(context.Background(), newRequest(map[string]any{
		paramPrompt:          "investigate this",
		paramOutputPath:      "/out/report.md",
		paramOutputFormat:    "JSON: {result: string}",
		paramSuccessCriteria: []any{"valid JSON"},
	}))
	require.NoError(t, err)
	require.False(t, got.IsError, msgUnexpectedErr, resultText(t, got))
	assert.Equal(t, "/out/report.md", r.gotResearchReq.OutputPath)
	assert.Equal(t, "JSON: {result: string}", r.gotResearchReq.OutputFormat)
	assert.Equal(t, []string{"valid JSON"}, r.gotResearchReq.SuccessCriteria)
}

func TestHandleSandboxScript_RejectsOpenEgress(t *testing.T) {
	r := &fakeRunner{}
	s := NewServer(r)
	got, err := s.handleSandboxScript(context.Background(), newRequest(map[string]any{
		paramCommand: testCmdTrue,
		paramEgress:  testEgressOpen,
	}))
	require.NoError(t, err)
	assert.True(t, got.IsError, "open egress must be refused for sandbox_script")
	assert.Contains(t, resultText(t, got), "sandbox_research")
	assert.Zero(t, r.scriptCalls, "runner must not be called")
}

func TestHandleSandboxCreate_RejectsOpenEgress(t *testing.T) {
	r := &fakeRunner{}
	s := NewServer(r)
	got, err := s.handleSandboxCreate(context.Background(), newRequest(map[string]any{
		paramEgress: testEgressOpen,
	}))
	require.NoError(t, err)
	assert.True(t, got.IsError, "open egress must be refused for sandbox_create")
	assert.Contains(t, resultText(t, got), "sandbox_research")
	assert.Zero(t, r.createCalls, "runner must not be called")
}

func TestHandleSandboxAgent_RejectsOpenEgress(t *testing.T) {
	r := &fakeRunner{}
	s := NewServer(r)
	got, err := s.handleSandboxAgent(context.Background(), newRequest(map[string]any{
		paramPrompt: "hi",
		paramEgress: testEgressOpen,
	}))
	require.NoError(t, err)
	assert.True(t, got.IsError, "open egress must be refused for sandbox_agent")
	assert.Contains(t, resultText(t, got), "sandbox_research")
	assert.Zero(t, r.agentCalls, "runner must not be called")
}

func TestHandleSandboxAgent_DefaultEgress(t *testing.T) {
	r := &fakeRunner{}
	s := NewServer(r)
	_, err := s.handleSandboxAgent(context.Background(), newRequest(map[string]any{
		paramPrompt: "hi",
	}))
	require.NoError(t, err)
	assert.Equal(t, sandbox.EgressNone, r.gotAgentReq.Egress)
}

func TestHandleSandboxAgent_RunnerErrorSurfaced(t *testing.T) {
	r := &fakeRunner{agentErr: errors.New(testError)}
	s := NewServer(r)
	got, err := s.handleSandboxAgent(context.Background(), newRequest(map[string]any{
		paramPrompt: "hi",
	}))
	require.NoError(t, err)
	assert.True(t, got.IsError, "expected IsError=true")
	assert.Contains(t, resultText(t, got), testError)
}

func TestHandleSandboxResearch_MissingPrompt(t *testing.T) {
	cases := []map[string]any{{}, {paramPrompt: ""}}
	for i, args := range cases {
		r := &fakeRunner{}
		s := NewServer(r)
		got, err := s.handleSandboxResearch(context.Background(), newRequest(args))
		require.NoError(t, err)
		assert.True(t, got.IsError, msgIsError, i, args)
		assert.Zero(t, r.researchCalls, msgNoCall, i)
	}
}

func TestHandleSandboxResearch_HappyPath(t *testing.T) {
	r := &fakeRunner{
		researchRes: sandbox.AgentResult{
			JobID:      sandbox.JobID("rsh"),
			OutputPath: "/tmp/demesne-out/rsh",
			Stdout:     doneStdout,
			ExitCode:   0,
			CostUSD:    0.42,
		},
	}
	s := NewServer(r)
	got, err := s.handleSandboxResearch(context.Background(), newRequest(map[string]any{
		paramPrompt:   "investigate the corpus",
		paramModel:    testModelSonnet,
		paramPreamble: "stay focused",
	}))
	require.NoError(t, err)
	require.False(t, got.IsError, msgUnexpectedErr, resultText(t, got))
	assert.Equal(t, sandbox.ResearchRequest{
		Agent:    "",
		Model:    testModelSonnet,
		Prompt:   "investigate the corpus",
		Preamble: "stay focused",
	}, r.gotResearchReq)
	text := resultText(t, got)
	for _, want := range []string{
		msgExitCodeZero, "output_dir: /tmp/demesne-out/rsh", "job_id: rsh",
		"cost_usd: 0.4200", "DONE",
	} {
		assert.Contains(t, text, want)
	}
	assert.Equal(t, agentRunOutput{
		ExitCode:  0,
		OutputDir: "/tmp/demesne-out/rsh",
		JobID:     "rsh",
		CostUSD:   0.42,
		Stdout:    doneStdout,
	}, resultStructured[agentRunOutput](t, got))
}

func TestHandleSandboxResearch_RunnerErrorSurfaced(t *testing.T) {
	r := &fakeRunner{researchErr: errors.New(testError)}
	s := NewServer(r)
	got, err := s.handleSandboxResearch(context.Background(), newRequest(map[string]any{
		paramPrompt: "hi",
	}))
	require.NoError(t, err)
	assert.True(t, got.IsError)
	assert.Contains(t, resultText(t, got), testError)
}
