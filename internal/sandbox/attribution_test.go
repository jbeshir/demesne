package sandbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testKeyType      = "type"
	testKeyRequestID = "requestId"
)

// appendJSONLLine marshals v as JSON and appends it as one line to path.
func appendJSONLLine(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // test temp file
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	_, err = f.Write(append(data, '\n'))
	require.NoError(t, err)
}

// transcriptEntry builds a test transcript entry map, omitting keys whose
// values are zero so the JSON stays realistic. stopReason="" simulates a
// null/absent stop_reason (the field is simply omitted).
func transcriptEntry(requestID, msgID, stopReason string, outputTokens int64, agent, skill, plugin string) map[string]any {
	msg := map[string]any{
		"id":    msgID,
		"usage": map[string]any{"output_tokens": outputTokens},
	}
	if stopReason != "" {
		msg["stop_reason"] = stopReason
	}
	entry := map[string]any{
		testKeyType:      "assistant",
		testKeyRequestID: requestID,
		"message":        msg,
	}
	if agent != "" {
		entry["attributionAgent"] = agent
	}
	if skill != "" {
		entry["attributionSkill"] = skill
	}
	if plugin != "" {
		entry["attributionPlugin"] = plugin
	}
	return entry
}

// readAttributionJSONL reads outHost/attribution.jsonl and returns a map
// from requestId to record (for assertion convenience).
func readAttributionJSONL(t *testing.T, outHost string) map[string]AttributionRecord {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(outHost, "attribution.jsonl")) //nolint:gosec // test temp
	require.NoError(t, err, "attribution.jsonl should exist")
	byReqID := make(map[string]AttributionRecord)
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if line == "" {
			continue
		}
		var rec AttributionRecord
		require.NoError(t, json.Unmarshal([]byte(line), &rec))
		byReqID[rec.RequestID] = rec
	}
	return byReqID
}

// TestDistillAttribution exercises the core distillation logic:
//   - main JSONL entry without attribution → plain record
//   - duplicate (requestId, message.id) with differing output_tokens → keep max
//   - stop_reason absent (null) → dropped
//   - missing requestId → dropped
//   - non-assistant type → dropped
//   - nested subagent JSONL with attributionAgent → captured
//   - subagent with full attribution triple → captured
func TestDistillAttribution(t *testing.T) {
	ws := t.TempDir()
	out := t.TempDir()
	jobID := JobID("test-job-001")

	attribDir := filepath.Join(ws, agentCwdSubdir(jobID), ".demesne-attrib")
	require.NoError(t, os.MkdirAll(attribDir, 0o750))

	// Real claude-code layout: subagent files are at
	// <parentUuid>/subagents/agent-<agentId>.jsonl
	subDir := filepath.Join(attribDir, "parent-uuid-abc/subagents")
	require.NoError(t, os.MkdirAll(subDir, 0o750))

	mainJSONL := filepath.Join(attribDir, "session-main.jsonl")
	subJSONL := filepath.Join(subDir, "agent-xyz.jsonl")

	// req-001: valid main-thread entry, no attribution fields
	appendJSONLLine(t, mainJSONL, transcriptEntry("req-001", "msg-001", "end_turn", 100, "", "", ""))

	// req-002: duplicate same key, different output_tokens — keep 200
	appendJSONLLine(t, mainJSONL, transcriptEntry("req-002", "msg-002", "end_turn", 50, "", "", ""))
	appendJSONLLine(t, mainJSONL, transcriptEntry("req-002", "msg-002", "end_turn", 200, "", "", ""))

	// req-003: stop_reason absent → must be dropped
	appendJSONLLine(t, mainJSONL, transcriptEntry("req-003", "msg-003", "", 75, "", "", ""))

	// missing requestId → must be dropped
	appendJSONLLine(t, mainJSONL, transcriptEntry("", "msg-004", "end_turn", 30, "", "", ""))

	// non-assistant type → must be dropped
	appendJSONLLine(t, mainJSONL, map[string]any{testKeyType: "user", testKeyRequestID: "req-005"})

	// req-006: subagent entry with attributionAgent only
	appendJSONLLine(t, subJSONL, transcriptEntry("req-006", "msg-006", "end_turn", 300, "explore", "", ""))

	// req-007: subagent entry with full attribution triple
	appendJSONLLine(t, subJSONL, transcriptEntry("req-007", "msg-007", "end_turn", 150, "general-purpose", "myplugin:myskill", "myplugin"))

	distillAttribution(ws, jobID, out)

	byReqID := readAttributionJSONL(t, out)
	assert.Len(t, byReqID, 4, "expected records for req-001, req-002, req-006, req-007")

	// req-001: main thread — no attribution fields
	assert.Equal(t, AttributionRecord{RequestID: "req-001"}, byReqID["req-001"])

	// req-002: dedup kept max output_tokens; record is otherwise the same
	assert.Equal(t, AttributionRecord{RequestID: "req-002"}, byReqID["req-002"])

	// req-003: null stop_reason → must not appear
	assert.NotContains(t, byReqID, "req-003", "stop_reason:null entry must be dropped")

	// req-006: subagent with attributionAgent
	assert.Equal(t, "explore", byReqID["req-006"].AttributionAgent)
	assert.Empty(t, byReqID["req-006"].AttributionSkill)

	// req-007: full attribution triple
	rec7 := byReqID["req-007"]
	assert.Equal(t, "general-purpose", rec7.AttributionAgent)
	assert.Equal(t, "myplugin:myskill", rec7.AttributionSkill)
	assert.Equal(t, "myplugin", rec7.AttributionPlugin)
}

// TestDistillAttribution_AbsentDir verifies that a missing .demesne-attrib
// directory is a silent no-op: no error and no attribution.jsonl written.
func TestDistillAttribution_AbsentDir(t *testing.T) {
	ws := t.TempDir()
	out := t.TempDir()

	distillAttribution(ws, JobID("no-attrib-job"), out)

	_, err := os.Stat(filepath.Join(out, "attribution.jsonl"))
	assert.True(t, os.IsNotExist(err), "attribution.jsonl must not be created when .demesne-attrib is absent")
}

// TestSummariseSubagents verifies named subagent ranking by total tokens.
func TestSummariseSubagents(t *testing.T) {
	out := t.TempDir()

	writeUsage := func(reqID string, input, output int64) {
		t.Helper()
		entry := map[string]any{testKeyRequestID: reqID, "model": "m", "input": input, "output": output}
		appendJSONLLine(t, filepath.Join(out, "usage.jsonl"), entry)
	}
	writeAttrib := func(reqID, agent string) {
		t.Helper()
		appendJSONLLine(t, filepath.Join(out, "attribution.jsonl"),
			AttributionRecord{RequestID: reqID, AttributionAgent: agent})
	}

	// explore: 600 tokens, general-purpose: 350, req-main: (main)
	writeUsage("req-main", 1000, 200)
	writeUsage("req-explore", 500, 100)
	writeUsage("req-gp", 300, 50)

	writeAttrib("req-explore", "explore")
	writeAttrib("req-gp", "general-purpose")
	// req-main has no attribution record → buckets as (main)

	line := summariseSubagents(out)
	assert.Equal(t, "top subagents by tokens: explore, general-purpose", line)
}

// TestSummariseSubagents_MainOnly verifies empty return when no named subagents.
func TestSummariseSubagents_MainOnly(t *testing.T) {
	out := t.TempDir()
	data := `{"requestId":"req-1","model":"m","input":100,"output":20}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(out, "usage.jsonl"), []byte(data), 0o600))
	// No attribution.jsonl → all requests are (main)
	assert.Empty(t, summariseSubagents(out))
}

// TestBuildUsageSummary_NoUsage verifies that an empty perModel map yields "".
func TestBuildUsageSummary_NoUsage(t *testing.T) {
	assert.Empty(t, buildUsageSummary(nil, t.TempDir()))
}

// TestBuildUsageSummary_CacheReadPct verifies the cache-read percentage formula.
func TestBuildUsageSummary_CacheReadPct(t *testing.T) {
	out := t.TempDir()
	perModel := map[string]TokenTotals{
		"claude-sonnet-5": {
			Input:         200,
			Output:        50,
			CacheCreation: 100,
			CacheRead:     700,
		},
	}
	// inputSide = input(200) + cache_creation(100) + cache_read(700) = 1000
	// pct = 700/1000 = 70%
	summary := buildUsageSummary(perModel, out)
	assert.Contains(t, summary, "cache-read 70% of input-side tokens")
	// No subagents → no semicolon part
	assert.NotContains(t, summary, ";")
}
