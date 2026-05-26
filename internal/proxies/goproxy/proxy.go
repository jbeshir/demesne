// Package goproxy implements the per-sandbox Go module proxy. It
// forwards Go module-proxy protocol requests (modules + the /sumdb/
// checksum-database endpoint) to proxy.golang.org over a transport that
// carries the egress-bypass SO_MARK, so a sandbox can fetch Go modules
// even under egress=none — the sandbox only ever talks to 127.0.0.1.
//
// The agent/scripts reach it via GOPROXY=http://127.0.0.1:<port>, which
// the runner injects into every sandbox's environment.
package goproxy

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jbeshir/demesne/internal/proxies"
)

// UpstreamHost is the public Go module proxy hostname.
const UpstreamHost proxies.EgressHost = "proxy.golang.org"

// UpstreamBase is the full upstream URL the proxy forwards to.
const UpstreamBase = "https://" + string(UpstreamHost)

// listenPort is the loopback port the proxy binds inside the per-sandbox
// sidecar (below the Anthropic proxy's 8088 and the MCP tunnel's 8089+).
const listenPort = "8087"

const listenAddr = "127.0.0.1:" + listenPort

// Name is the registered name for the Go module proxy.
const Name = "go-mod"

// SumDBHost is the Go checksum database. Module *downloads* go through
// this proxy (SO_MARK bypass, so the upstream needs no allowlist
// entry), but `go` contacts the checksum database directly —
// proxy.golang.org does not advertise sumdb proxying — so the sandbox
// itself must be allowed to reach it. EgressHosts surfaces it for the
// runner to add to every sandbox's egress allowlist, keeping module
// checksum verification on even under egress=none.
const SumDBHost proxies.EgressHost = "sum.golang.org"

// ProxyURL is the value the runner sets as GOPROXY in every sandbox.
func ProxyURL() string { return "http://" + listenAddr }

// BindAddr is the 127.0.0.1:<port> the proxy binds inside the sidecar.
func BindAddr() string { return listenAddr }

func init() {
	proxies.Register(registration{})
}

// registration is the discovery-only registry entry. EgressHosts is nil
// because SO_MARK bypasses the egress filter (the upstream never needs
// to be in the sandbox allowlist).
type registration struct{}

func (registration) Name() string                      { return Name }
func (registration) EgressHosts() []proxies.EgressHost { return []proxies.EgressHost{SumDBHost} }

// ProxyServer is a forwarding proxy for proxy.golang.org.
type ProxyServer struct {
	bindAddr string
	server   *http.Server
}

// NewProxyServer constructs the production proxy: bound to bindAddr,
// forwarding to UpstreamBase over the egress-bypass transport.
func NewProxyServer(bindAddr string) *ProxyServer {
	return newProxyServer(bindAddr, UpstreamBase, proxies.BypassTransport())
}

// newProxyServerTo is the test-only constructor: forwards to the given
// upstream over http.DefaultTransport, so tests don't need
// CAP_NET_ADMIN for setsockopt(SO_MARK).
func newProxyServerTo(bindAddr, upstreamURL string) *ProxyServer {
	return newProxyServer(bindAddr, upstreamURL, http.DefaultTransport)
}

func newProxyServer(bindAddr, upstreamURL string, transport http.RoundTripper) *ProxyServer {
	target, err := url.Parse(upstreamURL)
	if err != nil {
		panic("goproxy: invalid upstream URL " + upstreamURL + ": " + err.Error())
	}
	h := &forwarder{
		target: target,
		client: &http.Client{Transport: transport},
	}
	return &ProxyServer{
		bindAddr: bindAddr,
		server: &http.Server{
			Addr:              bindAddr,
			Handler:           methodGate(h),
			ReadHeaderTimeout: 30 * time.Second,
		},
	}
}

// methodGate forwards only GET/HEAD: the Go module-proxy protocol is
// read-only, so nothing else has any business reaching the upstream.
func methodGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			next.ServeHTTP(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

// forwarder proxies Go module-proxy requests to the upstream, following
// redirects internally over the egress-bypass transport. proxy.golang.org
// serves large module zips by 302-redirecting to a storage.googleapis.com
// signed URL; the sandbox client cannot resolve that host, so the proxy
// must follow the redirect itself (it holds the egress bypass) and stream
// the final response back. A ReverseProxy would relay the 302 unchanged
// and the client's follow-up fetch would fail DNS.
type forwarder struct {
	target *url.URL
	client *http.Client
}

func (f *forwarder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	out := *f.target
	out.Path = singleJoiningSlash(f.target.Path, r.URL.Path)
	out.RawQuery = r.URL.RawQuery

	req, err := http.NewRequestWithContext(r.Context(), r.Method, out.String(), nil)
	if err != nil {
		http.Error(w, "bad upstream request", http.StatusBadGateway)
		return
	}

	resp, err := f.client.Do(req) //nolint:gosec // fixed upstream host; only path/query come from the request
	if err != nil {
		log.Printf("go-mod proxy: upstream error: %v", err)
		http.Error(w, "upstream fetch failed", http.StatusBadGateway)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	for key, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(key, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("go-mod proxy: response copy error: %v", err)
	}
}

// singleJoiningSlash joins two URL path segments with exactly one slash,
// matching net/http/httputil's path-join semantics.
func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash && a != "" && b != "":
		return a + "/" + b
	default:
		return a + b
	}
}

// Start binds and serves until ctx is cancelled, then gracefully shuts
// down. Returns nil on clean shutdown.
func (p *ProxyServer) Start(ctx context.Context) error {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", p.bindAddr)
	if err != nil {
		return err
	}
	log.Printf("go-mod proxy: listening on %s", ln.Addr())

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := p.server.Shutdown(shutCtx); err != nil {
			log.Printf("go-mod proxy: shutdown error: %v", err)
		}
	}()

	if err := p.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
