package codex

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResultText_ReturnsLastAgentMessage(t *testing.T) {
	transcript := `{"type":"thread.started","thread_id":"0199a213-81c0-7800-8aa1-bbab2a035a53"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_3","type":"agent_message","text":"Repo contains docs, sdk, and examples."}}
{"type":"turn.completed","usage":{"input_tokens":24763,"cached_input_tokens":24448,"output_tokens":122,"reasoning_output_tokens":0}}
`
	got := codexAgent{}.ResultText([]byte(transcript))
	assert.Equal(t, "Repo contains docs, sdk, and examples.", got)
}

func TestResultText_LastAgentMessageWins(t *testing.T) {
	// When multiple agent_message items appear, the last one is returned.
	transcript := `{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"first answer"}}
{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"final answer"}}
`
	got := codexAgent{}.ResultText([]byte(transcript))
	assert.Equal(t, "final answer", got)
}

func TestResultText_IgnoresNonAnswerEvents(t *testing.T) {
	tests := []struct {
		name       string
		transcript string
		want       string
	}{
		{
			name: "non agent item ignored",
			transcript: `{"type":"item.completed","item":{"id":"item_1","type":"command_execution","text":"ls output"}}
{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"the answer"}}
`,
			want: "the answer",
		},
		{
			name: "agent looking payload on wrong event type ignored",
			transcript: `{"type":"item.started","item":{"id":"item_1","type":"agent_message","text":"not final"}}
{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"the answer"}}
`,
			want: "the answer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := codexAgent{}.ResultText([]byte(tt.transcript))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResultText_EmptyInputReturnsEmpty(t *testing.T) {
	assert.Empty(t, codexAgent{}.ResultText(nil))
	assert.Empty(t, codexAgent{}.ResultText([]byte("")))
}

func TestResultText_SkipsMalformedLines(t *testing.T) {
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
{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"valid answer"}}
`,
			want: "valid answer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, codexAgent{}.ResultText([]byte(tt.transcript)))
		})
	}
}
