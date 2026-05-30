package anthropic

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTracker_AddAccumulates(t *testing.T) {
	tr := NewTracker("")
	tr.Add("claude-sonnet-4-6", TokenCounts{InputTokens: 100, OutputTokens: 200})
	tr.Add("claude-sonnet-4-6", TokenCounts{InputTokens: 50, OutputTokens: 25})
	snap := tr.snapshot()
	model := snap.PerModel["claude-sonnet-4-6"]
	assert.Equal(t, int64(150), model.InputTokens)
	assert.Equal(t, int64(225), model.OutputTokens)
	// 150 input @ $3/MTok + 225 output @ $15/MTok = 0.00045 + 0.003375 = 0.003825
	assert.InDelta(t, 0.003825, float64(snap.CostUSD), 1e-9)
}

func TestTracker_WritesUsageJSONAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "usage.json")
	tr := NewTracker(path)
	tr.Add("claude-sonnet-4-6", TokenCounts{InputTokens: 100, OutputTokens: 200})

	data, err := os.ReadFile(path) //nolint:gosec // path is under t.TempDir()
	require.NoError(t, err)
	var snap Snapshot
	require.NoError(t, json.Unmarshal(data, &snap))
	assert.Equal(t, int64(100), snap.PerModel["claude-sonnet-4-6"].InputTokens)

	// .tmp must not survive the rename.
	_, err = os.Stat(path + ".tmp")
	assert.True(t, os.IsNotExist(err), "tmp file should be renamed away")
}

func TestSSEInterceptor_AccumulatesFromStartAndDelta(t *testing.T) {
	body := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4-6-20260101","usage":{"input_tokens":42,"output_tokens":1,"cache_creation_input_tokens":7,"cache_read_input_tokens":3}}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}`,
		``,
		`event: message_delta`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":99}}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	tr := NewTracker("")
	r := &nopReadCloser{Reader: strings.NewReader(body)}
	w := wrapResponseBody(r, contentTypeEventStream, tr)
	_, err := io.Copy(io.Discard, w)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	m := tr.snapshot().PerModel["claude-sonnet-4-6-20260101"]
	assert.Equal(t, int64(42), m.InputTokens)
	assert.Equal(t, int64(99), m.OutputTokens, "message_delta output supersedes message_start")
	assert.Equal(t, int64(7), m.CacheCreationInputTokens)
	assert.Equal(t, int64(3), m.CacheReadInputTokens)
}

func TestSSEInterceptor_HandlesSplitReads(t *testing.T) {
	body := `event: message_start
data: {"type":"message_start","message":{"model":"claude-haiku-4-5","usage":{"input_tokens":10,"output_tokens":1}}}

event: message_delta
data: {"type":"message_delta","delta":{},"usage":{"output_tokens":5}}

`
	// Feed one byte at a time to exercise the buffer.
	tr := NewTracker("")
	r := &nopReadCloser{Reader: byteByByteReader(body)}
	w := wrapResponseBody(r, contentTypeEventStream, tr)
	_, err := io.Copy(io.Discard, w)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	m := tr.snapshot().PerModel["claude-haiku-4-5"]
	assert.Equal(t, int64(10), m.InputTokens)
	assert.Equal(t, int64(5), m.OutputTokens)
}

func TestJSONInterceptor_NonStreamingResponse(t *testing.T) {
	body := `{"id":"msg_1","type":"message","role":"assistant","model":"` + "claude-sonnet-4-6" + `","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":13,"output_tokens":7}}`
	tr := NewTracker("")
	r := &nopReadCloser{Reader: strings.NewReader(body)}
	w := wrapResponseBody(r, "application/json", tr)
	out, err := io.ReadAll(w)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	assert.Equal(t, body, string(out), "non-streaming body must pass through unchanged")
	m := tr.snapshot().PerModel["claude-sonnet-4-6"]
	assert.Equal(t, int64(13), m.InputTokens)
	assert.Equal(t, int64(7), m.OutputTokens)
}

func TestSSEInterceptor_IgnoresGarbage(t *testing.T) {
	body := "data: not-json-at-all\n\nfoo: bar\n\ndata: {\"type\":\"unknown\"}\n\n"
	tr := NewTracker("")
	r := &nopReadCloser{Reader: strings.NewReader(body)}
	w := wrapResponseBody(r, contentTypeEventStream, tr)
	_, err := io.Copy(io.Discard, w)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	assert.InDelta(t, 0.0, float64(tr.snapshot().CostUSD), 1e-9)
}

type nopReadCloser struct{ io.Reader }

func (nopReadCloser) Close() error { return nil }

type oneByteReader struct {
	src []byte
	pos int
}

func (r *oneByteReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.src) {
		return 0, io.EOF
	}
	p[0] = r.src[r.pos]
	r.pos++
	return 1, nil
}

func byteByByteReader(s string) io.Reader {
	return &oneByteReader{src: []byte(s)}
}
