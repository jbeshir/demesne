package codex

import (
	"context"

	"github.com/jbeshir/demesne/internal/agents/agentcommon"
	proxyopenai "github.com/jbeshir/demesne/internal/proxies/openai"
)

// codexVersionArg is the Dockerfile ARG the agent image reads to pin the
// installed Codex CLI release.
const codexVersionArg = "CODEX_VERSION"

var imageBuilder = &agentcommon.ImageBuilder{
	Repo:       "demesne-codex",
	TmpPrefix:  "demesne-codex-build-*",
	Dockerfile: dockerfileBytes,
	BuildArgs:  codexBuildArgs,
}

// ensureImage builds the codex image if it isn't already present in
// the local Docker daemon. Safe for concurrent first-callers.
func ensureImage(ctx context.Context) (string, error) { return imageBuilder.Ensure(ctx) }

// codexBuildArgs pins the image's Codex CLI to proxyopenai.CodexVersion, the
// same version the proxy reports upstream. The builder folds the version into
// the image tag, so bumping CodexVersion triggers an automatic rebuild.
func codexBuildArgs(context.Context) (map[string]string, error) {
	return map[string]string{codexVersionArg: proxyopenai.CodexVersion}, nil
}
