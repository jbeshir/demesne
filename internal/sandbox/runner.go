package sandbox

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
	"github.com/google/uuid"

	"github.com/jbeshir/demesne/internal/mcpproxy"
	"github.com/jbeshir/demesne/internal/proxies"
	"github.com/jbeshir/demesne/internal/sidecar"
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
	ToolSandboxScript   = "sandbox_script"
	ToolSandboxAgent    = "sandbox_agent"
	ToolSandboxResearch = "sandbox_research"
	ToolSandboxCreate   = "sandbox_create"
	ToolSandboxExec     = "sandbox_exec"
	ToolSandboxDestroy  = "sandbox_destroy"
	ToolSandboxUpload   = "sandbox_upload"
	ToolSandboxDownload = "sandbox_download"
	mountOut            = "/out"
	mountWorkspace      = "/workspace"
	outVolumeName       = "out"
)

// createSandboxMaxAttempts is the maximum number of times launchSandbox will
// call CreateSandbox before giving up on transient errors.
const createSandboxMaxAttempts = 3

// Both substrings must appear in the error to count as the buildah-copier
// race; "broken pipe" alone over-matches unrelated pipe errors, so requiring
// BOTH the distribution-failed code AND the bulk-input substring keeps the
// match specific to the buildah-copier race.
const (
	createSandboxTransientCode    = "DOCKER::SANDBOX_EXECD_DISTRIBUTION_FAILED"
	createSandboxTransientMessage = "passing bulk input to subprocess"
)

// createSandboxFn is the SDK call site. Tests replace this to inject failures.
var createSandboxFn = opensandbox.CreateSandbox

// createSandboxBackoffs holds the inter-attempt sleeps. Length is
// createSandboxMaxAttempts-1 — after the final attempt we return without sleeping.
// Production: 500ms then 1.5s. Tests shorten or zero this out.
var createSandboxBackoffs = []time.Duration{500 * time.Millisecond, 1500 * time.Millisecond}

// dockerRemoveFn force-removes a leaked partial container by ID between
// retry attempts. Best-effort; failure is logged and ignored. Test seam.
var dockerRemoveFn = dockerForceRemove

// Runner orchestrates sandbox operations: it validates inputs, talks to the
// OpenSandbox lifecycle server via its SDK, and surfaces results to the MCP
// layer. One Runner serves all tools (sandbox_script, sandbox_create,
// sandbox_exec, etc.) — methods live in sibling files in this package.
type Runner struct {
	cfg      Config
	registry *ChildRegistry
}

// NewRunner constructs a Runner against the given configuration.
func NewRunner(cfg Config) *Runner {
	return &Runner{cfg: cfg, registry: newChildRegistry()}
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
		Tool:           ToolSandboxScript,
		TimeoutSeconds: oneShotSandboxTTLSeconds,
		Child:          child,
	})
	if err != nil {
		return ScriptResult{}, err
	}
	defer killSandbox(ctx, sb)

	if err := r.startGoproxySidecar(ctx, sb.ID()); err != nil {
		return ScriptResult{}, fmt.Errorf("start sidecar: %w", err)
	}
	defer func() {
		if err := sidecar.Remove(context.WithoutCancel(ctx), sb.ID()); err != nil {
			log.Printf("sandbox_script: sidecar cleanup failed: %v", err)
		}
	}()

	wrapped := wrapStdoutStderr(req.Command)
	exec, err := sb.RunCommandWithOpts(ctx, opensandbox.RunCommandRequest{
		Command: wrapped,
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
	// The wrapper redirect always creates both files, so a read error is
	// exceptional; log it rather than silently surfacing empty output (the
	// command itself may have succeeded, so don't fail the run).
	stdoutBytes, err := readOutputFile(outputHost, "stdout.log")
	if err != nil {
		log.Printf("sandbox_script: read stdout.log: %v", err)
	}
	stderrBytes, err := readOutputFile(outputHost, "stderr.log")
	if err != nil {
		log.Printf("sandbox_script: read stderr.log: %v", err)
	}
	return ScriptResult{
		JobID:      jobID,
		OutputPath: outputHost,
		Stdout:     string(stdoutBytes),
		ExitCode:   exitCode,
		Stderr:     tailStderr(stderrBytes),
	}, nil
}

// connectionConfigFor builds an OpenSandbox connection config from a Config.
// Extracted from Runner.connectionConfig so the startup reaper can build a
// connection without needing a Runner instance.
func connectionConfigFor(cfg Config) opensandbox.ConnectionConfig {
	return opensandbox.ConnectionConfig{
		Domain:   cfg.OpenSandboxDomain,
		Protocol: cfg.OpenSandboxProtocol,
		APIKey:   cfg.OpenSandboxAPIKey,
		// The SDK default is 30s, which kills long-running RunCommand SSE
		// reads (agent tasks and data jobs both). Match commandTimeout so
		// the transport never expires before the in-sandbox command does.
		RequestTimeout: commandTimeout,
	}
}

// connectionConfig packages the Runner's OpenSandbox connection settings
// into the SDK's shape. Used by every entry point that talks to OpenSandbox.
func (r *Runner) connectionConfig() opensandbox.ConnectionConfig {
	return connectionConfigFor(r.cfg)
}

// attach re-binds to an existing sandbox by ID. Errors are wrapped with the
// sandbox ID so the caller can tell which handle failed.
func (r *Runner) attach(ctx context.Context, sandboxID SandboxID) (*opensandbox.Sandbox, error) {
	sb, err := opensandbox.ConnectSandbox(ctx, r.connectionConfig(), string(sandboxID))
	if err != nil {
		return nil, fmt.Errorf("attach to sandbox %s: %w", sandboxID, err)
	}
	return sb, nil
}

// launchSandbox calls CreateSandbox with demesne's standard envelope:
// metadata labels, GOPROXY env, and the given image/mounts/policy. It retries
// up to createSandboxMaxAttempts times on the known buildah-copier broken-pipe
// race, cleaning up any leaked partial container between attempts. Both
// prepareSandbox and createSandbox delegate here so there is exactly one call
// site for sandbox creation.
func (r *Runner) launchSandbox(
	ctx context.Context,
	image ImageURI,
	mounts []opensandbox.Volume,
	policy *opensandbox.NetworkPolicy,
	timeoutSec int,
	jobID JobID,
	tool string,
) (*opensandbox.Sandbox, error) {
	conn := r.connectionConfig()
	meta := map[string]string{
		metadataDemesneJob:  string(jobID),
		metadataDemesneTool: tool,
	}
	if r.cfg.Owner != "" {
		meta[metadataDemesneOwner] = r.cfg.Owner
	}
	opts := opensandbox.SandboxCreateOptions{
		Image:          string(image),
		Volumes:        mounts,
		NetworkPolicy:  policy,
		TimeoutSeconds: &timeoutSec,
		Env:            sandboxEnv(),
		Metadata:       meta,
	}
	for attempt := range createSandboxMaxAttempts {
		sb, err := createSandboxFn(ctx, conn, opts)
		if err == nil {
			return sb, nil
		}
		if !isCreateSandboxTransient(err) {
			return nil, fmt.Errorf("create sandbox: %w", err)
		}
		// Clean up the partial containers this attempt leaked before retrying
		// or giving up. This must run on the final attempt too: a sandbox that
		// fails every attempt otherwise leaks its egress sidecar (and anything
		// sharing its namespace), which accumulates as pipe-page pressure that
		// provokes further create failures.
		if id := containerIDFromError(err); id != "" {
			if rmErr := dockerRemoveFn(ctx, id); rmErr != nil {
				log.Printf("launchSandbox: cleanup of partial container %s after attempt %d failed: %v",
					id, attempt+1, rmErr)
			}
		}
		if attempt == createSandboxMaxAttempts-1 {
			return nil, fmt.Errorf("create sandbox after %d attempts: %w", createSandboxMaxAttempts, err)
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("create sandbox: %w", ctx.Err())
		case <-time.After(createSandboxBackoffs[attempt]):
		}
	}
	// Unreachable: loop always returns inside.
	return nil, fmt.Errorf("create sandbox: unexpected loop exit")
}

// sandboxPrepOptions captures everything prepareSandbox needs. Used by
// the script and create paths; the agent path calls createSandbox directly.
type sandboxPrepOptions struct {
	Image       string     // resolved via ResolveImage
	Egress      EgressMode // resolved via BuildNetworkPolicy
	Files       []string
	Directories []string
	Tool        string // value of the demesne.tool metadata label
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
) (*opensandbox.Sandbox, string, JobID, error) {
	imageURI, err := ResolveImage(opts.Image)
	if err != nil {
		return nil, "", "", err
	}

	policy, err := BuildNetworkPolicy(opts.Egress, proxies.EgressHostStrings())
	if err != nil {
		return nil, "", "", err
	}

	jobID := JobID(uuid.NewString())
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
	sb, err := r.launchSandbox(ctx, imageURI, mounts, policy, opts.TimeoutSeconds, jobID, opts.Tool)
	if err != nil {
		return nil, "", "", err
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
	jobID JobID,
	files, directories []string,
) ([]opensandbox.Volume, string, error) {
	mounts, err := r.resolveMounts(files, directories)
	if err != nil {
		return nil, "", err
	}
	outputHost := filepath.Join(r.cfg.OutputRoot, string(jobID))
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
	prevVols := previousJobVolumes(prior)
	mounts := make([]opensandbox.Volume, 0, len(c.parent.inputVolumes)+len(prevVols)+2)
	mounts = append(mounts, c.parent.inputVolumes...)
	mounts = append(mounts, prevVols...)
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

// isCreateSandboxTransient reports whether err is the known buildah-copier
// broken-pipe race (buildah#6573, fixed in v1.44.0 — not yet vendored into
// any podman release). BOTH substrings must appear: the DOCKER code AND the
// "passing bulk input to subprocess" message body. Matching on "broken pipe"
// alone would over-match unrelated pipe errors.
func isCreateSandboxTransient(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, createSandboxTransientCode) &&
		strings.Contains(msg, createSandboxTransientMessage)
}

// containerIDFromError extracts the docker container ID from the URL that
// OpenSandbox includes in the put_archive failure message:
//
//	500 Server Error for http+docker://localhost/v1.41/containers/<id>/archive?path=%2F
//
// Returns "" if no match. The ID is the hex container ID; the regex accepts
// lowercase hex of length 8..64 to cover short and full IDs.
func containerIDFromError(err error) string {
	if err == nil {
		return ""
	}
	m := containerIDRegexp.FindStringSubmatch(err.Error())
	if m == nil {
		return ""
	}
	return m[1]
}

var containerIDRegexp = regexp.MustCompile(`/containers/([0-9a-f]{8,64})/archive`)

// dockerForceRemove is the default dockerRemoveFn: it removes the partial
// container a failed create left behind, together with the OpenSandbox egress
// sidecar it shares a network namespace with and anything else joined to that
// namespace (a demesne sidecar, say). A create that dies at the
// execd-distribution step leaves both the sandbox container and its egress
// sidecar running, but the failure names only the sandbox container; removing
// just that leaks the egress, which then pins its dependents and accumulates
// as pipe-page pressure that provokes further create failures.
//
// The egress sidecar is the network anchor — the sandbox container's network
// mode is "container:<egressID>" — so we resolve it and remove it with
// --depend, which cascades to the sandbox and any sidecar. The named container
// is then removed directly as a safety net for the case where the egress was
// already gone. Best-effort and idempotent: a missing container is success.
func dockerForceRemove(ctx context.Context, containerID string) error {
	var errs []error
	if egressID := egressAnchorID(ctx, containerID); egressID != "" {
		if err := dockerRemoveDepend(ctx, egressID); err != nil {
			errs = append(errs, err)
		}
	}
	if err := dockerRemoveDepend(ctx, containerID); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// egressAnchorID returns the container ID of the OpenSandbox egress sidecar
// the given container shares a network namespace with, or "" if it can't be
// determined (the container is already gone, or isn't joined to another
// container's namespace). OpenSandbox runs the sandbox in the egress
// sidecar's namespace via a "container:<egressID>" network mode.
func egressAnchorID(ctx context.Context, containerID string) string {
	cmd := dockerCommand(ctx, "inspect", "-f", "{{.HostConfig.NetworkMode}}", containerID)
	// Output (stdout only): the podman docker-compat banner goes to stderr and
	// would otherwise corrupt the parsed network mode.
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return egressIDFromNetworkMode(strings.TrimSpace(string(out)))
}

// egressIDFromNetworkMode extracts the anchor container ID from a docker
// network mode of the form "container:<id>", or "" for any other mode.
func egressIDFromNetworkMode(mode string) string {
	id, ok := strings.CutPrefix(mode, "container:")
	if !ok {
		return ""
	}
	return strings.TrimSpace(id)
}

// dockerRemoveDepend force-removes a container and every container that
// depends on it (--depend), treating an already-absent container as success.
func dockerRemoveDepend(ctx context.Context, target string) error {
	cmd := dockerCommand(ctx, "rm", "-f", "--depend", target)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(string(out)), "no such container") {
		return nil
	}
	return fmt.Errorf("docker rm -f --depend %s: %w\n%s", target, err, out)
}

// dockerCommand wraps exec.CommandContext("docker", ...) so the gosec
// G204 suppression for sandbox-package docker calls lives here. argv
// only; callers pass a constant subcommand plus container IDs that are
// hex-validated (containerIDFromError) or otherwise OpenSandbox-generated.
func dockerCommand(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "docker", args...) //nolint:gosec // argv-only; subcommand const, args hex-validated
}
