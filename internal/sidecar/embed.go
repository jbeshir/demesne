package sidecar

import _ "embed"

//go:embed Dockerfile
var dockerfileBytes []byte

// sidecarBinary is the linux/amd64 demesne-sidecar binary, baked in at
// build time. The Makefile cross-compiles it to
// internal/sidecar/dist/demesne-sidecar before `go build ./cmd/...`
// runs, so this file always reflects the freshly-compiled version.
//
//go:embed dist/demesne-sidecar
var sidecarBinary []byte
