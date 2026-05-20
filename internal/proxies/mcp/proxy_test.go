package mcp

import (
	"context"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func postJSON(t *testing.T, url, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func TestParseBindings(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		raw := `[{"name":"workflowy","listen_port":8089,"path":"/workflowy/mcp"}]`
		got, err := ParseBindings(raw)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "workflowy", got[0].Name)
		assert.Equal(t, 8089, got[0].ListenPort)
		assert.Equal(t, "/workflowy/mcp", got[0].Path)
	})
	t.Run("empty string yields nil", func(t *testing.T) {
		got, err := ParseBindings("")
		require.NoError(t, err)
		assert.Nil(t, got)
	})
	t.Run("malformed json", func(t *testing.T) {
		_, err := ParseBindings(`[not json]`)
		assert.Error(t, err)
	})
	t.Run("missing fields", func(t *testing.T) {
		_, err := ParseBindings(`[{"name":"x"}]`)
		assert.ErrorContains(t, err, "required")
	})
}

// startUnixUpstream serves the given handler on a unix socket in a
// temp dir and returns the socket path.
func startUnixUpstream(t *testing.T, handler http.Handler) string {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "up.sock")
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "unix", sock)
	require.NoError(t, err)
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})
	return sock
}

// startTunnel boots a tunnel for one binding pointing at the given
// unix socket + path on an OS-assigned listen port, returning that port.
func startTunnel(t *testing.T, socketPath, upstreamPath string) (int, context.CancelFunc) {
	t.Helper()
	port := freePort(t)
	b := Binding{Name: "test", ListenPort: port, Path: upstreamPath}
	srv := NewServer(socketPath, []Binding{b})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Start(ctx) }()
	waitListening(t, port)
	return port, cancel
}

func TestServer_ForwardsToUpstream(t *testing.T) {
	var gotPath, gotBody, gotMethod string
	sock := startUnixUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		_, _ = w.Write([]byte("pong"))
	}))

	port, cancel := startTunnel(t, sock, "/workflowy/mcp")
	defer cancel()

	resp := postJSON(t, localURL(port), `{"jsonrpc":"2.0"}`)
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "pong", string(body))
	assert.Equal(t, "/workflowy/mcp", gotPath)
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.JSONEq(t, `{"jsonrpc":"2.0"}`, gotBody)
}

func TestServer_InjectsParentHeader(t *testing.T) {
	var gotHeader string
	sock := startUnixUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get(ParentHeader)
		w.WriteHeader(http.StatusOK)
	}))

	port := freePort(t)
	b := Binding{Name: "demesne", ListenPort: port, Path: "/demesne/mcp", ParentJobID: "job-123"}
	srv := NewServer(sock, []Binding{b})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = srv.Start(ctx) }()
	waitListening(t, port)

	// Client supplies its own (spoofed) header; the tunnel must overwrite it.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, localURL(port), strings.NewReader("{}"))
	require.NoError(t, err)
	req.Header.Set(ParentHeader, "spoofed")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, "job-123", gotHeader)
}

func TestServer_NoParentHeaderWhenUnset(t *testing.T) {
	var present bool
	sock := startUnixUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, present = r.Header[ParentHeader]
		w.WriteHeader(http.StatusOK)
	}))

	port, cancel := startTunnel(t, sock, "/x/mcp") // no ParentJobID
	defer cancel()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, localURL(port), strings.NewReader("{}"))
	require.NoError(t, err)
	req.Header.Set(ParentHeader, "spoofed")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.False(t, present, "client-supplied parent header must be stripped when binding has none")
}

func TestServer_RejectsBadMethod(t *testing.T) {
	sock := startUnixUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	port, cancel := startTunnel(t, sock, "/x/mcp")
	defer cancel()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPut, localURL(port), nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestServer_NoBindingsBlocksUntilCancel(t *testing.T) {
	srv := NewServer("", nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Start(ctx) }()
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after cancel")
	}
}
