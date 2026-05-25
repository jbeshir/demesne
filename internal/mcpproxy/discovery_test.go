package mcpproxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseUpstreams(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []UpstreamSpec
	}{
		{
			name: "stdio entries kept, demesne dropped, http dropped",
			input: `{
				"mcpServers": {
					"workflowy": {"type": "stdio", "command": "/usr/bin/wf", "args": ["-v"], "env": {"TOKEN": "x"}},
					"demesne":   {"type": "stdio", "command": "/usr/bin/demesne"},
					"sentry":    {"type": "http", "url": "https://example/mcp"},
					"alignment": {"command": "/usr/bin/al"}
				}
			}`,
			want: []UpstreamSpec{
				{Name: serverAlignment, Command: "/usr/bin/al"},
				{Name: serverWorkflowy, Command: "/usr/bin/wf", Args: []string{"-v"}, Env: map[string]string{"TOKEN": "x"}},
			},
		},
		{
			name: "entry without command is skipped",
			input: `{
				"mcpServers": {
					"broken": {"type": "stdio", "args": ["-x"]}
				}
			}`,
			want: []UpstreamSpec{},
		},
		{
			name:  "empty config returns empty slice",
			input: `{}`,
			want:  []UpstreamSpec{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseUpstreams([]byte(tt.input))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseUpstreams_MalformedJSON(t *testing.T) {
	_, err := parseUpstreams([]byte(`{not json}`))
	assert.Error(t, err)
}

func TestDiscoverUpstreams_MissingFile(t *testing.T) {
	got, err := DiscoverUpstreams("/nonexistent/path/.claude.json")
	require.NoError(t, err)
	assert.Empty(t, got)
}
