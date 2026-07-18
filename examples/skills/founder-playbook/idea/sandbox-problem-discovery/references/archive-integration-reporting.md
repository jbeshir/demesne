# Archive, integration, and reporting contract

Use this contract whenever candidates are compared, investigated, reported, or packaged. It supplements, and never changes, the frozen gate contract.

## Identity and archive comparison

Assign a fresh `candidate_id` for this run. Treat repository track IDs, aliases, slugs, and stable labels as separate `external_identity` values; a collision is comparison input, not identity or novelty proof.

When an archive is mounted, inventory every archive candidate, including inactive, rejected, superseded, and archived records. Before ranking, compare every run candidate against the full inventory and record nearest neighbors across all nine mechanism dimensions:

1. customer or user;
2. payer and buying authority;
3. triggering event and cadence;
4. first-value job and promised outcome;
5. workflow inputs and data rights;
6. workflow steps and output;
7. acquisition or channel route;
8. economic model and continuing burden; and
9. external dependencies, permissions, and risk boundary.

Write typed `archive/inventory.json` records with stable ID, record type, status, byte-resolving source path/hash, and a digest of the ordered records. `archive/comparisons.json` must account for every inventory ID for every run candidate, include all nine dimensions at candidate and neighbor levels, and deterministically mark every neighbor tied at minimum distance. Append `archive/novelty-decisions.jsonl` with unique IDs, monotonic sequence, timestamp, inventory digest, and the digest of the preceding complete decision (null only for the first). Each decision resolves one comparison ID, names exactly its nearest neighbors, explains the mechanism delta, and uses `keep`, `dedup`, or `supersede`. Labels, aliases, embeddings, and lexical similarity may retrieve neighbors but cannot establish novelty. Do not alter archive records or statuses.

If no archive is mounted, write an empty inventory, comparisons with `archive_mounted: false`, and a `keep` novelty decision for every candidate explaining that only within-run comparison was possible.

## Diversity and selection transitions

Before provisional selection, write `aggregate/diversity-matrix.json` for every candidate pair that includes a finalist. Define adjacency as the number of identical mechanism dimensions meeting or exceeding the declared threshold. A pair above the threshold requires either a non-finalist or an explicit waiver explaining why its genuinely distinct mechanism delta merits retaining both. Shared shape alone is not grounds to collapse distinct jobs.

Append one contemporaneous transition to `selection-transitions.jsonl` whenever a candidate is meaningfully investigated or its selection state changes. Start at `null -> proposed`, follow the legal causal chain through investigation/finalist states, and end in a terminal catalog disposition. Every transition has a unique record ID, ordered timestamp, nonempty claim IDs, and resolving citations. Never infer a transition from score, rank, or caps. The last transition must equal both catalog and finalist disposition.

## Evidence effort and anchors

For C or D findings, record a claim ID, exact proposition, dated attempt log entries (date, query or action, locator, result), named source classes, access limits, bounded result, and the remaining proposition. Link score and gate bases to claim IDs and citations. Describe limited search as limited; never claim universal absence.

Score competitive stagnation/whitespace with these complete ranges: unknown = exactly 0; absence-only search = 0–1; named alternatives with an evidenced exact-job mismatch = 2–3; longitudinal or repeated evidence that the exact job remains unresolved = 4–5. Every nonzero score requires linked claims and resolving citations.

Decompose feasibility into structured `build_complexity`, `external_permissions_data`, `expert_judgment`, `procurement_channel`, `continuing_manual_burden`, and `unit_economics` assessments. Each records claims, citations, whether the strong-builder prior was applied, and either null or an exact frozen hard-red-line ID. Apply that prior only to build complexity. Treat ordinary privacy/compliance as meetable operational burden unless accepted evidence establishes an enumerated frozen hard red line.

## Review order and reports

For each provisional finalist, write `reviews/initial.json` from independent review and its SHA-256 into `gap-plans/round-1.json` before any gap work. Derive the round-1 plan from that artifact. Write `reviews/post-round-1.json` before a round-2 plan; derive round 2 from it. Always write `reviews/final.json` after the last round. Empty plans still preserve this order and provenance.

Write `reports/<candidate_id>.md` and a catalog-linked structured attestation for every meaningfully investigated candidate. The attestation maps negative searches, competitors and alternatives, claim-linked reasoning, selection-transition history, and reconsideration evidence to resolving report citations, finding IDs, and transition IDs. Finalists additionally attest every complete standalone section required by the main workflow. Record every report path, hash, and attestation path in the catalog.

## Immutable bundle and integration boundary

Emit `catalog.json` mapping every run candidate ID to its report path/hash, final disposition, external identity inputs, archive neighbors, and exactly one repository recommendation: `create`, `append`, or `supersede`. Recommendations are repository-neutral and do not claim integration.

Write `bundle-manifest.json` last with an immutable bundle ID, creation time, catalog path/hash, contract hash, and hashes for every integration-ready artifact. The canonical delivered bundle is append-only after hashing.

Set integration status to `not-integrated` unless explicit authorization, a safe real repository integration-contract path with its verified hash, and an integration completion artifact are recorded in `run-spec.json`. The completion artifact links the same contract and lists at least one completed action with timestamp and a real output path/hash. Even then, report only actions actually completed. Never claim repository integration from catalog recommendations, path guesses, label matches, or successful bundle creation.
