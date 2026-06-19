package anthropic

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/jbeshir/demesne/internal/proxies/proxycommon"
)

// Tracker accumulates Anthropic token usage and cost across all
// requests handled by one sidecar instance. It's safe for concurrent
// use; the response interceptors fold per-request usage into it via
// Add. The host reads the resulting snapshot from usage.json after the
// sandbox exits.
type Tracker struct {
	*proxycommon.Tracker[TokenCounts]
}

// NewTracker constructs a Tracker. usagePath is the host-bind-mounted
// path the tracker rewrites with a JSON snapshot after every Add
// (empty disables writes — useful in tests).
func NewTracker(usagePath string) *Tracker {
	return &Tracker{proxycommon.NewTracker[TokenCounts](
		usagePath, "anthropic proxy",
		combineCounts, modelCostUSD, modelReport, normalizeTokens,
	)}
}

// Add folds another request's token counts into the per-model totals
// and rewrites usage.json. modelID may be a dated Anthropic ID (e.g.
// "claude-opus-4-8-20260101"); pricing uses longest-prefix-match so
// dated IDs route to their family.
func (t *Tracker) Add(id ModelID, tc TokenCounts, requestID string) {
	t.Tracker.Add(string(id), tc, requestID)
}

// snapshot returns the current state. Called only by in-package tests;
// production reads come from usage.json written by the generic tracker.
func (t *Tracker) snapshot() Snapshot {
	s := t.Snapshot()
	result := Snapshot{
		CostUSD:  USD(s.CostUSD),
		PerModel: make(map[string]ModelReport, len(s.PerModel)),
	}
	for k, v := range s.PerModel {
		result.PerModel[k] = v.(ModelReport)
	}
	return result
}

func combineCounts(into *TokenCounts, add TokenCounts) {
	into.InputTokens += add.InputTokens
	into.OutputTokens += add.OutputTokens
	into.CacheCreationInputTokens += add.CacheCreationInputTokens
	into.CacheReadInputTokens += add.CacheReadInputTokens
}

func modelCostUSD(m string, tc TokenCounts) float64 {
	return float64(CostUSD(ModelID(m), tc))
}

func modelReport(m string, tc TokenCounts) any {
	return ModelReport{
		InputTokens:              tc.InputTokens,
		OutputTokens:             tc.OutputTokens,
		CacheCreationInputTokens: tc.CacheCreationInputTokens,
		CacheReadInputTokens:     tc.CacheReadInputTokens,
		CostUSD:                  CostUSD(ModelID(m), tc),
	}
}

func normalizeTokens(tc TokenCounts) proxycommon.TokenBreakdown {
	return proxycommon.TokenBreakdown{
		Input:         tc.InputTokens,
		Output:        tc.OutputTokens,
		CacheCreation: tc.CacheCreationInputTokens,
		CacheRead:     tc.CacheReadInputTokens,
	}
}

// Snapshot is the JSON-serializable view of the tracker.
type Snapshot struct {
	CostUSD  USD                    `json:"cost_usd"`
	PerModel map[string]ModelReport `json:"per_model"`
}

// ModelReport breaks usage down for one Anthropic model family.
type ModelReport struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CostUSD                  USD   `json:"cost_usd"`
}

const contentTypeEventStream = "text/event-stream"

// wrapResponseBody returns a ReadCloser that tees the upstream body
// through a usage parser. The parser folds any usage it finds into the
// tracker when the body is closed.
//
// Streaming responses (Content-Type starts with text/event-stream) are
// parsed line-by-line as SSE; non-streaming responses are buffered and
// parsed as a single JSON document at close.
func wrapResponseBody(upstream io.ReadCloser, contentType string, requestID string, t *Tracker) io.ReadCloser {
	if strings.HasPrefix(contentType, contentTypeEventStream) {
		state := &sseState{tracker: t, requestID: requestID}
		return proxycommon.NewSSEInterceptor(upstream, state.handleLine, state.flush)
	}
	return proxycommon.NewJSONInterceptor(upstream, func(data []byte) {
		var body struct {
			Model string       `json:"model"`
			Usage *TokenCounts `json:"usage"`
		}
		if err := json.Unmarshal(data, &body); err != nil {
			t.AddDroppedParseError()
			return
		}
		if body.Usage == nil {
			t.AddDroppedNoUsageBlock()
			return
		}
		t.Add(ModelID(body.Model), *body.Usage, requestID)
	})
}

// sseState holds per-response SSE parsing state for the Anthropic vendor.
type sseState struct {
	tracker   *Tracker
	modelID   ModelID
	counts    TokenCounts
	sawStart  bool
	requestID string
}

type sseEvent struct {
	Type    string `json:"type"`
	Message *struct {
		Model string       `json:"model"`
		Usage *TokenCounts `json:"usage"`
	} `json:"message"`
	Usage *TokenCounts `json:"usage"`
}

func (s *sseState) handleLine(line string) {
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
		// per-line parse failures are expected and not counted as drops.
		return
	}
	switch ev.Type {
	case "message_start":
		s.applyStart(ev)
	case "message_delta":
		s.applyDelta(ev)
	}
}

func (s *sseState) applyStart(ev sseEvent) {
	if ev.Message == nil {
		return
	}
	if ev.Message.Model != "" {
		s.modelID = ModelID(ev.Message.Model)
	}
	if ev.Message.Usage == nil {
		return
	}
	s.counts = *ev.Message.Usage
	s.sawStart = true
}

func (s *sseState) applyDelta(ev sseEvent) {
	if ev.Usage == nil {
		return
	}
	// message_delta.usage.output_tokens is the cumulative output for
	// the message; it supersedes the initial value from message_start.
	s.counts.OutputTokens = ev.Usage.OutputTokens
	if ev.Usage.InputTokens > 0 {
		s.counts.InputTokens = ev.Usage.InputTokens
	}
	if ev.Usage.CacheCreationInputTokens > 0 {
		s.counts.CacheCreationInputTokens = ev.Usage.CacheCreationInputTokens
	}
	if ev.Usage.CacheReadInputTokens > 0 {
		s.counts.CacheReadInputTokens = ev.Usage.CacheReadInputTokens
	}
}

func (s *sseState) flush() {
	if !s.sawStart {
		s.tracker.AddDroppedNoUsageBlock()
		return
	}
	s.tracker.Add(s.modelID, s.counts, s.requestID)
}
