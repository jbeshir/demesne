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

// sandboxLayout captures the host paths and mounts for one agent run,
// abstracting over root (host-invoked) and child (in-sandbox-spawned)
// runs so runAgent has a single code path. inputVolumes are the
// /in/<basename> mounts; workspaceHost backs /workspace (fresh for a
// root, the parent's shared dir for a child); outHost backs /out
// (OutputRoot/<jobID>/out for a root, <parentOut>/child/<name> for a
// child); configDir backs the read-only agents.AgentConfigDir mount
// (context file + MCP config). resultsHost is bind-mounted into the
// sidecar only, so the agent can't tamper with usage.json.
type sandboxLayout struct {
	jobID         string
	inputVolumes  []opensandbox.Volume
	workspaceHost string
	outHost       string
	configDir     string
	resultsHost   string
	depth         int
	childName     string // empty for a root run
	// previousJobs maps each completed sibling's name to its outHost,
	// mounted read-only at /in/previous-jobs/<name>. Empty for a root
	// run and for the first child of a parent.
	previousJobs map[string]string
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
// before handing off. child is nil for host-invoked runs and set for
// in-sandbox-spawned children (which inherit inputs + workspace).
type internalAgentSpec struct {
	agentName   string
	model       string
	prompt      string
	preamble    string
	files       []string
	directories []string
	egress      EgressMode
	tool        string
	child       *childSpawn
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
	TotalUsageUSD float64
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

	layout, err := r.buildLayout(spec, prep.agent.ContextFileName())
	if err != nil {
		return agentRunResult{}, err
	}

	// Register this run so its own in-sandbox demesne tools can spawn
	// children that inherit our inputs + workspace and nest under our
	// /out. Deregister on the way out.
	r.registerChild(layout.jobID, &childContext{
		inputVolumes:  layout.inputVolumes,
		inputs:        prep.inputs,
		workspaceHost: layout.workspaceHost,
		outHost:       layout.outHost,
		depth:         layout.depth,
		usedNames:     map[string]bool{},
	})
	defer r.deregisterChild(layout.jobID)

	wiring := r.buildMCPWiring(layout.jobID)

	sb, err := r.createSandbox(ctx, spec, prep, layout, wiring.agentServers)
	if err != nil {
		return agentRunResult{}, err
	}
	defer killSandbox(ctx, sb)

	side, err := sidecar.Start(ctx, sb.ID(), sidecarImage, sidecar.ProxyConfig{
		AgentToken:    agentToken,
		UpstreamToken: r.cfg.ClaudeCodeOAuthToken,
		ResultsHost:   layout.resultsHost,
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

	ctxName := prep.agent.ContextFileName()
	// Each agent runs from a private subdirectory of the (possibly
	// shared) /workspace so the context-file symlink and working tree
	// never collide between siblings. OpenSandbox validates Cwd exists
	// before running, so we can't point Cwd at a not-yet-created dir —
	// instead run from /workspace and mkdir+cd into the private subdir
	// inside the command. The context file lives in the read-only
	// config-dir mount; symlinking it into cwd is how the CLI finds it.
	sub := agentCwdSubdir(layout.jobID)
	// Redirect the agent's stdout to /out/<transcript> and stderr to
	// /out/<stderr> inside the sandbox. /out is a host bind-mount, so the
	// structured transcript streams to the host live (partial output
	// survives an interrupted run) without any SDK streaming handlers.
	setup := fmt.Sprintf("mkdir -p %s && cd %s && ln -sf %s/%s ./%s",
		sub, sub, agents.AgentConfigDir, ctxName, ctxName)
	command := fmt.Sprintf("%s && %s > /out/%s 2> /out/%s",
		setup, shellQuote(prep.agent.Command(spec.prompt, prep.model)),
		agentTranscriptBasename, agentStderrBasename)

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

	// The agent's stdout went to the transcript file (not the SDK
	// stream), so recover the final answer from it for the MCP result.
	stdout := prep.agent.ResultText(readAgentTranscript(layout.outHost))

	usage := readUsageSnapshot(layout.resultsHost)
	// Copy the usage record into /out so it's surfaced alongside the
	// agent's artefacts when the host inspects the output dir later.
	copyUsageToOut(layout.resultsHost, layout.outHost)

	// Roll up own + descendant usage into results.json at /out.
	total := writeResults(layout, spec.tool, exitCode, usage.CostUSD)

	return agentRunResult{
		JobID:         layout.jobID,
		OutputPath:    layout.outHost,
		WorkspacePath: layout.workspaceHost,
		Stdout:        stdout,
		ExitCode:      exitCode,
		CostUSD:       usage.CostUSD,
		TotalUsageUSD: total,
	}, nil
}

// Agent stdout/stderr are redirected to these files under /out; /out is
// a host bind-mount so they stream to the host as the agent runs.
const (
	agentTranscriptBasename = "transcript.jsonl"
	agentStderrBasename     = "stderr.log"
)

// readAgentTranscript reads the agent's redirected stdout transcript
// from the host /out dir. Missing/unreadable yields nil so ResultText
// returns an empty string. The path is runner-composed under
// cfg.OutputRoot; gosec G304 false-positive.
func readAgentTranscript(outHost string) []byte {
	data, err := os.ReadFile(filepath.Join(outHost, agentTranscriptBasename)) //nolint:gosec
	if err != nil {
		return nil
	}
	return data
}

// agentCwdSubdir is the private per-run working directory relative to
// the /workspace mount root. Agents collaborate via absolute
// /workspace paths while keeping their own working tree and
// context-file symlink isolated under this subdir.
func agentCwdSubdir(jobID string) string {
	return ".demesne/" + jobID
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
	// Children inherit the parent's inputs (no Files/Directories of
	// their own); describe those for the context file instead.
	var inputs []agents.InputInfo
	if spec.child != nil {
		inputs = spec.child.parent.inputs
	} else {
		inputs, err = r.describeInputs(spec.files, spec.directories)
		if err != nil {
			return agentPrep{}, err
		}
	}
	tag, err := agent.EnsureImage(ctx)
	if err != nil {
		return agentPrep{}, fmt.Errorf("build agent image: %w", err)
	}
	return agentPrep{agent: agent, model: model, inputs: inputs, tag: tag}, nil
}

// buildLayout produces the host paths + mounts for an agent run,
// dispatching to the root or child builder.
func (r *Runner) buildLayout(spec internalAgentSpec, contextFileName string) (sandboxLayout, error) {
	if spec.child != nil {
		return r.buildChildLayout(spec.child, contextFileName)
	}
	return r.buildRootLayout(spec.files, spec.directories, contextFileName)
}

// buildRootLayout creates the host dirs for a host-invoked agent run:
// a fresh workspace, /out, config dir, and sidecar-results dir under
// OutputRoot/<jobID>, with caller-supplied inputs resolved into /in.
func (r *Runner) buildRootLayout(files, directories []string, _ string) (sandboxLayout, error) {
	inputVolumes, err := r.resolveMounts(files, directories)
	if err != nil {
		return sandboxLayout{}, err
	}
	jobID := uuid.NewString()
	jobDir := filepath.Join(r.cfg.OutputRoot, jobID)
	l := sandboxLayout{
		jobID:         jobID,
		inputVolumes:  inputVolumes,
		workspaceHost: filepath.Join(jobDir, "workspace"),
		outHost:       filepath.Join(jobDir, "out"),
		configDir:     filepath.Join(jobDir, "config"),
		resultsHost:   filepath.Join(jobDir, "sidecar-results"),
		depth:         0,
	}
	return l, mkLayoutDirs(l.workspaceHost, l.outHost, l.configDir, l.resultsHost)
}

// buildChildLayout creates the host dirs for an in-sandbox-spawned
// child: it inherits the parent's /in mounts and shared /workspace,
// nests its /out at <parentOut>/child/<name>, and keeps its own
// (private) config + sidecar-results dirs under OutputRoot/<jobID>.
// Reserving the name (unique per parent) is a side effect.
func (r *Runner) buildChildLayout(c *childSpawn, _ string) (sandboxLayout, error) {
	if err := c.parent.reserveName(c.name); err != nil {
		return sandboxLayout{}, err
	}
	prior := c.parent.priorSiblings()
	jobID := uuid.NewString()
	privDir := filepath.Join(r.cfg.OutputRoot, jobID)
	l := sandboxLayout{
		jobID:         jobID,
		inputVolumes:  c.parent.inputVolumes,
		workspaceHost: c.parent.workspaceHost,
		outHost:       filepath.Join(c.parent.outHost, "child", c.name),
		configDir:     filepath.Join(privDir, "config"),
		resultsHost:   filepath.Join(privDir, "sidecar-results"),
		depth:         c.parent.depth + 1,
		childName:     c.name,
		previousJobs:  prior,
	}
	// workspaceHost already exists (shared); only create our own dirs.
	if err := mkLayoutDirs(l.outHost, l.configDir, l.resultsHost); err != nil {
		return sandboxLayout{}, err
	}
	// Record once our output dir exists, so the next sibling (spawned
	// after this run returns) can mount it under /in/previous-jobs.
	c.parent.recordSibling(c.name, l.outHost)
	return l, nil
}

// mkLayoutDirs creates the given host directories. Paths are composed
// from r.cfg.OutputRoot, a uuid, and constant suffixes; gosec G703
// fires under -tags=integration but default lint is clean.
func mkLayoutDirs(dirs ...string) error {
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o750); err != nil { //nolint:gosec,nolintlint
			return fmt.Errorf("create %s: %w", d, err)
		}
	}
	return nil
}

// createSandbox writes the generated context file + agent config into
// the run's config dir and creates the sandbox from the layout's
// mounts.
func (r *Runner) createSandbox(
	ctx context.Context,
	spec internalAgentSpec,
	prep agentPrep,
	layout sandboxLayout,
	mcpServers []agents.MCPServerInfo,
) (*opensandbox.Sandbox, error) {
	policy, err := BuildNetworkPolicy(spec.egress, proxies.EgressHosts())
	if err != nil {
		return nil, err
	}
	ctxName := prep.agent.ContextFileName()
	body := prep.agent.GenerateContext(
		spec.preamble, spec.prompt, string(spec.egress), prep.inputs, mcpServers,
		previousJobNames(layout.previousJobs),
	)
	contextHost := filepath.Join(layout.configDir, ctxName)
	if err := os.WriteFile(contextHost, []byte(body), 0o600); err != nil {
		return nil, fmt.Errorf("write %s: %w", contextHost, err)
	}
	if err := prep.agent.WriteAgentConfig(layout.configDir, agents.AgentConfig{MCPServers: mcpServers}); err != nil {
		return nil, fmt.Errorf("write agent config: %w", err)
	}

	mounts := append([]opensandbox.Volume{}, layout.inputVolumes...)
	mounts = append(mounts, previousJobVolumes(layout.previousJobs)...)
	mounts = append(mounts, agentVolumes(layout)...)

	timeoutSec := oneShotSandboxTTLSeconds
	sb, err := opensandbox.CreateSandbox(ctx, r.connectionConfig(), opensandbox.SandboxCreateOptions{
		Image:          prep.tag,
		Volumes:        mounts,
		NetworkPolicy:  policy,
		TimeoutSeconds: &timeoutSec,
		Metadata: map[string]string{
			metadataDemesneJob:  layout.jobID,
			metadataDemesneTool: spec.tool,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create sandbox: %w", err)
	}
	return sb, nil
}

// agentVolumes is the set of mounts specific to an agent run: the
// read-only config dir (context file + MCP config) at
// agents.AgentConfigDir, the writable /workspace, and the writable
// /out. The sidecar-results dir is mounted only into the sidecar (see
// sidecar.Start), not into the agent. Inherited /in inputs are
// prepended by createSandbox.
func agentVolumes(l sandboxLayout) []opensandbox.Volume {
	return []opensandbox.Volume{
		{
			Name:      "agent-config",
			Host:      &opensandbox.Host{Path: l.configDir},
			MountPath: agents.AgentConfigDir,
			ReadOnly:  true,
		},
		{
			Name:      "workspace",
			Host:      &opensandbox.Host{Path: l.workspaceHost},
			MountPath: "/workspace",
		},
		{
			Name:      "out",
			Host:      &opensandbox.Host{Path: l.outHost},
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
// sandbox-local loopback. The demesne self-server's binding carries
// this run's jobID as its ParentJobID so the tunnel injects the
// trusted parent-identity header on calls to it.
func (r *Runner) buildMCPWiring(jobID string) mcpWiring {
	var w mcpWiring
	for i, name := range r.cfg.MCPServers {
		port := proxymcp.FirstListenPort + i
		up := sidecar.MCPUpstream{
			Name:       name,
			ListenPort: port,
			Path:       "/" + name + "/mcp",
		}
		if name == DemesneServerName {
			up.ParentJobID = jobID
		}
		w.sidecarUpstreams = append(w.sidecarUpstreams, up)
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
