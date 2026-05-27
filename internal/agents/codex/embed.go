package codex

import _ "embed"

// The image installs the pinned standalone musl binary (see Dockerfile).

//go:embed Dockerfile
var dockerfileBytes []byte

//go:embed codex-exec.sh
var wrapperScriptBytes []byte
