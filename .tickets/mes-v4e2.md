---
id: mes-v4e2
status: closed
deps: [mes-pmdh, mes-r8p7]
links: []
created: 2026-07-17T02:40:47Z
type: task
priority: 2
assignee: Andre Silva
parent: mes-dvte
tags: [verification, lean, phase-e]
---
# E2. Derive the level measure (whole-algorithm termination)

In `Meso/Termination.lean`: `levelStep_size_lt_or_injective` derives the halting dichotomy (a level strictly lowers `numComm` or its partition is discrete), and `RunningLevelStep` / `no_infinite_runningLevel_run` prove the guarded loop halts outright.
