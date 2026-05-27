package codex

import (
	"bufio"
	"bytes"
	"encoding/json"
)

// Codex --json event type and item type constants.
// Values match the Codex JSONL event stream spec; typos here would silently
// skip events rather than failing loudly.
const (
	evtTypeItemCompleted = "item.completed"
	itemTypeAgentMessage = "agent_message"
)

// codexEvent is the subset of a Codex --json NDJSON line we read to
// recover the final answer.
type codexEvent struct {
	Type string `json:"type"`
	Item struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"item"`
}

// ResultText recovers the final answer from a Codex --json transcript
// (the NDJSON the CLI writes with --json). It collects all
// item.completed events whose item.type is "agent_message" and returns
// the LAST such text — the final assistant message. If none are found,
// it returns "". Malformed lines are skipped.
func (codexAgent) ResultText(transcript []byte) string {
	var last string

	sc := bufio.NewScanner(bytes.NewReader(transcript))
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		var evt codexEvent
		if err := json.Unmarshal(sc.Bytes(), &evt); err != nil {
			continue
		}
		if evt.Type == evtTypeItemCompleted && evt.Item.Type == itemTypeAgentMessage {
			last = evt.Item.Text
		}
	}

	return last
}
