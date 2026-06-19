---
name: sandbox-routing-triage
description: Route a heterogeneous batch of items — issues, PRs, support tickets, security reports, intake documents, customer emails — through a classify-then-dispatch pipeline. An orchestrator reads the batch, builds a closed taxonomy of classes, runs classifier children to assign each item to exactly one class, then spawns specialist handler children for each populated class. Low-confidence items go to a quarantine bucket and are never auto-dispatched, and every routing decision is logged with its reason. Apply when a mixed batch of work items must be forwarded to the right handler without manual triage. Triggers include "triage these issues", "route these support tickets", "classify and dispatch", "batch intake routing", "assign issues to handlers", "sort this queue". Skip for single-item classification, a known-class change already understood (use sandbox-feature-work), and research or audit passes over a single corpus (use sandbox-code-defect-survey or sandbox-product-research).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_research, mcp__demesne__sandbox_script
---

Route a heterogeneous batch of work items through a classify-then-dispatch pipeline. The host writes one orchestrator prompt and launches a single slow-tier `sandbox_agent`; that orchestrator reads the batch, fixes a closed taxonomy, classifies each item, and spawns per-class handler children. The classifier only routes — it does not solve the work. No item is silently dropped; every routing decision appears in `/out/routes.jsonl`. The deliverable is `/out/SUMMARY.md` plus per-class results in `/out/results/<class>/`; there is no code or branch landing from this skill.

**Watch out:** copy results from `/out/child/handler-<class>/results` to `/out/results/<class>` in the orchestrator's own process — a `sandbox_script` child writes only to its own `/out/child/<name>` and strands the files. Never auto-dispatch items below the confidence threshold.

## Procedure

1. **INTAKE.** The orchestrator reads every file under `/in/<batch>/` and writes `/workspace/queue.jsonl`, one JSON object per line: `{id, path, preview, raw_type}`. `path` is the absolute `/in/<batch>/…` path for later use by handler children. Set `preview` to the first ~500 characters of the item's text — enough for the classifier to act without mounting the full batch to the classifier child. Log anything unreadable with `raw_type: "unreadable"` so it routes to `escalate-human` rather than disappearing.

2. **TAXONOMY.** Write `/workspace/classes.md` as a closed set before the classifier runs — a classifier briefed on an open set invents classes that have no handler. Each entry needs: a slug (lowercase letters, hyphens, ≤30 chars), a 2–4 sentence description, example signals, a handler spec (child name, output format, model tier), and a per-class confidence threshold. One mandatory entry: **`escalate-human`** for items outside the taxonomy, requiring policy review, or arriving in an unrecognised format. `escalate-human` has no handler child; its items go directly to `/out/quarantine.jsonl`. If the user supplied a taxonomy, write it verbatim; otherwise derive one from a scan of the batch's contents. If >20% of items land in `escalate-human`, note in `SUMMARY.md` that the taxonomy is too narrow — don't silently emit a large quarantine.

3. **CLASSIFY.** Spawn `name=classify01`, medium tier. DNS-1123 names are required throughout: lowercase letters, digits, interior hyphens only, ≤40 chars — bad names produce invalid volume names and poison sibling spawns. The classifier reads `/workspace/queue.jsonl` and `/workspace/classes.md`, then writes `/workspace/routes.jsonl`: `{id, class, confidence, reason}` per line. `class` must be a slug from `classes.md`; `confidence` is a float 0–1; `reason` is 1–2 plain-English sentences an auditor can read. Sharding: if the queue exceeds ~150 items (likely to overflow one classifier's context), split into `queue-shard-NN.jsonl`, dispatch N classifier children with `background: true` and poll with `sandbox_wait` (≤8 in flight) so they run concurrently, then `cat` their `routes-shard-NN.jsonl` outputs into `/workspace/routes.jsonl` before dispatch. Each shard's prompt must embed the full taxonomy — shards don't share context. Under ~150 items, one classifier child is enough; sharding adds merge complexity and an extra round trip.

4. **DISPATCH.** The orchestrator (not demesne) reads `/workspace/routes.jsonl` with `jq`, groups by class, and calls `sandbox_agent` once per populated class. Items below their per-class threshold (or a global floor if none is set) go to `/workspace/quarantine.jsonl` — never dispatched. Write per-class slices to `/workspace/slices/<class>.jsonl`. Before spawning any handler, write `/out/routes.jsonl` with `{id, class, confidence, reason}` for every item — this is the durable audit trail even if a handler later fails; COLLATE only appends the final `status`. Then spawn one `sandbox_agent` handler per non-empty, non-escalate-human class (`name=handler-<class>`, tier from taxonomy), dispatching each with `background: true` and polling with `sandbox_wait`, **≤8 in flight** — a host-resource guard, not a demesne-enforced cap (blocking calls are issued one per turn and run sequentially). Each handler child's prompt receives: its class definition, its slice at `/workspace/slices/<class>.jsonl`, and access (via the inherited `/in/<batch>` mount) to the actual item files; never pass `queue.jsonl` or the full batch to handlers. If a class has more items than fit in one context pass, tell the handler to chunk internally — the orchestrator does not spawn multiple children for the same class. If any handler needs open-web access (e.g. a CVE lookup), use `sandbox_research`: it gets a fresh private `/workspace` with no `/in` mounts — pass everything it needs in its prompt and collect output from its `/out`. (`sandbox_agent` with `egress: "open"` is not valid; that mode is rejected.) If a handler fails, still write its items with `status: "handler-error"`.

5. **COLLATE.** In the orchestrator's own process, run `cp -r /out/child/handler-<class>/results /out/results/<class>` for each completed handler. Write `/out/SUMMARY.md` (routing stats, per-class counts, quarantine count, any handler errors), `/out/routes.jsonl` (complete audit trail with final `status` per item: `dispatched`, `quarantined`, `escalated`, or `handler-error`), and `/out/quarantine.jsonl` (items for human review). Print `DONE`.

## Writing the orchestrator prompt

Brief it as a complete document:

1. **The batch** — path (`/in/<batch>`), format, provenance, typical size, and high-level routing intent.
2. **The taxonomy** — a fully specified closed set or enough domain context to derive one. Every item routes to exactly one class; `escalate-human` is always in scope; no invented classes.
3. **Confidence thresholds** — a global floor (e.g. 0.6) or per-class values. A silent prompt forces the orchestrator to guess and may be too aggressive or too permissive.
4. **The pipeline contract** — the five steps above, the child-naming rule, the background-dispatched handler fan-out (≤8 in flight via `sandbox_wait`), and the sharding rule (split only if queue > ~150 items).
5. **Handler specs** — what each handler produces (markdown summary, JSON records, draft reply). If classes have different tier requirements (slow for security reports, fast for labelling), say so explicitly.
6. **Audit-trail requirement** — every routing decision must appear in `/out/routes.jsonl` with its reason.
7. **Output contract** — the file tree below.

Terse prompts produce shallow pipelines and a classifier that invents its own classes. Over-specify the taxonomy and confidence policy.

## Output contract

```
/out/
  SUMMARY.md                     # routing stats, per-class counts, quarantine count, errors
  routes.jsonl                   # audit trail: {id, class, confidence, reason, status} per item
  quarantine.jsonl               # items below threshold + escalate-human items, for human review
  results/
    <class>/                     # one directory per dispatched class
      *                          # handler-defined output (markdown, JSON, diffs, etc.)
```

## Launching the orchestrator

- **`directories: ["<abs path to batch>"]` is mandatory.** The batch mounts as `/in/<batch>` inside the orchestrator and all children that inherit its `/in`; omitting it leaves the orchestrator with nothing to read.
- **Slow** tier for the orchestrator. **Medium** tier for the classifier and most handler children. **Slow** for handler classes requiring sustained multi-step reasoning (e.g. security-report triage, complex policy matching). **Fast** for high-volume, low-complexity classes (e.g. labelling, deduplication).
- Child naming: `classify01`, `handler-<class>` where `<class>` is the class slug sanitised to DNS-1123 form, ≤40 chars total including the `handler-` prefix.