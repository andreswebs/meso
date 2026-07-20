---
id: mes-gz8f
status: closed
deps: [mes-pgah, mes-vy3a]
links: []
created: 2026-07-14T03:40:30Z
type: task
priority: 2
assignee: Andre Silva
parent: mes-q7p0
tags: [directed, move-delta, step-8, reference-tested]
---

# Directed move-delta + property test against the directed oracle

The incremental gain for directed modularity (both in- and out-degree contributions change on a move) with its own property test against the directed oracle. Do not reuse the undirected oracle. Spec: `docs/research/move-delta-verification.md` directed note; step 8 of `docs/specs/001-initial-implementation/plan.md`.

## Design

Implement the directed delta from the directed definition (`k_i^out k_j^in / m`). Reuse the property-test generator from the modularity move-delta ticket but with asymmetric edge sets and the directed from-scratch evaluator (`QDIR`) as oracle. Per `docs/research/move-delta-verification.md`: the move changes both in- and out-degree contributions; the oracle must use the directed definition. No Lean original (directed descope).

## Acceptance Criteria

TDD order. 1) No-op directed move yields delta exactly 0. 2) Property test over random asymmetric graphs (self-loops, disconnected, isolated, random gamma) asserting `abs(delta - (Qdir_after - Qdir_before)) <= 1e-9`, seed logged on failure, using the directed oracle. 3) Explicit cases: singleton in/out, self-loop on the moved node, empty-target. 4) On a symmetric graph the directed delta matches the undirected delta. `make validate` and `make test-race` green.

## Notes

**2026-07-17T22:57:26Z**

Implemented directedModularity.moveDelta in directed_move.go with the Leicht-Newman directed delta. Because A_ij = w_ij - gamma*kout_i*kin_j/T is asymmetric, the move perturbs both row u (out-arcs) and column u (in-arcs); the j==u diagonal drops out so u's self-loop never enters the edge term. Formula: (1/T)[(Wout_ut-Wout_us)+(Win_ut-Win_us) - gamma*koutU*(Kin_t-Kin_{s\u})/T - gamma*kinU*(Kout_t-Kout_{s\u})/T]. Added in-arc accessors inNeighbors/inNeighborWeights to graph.go (undirected fallback to out, mirroring inDegree). Tests in directed_move_test.go: no-op(=0), property test (5000 random asymmetric graphs vs DirectedModularity oracle, seed logged, tol 1e-9), regressions (singleton join hand value 0.3125, singleton leave, self-loop-on-moved-node, empty-target), and symmetric-reduces-to-undirected. NOTE: the symmetric-equals-undirected check is to moveTol, NOT bit-identical: the two formulas are algebraically equal but sum the null term in a different order (directed does koutU*.. + kinU*.. vs undirected 2*ku*..), differing by 1 ULP. The Quality double-sum reduction stays bit-identical since its summation order is literally the same. Unblocks mes-28rk (directed local move/refine/aggregate + API). make validate and make test-race green.
