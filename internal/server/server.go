package server

import (
	"context"

	"github.com/jbeshir/demesne/internal/sandbox"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
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
}

// Server is the MCP server for Demesne.
type Server struct {
	runner    Runner
	mcpServer *server.MCPServer
}

// NewServer creates a new MCP server backed by the given runner.
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

// Run starts the MCP server with stdio transport.
func (s *Server) Run() error {
	return server.ServeStdio(s.mcpServer)
}

func (s *Server) registerTools() {
	s.mcpServer.AddTool(mcp.NewTool("sandbox_script",
		mcp.WithDescription(scriptToolDescription),
		mcp.WithString("command",
			mcp.Required(),
			mcp.Description(
				"Shell command to run inside the sandbox. "+
					"Executed with /bin/sh -c. Working directory is /out.",
			),
		),
		mcp.WithString("image", mcp.Description(imageParamDescription)),
		mcp.WithString("egress", mcp.Description(egressParamDescription)),
		mcp.WithArray("files",
			mcp.Description(filesParamDescription),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithArray("directories",
			mcp.Description(directoriesParamDescription),
			mcp.Items(map[string]any{"type": "string"}),
		),
	), s.handleSandboxScript)

	s.mcpServer.AddTool(mcp.NewTool("sandbox_create",
		mcp.WithDescription(createToolDescription),
		mcp.WithString("image", mcp.Description(imageParamDescription)),
		mcp.WithString("egress", mcp.Description(egressParamDescription)),
		mcp.WithArray("files",
			mcp.Description(filesParamDescription),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithArray("directories",
			mcp.Description(directoriesParamDescription),
			mcp.Items(map[string]any{"type": "string"}),
		),
	), s.handleSandboxCreate)

	s.mcpServer.AddTool(mcp.NewTool("sandbox_exec",
		mcp.WithDescription(execToolDescription),
		mcp.WithString("sandbox_id",
			mcp.Required(),
			mcp.Description("Sandbox handle returned by sandbox_create."),
		),
		mcp.WithString("command",
			mcp.Required(),
			mcp.Description(
				"Shell command to run inside the sandbox. "+
					"Executed with /bin/sh -c. Working directory is /out.",
			),
		),
	), s.handleSandboxExec)

	s.mcpServer.AddTool(mcp.NewTool("sandbox_upload",
		mcp.WithDescription(uploadToolDescription),
		mcp.WithString("sandbox_id",
			mcp.Required(),
			mcp.Description("Sandbox handle returned by sandbox_create."),
		),
		mcp.WithString("src",
			mcp.Required(),
			mcp.Description(
				"Host file path to upload. Must be absolute and inside "+
					"DEMESNE_ALLOWED_PATHS. Symlinks are resolved before the check.",
			),
		),
		mcp.WithString("dst",
			mcp.Required(),
			mcp.Description(
				"Destination path inside the sandbox. Must be absolute. "+
					"Parent directory must already exist.",
			),
		),
	), s.handleSandboxUpload)

	s.mcpServer.AddTool(mcp.NewTool("sandbox_download",
		mcp.WithDescription(downloadToolDescription),
		mcp.WithString("sandbox_id",
			mcp.Required(),
			mcp.Description("Sandbox handle returned by sandbox_create."),
		),
		mcp.WithString("src",
			mcp.Required(),
			mcp.Description("Absolute path inside the sandbox to download."),
		),
	), s.handleSandboxDownload)

	s.mcpServer.AddTool(mcp.NewTool("sandbox_destroy",
		mcp.WithDescription(destroyToolDescription),
		mcp.WithString("sandbox_id",
			mcp.Required(),
			mcp.Description("Sandbox handle returned by sandbox_create."),
		),
	), s.handleSandboxDestroy)
}

const imageParamDescription = "Container image. One of: 'node' (node:22), " +
	"'python' (python:3.12), 'anaconda' (continuumio/anaconda3:latest, default)."

const egressParamDescription = "Outbound network policy. 'package-managers' (default) allows " +
	"npm, PyPI, and conda registries; 'none' denies all egress."

const filesParamDescription = "Host file paths to mount read-only into /in/<basename>. " +
	"Each path must be absolute and inside DEMESNE_ALLOWED_PATHS."

const directoriesParamDescription = "Host directory paths to mount read-only into /in/<basename>. " +
	"Each path must be absolute and inside DEMESNE_ALLOWED_PATHS."

const scriptToolDescription = `Run a single shell command in a fresh sandbox and return its stdout.

The sandbox is created from a whitelisted image (anaconda by default), the
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

The sandbox is built from a whitelisted image (anaconda by default) with any
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
