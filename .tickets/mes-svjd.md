---
id: mes-svjd
status: closed
deps: []
links: []
created: 2026-07-14T03:40:29Z
type: epic
priority: 2
assignee: Andre Silva
tags: [milestone, parallel]
---

# M4: Deterministic parallelism (synchronous-round local moving)

Milestone 4 of `docs/meso-design.md` sections 4.5 and 10, step 9 of `docs/specs/001-initial-implementation/plan.md`. Synchronous-round fast-local-move with snapshot decisions applied in deterministic node order, byte-identical across core counts. Design half is already proved in Lean (`verification/lean/Meso/Round.lean`); this milestone builds the code half.
