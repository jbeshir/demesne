package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jbeshir/demesne/internal/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// truncMarker is the verbatim truncation prefix produced by tailStderr when
// stderr exceeds stderrResultLimit. Hardcoded here so format_test stays
// independent of the unexported constant in package sandbox.
const truncMarker = "[stderr truncated to last 16384 bytes; full log in output_dir/stderr.log]\n"

func marshalStructured(t *testing.T, s any) string {
	t.Helper()
	data, err := json.Marshal(s)
	require.NoError(t, err)
	return string(data)
}

// --- formatScriptResult ---

func TestFormatScriptResult_StderrField(t *testing.T) {
	res := sandbox.ScriptResult{Stderr: "boom"}
	got := formatScriptResult(res)

	// a. structuredContent JSON contains "stderr":"boom"
	j := marshalStructured(t, got.StructuredContent)
	assert.Contains(t, j, `"stderr":"boom"`)

	// b. text payload contains ---stderr---\nboom
	text := resultText(t, got)
	assert.Contains(t, text, "---stderr---\nboom")
}

func TestFormatScriptResult_StderrTailMarker(t *testing.T) {
	// Build a Stderr value as tailStderr would produce for >16 KiB input.
	largeStderr := truncMarker + strings.Repeat("x", 16384)
	res := sandbox.ScriptResult{Stderr: largeStderr}
	got := formatScriptResult(res)

	j := marshalStructured(t, got.StructuredContent)
	assert.Contains(t, j, "stderr truncated")
}

// --- formatExecResult ---

func TestFormatExecResult_StderrField(t *testing.T) {
	res := sandbox.ExecResult{ExitCode: 0, Stdout: "out\n", Stderr: "err\n"}
	got := formatExecResult(res)

	j := marshalStructured(t, got.StructuredContent)
	assert.Contains(t, j, `"stderr":"err\n"`)

	text := resultText(t, got)
	assert.Contains(t, text, "---stderr---\nerr\n")
}

func TestFormatExecResult_StderrTailMarker(t *testing.T) {
	largeStderr := truncMarker + strings.Repeat("y", 16384)
	res := sandbox.ExecResult{Stderr: largeStderr}
	got := formatExecResult(res)

	j := marshalStructured(t, got.StructuredContent)
	assert.Contains(t, j, "stderr truncated")
}

// --- formatAgentRunResult ---

func TestFormatAgentRunResult_StderrField(t *testing.T) {
	res := sandbox.AgentResult{
		JobID:  sandbox.JobID("abc"),
		Stdout: doneStdout,
		Stderr: "warn: something\n",
	}
	got := formatAgentRunResult(res)

	j := marshalStructured(t, got.StructuredContent)
	assert.Contains(t, j, `"stderr":"warn: something\n"`)

	text := resultText(t, got)
	assert.Contains(t, text, "---stderr---\nwarn: something\n")
}

func TestFormatAgentRunResult_StderrTailMarker(t *testing.T) {
	largeStderr := truncMarker + strings.Repeat("z", 16384)
	res := sandbox.AgentResult{Stderr: largeStderr}
	got := formatAgentRunResult(res)

	j := marshalStructured(t, got.StructuredContent)
	assert.Contains(t, j, "stderr truncated")
}
