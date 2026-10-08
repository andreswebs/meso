---
id: mes-5e63
status: closed
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

## Notes

**2026-10-08T03:02:33Z**

Done. `make mutation` (gremlins v0.6.0, threshold 85 unchanged) run from a clean `git archive HEAD` export in a scratch dir with MUTATION_WORKERS=4 and TMPDIR outside it, to keep the footprint small (~10 MB tree, ~110 MB temp, vs ~8 GB per copy with verification/lean/.lake): 30m53s, exit 0.

Result: efficacy 88.66% (383 killed / 432 killed+lived; 15 timed out; spec 001 was 89.04%). Mutator coverage 54.89% is misleading: 343 of 355 NOT COVERED are in verification/reference/directed-scout/main.go (a reference tool inside the module); excluding it coverage is ~97%. Raised for discussion, Makefile unchanged.

Spec 002 survivors: two dead guards removed (centrality.go `dist[v] >= 0` in the accumulation; subgraph.go `w > 0` before copying a self-loop), the remaining subgraph.go dedup flips are equivalent (documented). The eight accessors.go NOT COVERED mutants sit on `switch` case conditions, which Go's coverage profile assigns to no block; applied by hand, all eight are killed by the accessor tests. Other survivors are spec 001 code already classified. `.gremlins.yaml` score comment updated; full triage in docs/specs/learnings.md. Not re-run after removing the guards (30 min); the removal only drops two equivalent mutants. `make validate` green.
