package server

import (
	"context"
	"fmt"

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
