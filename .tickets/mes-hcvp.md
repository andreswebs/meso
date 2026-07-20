---
id: mes-hcvp
status: closed
deps: [mes-vy3a, mes-l38o, mes-w60v]
links: []
created: 2026-07-14T03:40:30Z
type: task
priority: 1
assignee: Andre Silva
parent: mes-t76u
tags: [core, louvain, step-4, verified]
---

# Louvain serial: local-move sweep + aggregation loop, monotone and terminating

Serial Louvain: repeated local-move sweeps to a fixed point, then aggregation, recursing until no level changes - no refinement. The internal differential baseline for Leiden and a useful algorithm in its own right. Design of record: `docs/meso-design.md` section 4.2; step 4 of `docs/specs/001-initial-implementation/plan.md`.

## Design

Sweep every node once (canonical order), applying the best move via the incremental delta (`DMOD`/`DCPM`); repeat sweeps until a pass moves nothing (`IsLocalMoveStable`). Then aggregate (`AGG`) and recurse from the sweep on the aggregate; stop when a level moves nothing - the discrete-partition fixed point. This is the guarded outer loop of `CORRESPONDENCE.md` section 4: implement the `RunningLevelStep` guard (continue only while `numComm` dropped / a node moved) to inherit termination. Verification originals: `Meso/Move.lean` (`localMoveRun_monotone`), `Meso/Level.lean` (`quality_monotone_of_stepwise`, `le_modularity_aggregate_run`, `modularity_le_of_algorithmRun`, cpm variants), `Meso/Termination.lean` (`no_infinite_acceptedMove_run`, `no_infinite_descending_levels`, `levelStep_size_lt_or_injective`, `levelRun_reaches_fixedPoint`, `RunningLevelStep.size_lt`, `no_infinite_runningLevel_run`), `Meso/Convergence.lean` (`IsLocalMove.quality_eq_of_stable`, `IsLocalMoveStable.no_strict_improvement`).

## Acceptance Criteria

TDD order. 1) `TestLocalMove_SweepMonotone`: a full sweep never lowers modularity (`localMoveRun_monotone`); `TestCPM_SweepMonotone` for CPM. 2) `TestLeiden_QualityMonotoneAcrossLevels`: quality is non-decreasing across aggregation levels (`quality_monotone_of_stepwise`). 3) `TestLeiden_LevelNonDecreasing` / `TestLeiden_CPMLevelNonDecreasing`: one level does not lose quality across the aggregation boundary (`le_modularity_aggregate_run` / `le_cpm_aggregate_run`). 4) `TestLeiden_QualityMonotoneWholeRun` / `TestLeiden_CPMMonotoneWholeRun`: the whole multilevel run never lowers quality (`modularity_le_of_algorithmRun` / `cpm_le_of_algorithmRun`). 5) `TestLocalMove_TerminatesBounded`: no infinite strictly-improving sweep (`no_infinite_acceptedMove_run`). 6) `TestLeiden_LevelsTerminate` / `TestLeiden_LevelSizeDichotomy` / `TestLeiden_LevelsReachFixedPoint` / `TestLeiden_RunningLevelShrinks` / `TestLeiden_GuardedLevelsTerminate`: the guarded outer loop shrinks the node count each running level and executes at most n levels (the `Termination.lean` cluster; see `CORRESPONDENCE.md` section 4). 7) `TestConverge_StableIsMoveFixedPoint` / `TestConverge_StableNoImprovement`: a stable partition is a genuine move fixed point (`Convergence.lean` adequacy). 8) Louvain on the karate corpus yields the expected community-count / partition shape. `make validate` and `make test-race` green.

## Notes

**2026-07-17T20:57:28Z**

Serial Louvain implemented in louvain.go: objective interface (unifies modularity/cpm Quality+moveDelta), localMoveSweep/localMoveToStable (monotone fast local move), and louvainTrace (guarded aggregation loop) + louvainLevels/louvain. Deterministic without the PRNG (mes-z0pd): ascending node order, neighbour communities sorted, ties to smallest label, canonical aggregate. Local move considers stay + neighbour communities + isolation (fresh label via a growing counter); isolation makes stability exhaustive up to sign, so a neighbour+isolate-stable partition is IsLocalMoveStable over ALL targets. Move accepted only when delta > moveImproveEps=1e-12 (above float noise, below real gaps) -> no float-noise oscillation, termination inherited. Level guard = numCommunities(p) < h.numNodes() (negation of RunningLevelStep), so the loop stops at the discrete fixed point in <= n levels. Karate (gamma=1) -> modularity 0.4198, 4 communities (known optimum). All 16 named acceptance tests pass; make validate + test-race green. Added a minimal GML reader as a test helper (full golden-corpus loader is mes-5wqp). moveDelta stays an unexported method reached via the objective interface, as mes-vy3a's note anticipated.
