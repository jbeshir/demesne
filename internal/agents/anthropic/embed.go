package anthropic

import _ "embed"

//go:embed Dockerfile
var dockerfileBytes []byte

//go:embed claude-retry.sh
var retryScriptBytes []byte
