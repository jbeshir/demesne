package anthropic

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
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

// Env-var names the sidecar passes through to the Anthropic proxy.
// Constants live next to the implementation so the contract is
// documented in one place, but the *reads* happen in the sidecar's
// main (see cmd/demesne-sidecar/main.go) — the proxy itself receives
// its config as explicit arguments.
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
	// UsagePathEnv is the host-bind-mounted file path the proxy
	// rewrites with a usage snapshot after every request. Empty means
	// "track in memory only" (used in tests).
	UsagePathEnv = "DEMESNE_ANTHROPIC_USAGE_PATH"
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

// BindAddr is the 127.0.0.1:<port> the proxy binds inside the sidecar.
// Used by the sidecar's main when constructing the proxy server.
func BindAddr() string { return listenAddr }

func init() {
	proxies.Register(registration{})
}

// registration is the discovery-only registry entry: it exposes Name,
// EgressHosts, and ListenAddr so the sandbox runner can collect the
// proxy's egress hosts (none, in this case — SO_MARK bypasses the
// egress filter) and the sidecar can log the bind addr. Construction
// and serving happen in the sidecar's main via NewProxyServer.
type registration struct{}

func (registration) Name() string          { return Name }
func (registration) EgressHosts() []string { return nil }
func (registration) ListenAddr() string    { return listenAddr }

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
// tracker accumulates usage (cost reported indicatively); pass nil to
// disable tracking.
func NewProxyServer(bindAddr, agentToken, upstreamToken string, tracker *Tracker) *ProxyServer {
	return newProxyServer(bindAddr, APIBase, proxies.BypassTransport(), agentToken, upstreamToken, tracker)
}

// NewProxyServerTo is the test-only constructor: forwards to the given
// upstream URL over http.DefaultTransport, so tests that exercise the
// gating logic on a host without CAP_NET_ADMIN don't fail in
// setsockopt(SO_MARK). Production callers must use NewProxyServer.
func NewProxyServerTo(bindAddr, upstreamURL, agentToken, upstreamToken string, tracker *Tracker) *ProxyServer {
	return newProxyServer(bindAddr, upstreamURL, http.DefaultTransport, agentToken, upstreamToken, tracker)
}

func newProxyServer(
	bindAddr, upstreamURL string,
	transport http.RoundTripper,
	agentToken, upstreamToken string,
	tracker *Tracker,
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
			if tracker != nil {
				// Force identity so the usage parser sees raw SSE/JSON
				// bytes — Anthropic otherwise responds with gzip and
				// the parser would silently miss every usage block.
				// Bandwidth cost is negligible (loopback to sidecar).
				r.Out.Header.Set("Accept-Encoding", "identity")
			}
		},
		Transport: transport,
		ErrorLog:  log.New(log.Writer(), "anthropic-proxy: ", log.LstdFlags),
	}
	if tracker != nil {
		rp.ModifyResponse = func(resp *http.Response) error {
			resp.Body = wrapResponseBody(resp.Body, resp.Header.Get("Content-Type"), tracker)
			return nil
		}
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
// and the agent-token check. Accepted requests have their
// Authorization header rewritten to the real upstream token; x-api-key
// (the alternative Anthropic auth scheme) is stripped so the proxy is
// the only credential authority for the upstream.
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
