---
id: mes-vy3a
status: closed
deps: [mes-orqz]
links: [mes-5wqp, mes-nqky, mes-wmzq]
created: 2026-07-14T03:40:29Z
type: task
priority: 1
assignee: Andre Silva
parent: mes-t76u
tags: [core, move-delta, modularity, step-3, verified]
---

# Incremental move-delta for modularity + property test vs oracle

The incremental gain of moving a node into a candidate community for modularity, and the property test proving `delta == Q(after) - Q(before)`. This is the single most bug-prone line in any modularity optimizer and gates all optimizer work. Full spec: `docs/research/move-delta-verification.md`; step 3 of `docs/specs/001-initial-implementation/plan.md`.

## Design

Implement the incremental modularity gain (remove node from current community, add to target; handle the null-model term `k_i * k_target / 2m` and the self-loop of the moved node exactly). The from-scratch modularity evaluator from the QMOD ticket is the oracle. Verification originals: `verification/lean/Meso/Move.lean` (`move_self`, `move_best_ge`, `IsLocalMove`) and the move-delta theorem target described in `docs/research/move-delta-verification.md`. CORRESPONDENCE rows: `move_self` -> `TestLocalMove_SelfIsNoOp`, `modularity_bestMove_ge` -> `TestLocalMove_BestMoveNonDecreasing`.

## Acceptance Criteria

TDD order following `docs/research/move-delta-verification.md`. 1) `TestLocalMove_SelfIsNoOp`: a no-op move (target == current community) yields delta exactly 0 (`move_self`). 2) Property test: thousands of random trials - n in `2..50`, random weighted edges, self-loops, disconnected parts, isolated nodes, random start partition, random legal move, random gamma above and below 1 - assert `abs(delta - (Q_after - Q_before)) <= 1e-9`, seed logged on failure. 3) Explicit regression cases: singleton in and out; moved node carrying a self-loop; empty-target move (node leaves to form its own singleton). 4) `TestLocalMove_BestMoveNonDecreasing`: the best available single-node move never lowers modularity, staying put being an option (`modularity_bestMove_ge`). `make validate` and `make test-race` green.

## Notes

**2026-07-17T02:29:41Z**

Exact move-delta oracle values now committed (2026-07-16, Lean Phase F). Beyond the internal
from-scratch Go evaluator (this ticket's stated oracle), each golden case emits per-move
"deltas" with the exact rational `moveDeltaModularityQ` (`Meso/Compute.lean`), proved equal to
`Q(after)-Q(before)` via `moveDeltaModularityQ_eq`. So the property test can additionally
cross-check the Go incremental delta against the committed Lean exact delta within a
float-rounding tolerance, tightening the oracle from "another Go computation" to "the proved
model's exact value". Oracle side done; only the Go core blocks.

**2026-07-17T20:26:39Z**

Implemented modularity move-delta in move.go: unexported method (modularity).moveDelta(g,p,u,target) plus a package-level move() partition-copy helper (Go image of Lean's move). Formula derived in the doc comment: (2/2m)*[(W_ut - W_us) - gamma*k_u*(K_t - (K_s - k_u))/2m]; the moved node's self-loop enters only via k_u (the u==u double-sum term is constant and cancels), off-diagonal edge terms exclude j==u since self-loops live off the neighbour list. No-op and 2m==0 both return 0. Oracle is the from-scratch (modularity).Quality per the ticket; move_test.go covers TestLocalMove_SelfIsNoOp (move_self), explicit regressions (singleton in/out, self-loop node, empty-target), a 5000-trial property test vs Q(after)-Q(before) at random gamma in [0.3,1.8] (seed logged on failure), and TestLocalMove_BestMoveNonDecreasing (modularity_bestMove_ge). Deliberately NOT added to the QualityFunction interface yet: CPM's Quality (mes-qvav) lands before CPM's delta (mes-l38o), so an interface method would break qvav's build; the Louvain ticket (mes-hcvp) can unify the deltas behind the interface once both exist. Lean golden-vector cross-check left to mes-nqky. make validate + make test-race green.
