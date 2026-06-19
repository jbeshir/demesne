package sandbox

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jbeshir/demesne/internal/proxies/anthropic"
)

// UsageReportRequest identifies the job output tree to report on.
// At least one of JobID or OutputDir must be non-empty.
type UsageReportRequest struct {
	// JobID is the demesne job UUID. When OutputDir is empty the root
	// output directory is resolved as OutputRoot/<JobID>/out.
	JobID JobID
	// OutputDir is the explicit host path of the /out root to report on.
	// Takes precedence over JobID when non-empty.
	OutputDir string
}

// DroppedCounts summarises requests whose usage could not be recorded,
// summed from every node in the output tree.
type DroppedCounts struct {
	ParseErrors  int64 `json:"parse_errors"`
	NoUsageBlock int64 `json:"no_usage_block"`
}

// ModelUsage is the per-model token and cost breakdown in a UsageReport.
type ModelUsage struct {
	// Model is the model ID as reported in usage.jsonl (e.g. "claude-sonnet-4-6").
	Model string
	TokenTotals
	// CostUSD is the vendor-reported cost taken from usage.json (authoritative).
	CostUSD float64
}

// ChildUsage is the per-node breakdown in a UsageReport (one entry per
// visited node in the output tree, including the root).
type ChildUsage struct {
	// Name is the child's name from results.json (empty for root nodes).
	Name string
	// Depth is the nesting depth from results.json (0 for root).
	Depth int
	// CostUSD is this node's own vendor-reported cost from usage.json.
	CostUSD float64
	TokenTotals
}

// SubagentUsage is the per-subagent attribution breakdown in a UsageReport.
// Costs are indicative list-price computed via anthropic.CostUSD (not the
// authoritative vendor total), and attribution is available only for
// claude-code runs (requires attribution.jsonl). Residual spend that could
// not be attributed to a named subagent — including all non-anthropic requests
// and any rounding difference — is bucketed into "(main)". Attribution records
// with an empty agent field are bucketed into "(unnamed)".
type SubagentUsage struct {
	// Name is the attributionAgent from attribution.jsonl, or "(main)"
	// for unattributed requests, or "(unnamed)" for records with an
	// empty agent field.
	Name string
	TokenTotals
	// CostUSD is the indicative spend for this bucket computed from
	// tokens via anthropic.CostUSD, with residual reconciled into "(main)".
	CostUSD float64
	// Requests is the number of usage.jsonl records attributed to this bucket.
	Requests int
}

// UsageReport is the output of Runner.UsageReport.
type UsageReport struct {
	// TotalCostUSD is the sum of vendor-reported costs across the whole tree.
	TotalCostUSD float64
	// TokenTypeTotals is the tree-wide four-token-type sum from usage.jsonl.
	TokenTypeTotals TokenTotals
	// CacheReadPct is cache_read / (input + cache_creation + cache_read) × 100.
	CacheReadPct float64
	// ByModel is the per-model token and cost breakdown, sorted by model ID.
	ByModel []ModelUsage
	// ByChild is the per-node cost and token breakdown (one entry per node).
	ByChild []ChildUsage
	// BySubagent is the per-subagent attribution breakdown.
	BySubagent []SubagentUsage
	// Dropped is the aggregate dropped-usage counters across all nodes.
	Dropped DroppedCounts
}

// usageReportAccum holds the mutable accumulators used while walking the tree.
type usageReportAccum struct {
	totalCostUSD  float64
	byModelTokens map[string]TokenTotals
	byModelCost   map[string]float64
	byChild       []ChildUsage
	bySubagent    map[string]*subagentEntry
	totalComputed float64 // sum of per-request anthropic.CostUSD across all nodes
	dropped       DroppedCounts
}

type subagentEntry struct {
	tokens   TokenTotals
	cost     float64
	requests int
}

// UsageReport walks the output tree rooted at the directory resolved from
// req, reads per-node usage.json, usage.jsonl, and attribution.jsonl, and
// returns an aggregated cost and token breakdown.
//
// Cost totals come from usage.json (authoritative vendor figures).
// Token breakdowns come from usage.jsonl. Subagent attribution is available
// only for claude-code runs (requires attribution.jsonl). Per-subagent costs
// are indicative list-price computed from tokens via anthropic.CostUSD;
// residual spend is reconciled into "(main)". Missing files at a node are
// skipped without error.
func (r *Runner) UsageReport(req UsageReportRequest) (UsageReport, error) {
	rootDir, err := r.resolveUsageReportDir(req)
	if err != nil {
		return UsageReport{}, err
	}

	acc := &usageReportAccum{
		byModelTokens: make(map[string]TokenTotals),
		byModelCost:   make(map[string]float64),
		bySubagent:    make(map[string]*subagentEntry),
	}
	walkUsageNode(rootDir, acc)

	return buildUsageReport(acc), nil
}

// resolveUsageReportDir returns the cleaned absolute path of the /out root
// from req, enforcing that it sits inside r.cfg.OutputRoot.
func (r *Runner) resolveUsageReportDir(req UsageReportRequest) (string, error) {
	var raw string
	if req.OutputDir != "" {
		raw = req.OutputDir
	} else {
		if req.JobID == "" {
			return "", fmt.Errorf("job_id or output_dir is required")
		}
		raw = filepath.Join(r.cfg.OutputRoot, string(req.JobID), "out")
	}

	cleanRoot := filepath.Clean(r.cfg.OutputRoot)
	cleanDir := filepath.Clean(raw)
	if cleanDir != cleanRoot && !strings.HasPrefix(cleanDir, cleanRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("output_dir %q is outside OutputRoot %q", cleanDir, r.cfg.OutputRoot)
	}
	return cleanDir, nil
}

// processUsageJSON reads usage.json at dir and accumulates cost and dropped
// counters into acc. Returns the node's own cost.
func processUsageJSON(dir string, acc *usageReportAccum) float64 {
	raw, err := readOutputFile(dir, "usage.json")
	if err != nil {
		return 0
	}
	var u struct {
		CostUSD  float64                    `json:"cost_usd"`
		PerModel map[string]json.RawMessage `json:"per_model"`
		Dropped  *struct {
			ParseErrors  int64 `json:"parse_errors"`
			NoUsageBlock int64 `json:"no_usage_block"`
		} `json:"dropped,omitempty"`
	}
	if json.Unmarshal(raw, &u) != nil {
		return 0
	}
	acc.totalCostUSD += u.CostUSD
	for model, mr := range u.PerModel {
		var me struct {
			CostUSD float64 `json:"cost_usd"`
		}
		if json.Unmarshal(mr, &me) == nil {
			acc.byModelCost[model] += me.CostUSD
		}
	}
	if u.Dropped != nil {
		acc.dropped.ParseErrors += u.Dropped.ParseErrors
		acc.dropped.NoUsageBlock += u.Dropped.NoUsageBlock
	}
	return u.CostUSD
}

// processUsageJSONL reads usage.jsonl at dir, accumulates per-model token
// totals and subagent attribution into acc, and returns the node-local totals.
func processUsageJSONL(dir string, acc *usageReportAccum) TokenTotals {
	raw, err := readOutputFile(dir, "usage.jsonl")
	if err != nil {
		return TokenTotals{}
	}
	attrib := readAttributionMap(dir)
	var nodeTokenTotals TokenTotals
	for _, line := range strings.Split(string(raw), "\n") {
		if line == "" {
			continue
		}
		var rec struct {
			RequestID     string `json:"requestId"`
			Model         string `json:"model"`
			Input         int64  `json:"input"`
			Output        int64  `json:"output"`
			CacheCreation int64  `json:"cache_creation"`
			CacheRead     int64  `json:"cache_read"`
		}
		if json.Unmarshal([]byte(line), &rec) != nil {
			continue
		}

		// Accumulate tree-wide per-model token totals.
		mt := acc.byModelTokens[rec.Model]
		mt.Input += rec.Input
		mt.Output += rec.Output
		mt.CacheCreation += rec.CacheCreation
		mt.CacheRead += rec.CacheRead
		acc.byModelTokens[rec.Model] = mt

		// Accumulate node-local token totals (for this ByChild entry).
		nodeTokenTotals.Input += rec.Input
		nodeTokenTotals.Output += rec.Output
		nodeTokenTotals.CacheCreation += rec.CacheCreation
		nodeTokenTotals.CacheRead += rec.CacheRead

		// Indicative per-request cost for subagent attribution.
		tc := anthropic.TokenCounts{
			InputTokens:              rec.Input,
			OutputTokens:             rec.Output,
			CacheCreationInputTokens: rec.CacheCreation,
			CacheReadInputTokens:     rec.CacheRead,
		}
		reqCost := float64(anthropic.CostUSD(anthropic.ModelID(rec.Model), tc))
		acc.totalComputed += reqCost

		// Determine attribution bucket.
		agentName := subagentMain
		if ar, ok := attrib[rec.RequestID]; ok {
			if ar.AttributionAgent != "" {
				agentName = ar.AttributionAgent
			} else {
				agentName = "(unnamed)"
			}
		}

		sa := acc.bySubagent[agentName]
		if sa == nil {
			sa = &subagentEntry{}
			acc.bySubagent[agentName] = sa
		}
		sa.tokens.Input += rec.Input
		sa.tokens.Output += rec.Output
		sa.tokens.CacheCreation += rec.CacheCreation
		sa.tokens.CacheRead += rec.CacheRead
		sa.cost += reqCost
		sa.requests++
	}
	return nodeTokenTotals
}

// walkUsageNode reads the four output files at dir, accumulates their data
// into acc, and recurses into every child listed in results.json. Missing
// files at a node are silently skipped.
func walkUsageNode(dir string, acc *usageReportAccum) {
	results, hasResults := readResultsFile(dir)

	nodeCostUSD := processUsageJSON(dir, acc)
	nodeTokenTotals := processUsageJSONL(dir, acc)

	// Record one ByChild entry for this node.
	name := ""
	depth := 0
	if hasResults {
		name = results.Name
		depth = results.Depth
	}
	acc.byChild = append(acc.byChild, ChildUsage{
		Name:        name,
		Depth:       depth,
		CostUSD:     nodeCostUSD,
		TokenTotals: nodeTokenTotals,
	})

	// Recurse into children listed in results.json.
	if hasResults {
		for _, childName := range results.Children {
			walkUsageNode(filepath.Join(dir, "child", childName), acc)
		}
	}
}

// buildUsageReport converts the accumulated data into a UsageReport.
func buildUsageReport(acc *usageReportAccum) UsageReport {
	// Tree-wide token totals from all per-model entries.
	var totalTokens TokenTotals
	for _, t := range acc.byModelTokens {
		totalTokens.Input += t.Input
		totalTokens.Output += t.Output
		totalTokens.CacheCreation += t.CacheCreation
		totalTokens.CacheRead += t.CacheRead
	}

	// CacheReadPct: cache_read / (input + cache_creation + cache_read).
	inputSide := totalTokens.Input + totalTokens.CacheCreation + totalTokens.CacheRead
	var cacheReadPct float64
	if inputSide > 0 {
		cacheReadPct = float64(totalTokens.CacheRead) / float64(inputSide) * 100
	}

	// ByModel: union of keys from both token and cost maps, sorted by model ID.
	modelKeys := make(map[string]bool, len(acc.byModelTokens)+len(acc.byModelCost))
	for m := range acc.byModelTokens {
		modelKeys[m] = true
	}
	for m := range acc.byModelCost {
		modelKeys[m] = true
	}
	models := make([]string, 0, len(modelKeys))
	for m := range modelKeys {
		models = append(models, m)
	}
	sort.Strings(models)
	byModel := make([]ModelUsage, len(models))
	for i, m := range models {
		byModel[i] = ModelUsage{
			Model:       m,
			TokenTotals: acc.byModelTokens[m],
			CostUSD:     acc.byModelCost[m],
		}
	}

	// Reconcile subagent costs: bucket the difference between the authoritative
	// total and the sum of per-request computed costs into "(main)". This
	// ensures no spend is lost (e.g. non-anthropic requests, rounding gaps).
	residual := acc.totalCostUSD - acc.totalComputed
	if residual != 0 {
		sa := acc.bySubagent[subagentMain]
		if sa == nil {
			sa = &subagentEntry{}
			acc.bySubagent[subagentMain] = sa
		}
		sa.cost += residual
	}

	// BySubagent: sorted by cost descending, then by name ascending.
	agentNames := make([]string, 0, len(acc.bySubagent))
	for name := range acc.bySubagent {
		agentNames = append(agentNames, name)
	}
	sort.Slice(agentNames, func(i, j int) bool {
		ci := acc.bySubagent[agentNames[i]].cost
		cj := acc.bySubagent[agentNames[j]].cost
		if ci != cj {
			return ci > cj
		}
		return agentNames[i] < agentNames[j]
	})
	bySubagent := make([]SubagentUsage, 0, len(agentNames))
	for _, name := range agentNames {
		sa := acc.bySubagent[name]
		bySubagent = append(bySubagent, SubagentUsage{
			Name:        name,
			TokenTotals: sa.tokens,
			CostUSD:     sa.cost,
			Requests:    sa.requests,
		})
	}

	return UsageReport{
		TotalCostUSD:    acc.totalCostUSD,
		TokenTypeTotals: totalTokens,
		CacheReadPct:    cacheReadPct,
		ByModel:         byModel,
		ByChild:         acc.byChild,
		BySubagent:      bySubagent,
		Dropped:         acc.dropped,
	}
}
