package sandbox

import "github.com/jbeshir/demesne/internal/egress"

// EgressMode is an alias for egress.Mode (kept so existing
// sandbox.EgressXxx references compile unchanged).
type EgressMode = egress.Mode

const (
	EgressNone            = egress.None
	EgressPackageManagers = egress.PackageManagers
	EgressOpen            = egress.Open
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

// AgentRequest captures the inputs to sandbox_agent.
type AgentRequest struct {
	Agent       string
	Model       string
	Prompt      string
	Preamble    string
	Files       []string
	Directories []string
	Egress      EgressMode
}

// AgentResult captures the outputs of sandbox_agent.
type AgentResult struct {
	JobID         string
	OutputPath    string // host path mounted at /out (output-only)
	WorkspacePath string // host path mounted at /workspace (the agent's scratch area)
	Stdout        string
	ExitCode      int
	// CostUSD is the indicative cumulative API spend (USD) the run
	// incurred through its vendor proxy.
	CostUSD float64
	// TotalUsageUSD adds CostUSD plus the spend of every descendant
	// sandbox this run spawned (see results.json).
	TotalUsageUSD float64
}

// ResearchRequest captures the inputs to sandbox_research. It mirrors
// AgentRequest minus the input-mount and egress knobs: research runs
// have no /in/<basename> mounts and unrestricted outbound internet.
type ResearchRequest struct {
	Agent    string
	Model    string
	Prompt   string
	Preamble string
}
