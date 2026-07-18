package sandbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testFieldDepth    = "depth"
	testFieldChildren = "children"
	testFieldCostUSD  = "cost_usd"
	testWorkerName    = "worker"
)

// writeJSONFile marshals v to JSON and writes it at dir/name.
func writeJSONFile(t *testing.T, dir, name string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0o600))
}

// writeTextFile writes text to dir/name.
func writeTextFile(t *testing.T, dir, name, text string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600))
}

// buildUsageReportTestTree creates a temp directory that mimics a completed
// job output tree:
//
//	<tmpDir>/job-test/out/             ← root node
//	<tmpDir>/job-test/out/child/worker/ ← one child
//
// Returns the tmpDir so the caller can set OutputRoot.
func buildUsageReportTestTree(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	rootDir := filepath.Join(tmpDir, "job-test", "out")
	childDir := filepath.Join(rootDir, "child", testWorkerName)
	require.NoError(t, os.MkdirAll(childDir, 0o750))

	// Root: results.json
	writeJSONFile(t, rootDir, "results.json", map[string]any{
		"job_id":          "job-test",
		childParamName:    "",
		testFieldDepth:    0,
		"exit_code":       0,
		"own_usage_usd":   0,
		"total_usage_usd": 0,
		testFieldChildren: []string{testWorkerName},
	})
	// Root: usage.json — cost 0.05; per_model only haiku; dropped 2+1
	writeJSONFile(t, rootDir, "usage.json", map[string]any{
		testFieldCostUSD: 0.05,
		"per_model": map[string]any{
			"claude-haiku-4-5": map[string]any{testFieldCostUSD: 0.04},
		},
		"dropped": map[string]any{"parse_errors": 2, "no_usage_block": 1},
	})
	// Root: usage.jsonl — req-1 (haiku, attributed) + req-2 (haiku, unattributed)
	writeTextFile(t, rootDir, "usage.jsonl",
		`{"requestId":"req-1","ts":"","model":"claude-haiku-4-5","input":100,"output":50,"cache_creation":0,"cache_read":20}`+"\n"+
			`{"requestId":"req-2","ts":"","model":"claude-haiku-4-5","input":200,"output":80,"cache_creation":0,"cache_read":0}`+"\n",
	)
	// Root: attribution.jsonl — only req-1 attributed to "planner"
	writeTextFile(t, rootDir, "attribution.jsonl",
		`{"requestId":"req-1","attributionAgent":"planner"}`+"\n",
	)

	// Child: results.json
	writeJSONFile(t, childDir, "results.json", map[string]any{
		childParamName:    testWorkerName,
		testFieldDepth:    1,
		testFieldChildren: []string{},
	})
	// Child: usage.json — cost 0.03, sonnet
	writeJSONFile(t, childDir, "usage.json", map[string]any{
		testFieldCostUSD: 0.03,
		"per_model": map[string]any{
			"claude-sonnet-5": map[string]any{testFieldCostUSD: 0.03},
		},
	})
	// Child: usage.jsonl — req-3 (sonnet, unnamed attribution)
	writeTextFile(t, childDir, "usage.jsonl",
		`{"requestId":"req-3","ts":"","model":"claude-sonnet-5","input":300,"output":100,"cache_creation":10,"cache_read":0}`+"\n",
	)
	// Child: attribution.jsonl — req-3 has empty agent → "(unnamed)"
	writeTextFile(t, childDir, "attribution.jsonl",
		`{"requestId":"req-3","attributionAgent":""}`+"\n",
	)

	return tmpDir
}

func TestUsageReport_AggregatesCorrectly(t *testing.T) {
	tmpDir := buildUsageReportTestTree(t)
	r := NewRunner(Config{OutputRoot: tmpDir})

	rep, err := r.UsageReport(UsageReportRequest{JobID: "job-test"})
	require.NoError(t, err)

	// Total cost = root 0.05 + child 0.03
	assert.InDelta(t, 0.08, rep.TotalCostUSD, 1e-9, "TotalCostUSD")

	// Token type totals across all nodes
	assert.Equal(t, int64(600), rep.TokenTypeTotals.Input, "Input tokens")
	assert.Equal(t, int64(230), rep.TokenTypeTotals.Output, "Output tokens")
	assert.Equal(t, int64(10), rep.TokenTypeTotals.CacheCreation, "CacheCreation tokens")
	assert.Equal(t, int64(20), rep.TokenTypeTotals.CacheRead, "CacheRead tokens")

	// CacheReadPct = 20 / (600 + 10 + 20) * 100 ≈ 3.17
	assert.InDelta(t, 3.17, rep.CacheReadPct, 0.01, "CacheReadPct")

	// ByModel: two models
	require.Len(t, rep.ByModel, 2, "ByModel count")
	haikuIdx, sonnetIdx := 0, 1
	if rep.ByModel[0].Model != "claude-haiku-4-5" {
		haikuIdx, sonnetIdx = 1, 0
	}
	haiku := rep.ByModel[haikuIdx]
	assert.Equal(t, "claude-haiku-4-5", haiku.Model)
	assert.Equal(t, int64(300), haiku.Input)
	assert.Equal(t, int64(130), haiku.Output)
	assert.Equal(t, int64(0), haiku.CacheCreation)
	assert.Equal(t, int64(20), haiku.CacheRead)
	assert.InDelta(t, 0.04, haiku.CostUSD, 1e-9, "haiku cost from usage.json")

	sonnet := rep.ByModel[sonnetIdx]
	assert.Equal(t, "claude-sonnet-5", sonnet.Model)
	assert.Equal(t, int64(300), sonnet.Input)
	assert.Equal(t, int64(100), sonnet.Output)
	assert.Equal(t, int64(10), sonnet.CacheCreation)
	assert.InDelta(t, 0.03, sonnet.CostUSD, 1e-9, "sonnet cost from usage.json")

	// ByChild: one entry per visited node (root + worker)
	require.Len(t, rep.ByChild, 2, "ByChild count")
	var rootChild, workerChild ChildUsage
	for _, c := range rep.ByChild {
		if c.Name == testWorkerName {
			workerChild = c
		} else {
			rootChild = c
		}
	}
	assert.Empty(t, rootChild.Name)
	assert.Equal(t, 0, rootChild.Depth)
	assert.InDelta(t, 0.05, rootChild.CostUSD, 1e-9)
	assert.Equal(t, int64(300), rootChild.Input)

	assert.Equal(t, testWorkerName, workerChild.Name)
	assert.Equal(t, 1, workerChild.Depth)
	assert.InDelta(t, 0.03, workerChild.CostUSD, 1e-9)
	assert.Equal(t, int64(300), workerChild.Input)
	assert.Equal(t, int64(10), workerChild.CacheCreation)

	// Dropped: root only (child has no dropped block)
	assert.Equal(t, int64(2), rep.Dropped.ParseErrors, "Dropped.ParseErrors")
	assert.Equal(t, int64(1), rep.Dropped.NoUsageBlock, "Dropped.NoUsageBlock")

	// BySubagent attribution
	require.NotEmpty(t, rep.BySubagent, "BySubagent must not be empty")
	byName := make(map[string]SubagentUsage, len(rep.BySubagent))
	for _, sa := range rep.BySubagent {
		byName[sa.Name] = sa
	}

	// req-1 attributed to "planner"
	planner, ok := byName["planner"]
	require.True(t, ok, "expected 'planner' bucket")
	assert.Equal(t, 1, planner.Requests)

	// req-2 has no attribution → "(main)"
	main, ok := byName["(main)"]
	require.True(t, ok, "expected '(main)' bucket for unattributed spend")
	assert.Equal(t, 1, main.Requests, "(main) should contain req-2")

	// req-3 has empty attributionAgent → "(unnamed)"
	unnamed, ok := byName["(unnamed)"]
	require.True(t, ok, "expected '(unnamed)' bucket for empty agent")
	assert.Equal(t, 1, unnamed.Requests)
}

func TestUsageReport_NoDroppedWhenFileAbsent(t *testing.T) {
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "noop-job", "out")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	// Write only results.json with no children; all other files absent.
	writeJSONFile(t, dir, "results.json", map[string]any{
		childParamName: "", testFieldDepth: 0, testFieldChildren: []string{},
	})

	r := NewRunner(Config{OutputRoot: tmpDir})
	rep, err := r.UsageReport(UsageReportRequest{JobID: "noop-job"})
	require.NoError(t, err)
	assert.InDelta(t, 0.0, rep.TotalCostUSD, 1e-9)
	assert.Equal(t, int64(0), rep.Dropped.ParseErrors)
	assert.Equal(t, int64(0), rep.Dropped.NoUsageBlock)
	assert.Empty(t, rep.ByModel)
	assert.Empty(t, rep.BySubagent)
}

func TestUsageReport_OutputRootEscapeRejected(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRunner(Config{OutputRoot: tmpDir})

	// Path that escapes OutputRoot via ..
	outsidePath := filepath.Join(tmpDir, "..", "outside")
	_, err := r.UsageReport(UsageReportRequest{OutputDir: outsidePath})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside OutputRoot")
}

func TestUsageReport_OutputDirEqualToRootAllowed(t *testing.T) {
	tmpDir := t.TempDir()
	// OutputRoot itself is allowed (edge case: dir == root).
	r := NewRunner(Config{OutputRoot: tmpDir})
	// No results.json → returns empty report without error.
	_, err := r.UsageReport(UsageReportRequest{OutputDir: tmpDir})
	require.NoError(t, err)
}

func TestUsageReport_MissingJobIDAndOutputDir(t *testing.T) {
	r := NewRunner(Config{OutputRoot: "/tmp/root"})
	_, err := r.UsageReport(UsageReportRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}
