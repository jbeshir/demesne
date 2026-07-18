---
name: sandbox-problem-discovery
description: Explore and map a customer problem or product idea, compare it with existing reports, and write a concise evidence-based Markdown report for a human reader. Use for founder problem discovery, opportunity mapping, early concept research, competitor and substitute analysis, or deciding which customer/problem hypothesis deserves later testing. Do not use to validate or stress-test one already-defined hypothesis.
---

# Sandbox Problem Discovery

Produce a useful decision document, not an audit package. Scale the work to the decision: a narrow question may need a short desk review; an unfamiliar or consequential market may justify broad, parallel research. Use subagents or research jobs when independent evidence lanes can be investigated faster or more thoroughly in parallel. Keep each assignment focused, share enough context to make its findings comparable, and synthesize all accepted findings into one report.

## Keep discovery distinct from hypothesis testing

Use discovery to determine what opportunity is present: identify plausible customers and payers, understand their workflows and pain, map alternatives and demand, compare possible framings, and decide what appears promising enough to investigate next. Preserve competing interpretations until the evidence makes one more useful.

Do not freeze one proposition in advance, organize every search around proving or disproving it, run an adversarial disconfirmation protocol, or claim that desk research validates a customer-problem hypothesis. If the input is already a precise falsifiable statement and the user wants to know whether it survives contrary evidence, use `sandbox-hypothesis-stress-test` instead. Discovery may recommend a specific hypothesis for that later step, but must not perform it implicitly.

## Start with what already exists

Read the repository guidance, catalog if present, and the most relevant canonical reports. Compare the idea by audience, painful workflow, concept, and alternatives. Update the existing report when it is substantially the same idea; create a new descriptively named report only when it is meaningfully distinct.

## Research for the decision

Clarify the audience, payer, recurring workflow, pain, current workaround, and concept being evaluated. Name direct competitors and practical substitutes. Use normal inline citations or footnotes to public sources and distinguish:

- observable demand, such as recurring behaviour, adoption, searches, reviews, or paid adjacent products;
- exact willingness to pay for this concept, which requires stronger evidence such as purchases, renewals, paid pilots, or buyer commitments.

Choose evidence lanes that fit the question rather than following a fixed checklist. Typical lanes include customer pain and workflow, market or behavioural demand, competitors and substitutes, pricing and willingness to pay, acquisition channels, and technical or regulatory constraints. Investigate the lanes actively using current public sources and any relevant mounted material. For a broad idea, assign independent lanes or genuinely different customer hypotheses in parallel; for a narrow idea, combine related lanes. Ask researchers to return concrete findings, sources, contradictions, and searches that found no useful evidence—not compliance artifacts.

Before writing, confirm that customer and workflow, competitors and substitutes, demand and willingness to pay, and acquisition channels have each been actively investigated. Use parallel lanes when two or more remain substantial. If a lane cannot be investigated, explain why under recommended next research rather than silently omitting it.

Reconcile overlapping findings and investigate material conflicts before writing. Synthesize what the sources support and make bounded judgments where evidence is incomplete. State an important unknown once, plainly, where it affects the decision; never use uncertainty as a reason to avoid searching for an answer. Stop when additional desk research is unlikely to materially improve the opportunity map or choice of promising framing. Put questions that require disconfirmation, interviews, experiments, or paid pilots into next research.

## Write one canonical report

Use a descriptive Markdown filename and a clear title. Cover:

1. headline decision about the opportunity, its strongest framing, and what deserves further investigation—not a claim that the hypothesis is validated;
2. target audience;
3. pain point and current workflow;
4. concept being evaluated;
5. named competitors and substitutes, including relevant features and pricing where supported;
6. demand and exact willingness to pay, kept distinct;
7. plausible acquisition channels;
8. differentiation opportunities;
9. risks and open questions;
10. recommended next research;
11. sources.

Prefer concrete prose and compact tables where comparisons benefit from them. If the repository uses a catalog, add a small descriptive entry with identity, title, decision, target customer, report path, date, keywords, and named competitors. Check JSON syntax and local links, but do not machine-grade research judgments.

## Keep the workflow human-first

Do not create gates, attestations, ledgers, immutable bundles, hash manifests, generated candidate IDs, fixed territory counts, compulsory review rounds, evidence-state matrices, or machine validation of research conclusions. Do not preserve process debris merely to prove that work happened. Avoid disclaimer-heavy prose and repetitive statements of the same limitation.
