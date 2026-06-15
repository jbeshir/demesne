package sandbox

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jbeshir/demesne/internal/mcpproxy"
	proxymcp "github.com/jbeshir/demesne/internal/proxies/mcp"
)

// testChildName is the child name reused across the spawn-handler tests.
const testChildName = "child"

const (
	testExistingJobID = "job-existing"
	testNewJobID      = "job-new"
)

func TestValidateChildName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{name: "a"},
		{name: "probe-1"},
		{name: "phase01"},
		{name: "ab-cd-3"},
		{name: strings.Repeat("a", 40)},
		{name: "a-b"},
		{name: "a--b"},
		{name: "", wantErr: true},
		{name: strings.Repeat("a", 41), wantErr: true},
		{name: "-", wantErr: true},
		{name: "--", wantErr: true},
		{name: "-x", wantErr: true},
		{name: "x-", wantErr: true},
		{name: "..", wantErr: true},
		{name: ".", wantErr: true},
		{name: "a/b", wantErr: true},
		{name: "a b", wantErr: true},
		{name: "a:b", wantErr: true},
		{name: "../escape", wantErr: true},
		{name: "my_child.v2", wantErr: true},
		{name: "ABC", wantErr: true},
		{name: "a_b", wantErr: true},
		{name: "a.b", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateChildName(tt.name)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestParentFromRequest(t *testing.T) {
	tests := []struct {
		name       string
		existingID string
		headerID   string
		wantID     any
		wantSame   bool
	}{
		{
			name:       "absent header preserves existing context",
			existingID: testExistingJobID,
			wantID:     testExistingJobID,
			wantSame:   true,
		},
		{
			name:     "absent header leaves empty context unchanged",
			wantID:   nil,
			wantSame: true,
		},
		{
			name:       "present header replaces existing parent id",
			existingID: testExistingJobID,
			headerID:   testNewJobID,
			wantID:     testNewJobID,
		},
		{
			name:     "present header sets parent id",
			headerID: testNewJobID,
			wantID:   testNewJobID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.existingID != "" {
				ctx = context.WithValue(ctx, parentKey, tt.existingID)
			}
			req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://x/demesne/mcp", nil)
			require.NoError(t, err)
			if tt.headerID != "" {
				req.Header.Set(proxymcp.ParentHeader, tt.headerID)
			}

			got := parentFromRequest(ctx, req)
			assert.Equal(t, tt.wantID, got.Value(parentKey))
			if tt.wantSame {
				assert.Equal(t, ctx, got)
			} else {
				assert.NotEqual(t, ctx, got)
			}
		})
	}
}

func TestReserveName_Unique(t *testing.T) {
	c := &spawnContext{usedNames: map[string]bool{}}
	require.NoError(t, c.reserveName("alpha"))
	require.Error(t, c.reserveName("alpha"), "duplicate must be rejected")
	require.NoError(t, c.reserveName("beta"))
}

func TestChildMCPServer_Catalogue(t *testing.T) {
	r := NewRunner(Config{})
	name, tools, handler := r.ChildMCPServer()
	assert.Equal(t, mcpproxy.DemesneServerName, name)
	require.NotNil(t, handler)

	got := map[string]bool{}
	for _, tl := range tools {
		got[tl.Name] = true
	}
	for _, want := range []string{
		ToolSandboxScript, ToolSandboxAgent, ToolSandboxResearch,
		ToolSandboxStatus, ToolSandboxWait, ToolSandboxCancel,
		ToolSandboxCreate, ToolSandboxExec, ToolSandboxDestroy,
	} {
		assert.True(t, got[want], "missing tool %q", want)
	}
	// upload/download are intentionally not exposed in-sandbox.
	assert.False(t, got["sandbox_upload"])
	assert.False(t, got["sandbox_download"])
}

func TestParentFor(t *testing.T) {
	r := NewRunner(Config{})
	parent := &spawnContext{usedNames: map[string]bool{}}
	r.registry.Register(JobID("job-7"), parent)

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
	parent := &spawnContext{usedNames: map[string]bool{}}
	r.registry.Register(JobID("job-9"), parent)
	ctx := context.WithValue(context.Background(), parentKey, "job-9")

	req := mcp.CallToolRequest{}
	req.Params.Name = ToolSandboxAgent
	req.Params.Arguments = map[string]any{
		childParamName:   testChildName,
		childParamPrompt: "do a thing",
		childParamEgress: string(EgressOpen),
	}
	res, err := r.handleChildAgent(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.IsError)
}

func TestHandleChildScript_RejectsOpenEgress(t *testing.T) {
	r := NewRunner(Config{})
	r.registry.Register(JobID("job-11"), &spawnContext{usedNames: map[string]bool{}})
	ctx := context.WithValue(context.Background(), parentKey, "job-11")

	req := mcp.CallToolRequest{}
	req.Params.Name = ToolSandboxScript
	req.Params.Arguments = map[string]any{
		childParamName:    testChildName,
		childParamCommand: "echo hi",
		childParamEgress:  string(EgressOpen),
	}
	res, err := r.handleChildScript(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.IsError)
}

func TestHandleChildCreate_RejectsOpenEgress(t *testing.T) {
	r := NewRunner(Config{})
	r.registry.Register(JobID("job-12"), &spawnContext{usedNames: map[string]bool{}})
	ctx := context.WithValue(context.Background(), parentKey, "job-12")

	req := mcp.CallToolRequest{}
	req.Params.Name = ToolSandboxCreate
	req.Params.Arguments = map[string]any{
		childParamName:   testChildName,
		childParamEgress: string(EgressOpen),
	}
	res, err := r.handleChildCreate(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.IsError)
}

// TestChildMCPServer_BackgroundParamPresent verifies that sandbox_script,
// sandbox_agent, and sandbox_research in the child catalogue each expose a
// "background" boolean property (phase02 feature).
func TestChildMCPServer_BackgroundParamPresent(t *testing.T) {
	r := NewRunner(Config{})
	_, tools, _ := r.ChildMCPServer()

	byName := make(map[string]mcp.Tool, len(tools))
	for _, tl := range tools {
		byName[tl.Name] = tl
	}

	for _, toolName := range []string{ToolSandboxScript, ToolSandboxAgent, ToolSandboxResearch} {
		tl, ok := byName[toolName]
		require.True(t, ok, "tool %q not in child catalogue", toolName)
		_, hasBg := tl.InputSchema.Properties[childParamBackground]
		assert.True(t, hasBg, "tool %q must expose %q property", toolName, childParamBackground)
	}
}

func TestHandleChildScript_NoParentIdentity(t *testing.T) {
	r := NewRunner(Config{})
	req := mcp.CallToolRequest{}
	req.Params.Name = ToolSandboxScript
	req.Params.Arguments = map[string]any{childParamName: "x", childParamCommand: "echo hi"}
	res, err := r.handleChildScript(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, res.IsError)
}

func TestFormatChildWaitResult_TerminalCarriesFullResult(t *testing.T) {
	out := formatChildWaitResult(WaitResult{
		JobID:         JobID("job-1"),
		Status:        JobStatusSucceeded,
		ResultText:    "the final answer",
		OutputPath:    "/out/child/x",
		ExitCode:      0,
		CostUSD:       0.0042,
		TotalUsageUSD: 0.0091,
	})
	for _, want := range []string{
		"job_id: job-1", "status: succeeded", "output_dir: /out/child/x",
		"exit_code: 0", "cost_usd: 0.0042", "total_usage_usd: 0.0091", "the final answer",
	} {
		assert.Contains(t, out, want)
	}
}

func TestFormatChildWaitResult_RunningSentinel(t *testing.T) {
	out := formatChildWaitResult(WaitResult{
		JobID:   JobID("job-2"),
		Status:  JobStatusRunning,
		Message: "still running; call sandbox_wait again",
	})
	assert.Contains(t, out, "status: running")
	assert.Contains(t, out, "still running; call sandbox_wait again")
	// A still-running wait must not claim a terminal output dir or cost.
	assert.NotContains(t, out, "output_dir:")
	assert.NotContains(t, out, "cost_usd:")
}

func TestFormatChildStatusResult_CarriesCostAndTail(t *testing.T) {
	out := formatChildStatusResult(StatusResult{
		JobID:          JobID("job-3"),
		Status:         JobStatusRunning,
		ElapsedSeconds: 12.5,
		StdoutTail:     "tail bytes",
		CostUSD:        0.001,
		TotalUsageUSD:  0.001,
		ExitCode:       0,
	})
	for _, want := range []string{
		"job_id: job-3", "status: running", "elapsed_seconds: 12.5",
		"cost_usd: 0.0010", "tail bytes",
	} {
		assert.Contains(t, out, want)
	}
}
