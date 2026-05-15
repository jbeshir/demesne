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
