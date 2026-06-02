package sandbox

import "fmt"

// ImageURI is a fully-resolved container image reference (e.g.
// "python:3.12") returned by ResolveImage. Distinct from the user-facing
// friendly name ("python") so callers that already have a built tag
// can use it directly without passing through ResolveImage.
type ImageURI string

// DefaultImage is the image used when the caller does not specify one.
const DefaultImage = "anaconda"

const imageAnaconda = "continuumio/anaconda3:latest"

// Images maps the names accepted by the sandbox_script tool to concrete
// container image references.
var Images = map[string]string{
	"node":     "node:22",
	"python":   "python:3.12",
	"anaconda": imageAnaconda,
	// golang:1 tracks the latest stable Go 1.x; the default bookworm
	// variant is batteries-included (Go toolchain + git + gcc + make).
	"go": "golang:1",
}

// ResolveImage returns the container image URI for a friendly name.
// An empty name resolves to DefaultImage. Unknown names are rejected.
func ResolveImage(name string) (ImageURI, error) {
	if name == "" {
		name = DefaultImage
	}
	uri, ok := Images[name]
	if !ok {
		return "", fmt.Errorf("image %q is not in the allowlist (node, python, anaconda, go)", name)
	}
	return ImageURI(uri), nil
}
