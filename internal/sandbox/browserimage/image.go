// Package browserimage builds the demesne-owned `browser` sandbox image:
// the upstream mcr.microsoft.com/playwright base ships version-matched
// browser binaries and Node but not the `playwright` JS package, so the
// embedded Dockerfile adds it via `npm install -g playwright@1.60.0` and
// sets NODE_PATH so CommonJS `require('playwright')` resolves from any
// cwd (including the read-only /in mount). Rebuilds are content-hash
// keyed by the embedded Dockerfile via agentcommon.ImageBuilder, so any
// recipe change forces a fresh build.
package browserimage

import (
	"context"

	"github.com/jbeshir/demesne/internal/agents/agentcommon"
)

var imageBuilder = &agentcommon.ImageBuilder{
	Repo:       "demesne-browser",
	TmpPrefix:  "demesne-browser-build-*",
	Dockerfile: dockerfileBytes,
}

// Ensure builds the browser image if it isn't already present in the
// local Docker daemon and returns its fully-qualified ref. Safe for
// concurrent first-callers.
//
// Builds run on the host docker daemon; nested in-sandbox use of
// `image=browser` cannot build (no docker in a sandbox), so that path
// surfaces the docker build error — out of scope, the capability
// targets the host tool surface.
func Ensure(ctx context.Context) (string, error) { return imageBuilder.Ensure(ctx) }
