# Evaluation lenses

Give each reviewer the target snapshot and deterministic evidence packet. Permit ordinary read-only commands to inspect the one resolved target `SKILL.md`, bundled files, repository instructions and conventions, and relevant checked-in documentation, schemas, and tool contracts. Evaluate only that target skill; use supporting repository context as needed. Do not mutate files, run build or test commands, use network access, evaluate another skill, or write anything except the reviewer's own report. Cite `TARGET-SKILL.md:<line>` and an evidence filename. Do not re-run checks.

## paths

Find hardcoded repository, user-home, machine, mount-basename, or provider filesystem paths. Permit only Demesne contract paths `/in`, `/workspace`, and `/out` when used generically; require runtime discovery for mounted input names.

## portability

Find assumptions about a particular agent, provider, model, credential, shell, OS, package layout, or host machine. The skill must be usable by Codex and Claude without provider-specific instructions unless a documented capability is explicitly necessary and safely conditional.

## tool-contracts

Check commands, package/version pins, tool names, parameters, output locations, sequencing, and failure handling against the deterministic evidence and the supplied instructions. Flag invented, missing, or contradictory contracts.

## imperative-focus

Check that the skill is focused, concise, and imperative: it says only what the invoked agent must do and how. Flag irrelevant rationale, duplicate procedure, vague discretionary work, hidden scope expansion, or prose that obscures a required action.

## Small cross-cutting checks

Attach a concern here only when no primary lens already covers it: authority/safety (report-only boundary and no unauthorized edits), reproducibility/evidence (preserved raw results, statuses, and citations), or edge/failure handling (invalid target, checker failure, or reviewer failure). Do not duplicate a primary-lens finding.
