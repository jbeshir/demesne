package sandbox

import (
	"fmt"
	"strings"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
)

// stderrResultLimit is the maximum number of bytes of stderr surfaced in
// the MCP result. The on-disk /out/stderr.log file is always the complete
// stream; this cap exists because the result field is fed into a calling
// model's context window. 16 KiB covers ~400 typical 40-char lines —
// enough for compile errors, panics, and test failures, where signal
// clusters at the tail.
const stderrResultLimit = 16 * 1024

// stderrTruncationMarker is prefixed to the surfaced stderr when the
// underlying stream exceeded stderrResultLimit. Single-line, bracket-
// delimited (machine-parseable), names the full-file location. Derived
// from stderrResultLimit so the byte count can't drift from the cap.
var stderrTruncationMarker = fmt.Sprintf(
	"[stderr truncated to last %d bytes; full log in output_dir/stderr.log]\n", stderrResultLimit)

// tailStderr returns the last stderrResultLimit bytes of b, prefixed by
// stderrTruncationMarker when truncation occurred. Short inputs pass
// through unchanged. To avoid surfacing a partial UTF-8 codepoint at the
// cut, the start offset is advanced past any UTF-8 continuation bytes
// (those with the top two bits == 10).
func tailStderr(b []byte) string {
	if len(b) <= stderrResultLimit {
		return string(b)
	}
	off := len(b) - stderrResultLimit
	for off < len(b) && b[off]&0xC0 == 0x80 {
		off++
	}
	return stderrTruncationMarker + string(b[off:])
}

// combineMessages flattens a slice of OutputMessage into a single string,
// joining the messages' Text fields with a single newline between each
// pair. Matches opensandbox.Execution.Text()'s format — used so the exec
// path's Stdout field keeps its historical format while exec.Text() is
// dropped in favour of accessing exec.Stdout / exec.Stderr directly.
func combineMessages(msgs []opensandbox.OutputMessage) string {
	var b strings.Builder
	for i, m := range msgs {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(m.Text)
	}
	return b.String()
}

// wrapStdoutStderr wraps a user shell command so its stdout streams to
// /out/stdout.log and stderr to /out/stderr.log inside the sandbox while
// preserving the user command's exit code. The subshell idiom is
// POSIX-portable (dash, busybox, bash) and propagates the exit status of
// the user command unchanged. The wrapper paths are fixed at /out (the
// sandbox's host-bind-mounted output dir) so the host can read them back
// after the SDK call returns.
func wrapStdoutStderr(userCmd string) string {
	return "( " + userCmd + " ) > /out/stdout.log 2> /out/stderr.log"
}
