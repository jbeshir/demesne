package anthropic

import (
	"testing"

	"github.com/jbeshir/demesne/internal/agents"
	"github.com/stretchr/testify/assert"
)

const (
	egressNone    = "none"
	promptDefault = "do the thing"
	wantTask      = "## Task"
	wantEnv       = "## Environment"
	inDataMount   = "/in/data"
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
			prompt:  promptDefault,
			egress:  egressNone,
			want:    []string{wantEnv, "No caller-supplied inputs", wantTask, promptDefault},
			notWant: []string{"/in/notes", inDataMount},
		},
		{
			name:     "preamble only",
			preamble: "Project: demesne. Stay terse.",
			prompt:   "explain the runner",
			egress:   egressNone,
			want: []string{
				"Project: demesne. Stay terse.",
				wantEnv,
				wantTask,
				"explain the runner",
			},
			notWant: []string{"/in/notes", inDataMount},
		},
		{
			name:   "inputs only",
			prompt: "summarise the file",
			egress: egressNone,
			inputs: []agents.InputInfo{
				{Basename: "notes.txt", Size: 1024},
				{Basename: "data", IsDir: true},
			},
			want: []string{
				"/in/notes.txt", "file (1024 bytes)",
				inDataMount, "dir",
				wantTask, "summarise the file",
			},
		},
		{
			name:     "preamble and inputs",
			preamble: "context: refactor",
			prompt:   "produce a plan",
			egress:   egressNone,
			inputs:   []agents.InputInfo{{Basename: "src", IsDir: true}},
			want: []string{
				"context: refactor",
				wantEnv,
				"/in/src",
				wantTask,
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
			egress: egressOpen,
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
			prompt:  promptDefault,
			egress:  egressNone,
			notWant: []string{"Available host tools"},
		},
		{
			name:   "no previous jobs omits the note",
			prompt: promptDefault,
			egress: egressNone,
			// The orchestration section always mentions /in/previous-jobs,
			// so key on the conditional note's distinctive phrasing.
			notWant: []string{"read earlier siblings' results"},
		},
		{
			name:   "orchestration guidance always present",
			prompt: promptDefault,
			egress: egressNone,
			want: []string{
				"## Orchestrating child agents",
				"Validate with real builds/tests",
				"including `.git`",
				"Plan and enforce the handoff",
			},
		},
		{
			name:         "previous jobs note listed when present",
			prompt:       "build on earlier work",
			egress:       egressNone,
			previousJobs: []string{"phase01", "phase02"},
			want: []string{
				"`/in/previous-jobs/<name>`",
				"read earlier siblings' results",
			},
		},
		{
			name:   "host tools listed under native names",
			prompt: "look something up",
			egress: egressNone,
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
