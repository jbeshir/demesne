#!/usr/bin/env bash
set -euo pipefail

DEMESNE=${DEMESNE_MCP:-demesne-mcp}

(
  printf '%s\n' '{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"demesne-example","version":"1.0"}}}'
  printf '%s\n' '{"jsonrpc":"2.0","method":"notifications/initialized"}'
  cat "$(dirname "$0")/request.json"
) | "$DEMESNE"
