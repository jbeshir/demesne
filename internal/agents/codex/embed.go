package codex

import _ "embed"

// The image installs the standalone musl binary of the Codex CLI release
// pinned by proxyopenai.CodexVersion (see Dockerfile and codexBuildArgs).

//go:embed Dockerfile
var dockerfileBytes []byte

//go:embed codex-retry.sh
var wrapperScriptBytes []byte
