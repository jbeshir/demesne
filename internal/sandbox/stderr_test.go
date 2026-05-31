package sandbox

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTailStderrSmall(t *testing.T) {
	assert.Empty(t, tailStderr(nil))
	assert.Empty(t, tailStderr([]byte{}))
	small := []byte("hello stderr")
	assert.Equal(t, "hello stderr", tailStderr(small))
	assert.NotContains(t, tailStderr(small), stderrTruncationMarker)
}

func TestTailStderrExactlyLimit(t *testing.T) {
	// Exactly 16384 bytes — no truncation, no marker.
	b := []byte(strings.Repeat("a", stderrResultLimit))
	result := tailStderr(b)
	assert.Equal(t, string(b), result)
	assert.NotContains(t, result, stderrTruncationMarker)
}

func TestTailStderrLarge(t *testing.T) {
	b := []byte(strings.Repeat("x", 20000))
	result := tailStderr(b)
	require.True(t, strings.HasPrefix(result, stderrTruncationMarker),
		"result must start with truncation marker")
	tail := result[len(stderrTruncationMarker):]
	assert.Len(t, tail, stderrResultLimit,
		"tail after marker must be exactly stderrResultLimit bytes")
	assert.Equal(t, string(b[len(b)-stderrResultLimit:]), tail,
		"tail content must match last stderrResultLimit bytes of input")
}

func TestTailStderrUTF8(t *testing.T) {
	// Build a slice where the raw byte boundary (len-stderrResultLimit) lands
	// on a UTF-8 continuation byte inside '中' (U+4E2D = E4 B8 AD, 3 bytes).
	//
	// Layout: prefixLen('a') + '中'(3 bytes) + tailLen('b')
	// boundary = len(data) - stderrResultLimit
	//          = prefixLen + 3 + tailLen - stderrResultLimit
	// We want boundary == prefixLen+1 (pointing at 0xB8, the continuation byte),
	// so tailLen = stderrResultLimit - 2 = 16382.
	const prefixLen = 10
	const tailLen = stderrResultLimit - 2
	cjk := []byte("\xe4\xb8\xad") // '中': E4 B8 AD
	data := make([]byte, 0, prefixLen+len(cjk)+tailLen)
	data = append(data, bytes.Repeat([]byte{'a'}, prefixLen)...)
	data = append(data, cjk...)
	data = append(data, bytes.Repeat([]byte{'b'}, tailLen)...)

	boundary := len(data) - stderrResultLimit
	require.Equal(t, prefixLen+1, boundary, "test setup: boundary must be at index of continuation byte")
	require.Equal(t, byte(0xB8), data[boundary], "boundary must land on UTF-8 continuation byte 0xB8")

	result := tailStderr(data)
	require.True(t, strings.HasPrefix(result, stderrTruncationMarker),
		"large input must be truncated")
	tail := result[len(stderrTruncationMarker):]

	assert.True(t, utf8.ValidString(tail), "surfaced tail must be valid UTF-8")
	assert.LessOrEqual(t, len(tail), stderrResultLimit, "tail must not exceed limit")
	assert.GreaterOrEqual(t, len(tail), stderrResultLimit-3,
		"offset must advance at most 3 bytes past boundary")
}

func TestCombineMessagesEmpty(t *testing.T) {
	assert.Empty(t, combineMessages(nil))
	assert.Empty(t, combineMessages([]opensandbox.OutputMessage{}))
}

func TestCombineMessagesJoin(t *testing.T) {
	msgs := []opensandbox.OutputMessage{
		{Text: "a"},
		{Text: "b"},
		{Text: "c"},
	}
	assert.Equal(t, "a\nb\nc", combineMessages(msgs))
}

func TestWrapStdoutStderr(t *testing.T) {
	got := wrapStdoutStderr("echo hi")
	assert.Equal(t, "( echo hi ) > /out/stdout.log 2> /out/stderr.log", got)
}

func TestWrapStdoutStderrAmpersand(t *testing.T) {
	// Commands with &&, ||, and quotes must be embedded literally, not re-quoted.
	cmd := `go test && echo "done" || echo 'fail'`
	got := wrapStdoutStderr(cmd)
	assert.Equal(t, "( "+cmd+" ) > /out/stdout.log 2> /out/stderr.log", got)
	assert.Contains(t, got, "&&")
	assert.Contains(t, got, "||")
	assert.Contains(t, got, `"done"`)
}
