package sandbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
	"github.com/google/uuid"
	"github.com/jbeshir/demesne/internal/agents"
	"github.com/jbeshir/demesne/internal/proxies"
	"github.com/jbeshir/demesne/internal/sidecar"
)

// agentPaths captures the host paths created for an agent run.
type agentPaths struct {
	jobID         string
	outHost       string
	workspaceHost string
	claudemdHost  string
}

// agentPrep collects everything Agent resolves before touching the
// sandbox runtime: provider, model, inputs, image tag.
type agentPrep struct {
	agent  agents.Agent
	model  string
	inputs []agents.InputInfo
	tag    string
}

// Agent runs an agent (e.g. claude-code) inside a fresh sandbox against
// the caller's prompt.
//
// Sandbox layout (cwd = /workspace):
//   - /in/<basename>     read-only caller inputs (files + dirs)
//   - /in/CLAUDE.md      read-only generated context file
//   - /workspace         writable scratch — agent copies inputs here to mutate
//   - /out               writable, output only — agent writes final artefacts here
//
// All three writable mounts are persisted on the host under
// cfg.OutputRoot/<jobID>/{out, workspace, claudemd/CLAUDE.md}. The
// context file is symlinked from /workspace/CLAUDE.md so the CLI finds
// it via the usual cwd lookup.
func (r *Runner) Agent(ctx context.Context, req AgentRequest) (AgentResult, error) {
	prep, err := r.prepareAgent(ctx, req)
	if err != nil {
		return AgentResult{}, err
	}

	sidecarImage, err := sidecar.EnsureImage(ctx)
	if err != nil {
		return AgentResult{}, fmt.Errorf("build sidecar image: %w", err)
	}

	// Per-sandbox fake credential. The agent never sees the real
	// Anthropic OAuth token; the proxy validates this value on every
	// inbound request and substitutes the real token before forwarding.
	agentToken, err := generateAgentToken()
	if err != nil {
		return AgentResult{}, fmt.Errorf("generate agent token: %w", err)
	}

	sb, paths, err := r.createAgentSandbox(ctx, req, prep)
	if err != nil {
		return AgentResult{}, err
	}
	defer killSandbox(ctx, sb)

	side, err := sidecar.Start(ctx, sb.ID(), sidecarImage, sidecar.ProxyTokens{
		AgentToken:    agentToken,
		UpstreamToken: r.cfg.ClaudeCodeOAuthToken,
	})
	if err != nil {
		return AgentResult{}, fmt.Errorf("start sidecar: %w", err)
	}
	defer func() {
		if err := side.Stop(context.WithoutCancel(ctx)); err != nil {
			log.Printf("sandbox_agent: sidecar cleanup failed: %v", err)
		}
	}()

	setup := fmt.Sprintf("ln -sf /in/%s ./%s",
		prep.agent.ContextFileName(), prep.agent.ContextFileName())
	command := setup + " && " + shellQuote(prep.agent.Command(req.Prompt, prep.model))

	exec, err := sb.RunCommandWithOpts(ctx, opensandbox.RunCommandRequest{
		Command: command,
		Cwd:     "/workspace",
		Envs:    prep.agent.EnvVars(agentToken, prep.model),
		Timeout: commandTimeout.Milliseconds(),
	}, nil)
	if err != nil {
		return AgentResult{}, fmt.Errorf("run agent: %w", err)
	}

	exitCode := 0
	if exec.ExitCode != nil {
		exitCode = *exec.ExitCode
	}
	return AgentResult{
		JobID:         paths.jobID,
		OutputPath:    paths.outHost,
		WorkspacePath: paths.workspaceHost,
		Stdout:        exec.Text(),
		ExitCode:      exitCode,
	}, nil
}

// prepareAgent validates the request, looks up the provider, resolves
// the model, describes inputs, and ensures the provider's image is
// built. No sandbox-runtime calls happen here.
func (r *Runner) prepareAgent(ctx context.Context, req AgentRequest) (agentPrep, error) {
	if r.cfg.ClaudeCodeOAuthToken == "" {
		return agentPrep{}, errors.New(
			"DEMESNE_CLAUDE_CODE_OAUTH_TOKEN is required for sandbox_agent " +
				"(run `claude setup-token` to obtain one)",
		)
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return agentPrep{}, errors.New("prompt is required")
	}
	agent, err := agents.Lookup(req.Agent)
	if err != nil {
		return agentPrep{}, err
	}
	model, err := agent.ResolveModel(req.Model)
	if err != nil {
		return agentPrep{}, err
	}
	inputs, err := r.describeInputs(req.Files, req.Directories)
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
	req AgentRequest,
	prep agentPrep,
) (*opensandbox.Sandbox, agentPaths, error) {
	policy, err := BuildNetworkPolicy(req.Egress, proxies.EgressHosts())
	if err != nil {
		return nil, agentPaths{}, err
	}
	mounts, err := r.resolveMounts(req.Files, req.Directories)
	if err != nil {
		return nil, agentPaths{}, err
	}
	paths, err := r.createAgentPaths(prep.agent.ContextFileName())
	if err != nil {
		return nil, agentPaths{}, err
	}
	body := prep.agent.GenerateContext(req.Preamble, req.Prompt, prep.inputs)
	if err := os.WriteFile(paths.claudemdHost, []byte(body), 0o600); err != nil {
		return nil, agentPaths{}, fmt.Errorf("write %s: %w", paths.claudemdHost, err)
	}

	mounts = append(mounts, agentVolumes(paths, prep.agent.ContextFileName())...)

	sb, err := opensandbox.CreateSandbox(ctx, r.connectionConfig(), opensandbox.SandboxCreateOptions{
		Image:         prep.tag,
		Volumes:       mounts,
		NetworkPolicy: policy,
		Metadata: map[string]string{
			metadataDemesneJob:  paths.jobID,
			metadataDemesneTool: "sandbox_agent",
		},
	})
	if err != nil {
		return nil, agentPaths{}, fmt.Errorf("create sandbox: %w", err)
	}
	return sb, paths, nil
}

// createAgentPaths mints the per-job UUID and creates the three
// writable host directories used by the agent layout.
func (r *Runner) createAgentPaths(contextFileName string) (agentPaths, error) {
	jobID := uuid.NewString()
	jobDir := filepath.Join(r.cfg.OutputRoot, jobID)
	p := agentPaths{
		jobID:         jobID,
		outHost:       filepath.Join(jobDir, "out"),
		workspaceHost: filepath.Join(jobDir, "workspace"),
		claudemdHost:  filepath.Join(jobDir, "claudemd", contextFileName),
	}
	for _, d := range []string{p.outHost, p.workspaceHost, filepath.Dir(p.claudemdHost)} {
		// d is composed from r.cfg.OutputRoot, a uuid, and a constant suffix.
		// gosec G703 fires under -tags=integration; default lint is clean.
		if err := os.MkdirAll(d, 0o750); err != nil { //nolint:gosec,nolintlint
			return agentPaths{}, fmt.Errorf("create %s: %w", d, err)
		}
	}
	return p, nil
}

// agentVolumes is the set of writable + read-only volumes specific to
// an agent run — the context file, /workspace, and /out.
func agentVolumes(p agentPaths, contextFileName string) []opensandbox.Volume {
	return []opensandbox.Volume{
		{
			Name:      "claudemd",
			Host:      &opensandbox.Host{Path: p.claudemdHost},
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
// agent uses it as CLAUDE_CODE_OAUTH_TOKEN; the proxy validates it on
// every inbound request and substitutes the real OAuth token before
// forwarding. The "demesne-agent-" prefix makes the value obvious in
// logs as a fake credential.
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
