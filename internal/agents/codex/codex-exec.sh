#!/bin/sh
# Wrapper that runs Codex CLI headlessly inside a demesne sandbox.
# $1 = model id, $2 = prompt.
# Codex requires model_provider/base_url in the user-level CODEX_HOME
# config, so we point CODEX_HOME at a writable per-run dir under the
# agent's private cwd and copy the generated config.toml in.
set -eu
CODEX_HOME="$PWD/.codex"
export CODEX_HOME
mkdir -p "$CODEX_HOME"
cp /in/.agent/config.toml "$CODEX_HOME/config.toml"
exec codex exec --json --skip-git-repo-check \
    --dangerously-bypass-approvals-and-sandbox \
    --cd "$PWD" \
    -m "$1" \
    -- "$2"
