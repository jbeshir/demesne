package sandbox

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// TokenTotals is the four-token-type breakdown for a single model,
// aggregated across the whole descendant tree (own run plus children).
type TokenTotals struct {
	Input         int64 `json:"input"`
	Output        int64 `json:"output"`
	CacheCreation int64 `json:"cache_creation"`
	CacheRead     int64 `json:"cache_read"`
}

// Results is the per-job summary written to <out>/results.json after
// an agent run. own_usage_usd is the indicative spend of this run
// alone; total_usage_usd adds the total of every descendant (children
// nest under <out>/child/<name>), so the root's results.json carries
// the whole tree's cost.
type Results struct {
	JobID          JobID                  `json:"job_id"`
	Tool           string                 `json:"tool"`
	Name           string                 `json:"name,omitempty"`
	Depth          int                    `json:"depth"`
	ExitCode       int                    `json:"exit_code"`
	OwnUsageUSD    float64                `json:"own_usage_usd"`
	TotalUsageUSD  float64                `json:"total_usage_usd"`
	Children       []string               `json:"children,omitempty"`
	PerModelTokens map[string]TokenTotals `json:"per_model_tokens,omitempty"`
}

// writeResults composes this run's results.json (rolling up any
// descendant results already written under <out>/child/<name>) and
// writes it atomically to outHost. Returns total_usage_usd and the
// tree-aggregated per-model token breakdown. Children always finish
// before their parent's tool call returns, so their results.json files
// exist by the time we roll up. Best-effort: a write failure is
// non-fatal (the headline cost is also in the tool result), so the
// returned total is still accurate.
func writeResults(layout sandboxLayout, tool string, exitCode int, ownUSD float64) (float64, map[string]TokenTotals) {
	children, childTotal := sumChildResults(layout.outHost)
	perModel := addPerModelTokens(readPerModelTokens(layout.outHost), sumChildPerModelTokens(layout.outHost))
	res := Results{
		JobID:          layout.jobID,
		Tool:           tool,
		Name:           layout.childName,
		Depth:          layout.depth,
		ExitCode:       exitCode,
		OwnUsageUSD:    ownUSD,
		TotalUsageUSD:  ownUSD + childTotal,
		Children:       children,
		PerModelTokens: perModel,
	}
	writeResultsFile(layout.outHost, res)
	return res.TotalUsageUSD, perModel
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

// readPerModelTokens reads outHost/usage.jsonl and sums the token counts
// per model. The file is written by the phase-01 proxy tracker; each line
// is a JSON object with model, input, output, cache_creation, cache_read
// fields. Missing file returns nil (normal when no requests were made).
// A present-but-unreadable file is logged once and also returns nil.
// Malformed individual lines are skipped silently (best-effort).
func readPerModelTokens(outHost string) map[string]TokenTotals {
	data, err := readOutputFile(outHost, "usage.jsonl")
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("sandbox: read usage.jsonl: %v", err)
		}
		return nil
	}
	var result map[string]TokenTotals
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		var rec struct {
			Model         string `json:"model"`
			Input         int64  `json:"input"`
			Output        int64  `json:"output"`
			CacheCreation int64  `json:"cache_creation"`
			CacheRead     int64  `json:"cache_read"`
		}
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if result == nil {
			result = make(map[string]TokenTotals)
		}
		t := result[rec.Model]
		t.Input += rec.Input
		t.Output += rec.Output
		t.CacheCreation += rec.CacheCreation
		t.CacheRead += rec.CacheRead
		result[rec.Model] = t
	}
	return result
}

// sumChildPerModelTokens reads each child's results.json and sums the
// PerModelTokens fields across all children of outHost.
func sumChildPerModelTokens(outHost string) map[string]TokenTotals {
	childRoot := filepath.Join(outHost, "child")
	entries, err := os.ReadDir(childRoot)
	if err != nil {
		return nil
	}
	var result map[string]TokenTotals
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		r, ok := readResultsFile(filepath.Join(childRoot, e.Name()))
		if !ok {
			continue
		}
		result = addPerModelTokens(result, r.PerModelTokens)
	}
	return result
}

// addPerModelTokens merges two per-model token maps by summing each
// token type per model. Returns nil when both inputs are empty.
func addPerModelTokens(a, b map[string]TokenTotals) map[string]TokenTotals {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	result := make(map[string]TokenTotals, len(a)+len(b))
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		t := result[k]
		t.Input += v.Input
		t.Output += v.Output
		t.CacheCreation += v.CacheCreation
		t.CacheRead += v.CacheRead
		result[k] = t
	}
	return result
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
		log.Printf("sandbox: write results.json: marshal: %v", err)
		return
	}
	tmp := filepath.Join(outHost, ".results.json.tmp")
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		log.Printf("sandbox: write results.json: write tmp: %v", err)
		return
	}
	if err := os.Rename(tmp, filepath.Join(outHost, "results.json")); err != nil {
		log.Printf("sandbox: write results.json: rename: %v", err)
	}
}
