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

func TestResultText_NonAgentMessageItemIgnored(t *testing.T) {
	// command_execution items should not contribute to the result.
	transcript := `{"type":"item.completed","item":{"id":"item_1","type":"command_execution","text":"ls output"}}
{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"the answer"}}
`
	got := codexAgent{}.ResultText([]byte(transcript))
	assert.Equal(t, "the answer", got)
}

func TestResultText_EmptyInputReturnsEmpty(t *testing.T) {
	assert.Empty(t, codexAgent{}.ResultText(nil))
	assert.Empty(t, codexAgent{}.ResultText([]byte("")))
}

func TestResultText_SkipsMalformedLines(t *testing.T) {
	assert.Empty(t, codexAgent{}.ResultText([]byte("not json\n{also not}\n")))
}
