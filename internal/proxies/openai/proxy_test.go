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

const testAgentToken = "test-agent-token"
const testAccountID = "test-account-id"

// newProxyServerTo is the test-only constructor: forwards to the given
// backend URL over http.DefaultTransport, so tests that exercise the
// gating logic on a host without CAP_NET_ADMIN don't fail in
// setsockopt(SO_MARK). Production callers must use NewProxyServer. The
// agent token is always the package test constant.
func newProxyServerTo(backendURL, accessToken, accountID string, tracker *Tracker) *ProxyServer {
	return newProxyServer("127.0.0.1:0", backendURL, http.DefaultTransport, testAgentToken, accessToken, accountID, tracker)
}

// TestProxyAllowedRequestRewritesHeaders confirms that a valid POST
// /v1/responses is forwarded with the rewritten backend path,
// Authorization swapped to the real OAuth token, and routing headers set.
func TestProxyAllowedRequestRewritesHeaders(t *testing.T) {
	const realToken = "real-access-token"
	var got struct {
		path       string
		method     string
		authHeader string
		accountID  string
		originator string
		version    string
		userAgent  string
		body       string
		host       string
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path = r.URL.Path
		got.method = r.Method
		got.authHeader = r.Header.Get(headerAuthorization)
		got.accountID = r.Header.Get(headerAccountID)
		got.originator = r.Header.Get(headerOriginator)
		got.version = r.Header.Get(headerVersion)
		got.userAgent = r.Header.Get(headerUserAgent)
		got.host = r.Host
		body, _ := io.ReadAll(r.Body)
		got.body = string(body)
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("upstream response"))
	}))
	defer upstream.Close()

	p := newProxyServerTo(upstream.URL, realToken, testAccountID, nil)
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

	assert.Equal(t, chatgptResponsesPath, got.path, "path must be rewritten to the ChatGPT backend path")
	assert.Equal(t, http.MethodPost, got.method)
	assert.Equal(t, bearerPrefix+realToken, got.authHeader, "agent token must be replaced with real OAuth token")
	assert.Equal(t, "test-account-id", got.accountID, "ChatGPT-Account-ID must be forwarded")
	assert.Equal(t, originatorValue, got.originator)
	assert.Equal(t, codexVersion, got.version)
	assert.Equal(t, userAgentValue, got.userAgent)
	assert.Equal(t, "hello body", got.body)
	assert.True(t, strings.HasSuffix(got.host, upstreamHostPort(upstream.URL)))
}

func TestProxyCatalogRequestRewritesHeadersAndQuery(t *testing.T) {
	const realToken = "real-access-token"
	var got struct {
		path           string
		method         string
		rawQuery       string
		authHeader     string
		accountID      string
		version        string
		userAgent      string
		acceptEncoding string
		body           string
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path = r.URL.Path
		got.method = r.Method
		got.rawQuery = r.URL.RawQuery
		got.authHeader = r.Header.Get(headerAuthorization)
		got.accountID = r.Header.Get(headerAccountID)
		got.version = r.Header.Get(headerVersion)
		got.userAgent = r.Header.Get(headerUserAgent)
		got.acceptEncoding = r.Header.Get("Accept-Encoding")
		body, _ := io.ReadAll(r.Body)
		got.body = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"models":[{"slug":"gpt-test"}]}`))
	}))
	defer upstream.Close()

	tracker := NewTracker("")
	p := newProxyServerTo(upstream.URL, realToken, testAccountID, tracker)
	tsrv := httptest.NewServer(p.server.Handler)
	defer tsrv.Close()

	req := mustGetRequest(t, tsrv.URL+pathModels+"?client_version=0.144.3")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.JSONEq(t, `{"models":[{"slug":"gpt-test"}]}`, string(body))
	assert.Equal(t, chatgptModelsPath, got.path)
	assert.Equal(t, http.MethodGet, got.method)
	assert.Equal(t, "client_version=0.144.3", got.rawQuery)
	assert.Equal(t, bearerPrefix+realToken, got.authHeader)
	assert.Equal(t, testAccountID, got.accountID)
	assert.Equal(t, codexVersion, got.version)
	assert.Equal(t, userAgentValue, got.userAgent)
	assert.Equal(t, "identity", got.acceptEncoding)
	assert.Empty(t, got.body)
	assert.Zero(t, tracker.snapshot().CostUSD, "catalog responses must not be parsed as usage")
}

// assertMethodsDenied confirms that each of the given methods against path
// returns 403 without ever reaching the upstream.
func assertMethodsDenied(t *testing.T, path string, methods []string) {
	t.Helper()
	upstreamHit := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	p := newProxyServerTo(upstream.URL, "any-token", testAccountID, nil)
	tsrv := httptest.NewServer(p.server.Handler)
	defer tsrv.Close()

	for _, method := range methods {
		req := mustRequest(t, method, tsrv.URL+path)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "method %s", method)
	}
	assert.False(t, upstreamHit)
}

// TestProxyModelsDeniesWrongMethod confirms only GET is permitted on
// /backend-api/codex/models; the catalog fetch has no POST/PUT/DELETE/PATCH form.
func TestProxyModelsDeniesWrongMethod(t *testing.T) {
	assertMethodsDenied(t, pathModels, []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch})
}

// TestProxyModelsDeniesWrongToken confirms GET /backend-api/codex/models
// enforces the same agent-token check as /v1/responses.
func TestProxyModelsDeniesWrongToken(t *testing.T) {
	upstreamHit := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	p := newProxyServerTo(upstream.URL, "real-access-token", testAccountID, nil)
	tsrv := httptest.NewServer(p.server.Handler)
	defer tsrv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tsrv.URL+pathModels, nil)
	require.NoError(t, err)
	req.Header.Set(headerAuthorization, "Bearer not-the-token")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.False(t, upstreamHit)
}

// TestProxyDeniesUnknownPath confirms paths outside the allowlist
// return 403 without hitting the upstream. This includes /v1/responses/compact
// which was removed from the allowlist.
func TestProxyDeniesUnknownPath(t *testing.T) {
	upstreamHit := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	p := newProxyServerTo(upstream.URL, "any-token", testAccountID, nil)
	tsrv := httptest.NewServer(p.server.Handler)
	defer tsrv.Close()

	for _, path := range []string{
		"/v1/chat/completions", "/v1/models", "/admin/keys", "/",
		"/v1/responses/compact",
		"/backend-api/codex/other",
	} {
		req := mustRequest(t, http.MethodPost, tsrv.URL+path)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "path %s", path)
	}
	assert.False(t, upstreamHit, "denied requests must not reach upstream")
}

// TestProxyDeniesUnknownMethod confirms methods outside the per-path allowlist
// return 403.
func TestProxyDeniesUnknownMethod(t *testing.T) {
	assertMethodsDenied(t, pathResponses, []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch})
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

	p := newProxyServerTo(upstream.URL, "real-access-token", testAccountID, nil)
	tsrv := httptest.NewServer(p.server.Handler)
	defer tsrv.Close()

	cases := []struct {
		name   string
		header string
	}{
		{name: "missing", header: ""},
		{name: "wrong token", header: "Bearer not-the-token"},
		{name: "real access token leaked back", header: bearerPrefix + "real-access-token"},
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

// TestProxyOmitsAccountIDWhenEmpty confirms ChatGPT-Account-ID is not
// sent when the credential has no account ID.
func TestProxyOmitsAccountIDWhenEmpty(t *testing.T) {
	accountIDSeen := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accountIDSeen = r.Header.Get(headerAccountID) != ""
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	p := newProxyServerTo(upstream.URL, "tok", "", nil)
	tsrv := httptest.NewServer(p.server.Handler)
	defer tsrv.Close()

	req := mustRequest(t, http.MethodPost, tsrv.URL+pathResponses)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.False(t, accountIDSeen, "ChatGPT-Account-ID must not be sent when account ID is empty")
}

// TestProxyShutdown confirms Start returns cleanly when its context is cancelled.
func TestProxyShutdown(t *testing.T) {
	p := newProxyServerTo("http://127.0.0.1:1", "tok", testAccountID, nil)

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
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sseFixture))
	}))
	defer upstream.Close()

	tracker := NewTracker("")
	p := newProxyServerTo(upstream.URL, "tok", testAccountID, tracker)
	tsrv := httptest.NewServer(p.server.Handler)
	defer tsrv.Close()

	req := mustRequest(t, http.MethodPost, tsrv.URL+pathResponses)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Contains(t, string(body), "response.completed", "body must pass through to caller")

	// httputil.ReverseProxy records usage when it closes the upstream-wrapped
	// body (sseInterceptor.flush), which runs in its request goroutine after
	// the copy to the client completes — asynchronously to the client's read
	// finishing. Poll until the cost lands rather than asserting immediately,
	// which otherwise races that close and intermittently reads 0.
	assert.Eventually(t, func() bool {
		return tracker.snapshot().CostUSD > 0
	}, time.Second, time.Millisecond, "cost must be recorded after the SSE stream is closed")
}

// mustRequest builds a request carrying a "{}" body (POST/PUT/PATCH's usual
// shape) with the agent token and a 5s timeout.
func mustRequest(t *testing.T, method, rawURL string) *http.Request {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, method, rawURL, strings.NewReader("{}"))
	require.NoError(t, err)
	req.Header.Set(headerAuthorization, bearerPrefix+testAgentToken)
	return req
}

// mustGetRequest builds a bodyless GET request with the agent token and a 5s
// timeout, matching the real Codex CLI's catalog fetch (no request body).
func mustGetRequest(t *testing.T, rawURL string) *http.Request {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	require.NoError(t, err)
	req.Header.Set(headerAuthorization, bearerPrefix+testAgentToken)
	return req
}

func upstreamHostPort(rawURL string) string {
	trim := strings.TrimPrefix(rawURL, "http://")
	trim = strings.TrimPrefix(trim, "https://")
	return trim
}
