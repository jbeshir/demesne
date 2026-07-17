# Gate contract v1

Read this contract before research. The unit of analysis is the proposed **first-value workflow** for one named customer and payer. Evidence states describe knowledge, not desirability.

## Evidence states

- **A — affirmative pass evidence:** accepted facts plus a bounded inference directly establish the observable.
- **B — affirmative disconfirming evidence:** accepted facts directly show that the stated first-value workflow violates the observable. Only B can establish product disconfirmation.
- **C — investigated, evidence insufficient:** the proposition was sought but permitted evidence establishes neither A nor B.
- **D — not adequately requested or operationalized:** no accepted artifact asked a usable test of the proposition.
- **E — contract or compiler conflict:** artifacts use materially different definitions, standards drifted after freeze, or similarly situated evidence was treated inconsistently.

Unknown is C or D. Unknown may block advancement but is never B. E requires reconciliation against this frozen contract, not majority vote.

## Hard-red-line policy

Use few true red lines. Reject automatically only when accepted evidence establishes that first value necessarily has one of these external or non-buildable properties:

1. required data is inaccessible or illegal to obtain or use;
2. mandatory privileged credentials or institutional cooperation cannot be obtained for a bounded first-value route;
3. delivery legally requires licensing or certification the founder does not and cannot reasonably hold;
4. delivery requires an unobtainable physical dependency;
5. the core output necessarily performs irreducible expert judgment or an unavoidable regulated/materially high-liability judgment in medical, legal, financial, safety, eligibility, safeguarding, or formal-compliance work, and normal controls or narrower scope cannot remove it; or
6. even one bounded unit has impossible unit economics under explicit assumptions.

G1–G8 are evidence concerns, not a shortcut list of universal knockouts. A B cell triggers automatic rejection only when it proves one of the six red lines above. Integration, scraping, public/enterprise procurement, channel weakness, low recurrence, or paid-demand rejection can block selection or reduce scores without becoming a buildability red line unless they prove an enumerated condition. Scores never override a hard red line. A genuinely narrower first-value workflow may be evaluated as a new version; never relabel the same workflow.

Ordinary privacy, security, communications, processor, contractual, tax, recordkeeping, and sector compliance duties are not hard red lines merely because regulation exists. Score their concrete operational burden unless the core deliverable necessarily performs the prohibited judgment. Controls reduce risk but do not prove absence of regulated judgment. Do not provide legal clearance.

Apply an optimistic strong-builder prior. Assume a capable technical founder can handle OCR, APIs, integrations, messy data, automation, moderate software complexity, difficult but conventional software, and bounded manual operations. None is a rejection ground. Penalize burden when it delays cheap validation, creates continuing manual cost, or requires external permission. Score implementation burden, continuing manual burden, scalability, and unit economics separately; use a red line only for the enumerated conditions.

## Gate observables and ownership

| Gate | Primary / support | A: minimum affirmative pass | B: affirmative fail | C trigger |
|---|---|---|---|---|
| G1 dataset | technical / risk, customer | Enumerated inputs; owner-authorized access; no mandatory paid/licensed/privileged source | A required field is available only through a paid/licensed third-party source or unauthorized/privileged access | Export exists but exact fields or rights remain unknown |
| G2 integration/scraping | technical / competitor | Stepwise first-value flow uses files/manual input without API, credentials, scraping, or automated recovery | A required step cannot work without one of those mechanisms | File route exists but completeness/joins are unknown; route those issues to G5 |
| G3 liability | risk / technical, customer | Declared jurisdiction and action/decision/data-role inventory establish no regulated/high-liability judgment | Core output necessarily decides safety, eligibility, compliance, finance, or similar high-liability matter | Boundary or authoritative basis is unvalidated |
| G4 procurement | customer / market, risk | Named payer has authority and documented direct/private route below formal thresholds | First pilot necessarily uses public/enterprise procurement | Organization exists but authority or payment route is unknown |
| G5 manual MVP | technical / customer, risk | Input/output SOP, bounded cap, representative authorized or synthetic timed packet, error/escalation criteria, founder hours, repeatability | Packet exceeds cap, is unreliable, or requires unavailable expertise | Workflow is plausible but lacks representative time-and-motion evidence |
| G6 channel | customer / market | Named buyer role, named channel, permission rule, and repeatable identification/reach route | Channel rules bar access or named buyer is unreachable there | Directory/community exists but authority, permission, or conversion route is unknown |
| G7 recurrence | customer / market | Segment-specific pain occurs at least monthly with observable metric and baseline | Credible evidence shows cadence below monthly or pain is not measurable | Category/report cadence or aggregate harm only |
| G8 paid pilot | customer / market, competitor | Fixed buyer, scope, duration, price, success metric, and actual payment or credible documented commitment | Qualified buyers reject the bounded offer/price or substitutes remove the paid job | Wedge or competitor price exists without WTP/commitment |

No generic lane owns every gate. The compiler owns consistency and provenance, not evidence acquisition or invention of criteria.

## Evidence and authorization rules

Use accepted raw sources and bounded inferences. A citation must identify an accepted artifact and source locator. Worker intent and unsupported summaries are not market evidence. Public competitor pricing is context, not WTP. A directory is not reachability. Platform cadence is not buyer-pain recurrence. Small organizations are not automatically procurement-free.

When outreach, a paid pilot, private data, counsel, or another nonpublic action is not authorized, record the relevant result as C and stop that path. Never simulate a commitment or convert inability to acquire evidence into B.

## Fixed scoring and threshold

Score each dimension 0–5 with evidence citations: pain magnitude/recurrence 20%, evidence strength 20%, channel accessibility 15%, pilot clarity 15%, competitive stagnation/whitespace 10%, implementation burden 10%, and continuing manual burden 10% (higher burden scores mean lighter burden). Normalize to 100 and preserve raw values. Set the frozen advancement threshold in `run-spec.json` before research; default to 65/100, G6/G7/G8 all A, and no E. A declared policy may require more A states. Unknown may block selection but never counts as affirmative disconfirmation. Scores rank only candidates without a hard red line and never override one.

## Barriers, transitions, and bounds

Artifact acceptance requires terminal success, exit code zero, required nonempty files, schema validity, and resolvable citations. Retry once only for execution/artifact/schema/citation failure. A valid unsuccessful evidence search remains C and receives no retry for favorability.

The independent review must contain exactly G1–G8. Allow transitions only when a later accepted artifact directly supports the new state; record prior state, new state, artifact, proposition, and rationale. Never transition C/D to B solely because the budget ended. Use E for inconsistent standards until reconciled.

Run no more than four targeted jobs per finalist in round 1 and two in round 2. Round 2 is only for newly exposed dependencies or repairable E states. Stop after round 2, on decisive red-line B, when unresolved evidence requires unauthorized action, or when evidence cannot change the decision.

## Decision vocabulary

- `advance`: no hard-red-line B; selection-required evidence meets the frozen threshold; a standalone brief is complete.
- `evidence-insufficient`: no decisive B, but required A evidence is C/D/E or validation is incomplete.
- `reject`: accepted B establishes a frozen hard red line, or the candidate falls below the frozen scored threshold after adequate evidence.

Always emit the evidence vector independently of the decision.
