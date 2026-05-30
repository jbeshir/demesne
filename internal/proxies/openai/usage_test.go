package openai

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
	var tc1, tc2 TokenCounts
	tc1.InputTokens = 100
	tc1.OutputTokens = 200
	tc1.InputTokensDetails.CachedTokens = 50
	tc1.OutputTokensDetails.ReasoningTokens = 10
	tc2.InputTokens = 50
	tc2.OutputTokens = 25
	tc2.InputTokensDetails.CachedTokens = 30
	tc2.OutputTokensDetails.ReasoningTokens = 5
	tr.Add("gpt-5.5", tc1)
	tr.Add("gpt-5.5", tc2)
	snap := tr.snapshot()
	model := snap.PerModel["gpt-5.5"]
	assert.Equal(t, int64(150), model.InputTokens)
	assert.Equal(t, int64(225), model.OutputTokens)
	assert.Equal(t, int64(80), model.CachedTokens)
	assert.Equal(t, int64(15), model.ReasoningTokens)
}

func TestTracker_WritesUsageJSONAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "usage.json")
	tr := NewTracker(path)
	var tc TokenCounts
	tc.InputTokens = 100
	tc.OutputTokens = 200
	tr.Add("gpt-5.5", tc)

	data, err := os.ReadFile(path) //nolint:gosec // path is under t.TempDir()
	require.NoError(t, err)
	var snap Snapshot
	require.NoError(t, json.Unmarshal(data, &snap))
	assert.Equal(t, int64(100), snap.PerModel["gpt-5.5"].InputTokens)

	// .tmp must not survive the rename.
	_, err = os.Stat(path + ".tmp")
	assert.True(t, os.IsNotExist(err), "tmp file should be renamed away")
}

// sseFixture is a realistic Responses API SSE stream with a
// response.completed event carrying input, cached, output, and
// reasoning token counts.
const sseFixture = `data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-5.5"}}

data: {"type":"response.output_text.delta","delta":"Hi"}

data: {"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.5","usage":{"input_tokens":1200,"input_tokens_details":{"cached_tokens":1000},"output_tokens":345,"output_tokens_details":{"reasoning_tokens":40},"total_tokens":1545}}}

data: [DONE]

`

func TestSSEInterceptor_ResponseCompleted(t *testing.T) {
	tr := NewTracker("")
	r := &nopReadCloser{Reader: strings.NewReader(sseFixture)}
	w := wrapResponseBody(r, "", tr)
	_, err := io.Copy(io.Discard, w)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	snap := tr.snapshot()
	require.Contains(t, snap.PerModel, "gpt-5.5", "tracker must record gpt-5.5 usage")
	m := snap.PerModel["gpt-5.5"]
	assert.Equal(t, int64(1200), m.InputTokens)
	assert.Equal(t, int64(1000), m.CachedTokens)
	assert.Equal(t, int64(345), m.OutputTokens)
	assert.Equal(t, int64(40), m.ReasoningTokens)
	// Cost > 0: gpt-5.5 has indicative pricing.
	assert.Greater(t, float64(snap.CostUSD), 0.0, "cost must be non-zero for known model")
}

// TestWrapResponseBody_DispatchesByContentType guards the real-world bug:
// the ChatGPT Codex backend streams SSE with an EMPTY Content-Type, so the
// body must still be parsed as SSE (not as a single JSON document).
func TestWrapResponseBody_DispatchesByContentType(t *testing.T) {
	for _, ct := range []string{"", "text/event-stream", "text/event-stream; charset=utf-8"} {
		tr := NewTracker("")
		r := &nopReadCloser{Reader: strings.NewReader(sseFixture)}
		w := wrapResponseBody(r, ct, tr)
		_, err := io.Copy(io.Discard, w)
		require.NoError(t, err)
		require.NoError(t, w.Close())
		assert.Contains(t, tr.snapshot().PerModel, "gpt-5.5",
			"content-type %q must be parsed as SSE", ct)
	}
}

func TestSSEInterceptor_HandlesSplitReads(t *testing.T) {
	// Feed one byte at a time to exercise the buffer.
	tr := NewTracker("")
	r := &nopReadCloser{Reader: byteByByteReader(sseFixture)}
	w := wrapResponseBody(r, "", tr)
	_, err := io.Copy(io.Discard, w)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	snap2 := tr.snapshot()
	require.Contains(t, snap2.PerModel, "gpt-5.5")
	m := snap2.PerModel["gpt-5.5"]
	assert.Equal(t, int64(1200), m.InputTokens)
	assert.Equal(t, int64(345), m.OutputTokens)
}

func TestSSEInterceptor_FallbackTopLevelUsage(t *testing.T) {
	// A stream with no response.completed but a top-level usage block.
	body := `data: {"type":"some.event","usage":{"input_tokens":100,"output_tokens":50,"input_tokens_details":{"cached_tokens":0},"output_tokens_details":{"reasoning_tokens":0}}}

data: [DONE]

`
	tr := NewTracker("")
	r := &nopReadCloser{Reader: strings.NewReader(body)}
	w := wrapResponseBody(r, "", tr)
	_, err := io.Copy(io.Discard, w)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	// Model is unknown (no response object); Add uses "unknown".
	snap := tr.snapshot()
	assert.NotEmpty(t, snap.PerModel, "fallback top-level usage must be recorded")
}

func TestSSEInterceptor_CompletedOverridesFallback(t *testing.T) {
	// A stream that has a top-level usage first, then a response.completed.
	// The response.completed values must win.
	body := `data: {"type":"some.event","usage":{"input_tokens":999,"output_tokens":999}}

data: {"type":"response.completed","response":{"model":"gpt-5.5","usage":{"input_tokens":1200,"input_tokens_details":{"cached_tokens":1000},"output_tokens":345,"output_tokens_details":{"reasoning_tokens":40},"total_tokens":1545}}}

data: [DONE]

`
	tr := NewTracker("")
	r := &nopReadCloser{Reader: strings.NewReader(body)}
	w := wrapResponseBody(r, "", tr)
	_, err := io.Copy(io.Discard, w)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	snap3 := tr.snapshot()
	require.Contains(t, snap3.PerModel, "gpt-5.5")
	m := snap3.PerModel["gpt-5.5"]
	assert.Equal(t, int64(1200), m.InputTokens, "response.completed must override top-level usage")
	assert.Equal(t, int64(345), m.OutputTokens)
}

func TestSSEInterceptor_IgnoresGarbage(t *testing.T) {
	body := "data: not-json-at-all\n\nfoo: bar\n\ndata: {\"type\":\"unknown\"}\n\n"
	tr := NewTracker("")
	r := &nopReadCloser{Reader: strings.NewReader(body)}
	w := wrapResponseBody(r, "", tr)
	_, err := io.Copy(io.Discard, w)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	assert.InDelta(t, 0.0, float64(tr.snapshot().CostUSD), 1e-9)
}

func TestJSONInterceptor_NonStreamingResponse(t *testing.T) {
	body := `{"id":"resp_1","model":"gpt-5.5","usage":{"input_tokens":13,"output_tokens":7,"input_tokens_details":{"cached_tokens":0},"output_tokens_details":{"reasoning_tokens":0},"total_tokens":20}}`
	tr := NewTracker("")
	r := &nopReadCloser{Reader: strings.NewReader(body)}
	w := wrapResponseBody(r, "application/json", tr)
	out, err := io.ReadAll(w)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	assert.Equal(t, body, string(out), "non-streaming body must pass through unchanged")
	snap4 := tr.snapshot()
	require.Contains(t, snap4.PerModel, "gpt-5.5")
	m := snap4.PerModel["gpt-5.5"]
	assert.Equal(t, int64(13), m.InputTokens)
	assert.Equal(t, int64(7), m.OutputTokens)
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
