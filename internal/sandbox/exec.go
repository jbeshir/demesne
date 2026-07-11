package sandbox

import (
	"context"
	"fmt"
	"time"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
)

// renewDuration is how far forward each Exec call pushes the sandbox TTL.
// Active sandboxes stay alive; idle ones expire after the original TTL.
const renewDuration = 48 * time.Hour

// Exec runs one command in an existing sandbox. The sandbox's TTL is
// refreshed before the command runs (best-effort — a renew failure does
// not block the exec).
func (r *Runner) Exec(ctx context.Context, req ExecRequest) (ExecResult, error) {
	sb, err := r.attach(ctx, req.SandboxID)
	if err != nil {
		return ExecResult{}, err
	}

	_, _ = sb.Renew(ctx, renewDuration)

	exec, err := sb.RunCommandWithOpts(ctx, opensandbox.RunCommandRequest{
		Command: req.Command,
		Cwd:     "/out",
		// Timeout is in milliseconds per SDK api/execd/gen.go; sibling
		// SandboxCreateOptions.TimeoutSeconds is seconds (units differ).
		Timeout: commandTimeout.Milliseconds(),
	}, nil)
	if err != nil {
		return ExecResult{}, fmt.Errorf("run command: %w", err)
	}

	exitCode := 0
	if exec.ExitCode != nil {
		exitCode = *exec.ExitCode
	}
	return ExecResult{
		Stdout:   combineMessages(exec.Stdout),
		ExitCode: exitCode,
		Stderr:   tailStderr([]byte(combineMessages(exec.Stderr))),
	}, nil
}
