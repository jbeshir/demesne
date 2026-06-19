package sandbox

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// AttributionRecord is one line in outHost/attribution.jsonl: the
// requestId join key plus whichever attribution fields were non-empty
// in the source transcript entry.
type AttributionRecord struct {
	RequestID         string `json:"requestId"`
	AttributionAgent  string `json:"attributionAgent,omitempty"`
	AttributionSkill  string `json:"attributionSkill,omitempty"`
	AttributionPlugin string `json:"attributionPlugin,omitempty"`
}

// transcriptLine is the subset of a claude-code JSONL entry we parse
// for attribution distillation.
type transcriptLine struct {
	Type              string `json:"type"`
	RequestID         string `json:"requestId"`
	AttributionAgent  string `json:"attributionAgent,omitempty"`
	AttributionSkill  string `json:"attributionSkill,omitempty"`
	AttributionPlugin string `json:"attributionPlugin,omitempty"`
	Message           tlMsg  `json:"message"`
}

type tlMsg struct {
	ID         string  `json:"id"`
	StopReason *string `json:"stop_reason"` // null during streaming
	Usage      tlUsage `json:"usage"`
}

type tlUsage struct {
	OutputTokens int64 `json:"output_tokens"`
}

// subagentMain is the attribution bucket for unattributed (main-thread) requests.
const subagentMain = "(main)"

// tlDedupKey is the dedup key for transcript entries.
type tlDedupKey struct {
	requestID string
	messageID string
}

type tlDedupVal struct {
	rec          AttributionRecord
	outputTokens int64
}

// distillAttribution walks the .demesne-attrib scratch directory under
// the agent's private workspace subdir, parses all *.jsonl files
// recursively (capturing both the main session and any nested subagent
// JSONLs), and writes compact attribution records to
// outHost/attribution.jsonl. One record per deduplicated requestId.
//
// Rules:
//   - Only "assistant" entries with a non-null stop_reason and a non-empty
//     requestId contribute.
//   - Dedup by (requestId, message.id); keep the entry with the highest
//     message.usage.output_tokens (the final complete streaming entry).
//   - Absent raw dir (e.g. codex runs, or no .demesne-attrib) → no-op.
//
// All errors are logged non-fatally; the run is never failed.
func distillAttribution(workspaceHost string, jobID JobID, outHost string) {
	rawDir := filepath.Join(workspaceHost, agentCwdSubdir(jobID), ".demesne-attrib")
	if _, err := os.Stat(rawDir); os.IsNotExist(err) {
		return
	}

	jsonlFiles, walkErr := collectJSONLFiles(rawDir)
	if walkErr != nil {
		log.Printf("sandbox: distillAttribution: walk %s: %v", rawDir, walkErr)
		return
	}

	seen := make(map[tlDedupKey]tlDedupVal)
	for _, path := range jsonlFiles {
		parseTranscriptJSONL(path, seen)
	}

	if len(seen) == 0 {
		return
	}

	// Sort for deterministic output.
	keys := make([]tlDedupKey, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].requestID != keys[j].requestID {
			return keys[i].requestID < keys[j].requestID
		}
		return keys[i].messageID < keys[j].messageID
	})

	outPath := filepath.Join(outHost, "attribution.jsonl")
	f, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec // outHost runner-composed
	if err != nil {
		log.Printf("sandbox: distillAttribution: create %s: %v", outPath, err)
		return
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)
	for _, k := range keys {
		if err := enc.Encode(seen[k].rec); err != nil {
			log.Printf("sandbox: distillAttribution: encode: %v", err)
		}
	}
}

// collectJSONLFiles returns all *.jsonl file paths found recursively under dir.
func collectJSONLFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// parseTranscriptJSONL reads one JSONL file and merges qualifying entries
// into the seen dedup map.
func parseTranscriptJSONL(path string, seen map[tlDedupKey]tlDedupVal) {
	f, err := os.Open(path) //nolint:gosec // path comes from filepath.Walk under rawDir
	if err != nil {
		log.Printf("sandbox: distillAttribution: open %s: %v", path, err)
		return
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 4*1024*1024), 4*1024*1024) // allow large transcript lines
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		var entry transcriptLine
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // skip malformed lines
		}
		if entry.Type != "assistant" || entry.RequestID == "" || entry.Message.StopReason == nil {
			continue
		}
		key := tlDedupKey{requestID: entry.RequestID, messageID: entry.Message.ID}
		rec := AttributionRecord{
			RequestID:         entry.RequestID,
			AttributionAgent:  entry.AttributionAgent,
			AttributionSkill:  entry.AttributionSkill,
			AttributionPlugin: entry.AttributionPlugin,
		}
		existing, exists := seen[key]
		if !exists || entry.Message.Usage.OutputTokens > existing.outputTokens {
			seen[key] = tlDedupVal{rec: rec, outputTokens: entry.Message.Usage.OutputTokens}
		}
	}
	if err := sc.Err(); err != nil {
		log.Printf("sandbox: distillAttribution: scan %s: %v", path, err)
	}
}

// readAttributionMap reads outHost/attribution.jsonl and returns a map
// from requestId to AttributionRecord. Missing file silently returns nil.
func readAttributionMap(outHost string) map[string]AttributionRecord {
	data, err := readOutputFile(outHost, "attribution.jsonl")
	if err != nil {
		return nil
	}
	result := make(map[string]AttributionRecord)
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		var rec AttributionRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if rec.RequestID != "" {
			result[rec.RequestID] = rec
		}
	}
	return result
}

// collectTokensByAgent accumulates token counts per agent name from parsed
// usage lines and the attribution map. Unattributed requests use subagentMain.
func collectTokensByAgent(lines []string, attrib map[string]AttributionRecord) map[string]int64 {
	byAgent := make(map[string]int64)
	for _, line := range lines {
		if line == "" {
			continue
		}
		var rec struct {
			RequestID     string `json:"requestId"`
			Input         int64  `json:"input"`
			Output        int64  `json:"output"`
			CacheCreation int64  `json:"cache_creation"`
			CacheRead     int64  `json:"cache_read"`
		}
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		name := subagentMain
		if ar, ok := attrib[rec.RequestID]; ok && ar.AttributionAgent != "" {
			name = ar.AttributionAgent
		}
		byAgent[name] += rec.Input + rec.Output + rec.CacheCreation + rec.CacheRead
	}
	return byAgent
}

// summariseSubagents joins usage.jsonl × attribution.jsonl on requestId
// for this node only, buckets tokens per subagent (named or subagentMain for
// unattributed requests), and returns a short string listing the top
// subagents by total token consumption. Returns "" when there is no usage
// data or when the only bucket is subagentMain.
func summariseSubagents(outHost string) string {
	data, err := readOutputFile(outHost, "usage.jsonl")
	if err != nil {
		return ""
	}
	attrib := readAttributionMap(outHost)
	byAgent := collectTokensByAgent(strings.Split(string(data), "\n"), attrib)

	// Collect named subagents (exclude subagentMain from the label list).
	type entry struct {
		name   string
		tokens int64
	}
	var named []entry
	for name, tok := range byAgent {
		if name != subagentMain {
			named = append(named, entry{name, tok})
		}
	}
	if len(named) == 0 {
		return ""
	}
	sort.Slice(named, func(i, j int) bool {
		if named[i].tokens != named[j].tokens {
			return named[i].tokens > named[j].tokens
		}
		return named[i].name < named[j].name
	})
	const maxShow = 3
	if len(named) > maxShow {
		named = named[:maxShow]
	}
	names := make([]string, len(named))
	for i, e := range named {
		names[i] = e.name
	}
	return "top subagents by tokens: " + strings.Join(names, ", ")
}

// buildUsageSummary computes the one-line usage summary for AgentResult:
// cache-read percentage of input-side tokens, optionally followed by the
// top subagents by token consumption. Returns "" when perModel is empty.
func buildUsageSummary(perModel map[string]TokenTotals, outHost string) string {
	if len(perModel) == 0 {
		return ""
	}
	var totalInput, totalCacheCreation, totalCacheRead int64
	for _, t := range perModel {
		totalInput += t.Input
		totalCacheCreation += t.CacheCreation
		totalCacheRead += t.CacheRead
	}
	inputSide := totalInput + totalCacheCreation + totalCacheRead
	if inputSide == 0 {
		return ""
	}
	pct := float64(totalCacheRead) / float64(inputSide) * 100
	parts := []string{fmt.Sprintf("cache-read %.0f%% of input-side tokens", pct)}
	if sub := summariseSubagents(outHost); sub != "" {
		parts = append(parts, sub)
	}
	return strings.Join(parts, "; ")
}
