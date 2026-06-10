// Package mediaimage builds the demesne-owned `media` sandbox image:
// FROM ubuntu:24.04 carrying a broad media toolbox — ffmpeg + ImageMagick +
// libvips + audio tooling (sox/lame/flac/opus-tools), plus
// mediainfo/exiftool/jpegoptim/pngquant/optipng/etc. — for video/audio/image
// conversion and manipulation. Rebuilds are content-hash keyed by the
// embedded Dockerfile via agentcommon.ImageBuilder, so any recipe change
// forces a fresh build.
package mediaimage

import (
	"context"

	"github.com/jbeshir/demesne/internal/agents/agentcommon"
)

var imageBuilder = &agentcommon.ImageBuilder{
	Repo:       "demesne-media",
	TmpPrefix:  "demesne-media-build-*",
	Dockerfile: dockerfileBytes,
}

// Ensure builds the media image if it isn't already present in the local
// Docker daemon and returns its fully-qualified ref. Safe for concurrent
// first-callers.
//
// The build always runs on the host docker daemon. Sandboxes are created
// host-side even when requested from a nested sandbox (the child tool call
// is tunneled back to the host runner), so `image=media` works for both
// host and nested callers — the first use pays the build, every later use
// hits the cache. This is what enables in-sandbox media-processing pipelines.
func Ensure(ctx context.Context) (string, error) { return imageBuilder.Ensure(ctx) }
