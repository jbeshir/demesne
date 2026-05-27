package openai

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

// Tracker accumulates OpenAI token usage and cost across all requests
// handled by one sidecar instance. It's safe for concurrent use; the
// response interceptors fold per-request usage into it via Add. The host
// reads the resulting snapshot from usage.json after the sandbox exits.
type Tracker struct {
	mu        sync.Mutex
	usagePath string // empty disables disk writes
	perModel  map[ModelID]*TokenCounts
}

// NewTracker constructs a Tracker. usagePath is the host-bind-mounted
// path the tracker rewrites with a JSON snapshot after every Add
// (empty disables writes — useful in tests).
func NewTracker(usagePath string) *Tracker {
	return &Tracker{
		usagePath: usagePath,
		perModel:  map[ModelID]*TokenCounts{},
	}
}

// Add folds another request's token counts into the per-model totals
// and rewrites usage.json. modelID may be a versioned OpenAI ID (e.g.
// "gpt-5.5-2026..."); pricing uses longest-prefix-match so versioned
// IDs route to their family.
func (t *Tracker) Add(modelID ModelID, tc TokenCounts) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if modelID == "" {
		modelID = "unknown"
	}
	mu, ok := t.perModel[modelID]
	if !ok {
		mu = &TokenCounts{}
		t.perModel[modelID] = mu
	}
	mu.InputTokens += tc.InputTokens
	mu.OutputTokens += tc.OutputTokens
	mu.TotalTokens += tc.TotalTokens
	mu.InputTokensDetails.CachedTokens += tc.InputTokensDetails.CachedTokens
	mu.OutputTokensDetails.ReasoningTokens += tc.OutputTokensDetails.ReasoningTokens
	t.persistLocked()
}

// costUSDLocked returns the cumulative cost; caller must hold the mutex.
func (t *Tracker) costUSDLocked() USD {
	var total USD
	for modelID, mu := range t.perModel {
		total += CostUSD(modelID, *mu)
	}
	return total
}

// Snapshot is the JSON-serializable view of the tracker.
type Snapshot struct {
	CostUSD  USD                    `json:"cost_usd"`
	PerModel map[string]ModelReport `json:"per_model"`
}

// ModelReport breaks usage down for one OpenAI model family.
type ModelReport struct {
	InputTokens     int64 `json:"input_tokens"`
	OutputTokens    int64 `json:"output_tokens"`
	CachedTokens    int64 `json:"cached_tokens"`
	ReasoningTokens int64 `json:"reasoning_tokens"`
	CostUSD         USD   `json:"cost_usd"`
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
		models = append(models, string(k))
	}
	sort.Strings(models)
	report := make(map[string]ModelReport, len(t.perModel))
	for _, m := range models {
		mu := t.perModel[ModelID(m)]
		report[m] = ModelReport{
			InputTokens:     mu.InputTokens,
			OutputTokens:    mu.OutputTokens,
			CachedTokens:    mu.InputTokensDetails.CachedTokens,
			ReasoningTokens: mu.OutputTokensDetails.ReasoningTokens,
			CostUSD:         CostUSD(ModelID(m), *mu),
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
		fmt.Fprintf(os.Stderr, "openai proxy: marshal usage snapshot: %v\n", err)
		return
	}
	tmp := t.usagePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "openai proxy: write usage tmp: %v\n", err)
		return
	}
	if err := os.Rename(tmp, t.usagePath); err != nil {
		fmt.Fprintf(os.Stderr, "openai proxy: rename usage: %v\n", err)
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

// sseInterceptor parses the OpenAI Responses API SSE stream as the
// response body is read. It watches for the response.completed event,
// which carries the definitive per-response usage totals, and submits
// one Add call to the tracker at body Close. If no response.completed
// is seen but an event carries a top-level usage block, that is used as
// a fallback; response.completed always takes precedence.
type sseInterceptor struct {
	upstream io.ReadCloser
	tracker  *Tracker
	buf      bytes.Buffer

	modelID  ModelID
	counts   TokenCounts
	sawUsage bool
	flushed  bool
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
// it also drains any final partial line.
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

// sseEvent is the minimal shape of an OpenAI Responses API SSE event.
// The response.completed event carries definitive usage in Response.Usage.
// Some other event types may carry a top-level Usage block.
type sseEvent struct {
	Type     string `json:"type"`
	Response *struct {
		Model string       `json:"model"`
		Usage *TokenCounts `json:"usage"`
	} `json:"response"`
	Usage *TokenCounts `json:"usage"` // top-level fallback; prefer response.completed
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
		// Unknown event types and comment lines are silently dropped.
		return
	}
	// Update tracked model from any event that carries it.
	if ev.Response != nil && ev.Response.Model != "" {
		s.modelID = ModelID(ev.Response.Model)
	}
	switch ev.Type {
	case "response.completed":
		// Primary path: response.completed carries the definitive usage.
		if ev.Response != nil && ev.Response.Usage != nil {
			s.counts = *ev.Response.Usage
			s.sawUsage = true
		}
	default:
		// Fallback: some events carry a top-level usage block; use it
		// only if a response.completed hasn't already been recorded
		// (response.completed takes precedence).
		if ev.Usage != nil && !s.sawUsage {
			s.counts = *ev.Usage
			s.sawUsage = true
		}
	}
}

func (s *sseInterceptor) flush() {
	if s.flushed {
		return
	}
	s.flushed = true
	if !s.sawUsage {
		return
	}
	s.tracker.Add(s.modelID, s.counts)
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
		Model string       `json:"model"`
		Usage *TokenCounts `json:"usage"`
	}
	if err := json.Unmarshal(j.tee.Bytes(), &body); err != nil {
		return nil
	}
	if body.Usage == nil {
		return nil
	}
	j.tracker.Add(ModelID(body.Model), *body.Usage)
	return nil
}
