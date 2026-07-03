---
name: sandbox-prelaunch-security-review
description: Run a pre-launch security audit of an MVP codebase across the playbook's four fixed axes before any real user touches it — a slow-tier orchestrator stages the repo, runs a deterministic dependency-vulnerability scan via a `sandbox_script` child (the ecosystem's own audit tool, never an LLM), fans out three medium-tier auditors over the source (authentication & session handling; data exposure in API responses; input validation & injection), each finding carrying file:line + severity + a short exploit sketch and marked confirmed vs suspected, flags every finding touching authentication, secrets, or data handling HUMAN-REVIEW-REQUIRED, and synthesises one ranked security report. Audit-only by default; an optional fix mode drafts patches but never lands a human-review finding unreviewed. Apply when the user wants a targeted security pass before shipping an MVP — "pre-launch security review", "security audit before launch", "is this safe to ship to real users", "check for auth / injection / data-exposure bugs before the first user". Skip when the goal is framework certification (SOC 2 / GDPR / HIPAA → sandbox-compliance-review, the Launch-stage framework-oriented review), general non-security quality hardening (sandbox-quality-improvement), or a broad AI-code defect taxonomy (sandbox-code-defect-survey).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_script
---

Audit an MVP codebase for the four pre-launch security concerns the playbook names verbatim — authentication & session handling, data exposure in API responses, input validation & injection, and dependencies with known vulnerabilities — before any real user touches the product. The host launches one slow-tier `sandbox_agent` orchestrator; it runs the dependency scan as deterministic tooling, fans out one auditor per code axis, and synthesises a ranked report. **Report-only by default** — no edits, no branch. An **optional fix mode** drafts patches, but a hard playbook rule holds: anything touching authentication, secrets, or data handling is `HUMAN-REVIEW-REQUIRED` and may be drafted but never landed unreviewed.

**Watch out (cross-cutting):** (1) The dependency axis is a `sandbox_script` running the ecosystem's audit tool — an LLM "reading the deps" hallucinates CVEs and misses real ones; never route deps to an agent. (2) Every auditor must confirm by reading code — a grep hit is a candidate, not a finding — and cite `file:line`; unbacked findings are noise before launch. (3) The orchestrator must `cp` deliverables into its own `/out` itself; a `sandbox_script` child's `/out` is `/out/child/<name>` and would strand them. (4) This is not a substitute for qualified security review at higher stakes — the caveat ships in the report, not just here.

## Procedure

1. **Stage the repo.** `cp -a /in/<repo>/. /workspace/repo` — the `-a` preserves `.git` so auditors have `git log` and fix mode has a branch to cut. Detect the ecosystem (package manifest / lockfile) and the app's request surface (routers, controllers, GraphQL/OpenAPI schemas, auth middleware) — pass both to the children in their prompts so they don't rediscover structure.

2. **DEPSCAN** (axis 4, deterministic — never an LLM). One `sandbox_script`, `name=depscan`, `image=<lang>` matching the project, `egress=package-managers` (the audit tool must reach its advisory DB). Install the **pinned** dependencies from the lockfile first (never `@latest`, so results match what ships), then run the ecosystem's auditor and capture full output to `/out/DEPSCAN.txt`: Node `npm audit --json` (or `pnpm`/`yarn audit`), Python `pip-audit` (or `uv pip audit`), Go `govulncheck ./...`, Ruby `bundle audit check --update`, Rust `cargo audit`. If the advisory DB is unreachable under this egress, say so in the output rather than reporting a clean scan. The orchestrator parses each advisory into a finding: package, installed vs fixed version, severity, and whether the vulnerable package sits on an auth/crypto/data path (→ `HUMAN-REVIEW-REQUIRED`).

3. **AUDIT** (axes 1–3, code review). Spawn three medium-tier `sandbox_agent` children — `name=audit-auth-session`, `audit-data-exposure`, `audit-input-injection` (DNS-1123: lowercase letters, digits, interior hyphens, ≤40 chars; bad names produce invalid volume names and poison sibling spawns). Dispatch each with `background: true`, collect `job_id`s, poll `sandbox_wait` (`timeout_seconds: 120`) to completion — ≤8 in flight (a host-resource guard, not an MCP cap; here N=3 so dispatch all three). Blocking calls won't parallelise: the orchestrator issues children one per turn, so blocking auditors run strictly sequentially however the prompt is phrased. `egress=none` — audit is read-only over `/workspace/repo`. Each writes `/out/AUDIT-<axis>.md`; every finding carries `file:line`, a **severity** (critical / high / medium / low), a **2–3 line exploit sketch** (the concrete request or input that abuses it), **confirmed vs suspected**, and the `HUMAN-REVIEW-REQUIRED` flag where it touches auth, secrets, or data handling. "Clean on this axis" stated plainly is a valid, useful result — no manufactured findings. Axis briefs:
   - **auth & session handling** — missing/❨mis❩placed authorization checks (IDOR, missing ownership check on a resource ID), broken/absent session invalidation, weak token generation or storage, password/reset flows, privilege escalation, auth entirely bypassable on a route. *(All findings here are HUMAN-REVIEW-REQUIRED.)*
   - **data exposure in API responses** — endpoints returning more than the caller should see: password hashes / tokens / internal IDs / other users' records in a serializer, verbose error messages leaking stack traces or secrets, missing field-level authorization, PII in logs. *(HUMAN-REVIEW-REQUIRED.)*
   - **input validation & injection** — SQL/NoSQL/command injection, unsanitised input reaching a query or shell or template, SSRF, path traversal, unrestricted file upload, missing output encoding (XSS), mass-assignment. Flag HUMAN-REVIEW-REQUIRED where the sink touches secrets or data handling.

4. **SYNTHESIZE.** After all four axes complete, the orchestrator merges DEPSCAN findings + the three `/out/AUDIT-*.md` into one `/out/SECURITY-REVIEW.md`, deduped and ranked by severity (critical first), each finding keeping its axis, `file:line` (or package), exploit sketch, and flag. Open with a one-paragraph ship/hold read and close with the fixed caveat (see Output contract). The orchestrator adjudicates overlaps itself — auditors do not debate each other. Then, if fix mode is off, print `DONE`.

5. **FIX (optional, only if the user opted in).** Group the **non-human-review** findings into small numbered phases; spawn one medium-tier `sandbox_agent` per phase (`name=fix01`, `fix02`, …) editing `/workspace/repo` on branch `security-fixes-DRAFT` cut from the mounted HEAD. Fix phases share `/workspace` — run them sequentially. For `HUMAN-REVIEW-REQUIRED` findings, draft the patch **but do not merge it into the working tree**: write it as `/out/drafts/<slug>.patch` with a note, so a human applies it after review. Each fix phase writes `/out/FIX-<nn>.md`.

6. **VERIFY & re-scan.** After the fix phases, spawn one **fresh** medium-tier `sandbox_agent` (`name=verify01`, never a producer scoring its own work) to read `git -C /workspace/repo diff` and confirm each applied fix closes its finding without changing unrelated behaviour → verdict `PASS` / `CHANGES_NEEDED` in `/out/VERIFY.md`. Re-run the step-2 `depscan` script to confirm dependency fixes cleared. On `CHANGES_NEEDED`, one more fix round, then re-verify — **cap 2 rounds**, then stop and document what remains. `cp -a /workspace/repo /out/repo` in the orchestrator's own process (not via a child). Print `DONE`.

## Severity & the human-review gate

Severity is exploitability × blast radius: **critical** = pre-auth or trivially remote and high-impact; **high** = authed-but-easy or sensitive data; **medium/low** = defence-in-depth. The gate is orthogonal to severity: *every* finding touching authentication, secrets, or data handling is `HUMAN-REVIEW-REQUIRED` regardless of severity, and the pipeline may only **draft** its fix — landing it unreviewed is the exact failure mode this review exists to prevent. The report must state, verbatim: *"This is an automated pre-launch pass, not a substitute for qualified security review at higher stakes."*

## Writing the orchestrator prompt

Brief it as a complete document — it starts cold with only your prompt and the mounted repo:

1. **Target & surface** — what the app is, its stack/ecosystem, and a map of routers/controllers/auth middleware/serializers so auditors don't rediscover structure. State whether fix mode is on or off.
2. **The four axes verbatim** — auth & session handling; data exposure in API responses; input validation & injection; dependencies with known vulnerabilities. Axes 1–3 are LLM auditors; **axis 4 is a `sandbox_script`, never an agent** — give the exact audit command for the ecosystem and `image=<lang>`, `egress=package-managers`, install pinned deps first.
3. **Pipeline contract** — the six steps; child-naming rule; background-dispatch + `sandbox_wait` for the three auditors (≤8 in flight); SYNTHESIZE only after all four axes finish.
4. **Finding discipline** — confirm by reading code (grep locates, reading confirms); `file:line`; severity scale; 2–3 line exploit sketch (the actual malicious request/input); confirmed vs suspected; "clean on this axis" is valid; no manufactured findings.
5. **The human-review hard rule** — every finding touching auth, secrets, or data handling is `HUMAN-REVIEW-REQUIRED`; fix mode drafts these as `.patch` files and never lands them; the caveat sentence ships in the report.
6. **Output contract** — the files below; report-only unless fix mode is on.

## Output contract

```
/out/
  SECURITY-REVIEW.md    # headline deliverable: ship/hold read, findings ranked
                        # by severity, HUMAN-REVIEW-REQUIRED flags, closing caveat
  DEPSCAN.txt           # raw ecosystem-auditor output (axis 4)
  AUDIT-auth-session.md
  AUDIT-data-exposure.md
  AUDIT-input-injection.md
  # fix mode only:
  drafts/<slug>.patch   # drafted fixes for HUMAN-REVIEW-REQUIRED findings (unapplied)
  FIX-<nn>.md           # per-phase applied-fix summaries
  VERIFY.md             # fresh-context verdict (PASS / CHANGES_NEEDED)
  repo/                 # branch security-fixes-DRAFT; NOT ff-mergeable unreviewed
```

`SECURITY-REVIEW.md` section order: **Ship/hold summary → Findings by axis (auth-session, data-exposure, input-injection, dependencies), each ranked by severity → HUMAN-REVIEW-REQUIRED index → Fixes applied / drafted (fix mode) → Caveat.**

Workspace artefacts (orchestrator reads; not in `/out`): `/workspace/repo/` (working copy with `.git`).

## Launching the orchestrator

- **`directories: ["<abs path to repo>"]` is mandatory.** Without it the orchestrator mounts no repo, the auditors have nothing to read, and the dep scan has no lockfile — the run produces an empty report.
- Tier: **slow** for the orchestrator; **`sandbox_script`** for `depscan`; **medium** for the three auditors, the fix phases, and `verify01`.
- The three auditors fan out via **background dispatch + `sandbox_wait`, ≤8 in flight** — say this in the prompt.

## Host-side finishing

1. Read `/out/SECURITY-REVIEW.md` — the ship/hold read, the ranked findings, and the `HUMAN-REVIEW-REQUIRED` index.
2. For every `HUMAN-REVIEW-REQUIRED` finding, a qualified human reviews the cited `file:line` and any drafted `/out/drafts/<slug>.patch` before anything lands — the pipeline deliberately did not merge these. Do **not** ff-merge `/out/repo`'s `security-fixes-DRAFT` branch without that review.
3. Non-human-review fixes (verified `PASS`, no auth/secrets/data touch) can be cherry-picked from the draft branch after the founder confirms the `VERIFY.md` verdict.
4. This pass does not certify the product secure. For framework certification (SOC 2 / GDPR / HIPAA) route to `sandbox-compliance-review`; for general quality hardening, `sandbox-quality-improvement`.
