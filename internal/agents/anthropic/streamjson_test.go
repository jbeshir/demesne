package anthropic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResultText_PrefersResultEvent(t *testing.T) {
	transcript := `{"type":"system","subtype":"init"}
{"type":"assistant","message":{"content":[{"type":"text","text":"thinking..."}]}}
{"type":"result","subtype":"success","is_error":false,"result":"PONG","total_cost_usd":0.01}
`
	got := claudeCodeAgent{}.ResultText([]byte(transcript))
	assert.Equal(t, "PONG", got)
}

func TestResultText_FallsBackToAssistantText(t *testing.T) {
	// No result event (e.g. interrupted run): concatenate assistant text.
	transcript := `{"type":"system","subtype":"init"}
{"type":"assistant","message":{"content":[{"type":"text","text":"hello "},{"type":"text","text":"world"}]}}
`
	got := claudeCodeAgent{}.ResultText([]byte(transcript))
	assert.Equal(t, "hello world", got)
}

func TestResultText_SkipsMalformedAndEmpty(t *testing.T) {
	assert.Empty(t, claudeCodeAgent{}.ResultText(nil))
	assert.Empty(t, claudeCodeAgent{}.ResultText([]byte("not json\n{also not}\n")))
}
