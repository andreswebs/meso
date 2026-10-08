---
id: mes-5e63
status: open
deps: [mes-ojdd, mes-8x09, mes-6mfk, mes-ar6b, mes-slkl]
links: []
created: 2026-10-07T16:34:16Z
type: task
priority: 3
assignee: Andre Silva
parent: mes-0jzi
tags: [mutation, step-7, implementation]
---
# Mutation gate over the spec 002 code, survivors triaged

Step 7 (mutation part) of `docs/specs/002-structural-measures/plan.md`, parent mes-0jzi. Prove the new tests kill injected bugs in the new code.

## Evidence (verified 2026-10-07)

- `make mutation` runs gremlins over the core and fails at or below the threshold in `.gremlins.yaml` (the gremlins CLI threshold flags are silently ignored, so the file is the only gate).
- `.gremlins.yaml` records the spec 001 score: 89.04% efficacy (390 killed of 438 killed plus survived).
- Survivors from spec 001 are triaged in `docs/specs/learnings.md`.

## Steps

1) Run `make mutation`. 2) For each survivor in the files touched by spec 002 (`builder.go` zero-weight rule, accessors, `Cohesion`, `Subgraph`, `centrality.go`), either add a killing test to the owning area or document it as an equivalent mutant in `docs/specs/learnings.md` under a spec 002 heading. 3) Keep the threshold unchanged unless the owner agrees; update the score comment in `.gremlins.yaml`.

## Acceptance Criteria

`make mutation` passes the committed threshold; every survivor in spec 002 files is either killed or triaged in `docs/specs/learnings.md`; the close note records the new efficacy and counts. `markdownlint-cli2` clean on `learnings.md`. `make validate` green in both modules (fmt-check, vet, lint, test).
