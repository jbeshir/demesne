package server

import (
	"context"
	"fmt"
	"testing"

	"github.com/jbeshir/demesne/internal/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sentinel job IDs used in background-path assertions. Named here so the
// goconst linter does not flag repeated literals.
const (
	bgScriptJobID   = "bg-script-job"
	bgAgentJobID    = "bg-agent-job"
	bgResearchJobID = "bg-research-job"
	bgStatusJobID   = "status-job-abc"
	bgWaitJobID     = "wait-job-xyz"
	bgCancelJobID   = "cancel-job-xyz"
)

// statusRunning is the literal status text produced by formatJobStarted and
// formatStatusResult / formatWaitResult for running jobs.
const statusRunning = "running"

// testNoSuchJobID is a job ID used in error-path tests where no matching job exists.
const testNoSuchJobID = "no-such"

// --- Background path: sandbox_script ---

func TestHandleSandboxScript_BackgroundTrue(t *testing.T) {
	r := &fakeRunner{startScriptJobID: bgScriptJobID}
	s := NewServer(r)
	got, err := s.handleSandboxScript(context.Background(), newRequest(map[string]any{
		paramCommand:    testCmdEcho,
		paramBackground: true,
	}))
	require.NoError(t, err)
	require.False(t, got.IsError, msgUnexpectedErr, resultText(t, got))
	assert.Equal(t, 1, r.startScriptCalls, "StartScript must be called once")
	assert.Equal(t, 0, r.scriptCalls, "RunScript must not be called in background mode")
	text := resultText(t, got)
	assert.Contains(t, text, bgScriptJobID)
	assert.Contains(t, text, statusRunning)
	out := resultStructured[jobStartedOutput](t, got)
	assert.Equal(t, bgScriptJobID, out.JobID)
	assert.Equal(t, string(sandbox.JobStatusRunning), out.Status)
}

func TestHandleSandboxScript_BackgroundFalseUsesBlocking(t *testing.T) {
	r := &fakeRunner{scriptRes: sandbox.ScriptResult{JobID: "sync-script", Stdout: "hi\n"}}
	s := NewServer(r)
	_, err := s.handleSandboxScript(context.Background(), newRequest(map[string]any{
		paramCommand: testCmdEcho,
	}))
	require.NoError(t, err)
	assert.Equal(t, 1, r.scriptCalls, "RunScript must be called in blocking mode")
	assert.Equal(t, 0, r.startScriptCalls, "StartScript must not be called in blocking mode")
}

// --- Background path: sandbox_agent ---

func TestHandleSandboxAgent_BackgroundTrue(t *testing.T) {
	r := &fakeRunner{startAgentJobID: bgAgentJobID}
	s := NewServer(r)
	got, err := s.handleSandboxAgent(context.Background(), newRequest(map[string]any{
		paramPrompt:     testPromptWork,
		paramBackground: true,
	}))
	require.NoError(t, err)
	require.False(t, got.IsError, msgUnexpectedErr, resultText(t, got))
	assert.Equal(t, 1, r.startAgentCalls, "StartAgent must be called once")
	assert.Equal(t, 0, r.agentCalls, "Agent must not be called in background mode")
	out := resultStructured[jobStartedOutput](t, got)
	assert.Equal(t, bgAgentJobID, out.JobID)
	assert.Equal(t, string(sandbox.JobStatusRunning), out.Status)
}

func TestHandleSandboxAgent_BackgroundFalseUsesBlocking(t *testing.T) {
	r := &fakeRunner{agentRes: sandbox.AgentResult{JobID: "sync-agent"}}
	s := NewServer(r)
	_, err := s.handleSandboxAgent(context.Background(), newRequest(map[string]any{
		paramPrompt: "do something",
	}))
	require.NoError(t, err)
	assert.Equal(t, 1, r.agentCalls, "Agent must be called in blocking mode")
	assert.Equal(t, 0, r.startAgentCalls, "StartAgent must not be called in blocking mode")
}

// --- Background path: sandbox_research ---

func TestHandleSandboxResearch_BackgroundTrue(t *testing.T) {
	r := &fakeRunner{startResearchJobID: bgResearchJobID}
	s := NewServer(r)
	got, err := s.handleSandboxResearch(context.Background(), newRequest(map[string]any{
		paramPrompt:     "investigate",
		paramBackground: true,
	}))
	require.NoError(t, err)
	require.False(t, got.IsError, msgUnexpectedErr, resultText(t, got))
	assert.Equal(t, 1, r.startResearchCalls, "StartResearch must be called once")
	assert.Equal(t, 0, r.researchCalls, "Research must not be called in background mode")
	out := resultStructured[jobStartedOutput](t, got)
	assert.Equal(t, bgResearchJobID, out.JobID)
	assert.Equal(t, string(sandbox.JobStatusRunning), out.Status)
}

func TestHandleSandboxResearch_BackgroundFalseUsesBlocking(t *testing.T) {
	r := &fakeRunner{researchRes: sandbox.AgentResult{JobID: "sync-research"}}
	s := NewServer(r)
	_, err := s.handleSandboxResearch(context.Background(), newRequest(map[string]any{
		paramPrompt: "investigate",
	}))
	require.NoError(t, err)
	assert.Equal(t, 1, r.researchCalls, "Research must be called in blocking mode")
	assert.Equal(t, 0, r.startResearchCalls, "StartResearch must not be called in blocking mode")
}

// --- handleSandboxStatus ---

func TestHandleSandboxStatus_MissingJobID(t *testing.T) {
	r := &fakeRunner{}
	s := NewServer(r)
	got, err := s.handleSandboxStatus(context.Background(), newRequest(map[string]any{}))
	require.NoError(t, err)
	assert.True(t, got.IsError, "expected IsError for missing job_id")
	assert.Zero(t, r.statusCalls, "Status must not be called without a job_id")
}

func TestHandleSandboxStatus_HappyPath(t *testing.T) {
	r := &fakeRunner{
		statusResult: sandbox.StatusResult{
			JobID:          bgStatusJobID,
			Status:         sandbox.JobStatusSucceeded,
			ElapsedSeconds: 1.5,
			StdoutTail:     "tail\n",
			CostUSD:        0.002,
			ExitCode:       0,
		},
	}
	s := NewServer(r)
	got, err := s.handleSandboxStatus(context.Background(), newRequest(map[string]any{
		paramJobID: bgStatusJobID,
	}))
	require.NoError(t, err)
	require.False(t, got.IsError, msgUnexpectedErr, resultText(t, got))
	assert.Equal(t, 1, r.statusCalls)
	assert.Equal(t, sandbox.StatusRequest{JobID: sandbox.JobID(bgStatusJobID)}, r.gotStatusReq)
	out := resultStructured[statusOutput](t, got)
	assert.Equal(t, bgStatusJobID, out.JobID)
	assert.Equal(t, string(sandbox.JobStatusSucceeded), out.Status)
	assert.InDelta(t, 1.5, out.ElapsedSeconds, 1e-9)
	assert.Equal(t, "tail\n", out.StdoutTail)
}

func TestHandleSandboxStatus_JobNotFound(t *testing.T) {
	r := &fakeRunner{statusErr: fmt.Errorf("%w: no-such", sandbox.ErrJobNotFound)}
	s := NewServer(r)
	got, err := s.handleSandboxStatus(context.Background(), newRequest(map[string]any{
		paramJobID: testNoSuchJobID,
	}))
	require.NoError(t, err)
	assert.True(t, got.IsError, "expected IsError for job-not-found")
	assert.Equal(t, 1, r.statusCalls)
}

// --- handleSandboxWait ---

func TestHandleSandboxWait_MissingJobID(t *testing.T) {
	r := &fakeRunner{}
	s := NewServer(r)
	got, err := s.handleSandboxWait(context.Background(), newRequest(map[string]any{}))
	require.NoError(t, err)
	assert.True(t, got.IsError, "expected IsError for missing job_id")
	assert.Zero(t, r.waitCalls, "Wait must not be called without a job_id")
}

func TestHandleSandboxWait_HappyPath(t *testing.T) {
	r := &fakeRunner{
		waitResult: sandbox.WaitResult{
			JobID:      bgWaitJobID,
			Status:     sandbox.JobStatusSucceeded,
			ResultText: "done",
			ExitCode:   0,
			CostUSD:    0.01,
		},
	}
	s := NewServer(r)
	got, err := s.handleSandboxWait(context.Background(), newRequest(map[string]any{
		paramJobID:          bgWaitJobID,
		paramTimeoutSeconds: 30,
	}))
	require.NoError(t, err)
	require.False(t, got.IsError, msgUnexpectedErr, resultText(t, got))
	assert.Equal(t, 1, r.waitCalls)
	assert.Equal(t, sandbox.WaitRequest{JobID: sandbox.JobID(bgWaitJobID), TimeoutSeconds: 30}, r.gotWaitReq)
	out := resultStructured[waitOutput](t, got)
	assert.Equal(t, bgWaitJobID, out.JobID)
	assert.Equal(t, string(sandbox.JobStatusSucceeded), out.Status)
	assert.Equal(t, "done", out.ResultText)
}

// TestHandleSandboxWait_StillRunning asserts that a "still running" wait
// result is returned as a normal (non-error) MCP result, not an error.
func TestHandleSandboxWait_StillRunning(t *testing.T) {
	r := &fakeRunner{
		waitResult: sandbox.WaitResult{
			JobID:   bgWaitJobID,
			Status:  sandbox.JobStatusRunning,
			Message: "still running; call sandbox_wait again",
		},
	}
	s := NewServer(r)
	got, err := s.handleSandboxWait(context.Background(), newRequest(map[string]any{
		paramJobID: bgWaitJobID,
	}))
	require.NoError(t, err)
	require.False(t, got.IsError, "still-running must be a non-error result")
	out := resultStructured[waitOutput](t, got)
	assert.Equal(t, string(sandbox.JobStatusRunning), out.Status)
	assert.Contains(t, out.Message, "still running")
}

func TestHandleSandboxWait_JobNotFound(t *testing.T) {
	r := &fakeRunner{waitErr: fmt.Errorf("%w: no-such", sandbox.ErrJobNotFound)}
	s := NewServer(r)
	got, err := s.handleSandboxWait(context.Background(), newRequest(map[string]any{
		paramJobID: testNoSuchJobID,
	}))
	require.NoError(t, err)
	assert.True(t, got.IsError, "expected IsError for job-not-found")
	assert.Equal(t, 1, r.waitCalls)
}

// --- handleSandboxCancel ---

func TestHandleSandboxCancel_MissingJobID(t *testing.T) {
	r := &fakeRunner{}
	s := NewServer(r)
	got, err := s.handleSandboxCancel(context.Background(), newRequest(map[string]any{}))
	require.NoError(t, err)
	assert.True(t, got.IsError, "expected IsError for missing job_id")
	assert.Zero(t, r.cancelCalls, "Cancel must not be called without a job_id")
}

func TestHandleSandboxCancel_HappyPath(t *testing.T) {
	r := &fakeRunner{
		cancelResult: sandbox.CancelResult{
			JobID:  bgCancelJobID,
			Status: sandbox.JobStatusCancelled,
		},
	}
	s := NewServer(r)
	got, err := s.handleSandboxCancel(context.Background(), newRequest(map[string]any{
		paramJobID: bgCancelJobID,
	}))
	require.NoError(t, err)
	require.False(t, got.IsError, msgUnexpectedErr, resultText(t, got))
	assert.Equal(t, 1, r.cancelCalls)
	assert.Equal(t, sandbox.CancelRequest{JobID: sandbox.JobID(bgCancelJobID)}, r.gotCancelReq)
	out := resultStructured[cancelOutput](t, got)
	assert.Equal(t, bgCancelJobID, out.JobID)
	assert.Equal(t, string(sandbox.JobStatusCancelled), out.Status)
}

func TestHandleSandboxCancel_JobNotFound(t *testing.T) {
	r := &fakeRunner{cancelErr: fmt.Errorf("%w: no-such", sandbox.ErrJobNotFound)}
	s := NewServer(r)
	got, err := s.handleSandboxCancel(context.Background(), newRequest(map[string]any{
		paramJobID: testNoSuchJobID,
	}))
	require.NoError(t, err)
	assert.True(t, got.IsError, "expected IsError for job-not-found")
	assert.Equal(t, 1, r.cancelCalls)
}

// --- Registration tests ---

// TestAsyncToolRegistration mirrors agent_enum_test.go: asserts sandbox_status,
// sandbox_wait, and sandbox_cancel appear in the registered tool catalogue, and
// that sandbox_script/agent/research each expose a "background" property.
func TestAsyncToolRegistration(t *testing.T) {
	s := NewServer(&fakeRunner{})
	tools := s.mcpServer.ListTools()

	for _, want := range []string{
		sandbox.ToolSandboxStatus,
		sandbox.ToolSandboxWait,
		sandbox.ToolSandboxCancel,
	} {
		_, ok := tools[want]
		assert.True(t, ok, "tool %q not registered", want)
	}

	for _, toolName := range []string{
		sandbox.ToolSandboxScript,
		sandbox.ToolSandboxAgent,
		sandbox.ToolSandboxResearch,
	} {
		st, ok := tools[toolName]
		require.True(t, ok, "tool %q not registered", toolName)
		_, hasBg := st.Tool.InputSchema.Properties[paramBackground]
		assert.True(t, hasBg, "tool %q must expose %q property", toolName, paramBackground)
	}
}
