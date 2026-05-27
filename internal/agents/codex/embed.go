package codex

import _ "embed"

// UNVERIFIED: This image has not been built or tested in this environment.
// The install method (npm install -g @openai/codex), base image (node:22-slim),
// and git inclusion are per Codex docs and must be confirmed once Codex is
// available. git is included because Codex prefers a git repo; we still
// pass --skip-git-repo-check to handle non-repo working directories.

//go:embed Dockerfile
var dockerfileBytes []byte

//go:embed codex-exec.sh
var wrapperScriptBytes []byte
