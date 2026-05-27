package anthropic

import (
	"context"

	"github.com/jbeshir/demesne/internal/agents/agentcommon"
)

var imageBuilder = &agentcommon.ImageBuilder{
	Repo:       "demesne-claude-code",
	TmpPrefix:  "demesne-anthropic-build-*",
	Dockerfile: dockerfileBytes,
}

// ensureImage builds the claude-code image if it isn't already present in
// the local Docker daemon. Safe for concurrent first-callers.
func ensureImage(ctx context.Context) (string, error) { return imageBuilder.Ensure(ctx) }
