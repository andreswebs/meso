---
id: mes-orp6
status: closed
deps: []
links: [mes-niic]
created: 2026-07-19T01:00:04Z
type: feature
priority: 2
assignee: Andre Silva
tags: [leiden, quality, api, iterations, subset-optimality]
---
# Leiden: WithIterations(k) fixed-count multi-pass option

A single Leiden pass can return converged partitions that still admit a strictly improving subset split (mes-niic: 1.2-1.3% of triage samples, minimal n=4 fixture in docs/research/directed-modularity-triage.md). The paper's subset-optimality guarantee is asymptotic over repeated randomized iterations, and the reference (leidenalg) exhibits the same per-run gap. meso runs exactly one pass today, so it never gets the paper's escape mechanism: re-refining the converged communities at base granularity with fresh randomness.

Add WithIterations(k): run k full Leiden passes, each starting from the previous pass's base partition, with the refinement seed derived per pass (e.g. nodeSeed(seed, pass)). Fixed pass count only: no until-stable mode, deliberately, because stopping at the first non-improving pass inherits leidenalg's misleading n_iterations=-1 semantics (it can stop on an unlucky pass; 57/200 seeds in the mes-niic experiment). Passes are monotone in Q, so k is an honest quality/runtime dial.

## Design

Constraints agreed in mes-niic: default k=1 keeps results byte-identical to today (no golden or benchmark churn); the run stays a pure function of (graph, options, seed); Louvain ignores the option like WithSeed; validation rejects k < 1. leidenWith needs to accept an initial base partition so pass i+1 can resume from pass i's result. With an unchanged seed a repeat pass makes identical refinement draws and is a no-op by construction, hence the per-pass seed derivation. Surfacing decision (mes-niic Q4): the WithIterations godoc states the phenomenon plainly (single pass can return communities admitting an improving split, matches reference per-run behaviour, iterations reduce but never eliminate it); design doc gets the lock-in mechanism (phase-1 basin, refinement freeze, aggregation lock-in); CORRESPONDENCE divergence note plus n=4 characterization test per the Phase 4 plan, and the test must pin node registration order 0,1,2,3 since the outcome flips with interning order.

## Acceptance Criteria

WithIterations(k) public option; k=1 default byte-identical to current outputs (golden tests unchanged); k>1 deterministic across runs and worker counts; n=4 fixture characterization test pinning the stuck all-in-one at k=1 under registration order 0,1,2,3 and documenting escape behaviour at higher k; godoc surfaces the subset-optimality caveat; make validate green.


## Notes

**2026-07-19T01:10:16Z**

Implemented. WithIterations(k) public option (api.go): default 1, k < 1 rejected at resolve time, godoc carries the subset-optimality surfacing agreed in mes-niic Q4. Engine (leiden.go): leidenWith now takes the initial base partition; leidenIterated runs k passes, each restarting from the previous pass's partition; passSeed keeps pass 0 on the caller's seed (byte-compat) and derives later passes through nodeSeed, since a pass re-run with an unchanged seed would make identical refinement draws and be a guaranteed no-op. leiden/leidenParallel keep their single-pass signatures (internal tests use them); the public path routes through leidenIterated with the serial or parallel mover.

Tests (iterations_test.go, public API only): TestLeiden_IterationsCharacterization pins the mes-niic n=4 fixture at seed 0 under registration order 0,1,2,3 (single pass stuck all-in-one Q=0; seed 0 escapes to the {0,1},{2,3} split Q=1/27 at k=8, pinned as a characterization of the pass-seed derivation; quality non-decreasing over k=1..16). TestLeiden_IterationsDeterministic (repeat serial runs identical; parallel identical across 1/2/4/8 workers at k=5). TestLouvain_IgnoresIterations. TestLeiden_IterationsValidation (0 and -3 rejected, 1 accepted).

Byte-compat at default: golden and determinism suites untouched and green. Design doc section 4.1 gained the outer-iteration paragraph with the lock-in mechanism and the rationale for fixed-count-only semantics. make validate green (fmt, vet, lint, test in all modules).
