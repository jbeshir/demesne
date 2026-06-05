package mcpproxy

import (
	"bytes"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func noEnv(string) (string, bool) { return "", false }

func TestParseCodexUpstreams_StdioEntry(t *testing.T) {
	tomlData := `
[mcp_servers.mytool]
command = "/usr/bin/mytool"
args = ["--verbose"]
env = { TOKEN = "abc" }
`
	got, err := parseCodexUpstreams([]byte(tomlData), noEnv)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "mytool", got[0].Name)
	assert.Equal(t, "/usr/bin/mytool", got[0].Command)
	assert.Equal(t, []string{"--verbose"}, got[0].Args)
	assert.Equal(t, map[string]string{testEnvTokenName: "abc"}, got[0].Env)
}

func TestParseCodexUpstreams_EnvVars(t *testing.T) {
	tomlData := `
[mcp_servers.mytool]
command = "/usr/bin/mytool"
env_vars = ["HOME", "NOT_SET"]
`
	lookup := func(name string) (string, bool) {
		if name == "HOME" {
			return "/home/x", true
		}
		return "", false
	}
	got, err := parseCodexUpstreams([]byte(tomlData), lookup)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, map[string]string{"HOME": "/home/x"}, got[0].Env)
}

func TestParseCodexUpstreams_ExplicitEnvWins(t *testing.T) {
	tomlData := `
[mcp_servers.mytool]
command = "/usr/bin/mytool"
env_vars = ["FOO"]
env = { FOO = "from-env" }
`
	lookup := func(name string) (string, bool) {
		if name == "FOO" {
			return "from-process", true
		}
		return "", false
	}
	got, err := parseCodexUpstreams([]byte(tomlData), lookup)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "from-env", got[0].Env["FOO"])
}

func TestParseCodexUpstreams_HTTPDropped(t *testing.T) {
	tomlData := `
[mcp_servers.webserver]
url = "https://example.com/mcp"
`
	got, err := parseCodexUpstreams([]byte(tomlData), noEnv)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestParseCodexUpstreams_DemesneDropped(t *testing.T) {
	tomlData := `
[mcp_servers.demesne]
command = "/usr/bin/demesne"
`
	got, err := parseCodexUpstreams([]byte(tomlData), noEnv)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestParseCodexUpstreams_InvalidSlug(t *testing.T) {
	tomlData := `
[mcp_servers.good]
command = "/bin/good"

[mcp_servers."bad/name"]
command = "/bin/bad"

[mcp_servers.UPPER]
command = "/bin/upper"
`
	got, err := parseCodexUpstreams([]byte(tomlData), noEnv)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "good", got[0].Name)
}

func TestDiscoverCodexUpstreams_MissingFile(t *testing.T) {
	got, err := DiscoverCodexUpstreams("/nonexistent/path/.codex/config.toml", noEnv)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestMergeUpstreams_Disjoint(t *testing.T) {
	claude := []UpstreamSpec{
		{Name: "a", Command: "/bin/a"},
		{Name: "c", Command: "/bin/c"},
	}
	codex := []UpstreamSpec{
		{Name: "b", Command: "/bin/b"},
		{Name: "d", Command: "/bin/d"},
	}
	got := mergeUpstreams(claude, codex)
	require.Len(t, got, 4)
	assert.Equal(t, []string{"a", "b", "c", "d"}, names(got))
}

func TestMergeUpstreams_Conflict_CodexWins(t *testing.T) {
	claude := []UpstreamSpec{
		{Name: "shared", Command: "/bin/claude-shared", Args: []string{"--claude"}},
	}
	codex := []UpstreamSpec{
		{Name: "shared", Command: "/bin/codex-shared", Args: []string{"--codex"}},
	}
	var logBuf bytes.Buffer
	origWriter := log.Writer()
	log.SetOutput(&logBuf)
	t.Cleanup(func() { log.SetOutput(origWriter) })

	got := mergeUpstreams(claude, codex)
	require.Len(t, got, 1)
	assert.Equal(t, "/bin/codex-shared", got[0].Command)
	assert.Equal(t, []string{"--codex"}, got[0].Args)
	assert.Contains(t, logBuf.String(), `codex MCP server "shared" overrides claude entry`)
}

func TestMergeUpstreams_DuplicateName_DedupsToOne(t *testing.T) {
	spec := UpstreamSpec{Name: "dup", Command: "/bin/dup"}
	got := mergeUpstreams([]UpstreamSpec{spec}, []UpstreamSpec{spec})
	require.Len(t, got, 1)
}

func names(specs []UpstreamSpec) []string {
	out := make([]string, len(specs))
	for i, s := range specs {
		out[i] = s.Name
	}
	return out
}
