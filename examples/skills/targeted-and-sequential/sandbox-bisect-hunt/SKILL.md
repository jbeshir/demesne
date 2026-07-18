---
name: sandbox-bisect-hunt
status: alpha
description: "Identify the change that introduced a reproducible regression without applying a fix."
---

Run a sequential, deterministic binary search and produce a root-cause report. Do not modify product code.

1. The host supplies a deterministic reproducer, classification mode, good and bad anchors, and a repository mount for Git/file axes. The orchestrator discovers the intended mount by enumerating `/in` and ignoring `previous-jobs`; missing or ambiguous input writes `/out/SUMMARY.md` `status: input-invalid` and stops. Copy the whole repository, including `.git`, to `/workspace/repo` and verify `git status` works; otherwise stop with `status: git-unavailable`.
2. Write `/workspace/symptom.md`. Functional mode classifies only the reproducer exit; performance mode requires a successful numeric metric and classifies only against its stated threshold. Never mix the rules.
3. For a Git axis, verify `good` is an ancestor of `bad`; write index 0 as `good`, then append `git rev-list --reverse --ancestry-path <good>..<bad>`, ending at `bad`. For other axes, write index 0=good and final=bad. If the axis exceeds 4096 entries, write `status: range-too-broad` and stop.
4. Run `probe-<NN>` sequentially. Each script distinguishes setup/infrastructure failure from a valid bad result and writes `/out/result.json` with `classification: good|bad|infrastructure-error`, exit, metric, and log tail. The parent reads `/out/child/probe-<NN>/result.json`, validates it, and requires tool success and classification other than infrastructure error before moving a bound. Retry a failed probe once as `probe-<NN>-retry`; then write the failure to `/out/bisect.log` and stop.
5. Run `verify-culprit` on predecessor and culprit. It writes `/out/verify.json` with both identities, measurements, classifications, and `assertion: pass|fail`. Read and validate `/out/child/verify-culprit/verify.json`; append the result to `/workspace/bisect.log`. A fail records `non-monotone-axis-suspected`.
6. Spawn `reader` only after a valid verify artifact. It reads the symptom, log, and diff, then writes `/out/ROOT_CAUSE.md`. Require succeeded, exit 0, and nonempty output; retry once as `reader-retry`. Copy accepted report and log to parent `/out`; write `/out/SUMMARY.md` with axis, bounds, culprit, verify result, and status.

## Output contract

```
/out/ROOT_CAUSE.md
/out/bisect.log
/out/SUMMARY.md
```
