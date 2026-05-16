package anthropic

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/jbeshir/demesne/internal/proxies"
)

// APIHost is the upstream Anthropic API hostname.
const APIHost = "api.anthropic.com"

// APIBase is the full upstream URL the proxy forwards to.
const APIBase = "https://" + APIHost

// listenPort is the loopback port the proxy binds inside the per-sandbox
// sidecar. The sidecar's network namespace is isolated, so the port is
// purely internal and has no host-side surface.
const listenPort = "8088"

// listenAddr is the 127.0.0.1:<port> the proxy binds.
const listenAddr = "127.0.0.1:" + listenPort

// Name is the registered name for the Anthropic API proxy.
const Name = "anthropic"

// Env vars the sidecar reads at startup. demesne-mcp sets both when it
// launches the sidecar container per agent invocation.
const (
	// AuthTokenEnv carries the per-sandbox agent-facing token. The
	// agent receives the same value in its CLAUDE_CODE_OAUTH_TOKEN env
	// var; the proxy rejects any request whose Authorization header
	// doesn't match this token.
	AuthTokenEnv = "DEMESNE_ANTHROPIC_AUTH_TOKEN" //nolint:gosec // env var name, not a credential
	// UpstreamTokenEnv carries the real Anthropic OAuth token. The
	// proxy substitutes this for the agent token before forwarding to
	// api.anthropic.com. The agent never sees this value.
	UpstreamTokenEnv = "DEMESNE_ANTHROPIC_UPSTREAM_TOKEN" //nolint:gosec // env var name, not a credential
)

// allowedEndpoints is the explicit (method, path) allowlist the proxy
// will forward. Everything else returns 403. Kept to the bare minimum
// claude-code needs for inference — no batches, no files, no admin or
// memory endpoints — so even a compromised agent can't store data or
// otherwise mutate state on the Anthropic side via this credential.
var allowedEndpoints = map[string]map[string]struct{}{
	http.MethodPost: {
		"/v1/messages":              {},
		"/v1/messages/count_tokens": {},
	},
}

// ListenURL is what agent providers wire into ANTHROPIC_BASE_URL.
func ListenURL() string { return "http://" + listenAddr }

func init() {
	proxies.Register(&registration{})
}

// registration implements proxies.Proxy. Tokens are read at Run time
// from env vars set by the sidecar container's docker run -e flags.
type registration struct{}

func (registration) Name() string          { return Name }
func (registration) EgressHosts() []string { return nil }
func (registration) ListenAddr() string    { return listenAddr }

func (r registration) Run(ctx context.Context) error {
	auth := os.Getenv(AuthTokenEnv)
	upstream := os.Getenv(UpstreamTokenEnv)
	if auth == "" {
		return errors.New(AuthTokenEnv + " must be set on the anthropic proxy sidecar")
	}
	if upstream == "" {
		return errors.New(UpstreamTokenEnv + " must be set on the anthropic proxy sidecar")
	}
	return NewProxyServer(r.ListenAddr(), auth, upstream).Start(ctx)
}

// ProxyServer is a hardened reverse proxy for api.anthropic.com.
// It enforces an explicit endpoint allowlist, verifies that the caller
// presents the per-sandbox agent token, and substitutes the real
// upstream token before forwarding.
type ProxyServer struct {
	bindAddr string
	server   *http.Server
}

// NewProxyServer constructs the production proxy: bound to bindAddr,
// forwarding to APIBase over a transport that sets the egress-bypass
// SO_MARK on every outbound socket. agentToken is the value the agent
// must present; upstreamToken is what the proxy sends to Anthropic.
func NewProxyServer(bindAddr, agentToken, upstreamToken string) *ProxyServer {
	return newProxyServer(bindAddr, APIBase, bypassTransport(), agentToken, upstreamToken)
}

// NewProxyServerTo is the test-only constructor: forwards to the given
// upstream URL over http.DefaultTransport, so tests that exercise the
// gating logic on a host without CAP_NET_ADMIN don't fail in
// setsockopt(SO_MARK). Production callers must use NewProxyServer.
func NewProxyServerTo(bindAddr, upstreamURL, agentToken, upstreamToken string) *ProxyServer {
	return newProxyServer(bindAddr, upstreamURL, http.DefaultTransport, agentToken, upstreamToken)
}

func newProxyServer(
	bindAddr, upstreamURL string,
	transport http.RoundTripper,
	agentToken, upstreamToken string,
) *ProxyServer {
	target, err := url.Parse(upstreamURL)
	if err != nil {
		panic("anthropic: invalid upstream URL " + upstreamURL + ": " + err.Error())
	}
	rp := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			// Make the upstream see the right virtual host.
			r.Out.Host = target.Host
		},
		Transport: transport,
		ErrorLog:  log.New(log.Writer(), "anthropic-proxy: ", log.LstdFlags),
	}

	return &ProxyServer{
		bindAddr: bindAddr,
		server: &http.Server{
			Addr:              bindAddr,
			Handler:           gatingHandler(rp, agentToken, upstreamToken),
			ReadHeaderTimeout: 30 * time.Second,
		},
	}
}

// gatingHandler wraps the reverse proxy with the endpoint allowlist
// and agent-token check. Accepted requests have their Authorization
// header rewritten to the real upstream token; x-api-key (the
// alternative Anthropic auth scheme) is stripped so the proxy is the
// only credential authority for the upstream.
func gatingHandler(next http.Handler, agentToken, upstreamToken string) http.Handler {
	expectedAuth := "Bearer " + agentToken
	upstreamAuth := "Bearer " + upstreamToken
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths, ok := allowedEndpoints[r.Method]
		if !ok {
			deny(w, r, http.StatusForbidden, "method not allowed")
			return
		}
		if _, ok := paths[r.URL.Path]; !ok {
			deny(w, r, http.StatusForbidden, "path not allowed")
			return
		}
		if r.Header.Get("Authorization") != expectedAuth {
			deny(w, r, http.StatusUnauthorized, "agent token mismatch")
			return
		}
		r.Header.Set("Authorization", upstreamAuth)
		r.Header.Del("x-api-key")
		next.ServeHTTP(w, r)
	})
}

func deny(w http.ResponseWriter, r *http.Request, code int, reason string) {
	// %q escapes control chars in the (attacker-controlled) method
	// and path so a request can't inject fake log lines.
	//nolint:gosec // G706: values are %q-escaped, defeating log injection
	log.Printf("anthropic proxy: deny method=%q path=%q reason=%s code=%d",
		r.Method, r.URL.Path, reason, code)
	http.Error(w, reason, code)
}

// bypassTransport returns an http.Transport whose outbound sockets
// (both TLS and DNS) carry the shared egress-bypass SO_MARK. See
// internal/proxies/sockmark_linux.go for the rationale.
func bypassTransport() *http.Transport {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   proxies.BypassDialerControl,
		Resolver: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := &net.Dialer{Control: proxies.BypassDialerControl}
				return d.DialContext(ctx, network, address)
			},
		},
	}
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DialContext = dialer.DialContext
	return t
}

// Start binds and serves until ctx is cancelled, then gracefully
// shuts the server down. Returns nil on clean shutdown; other errors
// are surfaced.
func (p *ProxyServer) Start(ctx context.Context) error {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", p.bindAddr)
	if err != nil {
		return err
	}
	log.Printf("anthropic proxy: listening on %s", ln.Addr())

	go func() {
		<-ctx.Done()
		// WithoutCancel preserves values/deadlines from ctx but drops
		// the cancellation — Shutdown needs a live ctx to drain.
		shutCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := p.server.Shutdown(shutCtx); err != nil {
			log.Printf("anthropic proxy: shutdown error: %v", err)
		}
	}()

	if err := p.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown gracefully stops the proxy.
func (p *ProxyServer) Shutdown(ctx context.Context) error {
	return p.server.Shutdown(ctx)
}
