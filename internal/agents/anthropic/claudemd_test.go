package anthropic

import (
	"testing"

	"github.com/jbeshir/demesne/internal/agents"
	"github.com/stretchr/testify/assert"
)

func TestGenerateContext(t *testing.T) {
	tests := []struct {
		name         string
		preamble     string
		prompt       string
		egress       string
		inputs       []agents.InputInfo
		mcpServers   []agents.MCPServerInfo
		previousJobs []string
		want         []string
		notWant      []string
	}{
		{
			name:    "no preamble no inputs",
			prompt:  "do the thing",
			egress:  "none",
			want:    []string{"## Environment", "No caller-supplied inputs", "## Task", "do the thing"},
			notWant: []string{"/in/notes", "/in/data"},
		},
		{
			name:     "preamble only",
			preamble: "Project: demesne. Stay terse.",
			prompt:   "explain the runner",
			egress:   "none",
			want: []string{
				"Project: demesne. Stay terse.",
				"## Environment",
				"## Task",
				"explain the runner",
			},
			notWant: []string{"/in/notes", "/in/data"},
		},
		{
			name:   "inputs only",
			prompt: "summarise the file",
			egress: "none",
			inputs: []agents.InputInfo{
				{Basename: "notes.txt", Size: 1024},
				{Basename: "data", IsDir: true},
			},
			want: []string{
				"/in/notes.txt", "file (1024 bytes)",
				"/in/data", "dir",
				"## Task", "summarise the file",
			},
		},
		{
			name:     "preamble and inputs",
			preamble: "context: refactor",
			prompt:   "produce a plan",
			egress:   "none",
			inputs:   []agents.InputInfo{{Basename: "src", IsDir: true}},
			want: []string{
				"context: refactor",
				"## Environment",
				"/in/src",
				"## Task",
				"produce a plan",
			},
		},
		{
			name:   "package-managers egress",
			prompt: "install a thing",
			egress: "package-managers",
			want: []string{
				"npm/PyPI/conda package registries",
			},
			notWant: []string{
				"unrestricted",
				"long-running research task",
			},
		},
		{
			name:   "open egress (research mode)",
			prompt: "investigate the corpus",
			egress: "open",
			want: []string{
				"Outbound network access is unrestricted",
				"long-running research task",
				"Flush partial findings to `/out`",
			},
			notWant: []string{
				"nothing else is reachable",
			},
		},
		{
			name:    "no host tools omits the section",
			prompt:  "do the thing",
			egress:  "none",
			notWant: []string{"Available host tools"},
		},
		{
			name:    "no previous jobs omits the note",
			prompt:  "do the thing",
			egress:  "none",
			notWant: []string{"/in/previous-jobs"},
		},
		{
			name:         "previous jobs note listed when present",
			prompt:       "build on earlier work",
			egress:       "none",
			previousJobs: []string{"phase01", "phase02"},
			want: []string{
				"`/in/previous-jobs/<name>`",
				"read earlier siblings' results",
			},
		},
		{
			name:   "host tools listed under native names",
			prompt: "look something up",
			egress: "none",
			mcpServers: []agents.MCPServerInfo{
				{
					Name: "workflowy",
					URL:  "http://127.0.0.1:8089/mcp",
					Tools: []agents.MCPToolInfo{
						{Name: "search_nodes", Description: "search the tree"},
						{Name: "get_node"},
					},
				},
			},
			want: []string{
				"## Available host tools",
				"**workflowy**",
				"`search_nodes` — search the tree",
				"`get_node`",
			},
			notWant: []string{"workflowy__search_nodes"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateContext(tt.preamble, tt.prompt, tt.egress, tt.inputs, tt.mcpServers, tt.previousJobs)
			for _, s := range tt.want {
				assert.Contains(t, got, s)
			}
			for _, s := range tt.notWant {
				assert.NotContains(t, got, s)
			}
		})
	}
}
