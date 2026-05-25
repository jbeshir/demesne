package sandbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	layout := sandboxLayout{jobID: "job-1", outHost: out}
	total := writeResults(layout, "sandbox_agent", 0, 0.10)

	assert.InDelta(t, 0.35, total, 1e-9)

	r, ok := readResultsFile(out)
	require.True(t, ok)
	assert.Equal(t, "job-1", r.JobID)
	assert.Equal(t, "sandbox_agent", r.Tool)
	assert.InDelta(t, 0.10, r.OwnUsageUSD, 1e-9)
	assert.InDelta(t, 0.35, r.TotalUsageUSD, 1e-9)
	assert.Equal(t, []string{testChildAlpha, testChildBeta}, r.Children)
}

func TestWriteResults_NoChildren(t *testing.T) {
	out := t.TempDir()
	layout := sandboxLayout{jobID: "job-2", outHost: out, childName: "leaf", depth: 2}
	total := writeResults(layout, "sandbox_research", 0, 0.42)

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
