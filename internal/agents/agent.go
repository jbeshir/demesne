// Package agents defines the vendor-neutral provider abstraction used by
// sandbox_agent. Each vendor (Anthropic, OpenAI, …) lives in its own
// subpackage and registers one or more Agent implementations from an init
// function. The top-level binary blank-imports the vendor packages it
// wants available.
//
// Models, auth tokens, container images, and CLI invocation are all
// vendor-specific and stay in the vendor packages. The Agent interface is
// what demesne's runner depends on.
package agents

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/jbeshir/demesne/internal/egress"
)

// InputInfo summarises one /in/<basename> mount for an agent's context
// file generator (e.g. CLAUDE.md, AGENTS.md).
type InputInfo struct {
	Basename string
	IsDir    bool
	Size     int64
}

// MCPToolInfo names one host MCP tool exposed to the agent, with the
// upstream's own description (for CLAUDE.md listing).
type MCPToolInfo struct {
	Name        string
	Description string
}

// MCPServerInfo describes one host MCP server the agent can reach
// through the sidecar tunnel. URL is the sandbox-facing endpoint
// (e.g. http://127.0.0.1:8089/mcp); Tools are the allowlisted tools
// the agent will see under their native names.
type MCPServerInfo struct {
	Name  string
	URL   string
	Tools []MCPToolInfo
}

// AgentConfig carries the data WriteAgentConfig needs to emit the
// agent CLI's MCP configuration into the config directory.
type AgentConfig struct {
	MCPServers []MCPServerInfo
}

// AgentConfigDir is the in-sandbox mount point for an agent run's
// read-only control files (the generated context file and the MCP
// config). The runner bind-mounts each run's private config dir here.
// It lives under /in (not /workspace) so that sibling agents sharing
// a /workspace mount can't clobber each other's control files, and so
// the context file isn't picked up as an ancestor CLAUDE.md by a
// nested working directory.
const AgentConfigDir = "/in/.agent"

// Agent is the provider abstraction for sandbox_agent. Each vendor's
// subpackage supplies one or more implementations.
type Agent interface {
	// Name is the caller-facing identifier (the value of the `agent` MCP
	// parameter).
	Name() string

	// EnsureImage ensures the provider's container image is available in
	// the local Docker daemon and returns its tag. Implementations should
	// cache aggressively — repeat callers should not trigger a rebuild.
	EnsureImage(ctx context.Context) (imageTag string, err error)

	// GenerateContext returns the contents of the context file (e.g.
	// CLAUDE.md) that should be written into the sandbox before the agent
	// starts. Inputs describe each /in/<basename> mount; egress is the
	// caller-visible egress.Mode so the provider can tell the model
	// exactly what's reachable. Empty egress is treated as "none".
	// mcpServers lists the host MCP servers wired into this run (empty
	// when none); the provider documents them in the context file.
	// previousJobs names completed sibling jobs whose /out is mounted
	// read-only under /in/previous-jobs/<name> (empty when none).
	GenerateContext(
		preamble, prompt string,
		egress egress.Mode,
		inputs []InputInfo,
		mcpServers []MCPServerInfo,
		previousJobs []string,
	) string

	// WriteAgentConfig writes whatever CLI configuration the agent needs
	// into configDir before the sandbox starts (e.g. an mcp.json
	// pointing at the per-sandbox MCP tunnel). configDir is bind-mounted
	// read-only at AgentConfigDir inside the sandbox. It is always
	// called, even when cfg has no MCP servers, so providers can no-op
	// cleanly.
	WriteAgentConfig(configDir string, cfg AgentConfig) error

	// ContextFileName is the basename of the context file under the
	// sandbox cwd (e.g. "CLAUDE.md").
	ContextFileName() string

	// ResultText extracts the human-facing result text from the bytes
	// the agent wrote to its transcript output (the runner redirects the
	// agent's stdout to /out and reads it back). The runner uses this to
	// populate the MCP result's stdout. For agents that stream structured
	// events (e.g. claude-code's stream-json), this parses out the final
	// answer; empty input yields an empty string.
	ResultText(transcript []byte) string

	// ResolveModel validates and normalises a caller-supplied model name
	// against the vendor's whitelist. Empty input must resolve to a
	// sensible default.
	ResolveModel(name string) (string, error)

	// Command is the argv the runner should execute inside the sandbox.
	Command(prompt, model string) []string

	// EnvVars are the environment variables to set on the command. Keys
	// are env var names; values are literal values to inject. Each
	// provider knows the URL of its own proxy (via the matching
	// internal/proxies/<vendor> package), so callers don't need to
	// thread proxy URLs through this interface.
	EnvVars(oauthToken, model string) map[string]string
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Agent{}
)

// Register adds an agent provider to the global registry. Intended to be
// called from a vendor subpackage's init function.
func Register(name string, a Agent) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("agents: %q already registered", name))
	}
	registry[name] = a
}

// Lookup returns the agent registered under the given name. The empty
// string resolves to the default agent (currently "claude-code"; this is
// a hardcoded default that ships with demesne's Anthropic provider).
func Lookup(name string) (Agent, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	if name == "" {
		name = DefaultAgent
	}
	if a, ok := registry[name]; ok {
		return a, nil
	}
	available := make([]string, 0, len(registry))
	for k := range registry {
		available = append(available, k)
	}
	sort.Strings(available)
	return nil, fmt.Errorf("agent %q is not registered (available: %v)", name, available)
}

// DefaultAgent is the name resolved when sandbox_agent's `agent`
// parameter is left empty. Anthropic's claude-code agent is the only
// provider demesne ships in M3.
const DefaultAgent = "claude-code"
