---
name: sandbox-founder-load-audit
status: alpha
description: "Audit mounted founder-workload evidence and produce delegation and automation designs. Do not build or deploy automations."
---

# Founder load audit

The host gives one `sandbox_agent` the complete procedure and the absolute evidence directory. Omit `model` unless the host supplies a concrete valid model; never pass `slow` or `medium` literally.

## Procedure

1. List `/in`, excluding `previous-jobs`. Resolve exactly one directory with the host-supplied evidence basename; otherwise write `/out/INCOMPLETE.md` and stop. Set `EVIDENCE` to that path. Write `/workspace/manifest.jsonl` with path, type, and size. If evidence is empty, write `/out/INTAKE-QUESTIONNAIRE.md` and stop.
2. Normalize CSV and ICS with a `sandbox_script` using `image=python`, `egress=none`, and Python standard library only: `python -c 'import csv,glob,os,shutil; os.makedirs("/workspace/normalized",exist_ok=True); [shutil.copyfile(p,"/workspace/normalized/"+os.path.basename(p)+".txt") for p in glob.glob("'+"$EVIDENCE"+'/**/*",recursive=True) if os.path.isfile(p)]'`. Record unreadable files in `normalize.log`; do not claim they were parsed.
3. Write `/workspace/op.md` with the locked extraction schema: `item_id, source_path, source_locator, title, kind, trigger, cadence, time_cost_est, who_does_it_today, founder_knowledge_dependency`. Dispatch `extract-*` jobs, at most eight, each writing `extracted.jsonl` and `log.md`.
4. For every child barrier, repeat `sandbox_wait` while running. Require succeeded, `exit_code == 0`, and a nonempty declared artifact. Preserve failure output, retry once under a fresh `-r2` name, then cancel dependents and write `/out/INCOMPLETE.md`; never continue from incomplete evidence.
5. After extracts pass, run `inventory` to deduplicate into `inventory.jsonl`; then `classify01` to assign exactly one bucket: `automate-entirely`, `human-but-not-founder`, or `founder-judgment`; then fresh `judgment-audit` to challenge only founder-judgment assignments. Harvest each successful child artifact to orchestrator `/out`.
6. Run `design-*` for automation candidates. Each writes `/out/designs/<item_id>.md` containing trigger, inputs, deterministic logic, exceptions, non-founder owner, audit trail, build effort, expected saved time, priority ratio, and knowledge to transfer. A recurring ceremony has a non-founder owner or scheduled job; name every founder-judgment exception.
7. Compile `/out/LOAD-AUDIT.md` from harvested files. Verify `LOAD-AUDIT.md`, `inventory.jsonl`, `audited.jsonl`, and every selected design exist and are nonempty before `DONE`.

## Output contract

```text
/out/LOAD-AUDIT.md
/out/inventory.jsonl
/out/audited.jsonl
/out/designs/<item_id>.md
/out/INTAKE-QUESTIONNAIRE.md         # only when evidence is insufficient
/out/INCOMPLETE.md                   # only on failed coverage
```

## Launch inputs

Pass `directories: ["<absolute evidence directory>"]` and state its basename, known formats, and any concrete host-approved model mapping. This pipeline reports and designs only.
