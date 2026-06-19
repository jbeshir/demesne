package sandbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeChildResultsFull writes a results.json for a child with optional
// PerModelTokens under outHost/child/<name>.
func writeChildResultsFull(t *testing.T, outHost, name string, total float64, perModel map[string]TokenTotals) {
	t.Helper()
	dir := filepath.Join(outHost, "child", name)
	require.NoError(t, os.MkdirAll(dir, 0o750))
	data, err := json.Marshal(Results{Name: name, TotalUsageUSD: total, PerModelTokens: perModel})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "results.json"), data, 0o600))
}

// writeChildResults writes a results.json for a child named under
// outHost/child/<name>.
func writeChildResults(t *testing.T, outHost, name string, total float64) {
	t.Helper()
	dir := filepath.Join(outHost, "child", name)
	require.NoError(t, os.MkdirAll(dir, 0o750))
	data, err := json.Marshal(Results{Name: name, TotalUsageUSD: total})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "results.json"), data, 0o600))
}

func TestWriteResults_RollsUpChildren(t *testing.T) {
	out := t.TempDir()
	writeChildResults(t, out, testChildAlpha, 0.20)
	writeChildResults(t, out, testChildBeta, 0.05)

	layout := sandboxLayout{jobID: JobID("job-1"), outHost: out}
	total, _ := writeResults(layout, "sandbox_agent", 0, 0.10)

	assert.InDelta(t, 0.35, total, 1e-9)

	r, ok := readResultsFile(out)
	require.True(t, ok)
	assert.Equal(t, JobID("job-1"), r.JobID)
	assert.Equal(t, "sandbox_agent", r.Tool)
	assert.InDelta(t, 0.10, r.OwnUsageUSD, 1e-9)
	assert.InDelta(t, 0.35, r.TotalUsageUSD, 1e-9)
	assert.Equal(t, []string{testChildAlpha, testChildBeta}, r.Children)
}

func TestWriteResults_NoChildren(t *testing.T) {
	out := t.TempDir()
	layout := sandboxLayout{jobID: JobID("job-2"), outHost: out, childName: "leaf", depth: 2}
	total, _ := writeResults(layout, "sandbox_research", 0, 0.42)

	assert.InDelta(t, 0.42, total, 1e-9)
	r, ok := readResultsFile(out)
	require.True(t, ok)
	assert.Equal(t, "leaf", r.Name)
	assert.Equal(t, 2, r.Depth)
	assert.Empty(t, r.Children)
	assert.InDelta(t, 0.42, r.TotalUsageUSD, 1e-9)
}

func TestSumChildResults_SkipsMalformed(t *testing.T) {
	out := t.TempDir()
	writeChildResults(t, out, "good", 0.10)
	bad := filepath.Join(out, "child", "bad")
	require.NoError(t, os.MkdirAll(bad, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(bad, "results.json"), []byte("{not json"), 0o600))

	names, total := sumChildResults(out)
	assert.Equal(t, []string{"good"}, names)
	assert.InDelta(t, 0.10, total, 1e-9)
}

func TestReadPerModelTokens_AbsentFile(t *testing.T) {
	out := t.TempDir()
	result := readPerModelTokens(out)
	assert.Nil(t, result)
}

func TestReadPerModelTokens_SumsPerModel(t *testing.T) {
	out := t.TempDir()
	lines := "" +
		`{"requestId":"r1","ts":"2026-01-01T00:00:00Z","model":"claude-3-5","input":100,"output":200,"cache_creation":50,"cache_read":10}` + "\n" +
		`{"requestId":"r2","ts":"2026-01-01T00:00:01Z","model":"claude-3-5","input":30,"output":40,"cache_creation":0,"cache_read":5}` + "\n" +
		`{"requestId":"r3","ts":"2026-01-01T00:00:02Z","model":"gpt-4o","input":10,"output":20,"cache_creation":0,"cache_read":3}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(out, "usage.jsonl"), []byte(lines), 0o600))

	result := readPerModelTokens(out)
	require.NotNil(t, result)
	assert.Equal(t, TokenTotals{Input: 130, Output: 240, CacheCreation: 50, CacheRead: 15}, result["claude-3-5"])
	assert.Equal(t, TokenTotals{Input: 10, Output: 20, CacheCreation: 0, CacheRead: 3}, result["gpt-4o"])
}

func TestReadPerModelTokens_SkipsMalformedLines(t *testing.T) {
	out := t.TempDir()
	lines := "" +
		`{"requestId":"r1","ts":"2026-01-01T00:00:00Z","model":"model-a","input":5,"output":10,"cache_creation":0,"cache_read":0}` + "\n" +
		`{not valid json}` + "\n" +
		`{"requestId":"r3","ts":"2026-01-01T00:00:02Z","model":"model-a","input":3,"output":7,"cache_creation":0,"cache_read":0}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(out, "usage.jsonl"), []byte(lines), 0o600))

	result := readPerModelTokens(out)
	require.NotNil(t, result)
	assert.Equal(t, TokenTotals{Input: 8, Output: 17}, result["model-a"])
}

func TestWriteResults_RollsUpPerModelTokens(t *testing.T) {
	out := t.TempDir()

	// Own usage.jsonl with two requests for model-a.
	lines := "" +
		`{"requestId":"r1","ts":"2026-01-01T00:00:00Z","model":"model-a","input":100,"output":200,"cache_creation":0,"cache_read":0}` + "\n" +
		`{"requestId":"r2","ts":"2026-01-01T00:00:01Z","model":"model-a","input":20,"output":30,"cache_creation":5,"cache_read":2}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(out, "usage.jsonl"), []byte(lines), 0o600))

	// Child with tokens in model-a and model-b.
	writeChildResultsFull(t, out, "child1", 0.01, map[string]TokenTotals{
		"model-a": {Input: 50, Output: 100, CacheCreation: 10, CacheRead: 5},
		"model-b": {Input: 20, Output: 30},
	})

	layout := sandboxLayout{jobID: JobID("job-pt"), outHost: out}
	_, perModel := writeResults(layout, "sandbox_agent", 0, 0.01)

	require.NotNil(t, perModel)
	// own model-a: 120 in, 230 out, 5 cc, 2 cr; child model-a: 50 in, 100 out, 10 cc, 5 cr → sum
	assert.Equal(t, TokenTotals{Input: 170, Output: 330, CacheCreation: 15, CacheRead: 7}, perModel["model-a"])
	assert.Equal(t, TokenTotals{Input: 20, Output: 30}, perModel["model-b"])

	// results.json must carry the same map.
	r, ok := readResultsFile(out)
	require.True(t, ok)
	assert.Equal(t, perModel, r.PerModelTokens)
}

func TestWriteResults_ZeroUsageOmitsPerModelTokens(t *testing.T) {
	out := t.TempDir()
	layout := sandboxLayout{jobID: JobID("job-zero"), outHost: out}
	_, perModel := writeResults(layout, "sandbox_agent", 0, 0.0)

	assert.Nil(t, perModel)

	r, ok := readResultsFile(out)
	require.True(t, ok)
	assert.Nil(t, r.PerModelTokens)

	// per_model_tokens must not appear in the marshaled JSON.
	data, err := os.ReadFile(filepath.Join(out, "results.json")) //nolint:gosec // path under t.TempDir()
	require.NoError(t, err)
	assert.NotContains(t, string(data), "per_model_tokens")
}
