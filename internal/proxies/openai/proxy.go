package openai

import (
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/jbeshir/demesne/internal/proxies"
	"github.com/jbeshir/demesne/internal/proxies/proxycommon"
)

// ChatGPTHost is the upstream ChatGPT API hostname. The proxy connects
// to it over proxies.BypassTransport() (SO_MARK egress bypass) rather
// than listing it in the egress allowlist, so EgressHosts returns nil.
const ChatGPTHost proxies.EgressHost = "chatgpt.com"

// ChatGPTBase is the full upstream URL the proxy forwards to.
const ChatGPTBase = "https://" + string(ChatGPTHost)

// chatgptResponsesPath is the path on the ChatGPT backend that handles
// the Codex Responses-wire-API POST. The incoming path /v1/responses is
// rewritten to this before forwarding.
const chatgptResponsesPath = "/backend-api/codex/responses"

// listenPort is the loopback port the proxy binds inside the per-sandbox
// sidecar. The sidecar's network namespace is isolated, so the port is
// purely internal and has no host-side surface.
const listenPort = "8086"

// listenAddr is the 127.0.0.1:<port> the proxy binds.
const listenAddr = "127.0.0.1:" + listenPort

// Name is the registered name for the OpenAI/Codex proxy.
const Name = "openai"

// Env-var names the sidecar passes through to the OpenAI/Codex proxy.
// Constants live next to the implementation so the contract is
// documented in one place, but the *reads* happen in the sidecar's
// main (see cmd/demesne-sidecar/main.go) — the proxy itself receives
// its config as explicit arguments.
const (
	// AuthTokenEnv carries the per-sandbox agent-facing token. The
	// agent receives the same value in its env_key env var; the proxy
	// rejects any request whose Authorization header doesn't match.
	AuthTokenEnv = "DEMESNE_OPENAI_AUTH_TOKEN" //nolint:gosec // env var name, not a credential
	// UpstreamTokensEnv carries the ChatGPT OAuth token set as a
	// JSON-encoded TokenSet. The proxy holds this set, refreshes it
	// autonomously, and swaps in a fresh access token on each request.
	// The agent never sees these values.
	UpstreamTokensEnv = "DEMESNE_OPENAI_UPSTREAM_TOKENS" //nolint:gosec // env var name, not a credential
	// UsagePathEnv is the host-bind-mounted file path the proxy
	// rewrites with a usage snapshot after every request. Empty means
	// "track in memory only" (used in tests).
	UsagePathEnv = "DEMESNE_OPENAI_USAGE_PATH"
)

const (
	headerAuthorization = "Authorization"
	headerAccountID     = "ChatGPT-Account-ID"
	headerOriginator    = "originator"
	headerVersion       = "version"
	headerUserAgent     = "User-Agent"

	bearerPrefix  = "Bearer "
	pathResponses = "/v1/responses"

	originatorValue = "codex_cli_rs"
	codexVersion    = "0.134.0"
	userAgentValue  = "codex_cli_rs/0.134.0 (demesne)"
)

// allowedEndpoints is the explicit (method, path) allowlist the proxy
// will forward. Only POST /v1/responses is permitted — the ChatGPT
// Codex backend uses SSE over POST, not WebSockets.
var allowedEndpoints = map[string]map[string]struct{}{
	http.MethodPost: {
		pathResponses: {},
	},
}

// ListenURL is what agent providers wire into the base_url config field.
// It returns the scheme+host:port only — NO path suffix. The Codex
// provider config appends /v1, giving base_url="http://127.0.0.1:8086/v1",
// and the client POSTs to base_url+"/responses" = host:port/v1/responses.
func ListenURL() string { return "http://" + listenAddr }

// BindAddr is the 127.0.0.1:<port> the proxy binds inside the sidecar.
// Used by the sidecar's main when constructing the proxy server.
func BindAddr() string { return listenAddr }

func init() {
	proxies.Register(registration{})
}

// registration is the discovery-only registry entry: it exposes Name
// and EgressHosts so the sandbox runner can collect the proxy's egress
// hosts (none here — SO_MARK bypass handles reachability to chatgpt.com
// and auth.openai.com without listing them in the egress allowlist).
type registration struct{}

func (registration) Name() string                      { return Name }
func (registration) EgressHosts() []proxies.EgressHost { return nil }

// ProxyServer is a hardened reverse proxy for the ChatGPT Codex endpoint.
// It enforces an explicit endpoint allowlist, verifies the per-sandbox
// agent token, refreshes the OAuth credential as needed, and rewrites
// the request to the real ChatGPT backend.
type ProxyServer struct {
	bindAddr string
	server   *http.Server
}

// NewProxyServer constructs the production proxy: bound to bindAddr,
// forwarding to ChatGPTBase over a transport that sets the egress-bypass
// SO_MARK on every outbound socket. agentToken is the value the agent
// must present; creds holds and autonomously refreshes the real OAuth
// token set. tracker accumulates usage (indicative only — ChatGPT-OAuth
// billing is subscription-based); pass nil to disable tracking.
func NewProxyServer(bindAddr, agentToken string, creds *Credential, tracker *Tracker) *ProxyServer {
	return newProxyServer(bindAddr, ChatGPTBase, proxies.BypassTransport(), agentToken, creds, tracker)
}

// newProxyServerTo is the test-only constructor: forwards to the given
// backend URL over http.DefaultTransport, so tests that exercise the
// gating logic on a host without CAP_NET_ADMIN don't fail in
// setsockopt(SO_MARK). Production callers must use NewProxyServer.
//
//nolint:unparam // agentToken is fixed in tests but kept explicit for clarity
func newProxyServerTo(backendURL, agentToken string, creds *Credential, tracker *Tracker) *ProxyServer {
	return newProxyServer("127.0.0.1:0", backendURL, http.DefaultTransport, agentToken, creds, tracker)
}

func newProxyServer(
	bindAddr, upstreamURL string,
	transport http.RoundTripper,
	agentToken string,
	creds *Credential,
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
			// Rewrite the path: the client sends /v1/responses but the
			// ChatGPT backend endpoint is /backend-api/codex/responses.
			r.Out.URL.Path = chatgptResponsesPath
			if tracker != nil {
				// Force identity so the usage parser sees raw SSE bytes —
				// the backend may otherwise respond with gzip, silently
				// breaking every usage parse.
				r.Out.Header.Set("Accept-Encoding", "identity")
			}
		},
		Transport:     transport,
		ErrorLog:      log.New(log.Writer(), "openai-proxy: ", log.LstdFlags),
		FlushInterval: -1, // stream SSE events immediately
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
			Handler:           gatingHandler(rp, agentToken, creds),
			ReadHeaderTimeout: 30 * time.Second,
		},
	}
}

// gatingHandler wraps the reverse proxy with the endpoint allowlist, the
// agent-token check, and OAuth token resolution. Only POST /v1/responses
// is permitted. On a valid request the handler fetches a fresh access token
// from creds (refreshing if needed) and stamps all outbound auth and routing
// headers before forwarding to the reverse proxy.
func gatingHandler(next http.Handler, agentToken string, creds *Credential) http.Handler {
	expectedAuth := bearerPrefix + agentToken
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths, ok := allowedEndpoints[r.Method]
		if !ok {
			proxycommon.Deny(w, r, http.StatusForbidden, "method not allowed", "openai proxy")
			return
		}
		if _, ok := paths[r.URL.Path]; !ok {
			proxycommon.Deny(w, r, http.StatusForbidden, "path not allowed", "openai proxy")
			return
		}
		if r.Header.Get(headerAuthorization) != expectedAuth {
			proxycommon.Deny(w, r, http.StatusUnauthorized, "agent token mismatch", "openai proxy")
			return
		}
		tok, err := creds.AccessToken(r.Context())
		if err != nil {
			proxycommon.Deny(w, r, http.StatusBadGateway, "upstream auth refresh failed", "openai proxy")
			return
		}
		r.Header.Set(headerAuthorization, bearerPrefix+tok)
		if id := creds.AccountID(); id != "" {
			r.Header.Set(headerAccountID, id)
		}
		r.Header.Set(headerOriginator, originatorValue)
		r.Header.Set(headerVersion, codexVersion)
		r.Header.Set(headerUserAgent, userAgentValue)
		next.ServeHTTP(w, r)
	})
}

// Start binds and serves until ctx is cancelled, then gracefully
// shuts the server down. Returns nil on clean shutdown; other errors
// are surfaced.
func (p *ProxyServer) Start(ctx context.Context) error {
	return proxycommon.Serve(ctx, p.bindAddr, p.server, "openai proxy")
}
