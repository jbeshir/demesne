package openai

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
	testAgentToken  = "test-agent-token"
	testUpstreamKey = "test-upstream-key"
)

// TestProxyAllowedRequestSwapsToken confirms that an allowed
// (method, path) pair authenticated with the agent token is forwarded
// upstream with the real upstream key and the Host rewritten.
func TestProxyAllowedRequestSwapsToken(t *testing.T) {
	var got struct {
		path        string
		method      string
		authHeader  string
		body        string
		host        string
		gotAuthSeen bool
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path = r.URL.Path
		got.method = r.Method
		got.authHeader = r.Header.Get(headerAuthorization)
		_, got.gotAuthSeen = r.Header[headerAuthorization]
		got.host = r.Host
		body, _ := io.ReadAll(r.Body)
		got.body = string(body)
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("upstream response"))
	}))
	defer upstream.Close()

	p := newProxyServerTo(upstream.URL, testAgentToken, testUpstreamKey, nil)
	tsrv := httptest.NewServer(p.server.Handler)
	defer tsrv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tsrv.URL+pathResponses,
		strings.NewReader("hello body"))
	require.NoError(t, err)
	req.Header.Set(headerAuthorization, bearerPrefix+testAgentToken)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusTeapot, resp.StatusCode)
	respBody, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "upstream response", string(respBody))

	assert.Equal(t, pathResponses, got.path)
	assert.Equal(t, http.MethodPost, got.method)
	assert.Equal(t, bearerPrefix+testUpstreamKey, got.authHeader, "agent token must be swapped for upstream key")
	assert.Equal(t, "hello body", got.body)
	assert.True(t, strings.HasSuffix(got.host, upstreamHostPort(upstream.URL)))
}

// TestProxyAllowsCompact confirms the second whitelisted endpoint
// (POST /v1/responses/compact) is also forwarded.
func TestProxyAllowsCompact(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	p := newProxyServerTo(upstream.URL, testAgentToken, testUpstreamKey, nil)
	tsrv := httptest.NewServer(p.server.Handler)
	defer tsrv.Close()

	req := mustRequest(t, http.MethodPost, tsrv.URL+pathResponsesCompact)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestProxyDeniesUnknownPath confirms paths outside the allowlist
// return 403 without hitting the upstream.
func TestProxyDeniesUnknownPath(t *testing.T) {
	upstreamHit := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	p := newProxyServerTo(upstream.URL, testAgentToken, testUpstreamKey, nil)
	tsrv := httptest.NewServer(p.server.Handler)
	defer tsrv.Close()

	for _, path := range []string{"/v1/chat/completions", "/v1/models", "/admin/keys", "/"} {
		req := mustRequest(t, http.MethodPost, tsrv.URL+path)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "path %s", path)
	}
	assert.False(t, upstreamHit, "denied requests must not reach upstream")
}

// TestProxyDeniesUnknownMethod confirms that non-POST, non-WS-GET
// methods to even whitelisted paths return 403.
func TestProxyDeniesUnknownMethod(t *testing.T) {
	upstreamHit := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	p := newProxyServerTo(upstream.URL, testAgentToken, testUpstreamKey, nil)
	tsrv := httptest.NewServer(p.server.Handler)
	defer tsrv.Close()

	for _, method := range []string{http.MethodPut, http.MethodDelete, http.MethodPatch} {
		req := mustRequest(t, method, tsrv.URL+pathResponses)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "method %s", method)
	}
	assert.False(t, upstreamHit)
}

// TestProxyDeniesPlainGet confirms a plain GET /v1/responses without
// WebSocket Upgrade headers returns 403.
func TestProxyDeniesPlainGet(t *testing.T) {
	upstreamHit := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	p := newProxyServerTo(upstream.URL, testAgentToken, testUpstreamKey, nil)
	tsrv := httptest.NewServer(p.server.Handler)
	defer tsrv.Close()

	req := mustRequest(t, http.MethodGet, tsrv.URL+pathResponses)
	// No Upgrade or Connection headers.
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "plain GET must be denied")
	assert.False(t, upstreamHit)
}

// TestProxyAllowsWebSocketUpgrade confirms that a GET /v1/responses
// carrying Connection: Upgrade and Upgrade: websocket headers passes the
// gate and reaches the upstream. A full WebSocket handshake is not
// exercised here — we only assert the gate does not reject the request.
func TestProxyAllowsWebSocketUpgrade(t *testing.T) {
	upstreamHit := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	p := newProxyServerTo(upstream.URL, testAgentToken, testUpstreamKey, nil)
	tsrv := httptest.NewServer(p.server.Handler)
	defer tsrv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tsrv.URL+pathResponses, nil)
	require.NoError(t, err)
	req.Header.Set(headerAuthorization, bearerPrefix+testAgentToken)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.NotEqual(t, http.StatusForbidden, resp.StatusCode, "WebSocket upgrade GET must not be denied")
	assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode)
	assert.True(t, upstreamHit, "gate must forward the upgrade request to upstream")
}

// TestProxyDeniesWrongToken confirms a request with a non-matching
// Authorization header (or none at all) returns 401 and never reaches
// the upstream.
func TestProxyDeniesWrongToken(t *testing.T) {
	upstreamHit := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	p := newProxyServerTo(upstream.URL, testAgentToken, testUpstreamKey, nil)
	tsrv := httptest.NewServer(p.server.Handler)
	defer tsrv.Close()

	cases := []struct {
		name   string
		header string
	}{
		{name: "missing", header: ""},
		{name: "wrong token", header: "Bearer not-the-token"},
		{name: "real upstream key leaked back", header: bearerPrefix + testUpstreamKey},
		{name: "wrong scheme", header: "Basic " + testAgentToken},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodPost,
				tsrv.URL+pathResponses, strings.NewReader("{}"))
			require.NoError(t, err)
			if tc.header != "" {
				req.Header.Set(headerAuthorization, tc.header)
			}
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			_ = resp.Body.Close()
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
	assert.False(t, upstreamHit)
}

// TestProxyShutdown confirms Start returns cleanly when its context is cancelled.
func TestProxyShutdown(t *testing.T) {
	p := newProxyServerTo("http://127.0.0.1:1", testAgentToken, testUpstreamKey, nil)

	startCtx, startCancel := context.WithCancel(context.Background())
	defer startCancel()
	done := make(chan error, 1)
	go func() { done <- p.Start(startCtx) }()

	time.Sleep(50 * time.Millisecond)
	startCancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after context cancellation")
	}
}

// TestProxyTracksUsageFromSSE confirms a streaming response with
// Responses-API-shaped SSE events updates the tracker's cost.
func TestProxyTracksUsageFromSSE(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentTypeEventStream)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sseFixture))
	}))
	defer upstream.Close()

	tracker := NewTracker("")
	p := newProxyServerTo(upstream.URL, testAgentToken, testUpstreamKey, tracker)
	tsrv := httptest.NewServer(p.server.Handler)
	defer tsrv.Close()

	req := mustRequest(t, http.MethodPost, tsrv.URL+pathResponses)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Contains(t, string(body), "response.completed", "body must pass through to caller")

	snap := tracker.snapshot()
	// gpt-5.5 placeholder: 200k uncached input @ $1.25/MTok + 1M cached @ $0.125/MTok + 345k output @ $10/MTok
	// = 0.00025 + 0.000125 + 0.00345 = $0.003825 (approximate; just assert non-zero)
	assert.Greater(t, float64(snap.CostUSD), 0.0, "cost must be recorded after SSE stream")
}

func mustRequest(t *testing.T, method, url string) *http.Request {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader("{}"))
	require.NoError(t, err)
	req.Header.Set(headerAuthorization, bearerPrefix+testAgentToken)
	return req
}

func upstreamHostPort(rawURL string) string {
	trim := strings.TrimPrefix(rawURL, "http://")
	trim = strings.TrimPrefix(trim, "https://")
	return trim
}
