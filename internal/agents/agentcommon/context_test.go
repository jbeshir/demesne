package agentcommon

import (
	"strings"
	"testing"

	"github.com/jbeshir/demesne/internal/agents"
	"github.com/jbeshir/demesne/internal/egress"
	"github.com/stretchr/testify/assert"
)

func TestProviderNeutralPromptBlocksOmitCodexEngineeringGuidance(t *testing.T) {
	var b strings.Builder
	p := agents.ContextParams{Prompt: "do the thing", Egress: egress.None}

	WritePreamble(&b, "project context")
	WriteEnvironment(&b, p, EgressSentence(p.Egress, "the vendor API"))
	WriteHostTools(&b, nil)
	WriteFileGenNote(&b, nil)
	WriteOrchestration(&b)
	WriteDefinitionOfDone(&b, agents.OutputContract{})
	WriteTask(&b, p.Prompt)

	assert.NotContains(t, b.String(), "## Engineering changes")
	assert.NotContains(t, b.String(), "variadic optional parameters")
}

func TestWriteFileGenNote_PresentWhenFileGenServerListed(t *testing.T) {
	var b strings.Builder
	WriteFileGenNote(&b, []agents.MCPServerInfo{{Name: "image-gen-mcp"}})
	assert.Contains(t, b.String(), "/workspace/generated/")
}

func TestWriteFileGenNote_AbsentWhenNoFileGenServers(t *testing.T) {
	var b strings.Builder
	WriteFileGenNote(&b, []agents.MCPServerInfo{{Name: "workflowy"}})
	assert.Empty(t, b.String())
}
