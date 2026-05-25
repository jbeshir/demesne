// Package mcp implements the per-sandbox sidecar's MCP tunnel. It
// runs one HTTP reverse-proxy listener per upstream MCP server,
// each bound to its own loopback port inside the sidecar network
// namespace, and forwards every request over a single bind-mounted
// unix socket to the host-side aggregator.
//
// A unix socket (rather than a TCP hop to the host) is what makes
// this work under rootless podman: the sandbox containers live in a
// separate network namespace where host-process TCP ports aren't
// reachable, but a bind-mounted socket is just a file and crosses
// the boundary regardless.
//
// There is no auth: the sandbox edge is the trust boundary, and the
// socket is only bind-mounted into demesne's own sidecars.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"sync"
	"time"

	"github.com/jbeshir/demesne/internal/proxies"
)

// Name is the registered name for the MCP tunnel proxy.
const Name = "mcp"

const tcpNetwork = "tcp"

// FirstListenPort is the loopback port the alphabetically-first
// upstream's listener binds inside the sidecar. Subsequent
// upstreams take FirstListenPort+1, +2, … in the order the host
// assigns them. The namespace is isolated, so these ports have no
// host-side surface.
const FirstListenPort = 8089

// BindingsEnv carries the JSON-encoded binding table from the host
// runner to the sidecar. Empty/unset means "no MCP tunnel".
const BindingsEnv = "DEMESNE_MCP_BINDINGS"

// SocketPathEnv carries the in-sidecar path of the bind-mounted unix
// socket the host aggregator listens on. Required when BindingsEnv
// is set.
const SocketPathEnv = "DEMESNE_MCP_SOCKET"

// ParentHeader is the trusted header the tunnel injects to identify
// the calling sandbox to the host's in-process demesne MCP server.
// The agent can't forge it: the agent only reaches the sidecar's
// loopback listener, and the tunnel strips any client-supplied value
// before setting the binding's own. Only the demesne binding carries
// it; external upstreams ignore it.
const ParentHeader = "X-Demesne-Parent"

// Binding describes one upstream's tunnel: the loopback port the
// agent connects to inside the sidecar, and the HTTP path on the
// aggregator socket that serves this upstream (e.g. /workflowy/mcp).
//
// ParentJobID, when set, is injected as the ParentHeader on every
// forwarded request so the host's demesne server can resolve which
// sandbox is calling. Empty for external upstreams.
type Binding struct {
	Name        string `json:"name"`
	ListenPort  int    `json:"listen_port"`
	Path        string `json:"path"`
	ParentJobID string `json:"parent_job_id,omitempty"`
}

// ParseBindings decodes the BindingsEnv JSON. An empty string
// yields a nil slice with no error.
func ParseBindings(raw string) ([]Binding, error) {
	if raw == "" {
		return nil, nil
	}
	var out []Binding
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("parse %s: %w", BindingsEnv, err)
	}
	for i, b := range out {
		if b.Name == "" || b.ListenPort == 0 || b.Path == "" {
			return nil, fmt.Errorf("%s[%d]: name, listen_port, and path are all required", BindingsEnv, i)
		}
	}
	return out, nil
}

func init() {
	proxies.Register(registration{})
}

// registration is the discovery-only registry entry. The MCP
// tunnel reaches the host over a unix socket, so it contributes no
// egress hosts. The actual port set is decided at runtime from the
// bindings.
type registration struct{}

func (registration) Name() string          { return Name }
func (registration) EgressHosts() []string { return nil }

// Server is the sidecar MCP tunnel: a set of per-upstream reverse
// proxies sharing one unix-socket transport to the host aggregator.
type Server struct {
	socketPath string
	bindings   []Binding
	transport  http.RoundTripper
	servers    []*http.Server
}

// NewServer builds a tunnel for the given bindings, forwarding over
// the unix socket at socketPath (the in-sidecar bind-mount path).
func NewServer(socketPath string, bindings []Binding) *Server {
	return &Server{
		socketPath: socketPath,
		bindings:   bindings,
		transport:  unixTransport(socketPath),
	}
}

// Start opens all per-upstream listeners and serves until ctx is
// cancelled, then gracefully shuts every listener down. Returns nil
// on clean shutdown.
func (s *Server) Start(ctx context.Context) error {
	if len(s.bindings) == 0 {
		<-ctx.Done()
		return nil
	}
	if s.socketPath == "" {
		return errors.New("mcp tunnel: socket path is required when bindings are present")
	}
	var lc net.ListenConfig
	errCh := make(chan error, len(s.bindings))
	var wg sync.WaitGroup
	for _, b := range s.bindings {
		handler := s.handlerFor(b)
		addr := fmt.Sprintf("127.0.0.1:%d", b.ListenPort)
		ln, err := lc.Listen(ctx, tcpNetwork, addr)
		if err != nil {
			return fmt.Errorf("mcp tunnel listen %s: %w", addr, err)
		}
		srv := &http.Server{
			Handler:           handler,
			ReadHeaderTimeout: 30 * time.Second,
		}
		s.servers = append(s.servers, srv)
		log.Printf("mcp tunnel: %q on %s -> unix:%s%s", b.Name, addr, s.socketPath, b.Path)
		wg.Add(1)
		go func(b Binding) {
			defer wg.Done()
			if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- fmt.Errorf("mcp tunnel %q: %w", b.Name, err)
			}
		}(b)
	}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		for _, srv := range s.servers {
			if err := srv.Shutdown(shutCtx); err != nil {
				log.Printf("mcp tunnel: shutdown error: %v", err)
			}
		}
	}()

	wg.Wait()
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

// handlerFor builds the reverse-proxy handler for one binding. It
// gates by method (the MCP Streamable HTTP verbs) and forwards over
// the shared unix-socket transport, rewriting the path to the
// binding's aggregator path. Bodies pass through unchanged so
// SSE-style streaming survives.
func (s *Server) handlerFor(b Binding) http.Handler {
	rp := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.Out.URL.Scheme = "http"
			// Host is a placeholder — the transport dials the socket
			// regardless — but it must be set for a valid request.
			r.Out.URL.Host = "demesne-mcp"
			r.Out.Host = "demesne-mcp"
			r.Out.URL.Path = b.Path
			r.Out.URL.RawQuery = r.In.URL.RawQuery
			// Strip any client-supplied identity header, then set the
			// trusted value. The agent can't forge it — it only reaches
			// this loopback listener, never the socket directly.
			r.Out.Header.Del(ParentHeader)
			if b.ParentJobID != "" {
				r.Out.Header.Set(ParentHeader, b.ParentJobID)
			}
		},
		Transport:     s.transport,
		ErrorLog:      log.New(log.Writer(), "mcp-tunnel: ", log.LstdFlags),
		FlushInterval: -1, // stream responses immediately (SSE)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodGet, http.MethodDelete:
			rp.ServeHTTP(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

// unixTransport returns an http transport that dials the given unix
// socket for every request, ignoring the request's host.
func unixTransport(socketPath string) *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	d := &net.Dialer{Timeout: 30 * time.Second}
	t.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return d.DialContext(ctx, "unix", socketPath)
	}
	return t
}
