---
id: mes-1lg6
status: closed
deps: [mes-hcvp]
links: []
created: 2026-07-14T03:40:30Z
type: task
priority: 1
assignee: Andre Silva
parent: mes-mklx
tags: [leiden, local-move, step-6]
---

# Leiden fast local-move phase: queue-driven with neighbor re-enqueue

The fast local-move phase of Leiden: a queue (not a full repeated sweep). Pop a node, move it to the best-gain neighboring community; if it moved, re-enqueue its neighbors not already queued. Repeat until the queue drains. Faster convergence than Louvain sweeps, same monotonicity. Design of record: `docs/meso-design.md` section 4.1 phase 1; step 6 of `docs/specs/001-initial-implementation/plan.md`.

## Design

Reuse the incremental deltas (`DMOD`/`DCPM`) and the canonical order/PRNG (`DET`). Maintain a FIFO queue and an in-queue bit set to avoid duplicates. A move re-enqueues only neighbors not currently queued. Termination and monotonicity are the same guarantees as the Louvain sweep (`verification/lean/Meso/Move.lean` `localMoveRun_monotone`, `Meso/Termination.lean` `no_infinite_acceptedMove_run`) since the queue only reorders accepted moves. This replaces the Louvain sweep as phase 1 of Leiden; Louvain keeps its own sweep.

## Acceptance Criteria

TDD order. 1) A queued run reaches the same local-move fixed point as an exhaustive sweep on fixtures (both stop at `IsLocalMoveStable`). 2) Monotone: no popped-and-applied move lowers quality (reuses `TestLocalMove_SweepMonotone` reasoning over the queue). 3) A move re-enqueues exactly the un-queued neighbors; a node already in the queue is not duplicated. 4) The queue drains (termination) within a bounded number of pops. 5) Result is byte-identical across runs at a fixed seed. `make validate` and `make test-race` green.

## Notes

**2026-07-17T21:34:26Z**

Implemented queue-driven fast local-move phase in leiden.go (new file), with leiden_test.go.

Design: nodeQueue is a FIFO with an in-queue bit set (push dedups, pop clears the mark). localMoveDrain runs one neighbour-re-enqueue pass; localMoveQueue drains to a fixed point. It is the queue-based drop-in for Louvain's localMoveToStable and reaches the same IsLocalMoveStable stopping condition. Louvain keeps its own sweep, unchanged.

KEY DIVERGENCE from the literal design text (docs/meso-design.md 4.1 phase 1 says a single drain): a single neighbour-re-enqueue drain does NOT reach IsLocalMoveStable for modularity/CPM. A node's gain can shift purely through a community aggregate (K_c / S_c) when a NON-adjacent node joins/leaves that community; neighbour re-enqueue never revisits it. Confirmed empirically: residual missed moves of delta up to ~2.4 on lesmis CPM and ~0.94 on fuzzed CPM. Fix: outer loop drains until a whole drain moves nothing (a clean drain = one no-move exhaustive sweep = IsLocalMoveStable). Each drain is still the design's neighbour-re-enqueue queue; the outer loop is what the AC's 'both stop at IsLocalMoveStable' requires. Monotone + terminating (bounded by quality strictly increasing above moveImproveEps per accepted move) + fully deterministic (no PRNG; canonical order). make validate + make test-race green.

Not yet wired into a public Leiden() (mes-jbc7) or refinement (mes-w7ko).
