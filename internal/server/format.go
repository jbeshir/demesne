package server

import (
	"fmt"
	"strings"

	"github.com/jbeshir/demesne/internal/sandbox"
	"github.com/mark3labs/mcp-go/mcp"
)

func formatScriptResult(res sandbox.ScriptResult) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "exit_code: %d\n", res.ExitCode)
	fmt.Fprintf(&b, "output_dir: %s\n", res.OutputPath)
	fmt.Fprintf(&b, "job_id: %s\n", res.JobID)
	b.WriteString("---\n")
	b.WriteString(res.Stdout)
	return mcp.NewToolResultText(b.String())
}

// formatAgentRunResult is the shared output formatter for sandbox_agent
// and sandbox_research. Both surface the same set of fields; keeping a
// single formatter ensures the result text doesn't drift between them.
// total_usage_usd adds the spend of any child sandboxes the run spawned.
func formatAgentRunResult(
	exitCode int,
	outputPath string,
	jobID sandbox.JobID,
	costUSD, totalUsageUSD float64,
	stdout string,
) string {
	return fmt.Sprintf(
		"exit_code: %d\noutput_dir: %s\njob_id: %s\ncost_usd: %.4f\ntotal_usage_usd: %.4f\n---\n%s",
		exitCode, outputPath, jobID, costUSD, totalUsageUSD, stdout,
	)
}
