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

// JobID is the demesne-internal UUID minted per run, stored in sandbox
// metadata and used to correlate /out dirs with their creating sandbox.
type JobID string

// SandboxID is the OpenSandbox UUID returned by sandbox_create, used to
// re-attach to a persistent sandbox via exec/upload/download/destroy.
type SandboxID string

// ScriptRequest captures the inputs to a single sandbox_script invocation.
type ScriptRequest struct {
	Command     string
	Image       string
	Egress      EgressMode
	Files       []string
	Directories []string
	// Background, when true, starts the job asynchronously and returns
	// immediately. Poll with Status/Wait; cancel with Cancel.
	Background bool
}

// ScriptResult captures the outputs of a single sandbox_script invocation.
type ScriptResult struct {
	JobID      JobID
	OutputPath string
	Stdout     string
	ExitCode   int
	Stderr     string
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
	SandboxID  SandboxID
	OutputPath string
}

// ExecRequest captures the inputs to sandbox_exec.
type ExecRequest struct {
	SandboxID SandboxID
	Command   string
}

// ExecResult captures the outputs of sandbox_exec.
type ExecResult struct {
	Stdout   string
	ExitCode int
	Stderr   string
}

// UploadRequest captures the inputs to sandbox_upload.
type UploadRequest struct {
	SandboxID  SandboxID
	HostSrc    string
	SandboxDst string
}

// DownloadRequest captures the inputs to sandbox_download.
type DownloadRequest struct {
	SandboxID  SandboxID
	SandboxSrc string
}

// DownloadResult captures the outputs of sandbox_download.
type DownloadResult struct {
	HostPath string
}

// DestroyRequest captures the inputs to sandbox_destroy.
type DestroyRequest struct {
	SandboxID SandboxID
}

// AgentRequest captures the inputs to sandbox_agent.
type AgentRequest struct {
	Agent           string
	Model           string
	Prompt          string
	Preamble        string
	Files           []string
	Directories     []string
	Egress          EgressMode
	OutputPath      string
	OutputFormat    string
	SuccessCriteria []string
	// Background, when true, starts the job asynchronously and returns
	// immediately. Poll with Status/Wait; cancel with Cancel.
	Background bool
}

// AgentResult captures the outputs of sandbox_agent.
type AgentResult struct {
	JobID         JobID
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
	Stderr        string
	// PerModelTokens is the four-token-type breakdown (input, output,
	// cache_creation, cache_read) per model ID, aggregated across this
	// run and all its descendant sandboxes. Nil when no requests were
	// recorded.
	PerModelTokens map[string]TokenTotals
	// UsageSummary is a one-line human-readable summary of token usage:
	// cache-read percentage of input-side tokens, plus the top subagents
	// by token consumption from attribution.jsonl. Empty when no usage
	// was recorded.
	UsageSummary string `json:"usage_summary,omitempty"`
}

// ResearchRequest captures the inputs to sandbox_research. It mirrors
// AgentRequest minus the input-mount and egress knobs: research runs
// have no /in/<basename> mounts and unrestricted outbound internet.
type ResearchRequest struct {
	Agent           string
	Model           string
	Prompt          string
	Preamble        string
	OutputPath      string
	OutputFormat    string
	SuccessCriteria []string
	// Background, when true, starts the job asynchronously and returns
	// immediately. Poll with Status/Wait; cancel with Cancel.
	Background bool
}

// StatusRequest captures the inputs to sandbox_status.
type StatusRequest struct {
	// JobID is the public handle returned by a background spawn.
	JobID JobID
}

// WaitRequest captures the inputs to sandbox_wait.
type WaitRequest struct {
	// JobID is the public handle returned by a background spawn.
	JobID JobID
	// TimeoutSeconds is the maximum time to wait; clamped to [1ms, 120s].
	// Zero or negative uses the default of 30s.
	TimeoutSeconds int
}

// CancelRequest captures the inputs to sandbox_cancel.
type CancelRequest struct {
	// JobID is the public handle returned by a background spawn.
	JobID JobID
}
