// Package webgamedevimage builds the demesne-owned `webgamedev` sandbox
// image: the upstream mcr.microsoft.com/playwright base (Chromium + Node,
// same as the browser image, with the `playwright` JS package added and
// NODE_PATH set so the offline playtest harness resolves it from any cwd)
// plus a warm Phaser + Vite + TypeScript project baked at /opt/game-template
// with its node_modules already installed. This bakes the full
// build-and-playtest toolchain for HTML5/web games so an agent can scaffold,
// build, and playtest a game at egress=none. Rebuilds are content-hash keyed
// by the embedded Dockerfile via agentcommon.ImageBuilder, so any recipe
// change — including a bumped version ARG default — forces a fresh build.
package webgamedevimage

import (
	"context"

	"github.com/jbeshir/demesne/internal/agents/agentcommon"
)

var imageBuilder = &agentcommon.ImageBuilder{
	Repo:       "demesne-webgamedev",
	TmpPrefix:  "demesne-webgamedev-build-*",
	Dockerfile: dockerfileBytes,
}

// Ensure builds the webgamedev image if it isn't already present in the
// local Docker daemon and returns its fully-qualified ref. Safe for
// concurrent first-callers.
//
// The build always runs on the host docker daemon. Sandboxes are created
// host-side even when requested from a nested sandbox (the child tool call
// is tunneled back to the host runner), so `image=webgamedev` works for both
// host and nested callers — the first use pays the build (including the
// template's npm install), every later use hits the cache. This is what
// enables in-sandbox web-game pipelines that build a game and then playtest
// it offline.
func Ensure(ctx context.Context) (string, error) { return imageBuilder.Ensure(ctx) }
