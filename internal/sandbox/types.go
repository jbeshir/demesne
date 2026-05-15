package sandbox

// EgressMode controls outbound network access for a sandbox.
type EgressMode string

const (
	EgressNone            EgressMode = "none"
	EgressPackageManagers EgressMode = "package-managers"
)

// ScriptRequest captures the inputs to a single sandbox_script invocation.
type ScriptRequest struct {
	Command     string
	Image       string
	Egress      EgressMode
	Files       []string
	Directories []string
}

// ScriptResult captures the outputs of a single sandbox_script invocation.
type ScriptResult struct {
	JobID      string
	OutputPath string
	Stdout     string
	ExitCode   int
}

// CreateRequest captures the inputs to sandbox_create.
type CreateRequest struct {
	Image       string
	Egress      EgressMode
	Files       []string
	Directories []string
}

// CreateResult captures the outputs of sandbox_create.
type CreateResult struct {
	SandboxID  string
	OutputPath string
}

// ExecRequest captures the inputs to sandbox_exec.
type ExecRequest struct {
	SandboxID string
	Command   string
}

// ExecResult captures the outputs of sandbox_exec.
type ExecResult struct {
	Stdout   string
	ExitCode int
}

// UploadRequest captures the inputs to sandbox_upload.
type UploadRequest struct {
	SandboxID  string
	HostSrc    string
	SandboxDst string
}

// DownloadRequest captures the inputs to sandbox_download.
type DownloadRequest struct {
	SandboxID  string
	SandboxSrc string
}

// DownloadResult captures the outputs of sandbox_download.
type DownloadResult struct {
	HostPath string
}

// DestroyRequest captures the inputs to sandbox_destroy.
type DestroyRequest struct {
	SandboxID string
}
