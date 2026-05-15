package server

import (
	"context"

	"github.com/jbeshir/demesne/internal/sandbox"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Runner is the dependency the server uses to execute scripts.
// Defined here as an interface so tests can inject a fake.
type Runner interface {
	RunScript(ctx context.Context, req sandbox.ScriptRequest) (sandbox.ScriptResult, error)
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
		mcp.WithString("image",
			mcp.Description(
				"Container image. One of: 'node' (node:22), "+
					"'python' (python:3.12), 'anaconda' (continuumio/anaconda3:latest, default).",
			),
		),
		mcp.WithString("egress",
			mcp.Description(
				"Outbound network policy. 'package-managers' (default) allows "+
					"npm, PyPI, and conda registries; 'none' denies all egress.",
			),
		),
		mcp.WithArray("files",
			mcp.Description(
				"Host file paths to mount read-only into /in/<basename>. "+
					"Each path must be absolute and inside DEMESNE_ALLOWED_PATHS.",
			),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithArray("directories",
			mcp.Description(
				"Host directory paths to mount read-only into /in/<basename>. "+
					"Each path must be absolute and inside DEMESNE_ALLOWED_PATHS.",
			),
			mcp.Items(map[string]any{"type": "string"}),
		),
	), s.handleSandboxScript)
}

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
