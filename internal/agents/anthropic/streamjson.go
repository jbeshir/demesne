package anthropic

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
)

// Claude Code stream-json event types and content-block type constants.
// Values match the Claude Code stream-json spec; typos here would silently
// skip events rather than failing loudly.
const (
	evtTypeResult    = "result"
	evtTypeAssistant = "assistant"
	contentTypeText  = "text"
)

// streamEvent is the subset of a Claude Code stream-json NDJSON line we
// read to recover the final answer.
type streamEvent struct {
	Type    string `json:"type"`
	Result  string `json:"result"`
	Message struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
}

// ResultText recovers the final answer from a Claude Code stream-json
// transcript (the NDJSON the CLI writes with
// --output-format stream-json --verbose). It prefers the terminal
// {"type":"result"} event's `result` field; if that's absent (e.g. the
// run errored before completing) it falls back to the concatenated text
// of the assistant messages. Malformed lines are skipped.
func (claudeCodeAgent) ResultText(transcript []byte) string {
	var result string
	var assistant strings.Builder

	sc := bufio.NewScanner(bytes.NewReader(transcript))
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		var evt streamEvent
		if err := json.Unmarshal(sc.Bytes(), &evt); err != nil {
			continue
		}
		switch evt.Type {
		case evtTypeResult:
			if evt.Result != "" {
				result = evt.Result
			}
		case evtTypeAssistant:
			for _, c := range evt.Message.Content {
				if c.Type == contentTypeText {
					assistant.WriteString(c.Text)
				}
			}
		}
	}

	if result != "" {
		return result
	}
	return strings.TrimSpace(assistant.String())
}
