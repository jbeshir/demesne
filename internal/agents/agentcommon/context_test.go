package agentcommon

import (
	"strings"
	"testing"

	"github.com/jbeshir/demesne/internal/agents"
	"github.com/stretchr/testify/assert"
)

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
