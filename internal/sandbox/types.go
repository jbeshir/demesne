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
