# Kinds of work you can request through demesne

Ask your agent to run any of the pipeline types below. Each section shows what it's for and how to phrase the request.

## One-off scripts

Run a single shell command or script in a fresh, isolated container — no state carries between runs. Reach for this when you want a quick build step, test run, or file transformation without any persistent setup.

> "Run this Python script against the files in `/workspace/data` and put the output in `/out/results.csv`."

See the [hello-script example](../../examples/hello-script/) for a walkthrough.

## Long-running research

Spawn a research agent with unrestricted internet access to gather information, summarise sources, or investigate a topic. Unlike `sandbox_agent`, the research agent has no access to your input files — it's for open-ended web research only.

> "Research the current state of X and write a structured summary to `/out/research.md`."

See the [`sandbox_research` tool reference](../reference/tools/sandbox_research.md) for parameters and egress details.

## Delegated agent tasks

Hand off a multi-step coding or analysis task to a containerised sub-agent with its own inputs and scratch workspace. Use when the work needs its own reasoning loop — reading, planning, writing, checking.

> "Analyse the files in `/in/repo` and write a refactoring plan to `/out/plan.md`."

See the [sandbox-agent-hello example](../../examples/sandbox-agent-hello/) for a minimal walkthrough.

## Persistent sessions

Create a sandbox, run several commands against it (state accumulates between commands), then destroy it. Use when a sequence of steps must share a filesystem — for example, installing dependencies and then running a script that uses them.

> "Create a Python sandbox, install the packages in `requirements.txt`, run `main.py`, and save the output."

See the [persistent-session example](../../examples/persistent-session/) for the full create/exec/destroy cycle.

## Multi-agent orchestration

Fan out work across several child agents, or use a verifier/judge pattern to have one agent check another's output. Reach for this when a task is large enough to benefit from parallel workers, or when you want an independent quality check.

> "Have one agent write the report and a second agent review it for accuracy. Return PASS or FAIL."

See the [sandbox-agent-verifier example](../../examples/sandbox-agent-verifier/) for a working verifier setup.

---

Composing larger pipelines as repeatable agent skills? See [Develop demesne skills](develop-demesne-skills.md).
