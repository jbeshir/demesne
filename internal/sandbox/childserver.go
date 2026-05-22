package sandbox

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	proxymcp "github.com/jbeshir/demesne/internal/proxies/mcp"
)

// DemesneServerName is the name the in-process demesne self-server is
// mounted under on the host MCP aggregator (and thus the MCP server
// name a sandboxed agent sees). The runner's buildMCPWiring keys the
// parent-identity header off this name.
const DemesneServerName = "demesne"

// parentKey carries the calling sandbox's jobID (from the trusted
// tunnel header) through the MCP handler context.
type parentKeyT struct{}

var parentKey parentKeyT

// parentFromRequest lifts the tunnel-injected parent-identity header
// into the MCP tool handler context. The header is set by the sidecar
// tunnel (the agent can't forge it), so its presence is authoritative.
func parentFromRequest(ctx context.Context, req *http.Request) context.Context {
	if jobID := req.Header.Get(proxymcp.ParentHeader); jobID != "" {
		return context.WithValue(ctx, parentKey, jobID)
	}
	return ctx
}

// ChildMCPServer builds the in-process demesne MCP server exposing the
// child-spawning tools, plus the tool catalogue and HTTP handler the
// aggregator mounts. The handler reads the parent-identity header into
// the context via WithHTTPContextFunc. Returned for the aggregator's
// ExtraServers and the runner's MCP wiring.
func (r *Runner) ChildMCPServer() (string, []mcp.Tool, http.Handler) {
	srv := server.NewMCPServer(DemesneServerName, "0", server.WithToolCapabilities(false))
	var catalogue []mcp.Tool
	add := func(tool mcp.Tool, h server.ToolHandlerFunc) {
		srv.AddTool(tool, h)
		catalogue = append(catalogue, tool)
	}

	add(mcp.NewTool("sandbox_script",
		mcp.WithDescription(childScriptDescription),
		mcp.WithString("name", mcp.Required(), mcp.Description(childNameDescription)),
		mcp.WithString("command", mcp.Required(),
			mcp.Description("Shell command to run. /bin/sh -c, cwd /out.")),
		mcp.WithString("image", mcp.Description(childImageDescription)),
		mcp.WithString("egress", mcp.Description(childEgressDescription)),
	), r.handleChildScript)

	add(mcp.NewTool("sandbox_agent",
		mcp.WithDescription(childAgentDescription),
		mcp.WithString("name", mcp.Required(), mcp.Description(childNameDescription)),
		mcp.WithString("prompt", mcp.Required(), mcp.Description("Task for the child agent.")),
		mcp.WithString("agent", mcp.Description("Agent provider. Defaults to 'claude-code'.")),
		mcp.WithString("model", mcp.Description("Model: 'opus', 'sonnet' (default), or 'haiku'.")),
		mcp.WithString("preamble", mcp.Description("Prose prepended to the child's context file.")),
		mcp.WithString("egress", mcp.Description(childEgressDescription)),
	), r.handleChildAgent)

	add(mcp.NewTool("sandbox_research",
		mcp.WithDescription(childResearchDescription),
		mcp.WithString("name", mcp.Required(), mcp.Description(childNameDescription)),
		mcp.WithString("prompt", mcp.Required(), mcp.Description("Research task for the child agent.")),
		mcp.WithString("agent", mcp.Description("Agent provider. Defaults to 'claude-code'.")),
		mcp.WithString("model", mcp.Description("Model: 'opus', 'sonnet' (default), or 'haiku'.")),
		mcp.WithString("preamble", mcp.Description("Prose prepended to the child's context file.")),
	), r.handleChildResearch)

	add(mcp.NewTool("sandbox_create",
		mcp.WithDescription(childCreateDescription),
		mcp.WithString("name", mcp.Required(), mcp.Description(childNameDescription)),
		mcp.WithString("image", mcp.Description(childImageDescription)),
		mcp.WithString("egress", mcp.Description(childEgressDescription)),
	), r.handleChildCreate)

	add(mcp.NewTool("sandbox_exec",
		mcp.WithDescription("Run a shell command in a child sandbox created by sandbox_create. /bin/sh -c, cwd /out."),
		mcp.WithString("sandbox_id", mcp.Required(), mcp.Description("Handle from sandbox_create.")),
		mcp.WithString("command", mcp.Required(), mcp.Description("Shell command to run.")),
	), r.handleChildExec)

	add(mcp.NewTool("sandbox_destroy",
		mcp.WithDescription(childDestroyDescription),
		mcp.WithString("sandbox_id", mcp.Required(), mcp.Description("Handle from sandbox_create.")),
	), r.handleChildDestroy)

	h := server.NewStreamableHTTPServer(srv, server.WithHTTPContextFunc(parentFromRequest))
	return DemesneServerName, catalogue, h
}

// parentFor resolves the calling sandbox's spawning context from the
// trusted identity header. An empty/unknown header means the caller
// isn't a registered agent run (should not happen via the tunnel).
func (r *Runner) parentFor(ctx context.Context) (*childContext, error) {
	jobID, _ := ctx.Value(parentKey).(string)
	if jobID == "" {
		return nil, errors.New("no parent sandbox identity on request")
	}
	c, ok := r.lookupChild(jobID)
	if !ok {
		return nil, errors.New("calling sandbox is not a registered agent run")
	}
	return c, nil
}

func (r *Runner) handleChildScript(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	parent, name, errResult := r.childSpawnParams(ctx, req)
	if errResult != nil {
		return errResult, nil
	}
	command, err := req.RequireString("command")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	res, err := r.runScript(ctx, ScriptRequest{
		Command: command,
		Image:   req.GetString("image", ""),
		Egress:  EgressMode(req.GetString("egress", string(EgressPackageManagers))),
	}, &childSpawn{name: name, parent: parent})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf(
		"name: %s\nexit_code: %d\noutput_dir: %s\n---\n%s",
		name, res.ExitCode, res.OutputPath, res.Stdout,
	)), nil
}

func (r *Runner) handleChildAgent(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	parent, name, errResult := r.childSpawnParams(ctx, req)
	if errResult != nil {
		return errResult, nil
	}
	prompt, err := req.RequireString("prompt")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	egress := req.GetString("egress", string(EgressNone))
	if EgressMode(egress) == EgressOpen {
		return mcp.NewToolResultError(
			"egress 'open' is not permitted for sandbox_agent; use sandbox_research"), nil
	}
	spec := internalAgentSpec{
		agentName: req.GetString("agent", ""),
		model:     req.GetString("model", ""),
		prompt:    prompt,
		preamble:  req.GetString("preamble", ""),
		egress:    EgressMode(egress),
		tool:      "sandbox_agent",
		child:     &childSpawn{name: name, parent: parent},
	}
	res, err := r.runAgent(ctx, spec)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(formatChildAgentResult(name, res)), nil
}

func (r *Runner) handleChildResearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	parent, name, errResult := r.childSpawnParams(ctx, req)
	if errResult != nil {
		return errResult, nil
	}
	prompt, err := req.RequireString("prompt")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	spec := internalAgentSpec{
		agentName: req.GetString("agent", ""),
		model:     req.GetString("model", ""),
		prompt:    prompt,
		preamble:  req.GetString("preamble", ""),
		egress:    EgressOpen,
		tool:      "sandbox_research",
		child:     &childSpawn{name: name, parent: parent},
	}
	res, err := r.runAgent(ctx, spec)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(formatChildAgentResult(name, res)), nil
}

func (r *Runner) handleChildCreate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	parent, name, errResult := r.childSpawnParams(ctx, req)
	if errResult != nil {
		return errResult, nil
	}
	res, err := r.create(ctx, CreateRequest{
		Image:  req.GetString("image", ""),
		Egress: EgressMode(req.GetString("egress", string(EgressPackageManagers))),
	}, &childSpawn{name: name, parent: parent})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf(
		"name: %s\nsandbox_id: %s\noutput_dir: %s", name, res.SandboxID, res.OutputPath,
	)), nil
}

func (r *Runner) handleChildExec(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sandboxID, err := req.RequireString("sandbox_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	command, err := req.RequireString("command")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	res, err := r.Exec(ctx, ExecRequest{SandboxID: sandboxID, Command: command})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("exit_code: %d\n---\n%s", res.ExitCode, res.Stdout)), nil
}

func (r *Runner) handleChildDestroy(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sandboxID, err := req.RequireString("sandbox_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := r.Destroy(ctx, DestroyRequest{SandboxID: sandboxID}); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText("destroyed: " + sandboxID), nil
}

// childSpawnParams resolves the parent context and the required name
// shared by every spawning tool. On error it returns a tool-result
// error (the caller returns it directly) instead of a Go error so the
// agent sees a clean message.
func (r *Runner) childSpawnParams(
	ctx context.Context,
	req mcp.CallToolRequest,
) (*childContext, string, *mcp.CallToolResult) {
	parent, err := r.parentFor(ctx)
	if err != nil {
		return nil, "", mcp.NewToolResultError(err.Error())
	}
	name, err := req.RequireString("name")
	if err != nil {
		return nil, "", mcp.NewToolResultError(err.Error())
	}
	if err := validateChildName(name); err != nil {
		return nil, "", mcp.NewToolResultError(err.Error())
	}
	return parent, name, nil
}

func formatChildAgentResult(name string, res agentRunResult) string {
	return fmt.Sprintf(
		"name: %s\nexit_code: %d\noutput_dir: %s\ncost_usd: %.4f\ntotal_usage_usd: %.4f\n---\n%s",
		name, res.ExitCode, res.OutputPath, res.CostUSD, res.TotalUsageUSD, res.Stdout,
	)
}

const childNameDescription = "Unique name for this child within the current sandbox. " +
	"Its output appears at /out/child/<name> (visible to you and your ancestors). " +
	"Allowed characters: letters, digits, '.', '_', '-'."

const childImageDescription = "Container image: 'node', 'python', 'go', or 'anaconda' (default)."

const childEgressDescription = "Outbound policy: 'package-managers' (default) or 'none'."

const childScriptDescription = `Run one shell command in a fresh child sandbox and return its stdout.

The child inherits this sandbox's read-only /in inputs and shared
/workspace; its /out is /out/child/<name>, which you can read back
afterwards. The child is destroyed when the command returns.`

const childAgentDescription = `Spawn a child AI agent in a fresh sandbox against a prompt.

The child inherits this sandbox's read-only /in inputs and shared
/workspace (collaborate via absolute /workspace paths). It cannot be
given its own input mounts. Its output lands at /out/child/<name>;
deeper descendants nest further under that path. 'open' egress is not
permitted here — use sandbox_research.`

const childResearchDescription = `Spawn a long-running child research agent with open internet egress.

Like sandbox_agent but with unrestricted outbound access and no extra
egress knob. Inherits /in and shared /workspace; output at
/out/child/<name>.`

const childCreateDescription = `Create a persistent child sandbox and return its handle.

Inherits this sandbox's read-only /in inputs and shared /workspace;
its writable /out is /out/child/<name>. Drive it with sandbox_exec and
tear it down with sandbox_destroy (both take the returned sandbox_id).`

const childDestroyDescription = "Destroy a child sandbox created by sandbox_create. " +
	"Its /out is preserved under the parent's tree."
