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
	command, errRes := requireNonEmpty(request, paramCommand)
	if errRes != nil {
		return errRes, nil
	}

	image := request.GetString(paramImage, "")
	egress := request.GetString(paramEgress, string(sandbox.EgressPackageManagers))
	if res, rejected := rejectOpenEgress(egress); rejected {
		return res, nil
	}

	files, err := optionalStringSlice(request, paramFiles)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	directories, err := optionalStringSlice(request, paramDirectories)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := sandbox.ScriptRequest{
		Command:     command,
		Image:       image,
		Egress:      sandbox.EgressMode(egress),
		Files:       files,
		Directories: directories,
	}
	if request.GetBool(paramBackground, false) {
		jobID := s.runner.StartScript(req, s.terminalNotifier(ctx))
		return formatJobStarted(jobID), nil
	}
	res, err := s.runner.RunScript(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return formatScriptResult(res), nil
}

func (s *Server) handleSandboxCreate(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	image := request.GetString(paramImage, "")
	egress := request.GetString(paramEgress, string(sandbox.EgressPackageManagers))
	if res, rejected := rejectOpenEgress(egress); rejected {
		return res, nil
	}

	files, err := optionalStringSlice(request, paramFiles)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	directories, err := optionalStringSlice(request, paramDirectories)
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
	return formatCreateResult(res), nil
}

func (s *Server) handleSandboxExec(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	sandboxID, errRes := requireNonEmpty(request, paramSandboxID)
	if errRes != nil {
		return errRes, nil
	}
	command, errRes := requireNonEmpty(request, paramCommand)
	if errRes != nil {
		return errRes, nil
	}

	res, err := s.runner.Exec(ctx, sandbox.ExecRequest{
		SandboxID: sandbox.SandboxID(sandboxID),
		Command:   command,
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return formatExecResult(res), nil
}

func (s *Server) handleSandboxUpload(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	sandboxID, err := request.RequireString(paramSandboxID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	src, err := request.RequireString(paramSrc)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	dst, err := request.RequireString(paramDst)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if sandboxID == "" || src == "" || dst == "" {
		return mcp.NewToolResultError("sandbox_id, src, and dst are required"), nil
	}

	if err := s.runner.Upload(ctx, sandbox.UploadRequest{
		SandboxID:  sandbox.SandboxID(sandboxID),
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
	sandboxID, err := request.RequireString(paramSandboxID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	src, err := request.RequireString(paramSrc)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if sandboxID == "" || src == "" {
		return mcp.NewToolResultError("sandbox_id and src are required"), nil
	}

	res, err := s.runner.Download(ctx, sandbox.DownloadRequest{
		SandboxID:  sandbox.SandboxID(sandboxID),
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
	sandboxID, errRes := requireNonEmpty(request, paramSandboxID)
	if errRes != nil {
		return errRes, nil
	}

	if err := s.runner.Destroy(ctx, sandbox.DestroyRequest{SandboxID: sandbox.SandboxID(sandboxID)}); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText("destroyed: " + sandboxID), nil
}

func (s *Server) handleSandboxAgent(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	prompt, errRes := requireNonEmpty(request, paramPrompt)
	if errRes != nil {
		return errRes, nil
	}

	model := request.GetString(paramModel, "")
	preamble := request.GetString(paramPreamble, "")
	egress := request.GetString(paramEgress, string(sandbox.EgressNone))
	if res, rejected := rejectOpenEgress(egress); rejected {
		return res, nil
	}

	files, err := optionalStringSlice(request, paramFiles)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	directories, err := optionalStringSlice(request, paramDirectories)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	outputPath := request.GetString(paramOutputPath, "")
	outputFormat := request.GetString(paramOutputFormat, "")
	successCriteria, err := optionalStringSlice(request, paramSuccessCriteria)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	agentReq := sandbox.AgentRequest{
		Model:           model,
		Prompt:          prompt,
		Preamble:        preamble,
		Files:           files,
		Directories:     directories,
		Egress:          sandbox.EgressMode(egress),
		OutputPath:      outputPath,
		OutputFormat:    outputFormat,
		SuccessCriteria: successCriteria,
	}
	if request.GetBool(paramBackground, false) {
		jobID := s.runner.StartAgent(agentReq, s.terminalNotifier(ctx))
		return formatJobStarted(jobID), nil
	}
	res, err := s.runner.Agent(ctx, agentReq)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return formatAgentRunResult(res), nil
}

func (s *Server) handleSandboxResearch(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	prompt, errRes := requireNonEmpty(request, paramPrompt)
	if errRes != nil {
		return errRes, nil
	}

	model := request.GetString(paramModel, "")
	preamble := request.GetString(paramPreamble, "")
	outputPath := request.GetString(paramOutputPath, "")
	outputFormat := request.GetString(paramOutputFormat, "")
	successCriteria, err := optionalStringSlice(request, paramSuccessCriteria)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	researchReq := sandbox.ResearchRequest{
		Model:           model,
		Prompt:          prompt,
		Preamble:        preamble,
		OutputPath:      outputPath,
		OutputFormat:    outputFormat,
		SuccessCriteria: successCriteria,
	}
	if request.GetBool(paramBackground, false) {
		jobID := s.runner.StartResearch(researchReq, s.terminalNotifier(ctx))
		return formatJobStarted(jobID), nil
	}
	res, err := s.runner.Research(ctx, researchReq)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return formatAgentRunResult(res), nil
}

func (s *Server) handleSandboxStatus(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	jobID, errRes := requireNonEmpty(request, paramJobID)
	if errRes != nil {
		return errRes, nil
	}
	res, err := s.runner.Status(sandbox.StatusRequest{
		JobID:             sandbox.JobID(jobID),
		IncludeStdoutTail: request.GetBool(paramIncludeStdoutTail, false),
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return formatStatusResult(res), nil
}

func (s *Server) handleSandboxWait(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	jobID, errRes := requireNonEmpty(request, paramJobID)
	if errRes != nil {
		return errRes, nil
	}
	timeout := request.GetInt(paramTimeoutSeconds, 0)
	res, err := s.runner.Wait(ctx, sandbox.WaitRequest{
		JobID:          sandbox.JobID(jobID),
		TimeoutSeconds: timeout,
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return formatWaitResult(res), nil
}

func (s *Server) handleSandboxCancel(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	jobID, errRes := requireNonEmpty(request, paramJobID)
	if errRes != nil {
		return errRes, nil
	}
	res, err := s.runner.Cancel(ctx, sandbox.CancelRequest{JobID: sandbox.JobID(jobID)})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return formatCancelResult(res), nil
}

func (s *Server) handleSandboxUsageReport(
	_ context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	jobID := request.GetString(paramJobID, "")
	outputDir := request.GetString(paramOutputDir, "")
	if jobID == "" && outputDir == "" {
		return mcp.NewToolResultError("job_id or output_dir is required"), nil
	}

	res, err := s.runner.UsageReport(sandbox.UsageReportRequest{
		JobID:     sandbox.JobID(jobID),
		OutputDir: outputDir,
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return formatUsageReport(res), nil
}

// requireNonEmpty extracts a required, non-empty string param, returning an
// error result when the param is absent or empty.
func requireNonEmpty(req mcp.CallToolRequest, key string) (string, *mcp.CallToolResult) {
	v, err := req.RequireString(key)
	if err != nil {
		return "", mcp.NewToolResultError(err.Error())
	}
	if v == "" {
		return "", mcp.NewToolResultError(key + " is required")
	}
	return v, nil
}

// rejectOpenEgress returns a CallToolResult error when egress is
// "open" (policy: open egress combined with read-only /in mounts is
// the data-exfiltration shape demesne keeps off this surface; callers
// wanting open egress must use sandbox_research, which has no /in
// mounts). Returns (nil, false) when egress is not "open".
func rejectOpenEgress(egress string) (*mcp.CallToolResult, bool) {
	if sandbox.EgressMode(egress) != sandbox.EgressOpen {
		return nil, false
	}
	return mcp.NewToolResultError(
		"egress 'open' is not permitted; use sandbox_research for unrestricted egress",
	), true
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
