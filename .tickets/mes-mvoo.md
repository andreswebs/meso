---
id: mes-mvoo
status: closed
deps: [mes-8k2l, mes-ntmq]
links: []
created: 2026-07-17T02:40:47Z
type: task
priority: 2
assignee: Andre Silva
parent: mes-dvte
tags: [verification, lean, phase-g]
---
# G1. Computable connectivity mirror

In `Meso/Predicates.lean` and `Meso/Reachability.lean`: an efficient reachable-set closure decides community connectivity, proved to decide `ConnectedCommunities` (`connectedCommunitiesFast_iff` via the reachable-set fixpoint); runs at corpus scale where Mathlib's walk-enumeration decision cannot.
