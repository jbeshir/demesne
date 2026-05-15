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
	scriptCalls    int
	gotScriptReq   sandbox.ScriptRequest
	scriptRes      sandbox.ScriptResult
	scriptErr      error
	createCalls    int
	gotCreateReq   sandbox.CreateRequest
	createRes      sandbox.CreateResult
	createErr      error
	execCalls      int
	gotExecReq     sandbox.ExecRequest
	execRes        sandbox.ExecResult
	execErr        error
	uploadCalls    int
	gotUploadReq   sandbox.UploadRequest
	uploadErr      error
	downloadCalls  int
	gotDownloadReq sandbox.DownloadRequest
	downloadRes    sandbox.DownloadResult
	downloadErr    error
	destroyCalls   int
	gotDestroyReq  sandbox.DestroyRequest
	destroyErr     error
}

func (f *fakeRunner) RunScript(_ context.Context, req sandbox.ScriptRequest) (sandbox.ScriptResult, error) {
	f.scriptCalls++
	f.gotScriptReq = req
	return f.scriptRes, f.scriptErr
}

func (f *fakeRunner) Create(_ context.Context, req sandbox.CreateRequest) (sandbox.CreateResult, error) {
	f.createCalls++
	f.gotCreateReq = req
	return f.createRes, f.createErr
}

func (f *fakeRunner) Exec(_ context.Context, req sandbox.ExecRequest) (sandbox.ExecResult, error) {
	f.execCalls++
	f.gotExecReq = req
	return f.execRes, f.execErr
}

func (f *fakeRunner) Upload(_ context.Context, req sandbox.UploadRequest) error {
	f.uploadCalls++
	f.gotUploadReq = req
	return f.uploadErr
}

func (f *fakeRunner) Download(_ context.Context, req sandbox.DownloadRequest) (sandbox.DownloadResult, error) {
	f.downloadCalls++
	f.gotDownloadReq = req
	return f.downloadRes, f.downloadErr
}

func (f *fakeRunner) Destroy(_ context.Context, req sandbox.DestroyRequest) error {
	f.destroyCalls++
	f.gotDestroyReq = req
	return f.destroyErr
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
	if r.scriptCalls != 0 {
		t.Fatalf("runner should not be called, got %d calls", r.scriptCalls)
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
	if r.scriptCalls != 0 {
		t.Fatal("runner should not be called when args invalid")
	}
}

func TestHandleSandboxScript_RunnerErrorSurfaced(t *testing.T) {
	r := &fakeRunner{scriptErr: errors.New("boom")}
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
		scriptRes: sandbox.ScriptResult{
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
	if r.scriptCalls != 1 {
		t.Fatalf("expected 1 runner call, got %d", r.scriptCalls)
	}
	wantReq := sandbox.ScriptRequest{
		Command:     "echo hello",
		Image:       "anaconda",
		Egress:      sandbox.EgressNone,
		Files:       []string{"/some/file.txt"},
		Directories: []string{"/some/dir"},
	}
	if !reflect.DeepEqual(r.gotScriptReq, wantReq) {
		t.Errorf("runner request = %+v\nwant %+v", r.gotScriptReq, wantReq)
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
	if r.gotScriptReq.Egress != sandbox.EgressPackageManagers {
		t.Fatalf("default egress = %q, want %q", r.gotScriptReq.Egress, sandbox.EgressPackageManagers)
	}
}

func TestHandleSandboxCreate_HappyPath(t *testing.T) {
	r := &fakeRunner{
		createRes: sandbox.CreateResult{
			SandboxID:  "sbx-1",
			OutputPath: "/tmp/demesne/out/job-1",
		},
	}
	s := NewServer(r)
	got, err := s.handleSandboxCreate(context.Background(), newRequest(map[string]any{
		"image":  "python",
		"egress": "none",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got.IsError {
		t.Fatalf("unexpected error result: %s", resultText(t, got))
	}
	if r.createCalls != 1 {
		t.Fatalf("expected 1 Create call, got %d", r.createCalls)
	}
	wantReq := sandbox.CreateRequest{Image: "python", Egress: sandbox.EgressNone}
	if !reflect.DeepEqual(r.gotCreateReq, wantReq) {
		t.Errorf("Create request = %+v\nwant %+v", r.gotCreateReq, wantReq)
	}
	text := resultText(t, got)
	for _, want := range []string{"sandbox_id: sbx-1", "output_dir: /tmp/demesne/out/job-1"} {
		if !strings.Contains(text, want) {
			t.Errorf("result missing %q\nfull:\n%s", want, text)
		}
	}
}

func TestHandleSandboxCreate_DefaultEgress(t *testing.T) {
	r := &fakeRunner{}
	s := NewServer(r)
	_, err := s.handleSandboxCreate(context.Background(), newRequest(map[string]any{}))
	if err != nil {
		t.Fatal(err)
	}
	if r.gotCreateReq.Egress != sandbox.EgressPackageManagers {
		t.Fatalf("default egress = %q, want %q", r.gotCreateReq.Egress, sandbox.EgressPackageManagers)
	}
}

func TestHandleSandboxExec_MissingParams(t *testing.T) {
	cases := []map[string]any{
		{},                                       // missing both
		{"sandbox_id": "sbx-1"},                  // missing command
		{"command": "echo hi"},                   // missing sandbox_id
		{"sandbox_id": "", "command": "echo hi"}, // empty sandbox_id
	}
	for i, args := range cases {
		r := &fakeRunner{}
		s := NewServer(r)
		got, err := s.handleSandboxExec(context.Background(), newRequest(args))
		if err != nil {
			t.Fatal(err)
		}
		if !got.IsError {
			t.Errorf("case %d: expected IsError=true for %v", i, args)
		}
		if r.execCalls != 0 {
			t.Errorf("case %d: runner should not be called", i)
		}
	}
}

func TestHandleSandboxExec_HappyPath(t *testing.T) {
	r := &fakeRunner{
		execRes: sandbox.ExecResult{Stdout: "hello\n", ExitCode: 0},
	}
	s := NewServer(r)
	got, err := s.handleSandboxExec(context.Background(), newRequest(map[string]any{
		"sandbox_id": "sbx-1",
		"command":    "echo hello",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got.IsError {
		t.Fatalf("unexpected error result: %s", resultText(t, got))
	}
	wantReq := sandbox.ExecRequest{SandboxID: "sbx-1", Command: "echo hello"}
	if !reflect.DeepEqual(r.gotExecReq, wantReq) {
		t.Errorf("Exec request = %+v\nwant %+v", r.gotExecReq, wantReq)
	}
	text := resultText(t, got)
	for _, want := range []string{"exit_code: 0", "hello"} {
		if !strings.Contains(text, want) {
			t.Errorf("result missing %q\nfull:\n%s", want, text)
		}
	}
}

func TestHandleSandboxExec_RunnerErrorSurfaced(t *testing.T) {
	r := &fakeRunner{execErr: errors.New("boom")}
	s := NewServer(r)
	got, err := s.handleSandboxExec(context.Background(), newRequest(map[string]any{
		"sandbox_id": "sbx-1",
		"command":    "true",
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

func TestHandleSandboxUpload_MissingParams(t *testing.T) {
	cases := []map[string]any{
		{},
		{"sandbox_id": "sbx-1"},
		{"sandbox_id": "sbx-1", "src": "/a"},
		{"src": "/a", "dst": "/b"},
		{"sandbox_id": "sbx-1", "src": "", "dst": "/b"},
	}
	for i, args := range cases {
		r := &fakeRunner{}
		s := NewServer(r)
		got, err := s.handleSandboxUpload(context.Background(), newRequest(args))
		if err != nil {
			t.Fatal(err)
		}
		if !got.IsError {
			t.Errorf("case %d: expected IsError=true for %v", i, args)
		}
		if r.uploadCalls != 0 {
			t.Errorf("case %d: runner should not be called", i)
		}
	}
}

func TestHandleSandboxUpload_HappyPath(t *testing.T) {
	r := &fakeRunner{}
	s := NewServer(r)
	got, err := s.handleSandboxUpload(context.Background(), newRequest(map[string]any{
		"sandbox_id": "sbx-1",
		"src":        "/host/data.txt",
		"dst":        "/tmp/data.txt",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got.IsError {
		t.Fatalf("unexpected error result: %s", resultText(t, got))
	}
	wantReq := sandbox.UploadRequest{SandboxID: "sbx-1", HostSrc: "/host/data.txt", SandboxDst: "/tmp/data.txt"}
	if !reflect.DeepEqual(r.gotUploadReq, wantReq) {
		t.Errorf("Upload request = %+v\nwant %+v", r.gotUploadReq, wantReq)
	}
	text := resultText(t, got)
	if !strings.Contains(text, "uploaded: data.txt -> /tmp/data.txt") {
		t.Errorf("result text = %q", text)
	}
}

func TestHandleSandboxDownload_MissingParams(t *testing.T) {
	cases := []map[string]any{
		{},
		{"sandbox_id": "sbx-1"},
		{"src": "/in/a"},
		{"sandbox_id": "", "src": "/in/a"},
	}
	for i, args := range cases {
		r := &fakeRunner{}
		s := NewServer(r)
		got, err := s.handleSandboxDownload(context.Background(), newRequest(args))
		if err != nil {
			t.Fatal(err)
		}
		if !got.IsError {
			t.Errorf("case %d: expected IsError=true for %v", i, args)
		}
		if r.downloadCalls != 0 {
			t.Errorf("case %d: runner should not be called", i)
		}
	}
}

func TestHandleSandboxDownload_HappyPath(t *testing.T) {
	r := &fakeRunner{
		downloadRes: sandbox.DownloadResult{HostPath: "/host/out/job-1/downloads/a.txt"},
	}
	s := NewServer(r)
	got, err := s.handleSandboxDownload(context.Background(), newRequest(map[string]any{
		"sandbox_id": "sbx-1",
		"src":        "/sandbox/a.txt",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got.IsError {
		t.Fatalf("unexpected error result: %s", resultText(t, got))
	}
	wantReq := sandbox.DownloadRequest{SandboxID: "sbx-1", SandboxSrc: "/sandbox/a.txt"}
	if !reflect.DeepEqual(r.gotDownloadReq, wantReq) {
		t.Errorf("Download request = %+v\nwant %+v", r.gotDownloadReq, wantReq)
	}
	text := resultText(t, got)
	if !strings.Contains(text, "downloaded: /sandbox/a.txt -> /host/out/job-1/downloads/a.txt") {
		t.Errorf("result text = %q", text)
	}
}

func TestHandleSandboxDestroy_MissingParam(t *testing.T) {
	r := &fakeRunner{}
	s := NewServer(r)
	got, err := s.handleSandboxDestroy(context.Background(), newRequest(map[string]any{}))
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsError {
		t.Fatal("expected IsError=true for missing sandbox_id")
	}
	if r.destroyCalls != 0 {
		t.Error("runner should not be called")
	}
}

func TestHandleSandboxDestroy_HappyPath(t *testing.T) {
	r := &fakeRunner{}
	s := NewServer(r)
	got, err := s.handleSandboxDestroy(context.Background(), newRequest(map[string]any{
		"sandbox_id": "sbx-1",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got.IsError {
		t.Fatalf("unexpected error result: %s", resultText(t, got))
	}
	if r.gotDestroyReq.SandboxID != "sbx-1" {
		t.Errorf("Destroy SandboxID = %q", r.gotDestroyReq.SandboxID)
	}
	if !strings.Contains(resultText(t, got), "destroyed: sbx-1") {
		t.Errorf("result text = %q", resultText(t, got))
	}
}
