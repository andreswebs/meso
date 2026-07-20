---
id: mes-aueb
status: closed
deps: [mes-y8ru, mes-wmzq]
links: []
created: 2026-07-14T03:40:30Z
type: task
priority: 3
assignee: Andre Silva
parent: mes-45a8
tags: [depth, mutation, step-12]
---

# Mutation testing: prove the suite kills injected bugs

Mutation testing on the core to prove the test suite actually kills injected bugs, targeting a high mutation score; surviving mutants drive new tests back into the relevant ticket. Design of record: `docs/meso-design.md` section 6.3; step 12 of `docs/specs/001-initial-implementation/plan.md`.

## Design

Run a Go mutation-testing tool over the core packages; triage survivors. A surviving mutant means a missing test - add it to whichever earlier ticket owns that behavior (move-delta, aggregation, refinement, etc.). Gate a minimum mutation score. This depends on a substantial suite already existing (FUZZ + LINV + the per-ticket tests).

## Acceptance Criteria

TDD/verify order. 1) The mutation harness runs over the core and reports a score. 2) The score meets the chosen threshold. 3) Each surviving mutant is either killed by a new test (added to the owning ticket) or explicitly justified as equivalent. `make validate` green.

## Notes

**2026-07-18T04:17:39Z**

Mutation testing wired up via go-gremlins v0.6.0. New on-demand 'make mutation' target (not in validate, like fuzz/bench) runs 'go run gremlins@VERSION unleash .' in isolated module-aware mode, so the dependency-free core go.mod/go.sum is untouched (same pattern as benchstat in bench-compare). Achieved efficacy 89.04% (390 killed / 438 killed+survived, 99.10% mutator coverage); the committed 'make mutation' run exits 0 against the gate.

The efficacy gate lives in .gremlins.yaml (threshold.efficacy: 85), NOT on the CLI: gremlins v0.6.0 has a bug where --threshold-* flags are parsed as strings and silently ignored, so only the config-file threshold fires. Threshold set a few points below achieved so a new equivalent mutant or a load timeout can't flip it red.

Real gaps found and fixed with tests in the owning files: builder.go '!(w >= 0)' rejection only tested negatives, so the >=/> boundary that would reject a legal zero weight survived -> TestBuilder_ZeroWeightsAccepted. parallel.go normalizeWorkers (worker clamp) and isolationBase (per-round fresh label) are pure helpers invisible to the core-count-invariance tests (a wrong clamp still yields byte-identical output) -> TestParallel_NormalizeWorkers, TestParallel_IsolationBase. The 48 remaining survivors are documented equivalent/defensive mutants in docs/specs/learnings.md (epsilon boundaries, order-invariant sort comparators, error-message-only arithmetic, validator boundaries unreachable for valid graphs, canonical-partition-unreachable fresh-label branches, and worker-count-invariant parallel mechanics).

Operational gotchas (in learnings.md): gremlins copies the whole module tree per worker, so the gitignored multi-GB verification/lean/.lake bloats copies and can exhaust /tmp (MUTATION_WORKERS defaults to 1; TMPDIR must be outside the repo). Per-mutant timeout is (2s+measured-suite)*coeff; the baseline is only trustworthy on a cold test cache, so MUTATION_TIMEOUT_COEFF defaults to 8. Infinite-loop mutants (i++->i-- on loop counters) are reported TIMED OUT (9 of them, effectively killed, not counted in efficacy).
