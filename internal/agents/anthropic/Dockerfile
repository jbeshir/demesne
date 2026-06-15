FROM node:22-slim

# CLAUDE_CODE_VERSION is resolved at build time from the host's installed
# Claude Code (see image.go) and folded into the image tag, so the sandbox
# CLI tracks the host's version and the image rebuilds automatically when the
# host upgrades. Defaults to the npm dist-tag for a plain `docker build`.
ARG CLAUDE_CODE_VERSION=latest

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/* \
    && npm install -g @anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}
