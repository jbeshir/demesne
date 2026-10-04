FROM node:22-slim

ARG CODEX_VERSION

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl git \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fL "https://github.com/openai/codex/releases/download/rust-v${CODEX_VERSION}/codex-x86_64-unknown-linux-musl.tar.gz" \
    | tar -xzf - \
    && mv codex-x86_64-unknown-linux-musl /usr/local/bin/codex \
    && chmod +x /usr/local/bin/codex

# Codex runs MCP tool calls through this helper, which it expects next to the
# codex binary; without it every tool call fails with "codex-code-mode-host
# is missing".
RUN curl -fL "https://github.com/openai/codex/releases/download/rust-v${CODEX_VERSION}/codex-code-mode-host-x86_64-unknown-linux-musl.tar.gz" \
    | tar -xzf - \
    && mv codex-code-mode-host-x86_64-unknown-linux-musl /usr/local/bin/codex-code-mode-host \
    && chmod +x /usr/local/bin/codex-code-mode-host

RUN codex --version
