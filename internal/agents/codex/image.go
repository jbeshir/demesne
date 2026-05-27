package codex

import (
	"context"

	"github.com/jbeshir/demesne/internal/agents/agentcommon"
)

var imageBuilder = &agentcommon.ImageBuilder{
	Repo:       "demesne-codex",
	TmpPrefix:  "demesne-codex-build-*",
	Dockerfile: dockerfileBytes,
}

// ensureImage builds the codex image if it isn't already present in
// the local Docker daemon. Safe for concurrent first-callers.
func ensureImage(ctx context.Context) (string, error) { return imageBuilder.Ensure(ctx) }
