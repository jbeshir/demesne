// Package twineimage builds the demesne-owned `twine` sandbox image:
// the upstream mcr.microsoft.com/playwright base (Chromium + Node, same as
// the browser image, with the `playwright` JS package added and NODE_PATH
// set so the offline playtest harness resolves it from any cwd) plus the
// Tweego interactive-fiction compiler and the bundled Twine story formats
// (Harlowe, SugarCube, ...) under a TWEEGO_PATH directory. This bakes the
// full build-and-playtest toolchain for Twine games so both `tweego` and
// headless playtesting work at egress=none. Rebuilds are content-hash keyed
// by the embedded Dockerfile via agentcommon.ImageBuilder, so any recipe
// change — including a bumped version ARG default — forces a fresh build.
package twineimage

import (
	"context"

	"github.com/jbeshir/demesne/internal/agents/agentcommon"
)

var imageBuilder = &agentcommon.ImageBuilder{
	Repo:       "demesne-twine",
	TmpPrefix:  "demesne-twine-build-*",
	Dockerfile: dockerfileBytes,
}

// Ensure builds the twine image if it isn't already present in the local
// Docker daemon and returns its fully-qualified ref. Safe for concurrent
// first-callers.
//
// The build always runs on the host docker daemon. Sandboxes are created
// host-side even when requested from a nested sandbox (the child tool call
// is tunneled back to the host runner), so `image=twine` works for both host
// and nested callers — the first use pays the build, every later use hits
// the cache. This is what enables in-sandbox interactive-fiction pipelines
// that compile a Twine story and then playtest it offline.
func Ensure(ctx context.Context) (string, error) { return imageBuilder.Ensure(ctx) }
