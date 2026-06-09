package sandbox

import (
	"fmt"
	"strings"
)

// ImageURI is a fully-resolved container image reference (e.g.
// "python:3.12" or a locally-built "demesne-browser:<hash>" tag),
// distinct from the user-facing friendly name ("python"). Produced by
// (*Runner).resolveImage.
type ImageURI string

// Friendly image names accepted by the sandbox tools. Extracted as
// constants so the same string literal isn't repeated across the map,
// the allowlist, the runner, and tests.
const (
	imageNode    = "node"
	imagePython  = "python"
	imageGo      = "go"
	imageBrowser = "browser"
	imageMedia   = "media"
)

// DefaultImage is the image used when the caller does not specify one.
const DefaultImage = "anaconda"

const imageAnaconda = "continuumio/anaconda3:latest"

// Images maps the pull-based friendly names to concrete container image
// references. Locally-built images (browser, media) are not here — the runner
// routes them to their builder package so they build lazily.
var Images = map[string]string{
	imageNode:    "node:22",
	imagePython:  "python:3.12",
	DefaultImage: imageAnaconda,
	// golang:1 tracks the latest stable Go 1.x; the default bookworm
	// variant is batteries-included (Go toolchain + git + gcc + make).
	imageGo: "golang:1",
}

// allowedImageNames lists every friendly name accepted by the sandbox
// tools, including locally-built ones. Used in the not-in-allowlist
// error so callers see the full allowlist, not just the static names.
var allowedImageNames = []string{imageNode, imagePython, DefaultImage, imageGo, imageBrowser, imageMedia}

// staticImageURI resolves a pull-based friendly name to its image URI.
// An empty name resolves to DefaultImage. The locally-built "browser" and
// "media" names are not handled here; (*Runner).resolveImage routes them to
// their builders before reaching this function.
func staticImageURI(name string) (ImageURI, error) {
	if name == "" {
		name = DefaultImage
	}
	uri, ok := Images[name]
	if !ok {
		return "", fmt.Errorf("image %q is not in the allowlist (%s)", name, strings.Join(allowedImageNames, ", "))
	}
	return ImageURI(uri), nil
}
