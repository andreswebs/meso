---
id: mes-ntmq
status: closed
deps: [mes-klzw]
links: []
created: 2026-07-17T02:40:47Z
type: task
priority: 2
assignee: Andre Silva
parent: mes-dvte
tags: [verification, lean, phase-f]
---
# F1. Computable rational mirror

In `Meso/Compute.lean`: `WeightedGraphQ` with rational weights; `modularityQ`, `cpmQ`, `cpmCanonicalQ`, and the move-deltas, all computable (plain defs, not noncomputable), so `lake build` compiling them is the proof they evaluate.
