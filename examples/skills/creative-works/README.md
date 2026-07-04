# Creative works

Build a durable, bespoke creative artifact through a demesne pipeline and deliver the editable project plus a runnable or consumable build — a game to play today, and room for other creative deliverables (interactive narrative, generated media, and the like) as this family grows. The output here is something to *experience*, not a codebase change, which is why it lives apart from `build-and-land/`. The orchestrator commits the project to a branch in `/out/repo`; the host lands it.

So far this covers playable games:

| Skill | What it does |
|-------|--------------|
| [`sandbox-make-ts-game`](sandbox-make-ts-game/) | Build a real coded TypeScript game (one-mechanic toy up to a multi-system game): research → game-design spec + system decomposition → spec-driven vertical-slice phases, each build + scenario-playtest gated → whole-game cohesion pass → deliver the editable project + runnable dist. |
| [`sandbox-make-twine-game`](sandbox-make-twine-game/) | Build a playable branching interactive-fiction (Twine) game: plan the passage graph → write Twee 3 source → compile with Tweego → offline playtest gate that walks the whole link graph → story-quality improvement cycle → deliver the editable `.twee` source + published HTML. |

See the [top-level skills README](../README.md) for the shared frontmatter format, symlinking, and the concurrent fan-out loop.
