package sandbox

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
	"github.com/google/uuid"

	"github.com/jbeshir/demesne/internal/mcpproxy"
	"github.com/jbeshir/demesne/internal/proxies"
)

// commandTimeout caps how long a single sandbox command may run. Set
// generously: long-running data-processing scripts are a legitimate use
// case. The caller can still cancel via the request context.
const commandTimeout = 12 * time.Hour

// oneShotSandboxTTLSeconds is the OpenSandbox-side TTL for sandboxes
// created by RunScript / Agent / Research — paths that always
// defer-Kill on return. Match commandTimeout so the server-side TTL
// can't undercut the command timeout (the OpenSandbox SDK default of
// 600s would).
const oneShotSandboxTTLSeconds = int(12 * 60 * 60)

// persistentSandboxTTLSeconds is the initial TTL for sandbox_create
// sandboxes. Matches the documented 24h, after which sandbox_exec's
// Renew(24h) refreshes the window on every call.
const persistentSandboxTTLSeconds = int(24 * 60 * 60)

const (
	metadataDemesneJob  = "demesne.job"
	metadataDemesneTool = "demesne.tool"
)

const (
	toolSandboxScript   = "sandbox_script"
	toolSandboxAgent    = "sandbox_agent"
	toolSandboxResearch = "sandbox_research"
	toolSandboxCreate   = "sandbox_create"
	mountOut            = "/out"
	mountWorkspace      = "/workspace"
	outVolumeName       = "out"
)

// Runner orchestrates sandbox operations: it validates inputs, talks to the
// OpenSandbox lifecycle server via its SDK, and surfaces results to the MCP
// layer. One Runner serves all tools (sandbox_script, sandbox_create,
// sandbox_exec, etc.) — methods live in sibling files in this package.
type Runner struct {
	cfg Config

	// children maps a running agent sandbox's jobID to the context its
	// in-sandbox demesne tools spawn children from (inherited inputs,
	// shared workspace, output dir, depth). Populated for every agent
	// run; the demesne MCP server looks an entry up by the trusted
	// parent-identity header. See childserver.go / children.go.
	childMu  sync.Mutex
	children map[string]*childContext
}

// NewRunner constructs a Runner against the given configuration.
func NewRunner(cfg Config) *Runner {
	return &Runner{cfg: cfg, children: map[string]*childContext{}}
}

// SetMCPWiring records the host MCP aggregator's exposed servers,
// socket path, and tool catalogue. Called by main after the
// aggregator starts (the runner must exist first, because the
// aggregator mounts the runner's own demesne server). Not env-derived.
func (r *Runner) SetMCPWiring(servers []string, socketPath string, catalogue mcpproxy.ToolCatalogue) {
	r.cfg.MCPServers = servers
	r.cfg.MCPSocketPath = socketPath
	r.cfg.MCPToolCatalogue = catalogue
}

// RunScript executes one sandbox_script invocation end-to-end: create a
// fresh sandbox, run a single command, return stdout, tear the sandbox down.
func (r *Runner) RunScript(ctx context.Context, req ScriptRequest) (ScriptResult, error) {
	return r.runScript(ctx, req, nil)
}

// runScript backs RunScript and the in-sandbox child sandbox_script.
// When child is set, the sandbox inherits the parent's /in and
// /workspace and writes to /out/child/<name>.
func (r *Runner) runScript(ctx context.Context, req ScriptRequest, child *childSpawn) (ScriptResult, error) {
	sb, outputHost, jobID, err := r.prepareSandbox(ctx, sandboxPrepOptions{
		Image:          req.Image,
		Egress:         req.Egress,
		Files:          req.Files,
		Directories:    req.Directories,
		Tool:           toolSandboxScript,
		TimeoutSeconds: oneShotSandboxTTLSeconds,
		Child:          child,
	})
	if err != nil {
		return ScriptResult{}, err
	}
	defer killSandbox(ctx, sb)

	side, err := r.startGoproxySidecar(ctx, sb.ID())
	if err != nil {
		return ScriptResult{}, fmt.Errorf("start sidecar: %w", err)
	}
	defer func() {
		if err := side.Stop(context.WithoutCancel(ctx)); err != nil {
			log.Printf("sandbox_script: sidecar cleanup failed: %v", err)
		}
	}()

	exec, err := sb.RunCommandWithOpts(ctx, opensandbox.RunCommandRequest{
		Command: req.Command,
		Cwd:     mountOut,
		Timeout: commandTimeout.Milliseconds(),
	}, nil)
	if err != nil {
		return ScriptResult{}, fmt.Errorf("run command: %w", err)
	}

	exitCode := 0
	if exec.ExitCode != nil {
		exitCode = *exec.ExitCode
	}
	return ScriptResult{
		JobID:      jobID,
		OutputPath: outputHost,
		Stdout:     exec.Text(),
		ExitCode:   exitCode,
	}, nil
}

// connectionConfig packages the Runner's OpenSandbox connection settings
// into the SDK's shape. Used by every entry point that talks to OpenSandbox.
func (r *Runner) connectionConfig() opensandbox.ConnectionConfig {
	return opensandbox.ConnectionConfig{
		Domain:   r.cfg.OpenSandboxDomain,
		Protocol: r.cfg.OpenSandboxProtocol,
		APIKey:   r.cfg.OpenSandboxAPIKey,
		// The SDK default is 30s, which kills long-running RunCommand SSE
		// reads (agent tasks and data jobs both). Match commandTimeout so
		// the transport never expires before the in-sandbox command does.
		RequestTimeout: commandTimeout,
	}
}

// attach re-binds to an existing sandbox by ID. Errors are wrapped with the
// sandbox ID so the caller can tell which handle failed.
func (r *Runner) attach(ctx context.Context, sandboxID string) (*opensandbox.Sandbox, error) {
	sb, err := opensandbox.ConnectSandbox(ctx, r.connectionConfig(), sandboxID)
	if err != nil {
		return nil, fmt.Errorf("attach to sandbox %s: %w", sandboxID, err)
	}
	return sb, nil
}

// sandboxPrepOptions captures everything prepareSandbox needs. The script
// and create paths use Image/Egress from the whitelist; the agent path
// supplies ImageOverride (a built image tag) and ExtraEgressAllow (the
// proxy host that must be reachable).
type sandboxPrepOptions struct {
	Image            string     // resolved via ResolveImage; ignored if ImageOverride is set
	ImageOverride    string     // a built image tag, used verbatim when non-empty
	Egress           EgressMode // resolved via BuildNetworkPolicy
	ExtraEgressAllow []string   // additional allow targets unioned with the egress mode
	Files            []string
	Directories      []string
	Tool             string // value of the demesne.tool metadata label
	// TimeoutSeconds is the sandbox's OpenSandbox TTL. The SDK default
	// is 600s which is too short for long-running agent or research
	// runs; callers must set this explicitly.
	TimeoutSeconds int
	// Child, when set, makes this a child sandbox: it inherits the
	// parent's /in mounts and shared /workspace, and its /out is
	// <parentOut>/child/<name>. Files/Directories are ignored.
	Child *childSpawn
}

// prepareSandbox validates inputs, mints the per-job UUID + host /out dir,
// and creates the sandbox via the OpenSandbox SDK. Shared by RunScript,
// Create, and Agent. The caller decides whether to defer Kill (RunScript
// and Agent do; Create does not — Create returns the sandbox for the
// caller to manage).
//
// Returns the SDK sandbox handle, the host /out path, and the demesne
// jobID (stored in sandbox metadata as demesne.job so it can be recovered
// from sb.GetInfo later — see Download).
func (r *Runner) prepareSandbox(
	ctx context.Context,
	opts sandboxPrepOptions,
) (*opensandbox.Sandbox, string, string, error) {
	imageURI := opts.ImageOverride
	if imageURI == "" {
		resolved, err := ResolveImage(opts.Image)
		if err != nil {
			return nil, "", "", err
		}
		imageURI = resolved
	}

	// Every sandbox gets the registered proxies' egress hosts (e.g. the
	// Go checksum DB) on top of any caller-supplied extras, matching the
	// agent path. Go module downloads still go through the sidecar proxy
	// (SO_MARK), not this allowlist.
	extraAllow := append(append([]string{}, opts.ExtraEgressAllow...), proxies.EgressHosts()...)
	policy, err := BuildNetworkPolicy(opts.Egress, extraAllow)
	if err != nil {
		return nil, "", "", err
	}

	jobID := uuid.NewString()
	var mounts []opensandbox.Volume
	var outputHost string
	if opts.Child != nil {
		mounts, outputHost, err = r.childMounts(opts.Child)
	} else {
		mounts, outputHost, err = r.rootMounts(jobID, opts.Files, opts.Directories)
	}
	if err != nil {
		return nil, "", "", err
	}

	if opts.TimeoutSeconds <= 0 {
		return nil, "", "", fmt.Errorf("sandboxPrepOptions: TimeoutSeconds must be set for %s", opts.Tool)
	}
	timeoutSec := opts.TimeoutSeconds
	sb, err := opensandbox.CreateSandbox(ctx, r.connectionConfig(), opensandbox.SandboxCreateOptions{
		Image:          imageURI,
		Volumes:        mounts,
		NetworkPolicy:  policy,
		TimeoutSeconds: &timeoutSec,
		Env:            sandboxEnv(),
		Metadata: map[string]string{
			metadataDemesneJob:  jobID,
			metadataDemesneTool: opts.Tool,
		},
	})
	if err != nil {
		return nil, "", "", fmt.Errorf("create sandbox: %w", err)
	}
	// Record this child as a sibling only after a successful create, so a
	// failed spawn never poisons later siblings' /in/previous-jobs mounts.
	if opts.Child != nil {
		opts.Child.parent.recordSibling(opts.Child.name, outputHost)
	}
	return sb, outputHost, jobID, nil
}

// rootMounts builds the volume set for a host-invoked sandbox:
// caller-supplied /in mounts plus a fresh /out under OutputRoot/jobID.
func (r *Runner) rootMounts(
	jobID string,
	files, directories []string,
) ([]opensandbox.Volume, string, error) {
	mounts, err := r.resolveMounts(files, directories)
	if err != nil {
		return nil, "", err
	}
	outputHost := filepath.Join(r.cfg.OutputRoot, jobID)
	if err := os.MkdirAll(outputHost, 0o750); err != nil {
		return nil, "", fmt.Errorf("create output dir: %w", err)
	}
	mounts = append(mounts, opensandbox.Volume{
		Name:      outVolumeName,
		Host:      &opensandbox.Host{Path: outputHost},
		MountPath: mountOut,
	})
	return mounts, outputHost, nil
}

// childMounts builds the volume set for an in-sandbox-spawned child:
// the parent's inherited /in mounts, the shared /workspace, and a
// /out at <parentOut>/child/<name>. Reserves the name (unique per
// parent) as a side effect.
func (r *Runner) childMounts(c *childSpawn) ([]opensandbox.Volume, string, error) {
	if err := c.parent.reserveName(c.name); err != nil {
		return nil, "", err
	}
	prior := c.parent.priorSiblings()
	outputHost := filepath.Join(c.parent.outHost, "child", c.name)
	if err := os.MkdirAll(outputHost, 0o750); err != nil {
		return nil, "", fmt.Errorf("create child output dir: %w", err)
	}
	mounts := append([]opensandbox.Volume{}, c.parent.inputVolumes...)
	mounts = append(mounts, previousJobVolumes(prior)...)
	mounts = append(mounts,
		opensandbox.Volume{
			Name:      "workspace",
			Host:      &opensandbox.Host{Path: c.parent.workspaceHost},
			MountPath: mountWorkspace,
		},
		opensandbox.Volume{
			Name:      outVolumeName,
			Host:      &opensandbox.Host{Path: outputHost},
			MountPath: mountOut,
		},
	)
	return mounts, outputHost, nil
}

// killSandbox tears the sandbox down even when the request context has
// already been cancelled. The caller passes ctx so values/deadlines
// can be preserved; WithoutCancel keeps them while dropping the
// cancellation that would otherwise short-circuit Kill.
func killSandbox(ctx context.Context, sb *opensandbox.Sandbox) {
	killCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if err := sb.Kill(killCtx); err != nil {
		log.Printf("sandbox: kill failed for %s: %v", sb.ID(), err)
	}
}

// resolveMounts validates each requested input path and turns the result
// into the OpenSandbox volume specification, mounted read-only under /in.
// Names collide if two inputs share a basename; the caller gets a clear error.
func (r *Runner) resolveMounts(files, directories []string) ([]opensandbox.Volume, error) {
	volumes := make([]opensandbox.Volume, 0, len(files)+len(directories))
	used := map[string]string{}
	add := func(host string, isDir bool) error {
		resolved, err := ValidateMountPath(host, r.cfg.AllowedPaths)
		if err != nil {
			return err
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return fmt.Errorf("stat %s: %w", host, err)
		}
		if isDir && !info.IsDir() {
			return fmt.Errorf("%s is not a directory", host)
		}
		if !isDir && !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", host)
		}
		base := filepath.Base(resolved)
		if existing, ok := used[base]; ok {
			return fmt.Errorf("mount basename %q would collide: %s and %s", base, existing, resolved)
		}
		used[base] = resolved
		volumes = append(volumes, opensandbox.Volume{
			Name:      "in-" + fmt.Sprint(len(volumes)),
			Host:      &opensandbox.Host{Path: resolved},
			MountPath: "/in/" + base,
			ReadOnly:  true,
		})
		return nil
	}
	for _, f := range files {
		if err := add(f, false); err != nil {
			return nil, err
		}
	}
	for _, d := range directories {
		if err := add(d, true); err != nil {
			return nil, err
		}
	}
	return volumes, nil
}
