# Survey / map-reduce over a corpus or codebase

Report-only (or a structured store) — fan a fixed set of lenses or detectors over a body of material, then reduce.

| Skill | What it does |
|-------|--------------|
| [`sandbox-code-defect-survey`](sandbox-code-defect-survey/) | Research a defect taxonomy, fan out one detector per type across the code, synthesise. |
| [`sandbox-prose-defect-survey`](sandbox-prose-defect-survey/) | The prose twin of the code survey — documentation, comments, and generated text. |
| [`sandbox-docs-quality`](sandbox-docs-quality/) | Map a fixed set of documentation-quality lenses over the docs tree. |
| [`sandbox-appearance-review`](sandbox-appearance-review/) | Render a front-end into a screenshot matrix, fan out one visual-review lens per agent, merge into tiered appearance-improvement proposals. |
| [`sandbox-corpus-map-reduce`](sandbox-corpus-map-reduce/) | Apply the same extraction/scoring op to every item in a corpus, then reduce to a ranked answer. |
| [`sandbox-etl-document`](sandbox-etl-document/) | Parse → extract → classify → validate → load unstructured documents into a structured store, with a quarantine pile. |

See the [top-level skills README](../README.md) for the shared frontmatter format, symlinking, and the concurrent fan-out loop.
