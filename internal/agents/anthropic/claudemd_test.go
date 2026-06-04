package anthropic

import (
	"testing"

	"github.com/jbeshir/demesne/internal/agents"
	"github.com/jbeshir/demesne/internal/egress"
	"github.com/stretchr/testify/assert"
)

const (
	promptDefault = "do the thing"
	wantTask      = "## Task"
	wantEnv       = "## Environment"
	wantDefOfDone = "## Definition of done"
	inDataMount   = "/in/data"
)

func TestGenerateContext(t *testing.T) {
	tests := []struct {
		name           string
		preamble       string
		prompt         string
		egress         egress.Mode
		inputs         []agents.InputInfo
		mcpServers     []agents.MCPServerInfo
		previousJobs   []string
		outputContract agents.OutputContract
		want           []string
		notWant        []string
	}{
		{
			name:    "no preamble no inputs",
			prompt:  promptDefault,
			egress:  egress.None,
			want:    []string{wantEnv, "No caller-supplied inputs", wantTask, promptDefault, "Flush partial findings to `/out`"},
			notWant: []string{"/in/notes", inDataMount},
		},
		{
			name:     "preamble only",
			preamble: "Project: demesne. Stay terse.",
			prompt:   "explain the runner",
			egress:   egress.None,
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
			egress: egress.None,
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
			egress:   egress.None,
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
			egress: egress.PackageManagers,
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
			egress: egress.Open,
			want: []string{
				"Outbound network access is unrestricted",
				"Flush partial findings to `/out`",
			},
			notWant: []string{
				"nothing else is reachable",
			},
		},
		{
			name:    "no host tools omits the section",
			prompt:  promptDefault,
			egress:  egress.None,
			notWant: []string{"Available host tools"},
		},
		{
			name:   "no previous jobs omits the note",
			prompt: promptDefault,
			egress: egress.None,
			// The orchestration section always mentions /in/previous-jobs,
			// so key on the conditional note's distinctive phrasing.
			notWant: []string{"read earlier siblings' results"},
		},
		{
			name:   "orchestration guidance always present",
			prompt: promptDefault,
			egress: egress.None,
			want: []string{
				"## Orchestrating child agents",
				"Delivering results is your job",
				"copy it into your own `/out` with plain `cp`",
				"Verify with an external signal",
				"transcript.jsonl",
				"including `.git`",
				"Plan and enforce the handoff",
				"Do not spawn a child for what your own tools can complete",
				"Match effort to task",
			},
		},
		{
			name:   "sandbox environment restrictions",
			prompt: promptDefault,
			egress: egress.None,
			want: []string{
				"Do not start persistent daemons",
				"`stdout` field of the parent's tool result",
			},
		},
		{
			name:         "previous jobs note listed when present",
			prompt:       "build on earlier work",
			egress:       egress.None,
			previousJobs: []string{"phase01", "phase02"},
			want: []string{
				"`/in/previous-jobs/<name>`",
				"read earlier siblings' results",
				"ls /in/previous-jobs/",
			},
		},
		{
			name:   "host tools listed under native names",
			prompt: "look something up",
			egress: egress.None,
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
		{
			name:    "empty contract emits no definition of done",
			prompt:  promptDefault,
			egress:  egress.None,
			notWant: []string{wantDefOfDone},
		},
		{
			name:           "contract with path only",
			prompt:         promptDefault,
			egress:         egress.None,
			outputContract: agents.OutputContract{Path: "/out/foo.md"},
			want:           []string{wantDefOfDone, "/out/foo.md"},
		},
		{
			name:   "contract with all three fields",
			prompt: promptDefault,
			egress: egress.None,
			outputContract: agents.OutputContract{
				Path:            "/out/report.md",
				Format:          "Markdown report",
				SuccessCriteria: []string{"covers all sections", "no broken links"},
			},
			want: []string{
				wantDefOfDone,
				"/out/report.md",
				"Markdown report",
				"covers all sections",
				"no broken links",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateContext(agents.ContextParams{
				Preamble:       tt.preamble,
				Prompt:         tt.prompt,
				Egress:         tt.egress,
				Inputs:         tt.inputs,
				MCPServers:     tt.mcpServers,
				PreviousJobs:   tt.previousJobs,
				OutputContract: tt.outputContract,
			})
			for _, s := range tt.want {
				assert.Contains(t, got, s)
			}
			for _, s := range tt.notWant {
				assert.NotContains(t, got, s)
			}
		})
	}
}
