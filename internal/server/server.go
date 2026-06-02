package server

import (
	"context"
	"os"

	"github.com/jbeshir/demesne/internal/sandbox"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	paramSandboxID    = "sandbox_id"
	paramCommand      = "command"
	paramImage        = "image"
	paramEgress       = "egress"
	paramFiles        = "files"
	paramDirectories  = "directories"
	paramPrompt       = "prompt"
	paramAgent        = "agent"
	paramModel        = "model"
	paramPreamble     = "preamble"
	paramSrc          = "src"
	paramDst          = "dst"
	sandboxHandleDesc = "Sandbox handle returned by sandbox_create."
)

// Runner is the dependency the server uses to drive sandbox lifecycle
// operations. Defined here as an interface so tests can inject a fake.
type Runner interface {
	RunScript(ctx context.Context, req sandbox.ScriptRequest) (sandbox.ScriptResult, error)
	Create(ctx context.Context, req sandbox.CreateRequest) (sandbox.CreateResult, error)
	Exec(ctx context.Context, req sandbox.ExecRequest) (sandbox.ExecResult, error)
	Upload(ctx context.Context, req sandbox.UploadRequest) error
	Download(ctx context.Context, req sandbox.DownloadRequest) (sandbox.DownloadResult, error)
	Destroy(ctx context.Context, req sandbox.DestroyRequest) error
	Agent(ctx context.Context, req sandbox.AgentRequest) (sandbox.AgentResult, error)
	Research(ctx context.Context, req sandbox.ResearchRequest) (sandbox.AgentResult, error)
}

// Server is the MCP server for Demesne.
type Server struct {
	runner    Runner
	mcpServer *server.MCPServer
}

// NewServer constructs the demesne MCP server and registers every tool against the supplied
// Runner. Call RunContext to serve over stdio.
func NewServer(runner Runner) *Server {
	s := &Server{runner: runner}

	s.mcpServer = server.NewMCPServer(
		"demesne",
		"0.1.0",
		server.WithLogging(),
	)

	s.registerTools()
	return s
}

// RunContext starts the MCP server with stdio transport, stopping when ctx is cancelled.
// SIGHUP (and SIGINT/SIGTERM) cancellation flows through Listen, which drains
// in-flight tool-call workers via workerWg.Wait() before returning — so deferred
// killSandbox / sidecar.Remove calls in the runner complete cleanly.
func (s *Server) RunContext(ctx context.Context) error {
	stdioSrv := server.NewStdioServer(s.mcpServer)
	return stdioSrv.Listen(ctx, os.Stdin, os.Stdout)
}

// Run starts the MCP server with stdio transport.
func (s *Server) Run() error { return s.RunContext(context.Background()) }

func stringArrayItems() map[string]any { return map[string]any{"type": "string"} }

func (s *Server) registerTools() {
	s.mcpServer.AddTool(mcp.NewTool(sandbox.ToolSandboxScript,
		mcp.WithDescription(scriptToolDescription),
		mcp.WithOutputSchema[scriptOutput](),
		mcp.WithString(paramCommand,
			mcp.Required(),
			mcp.Description(
				"Shell command to run inside the sandbox. "+
					"Executed with /bin/sh -c. Working directory is /out.",
			),
		),
		mcp.WithString(paramImage, mcp.Description(imageParamDescription)),
		mcp.WithString(paramEgress, mcp.Description(egressParamDescription)),
		mcp.WithArray(paramFiles,
			mcp.Description(filesParamDescription),
			mcp.Items(stringArrayItems()),
		),
		mcp.WithArray(paramDirectories,
			mcp.Description(directoriesParamDescription),
			mcp.Items(stringArrayItems()),
		),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	), s.handleSandboxScript)

	s.mcpServer.AddTool(mcp.NewTool(sandbox.ToolSandboxCreate,
		mcp.WithDescription(createToolDescription),
		mcp.WithOutputSchema[createOutput](),
		mcp.WithString(paramImage, mcp.Description(imageParamDescription)),
		mcp.WithString(paramEgress, mcp.Description(egressParamDescription)),
		mcp.WithArray(paramFiles,
			mcp.Description(filesParamDescription),
			mcp.Items(stringArrayItems()),
		),
		mcp.WithArray(paramDirectories,
			mcp.Description(directoriesParamDescription),
			mcp.Items(stringArrayItems()),
		),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	), s.handleSandboxCreate)

	s.mcpServer.AddTool(mcp.NewTool(sandbox.ToolSandboxExec,
		mcp.WithDescription(execToolDescription),
		mcp.WithOutputSchema[execOutput](),
		mcp.WithString(paramSandboxID,
			mcp.Required(),
			mcp.Description(sandboxHandleDesc),
		),
		mcp.WithString(paramCommand,
			mcp.Required(),
			mcp.Description(
				"Shell command to run inside the sandbox. "+
					"Executed with /bin/sh -c. Working directory is /out.",
			),
		),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	), s.handleSandboxExec)

	s.mcpServer.AddTool(mcp.NewTool(sandbox.ToolSandboxUpload,
		mcp.WithDescription(uploadToolDescription),
		mcp.WithString(paramSandboxID,
			mcp.Required(),
			mcp.Description(sandboxHandleDesc),
		),
		mcp.WithString(paramSrc,
			mcp.Required(),
			mcp.Description(
				"Host file path to upload. Must be absolute and inside "+
					"DEMESNE_ALLOWED_PATHS. Symlinks are resolved before the check.",
			),
		),
		mcp.WithString(paramDst,
			mcp.Required(),
			mcp.Description(
				"Destination path inside the sandbox. Must be absolute. "+
					"Parent directory must already exist.",
			),
		),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), s.handleSandboxUpload)

	s.mcpServer.AddTool(mcp.NewTool(sandbox.ToolSandboxDownload,
		mcp.WithDescription(downloadToolDescription),
		mcp.WithString(paramSandboxID,
			mcp.Required(),
			mcp.Description(sandboxHandleDesc),
		),
		mcp.WithString(paramSrc,
			mcp.Required(),
			mcp.Description("Absolute path inside the sandbox to download."),
		),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), s.handleSandboxDownload)

	s.mcpServer.AddTool(mcp.NewTool(sandbox.ToolSandboxDestroy,
		mcp.WithDescription(destroyToolDescription),
		mcp.WithString(paramSandboxID,
			mcp.Required(),
			mcp.Description(sandboxHandleDesc),
		),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), s.handleSandboxDestroy)

	s.mcpServer.AddTool(mcp.NewTool(sandbox.ToolSandboxAgent,
		mcp.WithDescription(agentToolDescription),
		mcp.WithOutputSchema[agentRunOutput](),
		mcp.WithString(paramPrompt,
			mcp.Required(),
			mcp.Description("Task for the agent. Free-form text."),
		),
		mcp.WithString(paramAgent,
			mcp.Description(
				"Agent provider. 'claude-code' (default) or 'codex' "+
					"(OpenAI Codex CLI, experimental — see README). Defaults to 'claude-code'.",
			),
		),
		mcp.WithString(paramModel,
			mcp.Description(
				"Model for the agent. Provider-specific: claude-code uses "+
					"'opus', 'sonnet' (default), or 'haiku'; codex uses the gpt-5.x family.",
			),
		),
		mcp.WithString(paramPreamble,
			mcp.Description(
				"Optional prose prepended verbatim to the generated agent "+
					"context file (e.g. CLAUDE.md for claude-code) before the "+
					"auto-generated environment section.",
			),
		),
		mcp.WithString(paramEgress,
			mcp.Description(
				"Additional outbound network policy on top of the agent's "+
					"backend proxy (which is always reachable). 'none' (default) "+
					"means only the proxy; 'package-managers' also allows "+
					"npm/PyPI/conda registries. 'open' is rejected — use "+
					"sandbox_research for unrestricted egress (which has no "+
					"input mounts).",
			),
		),
		mcp.WithArray(paramFiles,
			mcp.Description(filesParamDescription),
			mcp.Items(stringArrayItems()),
		),
		mcp.WithArray(paramDirectories,
			mcp.Description(directoriesParamDescription),
			mcp.Items(stringArrayItems()),
		),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	), s.handleSandboxAgent)

	s.mcpServer.AddTool(mcp.NewTool(sandbox.ToolSandboxResearch,
		mcp.WithDescription(researchToolDescription),
		mcp.WithOutputSchema[agentRunOutput](),
		mcp.WithString(paramPrompt,
			mcp.Required(),
			mcp.Description("Research task for the agent. Free-form text."),
		),
		mcp.WithString(paramAgent,
			mcp.Description(
				"Agent provider. 'claude-code' (default) or 'codex' "+
					"(OpenAI Codex CLI, experimental — see README). Defaults to 'claude-code'.",
			),
		),
		mcp.WithString(paramModel,
			mcp.Description(
				"Model for the agent. Provider-specific: claude-code uses "+
					"'opus', 'sonnet' (default), or 'haiku'; codex uses the gpt-5.x family.",
			),
		),
		mcp.WithString(paramPreamble,
			mcp.Description(
				"Optional prose prepended verbatim to the generated agent "+
					"context file (e.g. CLAUDE.md for claude-code) before the "+
					"auto-generated environment section.",
			),
		),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	), s.handleSandboxResearch)
}

const imageParamDescription = "Container image. One of: 'node' (node:22), " +
	"'python' (python:3.12), 'go' (golang:1), 'anaconda' " +
	"(continuumio/anaconda3:latest, default)."

const egressParamDescription = "Outbound network policy. 'package-managers' (default) allows " +
	"npm, PyPI, and conda registries; 'none' denies all egress."

const filesParamDescription = "Host file paths to mount read-only into /in/<basename>. " +
	"Each path must be absolute and inside DEMESNE_ALLOWED_PATHS."

const directoriesParamDescription = "Host directory paths to mount read-only into /in/<basename>. " +
	"Each path must be absolute and inside DEMESNE_ALLOWED_PATHS."

const scriptToolDescription = `Run a single shell command in a fresh sandbox and return its stdout.

The sandbox is created from an allowlisted image (anaconda by default), the
command runs once, and the sandbox is destroyed when the command returns.

Mounts:
  /in/<basename>    read-only mounts of caller-supplied host inputs
  /out              writable mount; anything written here is preserved on the
                    host and the host path is returned in the result

Egress is restricted by default. 'package-managers' allows npm/PyPI/conda
registries; 'none' denies all outbound traffic.

The result text contains the exit code, the host path of /out, the job ID,
and the captured stdout.`

const createToolDescription = `Create a persistent sandbox and return its handle.

The sandbox is built from an allowlisted image (anaconda by default) with any
caller-supplied host paths mounted read-only at /in/<basename>. A writable
/out mount is provisioned automatically; its host path is returned so the
caller can read produced artifacts.

Egress is restricted by default. 'package-managers' allows npm/PyPI/conda
registries; 'none' denies all outbound traffic.

The returned sandbox_id is passed to sandbox_exec, sandbox_upload,
sandbox_download, and sandbox_destroy. TTL is 24h, refreshed on each
sandbox_exec call. Use sandbox_destroy to tear it down explicitly.`

const execToolDescription = `Run a shell command in an existing sandbox.

Executed with /bin/sh -c. Working directory is /out. The sandbox's TTL is
refreshed by 24h before the command runs.

The result text contains the exit code and the captured stdout.`

const uploadToolDescription = `Copy a host file into an existing sandbox.

The host source must be a regular file (not a directory), absolute, and
inside DEMESNE_ALLOWED_PATHS. The sandbox destination is an absolute path;
its parent directory must already exist.`

const downloadToolDescription = `Copy a file out of an existing sandbox to the host.

The file is written under <output_dir>/downloads/<basename>, where
output_dir is the host path returned by sandbox_create. The full host path
of the downloaded file is returned.`

const destroyToolDescription = `Destroy an existing sandbox.

The sandbox container is killed. The host output directory (containing /out
artefacts and any sandbox_download results) is preserved on the host for
later inspection — remove it separately if no longer needed.`

const agentToolDescription = `Run an AI agent inside a fresh sandbox against the caller's prompt.

The sandbox is built from the agent's own container image (built lazily on
first use) and torn down when the agent exits. Working directory is /workspace.

The agent reads its provider-specific context file (e.g.
/in/CLAUDE.md for claude-code) before processing the task. The file
is generated from the optional 'preamble' parameter plus an
auto-generated 'Environment' section listing any /in/<basename>
inputs and the /out writable mount.

Outbound network access is restricted: the vendor proxy is always reachable (the agent's CLI uses it), and the 'egress'
parameter controls whatever else the sandbox may reach. 'open' egress
is not permitted here — use sandbox_research for unrestricted egress
(which also forbids /in mounts).

The result text contains the exit code, the host path of /out
(containing the generated agent context file and any agent-written
artefacts), the job ID, the cost summary, and the agent's stdout. The
cost_usd field is the *indicative* dollar spend the run incurred
through its vendor proxy, computed from the vendor's published API
pricing; it is reported regardless of how the underlying OAuth token
is billed (for example, Claude Code OAuth tokens typically authorise
against a Claude Console subscription rather than per-request API
billing, so the user is not charged per request).`

const researchToolDescription = `Run a long-running research agent in a fresh sandbox with unrestricted
outbound internet access.

Like sandbox_agent but with two deliberate differences:
  - No /in mounts. The agent only sees its prompt and whatever it
    fetches from the open web.
  - Egress is fully open. The vendor proxy still gates calls to
    the model API; any other public HTTPS endpoint is reachable
    directly.

These choices are tied together: open egress without input mounts is
research; input mounts without open egress is the agent path. The
combination of inputs + open egress is not exposed.

The result text contains the exit code, the host path of /out
(containing the generated agent context file, usage.json, and any
artefacts the agent wrote), the job ID, the cost summary, and the
agent's stdout. The cost_usd field is the *indicative* dollar spend
the run incurred through its vendor proxy, computed from the vendor's
published API pricing; it is reported regardless of how the underlying
OAuth token is billed (for example, Claude Code OAuth tokens typically
authorise against a Claude Console subscription rather than
per-request API billing, so the user is not charged per request).`
