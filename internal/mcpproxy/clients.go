package mcpproxy

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// idleTimeout is how long an upstream stdio MCP server is kept
// alive after its most recent activity. Beyond this the client
// is shut down (next use respawns it). Picked at 10 minutes:
// long enough that interactive bursts share one client, short
// enough that idle subprocesses don't linger across sandbox
// runs.
const idleTimeout = 10 * time.Minute

// clientInfo is the metadata sent on Initialize to upstream
// servers. The upstream sees this in its initialise log, so the
// name is intentionally explicit.
var clientInfo = mcp.Implementation{Name: "demesne-mcp", Version: "0"}

// upstreamClient pairs a live mcp-go client with its expiry
// timer. Both are guarded by Pool.mu in the surrounding pool.
type upstreamClient struct {
	c           *client.Client
	idleTimer   *time.Timer
	lastUsedUTC time.Time
}

// Pool manages lazy stdio MCP clients keyed by server name. The
// first call to a server's tool starts the subprocess and runs
// the MCP initialise handshake; subsequent calls reuse the cached
// client until idleTimeout elapses.
//
// All exported methods are safe for concurrent use.
type Pool struct {
	mu      sync.Mutex
	specs   map[string]UpstreamSpec
	clients map[string]*upstreamClient
	timeout time.Duration
}

// NewPool builds a pool seeded with the given upstream specs. No
// subprocesses are started yet — they spawn lazily on first
// matching CallTool / ListUpstreamTools.
func NewPool(specs []UpstreamSpec) *Pool {
	by := make(map[string]UpstreamSpec, len(specs))
	for _, s := range specs {
		by[s.Name] = s
	}
	return &Pool{
		specs:   by,
		clients: make(map[string]*upstreamClient),
		timeout: idleTimeout,
	}
}

// CallTool routes a tools/call to the named upstream, spawning
// it on first use. Returns an error if the upstream is not in
// the spec set the pool was built with.
func (p *Pool) CallTool(ctx context.Context, server string, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := p.acquire(ctx, server)
	if err != nil {
		return nil, err
	}
	return c.CallTool(ctx, req)
}

// ListUpstreamTools returns the upstream's full tool catalogue
// via tools/list. Used at aggregator-start time to populate the
// in-process MCPServer entries, and to resolve `*` allowlist
// values to concrete tool names.
func (p *Pool) ListUpstreamTools(ctx context.Context, server string) ([]mcp.Tool, error) {
	c, err := p.acquire(ctx, server)
	if err != nil {
		return nil, err
	}
	res, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("list tools for %q: %w", server, err)
	}
	return res.Tools, nil
}

// Shutdown terminates every active upstream subprocess. Errors
// from individual clients are logged but not surfaced — shutdown
// is best-effort.
func (p *Pool) Shutdown() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for name, uc := range p.clients {
		if uc.idleTimer != nil {
			uc.idleTimer.Stop()
		}
		if err := uc.c.Close(); err != nil {
			log.Printf("mcpproxy: close %q client: %v", name, err)
		}
		delete(p.clients, name)
	}
}

func (p *Pool) acquire(ctx context.Context, server string) (*client.Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	spec, ok := p.specs[server]
	if !ok {
		return nil, fmt.Errorf("mcpproxy: no upstream registered for %q", server)
	}
	if uc, ok := p.clients[server]; ok {
		uc.lastUsedUTC = time.Now().UTC()
		p.resetIdleTimerLocked(server, uc)
		return uc.c, nil
	}
	c, err := spawnClient(ctx, spec)
	if err != nil {
		return nil, err
	}
	uc := &upstreamClient{c: c, lastUsedUTC: time.Now().UTC()}
	p.clients[server] = uc
	p.resetIdleTimerLocked(server, uc)
	return c, nil
}

func (p *Pool) resetIdleTimerLocked(server string, uc *upstreamClient) {
	if uc.idleTimer != nil {
		uc.idleTimer.Stop()
	}
	uc.idleTimer = time.AfterFunc(p.timeout, func() { p.evict(server) })
}

func (p *Pool) evict(server string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	uc, ok := p.clients[server]
	if !ok {
		return
	}
	if err := uc.c.Close(); err != nil {
		log.Printf("mcpproxy: idle-evict close %q: %v", server, err)
	}
	delete(p.clients, server)
}

// spawnClient starts a stdio MCP subprocess for the given spec
// and runs the MCP initialise handshake.
func spawnClient(ctx context.Context, spec UpstreamSpec) (*client.Client, error) {
	c, err := client.NewStdioMCPClient(spec.Command, envSlice(spec.Env), spec.Args...)
	if err != nil {
		return nil, fmt.Errorf("start %q upstream %q: %w", spec.Name, spec.Command, err)
	}
	initReq := mcp.InitializeRequest{}
	initReq.Params.ClientInfo = clientInfo
	if _, err := c.Initialize(ctx, initReq); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("initialize %q upstream: %w", spec.Name, err)
	}
	return c, nil
}

func envSlice(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}
