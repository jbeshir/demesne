package sandbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// Results is the per-job summary written to <out>/results.json after
// an agent run. own_usage_usd is the indicative spend of this run
// alone; total_usage_usd adds the total of every descendant (children
// nest under <out>/child/<name>), so the root's results.json carries
// the whole tree's cost.
type Results struct {
	JobID         JobID    `json:"job_id"`
	Tool          string   `json:"tool"`
	Name          string   `json:"name,omitempty"`
	Depth         int      `json:"depth"`
	ExitCode      int      `json:"exit_code"`
	OwnUsageUSD   float64  `json:"own_usage_usd"`
	TotalUsageUSD float64  `json:"total_usage_usd"`
	Children      []string `json:"children,omitempty"`
}

// writeResults composes this run's results.json (rolling up any
// descendant results already written under <out>/child/<name>) and
// writes it atomically to outHost. Returns total_usage_usd. Children
// always finish before their parent's tool call returns, so their
// results.json files exist by the time we roll up. Best-effort: a
// write failure is non-fatal (the headline cost is also in the tool
// result), so the returned total is still accurate.
func writeResults(layout sandboxLayout, tool string, exitCode int, ownUSD float64) float64 {
	children, childTotal := sumChildResults(layout.outHost)
	res := Results{
		JobID:         layout.jobID,
		Tool:          tool,
		Name:          layout.childName,
		Depth:         layout.depth,
		ExitCode:      exitCode,
		OwnUsageUSD:   ownUSD,
		TotalUsageUSD: ownUSD + childTotal,
		Children:      children,
	}
	writeResultsFile(layout.outHost, res)
	return res.TotalUsageUSD
}

// sumChildResults reads every <outHost>/child/<name>/results.json,
// returning the child names (sorted) and the sum of their
// total_usage_usd. Missing or malformed child results are skipped.
func sumChildResults(outHost string) ([]string, float64) {
	childRoot := filepath.Join(outHost, "child")
	entries, err := os.ReadDir(childRoot)
	if err != nil {
		return nil, 0
	}
	var names []string
	var total float64
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		r, ok := readResultsFile(filepath.Join(childRoot, e.Name()))
		if !ok {
			continue
		}
		names = append(names, e.Name())
		total += r.TotalUsageUSD
	}
	sort.Strings(names)
	return names, total
}

// readResultsFile reads results.json from dir. The dir is composed
// from runner-controlled paths under r.cfg.OutputRoot.
func readResultsFile(dir string) (Results, bool) {
	data, err := readOutputFile(dir, "results.json")
	if err != nil {
		return Results{}, false
	}
	var r Results
	if err := json.Unmarshal(data, &r); err != nil {
		return Results{}, false
	}
	return r, true
}

// writeResultsFile atomically writes results.json to outHost (write to
// .tmp then rename). Best-effort: errors are dropped.
func writeResultsFile(outHost string, res Results) {
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return
	}
	tmp := filepath.Join(outHost, ".results.json.tmp")
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, filepath.Join(outHost, "results.json"))
}
