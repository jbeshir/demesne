# Evaluation lenses

Review only the supplied target snapshot and immutable evidence packet. Use normal read-only commands to inspect the target, bundled files, repository instructions and conventions, and relevant checked-in documentation, schemas, and contracts. Do not mutate, use network access, run build/test commands, evaluate another skill, or rerun checks. Write only the assigned report. Cite `TARGET-SKILL.md:<line>` and an evidence filename. State `clean` or give severity, issue, rationale, and an imperative repair. Keep execution failures separate from target defects.

## paths

Find hardcoded repository, user-home, machine, mount-basename, or provider filesystem paths. Permit generic Demesne paths `/in`, `/workspace`, and `/out`; require runtime discovery for mounted names.

## portability

Find assumptions about one agent, provider, model, credential, shell, OS, package layout, or host machine. Permit a run-specific model override only when it is explicitly scoped and the reusable workflow remains cross-provider.

## tool-contracts

Check tool names, supported parameters, pins, output locations, sequencing, terminal handling, retries, and parent copying against the supplied evidence. Treat a checked-in host schema and nested-child contract as complementary: flag unsupported parameters, but do not flag required nested `name` merely because the host schema omits it.

## imperative-focus

Check concise imperative instructions. Flag irrelevant rationale, duplicate procedure, vague discretion, hidden scope expansion, or prose that obscures a required action.

## Cross-cutting

Use only when no primary lens applies: report-only authority, preserved raw evidence/statuses, invalid target handling, or reviewer coverage gaps. Do not duplicate a primary finding.
