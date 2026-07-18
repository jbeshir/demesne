---
name: sandbox-product-ops-system
status: alpha
description: "Design a report-only product operating system from mounted context, metrics, and scope inputs. Do not create live schedules or integrations."
---

# Product operations system

The host supplies the complete procedure, three input basenames, and absolute directories to one orchestrator. Omit `model` unless the host supplies a concrete valid model.

## Procedure

1. Enumerate `/in`, excluding `previous-jobs`. Match each supplied basename to exactly one context, metrics, or scope mount. On missing or duplicate matches write `/out/INCOMPLETE.md` and stop. Stage resolved inputs under `/workspace/inputs/`.
2. Dispatch the four design children in background, at most eight. Each writes its declared design file and a `routing-table.json` entry. Maintain `/workspace/current-artifacts.json` mapping each logical artifact to its current child name and path.
3. Validate every round with `sandbox_script`, `image=python`, `egress=none`, and this command: `python -c 'import json,sys; p=sys.argv[1]; d=json.load(open(p)); assert isinstance(d,dict) and d, "nonempty JSON object required"' /workspace/current-artifacts.json`. Run it for round one and every `-r2`/`-r3` revision.
4. At every barrier repeat `sandbox_wait` while status is `running`. Accept only `succeeded`, `exit_code == 0`, and a nonempty declared artifact. Preserve diagnostics, retry once under a fresh name within the three-round cap, then abort with `INCOMPLETE.md`; do not read superseded `design-*` mounts.
5. Give each audit child the explicit four-entry current-artifact manifest. It writes `audit.md`. After each accepted revision, update the manifest before the next audit. On cap exhaustion retain the final audit and append still-open issues verbatim.
6. Compile only manifest-selected artifacts and the selected routing table into the orchestrator `/out`. Copy the final child audit to `/out/audit.md`. Write `/out/schedule.md`; stop there. Verify every required file is nonempty before `DONE`.

## Output contract

```text
/out/schedule.md
/out/audit.md
/out/routing-table.json
/out/<four manifest-selected design artifacts>
```

Every recurring ceremony has a non-founder owner or scheduled job; name any founder-judgment exception explicitly. This is a design report, not authorization to create calendar events, schedules, tool wiring, or messages.

## Launch inputs

Pass the three absolute directories and state their basenames and intended roles: context, metrics, scope.
