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
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClaude is a /bin/sh script emitted into a temp bin dir for each test.
// It records its argv (space-joined) to $CLAUDE_CALLS_FILE and its arg count
// to $CLAUDE_ARGC_FILE (one integer per line — newline-safe, unlike the
// space-joined form when a prompt contains newlines), then emits canned
// stream-json based on $CLAUDE_SCENARIO. A "-then-success" suffix makes every
// call after the first behave as the success scenario. Scenarios that use
// emit also append their stdout to $CLAUDE_STDOUT_COPY_FILE.
const fakeClaude = `#!/bin/sh
printf '%s\n' "$*" >>"$CLAUDE_CALLS_FILE"
printf '%s\n' "$#" >>"$CLAUDE_ARGC_FILE"

call_count=$(wc -l <"$CLAUDE_ARGC_FILE" 2>/dev/null || echo 0)

emit() {
    printf "$@"
    printf "$@" >>"$CLAUDE_STDOUT_COPY_FILE"
}

scenario="$CLAUDE_SCENARIO"
case "$scenario" in
    *-then-success)
        scenario="${scenario%-then-success}"
        [ "$call_count" -ge 2 ] && scenario="success"
        ;;
esac

init_line='{"type":"system","subtype":"init","session_id":"SID-1"}'
now=$(date +%s)
past_epoch=$((now - 60))
far_epoch=$((now + 26 * 3600))
result_429='{"type":"result","subtype":"success","is_error":true,"api_error_status":429,"api_error":"usage_limit_reached","session_id":"SID-1"}'

case "$scenario" in
    success)
        emit '%s\n' "$init_line"
        emit '%s\n' '{"type":"assistant","message":{"content":[{"type":"text","text":"done"}]}}'
        emit '%s\n' '{"type":"result","subtype":"success","is_error":false,"result":"OK","session_id":"SID-1"}'
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
    quota-unified)
        # The rejected event exactly as the real CLI emits it: a five_hour
        # rejection whose line also lists the seven_day window, which resets
        # a day later.
        printf '%s\n' "$init_line"
        printf '{"type":"rate_limit_event","rate_limit_info":{"status":"rejected","resetsAt":%d,"rateLimitType":"five_hour","overageStatus":"rejected","overageDisabledReason":"org_level_disabled","isUsingOverage":false,"unifiedWindows":{"five_hour":{"status":"rejected","resetsAt":%d},"seven_day":{"status":"allowed","resetsAt":%d}}},"uuid":"UUID-1","session_id":"SID-1"}\n' \
            "$past_epoch" "$past_epoch" "$far_epoch"
        printf '%s\n' "$result_429"
        exit 0
        ;;
    quota-unified-fallback)
        # No top-level resetsAt: the rejected window's own entry in
        # unifiedWindows must be used, not the later seven_day one.
        printf '%s\n' "$init_line"
        printf '{"type":"rate_limit_event","rate_limit_info":{"status":"rejected","rateLimitType":"five_hour","overageStatus":"rejected","isUsingOverage":false,"unifiedWindows":{"five_hour":{"status":"rejected","resetsAt":%d},"seven_day":{"status":"allowed","resetsAt":%d}}},"uuid":"UUID-1","session_id":"SID-1"}\n' \
            "$past_epoch" "$far_epoch"
        printf '%s\n' "$result_429"
        exit 0
        ;;
    quota-noise)
        # Non-JSON and non-object lines around the real events; they must be
        # passed through but never confuse detection.
        printf '%s\n' 'plain text noise'
        printf '%s\n' '{not json'
        printf '\n'
        printf '%s\n' "$init_line"
        printf '%s\n' '[1,2]'
        printf '%s\n' 'noise "status":"rejected" "api_error_status":429'
        printf '{"type":"rate_limit_event","rate_limit_info":{"status":"rejected","resetsAt":1000000000,"rateLimitType":"five_hour","overageStatus":"rejected","isUsingOverage":false}}\n'
        printf '%s\n' '{"unterminated": '
        printf '%s\n' "$result_429"
        exit 0
        ;;
    quota-bytes)
        # Multibyte UTF-8 and CRLF in the stream, and a final result line with
        # no trailing newline that only the end-of-stream flush can detect.
        emit '%s\n' "$init_line"
        emit '%s\r\n' '{"type":"assistant","message":{"content":[{"type":"text","text":"héllo → 日本語 🎌"}]}}'
        emit '%s\n' '{"type":"rate_limit_event","rate_limit_info":{"status":"rejected","resetsAt":1000000000,"rateLimitType":"five_hour"}}'
        emit '%s' "$result_429"
        exit 0
        ;;
    quota-sleep)
        # Reset an hour out: the wrapper sleeps until signalled.
        printf '%s\n' "$init_line"
        printf '{"type":"rate_limit_event","rate_limit_info":{"status":"rejected","resetsAt":%d,"rateLimitType":"five_hour","isUsingOverage":false}}\n' "$((now + 3600))"
        printf '%s\n' "$result_429"
        exit 0
        ;;
    hang)
        # Runs until it gets SIGTERM, recording that it did.
        printf '%s\n' "$init_line"
        trap 'printf term >"$CLAUDE_SIGNAL_FILE"; exit 0' TERM
        : >"$CLAUDE_READY_FILE"
        while :; do sleep 0.1; done
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

// wrapperRun is a configured but not yet started wrapper process plus the
// files the fake claude and the harness use to talk to each other.
type wrapperRun struct {
	cmd        *exec.Cmd
	stdout     *bytes.Buffer
	stderrFile string // the wrapper's stderr, readable while it runs
	callsFile  string
	argcFile   string
	copyFile   string // everything the fake claude wrote to stdout
	signalFile string // written by the fake claude's TERM trap
	readyFile  string // created by the hang scenario once its trap is set
}

func runWrapper(t *testing.T, scenario string) scriptResult {
	t.Helper()
	return runWrapperOpts(t, wrapperOpts{scenario: scenario})
}

func runWrapperOpts(t *testing.T, opts wrapperOpts) scriptResult {
	t.Helper()

	run := newWrapperRun(t, opts)
	err := run.cmd.Run()
	return run.result(t, err)
}

// newWrapperRun writes the wrapper and the fake claude into temp dirs and
// returns the command, unstarted, wired to run them.
func newWrapperRun(t *testing.T, opts wrapperOpts) *wrapperRun {
	t.Helper()

	scriptDir := t.TempDir()
	scriptPath := filepath.Join(scriptDir, "claude-retry.mjs")
	require.NoError(t, os.WriteFile(scriptPath, retryScriptBytes, 0o600)) // run via `node`; no exec bit

	binDir := t.TempDir()
	claudePath := filepath.Join(binDir, "claude")
	// fake claude is resolved + executed via PATH, so it needs the exec bit.
	require.NoError(t, os.WriteFile(claudePath, []byte(fakeClaude), 0o755)) //nolint:gosec // test fake must be executable

	recDir := t.TempDir()
	run := &wrapperRun{
		stdout:     &bytes.Buffer{},
		stderrFile: filepath.Join(recDir, "stderr.txt"),
		callsFile:  filepath.Join(recDir, "calls.txt"),
		argcFile:   filepath.Join(recDir, "argc.txt"),
		copyFile:   filepath.Join(recDir, "stdout-copy.txt"),
		signalFile: filepath.Join(recDir, "signal.txt"),
		readyFile:  filepath.Join(recDir, "ready.txt"),
	}

	newPath := fmt.Sprintf("%s:%s", binDir, os.Getenv("PATH"))
	env := append(os.Environ(),
		"PATH="+newPath,
		"CLAUDE_SCENARIO="+opts.scenario,
		"CLAUDE_CALLS_FILE="+run.callsFile,
		"CLAUDE_ARGC_FILE="+run.argcFile,
		"CLAUDE_STDOUT_COPY_FILE="+run.copyFile,
		"CLAUDE_SIGNAL_FILE="+run.signalFile,
		"CLAUDE_READY_FILE="+run.readyFile,
		"RETRY_RESET_BUFFER_SECS=0",
		"RETRY_BACKOFF_BASE_SECS=1",
		"RETRY_MAX_ATTEMPTS=3",
	)
	env = append(env, opts.extraEnv...)

	prompt := opts.prompt
	if prompt == "" {
		prompt = "do work"
	}

	stderr, err := os.Create(run.stderrFile)
	require.NoError(t, err)
	t.Cleanup(func() { _ = stderr.Close() })

	//nolint:gosec // G204: scriptPath is a test temp path, not external input
	run.cmd = exec.CommandContext(context.Background(), "node", scriptPath, "claude",
		"-p", prompt,
		"--model", "sonnet",
		"--output-format", "stream-json",
		"--verbose",
	)
	run.cmd.Env = env
	run.cmd.Stdout = run.stdout
	run.cmd.Stderr = stderr
	run.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	run.cmd.WaitDelay = 5 * time.Second
	return run
}

// result collects the outputs of a finished run; waitErr is the error from
// Run or Wait.
func (r *wrapperRun) result(t *testing.T, waitErr error) scriptResult {
	t.Helper()

	exitCode := 0
	if waitErr != nil {
		var ee *exec.ExitError
		if !errors.As(waitErr, &ee) {
			t.Fatalf("unexpected exec error: %v", waitErr)
		}
		exitCode = ee.ExitCode()
	}

	return scriptResult{
		stdout:     r.stdout.String(),
		stderr:     r.stderr(t),
		exitCode:   exitCode,
		callsLines: readLines(t, r.callsFile),
		argc:       readLines(t, r.argcFile),
	}
}

func (r *wrapperRun) stderr(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(r.stderrFile)
	require.NoError(t, err)
	return string(data)
}

// stderrContains reports whether the wrapper's stderr so far contains want.
// It is safe to call from an Eventually condition: it never fails the test.
func (r *wrapperRun) stderrContains(want string) func() bool {
	return func() bool {
		data, err := os.ReadFile(r.stderrFile)
		return err == nil && strings.Contains(string(data), want)
	}
}

// startAndWaitFor starts the wrapper in its own process group and blocks until
// cond holds, returning a channel that delivers the wrapper's Wait error when
// it exits. The whole group, including any fake claude left behind, is killed
// when the test ends.
func (r *wrapperRun) startAndWaitFor(t *testing.T, what string, cond func() bool) <-chan error {
	t.Helper()

	require.NoError(t, r.cmd.Start())
	t.Cleanup(func() { _ = syscall.Kill(-r.cmd.Process.Pid, syscall.SIGKILL) })
	done := make(chan error, 1)
	go func() { done <- r.cmd.Wait() }()

	require.Eventually(t, cond, 10*time.Second, 20*time.Millisecond, "waiting for %s", what)
	return done
}

// terminate sends SIGTERM to the wrapper and returns its result, failing if
// it does not exit promptly.
func (r *wrapperRun) terminate(t *testing.T, done <-chan error) scriptResult {
	t.Helper()

	require.NoError(t, r.cmd.Process.Signal(syscall.SIGTERM))
	select {
	case err := <-done:
		return r.result(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("wrapper did not exit promptly after SIGTERM")
		return scriptResult{}
	}
}

func fileExists(path string) func() bool {
	return func() bool {
		_, err := os.Stat(path)
		return err == nil
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
	requireNode(t)
	res := runWrapper(t, "success")

	assert.Equal(t, 0, res.exitCode)
	assert.Contains(t, res.stdout, `"result":"OK"`)
	require.Len(t, res.argc, 1)
	require.Len(t, res.callsLines, 1)
	assert.Contains(t, res.callsLines[0], "-p do work")
	assert.NotContains(t, res.callsLines[0], "--resume")
}

func TestWrapperScript_Auth(t *testing.T) {
	requireNode(t)
	res := runWrapper(t, "auth")

	assert.Equal(t, 1, res.exitCode)
	require.Len(t, res.argc, 1, "auth error should not trigger retry")
}

func TestWrapperScript_QuotaThenSuccess(t *testing.T) {
	requireNode(t)
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

// A five_hour rejection's line also carries the seven_day window's resetsAt,
// a day later. Taking the last resetsAt on the line made the wrapper see a
// reset far beyond the default budget and give up; the rejected window's own
// reset (already past here) must be used instead.
func TestWrapperScript_UnifiedWindowsUsesRejectedWindowReset(t *testing.T) {
	requireNode(t)
	res := runWrapper(t, "quota-unified-then-success")

	assert.Equal(t, 0, res.exitCode)
	require.Len(t, res.callsLines, 2, "five_hour exhaustion must be retried")
	assert.Contains(t, res.callsLines[1], "--resume SID-1")
	assert.Contains(t, res.callsLines[1], "-p continue")
	assert.NotContains(t, res.stderr, "beyond the wait budget")
	assert.Contains(t, res.stderr, "sleeping 0s until reset")
}

func TestWrapperScript_UnifiedWindowsFallbackWithoutTopLevelReset(t *testing.T) {
	requireNode(t)
	res := runWrapper(t, "quota-unified-fallback-then-success")

	assert.Equal(t, 0, res.exitCode)
	require.Len(t, res.callsLines, 2, "five_hour exhaustion must be retried")
	assert.Contains(t, res.callsLines[1], "--resume SID-1")
	assert.NotContains(t, res.stderr, "beyond the wait budget")
	assert.Contains(t, res.stderr, "sleeping 0s until reset")
}

func TestWrapperScript_NonJSONNoiseIgnoredAndPassedThrough(t *testing.T) {
	requireNode(t)
	res := runWrapper(t, "quota-noise-then-success")

	assert.Equal(t, 0, res.exitCode)
	require.Len(t, res.callsLines, 2, "quota must still be detected amid noise")
	assert.Contains(t, res.stdout, "plain text noise\n{not json\n\n")
	assert.Contains(t, res.stdout, "\n[1,2]\nnoise \"status\":\"rejected\" \"api_error_status\":429\n")
	assert.Contains(t, res.stdout, "{\"unterminated\": \n")
	assert.Contains(t, res.stdout, `"result":"OK"`)
}

func TestWrapperScript_StdoutByteIdenticalAcrossRetry(t *testing.T) {
	requireNode(t)
	run := newWrapperRun(t, wrapperOpts{scenario: "quota-bytes-then-success"})
	res := run.result(t, run.cmd.Run())

	assert.Equal(t, 0, res.exitCode)
	require.Len(t, res.argc, 2, "429 on an unterminated final line must be detected")

	want, err := os.ReadFile(run.copyFile)
	require.NoError(t, err)
	assert.Equal(t, string(want), res.stdout)
	assert.Contains(t, res.stdout, "héllo → 日本語 🎌")
	assert.Contains(t, res.stdout, "\r\n")
}

func TestWrapperScript_SigtermDuringSleepExitsPromptly(t *testing.T) {
	requireNode(t)
	run := newWrapperRun(t, wrapperOpts{scenario: "quota-sleep"})
	done := run.startAndWaitFor(t, "the wrapper to start sleeping", run.stderrContains("sleeping"))

	res := run.terminate(t, done)

	assert.Equal(t, 143, res.exitCode)
	assert.Len(t, res.argc, 1, "no retry after SIGTERM")
}

func TestWrapperScript_SigtermForwardedToRunningChild(t *testing.T) {
	requireNode(t)
	run := newWrapperRun(t, wrapperOpts{scenario: "hang"})
	done := run.startAndWaitFor(t, "claude to be ready", fileExists(run.readyFile))

	res := run.terminate(t, done)

	signal, err := os.ReadFile(run.signalFile)
	require.NoError(t, err, "claude should have received SIGTERM")
	assert.Equal(t, "term", string(signal))
	assert.Equal(t, 0, res.exitCode, "wrapper exits with the child's code")
	assert.Len(t, res.argc, 1)
}

func TestWrapperScript_Billing(t *testing.T) {
	requireNode(t)
	res := runWrapper(t, "billing")

	assert.Equal(t, 0, res.exitCode)
	require.Len(t, res.argc, 1, "billing (out_of_credits) must not trigger retry")
	assert.Contains(t, res.stderr, "billing limit (out_of_credits), no retry")
}

func TestWrapperScript_WaitBudgetExceeded(t *testing.T) {
	requireNode(t)
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
	requireNode(t)
	// Quota rejection but no init line means no session_id to resume with.
	res := runWrapper(t, "quota-no-init")

	require.Len(t, res.argc, 1, "missing session_id must not retry")
	assert.Contains(t, res.stderr, "no session_id")
}

func TestWrapperScript_MultiLinePromptNotSplit(t *testing.T) {
	requireNode(t)
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

// requireNode skips when node (which runs the wrapper) or sh (which runs the
// fake claude) is unavailable.
func requireNode(t *testing.T) {
	t.Helper()
	requireSh(t)
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node unavailable")
	}
}

func requireSh(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh unavailable")
	}
}
