#!/bin/sh
# Wraps codex so a ChatGPT usage-limit window is survived rather than fatal:
# the wrapper waits for the reset time reported in the failure message — even
# when that message also carries "Upgrade to Pro"/"purchase more credits" upsell
# text — and resumes the same thread. When the message has no parseable reset
# time it falls back to exponential backoff. stdout must stay the verbatim JSONL
# stream so the caller's transcript is uninterrupted across retries.
set -u

# Retry budget: survive a usage-limit / billing window for up to six hours,
# with the first backoff retry after five minutes (used when the failure
# message carries no explicit reset time). RETRY_MAX_TOTAL_WAIT_SECS is the
# hard ceiling on total time spent waiting across all attempts; RETRY_MAX_ATTEMPTS
# is high enough that the time budget, not the attempt count, is the binding limit.
RETRY_MAX_ATTEMPTS=${RETRY_MAX_ATTEMPTS:-12}
RETRY_MAX_TOTAL_WAIT_SECS=${RETRY_MAX_TOTAL_WAIT_SECS:-21600}
RETRY_RESET_BUFFER_SECS=${RETRY_RESET_BUFFER_SECS:-30}
RETRY_BACKOFF_BASE_SECS=${RETRY_BACKOFF_BASE_SECS:-300}

model=$1
prompt=$2

CODEX_HOME="$PWD/.codex"
CODEX_CONFIG_PATH=${CODEX_CONFIG_PATH:-/in/.agent/config.toml}
CODEX_MODEL_CATALOG_PATH="$CODEX_HOME/model-catalog.json"
export CODEX_HOME
mkdir -p "$CODEX_HOME"
cp "$CODEX_CONFIG_PATH" "$CODEX_HOME/config.toml"
# model_catalog_json must be a top-level key, so it has to land before the
# first `[table]` header (`[model_providers.demesne]`) in the copied config —
# a bare `key = value` appended after that header would be parsed as
# belonging to that table instead. Prepending guarantees top-level placement
# regardless of the file's internal structure.
tmp_config=$(mktemp)
{
    printf 'model_catalog_json = "%s"\n' "$CODEX_MODEL_CATALOG_PATH"
    cat "$CODEX_HOME/config.toml"
} >"$tmp_config"
mv "$tmp_config" "$CODEX_HOME/config.toml"

fetch_model_catalog() {
    if [ -z "${DEMESNE_OPENAI_AGENT_KEY:-}" ]; then
        printf 'codex-retry: %s is required to fetch Codex model catalog\n' "DEMESNE_OPENAI_AGENT_KEY" >&2
        return 1
    fi

    tmp_catalog=$(mktemp)
    status=$(curl -sS \
        -H "Authorization: Bearer $DEMESNE_OPENAI_AGENT_KEY" \
        -H "version: 0.144.3" \
        -H "User-Agent: codex_cli_rs/0.144.3 (demesne)" \
        -o "$tmp_catalog" \
        -w '%{http_code}' \
        'http://127.0.0.1:8086/backend-api/codex/models?client_version=0.144.3') || {
        rc=$?
        rm -f "$tmp_catalog"
        printf 'codex-retry: failed to fetch Codex model catalog from sidecar (curl exit %d)\n' "$rc" >&2
        return 1
    }
    if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
        printf 'codex-retry: failed to fetch Codex model catalog from sidecar: HTTP %s\n' "$status" >&2
        if [ -s "$tmp_catalog" ]; then
            sed -n '1,5p' "$tmp_catalog" >&2
        fi
        rm -f "$tmp_catalog"
        return 1
    fi
    if ! node -e 'const fs = require("fs"); const path = process.argv[1]; const body = JSON.parse(fs.readFileSync(path, "utf8")); if (!body || !Array.isArray(body.models) || body.models.length === 0) { throw new Error("expected non-empty models array"); }' "$tmp_catalog" 2>/dev/null; then
        rm -f "$tmp_catalog"
        printf 'codex-retry: sidecar returned unusable Codex model catalog JSON; expected object with non-empty models array\n' >&2
        return 1
    fi
    mv "$tmp_catalog" "$CODEX_MODEL_CATALOG_PATH"
}

fetch_model_catalog || exit 1

cap=$(mktemp)
rc_file=$(mktemp)
trap 'rm -f "$cap" "$rc_file"' EXIT

attempt=1
total_waited=0
thread_id=""
rc=0

extract_thread_id() {
    grep -F '"type":"thread.started"' "$cap" | head -1 | sed -n 's/.*"thread_id":"\([^"]*\)".*/\1/p' | head -1
}

quota_line() {
    grep -F '"type":"turn.failed"' "$cap" | grep -F "You've hit your usage limit" | head -1
}

reset_wait() {
    line=$1
    reset_text=$(printf '%s' "$line" | sed -n "s/.*[Tt]ry again at \\([^\".]*\\).*/\\1/p" | head -1)
    if [ -z "$reset_text" ]; then
        return 1
    fi

    # GNU date does not consistently accept ordinal day suffixes.
    reset_text=$(printf '%s' "$reset_text" | sed 's/\([0-9][0-9]*\)\(st\|nd\|rd\|th\)/\1/g')
    reset_epoch=$(date -d "$reset_text" +%s 2>/dev/null) || return 1
    now=$(date +%s)
    wait=$((reset_epoch - now + RETRY_RESET_BUFFER_SECS))
    if [ "$wait" -lt "$RETRY_RESET_BUFFER_SECS" ]; then
        wait=$RETRY_RESET_BUFFER_SECS
    fi
    printf '%s\n' "$wait"
}

backoff_wait() {
    exp=$((attempt - 1))
    wait=$RETRY_BACKOFF_BASE_SECS
    i=0
    while [ "$i" -lt "$exp" ]; do
        wait=$((wait * 2))
        if [ "$wait" -gt 3600 ]; then
            wait=3600
            break
        fi
        i=$((i + 1))
    done
    printf '%s\n' "$wait"
}

while [ "$attempt" -le "$RETRY_MAX_ATTEMPTS" ]; do
    : >"$cap"
    : >"$rc_file"

    if [ "$attempt" -eq 1 ]; then
        { codex exec --json -s danger-full-access --skip-git-repo-check -C "$PWD" -m "$model" -- "$prompt" </dev/null; echo $? >"$rc_file"; } | tee "$cap"
    else
        { codex exec resume "$thread_id" --json -m "$model" --skip-git-repo-check continue </dev/null; echo $? >"$rc_file"; } | tee "$cap"
    fi
    rc=$(cat "$rc_file")

    if [ -z "$thread_id" ]; then
        thread_id=$(extract_thread_id)
    fi

    line=$(quota_line)
    if [ -z "$line" ]; then
        break
    fi

    if [ -z "$thread_id" ]; then
        printf 'codex-retry: usage limit hit but no thread_id captured; cannot resume\n' >&2
        break
    fi

    if [ "$attempt" -ge "$RETRY_MAX_ATTEMPTS" ]; then
        printf 'codex-retry: usage limit hit and max attempts (%d) reached; giving up\n' "$RETRY_MAX_ATTEMPTS" >&2
        break
    fi

    # Prefer the reset time the failure message reports; ChatGPT usage-limit
    # messages routinely carry "Upgrade to Pro"/"purchase more credits" upsell
    # text alongside a real "try again at <time>", so honour the reset rather
    # than treating the upsell as fatal. With no parseable reset time, fall
    # back to exponential backoff instead of giving up.
    if wait=$(reset_wait "$line"); then
        wait_reason='reported reset'
    else
        wait=$(backoff_wait)
        wait_reason='exponential backoff (no reset time in message)'
    fi

    remaining=$((RETRY_MAX_TOTAL_WAIT_SECS - total_waited))
    if [ "$wait" -gt "$remaining" ]; then
        printf 'codex-retry: next wait of %ds is beyond the wait budget (%ds remaining); giving up\n' "$wait" "$remaining" >&2
        break
    fi

    printf 'codex-retry: usage limit hit, attempt %d/%d, sleeping %ds (%s), resuming thread %s\n' \
        "$attempt" "$RETRY_MAX_ATTEMPTS" "$wait" "$wait_reason" "$thread_id" >&2
    sleep "$wait"
    total_waited=$((total_waited + wait))
    attempt=$((attempt + 1))
done

exit "$rc"
