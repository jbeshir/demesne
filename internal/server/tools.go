package server

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/jbeshir/demesne/internal/sandbox"
	"github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleSandboxScript(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	command, err := request.RequireString("command")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if command == "" {
		return mcp.NewToolResultError("command is required"), nil
	}

	image := request.GetString("image", "")
	egress := request.GetString("egress", string(sandbox.EgressPackageManagers))

	files, err := optionalStringSlice(request, "files")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	directories, err := optionalStringSlice(request, "directories")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	res, err := s.runner.RunScript(ctx, sandbox.ScriptRequest{
		Command:     command,
		Image:       image,
		Egress:      sandbox.EgressMode(egress),
		Files:       files,
		Directories: directories,
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return formatScriptResult(res), nil
}

func (s *Server) handleSandboxCreate(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	image := request.GetString("image", "")
	egress := request.GetString("egress", string(sandbox.EgressPackageManagers))

	files, err := optionalStringSlice(request, "files")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	directories, err := optionalStringSlice(request, "directories")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	res, err := s.runner.Create(ctx, sandbox.CreateRequest{
		Image:       image,
		Egress:      sandbox.EgressMode(egress),
		Files:       files,
		Directories: directories,
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(
		fmt.Sprintf("sandbox_id: %s\noutput_dir: %s", res.SandboxID, res.OutputPath),
	), nil
}

func (s *Server) handleSandboxExec(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	sandboxID, err := request.RequireString("sandbox_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if sandboxID == "" {
		return mcp.NewToolResultError("sandbox_id is required"), nil
	}
	command, err := request.RequireString("command")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if command == "" {
		return mcp.NewToolResultError("command is required"), nil
	}

	res, err := s.runner.Exec(ctx, sandbox.ExecRequest{
		SandboxID: sandboxID,
		Command:   command,
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(
		fmt.Sprintf("exit_code: %d\n---\n%s", res.ExitCode, res.Stdout),
	), nil
}

func (s *Server) handleSandboxUpload(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	sandboxID, err := request.RequireString("sandbox_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	src, err := request.RequireString("src")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	dst, err := request.RequireString("dst")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if sandboxID == "" || src == "" || dst == "" {
		return mcp.NewToolResultError("sandbox_id, src, and dst are required"), nil
	}

	if err := s.runner.Upload(ctx, sandbox.UploadRequest{
		SandboxID:  sandboxID,
		HostSrc:    src,
		SandboxDst: dst,
	}); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(
		fmt.Sprintf("uploaded: %s -> %s", filepath.Base(src), dst),
	), nil
}

func (s *Server) handleSandboxDownload(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	sandboxID, err := request.RequireString("sandbox_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	src, err := request.RequireString("src")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if sandboxID == "" || src == "" {
		return mcp.NewToolResultError("sandbox_id and src are required"), nil
	}

	res, err := s.runner.Download(ctx, sandbox.DownloadRequest{
		SandboxID:  sandboxID,
		SandboxSrc: src,
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(
		fmt.Sprintf("downloaded: %s -> %s", src, res.HostPath),
	), nil
}

func (s *Server) handleSandboxDestroy(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	sandboxID, err := request.RequireString("sandbox_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if sandboxID == "" {
		return mcp.NewToolResultError("sandbox_id is required"), nil
	}

	if err := s.runner.Destroy(ctx, sandbox.DestroyRequest{SandboxID: sandboxID}); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText("destroyed: " + sandboxID), nil
}

func (s *Server) handleSandboxAgent(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	prompt, err := request.RequireString("prompt")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if prompt == "" {
		return mcp.NewToolResultError("prompt is required"), nil
	}

	agentName := request.GetString("agent", "")
	model := request.GetString("model", "")
	preamble := request.GetString("preamble", "")
	egress := request.GetString("egress", string(sandbox.EgressNone))

	files, err := optionalStringSlice(request, "files")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	directories, err := optionalStringSlice(request, "directories")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	res, err := s.runner.Agent(ctx, sandbox.AgentRequest{
		Agent:       agentName,
		Model:       model,
		Prompt:      prompt,
		Preamble:    preamble,
		Files:       files,
		Directories: directories,
		Egress:      sandbox.EgressMode(egress),
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(
		fmt.Sprintf("exit_code: %d\noutput_dir: %s\njob_id: %s\n---\n%s",
			res.ExitCode, res.OutputPath, res.JobID, res.Stdout),
	), nil
}

// optionalStringSlice returns the named argument as []string. It treats a
// missing argument as an empty slice but rejects a present-but-wrong-typed one.
func optionalStringSlice(request mcp.CallToolRequest, key string) ([]string, error) {
	args := request.GetArguments()
	raw, present := args[key]
	if !present || raw == nil {
		return nil, nil
	}
	switch v := raw.(type) {
	case []string:
		return v, nil
	case []any:
		out := make([]string, 0, len(v))
		for i, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("%s[%d] is not a string", key, i)
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%s must be an array of strings", key)
	}
}
