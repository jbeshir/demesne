package anthropic

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testAgentToken    = "test-agent-token"
	testUpstreamToken = "test-upstream-token"
)

// TestProxyAllowedRequestSwapsToken confirms that an allowed
// (method, path) pair authenticated with the agent token is forwarded
// upstream with the real upstream token, the Host rewritten, and
// x-api-key stripped.
func TestProxyAllowedRequestSwapsToken(t *testing.T) {
	var got struct {
		path        string
		method      string
		authHeader  string
		apiKey      string
		anthropicH  string
		body        string
		host        string
		gotXAPIKey  bool
		gotAuthSeen bool
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path = r.URL.Path
		got.method = r.Method
		got.authHeader = r.Header.Get("Authorization")
		_, got.gotAuthSeen = r.Header["Authorization"]
		got.apiKey = r.Header.Get("x-api-key")
		_, got.gotXAPIKey = r.Header["X-Api-Key"]
		got.anthropicH = r.Header.Get("anthropic-version")
		got.host = r.Host
		body, _ := io.ReadAll(r.Body)
		got.body = string(body)
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("upstream response"))
	}))
	defer upstream.Close()

	p := NewProxyServerTo("127.0.0.1:0", upstream.URL, testAgentToken, testUpstreamToken, nil)
	tsrv := httptest.NewServer(p.server.Handler)
	defer tsrv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tsrv.URL+"/v1/messages",
		strings.NewReader("hello body"))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+testAgentToken)
	req.Header.Set("x-api-key", "stolen-key")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusTeapot, resp.StatusCode)
	respBody, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "upstream response", string(respBody))

	assert.Equal(t, "/v1/messages", got.path)
	assert.Equal(t, http.MethodPost, got.method)
	assert.Equal(t, "Bearer "+testUpstreamToken, got.authHeader, "agent token must be swapped for upstream token")
	assert.False(t, got.gotXAPIKey, "x-api-key must be stripped before forwarding (got %q)", got.apiKey)
	assert.Equal(t, "2023-06-01", got.anthropicH)
	assert.Equal(t, "hello body", got.body)
	assert.True(t, strings.HasSuffix(got.host, upstreamHostPort(upstream.URL)))
}

// TestProxyAllowsCountTokens confirms the second whitelisted endpoint
// (POST /v1/messages/count_tokens) is also forwarded.
func TestProxyAllowsCountTokens(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	p := NewProxyServerTo("127.0.0.1:0", upstream.URL, testAgentToken, testUpstreamToken, nil)
	tsrv := httptest.NewServer(p.server.Handler)
	defer tsrv.Close()

	req := mustRequest(t, http.MethodPost, tsrv.URL+"/v1/messages/count_tokens")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestProxyDeniesUnknownPath confirms paths outside the allowlist
// (here a Files API endpoint) return 403 without hitting the upstream.
func TestProxyDeniesUnknownPath(t *testing.T) {
	upstreamHit := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	p := NewProxyServerTo("127.0.0.1:0", upstream.URL, testAgentToken, testUpstreamToken, nil)
	tsrv := httptest.NewServer(p.server.Handler)
	defer tsrv.Close()

	for _, path := range []string{"/v1/files", "/v1/messages/batches", "/admin/keys", "/"} {
		req := mustRequest(t, http.MethodPost, tsrv.URL+path)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "path %s", path)
	}
	assert.False(t, upstreamHit, "denied requests must not reach upstream")
}

// TestProxyDeniesUnknownMethod confirms that non-POST methods (GET, etc.)
// to even whitelisted paths return 403.
func TestProxyDeniesUnknownMethod(t *testing.T) {
	upstreamHit := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	p := NewProxyServerTo("127.0.0.1:0", upstream.URL, testAgentToken, testUpstreamToken, nil)
	tsrv := httptest.NewServer(p.server.Handler)
	defer tsrv.Close()

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		req := mustRequest(t, method, tsrv.URL+"/v1/messages")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "method %s", method)
	}
	assert.False(t, upstreamHit)
}

// TestProxyDeniesWrongToken confirms a request with a non-matching
// Authorization header (or none at all) returns 401 and never reaches
// the upstream. The real upstream token must never reach Anthropic on
// behalf of a request that didn't authenticate as the agent.
func TestProxyDeniesWrongToken(t *testing.T) {
	upstreamHit := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	p := NewProxyServerTo("127.0.0.1:0", upstream.URL, testAgentToken, testUpstreamToken, nil)
	tsrv := httptest.NewServer(p.server.Handler)
	defer tsrv.Close()

	cases := []struct {
		name   string
		header string
	}{
		{name: "missing", header: ""},
		{name: "wrong token", header: "Bearer not-the-token"},
		{name: "real upstream token leaked back", header: "Bearer " + testUpstreamToken},
		{name: "wrong scheme", header: "Basic " + testAgentToken},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodPost,
				tsrv.URL+"/v1/messages", strings.NewReader("{}"))
			require.NoError(t, err)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			_ = resp.Body.Close()
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
	assert.False(t, upstreamHit)
}

// TestProxyShutdown confirms Shutdown stops the server cleanly.
func TestProxyShutdown(t *testing.T) {
	p := NewProxyServerTo("127.0.0.1:0", "http://127.0.0.1:1", testAgentToken, testUpstreamToken, nil)

	startCtx, startCancel := context.WithCancel(context.Background())
	defer startCancel()
	done := make(chan error, 1)
	go func() { done <- p.Start(startCtx) }()

	time.Sleep(50 * time.Millisecond)
	shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, p.Shutdown(shutCtx))
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after Shutdown")
	}
}

func mustRequest(t *testing.T, method, url string) *http.Request {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader("{}"))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+testAgentToken)
	return req
}

func upstreamHostPort(rawURL string) string {
	trim := strings.TrimPrefix(rawURL, "http://")
	trim = strings.TrimPrefix(trim, "https://")
	return trim
}

// TestProxyTracksUsageFromSSE confirms a streaming response with
// Anthropic-shaped SSE events updates the tracker's cost.
func TestProxyTracksUsageFromSSE(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`event: message_start
data: {"type":"message_start","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":1000,"output_tokens":1}}}

event: message_delta
data: {"type":"message_delta","delta":{},"usage":{"output_tokens":2000}}

event: message_stop
data: {"type":"message_stop"}

`))
	}))
	defer upstream.Close()

	tracker := NewTracker("")
	p := NewProxyServerTo("127.0.0.1:0", upstream.URL, testAgentToken, testUpstreamToken, tracker)
	tsrv := httptest.NewServer(p.server.Handler)
	defer tsrv.Close()

	req := mustRequest(t, http.MethodPost, tsrv.URL+"/v1/messages")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Contains(t, string(body), "message_stop", "body must pass through to caller")

	snap := tracker.Snapshot()
	// 1k input @ $3/MTok + 2k output @ $15/MTok = $0.003 + $0.030 = $0.033.
	assert.InDelta(t, 0.033, snap.CostUSD, 1e-9)
}
