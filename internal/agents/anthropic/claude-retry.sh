#!/bin/sh
# Wraps claude so a 5-hour rate-limit exhaustion waits for the reported reset
# time and resumes the same session; stdout must stay the verbatim stream-json
# stream so the caller's transcript is uninterrupted across retries.
set -u

RETRY_MAX_ATTEMPTS=${RETRY_MAX_ATTEMPTS:-6}
RETRY_MAX_TOTAL_WAIT_SECS=${RETRY_MAX_TOTAL_WAIT_SECS:-39600}
RETRY_RESET_BUFFER_SECS=${RETRY_RESET_BUFFER_SECS:-30}
RETRY_BACKOFF_BASE_SECS=${RETRY_BACKOFF_BASE_SECS:-300}

cap=$(mktemp)
rc_file=$(mktemp)
trap 'rm -f "$cap" "$rc_file"' EXIT

attempt=1
total_waited=0
session_id=""
rc=0

while [ "$attempt" -le "$RETRY_MAX_ATTEMPTS" ]; do
    if [ "$attempt" -gt 1 ]; then
        # Rebuild the argv in place for the resume attempt, transforming the
        # current "$@" (the OS-provided argv on the first retry, the already
        # transformed argv on later ones — the transform is idempotent). The
        # `for` snapshots "$@" once, so clearing it on the first token and
        # appending the rebuilt tokens preserves each arg's boundaries even
        # when the prompt contains newlines. Prepend --resume <session_id>
        # after the leading `claude`, replace the -p/--print value with
        # `continue`, and drop any pre-existing resume/continue flags.
        first=1
        skip_next=0
        next_is_prompt=0
        for tok in "$@"; do
            if [ "$first" -eq 1 ]; then
                set -- "$tok" --resume "$session_id"
                first=0
                continue
            fi
            if [ "$skip_next" -eq 1 ]; then
                skip_next=0
                continue
            fi
            if [ "$next_is_prompt" -eq 1 ]; then
                set -- "$@" continue
                next_is_prompt=0
                continue
            fi
            case "$tok" in
                --resume|-r)
                    skip_next=1
                    continue
                    ;;
                --continue|-c)
                    continue
                    ;;
                -p|--print)
                    set -- "$@" "$tok"
                    next_is_prompt=1
                    continue
                    ;;
            esac
            set -- "$@" "$tok"
        done
    fi

    # Run claude, passing stdout through via tee to $cap; capture claude's real
    # exit code (the echo runs in the same subshell, before the pipe), not tee's.
    : >"$cap"
    : >"$rc_file"
    { "$@"; echo $? >"$rc_file"; } | tee "$cap"
    rc=$(cat "$rc_file")

    # Capture session_id from the init line the first time it appears.
    if [ -z "$session_id" ]; then
        init_line=$(grep -F '"subtype":"init"' "$cap" | head -1)
        if [ -n "$init_line" ]; then
            session_id=$(printf '%s' "$init_line" | sed -n 's/.*"session_id":"\([^"]*\)".*/\1/p' | head -1)
        fi
    fi

    # Confident quota signal: a rejected rate_limit_event AND a result line
    # carrying api_error_status 429 (same line).
    rejected_line=$(grep -F '"type":"rate_limit_event"' "$cap" | grep -F '"status":"rejected"' | head -1)
    result_429=""
    if grep -F '"type":"result"' "$cap" | grep -qF '"api_error_status":429'; then
        result_429=true
    fi

    quota_exhausted=false
    if [ -n "$rejected_line" ] && [ -n "$result_429" ]; then
        quota_exhausted=true
    fi

    # Billing exclusion: out_of_credits is fatal, not a retryable quota window.
    if [ "$quota_exhausted" = true ] && printf '%s' "$rejected_line" | grep -qF '"overageDisabledReason":"out_of_credits"'; then
        quota_exhausted=false
        printf 'claude-retry: billing limit (out_of_credits), no retry\n' >&2
    fi

    if [ "$quota_exhausted" = false ]; then
        break
    fi

    if [ -z "$session_id" ]; then
        printf 'claude-retry: quota exhausted but no session_id captured; cannot resume\n' >&2
        break
    fi

    if [ "$attempt" -ge "$RETRY_MAX_ATTEMPTS" ]; then
        printf 'claude-retry: quota exhausted and max attempts (%d) reached\n' "$RETRY_MAX_ATTEMPTS" >&2
        break
    fi

    # Compute how long to wait: until the reported reset (resetsAt, unix
    # seconds) plus a buffer, else exponential backoff.
    resets_at=$(printf '%s' "$rejected_line" | sed -n 's/.*"resetsAt":\([0-9][0-9]*\).*/\1/p' | head -1)
    if [ -n "$resets_at" ] && [ "$resets_at" -gt 0 ] 2>/dev/null; then
        now=$(date +%s)
        wait=$((resets_at - now + RETRY_RESET_BUFFER_SECS))
        if [ "$wait" -lt 0 ]; then
            wait=$RETRY_RESET_BUFFER_SECS
        fi
    else
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
    fi

    # Cap cumulative waiting under the sandbox command timeout: a reset beyond
    # the remaining budget (e.g. a 7-day window) can't be outlasted, so exit
    # instead of hanging.
    remaining=$((RETRY_MAX_TOTAL_WAIT_SECS - total_waited))
    if [ "$wait" -gt "$remaining" ]; then
        printf 'claude-retry: reset is beyond the wait budget; not retrying\n' >&2
        break
    fi

    printf 'claude-retry: quota exhausted, attempt %d/%d, sleeping %ds until reset, resuming session %s\n' \
        "$attempt" "$RETRY_MAX_ATTEMPTS" "$wait" "$session_id" >&2
    sleep "$wait"
    total_waited=$((total_waited + wait))
    attempt=$((attempt + 1))
done

exit "$rc"
