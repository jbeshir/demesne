package codex

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	proxyopenai "github.com/jbeshir/demesne/internal/proxies/openai"
)

const fakeCodex = `#!/bin/sh
printf '%s\n' "$*" >>"$CODEX_CALLS_FILE"
printf '%s\n' "$#" >>"$CODEX_ARGC_FILE"
if [ -n "${CODEX_CONFIG_SNAPSHOT_FILE:-}" ]; then
    cat "$CODEX_HOME/config.toml" >"$CODEX_CONFIG_SNAPSHOT_FILE"
fi

call_count=$(wc -l <"$CODEX_ARGC_FILE" 2>/dev/null || echo 0)

# out writes to stdout and, when asked, to a copy file the tests compare the
# wrapper's stdout against byte for byte.
out() {
    printf "$@"
    if [ -n "${CODEX_STDOUT_COPY_FILE:-}" ]; then
        printf "$@" >>"$CODEX_STDOUT_COPY_FILE"
    fi
}

scenario="$CODEX_SCENARIO"
case "$scenario" in
    *-then-success)
        scenario="${scenario%-then-success}"
        [ "$call_count" -ge 2 ] && scenario="success"
        ;;
esac

thread_line='{"type":"thread.started","thread_id":"THREAD-1"}'
success_line='{"type":"item.completed","item":{"type":"agent_message","text":"done"}}'
complete_line='{"type":"turn.completed","usage":{"input_tokens":1}}'
quota_message="You've hit your usage limit. Try again at 1:00 AM."
no_reset_message="You've hit your usage limit, try again later."
quota_event="{\"type\":\"turn.failed\",\"error\":{\"message\":\"$quota_message\"}}"

case "$scenario" in
    success)
        out '%s\n' "$thread_line"
        out '%s\n' "$success_line"
        out '%s\n' "$complete_line"
        exit 0
        ;;
    quota)
        printf '%s\n' "$thread_line"
        printf '%s\n' '{"type":"turn.started"}'
        printf '{"type":"error","message":"%s"}\n' "$quota_message"
        printf '%s\n' "$quota_event"
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
    noise)
        # Non-JSON lines, including ones that carry the quota markers but are
        # not valid JSON, plus a valid turn.failed without the usage-limit
        # phrase: none of these may count as quota.
        out '%s\n' 'plain text noise'
        out '%s\n' "$thread_line"
        out '%s\n' '{not json'
        out '%s\n' "{\"type\":\"turn.failed\",\"error\":{\"message\":\"$quota_message\""
        out '\n'
        out '%s\n' '{"type":"turn.failed","error":{"message":"some other failure"}}'
        out '%s\n' '[1,2]'
        out '%s\n' "$quota_event"
        exit 1
        ;;
    noise-lookalike)
        # An invalid-JSON line carrying both quota markers is the only
        # candidate: it must not count as quota.
        out '%s\n' "$thread_line"
        out '%s\n' "{\"type\":\"turn.failed\",\"error\":{\"message\":\"$quota_message\""
        exit 1
        ;;
    substring)
        # A valid event whose raw text contains the quota markers at a nested
        # level (a top-level "type" other than turn.failed) is not a quota
        # failure.
        out '%s\n' "$thread_line"
        out '%s\n' '{"type":"item.completed","item":{"type":"turn.failed","text":"You'"'"'ve hit your usage limit. Try again at 1:00 AM."}}'
        exit 1
        ;;
    bytes)
        out '%s\n' "$thread_line"
        out '%s\r\n' '{"type":"item.completed","item":{"type":"agent_message","text":"héllo wörld ✓ 日本語"}}'
        out '%s\n' 'ünïcode noise ✓'
        # The final line has no trailing newline.
        out '%s' "$quota_event"
        exit 1
        ;;
    hang)
        out '%s\n' "$thread_line"
        trap 'printf term >"$CODEX_SIGNAL_FILE"; exit 0' TERM
        : >"$CODEX_READY_FILE"
        while :; do sleep 0.1; done
        ;;
    reset-text)
        out '%s\n' "$thread_line"
        out '{"type":"turn.failed","error":{"message":"You'"'"'ve hit your usage limit. Try again at %s."}}\n' "$CODEX_RESET_TEXT"
        exit 1
        ;;
    *)
        printf 'unknown scenario: %s\n' "$scenario" >&2
        exit 2
        ;;
esac
`

const fakeCurl = `#!/bin/sh
out=
while [ "$#" -gt 0 ]; do
    case "$1" in
        -o)
            shift
            out=$1
            ;;
    esac
    shift
done

case "${CATALOG_SCENARIO:-success}" in
    success)
        printf '{"models":[{"slug":"gpt-test"}]}\n' >"$out"
        printf '200'
        ;;
    http)
        printf 'upstream denied catalog\n' >"$out"
        printf '503'
        ;;
    invalid)
        printf '{"models":[]}\n' >"$out"
        printf '200'
        ;;
    curl-fail)
        printf 'curl transport failed\n' >&2
        exit 7
        ;;
    *)
        printf 'unknown catalog scenario: %s\n' "${CATALOG_SCENARIO:-}" >&2
        exit 2
        ;;
esac
`

const (
	scenarioSuccess = "success"
	envNoBackoff    = "RETRY_BACKOFF_BASE_SECS=0"
)

type codexScriptResult struct {
	stdout     string
	stderr     string
	exitCode   int
	callsLines []string
	argc       []string
	sleeps     []string
	config     string
}

type codexWrapperOpts struct {
	scenario string
	prompt   string
	extraEnv []string
	tz       string // TZ for the wrapper; defaults to a zone where it is about midday
}

// codexRun is a configured but not yet started wrapper process, with the
// files its fakes record into.
type codexRun struct {
	cmd        *exec.Cmd
	stdout     *syncBuffer
	stderr     *syncBuffer
	callsFile  string
	argcFile   string
	configFile string
	copyFile   string
	signalFile string
	readyFile  string
}

// syncBuffer is a bytes.Buffer that can be read while the process under test
// is still writing to it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

var sleepingRe = regexp.MustCompile(`sleeping (\d+)s`)

// pinnedZone returns an offset such that local time in it is about midday,
// as both an Etc/GMT TZ name and a fixed zone. The wrapper resolves "1:00 AM"
// to today at 01:00 local, so pinning keeps that in the past no matter when
// the tests run (otherwise a run just after midnight would really sleep).
// Etc/GMT names have an inverted sign: Etc/GMT-3 is UTC+3.
func pinnedZone() (string, *time.Location) {
	offset := 12 - time.Now().UTC().Hour()
	name := "Etc/GMT"
	switch {
	case offset > 0:
		name = fmt.Sprintf("Etc/GMT-%d", offset)
	case offset < 0:
		name = fmt.Sprintf("Etc/GMT+%d", -offset)
	}
	return name, time.FixedZone(name, offset*3600)
}

func runCodexWrapper(t *testing.T, scenario string) codexScriptResult {
	t.Helper()
	return runCodexWrapperOpts(t, codexWrapperOpts{scenario: scenario})
}

func runCodexWrapperOpts(t *testing.T, opts codexWrapperOpts) codexScriptResult {
	t.Helper()
	run := setupCodexWrapper(t, opts)
	return run.finish(t, run.cmd.Run())
}

// setupCodexWrapper writes the script and fakes into temp dirs and returns the
// wrapper command, ready to Run or Start.
func setupCodexWrapper(t *testing.T, opts codexWrapperOpts) *codexRun {
	t.Helper()

	scriptDir := t.TempDir()
	scriptPath := filepath.Join(scriptDir, "codex-retry.mjs")
	require.NoError(t, os.WriteFile(scriptPath, wrapperScriptBytes, 0o600))

	binDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "codex"), []byte(fakeCodex), 0o755)) //nolint:gosec // test temp dir; fake codex binary must be executable
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "curl"), []byte(fakeCurl), 0o755))   //nolint:gosec // test temp dir; fake curl binary must be executable

	inDir := filepath.Join(t.TempDir(), ".agent")
	require.NoError(t, os.MkdirAll(inDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(inDir, "config.toml"), []byte("model_provider = \"test\"\n"), 0o600))

	recDir := t.TempDir()
	run := &codexRun{
		stdout:     &syncBuffer{},
		stderr:     &syncBuffer{},
		callsFile:  filepath.Join(recDir, "calls.txt"),
		argcFile:   filepath.Join(recDir, "argc.txt"),
		configFile: filepath.Join(recDir, "config.toml"),
		copyFile:   filepath.Join(recDir, "stdout-copy.txt"),
		signalFile: filepath.Join(recDir, "signal.txt"),
		readyFile:  filepath.Join(recDir, "ready.txt"),
	}

	tz := opts.tz
	if tz == "" {
		tz, _ = pinnedZone()
	}
	env := append(os.Environ(),
		"PATH="+binDir+":"+os.Getenv("PATH"),
		"TZ="+tz,
		"CODEX_SCENARIO="+opts.scenario,
		"CODEX_CALLS_FILE="+run.callsFile,
		"CODEX_ARGC_FILE="+run.argcFile,
		"CODEX_CONFIG_SNAPSHOT_FILE="+run.configFile,
		"CODEX_STDOUT_COPY_FILE="+run.copyFile,
		"CODEX_SIGNAL_FILE="+run.signalFile,
		"CODEX_READY_FILE="+run.readyFile,
		"CODEX_CONFIG_PATH="+filepath.Join(inDir, "config.toml"),
		"DEMESNE_OPENAI_AGENT_KEY=agent-token-for-catalog",
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

	//nolint:gosec // G204: scriptPath and args are test-controlled.
	cmd := exec.CommandContext(context.Background(), "node", scriptPath, "gpt-test", prompt)
	cmd.Env = env
	cmd.Stdout = run.stdout
	cmd.Stderr = run.stderr
	cmd.Dir = workDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 5 * time.Second
	run.cmd = cmd
	return run
}

// finish collects the result of a finished wrapper; waitErr is the error from
// Run or Wait.
func (r *codexRun) finish(t *testing.T, waitErr error) codexScriptResult {
	t.Helper()

	exitCode := 0
	if waitErr != nil {
		var ee *exec.ExitError
		if !errors.As(waitErr, &ee) {
			t.Fatalf("unexpected exec error: %v", waitErr)
		}
		exitCode = ee.ExitCode()
	}

	stderr := r.stderr.String()
	var sleeps []string
	for _, m := range sleepingRe.FindAllStringSubmatch(stderr, -1) {
		sleeps = append(sleeps, m[1])
	}

	return codexScriptResult{
		stdout:     r.stdout.String(),
		stderr:     stderr,
		exitCode:   exitCode,
		callsLines: readCodexLines(t, r.callsFile),
		argc:       readCodexLines(t, r.argcFile),
		sleeps:     sleeps,
		config:     readCodexFile(t, r.configFile),
	}
}

// waitFor polls until cond holds or the timeout elapses.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	require.Eventually(t, cond, 10*time.Second, 20*time.Millisecond, what)
}

// start starts the wrapper in its own process group and arranges for the whole
// group, including any fake codex it left behind, to be killed when the test
// ends. It returns a channel that delivers the Wait error once it exits.
func (r *codexRun) start(t *testing.T) <-chan error {
	t.Helper()
	require.NoError(t, r.cmd.Start())
	t.Cleanup(func() { _ = syscall.Kill(-r.cmd.Process.Pid, syscall.SIGKILL) })
	done := make(chan error, 1)
	go func() { done <- r.cmd.Wait() }()
	return done
}

// terminate sends SIGTERM to the wrapper and returns its result, failing if
// it does not exit promptly.
func (r *codexRun) terminate(t *testing.T, done <-chan error) codexScriptResult {
	t.Helper()
	require.NoError(t, r.cmd.Process.Signal(syscall.SIGTERM))
	select {
	case err := <-done:
		return r.finish(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("wrapper did not exit promptly after SIGTERM")
		return codexScriptResult{}
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

func readCodexFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // path is a test temp file
	if err != nil {
		return ""
	}
	return string(data)
}

func TestCodexWrapperScript_Success(t *testing.T) {
	requireTools(t)
	res := runCodexWrapper(t, scenarioSuccess)

	assert.Equal(t, 0, res.exitCode)
	assert.Contains(t, res.stdout, `"text":"done"`)
	require.Len(t, res.callsLines, 1)
	assert.Contains(t, res.callsLines[0], "exec --json -s danger-full-access --skip-git-repo-check -C ")
	assert.Contains(t, res.callsLines[0], "-m gpt-test -- do work")
	assert.NotContains(t, res.callsLines[0], "resume")
	assert.Contains(t, res.config, `model_catalog_json = "`)
	assert.Contains(t, res.config, `/.codex/model-catalog.json"`)
	assert.NotContains(t, res.config, "agent-token-for-catalog")
	assert.Empty(t, res.sleeps)
}

func TestCodexWrapperScript_CatalogHTTPFailureStopsBeforeCodex(t *testing.T) {
	requireTools(t)
	res := runCodexWrapperOpts(t, codexWrapperOpts{
		scenario: scenarioSuccess,
		extraEnv: []string{
			"CATALOG_SCENARIO=http",
		},
	})

	assert.Equal(t, 1, res.exitCode)
	assert.Empty(t, res.callsLines)
	assert.Contains(t, res.stderr, "failed to fetch Codex model catalog from sidecar: HTTP 503")
	assert.Contains(t, res.stderr, "upstream denied catalog")
}

func TestCodexWrapperScript_CatalogInvalidJSONStopsBeforeCodex(t *testing.T) {
	requireTools(t)
	res := runCodexWrapperOpts(t, codexWrapperOpts{
		scenario: scenarioSuccess,
		extraEnv: []string{
			"CATALOG_SCENARIO=invalid",
		},
	})

	assert.Equal(t, 1, res.exitCode)
	assert.Empty(t, res.callsLines)
	assert.Contains(t, res.stderr, "unusable Codex model catalog JSON")
}

func TestCodexWrapperScript_CatalogTokenMissingStopsBeforeCodex(t *testing.T) {
	requireTools(t)
	res := runCodexWrapperOpts(t, codexWrapperOpts{
		scenario: scenarioSuccess,
		extraEnv: []string{
			"DEMESNE_OPENAI_AGENT_KEY=",
		},
	})

	assert.Equal(t, 1, res.exitCode)
	assert.Empty(t, res.callsLines)
	assert.Contains(t, res.stderr, "DEMESNE_OPENAI_AGENT_KEY is required")
}

func TestCodexWrapperScript_QuotaThenSuccess(t *testing.T) {
	requireTools(t)
	res := runCodexWrapperOpts(t, codexWrapperOpts{
		scenario: "quota-then-success",
		extraEnv: []string{envNoBackoff},
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
	requireTools(t)
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
	requireTools(t)
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
	requireTools(t)
	res := runCodexWrapper(t, "no-reset-then-success")

	assert.Equal(t, 0, res.exitCode)
	require.Len(t, res.callsLines, 2)
	assert.Equal(t, []string{"1"}, res.sleeps)
	assert.Contains(t, res.callsLines[1], "exec resume THREAD-1")
}

func TestCodexWrapperScript_WaitBudgetExceeded(t *testing.T) {
	requireTools(t)
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
	requireTools(t)
	res := runCodexWrapper(t, "quota-no-thread")

	assert.Equal(t, 1, res.exitCode)
	require.Len(t, res.callsLines, 1)
	assert.Contains(t, res.stderr, "no thread_id")
	assert.Empty(t, res.sleeps)
}

// requireTools skips when node (runs the wrapper) or sh (runs the fakes) is missing.
func requireTools(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"node", "sh"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " unavailable")
		}
	}
}

// TestCodexWrapperScript_SendsNoClientVersion pins that the catalog fetch
// leaves version, User-Agent and client_version to the sidecar proxy, so
// proxyopenai.CodexVersion is the only place the Codex version is set.
func TestCodexWrapperScript_SendsNoClientVersion(t *testing.T) {
	script := string(wrapperScriptBytes)
	assert.NotContains(t, script, "client_version=")
	assert.NotContains(t, script, `"version:`)
	assert.NotContains(t, script, `"User-Agent:`)
	assert.NotContains(t, script, proxyopenai.CodexVersion)
}

func TestCodexWrapperScript_NoiseAndParsedFieldDetection(t *testing.T) {
	requireTools(t)
	res := runCodexWrapperOpts(t, codexWrapperOpts{
		scenario: "noise-then-success",
		extraEnv: []string{envNoBackoff},
	})

	assert.Equal(t, 0, res.exitCode)
	require.Len(t, res.callsLines, 2)
	assert.Contains(t, res.callsLines[1], "exec resume THREAD-1")
	assert.Contains(t, res.stderr, "reported reset")
	for _, line := range []string{"plain text noise", "{not json", "[1,2]", "some other failure"} {
		assert.Contains(t, res.stdout, line)
	}
	assert.Contains(t, res.stdout, "\n\n", "empty line passed through")
}

// TestCodexWrapperScript_NoQuotaOnInvalidJSON pins that a line which is not
// valid JSON never counts as quota, even when it carries the quota markers.
func TestCodexWrapperScript_NoQuotaOnInvalidJSON(t *testing.T) {
	requireTools(t)
	res := runCodexWrapper(t, "noise-lookalike")

	assert.Equal(t, 1, res.exitCode)
	require.Len(t, res.callsLines, 1)
	assert.NotContains(t, res.stderr, "codex-retry:")
	assert.Empty(t, res.sleeps)
}

// TestCodexWrapperScript_NoQuotaOnSubstring pins that detection uses parsed
// top-level fields, not a text match: an event whose nested item merely
// carries the quota markers must not trigger a retry.
func TestCodexWrapperScript_NoQuotaOnSubstring(t *testing.T) {
	requireTools(t)
	res := runCodexWrapper(t, "substring")

	assert.Equal(t, 1, res.exitCode)
	require.Len(t, res.callsLines, 1)
	assert.NotContains(t, res.stderr, "codex-retry:")
	assert.Empty(t, res.sleeps)
}

func TestCodexWrapperScript_StdoutByteIdenticalAcrossRetry(t *testing.T) {
	requireTools(t)
	run := setupCodexWrapper(t, codexWrapperOpts{
		scenario: "bytes-then-success",
		extraEnv: []string{envNoBackoff},
	})
	res := run.finish(t, run.cmd.Run())

	assert.Equal(t, 0, res.exitCode)
	require.Len(t, res.callsLines, 2)
	assert.Contains(t, res.stderr, "reported reset")
	assert.Equal(t, readCodexFile(t, run.copyFile), res.stdout)
	assert.Contains(t, res.stdout, "日本語")
	assert.Contains(t, res.stdout, "\r\n")
}

func TestCodexWrapperScript_SigtermDuringSleepExitsPromptly(t *testing.T) {
	requireTools(t)
	run := setupCodexWrapper(t, codexWrapperOpts{
		scenario: "no-reset",
		extraEnv: []string{"RETRY_BACKOFF_BASE_SECS=3600"},
	})
	done := run.start(t)

	waitFor(t, "wrapper to start sleeping", func() bool {
		return strings.Contains(run.stderr.String(), "sleeping")
	})
	res := run.terminate(t, done)

	assert.Equal(t, 143, res.exitCode)
	assert.Len(t, res.callsLines, 1)
}

func TestCodexWrapperScript_SigtermForwardedToRunningCodex(t *testing.T) {
	requireTools(t)
	run := setupCodexWrapper(t, codexWrapperOpts{scenario: "hang"})
	done := run.start(t)

	waitFor(t, "fake codex to be ready", func() bool {
		_, err := os.Stat(run.readyFile)
		return err == nil
	})
	res := run.terminate(t, done)

	assert.Equal(t, 0, res.exitCode, "wrapper exits with the child's code")
	assert.Equal(t, "term", readCodexFile(t, run.signalFile))
	assert.Len(t, res.callsLines, 1)
}

// TestCodexWrapperScript_FullDateResetParses covers the dated form of the
// reset text ("Jan 2nd, 2026 3:04 PM"): a time in the past parses to a wait of
// just the buffer, so the wrapper resumes after sleeping 0s.
func TestCodexWrapperScript_FullDateResetParses(t *testing.T) {
	requireTools(t)
	tz, loc := pinnedZone()
	past := time.Now().In(loc).Add(-2 * time.Minute)
	suffix := "th"
	switch past.Day() {
	case 1, 21, 31:
		suffix = "st"
	case 2, 22:
		suffix = "nd"
	case 3, 23:
		suffix = "rd"
	}
	text := fmt.Sprintf("%s %d%s, %d %s", past.Format("Jan"), past.Day(), suffix, past.Year(), past.Format("3:04 PM"))

	res := runCodexWrapperOpts(t, codexWrapperOpts{
		scenario: "reset-text-then-success",
		extraEnv: []string{"CODEX_RESET_TEXT=" + text},
		tz:       tz,
	})

	assert.Equal(t, 0, res.exitCode)
	require.Len(t, res.callsLines, 2)
	assert.Equal(t, []string{"0"}, res.sleeps)
	assert.Contains(t, res.stderr, "reported reset")
}
