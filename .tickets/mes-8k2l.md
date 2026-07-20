---
id: mes-8k2l
status: closed
deps: []
links: []
created: 2026-07-17T02:40:47Z
type: task
priority: 2
assignee: Andre Silva
parent: mes-dvte
tags: [verification, lean, phase-a]
---
# A1. Refinement operator + connectivity guarantee

`RefineStep` models the refinement phase (singletons, merges under the gate's shared-edge consequence, within one outer community); `refineRun_isMergeRun` shows its output is a valid `MergeStep` run, and `connectedCommunities_of_refineRun` gives connectivity end to end. In `Meso/Refinement.lean`, `Meso/Connectivity.lean`.
