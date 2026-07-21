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
	"github.com/jbeshir/demesne/internal/mcpproxy"
	"github.com/jbeshir/demesne/internal/proxies"
	proxygo "github.com/jbeshir/demesne/internal/proxies/goproxy"
	proxymcp "github.com/jbeshir/demesne/internal/proxies/mcp"
	proxyopenai "github.com/jbeshir/demesne/internal/proxies/openai"
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
	jobID         JobID
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
// sandbox runtime: provider, model, inputs, image tag, and the
// freshly host-refreshed Codex token set (zero for non-OpenAI agents).
type agentPrep struct {
	agent       agents.Agent
	model       agents.ModelName
	inputs      []agents.InputInfo
	tag         ImageURI
	codexTokens proxyopenai.TokenSet
}

// internalAgentSpec is the internal request shape runAgent takes.
// Both the public Agent and Research entry points translate their
// public requests into this struct and set the tool metadata label
// before handing off. child is nil for host-invoked runs and set for
// in-sandbox-spawned children (which inherit inputs + workspace).
type internalAgentSpec struct {
	model           string
	prompt          string
	preamble        string
	files           []string
	directories     []string
	egress          EgressMode
	tool            string
	child           *childSpawn
	outputPath      string
	outputFormat    string
	successCriteria []string
	// onOutputReady is the live-Status hook for background runs; nil for
	// blocking callers. It records outHost/resultsHost into the job's in-memory
	// fields so Status can read them while the run is still in progress.
	onOutputReady func(string, string)
	// bgSelf is the public JobID handle for the background job running
	// this agent. It is stamped onto the spawnContext so nested child
	// background jobs can register under this parent.
	bgSelf JobID
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
		model:           req.Model,
		prompt:          req.Prompt,
		preamble:        req.Preamble,
		files:           req.Files,
		directories:     req.Directories,
		egress:          egressOrDefault(req.Egress, EgressNone),
		tool:            ToolSandboxAgent,
		outputPath:      req.OutputPath,
		outputFormat:    req.OutputFormat,
		successCriteria: req.SuccessCriteria,
	}
	res, err := r.runAgent(ctx, spec)
	if err != nil {
		return AgentResult{}, err
	}
	return res, nil
}

// runAgent is the shared implementation behind Agent and Research.
// It does the full create→start sidecar→exec→teardown cycle and reads
// the proxy's usage snapshot back off disk.
func (r *Runner) runAgent(ctx context.Context, spec internalAgentSpec) (AgentResult, error) {
	prep, err := r.prepareAgent(ctx, spec)
	if err != nil {
		return AgentResult{}, err
	}

	sidecarImage, err := sidecar.EnsureImage(ctx)
	if err != nil {
		return AgentResult{}, fmt.Errorf("build sidecar image: %w", err)
	}

	// Per-sandbox fake credential. The agent never sees the real
	// upstream OAuth token; the vendor proxy validates this
	// value on every inbound request and substitutes the real token
	// before forwarding.
	agentToken, err := generateAgentToken()
	if err != nil {
		return AgentResult{}, fmt.Errorf("generate agent token: %w", err)
	}

	layout, err := r.buildLayout(spec)
	if err != nil {
		return AgentResult{}, err
	}

	if spec.onOutputReady != nil {
		spec.onOutputReady(layout.outHost, layout.resultsHost)
	}

	// Register this run so its own in-sandbox demesne tools can spawn
	// children that inherit our inputs + workspace and nest under our
	// /out. Deregister on the way out.
	r.registry.Register(layout.jobID, &spawnContext{
		inputVolumes:   layout.inputVolumes,
		inputs:         prep.inputs,
		workspaceHost:  layout.workspaceHost,
		outHost:        layout.outHost,
		depth:          layout.depth,
		bgJobID:        spec.bgSelf,
		usedNames:      map[string]bool{},
		siblingOutputs: map[string]string{},
	})
	defer r.registry.Deregister(layout.jobID)

	wiring := r.buildMCPWiring(layout.jobID)

	sb, err := r.createSandbox(ctx, spec, prep, layout, wiring.agentServers)
	if err != nil {
		return AgentResult{}, err
	}
	recordChildSibling(spec, layout)
	defer killSandbox(ctx, sb)

	proxyCfg := r.buildProxyConfig(prep.agent, agentToken, layout.resultsHost, prep.codexTokens)
	if len(wiring.sidecarUpstreams) > 0 {
		proxyCfg.MCP = &sidecar.MCPTunnelConfig{
			Upstreams:  wiring.sidecarUpstreams,
			SocketHost: r.cfg.MCPSocketPath,
		}
	}
	if _, err := sidecar.Start(ctx, sb.ID(), sidecarImage, proxyCfg); err != nil {
		return AgentResult{}, fmt.Errorf("start sidecar: %w", err)
	}
	defer func() {
		// sidecar.Remove is idempotent by deterministic name and retries
		// on transient docker errors. It MUST run before killSandbox (LIFO):
		// our sidecar shares the egress container's network/PID namespace,
		// so podman refuses to remove the egress while ours still exists,
		// leaking both containers. See internal/sidecar/runtime.go.
		if err := sidecar.Remove(context.WithoutCancel(ctx), sb.ID()); err != nil {
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
	command := fmt.Sprintf("%s && { %s > /out/%s 2> /out/%s; rc=$?; %s; exit $rc; }",
		setup, shellQuote(prep.agent.Command(spec.prompt, prep.model)),
		agentTranscriptBasename, agentStderrBasename, prep.agent.PostRunCapture())

	exec, err := sb.RunCommandWithOpts(ctx, opensandbox.RunCommandRequest{
		Command: command,
		Cwd:     mountWorkspace,
		Envs:    prep.agent.EnvVars(agentToken, prep.model),
		// Timeout is in milliseconds per SDK api/execd/gen.go; sibling
		// SandboxCreateOptions.TimeoutSeconds is seconds (units differ).
		Timeout: commandTimeout.Milliseconds(),
	}, nil)
	if err != nil {
		return AgentResult{}, fmt.Errorf("run agent: %w", err)
	}

	return finishAgentRun(exec.ExitCode, prep, layout, spec.tool), nil
}

// recordChildSibling records the child sibling after a successful sandbox
// create. Extracted from runAgent to reduce its cyclomatic complexity.
func recordChildSibling(spec internalAgentSpec, layout sandboxLayout) {
	// Record this child as a sibling only after a successful create, so a
	// failed spawn never poisons later siblings' /in/previous-jobs mounts.
	if spec.child != nil {
		spec.child.parent.recordSibling(spec.child.name, layout.outHost)
	}
}

// finishAgentRun reads the usage snapshot and transcript from disk, copies
// usage into /out, distills attribution records (anthropic-only), rolls up
// results.json, and builds the AgentResult.
// Extracted from runAgent to keep its cyclomatic complexity in bounds.
func finishAgentRun(exitCodePtr *int, prep agentPrep, layout sandboxLayout, tool string) AgentResult {
	exitCode := 0
	if exitCodePtr != nil {
		exitCode = *exitCodePtr
	}
	stdout := tailStdout(prep.agent.ResultText(readAgentTranscript(layout.outHost)))
	usage := readUsageSnapshot(layout.resultsHost)
	copyUsageToOut(layout.resultsHost, layout.outHost)
	if prep.agent.ProxyVendor() == agents.ProxyAnthropic {
		distillAttribution(layout.workspaceHost, layout.jobID, layout.outHost)
	}
	total, perModel := writeResults(layout, tool, exitCode, usage.CostUSD)
	return AgentResult{
		JobID:          layout.jobID,
		OutputPath:     layout.outHost,
		WorkspacePath:  layout.workspaceHost,
		Stdout:         stdout,
		ExitCode:       exitCode,
		CostUSD:        usage.CostUSD,
		TotalUsageUSD:  total,
		Stderr:         tailStderr(readAgentStderr(layout.outHost)),
		PerModelTokens: perModel,
		UsageSummary:   buildUsageSummary(perModel, layout.outHost),
	}
}

// buildProxyConfig selects the vendor proxy for the
// agent's vendor: the agent never sees the real upstream credential, so
// the runner hands it to the sidecar proxy here (validate fake token →
// swap real). codexTokens is the freshly host-refreshed token set
// passed in by the caller for OpenAI-vendor agents. The MCP tunnel is
// layered on by the caller. Exactly one vendor branch fires; an
// unrecognised vendor yields an empty config (no proxy), which
// sidecar.Start treats as a non-agent run.
func (r *Runner) buildProxyConfig(
	agent agents.Agent, agentToken, resultsHost string, codexTokens proxyopenai.TokenSet,
) sidecar.ProxyConfig {
	switch agent.ProxyVendor() {
	case agents.ProxyAnthropic:
		return sidecar.ProxyConfig{Anthropic: &sidecar.AnthropicProxyConfig{
			AgentToken:    agentToken,
			UpstreamToken: r.cfg.ClaudeCodeOAuthToken,
			ResultsHost:   resultsHost,
		}}
	case agents.ProxyOpenAI:
		return sidecar.ProxyConfig{Codex: &sidecar.CodexProxyConfig{
			AgentToken:  agentToken,
			Tokens:      codexTokens,
			ResultsHost: resultsHost,
		}}
	default:
		return sidecar.ProxyConfig{}
	}
}

// Agent stdout/stderr are redirected to these files under /out; /out is
// a host bind-mount so they stream to the host as the agent runs.
const (
	agentTranscriptBasename = "transcript.jsonl"
	agentStderrBasename     = "stderr.log"
)

// readAgentTranscript reads the agent's redirected stdout transcript
// from the host /out dir. Missing/unreadable yields nil so ResultText
// returns an empty string. The path is runner-composed under cfg.OutputRoot.
func readAgentTranscript(outHost string) []byte {
	data, err := readOutputFile(outHost, agentTranscriptBasename)
	if err != nil {
		return nil
	}
	return data
}

// readAgentStderr reads the agent's redirected stderr from the host /out
// dir. Missing/unreadable yields nil; the caller tail-bounds and surfaces
// it. The path is runner-composed under cfg.OutputRoot.
func readAgentStderr(outHost string) []byte {
	data, err := readOutputFile(outHost, agentStderrBasename)
	if err != nil {
		return nil
	}
	return data
}

// agentCwdSubdir is the private per-run working directory relative to
// the /workspace mount root. Agents collaborate via absolute
// /workspace paths while keeping their own working tree and
// context-file symlink isolated under this subdir.
func agentCwdSubdir(jobID JobID) string {
	return ".demesne/" + string(jobID)
}

// sandboxEnv is the environment injected into every sandbox at create
// time.
//
// GOPROXY points at the sidecar's Go module proxy so `go` fetches
// modules via 127.0.0.1 (the SO_MARK bypass reaches the real proxy)
// even under egress=none. The /sumdb/ path is proxied too, so checksum
// verification works unchanged.
//
// The remaining vars silence telemetry / metrics / analytics /
// update-checks in the JS, Python, and Cloudflare build toolchains.
// Under restricted egress these phone-home calls don't just leak data:
// they hit the deny-by-default network policy and stall the build until
// they time out. Names and values are exact — many tools accept only a
// specific token (e.g. wrangler wants "false", not "0") and do not honor
// the DO_NOT_TRACK convention. DO_NOT_TRACK=1 is set as the catch-all
// that covers tools which respect it (Turborepo, Gatsby, Astro, and
// future adopters); everything else needs its own var.
//
// Go's own toolchain telemetry (1.23+) has no working env-var opt-out
// and defaults to local-only mode that never uploads, so egress alone
// already prevents it from phoning home — nothing to set here.
func sandboxEnv() map[string]string {
	// falseVal is the literal these vars require to mean "off"; hoisted
	// so the recurring string is a single source of truth.
	const falseVal = "false"

	return map[string]string{
		"GOPROXY": proxygo.ProxyURL(),

		// Serialize the A and AAAA DNS queries glibc otherwise issues in
		// parallel from a single UDP source port. Under restricted egress
		// every lookup is NAT-redirected to the egress sidecar's resolver;
		// the parallel pair races through conntrack, one query is dropped,
		// and the lookup then stalls for glibc's full 5s timeout. The
		// effect is intermittent but brutal under DNS bursts — npm's and
		// pip's metadata-resolve phases routinely eat multiple 5s stalls.
		// single-request-reopen uses a fresh socket per query (killing the
		// race); the shorter timeout/attempts bound the cost of any
		// residual loss. Honored by every glibc resolver in the sandbox.
		"RES_OPTIONS": "single-request-reopen timeout:2 attempts:3",

		// Cross-tool standard (Turborepo, Gatsby, Astro, ...).
		"DO_NOT_TRACK": "1",

		// Cloudflare Wrangler — does not honor DO_NOT_TRACK.
		"WRANGLER_SEND_METRICS":       falseVal,
		"WRANGLER_SEND_ERROR_REPORTS": falseVal,

		// JS framework / CLI telemetry — none honor DO_NOT_TRACK.
		"NEXT_TELEMETRY_DISABLED":     "1",
		"NUXT_TELEMETRY_DISABLED":     "1",
		"NG_CLI_ANALYTICS":            falseVal,
		"STORYBOOK_DISABLE_TELEMETRY": "1",
		"VERCEL_TELEMETRY_DISABLED":   "1",
		"YARN_ENABLE_TELEMETRY":       "0",

		// npm noise / update-check / postinstall analytics.
		"NO_UPDATE_NOTIFIER":     "1",
		"npm_config_fund":        falseVal,
		"DISABLE_OPENCOLLECTIVE": "true",

		// Python — pip version check + interactive prompts.
		"PIP_DISABLE_PIP_VERSION_CHECK": "1",
		"PIP_NO_INPUT":                  "1",

		// Other common build-toolchain phone-homes.
		"CHECKPOINT_DISABLE": "1",    // Prisma / HashiCorp checkpoint
		"NX_NO_CLOUD":        "true", // Nx Cloud
	}
}

// startGoproxySidecar builds (if needed) and starts a sidecar carrying
// only the Go module proxy — used by the script and persistent-sandbox
// paths so every sandbox can fetch Go modules via 127.0.0.1. Agent runs
// start their own sidecar (with the Anthropic proxy + MCP tunnel) which
// runs the Go proxy too.
func (r *Runner) startGoproxySidecar(ctx context.Context, sandboxID string) error {
	img, err := sidecar.EnsureImage(ctx)
	if err != nil {
		return fmt.Errorf("build sidecar image: %w", err)
	}
	_, err = sidecar.Start(ctx, sandboxID, img, sidecar.ProxyConfig{})
	return err
}

// checkAgentCredentials verifies that the runner config contains the
// credentials required for the agent's proxy vendor. It is extracted from
// prepareAgent to keep that function's cyclomatic complexity below the limit.
func (r *Runner) checkAgentCredentials(agent agents.Agent, tool string) error {
	switch agent.ProxyVendor() {
	case agents.ProxyAnthropic:
		if r.cfg.ClaudeCodeOAuthToken == "" {
			return errors.New(
				"DEMESNE_CLAUDE_CODE_OAUTH_TOKEN is required for " + tool +
					" (run `claude setup-token` to obtain one)",
			)
		}
	case agents.ProxyOpenAI:
		if r.cfg.CodexAuthFile == "" {
			return errors.New(
				"DEMESNE_CODEX_AUTH_FILE (default ~/.codex/auth.json) is required for " +
					tool + " when using a codex model",
			)
		}
	}
	return nil
}

// resolveCodexTokens refreshes and persists the host-side Codex auth file
// for an OpenAI-vendor agent, returning the fresh token set to hand to the
// sidecar. For every other vendor it is a no-op returning a zero TokenSet.
func (r *Runner) resolveCodexTokens(ctx context.Context, agent agents.Agent) (proxyopenai.TokenSet, error) {
	if agent.ProxyVendor() != agents.ProxyOpenAI {
		return proxyopenai.TokenSet{}, nil
	}
	return proxyopenai.RefreshAuthFile(ctx, r.cfg.CodexAuthFile)
}

// resolveAgentModel selects the provider and model for a run. An empty
// model falls back to the credential-aware default provider and that
// provider's default model; a non-empty model fully determines the
// provider, since model aliases are globally unique across providers.
func (r *Runner) resolveAgentModel(rawModel string) (agents.Agent, agents.ModelName, error) {
	if rawModel == "" {
		name, err := resolveDefaultAgent(r.cfg)
		if err != nil {
			return nil, "", err
		}
		agent, err := agents.Lookup(name)
		if err != nil {
			return nil, "", err
		}
		model, err := agent.ResolveModel("")
		if err != nil {
			return nil, "", err
		}
		return agent, model, nil
	}
	agent, err := agents.LookupByModel(rawModel)
	if err != nil {
		return nil, "", err
	}
	if !agentEnabled(r.cfg, agent.Name()) {
		return nil, "", fmt.Errorf("model %q is unavailable because agent provider %q is disabled", rawModel, agent.Name())
	}
	model, err := agent.ResolveModel(rawModel)
	if err != nil {
		return nil, "", err
	}
	return agent, model, nil
}

// prepareAgent validates the request, looks up the provider, resolves
// the model, describes inputs, refreshes+persists the Codex auth file
// for codex runs, and ensures the provider's image is built. No
// sandbox-runtime calls happen here.
func (r *Runner) prepareAgent(ctx context.Context, spec internalAgentSpec) (agentPrep, error) {
	if strings.TrimSpace(spec.prompt) == "" {
		return agentPrep{}, errors.New("prompt is required")
	}
	agent, model, err := r.resolveAgentModel(spec.model)
	if err != nil {
		return agentPrep{}, err
	}
	if err := r.checkAgentCredentials(agent, spec.tool); err != nil {
		return agentPrep{}, err
	}
	codexTokens, err := r.resolveCodexTokens(ctx, agent)
	if err != nil {
		return agentPrep{}, err
	}
	// Non-isolated children inherit the parent's inputs (no
	// Files/Directories of their own); isolated (research) children
	// inherit none; root runs describe their caller-supplied inputs.
	var inputs []agents.InputInfo
	switch {
	case spec.child != nil:
		if !spec.child.isolated {
			inputs = spec.child.parent.inputs
		}
	default:
		inputs, err = r.describeInputs(spec.files, spec.directories)
		if err != nil {
			return agentPrep{}, err
		}
	}
	imgTag, err := agent.EnsureImage(ctx)
	if err != nil {
		return agentPrep{}, fmt.Errorf("build agent image: %w", err)
	}
	return agentPrep{agent: agent, model: model, inputs: inputs, tag: ImageURI(imgTag), codexTokens: codexTokens}, nil
}

// Agent-provider names used by resolveDefaultAgent. Mirrored from the
// vendor packages' own AgentName constants to avoid importing the vendor
// subpackages here (which would couple the runner to specific providers).
const (
	agentNameCodex      = "codex"
	agentNameClaudeCode = "claude-code"
)

// availableAgentNames returns the enabled agent provider names whose host
// credentials are configured, in codex-first order. Codex availability
// is determined by the resolved auth file path existing on disk
// (matching how checkAgentCredentials and resolveCodexTokens use it).
// claude-code availability is the OAuth token field being non-empty.
//
// Order is the canonical codex-first one used everywhere the runner
// exposes the available providers (default-agent resolution,
// AvailableAgents). An empty slice means neither is configured.
func availableAgentNames(cfg Config) []string {
	var names []string
	if cfg.CodexEnabled && cfg.CodexAuthFile != "" {
		if _, err := os.Stat(cfg.CodexAuthFile); err == nil {
			names = append(names, agentNameCodex)
		}
	}
	if cfg.ClaudeCodeEnabled && cfg.ClaudeCodeOAuthToken != "" {
		names = append(names, agentNameClaudeCode)
	}
	return names
}

// resolveDefaultAgent picks the default enabled provider when the caller
// leaves the model empty. Configured credentials win in codex-first order;
// without credentials it falls back in the same order so the subsequent
// error identifies the enabled provider's setup path. Disabling both is an
// immediate configuration error for agent calls.
func resolveDefaultAgent(cfg Config) (string, error) {
	names := availableAgentNames(cfg)
	if len(names) > 0 {
		return names[0], nil
	}
	if cfg.CodexEnabled {
		return agentNameCodex, nil
	}
	if cfg.ClaudeCodeEnabled {
		return agentNameClaudeCode, nil
	}
	return "", errors.New("no agent providers are enabled; enable DEMESNE_CODEX_ENABLED or DEMESNE_CLAUDE_CODE_ENABLED")
}

func agentEnabled(cfg Config, name string) bool {
	switch name {
	case agentNameCodex:
		return cfg.CodexEnabled
	case agentNameClaudeCode:
		return cfg.ClaudeCodeEnabled
	default:
		return true
	}
}

// AgentOption describes one available agent provider and the model
// allowlist that pairs with it. The server uses []AgentOption to
// populate the `model` enum on sandbox_agent /
// sandbox_research at registration time, filtered to the configured
// credentials. Codex-first order is preserved.
type AgentOption struct {
	Name   string
	Models []string
}

// AllowedMountPaths returns the host paths under which callers may
// mount files or directories (or upload from), in the same order
// LoadConfigFromEnv produced — configured paths first, then the
// implicitly-appended output root. Used by the server to populate the
// `files` / `directories` / `src` param descriptions advertised on
// sandbox_script / sandbox_create / sandbox_agent / sandbox_upload.
func (r *Runner) AllowedMountPaths() []string { return r.cfg.AllowedPaths }

// AvailableAgents returns the configured agent providers and their
// model allowlists in codex-first order. Empty when no agent
// credentials are configured. Used by the server to build the
// runtime-filtered `model` enum advertised on
// sandbox_agent / sandbox_research.
func (r *Runner) AvailableAgents() []AgentOption {
	names := availableAgentNames(r.cfg)
	out := make([]AgentOption, 0, len(names))
	for _, name := range names {
		a, err := agents.Lookup(name)
		if err != nil {
			continue
		}
		modelNames := a.Models()
		models := make([]string, len(modelNames))
		for i, m := range modelNames {
			models[i] = string(m)
		}
		out = append(out, AgentOption{Name: name, Models: models})
	}
	return out
}

// buildLayout produces the host paths + mounts for an agent run,
// dispatching to the root or child builder.
func (r *Runner) buildLayout(spec internalAgentSpec) (sandboxLayout, error) {
	if spec.child != nil {
		return r.buildChildLayout(spec.child)
	}
	return r.buildRootLayout(spec.files, spec.directories)
}

// buildRootLayout creates the host dirs for a host-invoked agent run:
// a fresh workspace, /out, config dir, and sidecar-results dir under
// OutputRoot/<jobID>, with caller-supplied inputs resolved into /in.
func (r *Runner) buildRootLayout(files, directories []string) (sandboxLayout, error) {
	inputVolumes, err := r.resolveMounts(files, directories)
	if err != nil {
		return sandboxLayout{}, err
	}
	jobID := JobID(uuid.NewString())
	jobDir := filepath.Join(r.cfg.OutputRoot, string(jobID))
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
// child. A normal child inherits the parent's /in mounts, shared
// /workspace, and /in/previous-jobs; an isolated (research) child gets
// none of those and a fresh private /workspace instead. Both nest /out
// at <parentOut>/child/<name> and keep their own private config +
// sidecar-results dirs under OutputRoot/<jobID>. Reserving the name
// (unique per parent) is a side effect.
func (r *Runner) buildChildLayout(c *childSpawn) (sandboxLayout, error) {
	if err := c.parent.reserveName(c.name); err != nil {
		return sandboxLayout{}, err
	}
	jobID := JobID(uuid.NewString())
	privDir := filepath.Join(r.cfg.OutputRoot, string(jobID))
	l := sandboxLayout{
		jobID:       jobID,
		outHost:     filepath.Join(c.parent.outHost, "child", c.name),
		configDir:   filepath.Join(privDir, "config"),
		resultsHost: filepath.Join(privDir, "sidecar-results"),
		depth:       c.parent.depth + 1,
		childName:   c.name,
	}
	dirs := []string{l.outHost, l.configDir, l.resultsHost}
	if c.isolated {
		// Fresh private workspace; no inherited inputs or previous-jobs.
		l.workspaceHost = filepath.Join(privDir, "workspace")
		dirs = append(dirs, l.workspaceHost)
	} else {
		l.inputVolumes = c.parent.inputVolumes
		l.workspaceHost = c.parent.workspaceHost // shared, already exists
		l.previousJobs = c.parent.priorSiblings()
	}
	if err := mkLayoutDirs(dirs...); err != nil {
		return sandboxLayout{}, err
	}
	return l, nil
}

// mkLayoutDirs creates the given host directories. Paths are composed
// from r.cfg.OutputRoot, a uuid, and constant suffixes.
func mkLayoutDirs(dirs ...string) error {
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o750); err != nil {
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
	policy, err := BuildNetworkPolicy(spec.egress, proxies.EgressHostStrings())
	if err != nil {
		return nil, err
	}
	ctxName := prep.agent.ContextFileName()
	body := prep.agent.GenerateContext(agents.ContextParams{
		Preamble:     spec.preamble,
		Prompt:       spec.prompt,
		Egress:       spec.egress,
		Inputs:       prep.inputs,
		MCPServers:   mcpServers,
		PreviousJobs: previousJobNames(layout.previousJobs),
		OutputContract: agents.OutputContract{
			Path:            spec.outputPath,
			Format:          spec.outputFormat,
			SuccessCriteria: spec.successCriteria,
		},
	})
	contextHost := filepath.Join(layout.configDir, ctxName)
	if err := os.WriteFile(contextHost, []byte(body), 0o600); err != nil {
		return nil, fmt.Errorf("write %s: %w", contextHost, err)
	}
	if err := prep.agent.WriteAgentConfig(layout.configDir, agents.AgentConfig{MCPServers: mcpServers}); err != nil {
		return nil, fmt.Errorf("write agent config: %w", err)
	}

	prevVols := previousJobVolumes(layout.previousJobs)
	agentVols := agentVolumes(layout)
	mounts := make([]opensandbox.Volume, 0, len(layout.inputVolumes)+len(prevVols)+len(agentVols))
	mounts = append(mounts, layout.inputVolumes...)
	mounts = append(mounts, prevVols...)
	mounts = append(mounts, agentVols...)

	return r.launchSandbox(ctx, prep.tag, mounts, policy, oneShotSandboxTTLSeconds, layout.jobID, spec.tool)
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
			MountPath: mountWorkspace,
		},
		{
			Name:      outVolumeName,
			Host:      &opensandbox.Host{Path: l.outHost},
			MountPath: mountOut,
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
// quotes are emitted as the four-byte sequence:
//
//	'\''
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
	sidecarUpstreams []proxymcp.Binding
	agentServers     []agents.MCPServerInfo
}

// buildMCPWiring assigns each exposed server a sidecar loopback port
// (FirstListenPort + index, matching the aggregator's stable
// alphabetical ordering) and produces both the sidecar upstream
// list and the agent-facing server list. Agent-facing URLs are
// sandbox-local loopback. The demesne self-server's binding and any
// file-gen server bindings carry this run's jobID as ParentJobID so
// the tunnel injects the trusted parent-identity header on calls to them.
func (r *Runner) buildMCPWiring(jobID JobID) mcpWiring {
	w := mcpWiring{
		sidecarUpstreams: make([]proxymcp.Binding, 0, len(r.cfg.MCPServers)),
		agentServers:     make([]agents.MCPServerInfo, 0, len(r.cfg.MCPServers)),
	}
	for i, name := range r.cfg.MCPServers {
		port := proxymcp.FirstListenPort + i
		up := proxymcp.Binding{
			Name:       name,
			ListenPort: port,
			Path:       "/" + name + "/mcp",
		}
		if name == mcpproxy.DemesneServerName || mcpproxy.IsFileGenServer(name) {
			up.ParentJobID = string(jobID) // cast at proxies/mcp boundary (Binding.ParentJobID stays string)
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
// runner needs to surface in AgentResult.
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
	data, err := readOutputFile(resultsHost, "usage.json")
	if err != nil {
		return usageSnapshot{}
	}
	var s usageSnapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return usageSnapshot{}
	}
	return s
}

// copyUsageToOut copies usage.json and usage.jsonl from the sidecar
// results dir to the agent's /out so the caller has a single place to
// find them. Best-effort: errors are logged non-fatally because the
// summary in AgentResult already carries the headline numbers.
// usage.jsonl must be present before writeResults reads it for the
// per-model token rollup.
func copyUsageToOut(resultsHost, outHost string) {
	for _, name := range []string{"usage.json", "usage.jsonl"} {
		data, err := readOutputPath(filepath.Join(resultsHost, name))
		if err != nil {
			continue
		}
		if err := writeOutputFile(outHost, name, data); err != nil {
			log.Printf("demesne: copy %s to out: %v", name, err)
		}
	}
}
