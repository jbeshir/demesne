#!/usr/bin/env bash
# This script demonstrates the full persistent-sandbox lifecycle:
#   sandbox_create → sandbox_exec → sandbox_upload → sandbox_exec → sandbox_download → sandbox_destroy
#
# Because the sandbox_id is minted dynamically by sandbox_create, this script uses jq to
# extract the ID from step 1's response and substitute it into the subsequent calls.
#
# Prerequisites:
#   - jq installed and on $PATH
#   - demesne-mcp on $PATH (or DEMESNE_MCP set to its path)
#   - OPEN_SANDBOX_DOMAIN, OPEN_SANDBOX_API_KEY set on the demesne process
#   - DEMESNE_ALLOWED_PATHS includes /home/username/demesne-example
#   - /home/username/demesne-example/data.csv exists (see README.md Setup section)

set -euo pipefail

DEMESNE=${DEMESNE_MCP:-demesne-mcp}
DIR="$(dirname "$0")"

# Helper: send one JSON-RPC request to demesne-mcp (prepending initialize/initialized).
# Usage: send_request <json-string>
send_request() {
  local payload="$1"
  (
    printf '%s\n' '{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"demesne-example","version":"1.0"}}}'
    printf '%s\n' '{"jsonrpc":"2.0","method":"notifications/initialized"}'
    printf '%s\n' "$payload"
  ) | "$DEMESNE"
}

echo "=== Step 1: sandbox_create ==="
CREATE_RESPONSE=$(send_request "$(cat "$DIR/01-create.json")")
echo "$CREATE_RESPONSE"

# Extract sandbox_id from the tool result text.
# The result text looks like:
#   sandbox_id: <UUID>\noutput_dir: <path>
SANDBOX_ID=$(echo "$CREATE_RESPONSE" \
  | jq -r '.. | objects | select(.result?) | .result.content[0].text' \
  | grep '^sandbox_id:' \
  | awk '{print $2}')

if [[ -z "$SANDBOX_ID" ]]; then
  echo "ERROR: could not extract sandbox_id from step 1 response" >&2
  exit 1
fi
echo ""
echo "sandbox_id: $SANDBOX_ID"
echo ""

# Substitute SANDBOX_ID_HERE in steps 2-6.
substitute() {
  sed "s/SANDBOX_ID_HERE/$SANDBOX_ID/g" "$1"
}

echo "=== Step 2: sandbox_exec — install pandas ==="
send_request "$(substitute "$DIR/02-exec-install.json")"
echo ""

echo "=== Step 3: sandbox_upload — upload data.csv ==="
send_request "$(substitute "$DIR/03-upload.json")"
echo ""

echo "=== Step 4: sandbox_exec — analyse with pandas ==="
send_request "$(substitute "$DIR/04-exec-analyse.json")"
echo ""

echo "=== Step 5: sandbox_download — download results.json ==="
send_request "$(substitute "$DIR/05-download.json")"
echo ""

echo "=== Step 6: sandbox_destroy ==="
send_request "$(substitute "$DIR/06-destroy.json")"
echo ""

echo "Done. output_dir is preserved on the host."
