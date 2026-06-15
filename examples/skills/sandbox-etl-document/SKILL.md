---
name: sandbox-etl-document
description: Drive a document ETL pipeline through demesne — define a canonical schema and taxonomy, parse raw unstructured documents (PDFs, emails, HTML, transcripts, scanned forms) to text with a sandbox_script parser, extract one structured record per item with batched parallel sandbox_agent children, enrich and classify against the taxonomy, validate every record, and load into a structured store under /out/store/ (JSONL, SQLite, CSV, Parquet). Records that fail validation go to a quarantine pile with reason codes, never silently dropped. Apply when the user wants to turn a batch of documents into a clean structured dataset — "extract structured data from these PDFs", "ingest these emails into a schema", "ETL these files into a database". Skip for analytical questions over an existing corpus (use sandbox-corpus-map-reduce), auditing prose quality without a store (use sandbox-prose-defect-survey), and refreshing human-facing docs (use uplift-docs).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_research, mcp__demesne__sandbox_script
---

Turn a batch of unstructured documents into a schema-validated structured data store. You supply the documents, target schema, and classification taxonomy; you author one orchestrator prompt, launch a single slow-tier `sandbox_agent`, and that orchestrator drives parse → shard → extract → enrich → validate → load autonomously. The deliverable is `/out/store/` plus a quarantine pile; there is no code landing.

**Watch out (cross-cutting):** The orchestrator must `cp -a /out/child/load/store /out/store` directly — a `sandbox_script` dispatched to do it writes only to its own `/out/child/<name>` and strands the store there. Quarantine is non-negotiable: every item that fails parsing or validation must land in a quarantine file with reason codes, never silently dropped.

## Procedure

1. **Write schema and taxonomy.** Before spawning any children, the orchestrator writes `/workspace/schema.json` and `/workspace/taxonomy.md`. `/workspace/schema.json` must specify every field: name, type (`string`, `integer`, `date`, `enum[…]`), required/optional, and `_confidence_threshold` (0.0–1.0 per field). Confidence thresholds belong in `schema.json`, not hardcoded in the validator — hardcoded thresholds are invisible to the schema contract and impossible to tune without re-running the pipeline. Schema-first is mandatory: if any EXTRACT child spawns before the schema exists, each shard infers its own field shapes and VALIDATE becomes unusable.

   If the taxonomy requires open-web data (NAICS codes, ICD-10, a controlled vocabulary standard), spawn a `sandbox_research` child now. `sandbox_research` runs in a fresh private workspace — it has no `/in` mounts and no access to the shared `/workspace`. The orchestrator harvests the result from the researcher's `/out` and writes it to `/workspace/taxonomy.md` itself.

2. **PARSE.** Spawn one `sandbox_script` child (`image=python` or `image=anaconda`, `egress=package-managers`). It walks `/in/<docs>`, `pip install`s needed parsers, and writes:
   - `/workspace/parsed/<itemid>.txt` — plain text per item
   - `/workspace/parsed.jsonl` — `{"id":…,"source_path":…,"doc_type":…,"char_len":…}` per item
   - `/workspace/quarantine-parse.jsonl` — parse exceptions: `{"id":…,"source_path":…,"error":…}`

   Common parsers: `pdfplumber` for text PDFs; `pytesseract`+`pdf2image` for scanned PDFs needing OCR (heavy — add only if needed); `beautifulsoup4`+`lxml` for HTML; `python-docx` for `.docx`; `mailparser`/`mail-parser` for `.eml`. For mixed formats, instruct PARSE to sniff MIME type and dispatch accordingly. Parse failures go to `quarantine-parse.jsonl`; never forward them to EXTRACT. Do not pass raw binary documents to an LLM — `sandbox_script` parsers are faster and produce consistent text.

3. **SHARD.** Spawn one `sandbox_script` child (python, `egress=none`). Reads `/workspace/parsed.jsonl`, partitions successfully-parsed items into batches, writes `/workspace/shards/shard-NN.jsonl`. Default batch size: 20–50 items for medium-length documents; smaller for long documents or complex schemas; larger for short-form text. Write a single `shard-00.jsonl` for small corpora. Do not shard inside an EXTRACT child — an agent that exceeds context silently drops trailing items with no error signal.

4. **EXTRACT.** Spawn one medium-tier `sandbox_agent` per shard (`name=extract-NN`; DNS-1123: lowercase letters, digits, interior hyphens only, ≤40 chars — `extract-00` is valid, `Extract_Phase_1` is not), in batches of ≤4 concurrent. Four is a recommended batch size — demesne enforces no cap, but beyond four, MCP keepalive pressure on nested sandboxes degrades stability. Dispatch in waves, awaiting each before spawning the next.

   Each child reads `/workspace/schema.json`, its `/workspace/shards/shard-NN.jsonl`, and the corresponding `.txt` files from `/workspace/parsed/`, and emits exactly one JSON record per item to `/workspace/extracted/shard-NN.jsonl`. Every record must carry `_meta.confidence` (0.0–1.0) and `_meta.evidence` (verbatim source snippet) — records missing either fail VALIDATE with `missing-required-field`. Each child writes to a unique shard path; two children writing the same file produce corrupt JSONL.

5. **ENRICH.** Spawn one medium-tier `sandbox_agent` per shard (`name=enrich-NN`), batches of ≤4 concurrent, same wave pattern as EXTRACT. Each reads its extracted shard and `/workspace/taxonomy.md`, appends classification labels (e.g. `category`, `subcategory`, `intent`) per record, and writes `/workspace/enriched/shard-NN.jsonl`. When extraction and classification prompts are coherent and reference the same material, merge ENRICH into EXTRACT: the merged child reads both `schema.json` and `taxonomy.md` and writes directly to `/workspace/enriched/`. Keep them separate only when the prompts diverge meaningfully.

6. **VALIDATE.** Spawn one `sandbox_script` child (`image=anaconda`, `egress=none` — anaconda bundles `jsonschema`; no network install needed). Reads all `/workspace/enriched/*.jsonl`, validates each record against `/workspace/schema.json`, checks `_meta.confidence` against per-field `_confidence_threshold`, and bisects:
   - `/workspace/valid.jsonl` — records passing all checks
   - `/workspace/quarantine.jsonl` — failing records, each carrying original fields plus `_quarantine: {"reasons": [{"field":…,"code":…,"detail":…}]}`

   Standardised reason codes: `schema-violation`, `missing-required-field`, `type-mismatch`, `confidence-below-threshold`. A quarantine rate above ~10–15% usually signals a schema mismatch or threshold too tight, not bad source data.

7. **LOAD.** Spawn one `sandbox_script` child (python, `egress=none`). Reads `/workspace/valid.jsonl` and writes to its own `/out/store/`:
   - `data.jsonl` (default), `data.sqlite`, `data.csv`, or `data.parquet` per user request
   - `schema.json` — canonical schema pinned at run time
   - `quarantine.jsonl` — both quarantine files merged with `_quarantine.stage` (`"parse"` or `"validate"`)
   - `MANIFEST.md` — run date, source paths, counts: parsed / parse-quarantined / extracted / validation-quarantined / valid / loaded

   Parquet requires `pyarrow`; for that step use `egress=package-managers`. The LOAD child writes to `/out/child/load/store` as seen by the orchestrator.

8. **DELIVER.** The orchestrator copies the store:
   ```
   cp -a /out/child/load/store /out/store
   ```
   Then writes `/out/SUMMARY.md`: item counts by stage, validation pass rate, common quarantine reason codes and frequencies, evidence gap patterns, and recommended next steps (re-review quarantine, relax a confidence threshold, fix a parser for a doc type). `/workspace` is torn down when the orchestrator exits; only `/out` persists. Prints `DONE`.

## Writing the orchestrator prompt

Brief it as a complete document:

1. **The corpus** — what the documents are, where they live (`/in/<docs>`), how many items, dominant formats (PDF / HTML / email / mixed), and known quirks (scanned PDFs, encoding edge cases). This determines which parser packages PARSE installs.
2. **The schema** — every field: name, type, required/optional, and `_confidence_threshold`. The orchestrator writes `/workspace/schema.json` before any children spawn. Over-specify — under-specifying produces silent quarantine pile-up.
3. **The taxonomy** — classification labels for ENRICH. Written as `/workspace/taxonomy.md`. If it requires open-web data, route a `sandbox_research` child first and harvest its output into `/workspace/taxonomy.md`.
4. **The pipeline contract** — the eight steps above; batches of ≤4 for EXTRACT and ENRICH; quarantine discipline (no silent drops, reason codes required); `egress=package-managers` on PARSE only (and LOAD if Parquet); `egress=none` for VALIDATE and LOAD by default; the DELIVER `cp` pattern.
5. **Parser selection** — name the packages PARSE should install. Instruct it to sniff MIME type for mixed-format corpora.
6. **Destination store format** — JSONL (default; universal), SQLite (SQL queries), CSV (spreadsheet import; loses nested fields), or Parquet (columnar; requires `pyarrow`).
7. **Sharding policy** — default 20–50 items per shard. State explicitly; the orchestrator cannot infer document length before PARSE runs.
8. **Output contract** — `/out/store/` tree, `/out/SUMMARY.md`, and that all intermediate `/workspace/` files are scratch.

Terse prompts produce incorrect schemas and silent quarantine pile-up. Over-specify the contract; under-specify the extraction strategy.

## Output contract

```
/out/
  SUMMARY.md                    # Item counts, pass rate, quarantine analysis, next steps
  store/
    data.jsonl                  # (or .sqlite / .csv / .parquet per user request)
    schema.json                 # Canonical schema, pinned at run time
    quarantine.jsonl            # All quarantined records (parse + validation, merged)
    MANIFEST.md                 # Run date, source paths, stage counts
```

Everything under `/workspace/` (parsed, shards, extracted, enriched, valid, quarantine fragments) is scratch. Only `/out/store/` and `/out/SUMMARY.md` are durable deliverables.

## Launching the orchestrator

- **`directories: ["<abs path to docs>"]` is mandatory** — the PARSE child inherits this mount and reads documents from `/in/<docs>`. Without it, PARSE walks an empty directory and the quarantine pile contains everything.
- Tier: **slow** for the orchestrator; **medium** for EXTRACT and ENRICH children. PARSE, SHARD, VALIDATE, and LOAD are `sandbox_script` steps — no LLM tier applies.
- EXTRACT and ENRICH children run in batches of ≤4 concurrent; state this explicitly in the prompt. The default failure mode is spawning all shards simultaneously.- Child names must be DNS-1123 labels: lowercase letters, digits, interior hyphens only, ≤40 chars — `parse`, `shard`, `extract-00`, `enrich-00`, `validate`, `load` — never `Extract_Phase_1` or `enrich.final`.
