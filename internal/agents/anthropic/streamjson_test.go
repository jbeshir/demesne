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
	tests := []struct {
		name       string
		transcript string
		want       string
	}{
		{
			name: "no result event",
			transcript: `{"type":"system","subtype":"init"}
{"type":"assistant","message":{"content":[{"type":"text","text":"hello "},{"type":"text","text":"world"}]}}
`,
			want: "hello world",
		},
		{
			name: "empty result event",
			transcript: `{"type":"assistant","message":{"content":[{"type":"text","text":"fallback answer"}]}}
{"type":"result","subtype":"success","is_error":false,"result":"","total_cost_usd":0.01}
`,
			want: "fallback answer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := claudeCodeAgent{}.ResultText([]byte(tt.transcript))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResultText_SkipsMalformedAndEmpty(t *testing.T) {
	assert.Empty(t, claudeCodeAgent{}.ResultText(nil))

	tests := []struct {
		name       string
		transcript string
		want       string
	}{
		{
			name:       "all malformed",
			transcript: "not json\n{also not}\n",
			want:       "",
		},
		{
			name: "malformed followed by valid event",
			transcript: `not json
{also not}
{"type":"result","subtype":"success","is_error":false,"result":"PONG"}
`,
			want: "PONG",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, claudeCodeAgent{}.ResultText([]byte(tt.transcript)))
		})
	}
}

func TestResultText_IgnoresNonAnswerEvents(t *testing.T) {
	tests := []struct {
		name       string
		transcript string
		want       string
	}{
		{
			name: "assistant looking payload on system event ignored",
			transcript: `{"type":"system","message":{"content":[{"type":"text","text":"not an assistant answer"}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"assistant answer"}]}}
`,
			want: "assistant answer",
		},
		{
			name: "result looking payload on unknown event ignored",
			transcript: `{"type":"completion","result":"not final"}
{"type":"result","subtype":"success","is_error":false,"result":"final answer"}
`,
			want: "final answer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := claudeCodeAgent{}.ResultText([]byte(tt.transcript))
			assert.Equal(t, tt.want, got)
		})
	}
}
