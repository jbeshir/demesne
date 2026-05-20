package mcpproxy

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Config configures an Aggregator.
type Config struct {
	// HostMCPConfigPath is the file demesne reads to discover the
	// user's stdio MCP servers. Typically ~/.claude.json.
	HostMCPConfigPath string
	// AllowlistFilePath is the optional user override file. Empty
	// means "use built-in defaults only".
	AllowlistFilePath string
	// SeedAllowlistFile, if true, writes a starter override file
	// at AllowlistFilePath when none exists.
	SeedAllowlistFile bool
	// SocketPath is the host filesystem path the aggregator's unix
	// socket listens on. The runner bind-mounts this socket into each
	// sandbox sidecar — a filesystem hop, so it works under rootless
	// podman where the sandbox network namespace can't reach a
	// host-process TCP port. Required.
	SocketPath string
	// ExtraServers are in-process MCP servers mounted alongside the
	// discovered stdio upstreams (e.g. demesne's own child-spawning
	// tools). Each is mounted at /{name}/mcp and appears in Servers()
	// and Catalogue() like any other server, so the runner wires it
	// into every sandbox through the same sidecar tunnel. Unlike the
	// stdio upstreams, an extra server's handler is supplied directly.
	ExtraServers []ExtraServer
}

// ExtraServer is an in-process MCP server mounted on the aggregator
// in addition to the discovered stdio upstreams. Tools is the
// catalogue surfaced to agents (CLAUDE.md + agent MCP config);
// Handler serves the MCP endpoint.
type ExtraServer struct {
	Name    string
	Tools   []mcp.Tool
	Handler http.Handler
}

// ToolCatalogue lists the tools exposed to sandboxed agents,
// grouped by upstream server name. Used by the runner to populate
// CLAUDE.md and the agent-side MCP config.
type ToolCatalogue map[string][]mcp.Tool

// Aggregator owns the host-side HTTP MCP server and the lazy stdio
// client pool that backs it. There is exactly one Aggregator per
// demesne-mcp process; its bindings are reused across all sandbox
// runs.
type Aggregator struct {
	pool       *Pool
	allow      map[string]ServerAllowlist
	httpSrv    *http.Server
	servers    []string
	catalogue  ToolCatalogue
	socketPath string
	extra      []ExtraServer
}

// NewAggregator validates cfg, discovers upstreams, and resolves
// the allowlist. No subprocesses are started and no HTTP listener
// is opened yet — call Start for that.
func NewAggregator(cfg Config) (*Aggregator, error) {
	if cfg.SocketPath == "" {
		return nil, errors.New("mcpproxy: SocketPath is required")
	}
	specs, err := DiscoverUpstreams(cfg.HostMCPConfigPath)
	if err != nil {
		return nil, fmt.Errorf("discover host MCP config: %w", err)
	}
	if cfg.SeedAllowlistFile && cfg.AllowlistFilePath != "" {
		if err := SeedOverrideFile(cfg.AllowlistFilePath); err != nil {
			log.Printf("mcpproxy: seed allowlist file: %v", err)
		}
	}
	allow, err := ResolveAllowlist(cfg.AllowlistFilePath)
	if err != nil {
		return nil, fmt.Errorf("resolve allowlist: %w", err)
	}
	keep := make([]UpstreamSpec, 0, len(specs))
	for _, s := range specs {
		entry, ok := allow[s.Name]
		if !ok {
			continue
		}
		if !entry.AllowAll && len(entry.Tools) == 0 {
			continue
		}
		keep = append(keep, s)
	}
	return &Aggregator{
		pool:       NewPool(keep),
		allow:      allow,
		catalogue:  ToolCatalogue{},
		socketPath: cfg.SocketPath,
		extra:      cfg.ExtraServers,
	}, nil
}

// Start spawns each upstream once to fetch its tool catalogue,
// filters by the allowlist, builds one in-process MCPServer per
// upstream, mounts them on a shared http.ServeMux at
// /{server-name}/mcp, then opens a single HTTP listener.
//
// Per-upstream initialise/list failures degrade gracefully: the
// affected server is omitted from the bindings, the rest still
// come up. Returns an error only if the listener itself can't
// open or if every upstream failed.
func (a *Aggregator) Start(ctx context.Context) error {
	if a.httpSrv != nil {
		return errors.New("mcpproxy: Aggregator already started")
	}
	mux := http.NewServeMux()
	specs := a.pool.knownSpecs()

	for _, spec := range specs {
		entry := a.allow[spec.Name]
		tools, err := a.pool.ListUpstreamTools(ctx, spec.Name)
		if err != nil {
			log.Printf("mcpproxy: %q: list tools failed, skipping: %v", spec.Name, err)
			continue
		}
		exposed := filterTools(tools, entry)
		if len(exposed) == 0 {
			log.Printf("mcpproxy: %q: no allowlisted tools after intersect, skipping", spec.Name)
			continue
		}
		srv := newServerForUpstream(spec.Name, exposed, a.pool)
		path := "/" + spec.Name + "/mcp"
		mux.Handle(path, server.NewStreamableHTTPServer(srv))
		a.catalogue[spec.Name] = exposed
	}

	for _, e := range a.extra {
		mux.Handle("/"+e.Name+"/mcp", e.Handler)
		a.catalogue[e.Name] = e.Tools
	}

	if len(a.catalogue) == 0 {
		return errors.New("mcpproxy: no upstreams produced exposable tools")
	}

	ln, err := a.listenSocket()
	if err != nil {
		return err
	}
	for name := range a.catalogue {
		a.servers = append(a.servers, name)
	}
	sort.Strings(a.servers)

	a.httpSrv = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
	}
	go func() {
		if err := a.httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("mcpproxy: http serve: %v", err)
		}
	}()
	log.Printf("mcpproxy: listening on unix:%s with %d upstreams", a.socketPath, len(a.servers))
	return nil
}

// listenSocket creates the parent dir, removes any stale socket from
// a prior run, and opens the unix listener.
func (a *Aggregator) listenSocket() (net.Listener, error) {
	if dir := filepath.Dir(a.socketPath); dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("create %s: %w", dir, err)
		}
	}
	if err := os.Remove(a.socketPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("remove stale socket %s: %w", a.socketPath, err)
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "unix", a.socketPath)
	if err != nil {
		return nil, fmt.Errorf("listen on unix socket %s: %w", a.socketPath, err)
	}
	return ln, nil
}

// Servers returns the upstream server names that produced exposable
// tools, sorted. The runner pairs each with a sidecar listener.
// Path convention: /{server}/mcp.
func (a *Aggregator) Servers() []string {
	out := make([]string, len(a.servers))
	copy(out, a.servers)
	return out
}

// SocketPath is the host filesystem path the aggregator's unix
// socket listens on; the runner bind-mounts it into each sidecar.
func (a *Aggregator) SocketPath() string { return a.socketPath }

// Catalogue returns the per-server filtered tool list used to
// populate CLAUDE.md and the agent-side MCP config.
func (a *Aggregator) Catalogue() ToolCatalogue {
	out := make(ToolCatalogue, len(a.catalogue))
	for k, v := range a.catalogue {
		cp := make([]mcp.Tool, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// Shutdown stops the HTTP server and terminates every cached
// upstream client.
func (a *Aggregator) Shutdown(ctx context.Context) error {
	var firstErr error
	if a.httpSrv != nil {
		if err := a.httpSrv.Shutdown(ctx); err != nil {
			firstErr = err
		}
	}
	a.pool.Shutdown()
	return firstErr
}

// newServerForUpstream builds an mcp-go MCPServer registered with
// exactly the given (already-allowlisted) tools, each routed via
// the pool to the named upstream.
func newServerForUpstream(name string, tools []mcp.Tool, pool *Pool) *server.MCPServer {
	srv := server.NewMCPServer("demesne-"+name, "0", server.WithToolCapabilities(false))
	for _, t := range tools {
		tool := t
		srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			req.Params.Name = tool.Name
			return pool.CallTool(ctx, name, req)
		})
	}
	return srv
}

func filterTools(tools []mcp.Tool, allow ServerAllowlist) []mcp.Tool {
	out := make([]mcp.Tool, 0, len(tools))
	for _, t := range tools {
		if !allow.Allowed(t.Name) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// knownSpecs returns the upstream specs the pool was constructed
// with, sorted by Name. Used by Aggregator.Start.
func (p *Pool) knownSpecs() []UpstreamSpec {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]UpstreamSpec, 0, len(p.specs))
	for _, s := range p.specs {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
