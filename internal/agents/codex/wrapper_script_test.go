package codex

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

const fakeCodex = `#!/bin/sh
printf '%s\n' "$*" >>"$CODEX_CALLS_FILE"
printf '%s\n' "$#" >>"$CODEX_ARGC_FILE"

call_count=$(wc -l <"$CODEX_ARGC_FILE" 2>/dev/null || echo 0)

scenario="$CODEX_SCENARIO"
if [ "$scenario" = "quota-then-success" ] || [ "$scenario" = "no-reset-then-success" ] || [ "$scenario" = "upsell-reset-then-success" ]; then
    case "$CODEX_SCENARIO" in
        no-reset-then-success) scenario="no-reset" ;;
        upsell-reset-then-success) scenario="upsell-reset" ;;
        *) scenario="quota" ;;
    esac
    [ "$call_count" -ge 2 ] && scenario="success"
fi

thread_line='{"type":"thread.started","thread_id":"THREAD-1"}'
success_line='{"type":"item.completed","item":{"type":"agent_message","text":"done"}}'
complete_line='{"type":"turn.completed","usage":{"input_tokens":1}}'
quota_message="You've hit your usage limit. Try again at 1:00 AM."
no_reset_message="You've hit your usage limit, try again later."

case "$scenario" in
    success)
        printf '%s\n' "$thread_line"
        printf '%s\n' "$success_line"
        printf '%s\n' "$complete_line"
        exit 0
        ;;
    quota)
        printf '%s\n' "$thread_line"
        printf '%s\n' '{"type":"turn.started"}'
        printf '{"type":"error","message":"%s"}\n' "$quota_message"
        printf '{"type":"turn.failed","error":{"message":"%s"}}\n' "$quota_message"
        exit 1
        ;;
    no-reset)
        printf '%s\n' "$thread_line"
        printf '{"type":"error","message":"%s"}\n' "$no_reset_message"
        printf '{"type":"turn.failed","error":{"message":"%s"}}\n' "$no_reset_message"
        exit 1
        ;;
    billing)
        msg="You've hit your usage limit: out of credits. Please purchase more credits."
        printf '%s\n' "$thread_line"
        printf '{"type":"turn.failed","error":{"message":"%s"}}\n' "$msg"
        exit 1
        ;;
    upsell-reset)
        msg="You've hit your usage limit. Upgrade to Pro (https://chatgpt.com/explore/pro), visit https://chatgpt.com/codex/settings/usage to purchase more credits or try again at 1:00 AM."
        printf '%s\n' "$thread_line"
        printf '{"type":"turn.failed","error":{"message":"%s"}}\n' "$msg"
        exit 1
        ;;
    quota-future)
        printf '%s\n' "$thread_line"
        printf '{"type":"turn.failed","error":{"message":"You'"'"'ve hit your usage limit. Try again at Jan 1st, 2999 1:00 AM."}}\n'
        exit 1
        ;;
    quota-no-thread)
        msg="You've hit your usage limit. Try again at 1:00 AM."
        printf '{"type":"turn.failed","error":{"message":"%s"}}\n' "$msg"
        exit 1
        ;;
    *)
        printf 'unknown scenario: %s\n' "$scenario" >&2
        exit 2
        ;;
esac
`

const fakeSleep = `#!/bin/sh
printf '%s\n' "$*" >>"$SLEEP_CALLS_FILE"
exit 0
`

type codexScriptResult struct {
	stdout     string
	stderr     string
	exitCode   int
	callsLines []string
	argc       []string
	sleeps     []string
}

type codexWrapperOpts struct {
	scenario string
	prompt   string
	extraEnv []string
}

func runCodexWrapper(t *testing.T, scenario string) codexScriptResult {
	t.Helper()
	return runCodexWrapperOpts(t, codexWrapperOpts{scenario: scenario})
}

func runCodexWrapperOpts(t *testing.T, opts codexWrapperOpts) codexScriptResult {
	t.Helper()

	scriptDir := t.TempDir()
	scriptPath := filepath.Join(scriptDir, "codex-retry.sh")
	require.NoError(t, os.WriteFile(scriptPath, wrapperScriptBytes, 0o600))

	binDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "codex"), []byte(fakeCodex), 0o755)) //nolint:gosec // test temp dir; fake codex binary must be executable
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "sleep"), []byte(fakeSleep), 0o755)) //nolint:gosec // test temp dir; fake sleep binary must be executable

	inDir := filepath.Join(t.TempDir(), ".agent")
	require.NoError(t, os.MkdirAll(inDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(inDir, "config.toml"), []byte("model_provider = \"test\"\n"), 0o600))

	recDir := t.TempDir()
	callsFile := filepath.Join(recDir, "calls.txt")
	argcFile := filepath.Join(recDir, "argc.txt")
	sleepFile := filepath.Join(recDir, "sleeps.txt")

	newPath := fmt.Sprintf("%s:%s", binDir, os.Getenv("PATH"))
	env := append(os.Environ(),
		"PATH="+newPath,
		"CODEX_SCENARIO="+opts.scenario,
		"CODEX_CALLS_FILE="+callsFile,
		"CODEX_ARGC_FILE="+argcFile,
		"CODEX_CONFIG_PATH="+filepath.Join(inDir, "config.toml"),
		"SLEEP_CALLS_FILE="+sleepFile,
		"RETRY_RESET_BUFFER_SECS=0",
		"RETRY_BACKOFF_BASE_SECS=1",
		"RETRY_MAX_ATTEMPTS=3",
	)
	env = append(env, opts.extraEnv...)

	prompt := opts.prompt
	if prompt == "" {
		prompt = "do work"
	}

	workDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workDir, ".codex"), 0o700))

	var stdout, stderr bytes.Buffer
	//nolint:gosec // G204: scriptPath and args are test-controlled.
	cmd := exec.CommandContext(context.Background(), "sh", scriptPath, "gpt-test", prompt)
	cmd.Env = env
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = workDir

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("unexpected exec error: %v", err)
		}
		exitCode = ee.ExitCode()
	}

	return codexScriptResult{
		stdout:     stdout.String(),
		stderr:     stderr.String(),
		exitCode:   exitCode,
		callsLines: readCodexLines(t, callsFile),
		argc:       readCodexLines(t, argcFile),
		sleeps:     readCodexLines(t, sleepFile),
	}
}

func readCodexLines(t *testing.T, path string) []string {
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

func TestCodexWrapperScript_Success(t *testing.T) {
	requireSh(t)
	res := runCodexWrapper(t, "success")

	assert.Equal(t, 0, res.exitCode)
	assert.Contains(t, res.stdout, `"text":"done"`)
	require.Len(t, res.callsLines, 1)
	assert.Contains(t, res.callsLines[0], "exec --json -s danger-full-access --skip-git-repo-check -C ")
	assert.Contains(t, res.callsLines[0], "-m gpt-test -- do work")
	assert.NotContains(t, res.callsLines[0], "resume")
	assert.Empty(t, res.sleeps)
}

func TestCodexWrapperScript_QuotaThenSuccess(t *testing.T) {
	requireSh(t)
	res := runCodexWrapperOpts(t, codexWrapperOpts{
		scenario: "quota-then-success",
		extraEnv: []string{"RETRY_BACKOFF_BASE_SECS=0"},
	})

	assert.Equal(t, 0, res.exitCode)
	require.Len(t, res.callsLines, 2)
	assert.Contains(t, res.callsLines[1], "exec resume THREAD-1 --json -m gpt-test --skip-git-repo-check continue")
	assert.NotContains(t, res.callsLines[1], "-s danger-full-access")
	assert.NotContains(t, res.callsLines[1], " -C ")
	assert.Contains(t, res.stdout, `"turn.failed"`)
	assert.Contains(t, res.stdout, `"text":"done"`)
	assert.NotContains(t, res.stdout, "codex-retry:")
	assert.Contains(t, res.stderr, "codex-retry:")
}

func TestCodexWrapperScript_OutOfCreditsNoResetBacksOff(t *testing.T) {
	requireSh(t)
	// "out of credits" with no "try again at" reset time: rather than giving
	// up ("no retry"), the wrapper backs off exponentially and resumes the
	// thread, until the attempt budget is exhausted.
	res := runCodexWrapper(t, "billing")

	assert.Equal(t, 1, res.exitCode)
	require.Len(t, res.callsLines, 3) // exec + 2 resumes (RETRY_MAX_ATTEMPTS=3)
	assert.Contains(t, res.callsLines[1], "exec resume THREAD-1")
	assert.Equal(t, []string{"1", "2"}, res.sleeps) // backoff base 1s, doubling
	assert.Contains(t, res.stderr, "max attempts")
	assert.NotContains(t, res.stderr, "no retry")
}

// TestCodexWrapperScript_UpsellWithResetRetries is the regression test for the
// real ChatGPT usage-limit message, which carries "Upgrade to Pro" /
// "purchase more credits" upsell text AND a "try again at <time>" reset. The
// wrapper must honour the reset and resume, not treat the upsell as a fatal
// billing error.
func TestCodexWrapperScript_UpsellWithResetRetries(t *testing.T) {
	requireSh(t)
	res := runCodexWrapperOpts(t, codexWrapperOpts{scenario: "upsell-reset-then-success"})

	assert.Equal(t, 0, res.exitCode)
	require.Len(t, res.callsLines, 2)
	assert.Contains(t, res.callsLines[1], "exec resume THREAD-1")
	require.Len(t, res.sleeps, 1)
	assert.Contains(t, res.stderr, "reported reset")
	assert.Contains(t, res.stdout, `"text":"done"`)
	assert.NotContains(t, res.stderr, "no retry")
}

func TestCodexWrapperScript_NoResetTimeFallsBackToBackoff(t *testing.T) {
	requireSh(t)
	res := runCodexWrapper(t, "no-reset-then-success")

	assert.Equal(t, 0, res.exitCode)
	require.Len(t, res.callsLines, 2)
	assert.Equal(t, []string{"1"}, res.sleeps)
	assert.Contains(t, res.callsLines[1], "exec resume THREAD-1")
}

func TestCodexWrapperScript_WaitBudgetExceeded(t *testing.T) {
	requireSh(t)
	res := runCodexWrapperOpts(t, codexWrapperOpts{
		scenario: "quota-future",
		extraEnv: []string{"RETRY_MAX_TOTAL_WAIT_SECS=1"},
	})

	assert.Equal(t, 1, res.exitCode)
	require.Len(t, res.callsLines, 1)
	assert.Contains(t, res.stderr, "beyond the wait budget")
	assert.Empty(t, res.sleeps)
}

func TestCodexWrapperScript_QuotaNoThreadID(t *testing.T) {
	requireSh(t)
	res := runCodexWrapper(t, "quota-no-thread")

	assert.Equal(t, 1, res.exitCode)
	require.Len(t, res.callsLines, 1)
	assert.Contains(t, res.stderr, "no thread_id")
	assert.Empty(t, res.sleeps)
}

func requireSh(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh unavailable")
	}
}
