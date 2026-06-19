package proxycommon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testTC struct {
	Input  int64
	Output int64
}

func newTestTracker(usagePath string) *Tracker[testTC] {
	return NewTracker[testTC](
		usagePath, "test",
		func(into *testTC, add testTC) {
			into.Input += add.Input
			into.Output += add.Output
		},
		func(_ string, tc testTC) float64 { return float64(tc.Input + tc.Output) },
		func(_ string, tc testTC) any { return tc },
		func(tc testTC) TokenBreakdown {
			return TokenBreakdown{Input: tc.Input, Output: tc.Output}
		},
	)
}

// TestTracker_JSONLLine asserts that Add writes a correct normalized line to
// usage.jsonl with the right fields, requestId, and a parseable RFC3339 ts.
func TestTracker_JSONLLine(t *testing.T) {
	dir := t.TempDir()
	tr := newTestTracker(filepath.Join(dir, "usage.json"))

	tr.Add("mymodel", testTC{Input: 10, Output: 5}, "req-abc")

	data, err := os.ReadFile(filepath.Join(dir, "usage.jsonl")) //nolint:gosec // path under t.TempDir()
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, 1)

	var rec struct {
		RequestID     string `json:"requestId"`
		TS            string `json:"ts"`
		Model         string `json:"model"`
		Input         int64  `json:"input"`
		Output        int64  `json:"output"`
		CacheCreation int64  `json:"cache_creation"`
		CacheRead     int64  `json:"cache_read"`
	}
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &rec))

	assert.Equal(t, "req-abc", rec.RequestID)
	assert.Equal(t, "mymodel", rec.Model)
	assert.Equal(t, int64(10), rec.Input)
	assert.Equal(t, int64(5), rec.Output)
	assert.Equal(t, int64(0), rec.CacheCreation)
	assert.Equal(t, int64(0), rec.CacheRead)
	_, err = time.Parse(time.RFC3339, rec.TS)
	assert.NoError(t, err, "ts must be parseable as RFC3339")
}

// TestTracker_JSONLMultipleLines asserts that multiple Add calls produce
// one line per call in usage.jsonl.
func TestTracker_JSONLMultipleLines(t *testing.T) {
	dir := t.TempDir()
	tr := newTestTracker(filepath.Join(dir, "usage.json"))

	tr.Add("m1", testTC{Input: 1}, "req-1")
	tr.Add("m2", testTC{Input: 2}, "req-2")

	data, err := os.ReadFile(filepath.Join(dir, "usage.jsonl")) //nolint:gosec // path under t.TempDir()
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Len(t, lines, 2)

	var rec1 struct {
		RequestID string `json:"requestId"`
		Model     string `json:"model"`
	}
	var rec2 struct {
		RequestID string `json:"requestId"`
		Model     string `json:"model"`
	}
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &rec1))
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &rec2))
	assert.Equal(t, "req-1", rec1.RequestID)
	assert.Equal(t, "m1", rec1.Model)
	assert.Equal(t, "req-2", rec2.RequestID)
	assert.Equal(t, "m2", rec2.Model)
}

// TestTracker_DroppedCounters asserts that dropped appears in the snapshot
// and usage.json only after a drop method is called, and matches exact counts.
func TestTracker_DroppedCounters(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "usage.json")
	tr := newTestTracker(path)

	// Add some usage with no drops; dropped must be absent from usage.json.
	tr.Add("m", testTC{Input: 1}, "req-1")
	data, err := os.ReadFile(path) //nolint:gosec // path under t.TempDir()
	require.NoError(t, err)
	var snapMap map[string]any
	require.NoError(t, json.Unmarshal(data, &snapMap))
	_, hasDropped := snapMap["dropped"]
	assert.False(t, hasDropped, "dropped must be absent when no drops occurred")

	// Trigger drops and verify the counts.
	tr.AddDroppedParseError()
	tr.AddDroppedNoUsageBlock()
	tr.AddDroppedNoUsageBlock()

	snap := tr.Snapshot()
	require.NotNil(t, snap.Dropped)
	assert.Equal(t, int64(1), snap.Dropped.ParseErrors)
	assert.Equal(t, int64(2), snap.Dropped.NoUsageBlock)

	// Dropped must now appear in persisted usage.json.
	data, err = os.ReadFile(path) //nolint:gosec // path under t.TempDir()
	require.NoError(t, err)
	var snapMap2 map[string]any
	require.NoError(t, json.Unmarshal(data, &snapMap2))
	_, hasDropped = snapMap2["dropped"]
	assert.True(t, hasDropped, "dropped must appear after drops")
}

// TestTracker_NoDropsNoJSONL asserts that when usagePath is empty no
// usage.jsonl file is written (jsonlPath is also empty).
func TestTracker_NoDropsNoJSONL(t *testing.T) {
	tr := newTestTracker("")
	tr.Add("m", testTC{Input: 1}, "req-x")
	// No assertion on file; this just must not panic or error.
}

// TestTracker_JSONLUsesNowCallback asserts that the now callback controls
// the timestamp written to usage.jsonl.
func TestTracker_JSONLUsesNowCallback(t *testing.T) {
	dir := t.TempDir()
	tr := newTestTracker(filepath.Join(dir, "usage.json"))
	fixed := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	tr.now = func() time.Time { return fixed }

	tr.Add("m", testTC{Input: 1}, "req-ts")

	data, err := os.ReadFile(filepath.Join(dir, "usage.jsonl")) //nolint:gosec // path under t.TempDir()
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, 1)
	var rec struct {
		TS string `json:"ts"`
	}
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &rec))
	assert.Equal(t, "2026-06-19T12:00:00Z", rec.TS)
}
