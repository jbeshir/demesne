package goproxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func freePort(t *testing.T) int {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), tcpNetwork, "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())
	return port
}

func waitListening(t *testing.T, addr string) {
	t.Helper()
	var d net.Dialer
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		conn, err := d.DialContext(ctx, tcpNetwork, addr)
		cancel()
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("proxy never started listening on %s", addr)
}

func startProxy(t *testing.T, upstreamURL string) string {
	t.Helper()
	addr := fmt.Sprintf("127.0.0.1:%d", freePort(t))
	p := NewProxyServerTo(addr, upstreamURL)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = p.Start(ctx) }()
	waitListening(t, addr)
	return addr
}

func TestProxy_ForwardsModuleAndSumdb(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo the (test-controlled) path so the assertion can confirm it
		// was forwarded unchanged.
		_, _ = w.Write([]byte("ok:" + r.URL.Path)) //nolint:gosec // G705: path is test input, not attacker-controlled
	}))
	defer upstream.Close()

	addr := startProxy(t, upstream.URL)

	// A module path and the sumdb path both forward unchanged.
	for _, path := range []string{
		"/github.com/foo/bar/@v/list",
		"/sumdb/sum.golang.org/lookup/github.com/foo/bar@v1.0.0",
	} {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr+path, nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode, path)
		assert.Equal(t, "ok:"+path, string(body), "path forwarded unchanged")
	}
}

func TestProxy_FollowsRedirectInternally(t *testing.T) {
	// proxy.golang.org serves large module zips by redirecting to a CDN
	// (storage.googleapis.com) the sandbox can't resolve. The proxy must
	// follow the redirect itself and return the final body.
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("zip-bytes"))
	}))
	defer final.Close()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".zip") {
			http.Redirect(w, r, final.URL+"/cdn/blob", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("ok:" + r.URL.Path)) //nolint:gosec // G705: path is test input
	}))
	defer upstream.Close()

	addr := startProxy(t, upstream.URL)

	// Disable client-side redirect following so the assertion proves the
	// proxy (not this client) followed the redirect.
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		"http://"+addr+"/github.com/foo/bar/@v/v1.0.0.zip", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "proxy should follow the redirect, not relay it")
	assert.Equal(t, "zip-bytes", string(body))
}

func TestProxy_RejectsNonGet(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	addr := startProxy(t, upstream.URL)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://"+addr+"/x", strings.NewReader("{}"))
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}
