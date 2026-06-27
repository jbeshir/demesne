package server

import (
	"context"
	"os"
	"strings"

	"github.com/jbeshir/demesne/internal/sandbox"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	paramSandboxID       = "sandbox_id"
	paramOutputDir       = "output_dir"
	paramCommand         = "command"
	paramImage           = "image"
	paramEgress          = "egress"
	paramFiles           = "files"
	paramDirectories     = "directories"
	paramPrompt          = "prompt"
	paramModel           = "model"
	paramPreamble        = "preamble"
	paramSrc             = "src"
	paramDst             = "dst"
	paramOutputPath      = "output_path"
	paramOutputFormat    = "output_format"
	paramSuccessCriteria = "success_criteria"
	paramBackground      = "background"
	paramJobID           = "job_id"
	paramTimeoutSeconds  = "timeout_seconds"
	sandboxHandleDesc    = "Sandbox handle returned by sandbox_create."

	// agentNameCodex / agentNameClaudeCode are the caller-facing
	// provider identifiers, mirrored from internal/sandbox so the
	// server doesn't import the sandbox package's unexported names. The
	// switch in modelParamOptions matches against these.
	agentNameCodex      = "codex"
	agentNameClaudeCode = "claude-code"
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
	StartScript(req sandbox.ScriptRequest) sandbox.JobID
	StartAgent(req sandbox.AgentRequest) sandbox.JobID
	StartResearch(req sandbox.ResearchRequest) sandbox.JobID
	Status(req sandbox.StatusRequest) (sandbox.StatusResult, error)
	Wait(ctx context.Context, req sandbox.WaitRequest) (sandbox.WaitResult, error)
	Cancel(ctx context.Context, req sandbox.CancelRequest) (sandbox.CancelResult, error)
	// UsageReport reads the output tree for a job or explicit directory and
	// returns an aggregated cost and token breakdown.
	UsageReport(req sandbox.UsageReportRequest) (sandbox.UsageReport, error)
	// AvailableAgents reports which agent providers have host credentials
	// configured (codex-first), with each provider's model allowlist. The
	// server uses it to filter the `model` enum advertised on
	// sandbox_agent / sandbox_research at registration time.
	AvailableAgents() []sandbox.AgentOption
	// AllowedMountPaths reports the host paths under which callers may
	// mount inputs (`files`/`directories`) or upload (`src`). The server
	// uses it to populate the matching MCP param descriptions at
	// registration time, so the agent calling the tool knows which roots
	// are reachable.
	AllowedMountPaths() []string
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

// modelParamOptions returns the mcp tool options for the `model`
// parameter, filtered to the configured credentials. The enum is the
// de-duplicated union of available providers' model allowlists in
// codex-first order; the description names the default-when-omitted
// behavior and lists which models pair with which available provider
// (filtered, so when only codex is configured the claude-code clause
// is dropped). Zero available → no enum + a brief note.
func modelParamOptions(available []sandbox.AgentOption) mcp.ToolOption {
	if len(available) == 0 {
		return mcp.WithString(paramModel, mcp.Description(
			"Model for the agent. No agent credentials are configured, so "+
				"no models are available.",
		))
	}
	var models []string
	seen := make(map[string]bool)
	for _, a := range available {
		for _, m := range a.Models {
			if seen[m] {
				continue
			}
			seen[m] = true
			models = append(models, m)
		}
	}
	clauses := make([]string, 0, len(available))
	for _, a := range available {
		switch a.Name {
		case agentNameClaudeCode:
			clauses = append(clauses, "claude-code uses 'fable' (most capable; hardest synthesis), "+
				"'opus' (complex synthesis), 'sonnet' (default; general agentic work), "+
				"or 'haiku' (lookup / cheap)")
		case agentNameCodex:
			clauses = append(clauses, "codex uses the gpt-5.x family")
		}
	}
	desc := "Model for the agent. The provider is inferred automatically from the chosen model. " +
		"When omitted, defaults to the default model of the credential-aware default provider " +
		"(codex's gpt-5.5 when Codex credentials are configured, otherwise claude-code's sonnet). " +
		"Provider-specific: " + joinSemi(clauses) + "."
	return mcp.WithString(paramModel, mcp.Description(desc), mcp.Enum(models...))
}

// allowedPathsClause renders the configured host mount-path allowlist
// as a trailing clause for the `files` / `directories` / `src`
// param descriptions. When at least one path is configured it returns
// a single sentence listing the roots; when empty it returns the
// no-host-inputs warning that names the env var to set. Shared by the
// files/directories builders and the upload-src builder so the wording
// stays consistent.
func allowedPathsClause(allowed []string) string {
	if len(allowed) == 0 {
		return "No host inputs can be mounted: DEMESNE_ALLOWED_PATHS is not configured."
	}
	quoted := make([]string, len(allowed))
	for i, p := range allowed {
		quoted[i] = "`" + p + "`"
	}
	return "Must be absolute and inside one of the configured mount roots: " + strings.Join(quoted, ", ") + "."
}

// filesParamDescriptionFor builds the description of the `files`
// array param, listing the configured DEMESNE_ALLOWED_PATHS roots.
func filesParamDescriptionFor(allowed []string) string {
	return "Host file paths to mount read-only into /in/<basename>. " + allowedPathsClause(allowed)
}

// directoriesParamDescriptionFor builds the description of the
// `directories` array param, listing the configured
// DEMESNE_ALLOWED_PATHS roots.
func directoriesParamDescriptionFor(allowed []string) string {
	return "Host directory paths to mount read-only into /in/<basename>. " + allowedPathsClause(allowed)
}

// uploadSrcDescriptionFor builds the description of sandbox_upload's
// `src` param, listing the configured DEMESNE_ALLOWED_PATHS roots.
func uploadSrcDescriptionFor(allowed []string) string {
	return "Host file path to upload. " + allowedPathsClause(allowed) +
		" Symlinks are resolved before the check."
}

// uploadToolDescriptionFor builds sandbox_upload's tool-level
// description, listing the configured DEMESNE_ALLOWED_PATHS roots in
// the same clause used by the param descriptions.
func uploadToolDescriptionFor(allowed []string) string {
	return "Copy a host file into an existing sandbox.\n\n" +
		"The host source must be a regular file (not a directory). " +
		allowedPathsClause(allowed) +
		" The sandbox destination is an absolute path; its parent directory must already exist."
}

// joinSemi joins clauses with "; " — used by the model-param
// description so each provider's pairing reads as its own clause.
func joinSemi(parts []string) string { return strings.Join(parts, "; ") }

func (s *Server) registerTools() {
	available := s.runner.AvailableAgents()
	modelOpt := modelParamOptions(available)
	allowed := s.runner.AllowedMountPaths()
	filesDesc := filesParamDescriptionFor(allowed)
	directoriesDesc := directoriesParamDescriptionFor(allowed)
	uploadSrcDesc := uploadSrcDescriptionFor(allowed)
	uploadDesc := uploadToolDescriptionFor(allowed)
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
			mcp.Description(filesDesc),
			mcp.Items(stringArrayItems()),
		),
		mcp.WithArray(paramDirectories,
			mcp.Description(directoriesDesc),
			mcp.Items(stringArrayItems()),
		),
		mcp.WithBoolean(paramBackground,
			mcp.Description("When true, returns immediately with {job_id, status:\"running\"} "+
				"instead of blocking; poll with sandbox_status / sandbox_wait, "+
				"cancel with sandbox_cancel."),
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
			mcp.Description(filesDesc),
			mcp.Items(stringArrayItems()),
		),
		mcp.WithArray(paramDirectories,
			mcp.Description(directoriesDesc),
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
		mcp.WithDescription(uploadDesc),
		mcp.WithString(paramSandboxID,
			mcp.Required(),
			mcp.Description(sandboxHandleDesc),
		),
		mcp.WithString(paramSrc,
			mcp.Required(),
			mcp.Description(uploadSrcDesc),
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
			mcp.Description(
				"Task for the agent. Name the expected output path "+
					"(e.g. /workspace/findings.md or /out/<name>.json) and a "+
					"short 'definition of done' checklist.",
			),
		),
		modelOpt,
		mcp.WithString(paramPreamble,
			mcp.Description(
				"Optional prose prepended verbatim to the generated agent "+
					"context file (e.g. CLAUDE.md for claude-code) before the "+
					"auto-generated environment section. The right place for "+
					"role framing and 'must not' constraints.",
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
			mcp.Description(filesDesc),
			mcp.Items(stringArrayItems()),
		),
		mcp.WithArray(paramDirectories,
			mcp.Description(directoriesDesc),
			mcp.Items(stringArrayItems()),
		),
		mcp.WithString(paramOutputPath,
			mcp.Description("Optional. Where the agent should write its final artefact (e.g. /out/summary.md). "+
				"Rendered as a Definition of done block in the agent's context."),
		),
		mcp.WithString(paramOutputFormat,
			mcp.Description("Optional. Expected shape/format of the output (e.g. 'Markdown report', "+
				"'JSON: {result: string}'). Rendered as a Definition of done block in the agent's context."),
		),
		mcp.WithArray(paramSuccessCriteria,
			mcp.Description("Optional. Checklist of conditions the output must satisfy. "+
				"Rendered as a bulleted Definition of done block."),
			mcp.Items(stringArrayItems()),
		),
		mcp.WithBoolean(paramBackground,
			mcp.Description("When true, returns immediately with {job_id, status:\"running\"} "+
				"instead of blocking; poll with sandbox_status / sandbox_wait, "+
				"cancel with sandbox_cancel."),
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
			mcp.Description(
				"Research task for the agent. Name the expected output path "+
					"and a short 'definition of done' checklist.",
			),
		),
		modelOpt,
		mcp.WithString(paramPreamble,
			mcp.Description(
				"Optional prose prepended verbatim to the generated agent "+
					"context file (e.g. CLAUDE.md for claude-code) before the "+
					"auto-generated environment section. The right place for "+
					"role framing and 'must not' constraints.",
			),
		),
		mcp.WithString(paramOutputPath,
			mcp.Description("Optional. Where the agent should write its final artefact (e.g. /out/summary.md). "+
				"Rendered as a Definition of done block in the agent's context."),
		),
		mcp.WithString(paramOutputFormat,
			mcp.Description("Optional. Expected shape/format of the output (e.g. 'Markdown report', "+
				"'JSON: {result: string}'). Rendered as a Definition of done block in the agent's context."),
		),
		mcp.WithArray(paramSuccessCriteria,
			mcp.Description("Optional. Checklist of conditions the output must satisfy. "+
				"Rendered as a bulleted Definition of done block."),
			mcp.Items(stringArrayItems()),
		),
		mcp.WithBoolean(paramBackground,
			mcp.Description("When true, returns immediately with {job_id, status:\"running\"} "+
				"instead of blocking; poll with sandbox_status / sandbox_wait, "+
				"cancel with sandbox_cancel."),
		),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	), s.handleSandboxResearch)

	s.mcpServer.AddTool(mcp.NewTool(sandbox.ToolSandboxStatus,
		mcp.WithDescription(statusToolDescription),
		mcp.WithOutputSchema[statusOutput](),
		mcp.WithString(paramJobID,
			mcp.Required(),
			mcp.Description("Job ID returned by a background sandbox_script/agent/research call."),
		),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), s.handleSandboxStatus)

	s.mcpServer.AddTool(mcp.NewTool(sandbox.ToolSandboxWait,
		mcp.WithDescription(waitToolDescription),
		mcp.WithOutputSchema[waitOutput](),
		mcp.WithString(paramJobID,
			mcp.Required(),
			mcp.Description("Job ID returned by a background sandbox_script/agent/research call."),
		),
		mcp.WithNumber(paramTimeoutSeconds,
			mcp.Description("Maximum seconds to wait for the job to reach a terminal state. "+
				"0 or omitted → 30 s default; hard-capped at 120 s."),
		),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), s.handleSandboxWait)

	s.mcpServer.AddTool(mcp.NewTool(sandbox.ToolSandboxCancel,
		mcp.WithDescription(cancelToolDescription),
		mcp.WithOutputSchema[cancelOutput](),
		mcp.WithString(paramJobID,
			mcp.Required(),
			mcp.Description("Job ID returned by a background sandbox_script/agent/research call."),
		),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), s.handleSandboxCancel)

	s.mcpServer.AddTool(mcp.NewTool(sandbox.ToolSandboxUsageReport,
		mcp.WithDescription(usageReportToolDescription),
		mcp.WithOutputSchema[usageReportOutput](),
		mcp.WithString(paramJobID,
			mcp.Description("Job ID from a sandbox_agent or sandbox_research run. "+
				"Used to locate the output directory at OutputRoot/<job_id>/out. "+
				"At least one of job_id or output_dir is required."),
		),
		mcp.WithString(paramOutputDir,
			mcp.Description("Explicit host path of the /out root to report on. "+
				"Takes precedence over job_id when both are supplied. "+
				"Must be inside OutputRoot. "+
				"At least one of job_id or output_dir is required."),
		),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), s.handleSandboxUsageReport)
}

const imageParamDescription = "Container image. One of: 'node' (node:22), " +
	"'python' (python:3.12), 'go' (golang:1), 'anaconda' " +
	"(continuumio/anaconda3:latest, default), 'browser' " +
	"(demesne-built; Playwright JS + Chromium/Firefox/WebKit + Node, " +
	"headless rendering at egress=none, built lazily on first use), " +
	"'media' (demesne-built; ffmpeg + ImageMagick + libvips + audio tooling " +
	"for video/audio/image conversion, built lazily on first use), " +
	"'twine' (demesne-built; Tweego + Twine story formats + Chromium for " +
	"offline interactive-fiction build/playtest, built lazily on first use), " +
	"'webgamedev' (demesne-built; warm Phaser + Vite + TypeScript template + " +
	"Chromium for offline HTML5-game build/playtest, built lazily on first use)."

const egressParamDescription = "Outbound network policy. 'package-managers' (default) allows " +
	"npm, PyPI, and conda registries; 'none' denies all egress."

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
/in/.agent/CLAUDE.md for claude-code) before processing the task. The file
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

const statusToolDescription = `Get the current status of a background sandbox job.

Returns the job ID, status (running/succeeded/failed/cancelled), elapsed
time, a tail of any captured stdout so far, and cost/exit-code when the
job has completed. Use sandbox_wait to block until completion.`

const waitToolDescription = `Block until a background sandbox job reaches a terminal state.

Returns when the job succeeds, fails, or is cancelled — or when the
optional timeout_seconds elapses (status is still "running" in that case).
ctx cancellation abandons the wait without affecting the job.`

const cancelToolDescription = `Request cancellation of a background sandbox job and its descendants.

Idempotent: calling cancel on an already-terminal job returns its final
status without error. Returns the job ID and its resulting status.`

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

const usageReportToolDescription = `Return an aggregated cost and token breakdown for a completed job.

Reads the output tree rooted at the job's /out directory (located via
job_id or an explicit output_dir) and returns:
  - total_cost_usd: vendor-reported cost across the whole tree
  - token_type_totals: tree-wide input/output/cache_creation/cache_read counts
  - cache_read_pct: cache_read / (input + cache_creation + cache_read)
  - by_model: per-model token and cost breakdown
  - by_subagent: per-subagent attribution breakdown
  - by_child: per-node own cost and token breakdown (one entry per node)
  - dropped: aggregate dropped-usage counters

Cost figures are taken from each node's usage.json (authoritative vendor
totals). Per-subagent costs are INDICATIVE list-price, computed from
usage.jsonl token counts via the Anthropic pricing table; they are NOT
the authoritative vendor total and are provided for attribution purposes
only. Subagent attribution is available ONLY for claude-code runs
(requires attribution.jsonl written by the proxy distillation step);
codex and other runs will show all spend under "(main)". Any spend that
cannot be attributed to a named subagent — including all non-Anthropic
requests and any rounding gap — is reconciled into "(main)".

Accepts job_id OR output_dir (at least one required). output_dir takes
precedence and must be inside OutputRoot.`
