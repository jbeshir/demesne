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
// The build always runs on the host docker daemon. Sandboxes are created
// host-side even when requested from a nested sandbox (the child tool
// call is tunneled back to the host runner), so `image=browser` works for
// both host and nested callers — the first use pays the build, every
// later use hits the cache. This is what enables in-sandbox pipelines
// such as end-to-end React development.
func Ensure(ctx context.Context) (string, error) { return imageBuilder.Ensure(ctx) }
