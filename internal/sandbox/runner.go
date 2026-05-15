package sandbox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
	"github.com/google/uuid"
)

// commandTimeout caps how long a single sandbox command may run. Set
// generously: long-running data-processing scripts are a legitimate use
// case. The caller can still cancel via the request context.
const commandTimeout = 12 * time.Hour

const (
	metadataDemesneJob  = "demesne.job"
	metadataDemesneTool = "demesne.tool"
)

// Runner orchestrates sandbox operations: it validates inputs, talks to the
// OpenSandbox lifecycle server via its SDK, and surfaces results to the MCP
// layer. One Runner serves all tools (sandbox_script, sandbox_create,
// sandbox_exec, etc.) — methods live in sibling files in this package.
type Runner struct {
	cfg Config
}

// NewRunner constructs a Runner against the given configuration.
func NewRunner(cfg Config) *Runner {
	return &Runner{cfg: cfg}
}

// RunScript executes one sandbox_script invocation end-to-end: create a
// fresh sandbox, run a single command, return stdout, tear the sandbox down.
func (r *Runner) RunScript(ctx context.Context, req ScriptRequest) (ScriptResult, error) {
	sb, outputHost, jobID, err := r.prepareSandbox(
		ctx, req.Image, req.Egress, req.Files, req.Directories, "sandbox_script",
	)
	if err != nil {
		return ScriptResult{}, err
	}
	defer killSandbox(sb)

	exec, err := sb.RunCommandWithOpts(ctx, opensandbox.RunCommandRequest{
		Command: req.Command,
		Cwd:     "/out",
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

// prepareSandbox validates inputs, mints the per-job UUID + host /out dir,
// and creates the sandbox via the OpenSandbox SDK. Shared by RunScript and
// Create. The caller decides whether to defer Kill (RunScript does; Create
// does not — Create returns the sandbox for the caller to manage).
//
// Returns the SDK sandbox handle, the host /out path, and the demesne
// jobID (stored in sandbox metadata as demesne.job so it can be recovered
// from sb.GetInfo later — see Download).
func (r *Runner) prepareSandbox(
	ctx context.Context,
	image string,
	egress EgressMode,
	files, directories []string,
	tool string,
) (*opensandbox.Sandbox, string, string, error) {
	imageURI, err := ResolveImage(image)
	if err != nil {
		return nil, "", "", err
	}
	policy, err := BuildNetworkPolicy(egress)
	if err != nil {
		return nil, "", "", err
	}
	mounts, err := r.resolveMounts(files, directories)
	if err != nil {
		return nil, "", "", err
	}

	jobID := uuid.NewString()
	outputHost := filepath.Join(r.cfg.OutputRoot, jobID)
	if err := os.MkdirAll(outputHost, 0o750); err != nil {
		return nil, "", "", fmt.Errorf("create output dir: %w", err)
	}
	mounts = append(mounts, opensandbox.Volume{
		Name:      "out",
		Host:      &opensandbox.Host{Path: outputHost},
		MountPath: "/out",
	})

	sb, err := opensandbox.CreateSandbox(ctx, r.connectionConfig(), opensandbox.SandboxCreateOptions{
		Image:         imageURI,
		Volumes:       mounts,
		NetworkPolicy: policy,
		Metadata: map[string]string{
			metadataDemesneJob:  jobID,
			metadataDemesneTool: tool,
		},
	})
	if err != nil {
		return nil, "", "", fmt.Errorf("create sandbox: %w", err)
	}
	return sb, outputHost, jobID, nil
}

// killSandbox tears the sandbox down with a fresh context, so cleanup runs
// even when the request context has been cancelled.
func killSandbox(sb *opensandbox.Sandbox) {
	killCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = sb.Kill(killCtx)
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
