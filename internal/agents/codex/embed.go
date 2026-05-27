package codex

import _ "embed"

//go:embed Dockerfile
var dockerfileBytes []byte

//go:embed codex-exec.sh
var wrapperScriptBytes []byte
