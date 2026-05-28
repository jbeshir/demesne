#!/bin/sh
# Wraps codex so a ChatGPT usage-limit window waits for the reported reset
# time and resumes the same thread; stdout must stay the verbatim JSONL stream
# so the caller's transcript is uninterrupted across retries.
set -u

RETRY_MAX_ATTEMPTS=${RETRY_MAX_ATTEMPTS:-6}
RETRY_MAX_TOTAL_WAIT_SECS=${RETRY_MAX_TOTAL_WAIT_SECS:-39600}
RETRY_RESET_BUFFER_SECS=${RETRY_RESET_BUFFER_SECS:-30}
RETRY_BACKOFF_BASE_SECS=${RETRY_BACKOFF_BASE_SECS:-300}

model=$1
prompt=$2

CODEX_HOME="$PWD/.codex"
CODEX_CONFIG_PATH=${CODEX_CONFIG_PATH:-/in/.agent/config.toml}
export CODEX_HOME
mkdir -p "$CODEX_HOME"
cp "$CODEX_CONFIG_PATH" "$CODEX_HOME/config.toml"

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

is_billing_fatal() {
    line=$1
    printf '%s' "$line" | grep -qiF 'out of credits' && return 0
    printf '%s' "$line" | grep -qiF 'spend cap' && return 0
    printf '%s' "$line" | grep -qiF 'purchase more credits' && return 0
    printf '%s' "$line" | grep -qiF 'upgrade to' && return 0
    printf '%s' "$line" | grep -qiF 'send a request to your admin' && return 0
    printf '%s' "$line" | grep -qiF 'ask your workspace owner' && return 0
    return 1
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

    if is_billing_fatal "$line"; then
        printf 'codex-retry: billing limit, no retry\n' >&2
        break
    fi

    if [ -z "$thread_id" ]; then
        printf 'codex-retry: quota exhausted but no thread_id captured; cannot resume\n' >&2
        break
    fi

    if [ "$attempt" -ge "$RETRY_MAX_ATTEMPTS" ]; then
        printf 'codex-retry: quota exhausted and max attempts (%d) reached\n' "$RETRY_MAX_ATTEMPTS" >&2
        break
    fi

    wait=$(reset_wait "$line" || backoff_wait)
    remaining=$((RETRY_MAX_TOTAL_WAIT_SECS - total_waited))
    if [ "$wait" -gt "$remaining" ]; then
        printf 'codex-retry: reset is beyond the wait budget; not retrying\n' >&2
        break
    fi

    printf 'codex-retry: quota exhausted, attempt %d/%d, sleeping %ds until reset, resuming thread %s\n' \
        "$attempt" "$RETRY_MAX_ATTEMPTS" "$wait" "$thread_id" >&2
    sleep "$wait"
    total_waited=$((total_waited + wait))
    attempt=$((attempt + 1))
done

exit "$rc"
