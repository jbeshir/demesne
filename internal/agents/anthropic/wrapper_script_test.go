package anthropic

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClaude is a /bin/sh script emitted into a temp bin dir for each test.
// It records its argv (space-joined) to $CLAUDE_CALLS_FILE and its arg count
// to $CLAUDE_ARGC_FILE (one integer per line — newline-safe, unlike the
// space-joined form when a prompt contains newlines), then emits canned
// stream-json based on $CLAUDE_SCENARIO.
const fakeClaude = `#!/bin/sh
printf '%s\n' "$*" >>"$CLAUDE_CALLS_FILE"
printf '%s\n' "$#" >>"$CLAUDE_ARGC_FILE"

call_count=$(wc -l <"$CLAUDE_ARGC_FILE" 2>/dev/null || echo 0)

scenario="$CLAUDE_SCENARIO"
if [ "$scenario" = "quota-then-success" ]; then
    scenario="quota"
    [ "$call_count" -ge 2 ] && scenario="success"
fi

init_line='{"type":"system","subtype":"init","session_id":"SID-1"}'

case "$scenario" in
    success)
        printf '%s\n' "$init_line"
        printf '%s\n' '{"type":"assistant","message":{"content":[{"type":"text","text":"done"}]}}'
        printf '%s\n' '{"type":"result","subtype":"success","is_error":false,"result":"OK","session_id":"SID-1"}'
        exit 0
        ;;
    auth)
        printf '%s\n' "$init_line"
        printf '%s\n' '{"type":"result","subtype":"success","is_error":true,"result":"Not logged in","session_id":"SID-1"}'
        exit 1
        ;;
    quota)
        printf '%s\n' "$init_line"
        printf '{"type":"rate_limit_event","rate_limit_info":{"status":"rejected","resetsAt":1000000000,"rateLimitType":"five_hour","overageStatus":"rejected","isUsingOverage":false}}\n'
        printf '%s\n' '{"type":"result","subtype":"success","is_error":true,"api_error_status":429,"session_id":"SID-1"}'
        exit 0
        ;;
    quota-future)
        future_epoch=$(( $(date +%s) + 3600 ))
        printf '%s\n' "$init_line"
        printf '{"type":"rate_limit_event","rate_limit_info":{"status":"rejected","resetsAt":%d,"rateLimitType":"seven_day","overageStatus":"rejected","isUsingOverage":false}}\n' "$future_epoch"
        printf '%s\n' '{"type":"result","subtype":"success","is_error":true,"api_error_status":429,"session_id":"SID-1"}'
        exit 0
        ;;
    quota-no-init)
        printf '{"type":"rate_limit_event","rate_limit_info":{"status":"rejected","resetsAt":1000000000,"rateLimitType":"five_hour","overageStatus":"rejected","isUsingOverage":false}}\n'
        printf '%s\n' '{"type":"result","subtype":"success","is_error":true,"api_error_status":429}'
        exit 0
        ;;
    billing)
        printf '%s\n' "$init_line"
        printf '{"type":"rate_limit_event","rate_limit_info":{"status":"rejected","resetsAt":1000000000,"rateLimitType":"five_hour","overageStatus":"rejected","overageDisabledReason":"out_of_credits","isUsingOverage":false}}\n'
        printf '%s\n' '{"type":"result","subtype":"success","is_error":true,"api_error_status":429,"session_id":"SID-1"}'
        exit 0
        ;;
    *)
        printf 'unknown scenario: %s\n' "$scenario" >&2
        exit 2
        ;;
esac
`

type scriptResult struct {
	stdout     string
	stderr     string
	exitCode   int
	callsLines []string
	argc       []string
}

type wrapperOpts struct {
	scenario string
	prompt   string   // defaults to "do work"
	extraEnv []string // appended to the base env (overrides win, last value)
}

func runWrapper(t *testing.T, scenario string) scriptResult {
	t.Helper()
	return runWrapperOpts(t, wrapperOpts{scenario: scenario})
}

func runWrapperOpts(t *testing.T, opts wrapperOpts) scriptResult {
	t.Helper()

	scriptDir := t.TempDir()
	scriptPath := filepath.Join(scriptDir, "claude-retry.sh")
	require.NoError(t, os.WriteFile(scriptPath, retryScriptBytes, 0o600)) // run via `sh`; no exec bit

	binDir := t.TempDir()
	claudePath := filepath.Join(binDir, "claude")
	// fake claude is resolved + executed via PATH, so it needs the exec bit.
	require.NoError(t, os.WriteFile(claudePath, []byte(fakeClaude), 0o755)) //nolint:gosec // test fake must be executable

	recDir := t.TempDir()
	callsFile := filepath.Join(recDir, "calls.txt")
	argcFile := filepath.Join(recDir, "argc.txt")

	newPath := fmt.Sprintf("%s:%s", binDir, os.Getenv("PATH"))
	env := append(os.Environ(),
		"PATH="+newPath,
		"CLAUDE_SCENARIO="+opts.scenario,
		"CLAUDE_CALLS_FILE="+callsFile,
		"CLAUDE_ARGC_FILE="+argcFile,
		"RETRY_RESET_BUFFER_SECS=0",
		"RETRY_BACKOFF_BASE_SECS=1",
		"RETRY_MAX_ATTEMPTS=3",
	)
	env = append(env, opts.extraEnv...)

	prompt := opts.prompt
	if prompt == "" {
		prompt = "do work"
	}

	var stdout, stderr bytes.Buffer
	//nolint:gosec // G204: scriptPath is a test temp path, not external input
	cmd := exec.CommandContext(context.Background(), "sh", scriptPath, "claude",
		"-p", prompt,
		"--model", "sonnet",
		"--output-format", "stream-json",
		"--verbose",
	)
	cmd.Env = env
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("unexpected exec error: %v", err)
		}
		exitCode = ee.ExitCode()
	}

	return scriptResult{
		stdout:     stdout.String(),
		stderr:     stderr.String(),
		exitCode:   exitCode,
		callsLines: readLines(t, callsFile),
		argc:       readLines(t, argcFile),
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // path is a test temp file
	if err != nil {
		return nil
	}
	raw := strings.TrimRight(string(data), "\n")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "\n")
}

func TestWrapperScript_Success(t *testing.T) {
	requireSh(t)
	res := runWrapper(t, "success")

	assert.Equal(t, 0, res.exitCode)
	assert.Contains(t, res.stdout, `"result":"OK"`)
	require.Len(t, res.argc, 1)
	require.Len(t, res.callsLines, 1)
	assert.Contains(t, res.callsLines[0], "-p do work")
	assert.NotContains(t, res.callsLines[0], "--resume")
}

func TestWrapperScript_Auth(t *testing.T) {
	requireSh(t)
	res := runWrapper(t, "auth")

	assert.Equal(t, 1, res.exitCode)
	require.Len(t, res.argc, 1, "auth error should not trigger retry")
}

func TestWrapperScript_QuotaThenSuccess(t *testing.T) {
	requireSh(t)
	res := runWrapper(t, "quota-then-success")

	assert.Equal(t, 0, res.exitCode)
	require.Len(t, res.callsLines, 2)

	secondCall := res.callsLines[1]
	assert.Contains(t, secondCall, "--resume SID-1")
	assert.Contains(t, secondCall, "-p continue")
	assert.NotContains(t, secondCall, "-p do work")

	// stdout is the combined verbatim passthrough of both attempts and must
	// carry none of the wrapper's own diagnostics.
	assert.Contains(t, res.stdout, `"status":"rejected"`)
	assert.Contains(t, res.stdout, `"result":"OK"`)
	assert.NotContains(t, res.stdout, "claude-retry:")
	assert.Contains(t, res.stderr, "claude-retry:")
}

func TestWrapperScript_Billing(t *testing.T) {
	requireSh(t)
	res := runWrapper(t, "billing")

	assert.Equal(t, 0, res.exitCode)
	require.Len(t, res.argc, 1, "billing (out_of_credits) must not trigger retry")
}

func TestWrapperScript_WaitBudgetExceeded(t *testing.T) {
	requireSh(t)
	// A reset 3600s out against a 1s budget can't be outlasted: exit, no sleep.
	res := runWrapperOpts(t, wrapperOpts{
		scenario: "quota-future",
		extraEnv: []string{"RETRY_MAX_TOTAL_WAIT_SECS=1"},
	})

	assert.Equal(t, 0, res.exitCode)
	require.Len(t, res.argc, 1, "reset beyond budget must not retry")
	assert.Contains(t, res.stderr, "beyond the wait budget")
}

func TestWrapperScript_QuotaNoSessionID(t *testing.T) {
	requireSh(t)
	// Quota rejection but no init line means no session_id to resume with.
	res := runWrapper(t, "quota-no-init")

	require.Len(t, res.argc, 1, "missing session_id must not retry")
	assert.Contains(t, res.stderr, "no session_id")
}

func TestWrapperScript_MultiLinePromptNotSplit(t *testing.T) {
	requireSh(t)
	// The first claude call must receive the multi-line prompt as a SINGLE
	// arg (8 args total), proving the argv handling preserves arg boundaries.
	res := runWrapperOpts(t, wrapperOpts{
		scenario: "quota-then-success",
		prompt:   "line one\nline two\nline three",
	})

	assert.Equal(t, 0, res.exitCode)
	require.Len(t, res.argc, 2)
	// $# excludes the program name. First attempt: -p <prompt> --model sonnet
	// --output-format stream-json --verbose = 7 args. A split prompt would
	// show 8, so 7 proves the multi-line prompt stayed a single arg.
	assert.Equal(t, "7", res.argc[0], "first attempt must keep the prompt as one arg")
	// Resume replaces the prompt with the single token `continue` and prepends
	// `--resume <id>`: --resume SID-1 -p continue --model sonnet
	// --output-format stream-json --verbose = 9 args.
	assert.Equal(t, "9", res.argc[1])
	assert.Contains(t, res.stdout, `"result":"OK"`)
}

func requireSh(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh unavailable")
	}
}
