package server

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jbeshir/demesne/internal/sandbox"
	"github.com/mark3labs/mcp-go/mcp"
)

type fakeRunner struct {
	calls  int
	gotReq sandbox.ScriptRequest
	res    sandbox.ScriptResult
	err    error
}

func (f *fakeRunner) RunScript(_ context.Context, req sandbox.ScriptRequest) (sandbox.ScriptResult, error) {
	f.calls++
	f.gotReq = req
	return f.res, f.err
}

func newRequest(args map[string]any) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	req.Params.Name = "sandbox_script"
	req.Params.Arguments = args
	return req
}

func resultText(t *testing.T, r *mcp.CallToolResult) string {
	t.Helper()
	if r == nil {
		t.Fatal("nil result")
	}
	if len(r.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := r.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content is %T, want TextContent", r.Content[0])
	}
	return tc.Text
}

func TestHandleSandboxScript_MissingCommand(t *testing.T) {
	r := &fakeRunner{}
	s := NewServer(r)
	got, err := s.handleSandboxScript(context.Background(), newRequest(map[string]any{}))
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsError {
		t.Fatal("expected IsError=true for missing command")
	}
	if r.calls != 0 {
		t.Fatalf("runner should not be called, got %d calls", r.calls)
	}
}

func TestHandleSandboxScript_InvalidFiles(t *testing.T) {
	r := &fakeRunner{}
	s := NewServer(r)
	got, err := s.handleSandboxScript(context.Background(), newRequest(map[string]any{
		"command": "true",
		"files":   "not-an-array",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsError {
		t.Fatal("expected IsError=true for non-array files")
	}
	if r.calls != 0 {
		t.Fatal("runner should not be called when args invalid")
	}
}

func TestHandleSandboxScript_RunnerErrorSurfaced(t *testing.T) {
	r := &fakeRunner{err: errors.New("boom")}
	s := NewServer(r)
	got, err := s.handleSandboxScript(context.Background(), newRequest(map[string]any{
		"command": "true",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsError {
		t.Fatal("expected IsError=true when runner fails")
	}
	if !strings.Contains(resultText(t, got), "boom") {
		t.Fatalf("expected error text to contain runner error, got %q", resultText(t, got))
	}
}

func TestHandleSandboxScript_HappyPath(t *testing.T) {
	r := &fakeRunner{
		res: sandbox.ScriptResult{
			JobID:      "abc-123",
			OutputPath: "/tmp/demesne/out/abc-123",
			Stdout:     "hello\n",
			ExitCode:   0,
		},
	}
	s := NewServer(r)
	got, err := s.handleSandboxScript(context.Background(), newRequest(map[string]any{
		"command":     "echo hello",
		"image":       "anaconda",
		"egress":      "none",
		"files":       []any{"/some/file.txt"},
		"directories": []any{"/some/dir"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got.IsError {
		t.Fatalf("unexpected error result: %s", resultText(t, got))
	}
	if r.calls != 1 {
		t.Fatalf("expected 1 runner call, got %d", r.calls)
	}
	wantReq := sandbox.ScriptRequest{
		Command:     "echo hello",
		Image:       "anaconda",
		Egress:      sandbox.EgressNone,
		Files:       []string{"/some/file.txt"},
		Directories: []string{"/some/dir"},
	}
	if !reflect.DeepEqual(r.gotReq, wantReq) {
		t.Errorf("runner request = %+v\nwant %+v", r.gotReq, wantReq)
	}

	text := resultText(t, got)
	for _, want := range []string{"exit_code: 0", "output_dir: /tmp/demesne/out/abc-123", "job_id: abc-123", "hello"} {
		if !strings.Contains(text, want) {
			t.Errorf("result missing %q\nfull:\n%s", want, text)
		}
	}
}

func TestHandleSandboxScript_DefaultEgress(t *testing.T) {
	r := &fakeRunner{}
	s := NewServer(r)
	_, err := s.handleSandboxScript(context.Background(), newRequest(map[string]any{
		"command": "true",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if r.gotReq.Egress != sandbox.EgressPackageManagers {
		t.Fatalf("default egress = %q, want %q", r.gotReq.Egress, sandbox.EgressPackageManagers)
	}
}
