package anthropic

import (
	"testing"

	"github.com/jbeshir/demesne/internal/agents"
	"github.com/stretchr/testify/assert"
)

func TestGenerateContext(t *testing.T) {
	tests := []struct {
		name     string
		preamble string
		prompt   string
		inputs   []agents.InputInfo
		want     []string
		notWant  []string
	}{
		{
			name:    "no preamble no inputs",
			prompt:  "do the thing",
			want:    []string{"## Environment", "No caller-supplied inputs", "## Task", "do the thing"},
			notWant: []string{"/in/notes", "/in/data"},
		},
		{
			name:     "preamble only",
			preamble: "Project: demesne. Stay terse.",
			prompt:   "explain the runner",
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
			inputs:   []agents.InputInfo{{Basename: "src", IsDir: true}},
			want: []string{
				"context: refactor",
				"## Environment",
				"/in/src",
				"## Task",
				"produce a plan",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateContext(tt.preamble, tt.prompt, tt.inputs)
			for _, s := range tt.want {
				assert.Contains(t, got, s)
			}
			for _, s := range tt.notWant {
				assert.NotContains(t, got, s)
			}
		})
	}
}
