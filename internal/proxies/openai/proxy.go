package openai

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/jbeshir/demesne/internal/proxies"
)

// APIHost is the upstream OpenAI API hostname.
const APIHost proxies.EgressHost = "api.openai.com"

// APIBase is the full upstream URL the proxy forwards to.
const APIBase = "https://" + string(APIHost)

// listenPort is the loopback port the proxy binds inside the per-sandbox
// sidecar. The sidecar's network namespace is isolated, so the port is
// purely internal and has no host-side surface.
const listenPort = "8086"

// listenAddr is the 127.0.0.1:<port> the proxy binds.
const listenAddr = "127.0.0.1:" + listenPort

// Name is the registered name for the OpenAI API proxy.
const Name = "openai"

// Env-var names the sidecar passes through to the OpenAI proxy.
// Constants live next to the implementation so the contract is
// documented in one place, but the *reads* happen in the sidecar's
// main (see cmd/demesne-sidecar/main.go) — the proxy itself receives
// its config as explicit arguments.
const (
	// AuthTokenEnv carries the per-sandbox agent-facing token. The
	// agent receives the same value in its API key env var; the proxy
	// rejects any request whose Authorization header doesn't match.
	AuthTokenEnv = "DEMESNE_OPENAI_AUTH_TOKEN" //nolint:gosec // env var name, not a credential
	// UpstreamKeyEnv carries the real OpenAI API key. The proxy
	// substitutes this for the agent token before forwarding to
	// api.openai.com. The agent never sees this value.
	UpstreamKeyEnv = "DEMESNE_OPENAI_UPSTREAM_KEY"
	// UsagePathEnv is the host-bind-mounted file path the proxy
	// rewrites with a usage snapshot after every request. Empty means
	// "track in memory only" (used in tests).
	UsagePathEnv = "DEMESNE_OPENAI_USAGE_PATH"
)

const (
	headerAuthorization  = "Authorization"
	bearerPrefix         = "Bearer "
	pathResponses        = "/v1/responses"
	pathResponsesCompact = "/v1/responses/compact"
)

// allowedEndpoints is the explicit (method, path) allowlist the proxy
// will forward. Everything else returns 403. Kept to the bare minimum
// Codex needs for inference — POST to /v1/responses and /v1/responses/compact,
// plus GET /v1/responses for WebSocket upgrades (the gating handler
// enforces the Upgrade header for the GET case).
var allowedEndpoints = map[string]map[string]struct{}{
	http.MethodPost: {
		pathResponses:        {},
		pathResponsesCompact: {},
	},
	// GET /v1/responses is only permitted as a WebSocket upgrade;
	// the gating handler enforces the Connection and Upgrade headers.
	http.MethodGet: {
		pathResponses: {},
	},
}

// ListenURL is what agent providers wire into the base_url config field.
// It returns the scheme+host:port only — NO path suffix. The caller
// (Codex provider config) appends /v1 before writing to config.toml.
func ListenURL() string { return "http://" + listenAddr }

// BindAddr is the 127.0.0.1:<port> the proxy binds inside the sidecar.
// Used by the sidecar's main when constructing the proxy server.
func BindAddr() string { return listenAddr }

func init() {
	proxies.Register(registration{})
}

// registration is the discovery-only registry entry: it exposes Name
// and EgressHosts so the sandbox runner can collect the proxy's egress
// hosts (none, in this case — SO_MARK bypasses the egress filter).
// Construction and serving happen in the sidecar's main via NewProxyServer.
type registration struct{}

func (registration) Name() string                      { return Name }
func (registration) EgressHosts() []proxies.EgressHost { return nil }

// ProxyServer is a hardened reverse proxy for api.openai.com.
// It enforces an explicit endpoint allowlist, verifies that the caller
// presents the per-sandbox agent token, and substitutes the real
// upstream key before forwarding.
type ProxyServer struct {
	bindAddr string
	server   *http.Server
}

// NewProxyServer constructs the production proxy: bound to bindAddr,
// forwarding to APIBase over a transport that sets the egress-bypass
// SO_MARK on every outbound socket. agentToken is the value the agent
// must present; upstreamKey is what the proxy sends to OpenAI.
// tracker accumulates usage (cost reported indicatively); pass nil to
// disable tracking.
func NewProxyServer(bindAddr, agentToken, upstreamKey string, tracker *Tracker) *ProxyServer {
	return newProxyServer(bindAddr, APIBase, proxies.BypassTransport(), agentToken, upstreamKey, tracker)
}

// newProxyServerTo is the test-only constructor: forwards to the given
// upstream URL over http.DefaultTransport, so tests that exercise the
// gating logic on a host without CAP_NET_ADMIN don't fail in
// setsockopt(SO_MARK). Production callers must use NewProxyServer.
func newProxyServerTo(upstreamURL, agentToken, upstreamKey string, tracker *Tracker) *ProxyServer { //nolint:unparam
	return newProxyServer("127.0.0.1:0", upstreamURL, http.DefaultTransport, agentToken, upstreamKey, tracker)
}

func newProxyServer(
	bindAddr, upstreamURL string,
	transport http.RoundTripper,
	agentToken, upstreamKey string,
	tracker *Tracker,
) *ProxyServer {
	target, err := url.Parse(upstreamURL)
	if err != nil {
		panic("openai: invalid upstream URL " + upstreamURL + ": " + err.Error())
	}
	rp := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			// Make the upstream see the right virtual host.
			r.Out.Host = target.Host
			if tracker != nil {
				// Force identity so the usage parser sees raw SSE/JSON
				// bytes — OpenAI otherwise responds with gzip and the
				// parser would silently miss every usage block.
				r.Out.Header.Set("Accept-Encoding", "identity")
			}
			// Do NOT touch Connection or Upgrade headers: ReverseProxy
			// handles WebSocket upgrades automatically when these are
			// left intact.
		},
		Transport:     transport,
		ErrorLog:      log.New(log.Writer(), "openai-proxy: ", log.LstdFlags),
		FlushInterval: -1, // stream SSE events and WebSocket frames immediately
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
			Handler:           gatingHandler(rp, agentToken, upstreamKey),
			ReadHeaderTimeout: 30 * time.Second,
		},
	}
}

// gatingHandler wraps the reverse proxy with the endpoint allowlist
// and the agent-token check. Accepted requests have their Authorization
// header rewritten to the real upstream key. GET /v1/responses is only
// forwarded when it carries a valid WebSocket Upgrade.
func gatingHandler(next http.Handler, agentToken, upstreamKey string) http.Handler {
	expectedAuth := bearerPrefix + agentToken
	upstreamAuth := bearerPrefix + upstreamKey
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
		// GET /v1/responses is only permitted as a WebSocket upgrade.
		if r.Method == http.MethodGet {
			upgradeHdr := r.Header.Get("Upgrade")
			connHdr := r.Header.Get("Connection")
			if !strings.EqualFold(upgradeHdr, "websocket") ||
				!strings.Contains(strings.ToLower(connHdr), "upgrade") {
				deny(w, r, http.StatusForbidden, "GET only permitted as WebSocket upgrade")
				return
			}
		}
		if r.Header.Get(headerAuthorization) != expectedAuth {
			deny(w, r, http.StatusUnauthorized, "agent token mismatch")
			return
		}
		r.Header.Set(headerAuthorization, upstreamAuth)
		next.ServeHTTP(w, r)
	})
}

func deny(w http.ResponseWriter, r *http.Request, code int, reason string) {
	// %q escapes control chars in the (attacker-controlled) method
	// and path so a request can't inject fake log lines.
	//nolint:gosec // G706: values are %q-escaped, defeating log injection
	log.Printf("openai proxy: deny method=%q path=%q reason=%s code=%d",
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
	log.Printf("openai proxy: listening on %s", ln.Addr())

	go func() {
		<-ctx.Done()
		// WithoutCancel preserves values/deadlines from ctx but drops
		// the cancellation — Shutdown needs a live ctx to drain.
		shutCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := p.server.Shutdown(shutCtx); err != nil {
			log.Printf("openai proxy: shutdown error: %v", err)
		}
	}()

	if err := p.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
