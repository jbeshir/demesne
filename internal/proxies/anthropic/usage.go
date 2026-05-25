package anthropic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Tracker accumulates Anthropic token usage and cost across all
// requests handled by one sidecar instance. It's safe for concurrent
// use; the response interceptors fold per-request usage into it via
// Add. The host reads the resulting snapshot from usage.json after the
// sandbox exits.
type Tracker struct {
	mu        sync.Mutex
	usagePath string // empty disables disk writes
	perModel  map[string]*modelUsage
}

type modelUsage struct {
	TokenCounts
}

// NewTracker constructs a Tracker. usagePath is the host-bind-mounted
// path the tracker rewrites with a JSON snapshot after every Add
// (empty disables writes — useful in tests).
func NewTracker(usagePath string) *Tracker {
	return &Tracker{
		usagePath: usagePath,
		perModel:  map[string]*modelUsage{},
	}
}

// Add folds another request's token counts into the per-model totals
// and rewrites usage.json. modelID may be a dated Anthropic ID (e.g.
// "claude-opus-4-7-20251201"); pricing uses longest-prefix-match so
// dated IDs route to their family.
func (t *Tracker) Add(modelID string, tc TokenCounts) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if modelID == "" {
		modelID = "unknown"
	}
	mu, ok := t.perModel[modelID]
	if !ok {
		mu = &modelUsage{}
		t.perModel[modelID] = mu
	}
	mu.InputTokens += tc.InputTokens
	mu.OutputTokens += tc.OutputTokens
	mu.CacheCreationInputTokens += tc.CacheCreationInputTokens
	mu.CacheReadInputTokens += tc.CacheReadInputTokens
	t.persistLocked()
}

// costUSDLocked returns the cumulative cost; caller must hold the mutex.
func (t *Tracker) costUSDLocked() float64 {
	var total float64
	for modelID, mu := range t.perModel {
		total += CostUSD(modelID, mu.TokenCounts)
	}
	return total
}

// Snapshot is the JSON-serializable view of the tracker.
type Snapshot struct {
	CostUSD  float64                `json:"cost_usd"`
	PerModel map[string]ModelReport `json:"per_model"`
}

// ModelReport breaks usage down for one Anthropic model family.
type ModelReport struct {
	InputTokens              int64   `json:"input_tokens"`
	OutputTokens             int64   `json:"output_tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens"`
	CostUSD                  float64 `json:"cost_usd"`
}

// snapshot returns the current state. Called only by in-package tests;
// production reads come from usage.json written by persistLocked.
func (t *Tracker) snapshot() Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.snapshotLocked()
}

func (t *Tracker) snapshotLocked() Snapshot {
	models := make([]string, 0, len(t.perModel))
	for k := range t.perModel {
		models = append(models, k)
	}
	sort.Strings(models)
	report := make(map[string]ModelReport, len(t.perModel))
	for _, m := range models {
		mu := t.perModel[m]
		report[m] = ModelReport{
			InputTokens:              mu.InputTokens,
			OutputTokens:             mu.OutputTokens,
			CacheCreationInputTokens: mu.CacheCreationInputTokens,
			CacheReadInputTokens:     mu.CacheReadInputTokens,
			CostUSD:                  CostUSD(m, mu.TokenCounts),
		}
	}
	return Snapshot{
		CostUSD:  t.costUSDLocked(),
		PerModel: report,
	}
}

func (t *Tracker) persistLocked() {
	if t.usagePath == "" {
		return
	}
	snap := t.snapshotLocked()
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		// MarshalIndent on this fixed shape can't fail in practice;
		// log to stderr and move on so the proxy keeps serving.
		fmt.Fprintf(os.Stderr, "anthropic proxy: marshal usage snapshot: %v\n", err)
		return
	}
	tmp := t.usagePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "anthropic proxy: write usage tmp: %v\n", err)
		return
	}
	if err := os.Rename(tmp, t.usagePath); err != nil {
		fmt.Fprintf(os.Stderr, "anthropic proxy: rename usage: %v\n", err)
	}
}

// EnsureUsageDir creates the parent directory for usagePath if missing.
// Caller invokes this once at startup so per-request writes can't fail
// on a missing dir.
func EnsureUsageDir(usagePath string) error {
	if usagePath == "" {
		return nil
	}
	dir := filepath.Dir(usagePath)
	return os.MkdirAll(dir, 0o750)
}

const contentTypeEventStream = "text/event-stream"

// wrapResponseBody returns a ReadCloser that tees the upstream body
// through a usage parser. The parser folds any usage it finds into the
// tracker when the body is closed.
//
// Streaming responses (Content-Type starts with text/event-stream) are
// parsed line-by-line as SSE; non-streaming responses are buffered and
// parsed as a single JSON document at close.
func wrapResponseBody(upstream io.ReadCloser, contentType string, t *Tracker) io.ReadCloser {
	if strings.HasPrefix(contentType, contentTypeEventStream) {
		return &sseInterceptor{
			upstream: upstream,
			tracker:  t,
		}
	}
	return &jsonInterceptor{
		upstream: upstream,
		tracker:  t,
	}
}

// sseInterceptor parses Anthropic's SSE stream as the response body is
// read. It tracks the running per-response totals and submits one Add
// call to the tracker at body Close. Output tokens are taken from the
// final message_delta (which carries the cumulative output count for
// the message); inputs and cache figures come from message_start.
type sseInterceptor struct {
	upstream io.ReadCloser
	tracker  *Tracker
	buf      bytes.Buffer

	modelID                  string
	inputTokens              int64
	cacheCreationInputTokens int64
	cacheReadInputTokens     int64
	outputTokens             int64
	sawStart                 bool
	flushed                  bool
}

func (s *sseInterceptor) Read(p []byte) (int, error) {
	n, err := s.upstream.Read(p)
	if n > 0 {
		s.buf.Write(p[:n])
		s.scan(false)
	}
	if err == io.EOF {
		s.scan(true)
	}
	return n, err
}

func (s *sseInterceptor) Close() error {
	s.flush()
	return s.upstream.Close()
}

// scan consumes complete SSE lines from the buffer. When eof is true,
// it also drains any final partial line (which Anthropic terminates
// with a newline before EOF; we tolerate either).
func (s *sseInterceptor) scan(eof bool) {
	for {
		idx := bytes.IndexByte(s.buf.Bytes(), '\n')
		if idx < 0 {
			if !eof {
				return
			}
			if s.buf.Len() == 0 {
				return
			}
			line := s.buf.String()
			s.buf.Reset()
			s.handleLine(line)
			return
		}
		raw := s.buf.Next(idx + 1)
		// Strip trailing \r\n or \n.
		line := strings.TrimRight(string(raw), "\r\n")
		s.handleLine(line)
	}
}

// sseUsage is the JSON shape of a usage block embedded in an SSE
// event. Anthropic puts one inside message_start.message and another
// inside message_delta directly.
type sseUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
}

type sseEvent struct {
	Type    string `json:"type"`
	Message *struct {
		Model string    `json:"model"`
		Usage *sseUsage `json:"usage"`
	} `json:"message"`
	Usage *sseUsage `json:"usage"`
}

func (s *sseInterceptor) handleLine(line string) {
	if !strings.HasPrefix(line, "data:") {
		return
	}
	payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if payload == "" || payload == "[DONE]" {
		return
	}
	var ev sseEvent
	if err := json.Unmarshal([]byte(payload), &ev); err != nil {
		// Anthropic occasionally emits comment lines / unknown events;
		// silently drop anything we can't parse.
		return
	}
	switch ev.Type {
	case "message_start":
		s.applyStart(ev)
	case "message_delta":
		s.applyDelta(ev)
	}
}

func (s *sseInterceptor) applyStart(ev sseEvent) {
	if ev.Message == nil {
		return
	}
	if ev.Message.Model != "" {
		s.modelID = ev.Message.Model
	}
	if ev.Message.Usage == nil {
		return
	}
	s.inputTokens = ev.Message.Usage.InputTokens
	s.cacheCreationInputTokens = ev.Message.Usage.CacheCreationInputTokens
	s.cacheReadInputTokens = ev.Message.Usage.CacheReadInputTokens
	s.outputTokens = ev.Message.Usage.OutputTokens
	s.sawStart = true
}

func (s *sseInterceptor) applyDelta(ev sseEvent) {
	if ev.Usage == nil {
		return
	}
	// message_delta.usage.output_tokens is the cumulative output for
	// the message; it supersedes the initial value from message_start.
	s.outputTokens = ev.Usage.OutputTokens
	if ev.Usage.InputTokens > 0 {
		s.inputTokens = ev.Usage.InputTokens
	}
	if ev.Usage.CacheCreationInputTokens > 0 {
		s.cacheCreationInputTokens = ev.Usage.CacheCreationInputTokens
	}
	if ev.Usage.CacheReadInputTokens > 0 {
		s.cacheReadInputTokens = ev.Usage.CacheReadInputTokens
	}
}

func (s *sseInterceptor) flush() {
	if s.flushed {
		return
	}
	s.flushed = true
	if !s.sawStart {
		return
	}
	s.tracker.Add(s.modelID, TokenCounts{
		InputTokens:              s.inputTokens,
		OutputTokens:             s.outputTokens,
		CacheCreationInputTokens: s.cacheCreationInputTokens,
		CacheReadInputTokens:     s.cacheReadInputTokens,
	})
}

// jsonInterceptor buffers a non-streaming JSON response body, forwards
// it to the caller as-is, and parses one usage block out of the
// completed JSON on Close.
type jsonInterceptor struct {
	upstream io.ReadCloser
	tracker  *Tracker
	tee      bytes.Buffer
}

func (j *jsonInterceptor) Read(p []byte) (int, error) {
	n, err := j.upstream.Read(p)
	if n > 0 {
		j.tee.Write(p[:n])
	}
	return n, err
}

func (j *jsonInterceptor) Close() error {
	defer func() { _ = j.upstream.Close() }()
	if j.tee.Len() == 0 {
		return nil
	}
	var body struct {
		Model string `json:"model"`
		Usage *struct {
			InputTokens              int64 `json:"input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(j.tee.Bytes(), &body); err != nil {
		return nil
	}
	if body.Usage == nil {
		return nil
	}
	j.tracker.Add(body.Model, TokenCounts{
		InputTokens:              body.Usage.InputTokens,
		OutputTokens:             body.Usage.OutputTokens,
		CacheCreationInputTokens: body.Usage.CacheCreationInputTokens,
		CacheReadInputTokens:     body.Usage.CacheReadInputTokens,
	})
	return nil
}
