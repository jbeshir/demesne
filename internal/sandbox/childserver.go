package sandbox

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jbeshir/demesne/internal/mcpproxy"
	proxymcp "github.com/jbeshir/demesne/internal/proxies/mcp"
)

// keepaliveInterval is how often a child-spawning handler emits a
// progress notification while the child runs. It must be comfortably
// below any client-side idle timeout on the nested streamable-HTTP MCP
// connection (observed safe at 30s+).
const keepaliveInterval = 15 * time.Second

// keepaliveProgressID labels the keepalive progress notifications. It's
// an opaque MCP progress identifier, not a credential.
const keepaliveProgressID = "demesne-keepalive"

const (
	childParamName      = "name"
	childParamCommand   = "command"
	childParamImage     = "image"
	childParamEgress    = "egress"
	childParamPrompt    = "prompt"
	childParamAgent     = "agent"
	childParamModel     = "model"
	childParamPreamble  = "preamble"
	childParamSandboxID = "sandbox_id"
)

// keepAlive holds the nested MCP connection open while a child runs. A
// child-spawning handler blocks for the child's whole lifecycle and
// sends nothing over MCP (the child's output streams to /out, a host
// mount), so the held-open streamable-HTTP POST goes idle and the
// agent-side client tears it down — cancelling the call and killing the
// child. Emitting a periodic progress notification (mcp-go writes it
// onto the held-open POST; the sidecar tunnel forwards it) keeps the
// stream warm. Returns a stop func; no-ops when not served over the
// streamable-HTTP transport (e.g. unit tests).
func keepAlive(ctx context.Context) func() {
	srv := server.ServerFromContext(ctx)
	if srv == nil {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(keepaliveInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = srv.SendNotificationToClient(ctx, "notifications/progress", map[string]any{
					"progressToken": keepaliveProgressID,
					"progress":      0,
					"message":       "demesne: child sandbox still running",
				})
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	return func() { close(done) }
}

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
	srv := server.NewMCPServer(mcpproxy.DemesneServerName, "0", server.WithToolCapabilities(false))
	var catalogue []mcp.Tool
	add := func(tool mcp.Tool, h server.ToolHandlerFunc) {
		srv.AddTool(tool, h)
		catalogue = append(catalogue, tool)
	}

	add(mcp.NewTool(toolSandboxScript,
		mcp.WithDescription(childScriptDescription),
		mcp.WithString(childParamName, mcp.Required(), mcp.Description(childNameDescription)),
		mcp.WithString(childParamCommand, mcp.Required(),
			mcp.Description("Shell command to run. /bin/sh -c, cwd /out.")),
		mcp.WithString(childParamImage, mcp.Description(childImageDescription)),
		mcp.WithString(childParamEgress, mcp.Description(childEgressDescription)),
	), r.handleChildScript)

	add(mcp.NewTool(toolSandboxAgent,
		mcp.WithDescription(childAgentDescription),
		mcp.WithString(childParamName, mcp.Required(), mcp.Description(childNameDescription)),
		mcp.WithString(childParamPrompt, mcp.Required(), mcp.Description("Task for the child agent.")),
		mcp.WithString(childParamAgent, mcp.Description("Agent provider. Defaults to 'claude-code'.")),
		mcp.WithString(childParamModel, mcp.Description("Model: 'opus', 'sonnet' (default), or 'haiku'.")),
		mcp.WithString(childParamPreamble, mcp.Description("Prose prepended to the child's context file.")),
		mcp.WithString(childParamEgress, mcp.Description(childEgressDescription)),
	), r.handleChildAgent)

	add(mcp.NewTool(toolSandboxResearch,
		mcp.WithDescription(childResearchDescription),
		mcp.WithString(childParamName, mcp.Required(), mcp.Description(childNameDescription)),
		mcp.WithString(childParamPrompt, mcp.Required(), mcp.Description("Research task for the child agent.")),
		mcp.WithString(childParamAgent, mcp.Description("Agent provider. Defaults to 'claude-code'.")),
		mcp.WithString(childParamModel, mcp.Description("Model: 'opus', 'sonnet' (default), or 'haiku'.")),
		mcp.WithString(childParamPreamble, mcp.Description("Prose prepended to the child's context file.")),
	), r.handleChildResearch)

	add(mcp.NewTool(toolSandboxCreate,
		mcp.WithDescription(childCreateDescription),
		mcp.WithString(childParamName, mcp.Required(), mcp.Description(childNameDescription)),
		mcp.WithString(childParamImage, mcp.Description(childImageDescription)),
		mcp.WithString(childParamEgress, mcp.Description(childEgressDescription)),
	), r.handleChildCreate)

	add(mcp.NewTool("sandbox_exec",
		mcp.WithDescription("Run a shell command in a child sandbox created by sandbox_create. /bin/sh -c, cwd /out."),
		mcp.WithString(childParamSandboxID, mcp.Required(), mcp.Description("Handle from sandbox_create.")),
		mcp.WithString(childParamCommand, mcp.Required(), mcp.Description("Shell command to run.")),
	), r.handleChildExec)

	add(mcp.NewTool("sandbox_destroy",
		mcp.WithDescription(childDestroyDescription),
		mcp.WithString(childParamSandboxID, mcp.Required(), mcp.Description("Handle from sandbox_create.")),
	), r.handleChildDestroy)

	h := server.NewStreamableHTTPServer(srv, server.WithHTTPContextFunc(parentFromRequest))
	return mcpproxy.DemesneServerName, catalogue, h
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
	defer keepAlive(ctx)()
	parent, name, errResult := r.childSpawnParams(ctx, req)
	if errResult != nil {
		return errResult, nil
	}
	command, err := req.RequireString(childParamCommand)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	egress := req.GetString(childParamEgress, string(EgressPackageManagers))
	if errResult := rejectOpenEgress(egress); errResult != nil {
		return errResult, nil
	}
	res, err := r.runScript(ctx, ScriptRequest{
		Command: command,
		Image:   req.GetString(childParamImage, ""),
		Egress:  EgressMode(egress),
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
	defer keepAlive(ctx)()
	parent, name, errResult := r.childSpawnParams(ctx, req)
	if errResult != nil {
		return errResult, nil
	}
	prompt, err := req.RequireString(childParamPrompt)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	egress := req.GetString(childParamEgress, string(EgressNone))
	if errResult := rejectOpenEgress(egress); errResult != nil {
		return errResult, nil
	}
	spec := internalAgentSpec{
		agentName: req.GetString(childParamAgent, ""),
		model:     req.GetString(childParamModel, ""),
		prompt:    prompt,
		preamble:  req.GetString(childParamPreamble, ""),
		egress:    EgressMode(egress),
		tool:      toolSandboxAgent,
		child:     &childSpawn{name: name, parent: parent},
	}
	res, err := r.runAgent(ctx, spec)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(formatChildAgentResult(name, res)), nil
}

func (r *Runner) handleChildResearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	defer keepAlive(ctx)()
	parent, name, errResult := r.childSpawnParams(ctx, req)
	if errResult != nil {
		return errResult, nil
	}
	prompt, err := req.RequireString(childParamPrompt)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	spec := internalAgentSpec{
		agentName: req.GetString(childParamAgent, ""),
		model:     req.GetString(childParamModel, ""),
		prompt:    prompt,
		preamble:  req.GetString(childParamPreamble, ""),
		egress:    EgressOpen,
		tool:      toolSandboxResearch,
		// Research is isolated like the host tool: no inherited /in or
		// shared /workspace, just a fresh sandbox with open egress —
		// inputs + open egress is the exfil shape we keep off the surface.
		child: &childSpawn{name: name, parent: parent, isolated: true},
	}
	res, err := r.runAgent(ctx, spec)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(formatChildAgentResult(name, res)), nil
}

func (r *Runner) handleChildCreate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	defer keepAlive(ctx)()
	parent, name, errResult := r.childSpawnParams(ctx, req)
	if errResult != nil {
		return errResult, nil
	}
	egress := req.GetString(childParamEgress, string(EgressPackageManagers))
	if errResult := rejectOpenEgress(egress); errResult != nil {
		return errResult, nil
	}
	res, err := r.create(ctx, CreateRequest{
		Image:  req.GetString(childParamImage, ""),
		Egress: EgressMode(egress),
	}, &childSpawn{name: name, parent: parent})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf(
		"name: %s\nsandbox_id: %s\noutput_dir: %s", name, res.SandboxID, res.OutputPath,
	)), nil
}

func (r *Runner) handleChildExec(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	defer keepAlive(ctx)()
	sandboxID, err := req.RequireString(childParamSandboxID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	command, err := req.RequireString(childParamCommand)
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
	defer keepAlive(ctx)()
	sandboxID, err := req.RequireString(childParamSandboxID)
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
	name, err := req.RequireString(childParamName)
	if err != nil {
		return nil, "", mcp.NewToolResultError(err.Error())
	}
	return parent, name, nil
}

// rejectOpenEgress returns an error result when a child tool that
// inherits the parent's read-only /in mounts is asked for 'open' egress.
// Inputs plus unrestricted outbound is the data-exfiltration shape demesne
// keeps off the child surface: sandbox_research is the only route to open
// egress, and it deliberately mounts no inputs. Returns nil when egress is
// acceptable (sandbox_script/sandbox_create/sandbox_agent share this guard).
func rejectOpenEgress(egress string) *mcp.CallToolResult {
	if EgressMode(egress) == EgressOpen {
		return mcp.NewToolResultError(
			"egress 'open' is not permitted here; it would combine caller inputs with " +
				"unrestricted outbound. Use sandbox_research (no input mounts) for open egress.")
	}
	return nil
}

func formatChildAgentResult(name string, res AgentResult) string {
	return fmt.Sprintf(
		"name: %s\nexit_code: %d\noutput_dir: %s\ncost_usd: %.4f\ntotal_usage_usd: %.4f\n---\n%s",
		name, res.ExitCode, res.OutputPath, res.CostUSD, res.TotalUsageUSD, res.Stdout,
	)
}

const childNameDescription = "Unique name for this child within the current sandbox. " +
	"Its output appears at /out/child/<name> (visible to you and your ancestors). " +
	"Allowed characters: lowercase letters, digits, and interior hyphens only " +
	"(no dots, underscores, or uppercase); at most 40 characters."

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
egress knob. Runs in a FRESH private workspace with NO /in mounts
(unlike sandbox_agent); output at /out/child/<name>.`

const childCreateDescription = `Create a persistent child sandbox and return its handle.

Inherits this sandbox's read-only /in inputs and shared /workspace;
its writable /out is /out/child/<name>. Drive it with sandbox_exec and
tear it down with sandbox_destroy (both take the returned sandbox_id).`

const childDestroyDescription = "Destroy a child sandbox created by sandbox_create. " +
	"Its /out is preserved under the parent's tree."
