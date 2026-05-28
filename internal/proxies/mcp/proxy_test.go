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

const testWorkflowyPath = "/workflowy/mcp"
const testRequiredErr = "required"

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
	tests := []struct {
		name    string
		raw     string
		want    []Binding
		wantErr string
	}{
		{
			name: "valid",
			raw:  `[{"name":"workflowy","listen_port":8089,"path":"/workflowy/mcp"}]`,
			want: []Binding{{Name: "workflowy", ListenPort: 8089, Path: testWorkflowyPath}},
		},
		{
			name: "multiple bindings preserve order and parent id",
			raw:  `[{"name":"workflowy","listen_port":8089,"path":"/workflowy/mcp","parent_job_id":"job-1"},{"name":"wanikani","listen_port":8090,"path":"/wanikani/mcp"}]`,
			want: []Binding{
				{Name: "workflowy", ListenPort: 8089, Path: testWorkflowyPath, ParentJobID: "job-1"},
				{Name: "wanikani", ListenPort: 8090, Path: "/wanikani/mcp"},
			},
		},
		{
			name: "empty string yields nil",
			raw:  "",
			want: nil,
		},
		{
			name: "null json yields nil",
			raw:  `null`,
			want: nil,
		},
		{
			name:    "malformed json",
			raw:     `[not json]`,
			wantErr: "parse",
		},
		{
			name:    "missing name",
			raw:     `[{"listen_port":8089,"path":"/workflowy/mcp"}]`,
			wantErr: testRequiredErr,
		},
		{
			name:    "missing listen port",
			raw:     `[{"name":"workflowy","path":"/workflowy/mcp"}]`,
			wantErr: testRequiredErr,
		},
		{
			name:    "missing path",
			raw:     `[{"name":"workflowy","listen_port":8089}]`,
			wantErr: testRequiredErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseBindings(tt.raw)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
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

	port, cancel := startTunnel(t, sock, testWorkflowyPath)
	defer cancel()

	resp := postJSON(t, localURL(port), `{"jsonrpc":"2.0"}`)
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "pong", string(body))
	assert.Equal(t, testWorkflowyPath, gotPath)
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
