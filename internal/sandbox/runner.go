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

// scriptTimeout caps how long a single sandbox_script invocation may run.
const scriptTimeout = 30 * time.Minute

// Runner orchestrates a single sandbox_script invocation: validates inputs,
// creates an OpenSandbox sandbox with the requested mounts and egress
// policy, runs the command, collects stdout, then tears the sandbox down.
type Runner struct {
	cfg Config
}

// NewRunner constructs a Runner against the given configuration.
func NewRunner(cfg Config) *Runner {
	return &Runner{cfg: cfg}
}

// RunScript executes one sandbox_script invocation end-to-end.
func (r *Runner) RunScript(ctx context.Context, req ScriptRequest) (ScriptResult, error) {
	imageURI, err := ResolveImage(req.Image)
	if err != nil {
		return ScriptResult{}, err
	}
	policy, err := BuildNetworkPolicy(req.Egress)
	if err != nil {
		return ScriptResult{}, err
	}

	mounts, err := r.resolveMounts(req.Files, req.Directories)
	if err != nil {
		return ScriptResult{}, err
	}

	jobID := uuid.NewString()
	outputHost := filepath.Join(r.cfg.OutputRoot, jobID)
	if err := os.MkdirAll(outputHost, 0o750); err != nil {
		return ScriptResult{}, fmt.Errorf("create output dir: %w", err)
	}

	mounts = append(mounts, opensandbox.Volume{
		Name:      "out",
		Host:      &opensandbox.Host{Path: outputHost},
		MountPath: "/out",
	})

	connConfig := opensandbox.ConnectionConfig{
		Domain:   r.cfg.OpenSandboxDomain,
		Protocol: r.cfg.OpenSandboxProtocol,
		APIKey:   r.cfg.OpenSandboxAPIKey,
	}
	createOpts := opensandbox.SandboxCreateOptions{
		Image:         imageURI,
		Volumes:       mounts,
		NetworkPolicy: policy,
		Metadata: map[string]string{
			"demesne.job":  jobID,
			"demesne.tool": "sandbox_script",
		},
	}

	sb, err := opensandbox.CreateSandbox(ctx, connConfig, createOpts)
	if err != nil {
		return ScriptResult{}, fmt.Errorf("create sandbox: %w", err)
	}
	defer func() {
		killCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = sb.Kill(killCtx)
	}()

	exec, err := sb.RunCommandWithOpts(ctx, opensandbox.RunCommandRequest{
		Command: req.Command,
		Cwd:     "/out",
		Timeout: scriptTimeout.Milliseconds(),
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
