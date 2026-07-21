package server

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jbeshir/demesne/internal/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type delayedEOFReader struct{ *strings.Reader }

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *lockedBuffer) bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return bytes.Clone(b.b.Bytes())
}

func (r delayedEOFReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if n == 0 && err != nil {
		time.Sleep(50 * time.Millisecond)
	}
	return n, err
}

func TestTerminalNotifierDeliveryFailureIsNonFatal(t *testing.T) {
	// There is deliberately no mcp-go session in this context. The send fails
	// at the closest delivery seam and terminalNotifier must absorb the error.
	s := NewServer(&fakeRunner{})
	assert.NotPanics(t, func() {
		s.terminalNotifier(context.Background())("job-disconnected", sandbox.JobStatusFailed)
	})
}

func TestStdioTerminalNotificationIsJSONRPCFramed(t *testing.T) {
	r := &fakeRunner{
		startScriptJobID: "job-framing",
		notifyOnStart:    true,
	}
	s := NewServer(r)
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":3,"method":"logging/setLevel","params":{"level":"info"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"sandbox_script","arguments":{"command":"true","background":true}}}`,
	}, "\n") + "\n"

	var stdout lockedBuffer
	require.NoError(t, s.serve(context.Background(), delayedEOFReader{strings.NewReader(input)}, &stdout))

	output := stdout.bytes()
	dec := json.NewDecoder(bytes.NewReader(output))
	notifications := 0
	objects := 0
	for dec.More() {
		var message map[string]any
		require.NoError(t, dec.Decode(&message), "stdout must contain only JSON-RPC values: %q", output)
		objects++
		assert.Equal(t, "2.0", message["jsonrpc"])
		if message["method"] == "notifications/message" {
			notifications++
			assert.NotContains(t, message, "id", "notifications must not carry a JSON-RPC id")
		}
	}
	require.GreaterOrEqual(t, objects, 3)
	assert.Equal(t, 1, notifications)
}
