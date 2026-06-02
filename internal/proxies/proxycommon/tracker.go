package proxycommon

import (
	"sort"
	"sync"
)

// Snapshot is the JSON-serializable view of a Tracker.
type Snapshot struct {
	CostUSD  float64        `json:"cost_usd"`
	PerModel map[string]any `json:"per_model"`
}

// Tracker is a generic per-model token-counts accumulator shared by
// vendor proxies. TC is the vendor-specific per-request usage shape; the
// vendor owns merge semantics by passing a combine function. The tracker
// holds the mutex, the perModel map, the usagePath, and offers snapshotting
// + atomic persistence to disk.
type Tracker[TC any] struct {
	mu        sync.Mutex
	usagePath string
	logPrefix string
	perModel  map[string]*TC
	combine   func(into *TC, add TC)
	cost      func(modelID string, tc TC) float64
	report    func(modelID string, tc TC) any
}

// NewTracker constructs a Tracker. usagePath is the path the tracker rewrites
// with a JSON snapshot after every Add (empty disables writes). logPrefix
// prefixes log lines (e.g. "anthropic proxy").
func NewTracker[TC any](
	usagePath, logPrefix string,
	combine func(*TC, TC),
	cost func(string, TC) float64,
	report func(string, TC) any,
) *Tracker[TC] {
	return &Tracker[TC]{
		usagePath: usagePath,
		logPrefix: logPrefix,
		perModel:  map[string]*TC{},
		combine:   combine,
		cost:      cost,
		report:    report,
	}
}

// Add folds another request's token counts into the per-model totals
// and rewrites the usage file. An empty modelID is stored as "unknown".
func (t *Tracker[TC]) Add(modelID string, tc TC) {
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
	t.persistLocked()
}

// Snapshot returns a JSON-serializable view of the current state.
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
	return Snapshot{
		CostUSD:  costTotal,
		PerModel: perModel,
	}
}

func (t *Tracker[TC]) persistLocked() {
	if t.usagePath == "" {
		return
	}
	WriteUsageAtomic(t.usagePath, t.logPrefix, t.snapshotLocked())
}
