package mcpproxy

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
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

// ErrAlreadyStarted is returned when Start is called on an Aggregator
// that has already been started.
var ErrAlreadyStarted = errors.New("mcpproxy: Aggregator already started")

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
	serveDone  chan struct{}
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
// Per-upstream tool list failure is fatal for that server (it is
// skipped). Resource, template, and prompt list failures are
// non-fatal (they are treated as empty — many servers don't
// implement those capabilities). A server is only skipped when
// all four lists are empty after filtering.
//
// Returns an error only if the listener itself can't open or if
// every upstream failed.
func (a *Aggregator) Start(ctx context.Context) error {
	if a.httpSrv != nil {
		return ErrAlreadyStarted
	}
	mux := http.NewServeMux()
	specs := a.pool.knownSpecs()
	mounted := 0

	for _, spec := range specs {
		if a.registerUpstream(ctx, mux, spec) {
			mounted++
		}
	}

	for _, e := range a.extra {
		mux.Handle("/"+e.Name+"/mcp", e.Handler)
		a.catalogue[e.Name] = e.Tools
		mounted++
	}

	if mounted == 0 {
		return errors.New("mcpproxy: no upstreams produced exposable tools, resources, or prompts")
	}

	ln, err := a.listenSocket(ctx)
	if err != nil {
		return err
	}

	a.httpSrv = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
	}
	a.serveDone = make(chan struct{})
	go func() {
		if err := a.httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("mcpproxy: http serve: %v", err)
		}
		close(a.serveDone)
	}()
	totalTools := 0
	for _, v := range a.catalogue {
		totalTools += len(v)
	}
	log.Printf("mcpproxy: listening on unix:%s with %d upstreams; tool catalogue exposes %d tool(s)",
		a.socketPath, mounted, totalTools)
	return nil
}

func (a *Aggregator) registerUpstream(ctx context.Context, mux *http.ServeMux, spec UpstreamSpec) bool {
	entry := a.allow[spec.Name]
	tools, err := a.pool.ListUpstreamTools(ctx, spec.Name)
	if err != nil {
		log.Printf("mcpproxy: %q: list tools failed, skipping: %v", spec.Name, err)
		return false
	}
	exposed := filterTools(tools, entry)

	resources, err := a.pool.ListUpstreamResources(ctx, spec.Name)
	if err != nil {
		log.Printf("mcpproxy: %q: list resources failed, treating as empty: %v", spec.Name, err)
		resources = nil
	}
	templates, err := a.pool.ListUpstreamResourceTemplates(ctx, spec.Name)
	if err != nil {
		log.Printf("mcpproxy: %q: list resource templates failed, treating as empty: %v", spec.Name, err)
		templates = nil
	}
	prompts, err := a.pool.ListUpstreamPrompts(ctx, spec.Name)
	if err != nil {
		log.Printf("mcpproxy: %q: list prompts failed, treating as empty: %v", spec.Name, err)
		prompts = nil
	}

	if len(exposed) == 0 && len(resources) == 0 && len(templates) == 0 && len(prompts) == 0 {
		log.Printf("mcpproxy: %q: no exposable tools/resources/prompts, skipping", spec.Name)
		return false
	}

	srv := newServerForUpstream(spec.Name, exposed, resources, templates, prompts, a.pool)
	path := "/" + spec.Name + "/mcp"
	mux.Handle(path, server.NewStreamableHTTPServer(srv))
	a.catalogue[spec.Name] = exposed
	return true
}

// listenSocket creates the parent dir, removes any stale socket from
// a prior run, and opens the unix listener.
func (a *Aggregator) listenSocket(ctx context.Context) (net.Listener, error) {
	if dir := filepath.Dir(a.socketPath); dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("create %s: %w", dir, err)
		}
	}
	if err := os.Remove(a.socketPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("remove stale socket %s: %w", a.socketPath, err)
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "unix", a.socketPath)
	if err != nil {
		return nil, fmt.Errorf("listen on unix socket %s: %w", a.socketPath, err)
	}
	return ln, nil
}

// Servers returns the upstream server names that produced exposable
// tools, resources, or prompts, sorted. The runner pairs each with
// a sidecar listener. Path convention: /{server}/mcp.
func (a *Aggregator) Servers() []string {
	out := make([]string, 0, len(a.catalogue))
	for k := range a.catalogue {
		out = append(out, k)
	}
	sort.Strings(out)
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
	if a.serveDone != nil {
		<-a.serveDone
	}
	a.pool.Shutdown()
	return firstErr
}

// newServerForUpstream builds an mcp-go MCPServer registered with
// the given (already-allowlisted) tools, resources, resource
// templates, and prompts, each routed via the pool to the named
// upstream. Completion is wired only when prompts or templates are
// present, since those are the only ref types that support it.
func newServerForUpstream(
	name string,
	tools []mcp.Tool,
	resources []mcp.Resource,
	templates []mcp.ResourceTemplate,
	prompts []mcp.Prompt,
	pool *Pool,
) *server.MCPServer {
	opts := []server.ServerOption{
		server.WithToolCapabilities(false),
		server.WithResourceCapabilities(false, false),
		server.WithPromptCapabilities(false),
	}
	if len(prompts) > 0 || len(templates) > 0 {
		opts = append(opts,
			server.WithCompletions(),
			server.WithPromptCompletionProvider(&promptCompletionRelay{server: name, pool: pool}),
			server.WithResourceCompletionProvider(&resourceCompletionRelay{server: name, pool: pool}),
		)
	}
	srv := server.NewMCPServer("demesne-"+name, "0", opts...)

	for _, t := range tools {
		tool := t
		srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			req.Params.Name = tool.Name
			return pool.CallTool(ctx, name, req)
		})
	}
	for _, r := range resources {
		res := r
		srv.AddResource(res, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			result, err := pool.ReadResource(ctx, name, req)
			if err != nil {
				return nil, err
			}
			return result.Contents, nil
		})
	}
	for _, t := range templates {
		tmpl := t
		srv.AddResourceTemplate(tmpl, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			result, err := pool.ReadResource(ctx, name, req)
			if err != nil {
				return nil, err
			}
			return result.Contents, nil
		})
	}
	for _, p := range prompts {
		prompt := p
		srv.AddPrompt(prompt, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			req.Params.Name = prompt.Name
			return pool.GetPrompt(ctx, name, req)
		})
	}
	return srv
}

// promptCompletionRelay forwards completion/complete requests for
// prompt arguments to the named upstream via the pool.
type promptCompletionRelay struct {
	server string
	pool   *Pool
}

func (p *promptCompletionRelay) CompletePromptArgument(
	ctx context.Context,
	promptName string,
	argument mcp.CompleteArgument,
	completeCtx mcp.CompleteContext,
) (*mcp.Completion, error) {
	req := mcp.CompleteRequest{}
	req.Params.Ref = mcp.PromptReference{Type: "ref/prompt", Name: promptName}
	req.Params.Argument = argument
	req.Params.Context = completeCtx
	result, err := p.pool.Complete(ctx, p.server, req)
	if err != nil {
		return nil, err
	}
	return &result.Completion, nil
}

// resourceCompletionRelay forwards completion/complete requests for
// resource template arguments to the named upstream via the pool.
type resourceCompletionRelay struct {
	server string
	pool   *Pool
}

func (r *resourceCompletionRelay) CompleteResourceArgument(
	ctx context.Context,
	uri string,
	argument mcp.CompleteArgument,
	completeCtx mcp.CompleteContext,
) (*mcp.Completion, error) {
	req := mcp.CompleteRequest{}
	req.Params.Ref = mcp.ResourceReference{Type: "ref/resource", URI: uri}
	req.Params.Argument = argument
	req.Params.Context = completeCtx
	result, err := r.pool.Complete(ctx, r.server, req)
	if err != nil {
		return nil, err
	}
	return &result.Completion, nil
}

func filterTools(tools []mcp.Tool, allow ServerAllowlist) []mcp.Tool {
	out := make([]mcp.Tool, 0, len(tools))
	for _, t := range tools {
		if !allow.Allowed(ToolName(t.Name)) {
			continue
		}
		out = append(out, t)
	}
	return out
}
