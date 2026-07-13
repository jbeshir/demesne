package openai

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/jbeshir/demesne/internal/proxies/proxycommon"
)

// Tracker accumulates OpenAI token usage and cost across all requests
// handled by one sidecar instance. It's safe for concurrent use; the
// response interceptors fold per-request usage into it via Add. The host
// reads the resulting snapshot from usage.json after the sandbox exits.
type Tracker struct {
	*proxycommon.Tracker[TokenCounts]
}

// NewTracker constructs a Tracker. usagePath is the host-bind-mounted
// path the tracker rewrites with a JSON snapshot after every Add
// (empty disables writes — useful in tests).
func NewTracker(usagePath string) *Tracker {
	return &Tracker{proxycommon.NewTracker[TokenCounts](
		usagePath, "openai proxy",
		combineCounts, modelCostUSD, modelReport, normalizeTokens,
	)}
}

// Add folds another request's token counts into the per-model totals
// and rewrites usage.json. modelID may be a versioned OpenAI ID (e.g.
// "gpt-5.6-sol-2026..."); pricing uses longest-prefix-match so versioned
// IDs route to their family.
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
	into.TotalTokens += add.TotalTokens
	into.InputTokensDetails.CachedTokens += add.InputTokensDetails.CachedTokens
	into.OutputTokensDetails.ReasoningTokens += add.OutputTokensDetails.ReasoningTokens
}

func modelCostUSD(m string, tc TokenCounts) float64 {
	return float64(CostUSD(ModelID(m), tc))
}

func modelReport(m string, tc TokenCounts) any {
	return ModelReport{
		InputTokens:     tc.InputTokens,
		OutputTokens:    tc.OutputTokens,
		CachedTokens:    tc.InputTokensDetails.CachedTokens,
		ReasoningTokens: tc.OutputTokensDetails.ReasoningTokens,
		CostUSD:         CostUSD(ModelID(m), tc),
	}
}

func normalizeTokens(tc TokenCounts) proxycommon.TokenBreakdown {
	return proxycommon.TokenBreakdown{
		Input:     tc.InputTokens,
		Output:    tc.OutputTokens,
		CacheRead: tc.InputTokensDetails.CachedTokens,
	}
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

const contentTypeJSON = "application/json"

// wrapResponseBody returns a ReadCloser that tees the upstream body
// through a usage parser. The parser folds any usage it finds into the
// tracker when the body is closed.
//
// The ChatGPT Codex backend streams Server-Sent Events but does NOT
// reliably set a Content-Type header (it commonly arrives empty), so we
// default to the SSE parser and use the single-document JSON parser only
// when the response is explicitly application/json.
//
// xRequestID is the x-request-id response header value, used as a
// fallback request identifier when the response body carries no id field.
func wrapResponseBody(upstream io.ReadCloser, contentType string, xRequestID string, t *Tracker) io.ReadCloser {
	if strings.HasPrefix(contentType, contentTypeJSON) {
		return proxycommon.NewJSONInterceptor(upstream, func(data []byte) {
			var body struct {
				ID    string       `json:"id"`
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
			reqID := body.ID
			if reqID == "" {
				reqID = xRequestID
			}
			t.Add(ModelID(body.Model), *body.Usage, reqID)
		})
	}
	state := &sseState{tracker: t, xRequestID: xRequestID}
	return proxycommon.NewSSEInterceptor(upstream, state.handleLine, state.flush)
}

// sseState holds per-response SSE parsing state for the OpenAI vendor.
type sseState struct {
	tracker    *Tracker
	modelID    ModelID
	counts     TokenCounts
	sawUsage   bool
	requestID  string // response.id from the SSE body
	xRequestID string // fallback: x-request-id response header
}

// sseEvent is the minimal shape of an OpenAI Responses API SSE event.
// The response.completed event carries definitive usage in Response.Usage.
// Some other event types may carry a top-level Usage block.
type sseEvent struct {
	Type     string `json:"type"`
	Response *struct {
		ID    string       `json:"id"`
		Model string       `json:"model"`
		Usage *TokenCounts `json:"usage"`
	} `json:"response"`
	Usage *TokenCounts `json:"usage"` // top-level fallback; prefer response.completed
}

// applyEvent applies the usage data from ev to s. Separated from handleLine
// to keep handleLine below the gocyclo threshold.
func (s *sseState) applyEvent(ev sseEvent) {
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
		// Unknown event types and comment lines are silently dropped;
		// per-line parse failures are expected and not counted as drops.
		return
	}
	// Update tracked model and request ID from any event that carries them.
	if ev.Response != nil {
		if ev.Response.Model != "" {
			s.modelID = ModelID(ev.Response.Model)
		}
		if ev.Response.ID != "" {
			s.requestID = ev.Response.ID
		}
	}
	s.applyEvent(ev)
}

func (s *sseState) flush() {
	if !s.sawUsage {
		s.tracker.AddDroppedNoUsageBlock()
		return
	}
	// requestID is recorded for completeness and is NOT used for subagent
	// attribution (anthropic/claude-code only).
	reqID := s.requestID
	if reqID == "" {
		reqID = s.xRequestID
	}
	s.tracker.Add(s.modelID, s.counts, reqID)
}
