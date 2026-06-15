package anthropic

import (
	"context"
	"os/exec"
	"strings"

	"github.com/jbeshir/demesne/internal/agents/agentcommon"
)

// claudeCodeVersionArg is the Dockerfile ARG the agent image reads to pin the
// installed Claude Code version.
const claudeCodeVersionArg = "CLAUDE_CODE_VERSION"

// claudeCodePackage is the npm package whose latest version is queried when
// the host CLI version can't be read directly.
const claudeCodePackage = "@anthropic-ai/claude-code"

var imageBuilder = &agentcommon.ImageBuilder{
	Repo:       "demesne-claude-code",
	TmpPrefix:  "demesne-anthropic-build-*",
	Dockerfile: dockerfileBytes,
	BuildArgs:  claudeCodeBuildArgs,
}

// ensureImage builds the claude-code image if it isn't already present in
// the local Docker daemon. Safe for concurrent first-callers.
func ensureImage(ctx context.Context) (string, error) { return imageBuilder.Ensure(ctx) }

// claudeCodeBuildArgs resolves the Claude Code version to install into the
// agent image. It tracks the host's installed CLI so the sandbox stays in
// step with what the user runs; the version is folded into the image tag by
// the builder, so a host upgrade triggers an automatic rebuild with no
// demesne release. The agent allowlist (and thus which model aliases resolve)
// follows from whatever Claude Code version this installs.
func claudeCodeBuildArgs(ctx context.Context) (map[string]string, error) {
	return map[string]string{claudeCodeVersionArg: resolveClaudeCodeVersion(ctx)}, nil
}

// resolveClaudeCodeVersion returns the Claude Code version to install,
// preferring the host's installed CLI, then the npm registry's current
// release, and finally the "latest" dist-tag. It never errors: a version it
// can't pin still builds (against "latest"), it just won't auto-rebuild on
// the next upstream release.
func resolveClaudeCodeVersion(ctx context.Context) string {
	hostCmd := exec.CommandContext(ctx, "claude", "--version")
	if out, err := hostCmd.Output(); err == nil {
		if v, ok := parseClaudeCodeVersion(string(out)); ok {
			return v
		}
	}
	npmCmd := exec.CommandContext(ctx, "npm", "view", claudeCodePackage, "version")
	if out, err := npmCmd.Output(); err == nil {
		if v := strings.TrimSpace(string(out)); v != "" {
			return v
		}
	}
	return "latest"
}

// parseClaudeCodeVersion extracts the leading version token from the output
// of `claude --version` (e.g. "2.1.0 (Claude Code)" -> "2.1.0"). It reports
// false when the first token doesn't look like a dotted version, so the
// caller can fall back rather than pin a garbage value.
func parseClaudeCodeVersion(output string) (string, bool) {
	fields := strings.Fields(output)
	if len(fields) == 0 {
		return "", false
	}
	v := fields[0]
	if !strings.Contains(v, ".") {
		return "", false
	}
	for _, r := range v {
		if (r < '0' || r > '9') && r != '.' {
			return "", false
		}
	}
	return v, true
}
