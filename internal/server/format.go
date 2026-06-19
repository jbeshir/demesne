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
	// PerModelTokens is the tree-aggregated per-model token-type breakdown
	// (input/output/cache_creation/cache_read), omitted when the run had no
	// tracked usage. UsageSummary is a one-line cache-read% / top-subagent
	// digest, omitted when there is no usage. Both are additive — older
	// clients that ignore them see the unchanged result shape.
	PerModelTokens map[string]tokenTotalsOutput `json:"per_model_tokens,omitempty"`
	UsageSummary   string                       `json:"usage_summary,omitempty"`
	Stdout         string                       `json:"stdout"`
	Stderr         string                       `json:"stderr"`
}

// toTokenTotalsOutput converts the sandbox per-model token map into the
// server's snake_case output shape. Returns nil for an empty map so the
// omitempty field stays absent.
func toTokenTotalsOutput(m map[string]sandbox.TokenTotals) map[string]tokenTotalsOutput {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]tokenTotalsOutput, len(m))
	for model, t := range m {
		out[model] = tokenTotalsOutput{
			Input:         t.Input,
			Output:        t.Output,
			CacheCreation: t.CacheCreation,
			CacheRead:     t.CacheRead,
		}
	}
	return out
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

type tokenTotalsOutput struct {
	Input         int64 `json:"input"`
	Output        int64 `json:"output"`
	CacheCreation int64 `json:"cache_creation"`
	CacheRead     int64 `json:"cache_read"`
}

type modelUsageOutput struct {
	Model         string  `json:"model"`
	Input         int64   `json:"input"`
	Output        int64   `json:"output"`
	CacheCreation int64   `json:"cache_creation"`
	CacheRead     int64   `json:"cache_read"`
	CostUSD       float64 `json:"cost_usd"`
}

type childUsageOutput struct {
	Name          string  `json:"name"`
	Depth         int     `json:"depth"`
	CostUSD       float64 `json:"cost_usd"`
	Input         int64   `json:"input"`
	Output        int64   `json:"output"`
	CacheCreation int64   `json:"cache_creation"`
	CacheRead     int64   `json:"cache_read"`
}

type subagentUsageOutput struct {
	Name          string  `json:"name"`
	Input         int64   `json:"input"`
	Output        int64   `json:"output"`
	CacheCreation int64   `json:"cache_creation"`
	CacheRead     int64   `json:"cache_read"`
	CostUSD       float64 `json:"cost_usd"`
	Requests      int     `json:"requests"`
}

type droppedCountsOutput struct {
	ParseErrors  int64 `json:"parse_errors"`
	NoUsageBlock int64 `json:"no_usage_block"`
}

type usageReportOutput struct {
	TotalCostUSD    float64               `json:"total_cost_usd"`
	TokenTypeTotals tokenTotalsOutput     `json:"token_type_totals"`
	CacheReadPct    float64               `json:"cache_read_pct"`
	ByModel         []modelUsageOutput    `json:"by_model"`
	ByChild         []childUsageOutput    `json:"by_child"`
	BySubagent      []subagentUsageOutput `json:"by_subagent"`
	Dropped         droppedCountsOutput   `json:"dropped"`
}

func formatUsageReport(res sandbox.UsageReport) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "total_cost_usd: $%.4f\n", res.TotalCostUSD)
	fmt.Fprintf(&b, "cache_read_pct: %.1f%%\n", res.CacheReadPct)
	fmt.Fprintf(&b, "tokens: input=%d output=%d cache_creation=%d cache_read=%d\n",
		res.TokenTypeTotals.Input, res.TokenTypeTotals.Output,
		res.TokenTypeTotals.CacheCreation, res.TokenTypeTotals.CacheRead)

	if len(res.ByModel) > 0 {
		b.WriteString("\nby_model:\n")
		for _, m := range res.ByModel {
			fmt.Fprintf(&b, "  %-40s input=%d output=%d cache_creation=%d cache_read=%d cost=$%.4f\n",
				m.Model, m.Input, m.Output, m.CacheCreation, m.CacheRead, m.CostUSD)
		}
	}

	if len(res.BySubagent) > 0 {
		b.WriteString("\nby_subagent:\n")
		for _, sa := range res.BySubagent {
			fmt.Fprintf(&b, "  %-30s requests=%d cost=$%.4f\n",
				sa.Name, sa.Requests, sa.CostUSD)
		}
	}

	if len(res.ByChild) > 0 {
		b.WriteString("\nby_child:\n")
		for _, c := range res.ByChild {
			name := c.Name
			if name == "" {
				name = "(root)"
			}
			fmt.Fprintf(&b, "  %-30s depth=%d cost=$%.4f input=%d output=%d\n",
				name, c.Depth, c.CostUSD, c.Input, c.Output)
		}
	}

	fmt.Fprintf(&b, "\ndropped: parse_errors=%d no_usage_block=%d\n",
		res.Dropped.ParseErrors, res.Dropped.NoUsageBlock)

	out := usageReportOutput{
		TotalCostUSD: res.TotalCostUSD,
		TokenTypeTotals: tokenTotalsOutput{
			Input:         res.TokenTypeTotals.Input,
			Output:        res.TokenTypeTotals.Output,
			CacheCreation: res.TokenTypeTotals.CacheCreation,
			CacheRead:     res.TokenTypeTotals.CacheRead,
		},
		CacheReadPct: res.CacheReadPct,
		Dropped: droppedCountsOutput{
			ParseErrors:  res.Dropped.ParseErrors,
			NoUsageBlock: res.Dropped.NoUsageBlock,
		},
	}

	for _, m := range res.ByModel {
		out.ByModel = append(out.ByModel, modelUsageOutput{
			Model:         m.Model,
			Input:         m.Input,
			Output:        m.Output,
			CacheCreation: m.CacheCreation,
			CacheRead:     m.CacheRead,
			CostUSD:       m.CostUSD,
		})
	}

	for _, c := range res.ByChild {
		out.ByChild = append(out.ByChild, childUsageOutput{
			Name:          c.Name,
			Depth:         c.Depth,
			CostUSD:       c.CostUSD,
			Input:         c.Input,
			Output:        c.Output,
			CacheCreation: c.CacheCreation,
			CacheRead:     c.CacheRead,
		})
	}

	for _, sa := range res.BySubagent {
		out.BySubagent = append(out.BySubagent, subagentUsageOutput{
			Name:          sa.Name,
			Input:         sa.Input,
			Output:        sa.Output,
			CacheCreation: sa.CacheCreation,
			CacheRead:     sa.CacheRead,
			CostUSD:       sa.CostUSD,
			Requests:      sa.Requests,
		})
	}

	return mcp.NewToolResultStructured(out, b.String())
}

// formatAgentRunResult is the shared output formatter for sandbox_agent and
// sandbox_research. Both surface the same set of fields; keeping a single
// formatter ensures the result doesn't drift between them. total_usage_usd
// adds the spend of any child sandboxes the run spawned.
func formatAgentRunResult(res sandbox.AgentResult) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b,
		"exit_code: %d\noutput_dir: %s\njob_id: %s\ncost_usd: %.4f\ntotal_usage_usd: %.4f\n",
		res.ExitCode, res.OutputPath, res.JobID, res.CostUSD, res.TotalUsageUSD,
	)
	if res.UsageSummary != "" {
		fmt.Fprintf(&b, "usage_summary: %s\n", res.UsageSummary)
	}
	fmt.Fprintf(&b, "---\n%s\n---stderr---\n%s", res.Stdout, res.Stderr)
	return mcp.NewToolResultStructured(agentRunOutput{
		ExitCode:       res.ExitCode,
		OutputDir:      res.OutputPath,
		JobID:          string(res.JobID),
		CostUSD:        res.CostUSD,
		TotalUsageUSD:  res.TotalUsageUSD,
		PerModelTokens: toTokenTotalsOutput(res.PerModelTokens),
		UsageSummary:   res.UsageSummary,
		Stdout:         res.Stdout,
		Stderr:         res.Stderr,
	}, b.String())
}
