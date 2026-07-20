---
id: mes-kvci
status: closed
deps: [mes-5smr, mes-ntmq]
links: []
created: 2026-07-17T02:40:47Z
type: task
priority: 2
assignee: Andre Silva
parent: mes-dvte
tags: [verification, lean, phase-g]
---
# G3. Computable subset-optimality mirror

In `Meso/Predicates.lean`: `SubsetOptimalQ` deciding `IsSubsetOptimal` by enumerating each community's subsets, proved equal to `IsSubsetOptimal`. Exponential in community size, so the emitter gates it to small fixtures.
