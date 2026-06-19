package proxycommon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// TokenBreakdown is the normalized per-request token breakdown written to
// usage.jsonl. Vendor proxies supply a callback to map their own TC into
// this shape when constructing a Tracker.
type TokenBreakdown struct {
	Input         int64
	Output        int64
	CacheCreation int64
	CacheRead     int64
}

// DroppedCounts tracks requests whose usage could not be recorded, either
// because the response body could not be parsed or because it contained no
// usage block.
type DroppedCounts struct {
	ParseErrors  int64 `json:"parse_errors"`
	NoUsageBlock int64 `json:"no_usage_block"`
}

// Snapshot is the JSON-serializable view of a Tracker.
type Snapshot struct {
	CostUSD  float64        `json:"cost_usd"`
	PerModel map[string]any `json:"per_model"`
	// Dropped is omitted when both counters are zero so that usage.json
	// remains byte-identical to pre-feature output on clean runs.
	Dropped *DroppedCounts `json:"dropped,omitempty"`
}

// Tracker is a generic per-model token-counts accumulator shared by
// vendor proxies. TC is the vendor-specific per-request usage shape; the
// vendor owns merge semantics by passing a combine function. The tracker
// holds the mutex, the perModel map, the usagePath, and offers snapshotting
// + atomic persistence to disk.
type Tracker[TC any] struct {
	mu                  sync.Mutex
	usagePath           string
	jsonlPath           string
	logPrefix           string
	perModel            map[string]*TC
	combine             func(into *TC, add TC)
	cost                func(modelID string, tc TC) float64
	report              func(modelID string, tc TC) any
	tokens              func(TC) TokenBreakdown
	now                 func() time.Time
	droppedParseErrors  int64
	droppedNoUsageBlock int64
}

// NewTracker constructs a Tracker. usagePath is the path the tracker rewrites
// with a JSON snapshot after every Add (empty disables writes). logPrefix
// prefixes log lines (e.g. "anthropic proxy"). tokens normalizes a vendor TC
// into the four token types written to usage.jsonl.
func NewTracker[TC any](
	usagePath, logPrefix string,
	combine func(*TC, TC),
	cost func(string, TC) float64,
	report func(string, TC) any,
	tokens func(TC) TokenBreakdown,
) *Tracker[TC] {
	jsonlPath := ""
	if usagePath != "" {
		jsonlPath = filepath.Join(filepath.Dir(usagePath), "usage.jsonl")
	}
	return &Tracker[TC]{
		usagePath: usagePath,
		jsonlPath: jsonlPath,
		logPrefix: logPrefix,
		perModel:  map[string]*TC{},
		combine:   combine,
		cost:      cost,
		report:    report,
		tokens:    tokens,
		now:       time.Now,
	}
}

// Add folds another request's token counts into the per-model totals,
// appends one normalized line to usage.jsonl keyed on requestID, and
// rewrites the usage file. An empty modelID is stored as "unknown".
func (t *Tracker[TC]) Add(modelID string, tc TC, requestID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if modelID == "" {
		modelID = "unknown"
	}
	mu, ok := t.perModel[modelID]
	if !ok {
		mu = new(TC)
		t.perModel[modelID] = mu
	}
	t.combine(mu, tc)
	t.appendRequestLocked(modelID, tc, requestID)
	t.persistLocked()
}

// AddDroppedParseError increments the parse-error drop counter and persists.
// Call this when the response body could not be decoded.
func (t *Tracker[TC]) AddDroppedParseError() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.droppedParseErrors++
	t.persistLocked()
}

// AddDroppedNoUsageBlock increments the no-usage-block drop counter and
// persists. Call this when the response decoded successfully but contained no
// usage data.
func (t *Tracker[TC]) AddDroppedNoUsageBlock() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.droppedNoUsageBlock++
	t.persistLocked()
}

// Snapshot returns per-model totals and the aggregate USD cost. Model IDs are sorted for stable
// output. Safe for concurrent use.
func (t *Tracker[TC]) Snapshot() Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.snapshotLocked()
}

func (t *Tracker[TC]) snapshotLocked() Snapshot {
	models := make([]string, 0, len(t.perModel))
	for k := range t.perModel {
		models = append(models, k)
	}
	sort.Strings(models)
	perModel := make(map[string]any, len(t.perModel))
	var costTotal float64
	for _, m := range models {
		tc := t.perModel[m]
		perModel[m] = t.report(m, *tc)
		costTotal += t.cost(m, *tc)
	}
	snap := Snapshot{
		CostUSD:  costTotal,
		PerModel: perModel,
	}
	if t.droppedParseErrors > 0 || t.droppedNoUsageBlock > 0 {
		snap.Dropped = &DroppedCounts{
			ParseErrors:  t.droppedParseErrors,
			NoUsageBlock: t.droppedNoUsageBlock,
		}
	}
	return snap
}

func (t *Tracker[TC]) persistLocked() {
	if t.usagePath == "" {
		return
	}
	WriteUsageAtomic(t.usagePath, t.logPrefix, t.snapshotLocked())
}

// appendRequestLocked appends one normalized JSON line to usage.jsonl.
// Must be called with t.mu held.
func (t *Tracker[TC]) appendRequestLocked(modelID string, tc TC, requestID string) {
	if t.jsonlPath == "" {
		return
	}
	bd := t.tokens(tc)
	rec := struct {
		RequestID     string `json:"requestId"`
		TS            string `json:"ts"`
		Model         string `json:"model"`
		Input         int64  `json:"input"`
		Output        int64  `json:"output"`
		CacheCreation int64  `json:"cache_creation"`
		CacheRead     int64  `json:"cache_read"`
	}{
		RequestID:     requestID,
		TS:            t.now().UTC().Format(time.RFC3339),
		Model:         modelID,
		Input:         bd.Input,
		Output:        bd.Output,
		CacheCreation: bd.CacheCreation,
		CacheRead:     bd.CacheRead,
	}
	data, err := json.Marshal(rec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: marshal usage.jsonl line: %v\n", t.logPrefix, err)
		return
	}
	data = append(data, '\n')
	f, err := os.OpenFile(t.jsonlPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: open usage.jsonl: %v\n", t.logPrefix, err)
		return
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(data); err != nil {
		fmt.Fprintf(os.Stderr, "%s: write usage.jsonl: %v\n", t.logPrefix, err)
	}
}
