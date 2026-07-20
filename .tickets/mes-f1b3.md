---
id: mes-f1b3
status: closed
deps: []
links: []
created: 2026-07-17T02:40:47Z
type: task
priority: 2
assignee: Andre Silva
parent: mes-dvte
tags: [verification, lean, phase-d]
---
# D1. Synchronous-round confluence (Lean)

In `Meso/Round.lean`: `applyRound` folds the snapshot-decided moves over a work-list; `applyRound_perm` proves the outcome is schedule- and core-count-independent. Proves determinism, not quality improvement.
