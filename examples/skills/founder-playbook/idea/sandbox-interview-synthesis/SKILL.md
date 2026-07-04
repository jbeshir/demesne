---
name: sandbox-interview-synthesis
status: alpha
description: Synthesise a corpus of customer-discovery interview notes/transcripts against a stated hypothesis with a built-in confirmation-bias check. An orchestrator mounts the interview corpus plus the hypothesis, batches interviews into chronological groups of five, fans out one debrief child per batch (per interview it records what CONFIRMED, what CHALLENGED, what was SURPRISING under a locked schema), then a SEPARATE fresh judge child per batch produces the two evidence lists — supporting vs challenging — and fires a suspicious-imbalance flag when supporting evidence outweighs challenging, and finally a slow-tier reducer synthesises across batches into a hypothesis-status verdict. Apply when a founder has run several discovery interviews and wants honest synthesis without hearing only what they want to hear — "synthesize my interview notes", "did these interviews validate the hypothesis", "check my customer interviews for confirmation bias", "what did my discovery calls actually say". Skip when you are drafting the interview questions (use sandbox-interview-kit-design), building the prospect list or scheduling (use sandbox-outreach-pipeline), sharpening the hypothesis itself or hunting disconfirming market evidence pre-interview (use sandbox-hypothesis-stress-test), or applying a generic extraction op to a corpus with no confirmation-bias framing (use sandbox-corpus-map-reduce).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_script
---

Synthesise mounted customer-discovery interviews against the founder's hypothesis, per interview and then in five-interview batches, with a fresh judge enforcing the confirmation-bias check the playbook demands. You author one orchestrator prompt, launch a slow-tier `sandbox_agent`, and it runs the pipeline autonomously. The deliverable is `/out/SYNTHESIS.md` (verdict-led report), `/out/debriefs.jsonl`, and per-batch audits; there is no code landing.

**Watch out (cross-cutting):** The imbalance flag is the entire point — if the child that WROTE a batch's debriefs also produces its supporting/challenging lists, the bias check is theatre (a debriefer rationalises its own optimism). Keep debrief and audit as separate fresh children. Interview ORDER is load-bearing: batches are chronological groups of five so the synthesis can read whether bias persists or shifts as interviews accumulate — a scrambled order corrupts that reading. Report-only over founder-private notes: children must LOG every unparseable file, never silently skip one, or the report claims a completeness it lacks.

## Procedure

1. **Intake (orchestrator's own process).** Require the hypothesis as an input — mounted at `/in/hypothesis/hypothesis.md` or embedded verbatim in the orchestrator prompt; without it there is nothing to debrief against. List `/in/interviews/`, determine each item's chronological position (from a filename date/number, in-content date, or a founder-supplied order file — log the assumed order and flag ambiguity), and write `/workspace/manifest.jsonl` (`{"item_id":"int-007","source_path":"...","interview_seq":7,"type":"txt|md|pdf|docx","interviewee":"role/segment or unknown"}`). One file may hold multiple interviews and one interview may span files — record the true interview count, not the file count. Note anything unidentifiable.

2. **Pre-process if needed (`sandbox_script`, `image=python`, `egress=package-managers`).** If the corpus is PDF/DOCX/RTF, convert to text into `/workspace/text/<item_id>.txt` so debrief children read clean text. Plain `.txt`/`.md` need no pre-process. Audio is out of scope — log it and require the founder to supply a transcript.

3. **Write `/workspace/op.md` (orchestrator).** Include the hypothesis verbatim, the debrief operation in plain English (per interview: what CONFIRMED the hypothesis, what CHALLENGED it, what was SURPRISING — L171–172), and the **locked JSONL schema** every `debriefs.jsonl` record must match: `item_id`, `source_path`, `interview_seq`, `interviewee`, `confirmed` (array of `{claim, quote, note}`), `challenged` (same shape), `surprised` (array of `{observation, quote}`), and `leading_risk` (per record: quotes that read as socially-desirable agreement or answers to a leading question, so the auditor can discount them). Every claim must carry a verbatim `quote` — no quote, no entry. Finalise the schema before any child runs; drift across children produces unmerge-able batches.

4. **Batch the manifest into chronological groups of ≤5 (orchestrator).** `batch-01` = interviews 1–5, `batch-02` = 6–10, … in `interview_seq` order. This is the playbook's "after every five interviews" unit, not a context-budget shard. Write `/workspace/batch-NN.jsonl` (the item list per batch). If a single transcript is large enough to threaten one child's context, split that batch into per-interview debrief children but keep the batch identity for the audit layer.

5. **DEBRIEF — one medium-tier `sandbox_agent` per batch** (`name=debrief-batch-01`, `debrief-batch-02`, …; DNS-1123: lowercase letters, digits, interior hyphens, ≤40 chars). Each child's prompt embeds its exact item list, full `/workspace/op.md`, the schema verbatim, and the hypothesis. It writes `/out/debriefs.jsonl` (one record per interview, schema-compliant) and `/out/log.md` (items skipped, parse failures, anomalies with reason). Dispatch with `background: true`, collect each `job_id`, poll `sandbox_wait` (`timeout_seconds: 120`), keep **≤8 in flight** (host-resource guard, not an MCP cap; launch a replacement as each finishes). Blocking calls are issued one per turn and run sequentially — background dispatch is the only way the debrief children run concurrently.

6. **AUDIT — one fresh medium-tier judge per batch, after ALL debrief children reach a terminal state** (`name=audit-batch-01`, …). This barrier holds: audit children read their batch's debriefs at `/in/previous-jobs/debrief-batch-NN/debriefs.jsonl`, and that mount fills only once the sibling completes. Each audit child is a FRESH context that never produced the debriefs it judges. It builds the two lists — **supporting** and **challenging** evidence — from the batch's debrief records, and is instructed adversarially (per L109–111, "the antidote is pointing it the other way"): actively hunt the `challenged`/`surprised`/`leading_risk` fields for friction the debriefer under-weighted, and treat each `confirmed` entry sceptically — is it genuine problem confirmation or socially-desirable agreement to a leading question? It then fires the **imbalance flag** on either trigger: (a) supporting entries ≥ 2× challenging entries, or (b) fewer than two substantive challenging entries across the five interviews — real discovery almost always surfaces friction, so a near-empty challenging list usually signals leading questions or optimistic debriefing, not a flawless hypothesis (L172–176). Output `/out/audit.md` (the two lists + verdict prose) and `/out/audit.jsonl` (`{batch_id, supporting_count, challenging_count, imbalance_flag, imbalance_reason, cited_item_ids}`). Fan out ≤8 in flight, same background loop.

7. **SYNTHESIS — one slow-tier `sandbox_agent` (`name=synthesis`), after ALL audit children terminate.** It reads every `/in/previous-jobs/audit-batch-NN/audit.jsonl` + `audit.md` and the debriefs, and writes `/out/SYNTHESIS.md`. It reports the cross-batch bias picture (per-batch supporting-vs-challenging counts, which batches flagged, whether the imbalance is systemic or shifts across the interview sequence), the confirmed/challenged/surprised themes with `item_id` citations, any REFRAME candidate (the real problem differing from the hypothesised one — L98), and a **hypothesis-status verdict**: `SUPPORTED` / `MIXED-REFRAME` / `NOT-SUPPORTED` / `CONFIRMATION-BIAS-WARNING` (the last overrides a positive verdict when imbalance flags are systemic — the data cannot be trusted until interviews or debriefing are de-biased). No single interview confirms a hypothesis; the verdict is a pattern across batches (L272–273 discipline). The reducer only reduces — it does no per-interview debriefing.

8. **Deliver (orchestrator's own process).** `cp` the synthesis child's `/out/SYNTHESIS.md`, and each batch's `audit.md`/`audit.jsonl` and `debriefs.jsonl`, into the orchestrator's own `/out`. Do NOT delegate this copy to a `sandbox_script` child — its `/out` is `/out/child/<name>` and the files would be stranded. Also write `/out/manifest.jsonl` and `/out/SUMMARY.md` (interviews processed / skipped / unparseable, batches audited, imbalance flags fired, assumed order). Print `DONE`.

## Writing the orchestrator prompt

Brief it as a complete document:

1. **The hypothesis and the corpus** — the hypothesis verbatim (WHO, HOW OFTEN, HOW SEVERELY, WHAT THEY DO TODAY — the L119–121 shape) and where the interviews are mounted (`/in/interviews/`). The debrief is always against THIS hypothesis.
2. **The debrief op + locked schema** — write `/workspace/op.md` before spawning any child; confirmed/challenged/surprised arrays, every entry carrying a verbatim quote, plus the `leading_risk` field. Schema compliance is enforced in each child prompt; the reducer cannot repair drift.
3. **Chronological batching rule** — groups of ≤5 in `interview_seq` order; batches are the playbook's every-five audit unit, and order must be preserved so the synthesis can read bias-over-time.
4. **Debrief/audit SEPARATION** — the audit judge is a fresh child that did not write the debriefs; state why (a debriefer rationalises its own imbalance). The audit is adversarially prompted to argue the challenging side and to discount socially-desirable confirmations.
5. **The imbalance triggers, verbatim** — flag when supporting ≥ 2× challenging OR challenging has <2 substantive entries per five interviews; cite the specific records; explain the reasoning in `imbalance_reason` (leading questions / optimistic debriefing vs a genuinely strong hypothesis).
6. **Verdict rubric** — the four verdict states and the rule that systemic imbalance flags force `CONFIRMATION-BIAS-WARNING` over any positive read; the verdict is a pattern across batches, never one interview.
7. **Output contract** — the files below; report-only, no edits/builds/commits; children log unparseable files rather than skipping them.

## Output contract

```
/out/
  SYNTHESIS.md         # verdict-led cross-batch report (cites item_ids)
  debriefs.jsonl       # every per-interview debrief record (concatenated)
  audits/
    audit-batch-01.md      # the two lists + batch verdict, human-readable
    audit-batch-01.jsonl   # {batch_id, supporting/challenging counts, imbalance_flag, ...}
    ...
  manifest.jsonl       # interview listing (item_id, source_path, interview_seq, ...)
  SUMMARY.md           # run summary: interviews processed/skipped, flags fired, assumed order
```

`SYNTHESIS.md` sections in order: **Verdict** (hypothesis-status + confidence + a bias-warning banner if flags are systemic, written last, placed first), **Hypothesis** (verbatim, what was tested), **Confirmation-bias audit** (per-batch supporting-vs-challenging counts, which batches flagged, whether imbalance is systemic or shifts over the sequence, cited records — the centrepiece), **Confirmed / Challenged / Surprised** (cross-batch themes with `item_id` citations), **Reframe candidates** (if the real problem differs from the hypothesised one), **Recommended next step** (proceed to solution concept / keep interviewing / re-audit debiased — cross-reference sandbox-solution-concept-pressure-test and sandbox-hypothesis-stress-test), **Methodology / Anomalies** (counts, unparseable items, assumed order).

## Launching the orchestrator

- **`directories: ["<abs path to interviews>"]` is mandatory, and the hypothesis must be mounted (`/in/hypothesis/`) or embedded in the prompt.** Forget the corpus mount and every debrief child wakes with nothing to read; forget the hypothesis and the debrief has no target.
- Debrief and audit children inherit the corpus mount and read their batch's interviews / a sibling's debriefs directly.
- Tier: **slow** for the orchestrator and the `synthesis` reducer; **medium** for debrief and audit children; deterministic format conversion is `sandbox_script`, never an LLM.
- Child names: `debrief-batch-01`, `audit-batch-01`, `synthesis` (and `convert-01` for any pre-process). Lowercase letters, digits, interior hyphens only, ≤40 chars.
