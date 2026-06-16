package server

import (
	"fmt"
	"strings"

	"github.com/jbeshir/demesne/internal/sandbox"
	"github.com/mark3labs/mcp-go/mcp"
)

// The *Output types are the structuredContent shapes for the stable-result
// tools. WithOutputSchema reflects them into each tool's outputSchema, and the
// formatters return them via NewToolResultStructured. Both Claude Code and
// Codex feed the model only the structuredContent and discard the text block,
// so each Output must carry the complete result (including stdout); the text
// fallback is retained for clients that don't consume structuredContent.

type scriptOutput struct {
	ExitCode  int    `json:"exit_code"`
	OutputDir string `json:"output_dir"`
	JobID     string `json:"job_id"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
}

type createOutput struct {
	SandboxID string `json:"sandbox_id"`
	OutputDir string `json:"output_dir"`
}

type execOutput struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

type agentRunOutput struct {
	ExitCode      int     `json:"exit_code"`
	OutputDir     string  `json:"output_dir"`
	JobID         string  `json:"job_id"`
	CostUSD       float64 `json:"cost_usd"`
	TotalUsageUSD float64 `json:"total_usage_usd"`
	Stdout        string  `json:"stdout"`
	Stderr        string  `json:"stderr"`
}

func formatScriptResult(res sandbox.ScriptResult) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "exit_code: %d\n", res.ExitCode)
	fmt.Fprintf(&b, "output_dir: %s\n", res.OutputPath)
	fmt.Fprintf(&b, "job_id: %s\n", res.JobID)
	b.WriteString("---\n")
	b.WriteString(res.Stdout)
	b.WriteString("---stderr---\n")
	b.WriteString(res.Stderr)
	return mcp.NewToolResultStructured(scriptOutput{
		ExitCode:  res.ExitCode,
		OutputDir: res.OutputPath,
		JobID:     string(res.JobID),
		Stdout:    res.Stdout,
		Stderr:    res.Stderr,
	}, b.String())
}

func formatCreateResult(res sandbox.CreateResult) *mcp.CallToolResult {
	text := fmt.Sprintf("sandbox_id: %s\noutput_dir: %s", res.SandboxID, res.OutputPath)
	return mcp.NewToolResultStructured(createOutput{
		SandboxID: string(res.SandboxID),
		OutputDir: res.OutputPath,
	}, text)
}

func formatExecResult(res sandbox.ExecResult) *mcp.CallToolResult {
	text := fmt.Sprintf("exit_code: %d\n---\n%s\n---stderr---\n%s", res.ExitCode, res.Stdout, res.Stderr)
	return mcp.NewToolResultStructured(execOutput{
		ExitCode: res.ExitCode,
		Stdout:   res.Stdout,
		Stderr:   res.Stderr,
	}, text)
}

type jobStartedOutput struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
}

type statusOutput struct {
	JobID          string  `json:"job_id"`
	Status         string  `json:"status"`
	ElapsedSeconds float64 `json:"elapsed_seconds"`
	StdoutTail     string  `json:"stdout_tail,omitempty"`
	CostUSD        float64 `json:"cost_usd,omitempty"`
	TotalUsageUSD  float64 `json:"total_usage_usd,omitempty"`
	ExitCode       int     `json:"exit_code,omitempty"`
	Message        string  `json:"message,omitempty"`
}

type waitOutput struct {
	JobID         string  `json:"job_id"`
	Status        string  `json:"status"`
	Message       string  `json:"message,omitempty"`
	ResultText    string  `json:"result_text,omitempty"`
	OutputPath    string  `json:"output_path,omitempty"`
	ExitCode      int     `json:"exit_code,omitempty"`
	CostUSD       float64 `json:"cost_usd,omitempty"`
	TotalUsageUSD float64 `json:"total_usage_usd,omitempty"`
}

type cancelOutput struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
}

func formatJobStarted(jobID sandbox.JobID) *mcp.CallToolResult {
	text := fmt.Sprintf("job_id: %s\nstatus: running", jobID)
	return mcp.NewToolResultStructured(jobStartedOutput{
		JobID:  string(jobID),
		Status: string(sandbox.JobStatusRunning),
	}, text)
}

func formatStatusResult(res sandbox.StatusResult) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "job_id: %s\nstatus: %s\nelapsed_seconds: %.1f\n",
		res.JobID, res.Status, res.ElapsedSeconds)
	if res.Message != "" {
		fmt.Fprintf(&b, "message: %s\n", res.Message)
	}
	if res.StdoutTail != "" {
		fmt.Fprintf(&b, "---stdout_tail---\n%s", res.StdoutTail)
	}
	return mcp.NewToolResultStructured(statusOutput{
		JobID:          string(res.JobID),
		Status:         string(res.Status),
		ElapsedSeconds: res.ElapsedSeconds,
		StdoutTail:     res.StdoutTail,
		CostUSD:        res.CostUSD,
		TotalUsageUSD:  res.TotalUsageUSD,
		ExitCode:       res.ExitCode,
		Message:        res.Message,
	}, b.String())
}

func formatWaitResult(res sandbox.WaitResult) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "job_id: %s\nstatus: %s\n", res.JobID, res.Status)
	if res.Message != "" {
		fmt.Fprintf(&b, "message: %s\n", res.Message)
	}
	if res.ResultText != "" {
		fmt.Fprintf(&b, "---\n%s", res.ResultText)
	}
	return mcp.NewToolResultStructured(waitOutput{
		JobID:         string(res.JobID),
		Status:        string(res.Status),
		Message:       res.Message,
		ResultText:    res.ResultText,
		OutputPath:    res.OutputPath,
		ExitCode:      res.ExitCode,
		CostUSD:       res.CostUSD,
		TotalUsageUSD: res.TotalUsageUSD,
	}, b.String())
}

func formatCancelResult(res sandbox.CancelResult) *mcp.CallToolResult {
	text := fmt.Sprintf("job_id: %s\nstatus: %s", res.JobID, res.Status)
	return mcp.NewToolResultStructured(cancelOutput{
		JobID:  string(res.JobID),
		Status: string(res.Status),
	}, text)
}

// formatAgentRunResult is the shared output formatter for sandbox_agent and
// sandbox_research. Both surface the same set of fields; keeping a single
// formatter ensures the result doesn't drift between them. total_usage_usd
// adds the spend of any child sandboxes the run spawned.
func formatAgentRunResult(res sandbox.AgentResult) *mcp.CallToolResult {
	text := fmt.Sprintf(
		"exit_code: %d\noutput_dir: %s\njob_id: %s\ncost_usd: %.4f\ntotal_usage_usd: %.4f\n---\n%s\n---stderr---\n%s",
		res.ExitCode, res.OutputPath, res.JobID, res.CostUSD, res.TotalUsageUSD, res.Stdout, res.Stderr,
	)
	return mcp.NewToolResultStructured(agentRunOutput{
		ExitCode:      res.ExitCode,
		OutputDir:     res.OutputPath,
		JobID:         string(res.JobID),
		CostUSD:       res.CostUSD,
		TotalUsageUSD: res.TotalUsageUSD,
		Stdout:        res.Stdout,
		Stderr:        res.Stderr,
	}, text)
}
