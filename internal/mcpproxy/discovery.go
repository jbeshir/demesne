// Package mcpproxy implements the host-side MCP aggregator: it
// reads the user's Claude Code MCP config, spawns the configured
// stdio MCP server subprocesses lazily on demand, and serves a
// per-upstream Streamable HTTP MCP endpoint on host loopback. The
// per-sandbox sidecar's MCP tunnel proxy points at these endpoints.
// Tools are advertised to the agent via the CLAUDE.md writer and the MCP config
// writer in the agents package, then filtered through the read-only allowlist below.
//
// Tools are filtered through a read-only allowlist (built-in defaults
// plus the user's optional override file). Resources, resource
// templates, prompts, and completion are relayed in full from any
// exposed upstream without allowlist filtering; listings reflect a
// static snapshot taken at aggregator Start.
package mcpproxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"regexp"
	"sort"
)

// DemesneServerName is the entry in the host MCP config we always
// skip during discovery — it is the demesne MCP server itself,
// re-proxying it would loop.
const DemesneServerName = "demesne"

var validServerName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,62}$`)

// transportStdio is the only MCP transport demesne currently supports.
const transportStdio = "stdio"

// UpstreamSpec describes one host-side stdio MCP server discovered
// from the Claude Code MCP config.
type UpstreamSpec struct {
	Name    string
	Command string
	Args    []string
	Env     map[string]string
}

// claudeConfig is the slice of ~/.claude.json we read. Other fields
// in the file (projects, sessions, telemetry, …) are ignored.
type claudeConfig struct {
	MCPServers map[string]claudeMCPServer `json:"mcpServers"`
}

// claudeMCPServer is the union shape Claude Code stores per server.
// Stdio entries set Command (and optionally Args/Env); HTTP/SSE
// entries set URL. Type is "stdio" / "sse" / "http"; older entries
// omit Type entirely and are treated as stdio.
type claudeMCPServer struct {
	Type    string            `json:"type,omitempty"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
}

// DiscoverUpstreams reads the Claude Code MCP config at the given
// path and returns the stdio server entries demesne should be
// willing to spawn — sorted alphabetically by Name for stable
// downstream ordering. The "demesne" self-entry is dropped to
// prevent self-loop. HTTP/SSE entries are dropped with a warning (only stdio is supported today).
// Missing or malformed files return an empty slice with
// nil error so demesne-mcp can start without host MCP tools.
func DiscoverUpstreams(path string) ([]UpstreamSpec, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path comes from operator config
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return parseUpstreams(data)
}

func parseUpstreams(data []byte) ([]UpstreamSpec, error) {
	var cfg claudeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse claude config: %w", err)
	}
	out := make([]UpstreamSpec, 0, len(cfg.MCPServers))
	for name, s := range cfg.MCPServers {
		if name == DemesneServerName {
			continue
		}
		if !validServerName.MatchString(name) {
			log.Printf("mcpproxy: skipping MCP server %q: name is not a valid slug (must match ^[a-z][a-z0-9_-]{0,62}$)", name)
			continue
		}
		if s.Type != "" && s.Type != transportStdio {
			log.Printf("mcpproxy: skipping MCP server %q: transport %q is not yet supported (only stdio)", name, s.Type)
			continue
		}
		if !isStdio(s) {
			continue
		}
		out = append(out, UpstreamSpec{
			Name:    name,
			Command: s.Command,
			Args:    append([]string(nil), s.Args...),
			Env:     copyEnv(s.Env),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func isStdio(s claudeMCPServer) bool {
	if s.Command == "" {
		return false
	}
	if s.Type != "" && s.Type != transportStdio {
		return false
	}
	return true
}

func copyEnv(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
