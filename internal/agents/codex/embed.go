package codex

import _ "embed"

// The image installs the pinned standalone musl binary (see Dockerfile).

//go:embed Dockerfile
var dockerfileBytes []byte

//go:embed codex-retry.sh
var wrapperScriptBytes []byte
