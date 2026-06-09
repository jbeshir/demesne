package sandbox

import (
	"testing"

	"github.com/jbeshir/demesne/internal/mcpproxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildMCPWiring_ParentJobIDForFileGenServers(t *testing.T) {
	r := &Runner{cfg: Config{MCPServers: []string{
		mcpproxy.DemesneServerName, "image-gen-mcp", "mermaid", "workflowy",
	}}}
	w := r.buildMCPWiring(JobID("job-test"))
	require.Len(t, w.sidecarUpstreams, 4)
	for _, b := range w.sidecarUpstreams {
		switch b.Name {
		case mcpproxy.DemesneServerName, "image-gen-mcp", "mermaid":
			assert.Equal(t, "job-test", b.ParentJobID, "server %s should carry parent job ID", b.Name)
		case "workflowy":
			assert.Empty(t, b.ParentJobID, "workflowy should NOT carry parent job ID")
		}
	}
}
