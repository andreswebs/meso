---
id: mes-ctmy
status: closed
deps: [mes-rogj]
links: []
created: 2026-10-08T03:49:46Z
type: chore
priority: 2
assignee: Andre Silva
parent: mes-0jzi
tags: [mutation, make, step-10, implementation]
---
# Mutation target on a clean export, excluding `verification/`, 4 workers

Step 10 of `docs/specs/002-structural-measures/plan.md`, parent `mes-0jzi`. Make `make mutation` cheap and its coverage figure honest. Decided with the plan owner on 2026-10-08.

## Evidence (verified 2026-10-08)

- The target in `Makefile`: `$(GO) run github.com/go-gremlins/gremlins/cmd/gremlins@$(GREMLINS_VERSION) unleash . --workers $(MUTATION_WORKERS) --timeout-coefficient $(MUTATION_TIMEOUT_COEFF) -E 'gonum/'`; `MUTATION_WORKERS ?= 1`.
- gremlins copies the module tree per worker. A working tree with a built `verification/lean/.lake` is about 8 GB; a `git archive HEAD` export is about 10 MB. The spec 002 run (mes-5e63) used such an export with 4 workers: 30m53s, about 110 MB of temp, 88.66% efficacy.
- 343 of that run's 355 NOT COVERED mutants were in `verification/reference/directed-scout/main.go`, a reference tool inside the module, so mutator coverage read 54.89% instead of about 97%.
- `.gremlins.yaml` holds the efficacy threshold (85); gremlins v0.6.0 ignores the CLI threshold flags (see its header comment and `docs/specs/learnings.md`).

## Design

The target exports `HEAD` (`git archive HEAD | tar -x -C <tmp>/tree`) into a fresh temp directory outside the repository, sets `TMPDIR` for gremlins to a sibling temp directory (pointing it inside the tree makes gremlins copy the tree into itself), runs gremlins there with `-E` excluding both `gonum/` and `verification/`, propagates gremlins' exit status, and removes the temp directory. Default `MUTATION_WORKERS ?= 4`. The target comment states that only committed code is mutated.

## Warnings

- Check how gremlins v0.6.0 takes several exclusions (one regex such as `'gonum/|verification/'`, or a repeated `-E`) and confirm from the run output that no `verification/` file is mutated.
- `go run pkg@version` must keep not touching the core `go.mod`/`go.sum` (verify byte-identical after a run, as spec 001 did).
- This ticket depends on the Level hierarchy ticket so its confirmation run covers that code; commit that work first, since the export only sees `HEAD`.

## Acceptance Criteria

1) `make mutation` passes the committed threshold from a working tree that has `.lake`, without copying it (temp footprint in the low hundreds of MB; record it). 2) No `verification/` mutants in the output; record mutator coverage and efficacy. 3) Survivors in the level-hierarchy code are killed or triaged in `docs/specs/learnings.md`. 4) `.gremlins.yaml` score comment and the Makefile comment block above the target updated. 5) `make validate` green.

## Notes

**2026-10-08T04:34:47Z**

Done. Makefile `mutation` target now: warns if tracked Go files have uncommitted changes (only HEAD is mutated); exports HEAD with `git archive` into `mktemp -d` under the system temp dir; runs gremlins from the export with its TMPDIR in a sibling dir (never inside the tree); excludes `-E 'gonum/' -E 'verification/'` (gremlins v0.6.0 takes a repeatable string array); removes the work dir via `trap` on exit; exit status propagates. `MUTATION_WORKERS ?= 4`; the comment block above the target explains the export and exclusions.

Confirmation run from the real working tree (with its 7.9 GB .lake) at 031452b: 32m07s, working area peaked at 126 MB and was removed afterwards, exit 0. Efficacy 89.34% (394 killed, 47 lived, 16 timed out; threshold 85 unchanged), mutator coverage 97.35% (was 54.89% with verification/ counted), no verification/ paths in the output, go.mod byte-identical. Level-hierarchy code: no survivors. Remaining spec 002 items are the already-triaged subgraph.go dedup equivalents and the accessors.go switch-case coverage artifacts; leiden.go:289 is the spec 001 aggregation guard (shifted lines). `.gremlins.yaml` score comment and learnings updated. `make validate` green.
