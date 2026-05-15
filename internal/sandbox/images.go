package sandbox

import "fmt"

// DefaultImage is the image used when the caller does not specify one.
const DefaultImage = "anaconda"

// Images maps the names accepted by the sandbox_script tool to concrete
// container image references.
var Images = map[string]string{
	"node":     "node:22",
	"python":   "python:3.12",
	"anaconda": "continuumio/anaconda3:latest",
}

// ResolveImage returns the container image URI for a friendly name.
// An empty name resolves to DefaultImage. Unknown names are rejected.
func ResolveImage(name string) (string, error) {
	if name == "" {
		name = DefaultImage
	}
	uri, ok := Images[name]
	if !ok {
		return "", fmt.Errorf("image %q is not in the whitelist (node, python, anaconda)", name)
	}
	return uri, nil
}
