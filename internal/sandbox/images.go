package sandbox

import "fmt"

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
func ResolveImage(name string) (string, error) {
	if name == "" {
		name = DefaultImage
	}
	uri, ok := Images[name]
	if !ok {
		return "", fmt.Errorf("image %q is not in the whitelist (node, python, anaconda, go)", name)
	}
	return uri, nil
}
