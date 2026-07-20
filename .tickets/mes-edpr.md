---
id: mes-edpr
status: closed
deps: [mes-jbc7, mes-z0pd]
links: []
created: 2026-07-14T03:40:30Z
type: task
priority: 2
assignee: Andre Silva
parent: mes-svjd
tags: [parallel, determinism, step-9, verified]
---

# Synchronous-round parallel local moving, byte-identical across cores

Parallel fast-local-move in synchronous rounds: within a round every active node computes its best move against the frozen previous-round assignment, then moves apply in deterministic node-index order. Output is identical regardless of core count. Design of record: `docs/meso-design.md` section 4.5; step 9 of `docs/specs/001-initial-implementation/plan.md`.

## Design

Snapshot semantics: each node target `t` is fixed from the round-start partition, no mid-round reads. Apply moves in node-index order. Settle the confluence mitigation (graph-coloring of independent non-adjacent nodes, or strictly-positive-gain acceptance with deterministic conflict resolution) to match the Lean design proof. Verification originals: `verification/lean/Meso/Round.lean` (`move_comm`, `applyRound`, `applyRound_eq`, `applyRound_perm`). CORRESPONDENCE rows: `applyRound_eq` -> `TestParallel_RoundClosedForm`; `applyRound_perm` -> `TestParallel_RoundCoreCountInvariant`. This is the code half of the concurrency track; the design half is already proved. (Gobra data-race proof - Lean TODO D2 - is a separate best-effort track, not this ticket.)

## Acceptance Criteria

TDD order. 1) `TestParallel_RoundClosedForm`: a round applies `t` on moved nodes and leaves `p` elsewhere - the closed form is a function of the moved-node set and the snapshot alone (`applyRound_eq`). 2) `TestParallel_RoundCoreCountInvariant`: a round outcome is independent of the order moves are applied, hence of core count (`applyRound_perm`), checked serial vs parallel and across `1,2,4,8,GOMAXPROCS`. 3) Byte-identical whole-run partition serial vs parallel and across core counts. 4) `make test-race` clean on the parallel core. 5) Convergence: synchronous updates reach a stable partition without oscillation on the corpus and on adversarial fixtures. `make validate` and `make test-race` green.

## Notes

**2026-07-17T23:35:29Z**

Implemented synchronous-round parallel local moving (M4, step 9) in parallel.go with TDD.

Design: applyRound(p, t, moved) is the mask closed-form of the Lean applyRound (verification/lean/Meso/Round.lean), so confluence/order-independence (applyRound_perm) is structural. parallelBestMoves fans the per-node snapshot best-move search across goroutines by contiguous node-index range (read-only shared g/obj/p, disjoint output slots -> race-free, byte-identical across worker counts). Per-node isolation labels isolationBase(p,n)+u instead of the serial shared fresh counter, so the choice is a pure function of the snapshot.

Confluence mitigation chosen (plan 4.5): 'strictly-positive-gain acceptance with deterministic conflict resolution', NOT graph-coloring (coloring by adjacency is not provably monotone under modularity's null model - non-adjacent same-target nodes still lose a cross term). parallelRound computes the round's exact realized delta as a telescoping moveDelta sum in ascending node order; accepts the whole moved set if it clears moveImproveEps, else falls back to the single highest-gain move (smallest index on tie), which is exactly monotone. Every changing round strictly improves, so termination is the serial loop's finite-range argument. The fallback stays applyRound(p,t,{best}) so it never leaves the proved-confluent shape.

Wiring: louvainTrace/leiden refactored to take a localMover (louvainTraceWith/leidenWith); serial movers wrap the existing sweep/queue phases, parallelMover(workers) wraps the synchronous phase - louvain/leiden/louvainLevels and all existing tests keep their signatures. Public opt-in via WithParallelism(workers) (default stays serial, so golden corpus untouched). Parallel and serial reach different but valid fixed points by design.

Tests (parallel_test.go): TestParallel_RoundClosedForm (applyRound_eq), TestParallel_RoundCoreCountInvariant (applyRound_perm + worker sweep 1,2,4,8,GOMAXPROCS), TestParallel_Convergence (corpus + even-cycle swap traps: monotone, terminates, local-move-stable), TestParallel_LocalMoveToStableMatchesRounds, TestParallel_WholeRunCoreCountInvariant, TestParallel_PublicOptionCoreCountInvariant. make validate + make test-race green; -race clean on the parallel core.

CORRESPONDENCE.md: applyRound representation row + applyRound_eq/applyRound_perm test rows flipped to landed; divergence register notes the Go-side monotonicity mitigation (beyond the model, covered by TestParallel_Convergence). learnings.md updated.
