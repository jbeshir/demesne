package sandbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/jbeshir/demesne/internal/agents"
	"github.com/jbeshir/demesne/internal/proxies"
	proxymcp "github.com/jbeshir/demesne/internal/proxies/mcp"
	"github.com/jbeshir/demesne/internal/sidecar"
)

// agentPaths captures the host paths created for an agent run.
type agentPaths struct {
	jobID         string
	outHost       string
	workspaceHost string
	contextHost   string
	// resultsHost is bind-mounted into the proxy sidecar (only) at
	// sidecar.SidecarResultsDir; the agent-vendor proxy writes
	// usage.json there. The agent container has no access to it, so
	// the agent can't tamper with usage records.
	resultsHost string
}

// agentPrep collects everything Agent resolves before touching the
// sandbox runtime: provider, model, inputs, image tag.
type agentPrep struct {
	agent  agents.Agent
	model  string
	inputs []agents.InputInfo
	tag    string
}

// internalAgentSpec is the internal request shape runAgent takes.
// Both the public Agent and Research entry points translate their
// public requests into this struct and set the tool metadata label
// before handing off.
type internalAgentSpec struct {
	agentName   string
	model       string
	prompt      string
	preamble    string
	files       []string
	directories []string
	egress      EgressMode
	tool        string
}

// agentRunResult is runAgent's return shape. The public adapters
// convert it to AgentResult / ResearchResult.
type agentRunResult struct {
	JobID         string
	OutputPath    string
	WorkspacePath string
	Stdout        string
	ExitCode      int
	CostUSD       float64
}

// Agent runs an agent (e.g. claude-code) inside a fresh sandbox against
// the caller's prompt.
//
// Sandbox layout (cwd = /workspace):
//   - /in/<basename>             read-only caller inputs (files + dirs)
//   - /in/<context-file>         read-only generated context file
//     (filename comes from agent.ContextFileName,
//     e.g. CLAUDE.md for claude-code)
//   - /workspace                 writable scratch — agent copies inputs here to mutate
//   - /out                       writable, output only — agent writes final artefacts here
//
// All three writable mounts are persisted on the host under
// cfg.OutputRoot/<jobID>/{out, workspace, context/<context-file>}.
// The context file is symlinked from /workspace/<context-file> so the
// CLI finds it via the usual cwd lookup.
func (r *Runner) Agent(ctx context.Context, req AgentRequest) (AgentResult, error) {
	spec := internalAgentSpec{
		agentName:   req.Agent,
		model:       req.Model,
		prompt:      req.Prompt,
		preamble:    req.Preamble,
		files:       req.Files,
		directories: req.Directories,
		egress:      egressOrDefault(req.Egress, EgressNone),
		tool:        "sandbox_agent",
	}
	res, err := r.runAgent(ctx, spec)
	if err != nil {
		return AgentResult{}, err
	}
	return AgentResult(res), nil
}

// runAgent is the shared implementation behind Agent and Research.
// It does the full create→start sidecar→exec→teardown cycle and reads
// the proxy's usage snapshot back off disk.
func (r *Runner) runAgent(ctx context.Context, spec internalAgentSpec) (agentRunResult, error) {
	prep, err := r.prepareAgent(ctx, spec)
	if err != nil {
		return agentRunResult{}, err
	}

	sidecarImage, err := sidecar.EnsureImage(ctx)
	if err != nil {
		return agentRunResult{}, fmt.Errorf("build sidecar image: %w", err)
	}

	// Per-sandbox fake credential. The agent never sees the real
	// upstream OAuth token; the agent-vendor proxy validates this
	// value on every inbound request and substitutes the real token
	// before forwarding.
	agentToken, err := generateAgentToken()
	if err != nil {
		return agentRunResult{}, fmt.Errorf("generate agent token: %w", err)
	}

	wiring := r.buildMCPWiring()

	sb, paths, err := r.createAgentSandbox(ctx, spec, prep, wiring.agentServers)
	if err != nil {
		return agentRunResult{}, err
	}
	defer killSandbox(ctx, sb)

	side, err := sidecar.Start(ctx, sb.ID(), sidecarImage, sidecar.ProxyConfig{
		AgentToken:    agentToken,
		UpstreamToken: r.cfg.ClaudeCodeOAuthToken,
		ResultsHost:   paths.resultsHost,
		MCPUpstreams:  wiring.sidecarUpstreams,
		MCPSocketHost: r.cfg.MCPSocketPath,
	})
	if err != nil {
		return agentRunResult{}, fmt.Errorf("start sidecar: %w", err)
	}
	defer func() {
		if err := side.Stop(context.WithoutCancel(ctx)); err != nil {
			log.Printf("%s: sidecar cleanup failed: %v", spec.tool, err)
		}
	}()

	setup := fmt.Sprintf("ln -sf /in/%s ./%s",
		prep.agent.ContextFileName(), prep.agent.ContextFileName())
	command := setup + " && " + shellQuote(prep.agent.Command(spec.prompt, prep.model))

	exec, err := sb.RunCommandWithOpts(ctx, opensandbox.RunCommandRequest{
		Command: command,
		Cwd:     "/workspace",
		Envs:    prep.agent.EnvVars(agentToken, prep.model),
		Timeout: commandTimeout.Milliseconds(),
	}, nil)
	if err != nil {
		return agentRunResult{}, fmt.Errorf("run agent: %w", err)
	}

	exitCode := 0
	if exec.ExitCode != nil {
		exitCode = *exec.ExitCode
	}

	usage := readUsageSnapshot(paths.resultsHost)
	// Copy the usage record into /out so it's surfaced alongside the
	// agent's artefacts when the host inspects the output dir later.
	copyUsageToOut(paths.resultsHost, paths.outHost)

	return agentRunResult{
		JobID:         paths.jobID,
		OutputPath:    paths.outHost,
		WorkspacePath: paths.workspaceHost,
		Stdout:        exec.Text(),
		ExitCode:      exitCode,
		CostUSD:       usage.CostUSD,
	}, nil
}

// prepareAgent validates the request, looks up the provider, resolves
// the model, describes inputs, and ensures the provider's image is
// built. No sandbox-runtime calls happen here.
func (r *Runner) prepareAgent(ctx context.Context, spec internalAgentSpec) (agentPrep, error) {
	if r.cfg.ClaudeCodeOAuthToken == "" {
		return agentPrep{}, errors.New(
			"DEMESNE_CLAUDE_CODE_OAUTH_TOKEN is required for " + spec.tool +
				" (run `claude setup-token` to obtain one)",
		)
	}
	if strings.TrimSpace(spec.prompt) == "" {
		return agentPrep{}, errors.New("prompt is required")
	}
	agent, err := agents.Lookup(spec.agentName)
	if err != nil {
		return agentPrep{}, err
	}
	model, err := agent.ResolveModel(spec.model)
	if err != nil {
		return agentPrep{}, err
	}
	inputs, err := r.describeInputs(spec.files, spec.directories)
	if err != nil {
		return agentPrep{}, err
	}
	tag, err := agent.EnsureImage(ctx)
	if err != nil {
		return agentPrep{}, fmt.Errorf("build agent image: %w", err)
	}
	return agentPrep{agent: agent, model: model, inputs: inputs, tag: tag}, nil
}

// createAgentSandbox builds the per-job host directories, writes the
// generated context file, and creates the sandbox. The host paths are
// returned so the caller can include them in AgentResult.
func (r *Runner) createAgentSandbox(
	ctx context.Context,
	spec internalAgentSpec,
	prep agentPrep,
	mcpServers []agents.MCPServerInfo,
) (*opensandbox.Sandbox, agentPaths, error) {
	policy, err := BuildNetworkPolicy(spec.egress, proxies.EgressHosts())
	if err != nil {
		return nil, agentPaths{}, err
	}
	mounts, err := r.resolveMounts(spec.files, spec.directories)
	if err != nil {
		return nil, agentPaths{}, err
	}
	paths, err := r.createAgentPaths(prep.agent.ContextFileName())
	if err != nil {
		return nil, agentPaths{}, err
	}
	body := prep.agent.GenerateContext(spec.preamble, spec.prompt, string(spec.egress), prep.inputs, mcpServers)
	if err := os.WriteFile(paths.contextHost, []byte(body), 0o600); err != nil {
		return nil, agentPaths{}, fmt.Errorf("write %s: %w", paths.contextHost, err)
	}
	if err := prep.agent.WriteAgentConfig(paths.workspaceHost, agents.AgentConfig{MCPServers: mcpServers}); err != nil {
		return nil, agentPaths{}, fmt.Errorf("write agent config: %w", err)
	}

	mounts = append(mounts, agentVolumes(paths, prep.agent.ContextFileName())...)

	timeoutSec := oneShotSandboxTTLSeconds
	sb, err := opensandbox.CreateSandbox(ctx, r.connectionConfig(), opensandbox.SandboxCreateOptions{
		Image:          prep.tag,
		Volumes:        mounts,
		NetworkPolicy:  policy,
		TimeoutSeconds: &timeoutSec,
		Metadata: map[string]string{
			metadataDemesneJob:  paths.jobID,
			metadataDemesneTool: spec.tool,
		},
	})
	if err != nil {
		return nil, agentPaths{}, fmt.Errorf("create sandbox: %w", err)
	}
	return sb, paths, nil
}

// createAgentPaths mints the per-job UUID and creates the four
// writable host directories used by the agent layout. resultsHost is
// the sidecar-only path the proxy writes usage.json to.
func (r *Runner) createAgentPaths(contextFileName string) (agentPaths, error) {
	jobID := uuid.NewString()
	jobDir := filepath.Join(r.cfg.OutputRoot, jobID)
	p := agentPaths{
		jobID:         jobID,
		outHost:       filepath.Join(jobDir, "out"),
		workspaceHost: filepath.Join(jobDir, "workspace"),
		contextHost:   filepath.Join(jobDir, "context", contextFileName),
		resultsHost:   filepath.Join(jobDir, "sidecar-results"),
	}
	for _, d := range []string{p.outHost, p.workspaceHost, filepath.Dir(p.contextHost), p.resultsHost} {
		// d is composed from r.cfg.OutputRoot, a uuid, and a constant suffix.
		// gosec G703 fires under -tags=integration; default lint is clean.
		if err := os.MkdirAll(d, 0o750); err != nil { //nolint:gosec,nolintlint
			return agentPaths{}, fmt.Errorf("create %s: %w", d, err)
		}
	}
	return p, nil
}

// agentVolumes is the set of writable + read-only volumes specific to
// an agent run — the context file, /workspace, and /out. The
// sidecar-results dir is mounted only into the sidecar (see
// sidecar.Start), not into the agent.
func agentVolumes(p agentPaths, contextFileName string) []opensandbox.Volume {
	return []opensandbox.Volume{
		{
			Name:      "context",
			Host:      &opensandbox.Host{Path: p.contextHost},
			MountPath: "/in/" + contextFileName,
			ReadOnly:  true,
		},
		{
			Name:      "workspace",
			Host:      &opensandbox.Host{Path: p.workspaceHost},
			MountPath: "/workspace",
		},
		{
			Name:      "out",
			Host:      &opensandbox.Host{Path: p.outHost},
			MountPath: "/out",
		},
	}
}

// describeInputs collects InputInfo (basename / IsDir / Size) for the
// context file generator without mutating anything. Each path is
// validated against AllowedPaths the same way resolveMounts validates
// later, so authorisation errors surface here too.
func (r *Runner) describeInputs(files, directories []string) ([]agents.InputInfo, error) {
	out := make([]agents.InputInfo, 0, len(files)+len(directories))
	collect := func(host string, wantDir bool) error {
		resolved, err := ValidateMountPath(host, r.cfg.AllowedPaths)
		if err != nil {
			return err
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return fmt.Errorf("stat %s: %w", host, err)
		}
		if wantDir && !info.IsDir() {
			return fmt.Errorf("%s is not a directory", host)
		}
		if !wantDir && !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", host)
		}
		size := int64(-1)
		if !wantDir {
			size = info.Size()
		}
		out = append(out, agents.InputInfo{
			Basename: filepath.Base(resolved),
			IsDir:    wantDir,
			Size:     size,
		})
		return nil
	}
	for _, f := range files {
		if err := collect(f, false); err != nil {
			return nil, err
		}
	}
	for _, d := range directories {
		if err := collect(d, true); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// generateAgentToken mints a per-sandbox random bearer token. The
// agent provider injects it as whatever auth env var its CLI expects
// (claude-code reads it as CLAUDE_CODE_OAUTH_TOKEN); the in-sidecar
// proxy validates it on every inbound request and substitutes the
// real upstream OAuth token before forwarding. The "demesne-agent-"
// prefix makes the value obvious in logs as a fake credential.
func generateAgentToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "demesne-agent-" + hex.EncodeToString(b), nil
}

// shellQuote joins argv into a single string that /bin/sh -c will
// re-tokenise identically. Each arg is single-quoted; embedded single
// quotes are emitted as '\”.
func shellQuote(args []string) string {
	var b strings.Builder
	for i, a := range args {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteByte('\'')
		b.WriteString(strings.ReplaceAll(a, "'", `'\''`))
		b.WriteByte('\'')
	}
	return b.String()
}

// egressOrDefault returns the given egress mode unless it's empty, in
// which case it returns the supplied default. Used by the Agent and
// Research adapters to apply their respective default modes.
func egressOrDefault(m, def EgressMode) EgressMode {
	if m == "" {
		return def
	}
	return m
}

// mcpWiring is the per-sandbox MCP plumbing derived from the host
// aggregator's exposed servers and tool catalogue: the sidecar
// tunnel upstreams (with assigned loopback ports — the host-side
// URL is completed in sidecar.Start once the egress gateway is
// known) and the agent-facing server descriptors.
type mcpWiring struct {
	sidecarUpstreams []sidecar.MCPUpstream
	agentServers     []agents.MCPServerInfo
}

// buildMCPWiring assigns each exposed server a sidecar loopback port
// (FirstListenPort + index, matching the aggregator's stable
// alphabetical ordering) and produces both the sidecar upstream
// list and the agent-facing server list. Agent-facing URLs are
// sandbox-local loopback; the sidecar's host-side upstream URL is
// built later (it needs the per-sandbox egress gateway IP).
func (r *Runner) buildMCPWiring() mcpWiring {
	var w mcpWiring
	for i, name := range r.cfg.MCPServers {
		port := proxymcp.FirstListenPort + i
		w.sidecarUpstreams = append(w.sidecarUpstreams, sidecar.MCPUpstream{
			Name:       name,
			ListenPort: port,
			Path:       "/" + name + "/mcp",
		})
		w.agentServers = append(w.agentServers, agents.MCPServerInfo{
			Name:  name,
			URL:   fmt.Sprintf("http://127.0.0.1:%d/mcp", port),
			Tools: toolInfos(r.cfg.MCPToolCatalogue[name]),
		})
	}
	return w
}

func toolInfos(tools []mcp.Tool) []agents.MCPToolInfo {
	out := make([]agents.MCPToolInfo, 0, len(tools))
	for _, t := range tools {
		out = append(out, agents.MCPToolInfo{Name: t.Name, Description: t.Description})
	}
	return out
}

// usageSnapshot is the subset of the proxy's usage.json that the
// runner needs to surface in AgentResult/ResearchResult.
type usageSnapshot struct {
	CostUSD float64 `json:"cost_usd"`
}

// readUsageSnapshot reads the proxy's usage.json from the sidecar
// results dir. Missing or malformed files return a zero value so the
// caller can rely on always-valid fields.
func readUsageSnapshot(resultsHost string) usageSnapshot {
	if resultsHost == "" {
		return usageSnapshot{}
	}
	// resultsHost is composed by the runner from r.cfg.OutputRoot +
	// a uuid; gosec G304 false-positive.
	data, err := os.ReadFile(filepath.Join(resultsHost, "usage.json")) //nolint:gosec
	if err != nil {
		return usageSnapshot{}
	}
	var s usageSnapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return usageSnapshot{}
	}
	return s
}

// copyUsageToOut copies the proxy's usage.json from the sidecar
// results dir to the agent's /out so the caller has a single place
// to find it. Best-effort: errors are silently dropped because the
// summary in AgentResult already carries the headline numbers.
func copyUsageToOut(resultsHost, outHost string) {
	src := filepath.Join(resultsHost, "usage.json")
	// src and outHost are runner-composed paths under r.cfg.OutputRoot;
	// gosec G304/G703 false-positives.
	data, err := os.ReadFile(src) //nolint:gosec
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(outHost, "usage.json"), data, 0o600) //nolint:gosec
}
