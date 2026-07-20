---
id: mes-l38o
status: closed
deps: [mes-qvav, mes-vy3a]
links: []
created: 2026-07-14T03:40:30Z
type: task
priority: 1
assignee: Andre Silva
parent: mes-t76u
tags: [core, move-delta, cpm, step-3, verified]
---

# Incremental move-delta for CPM + property test; CPM merge-gain formula

The incremental CPM gain (its formula differs from modularity - node-size penalty, no `2m`) with its own property test, plus the two-singleton merge-gain formula used later by gamma-separation. Spec: `docs/research/move-delta-verification.md`; step 3 of `docs/specs/001-initial-implementation/plan.md`.

## Design

Implement CPM incremental gain against the from-scratch CPM oracle (QCPM ticket). One property test per objective, so this is a separate harness instance from modularity but shares the generator. Also implement the merge gain of joining two singleton communities: `2*(w_AB - gamma s_A s_B)`, the aggregate-level formula. Verification originals: `verification/lean/Meso/CPM.lean` (shared local-move interface via `Move.lean`) and `Meso/Separation.lean` (`cpm_merge_two_singletons`). CORRESPONDENCE rows: `cpm_merge_two_singletons` -> `TestCPM_MergeGain`; `cpm_localMoveRun_monotone` -> `TestCPM_SweepMonotone` (the sweep test lands with Louvain).

## Acceptance Criteria

TDD order. 1) No-op CPM move yields delta exactly 0. 2) Property test (thousands of trials, same generator as modularity: weighted, self-loops, disconnected, isolated, random gamma) asserting `abs(delta - (CPM_after - CPM_before)) <= 1e-9`, seed logged on failure. 3) Explicit cases: singleton in/out, self-loop on the moved node, empty-target move. 4) `TestCPM_MergeGain`: merging two singletons changes CPM by exactly `2*(w_AB - gamma s_A s_B)` on fixtures (Go image of `cpm_merge_two_singletons`). `make validate` and `make test-race` green.

## Notes

**2026-07-17T20:35:53Z**

Implemented CPM incremental move-delta as (cpm).moveDelta in move.go, mirroring modularity's but with s_u*S in place of k_u*K/2m and no 2m normalisation. Key subtlety: the diagonal term B_uu = w_uu - gamma*s_u^2 is constant across a move (u always shares a community with itself), so the moved node's self-loop weight and own size-squared penalty drop out of the delta entirely; W_us/W_ut sum only over neighbours (self-loops are stored separately in csr, so they are naturally excluded). Added cpmMergeGain in cpm.go = 2*(w_ab - gamma*s_a*s_b), the Go image of Lean cpm_merge_two_singletons; it equals the general moveDelta specialised to the singleton partition (verified in TestCPM_MergeGain). Tests in cpm_test.go: TestCPM_MoveSelfIsNoOp, TestCPM_MoveRegressions (singleton in/out, self-loop node, empty target), TestCPM_MoveMatchesOracle (5000 trials, randomCSRSized for non-unit node sizes, seed logged), TestCPM_MergeGain. Tolerance cpmMoveTol=1e-9 holds comfortably (observed agreement ~1e-11 at n<=50). No twoM==0 guard needed since CPM has no 2m factor. TestCPM_SweepMonotone lands later with Louvain (mes-hcvp), which this unblocks. make validate and make test-race green.
