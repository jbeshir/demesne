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

	p := newProxyServerTo(upstream.URL, testAgentToken, realToken, testAccountID, nil)
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

	p := newProxyServerTo(upstream.URL, testAgentToken, "any-token", testAccountID, nil)
	tsrv := httptest.NewServer(p.server.Handler)
	defer tsrv.Close()

	for _, path := range []string{
		"/v1/chat/completions", "/v1/models", "/admin/keys", "/",
		"/v1/responses/compact",
	} {
		req := mustRequest(t, http.MethodPost, tsrv.URL+path)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "path %s", path)
	}
	assert.False(t, upstreamHit, "denied requests must not reach upstream")
}

// TestProxyDeniesUnknownMethod confirms non-POST methods, including GET
// (WebSocket upgrades no longer permitted), return 403.
func TestProxyDeniesUnknownMethod(t *testing.T) {
	upstreamHit := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	p := newProxyServerTo(upstream.URL, testAgentToken, "any-token", testAccountID, nil)
	tsrv := httptest.NewServer(p.server.Handler)
	defer tsrv.Close()

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		req := mustRequest(t, method, tsrv.URL+pathResponses)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "method %s", method)
	}
	assert.False(t, upstreamHit)
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

	p := newProxyServerTo(upstream.URL, testAgentToken, "real-access-token", testAccountID, nil)
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

	p := newProxyServerTo(upstream.URL, testAgentToken, "tok", "", nil)
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
	p := newProxyServerTo("http://127.0.0.1:1", testAgentToken, "tok", testAccountID, nil)

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
	p := newProxyServerTo(upstream.URL, testAgentToken, "tok", testAccountID, tracker)
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
	assert.Greater(t, float64(snap.CostUSD), 0.0, "cost must be recorded after SSE stream")
}

// mustRequest builds a POST request with the agent token and a 5s timeout.
func mustRequest(t *testing.T, method, rawURL string) *http.Request {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, method, rawURL, strings.NewReader("{}"))
	require.NoError(t, err)
	req.Header.Set(headerAuthorization, bearerPrefix+testAgentToken)
	return req
}

func upstreamHostPort(rawURL string) string {
	trim := strings.TrimPrefix(rawURL, "http://")
	trim = strings.TrimPrefix(trim, "https://")
	return trim
}
