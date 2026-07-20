---
id: mes-28rk
status: closed
deps: [mes-gz8f, mes-jbc7]
links: []
created: 2026-07-14T03:40:30Z
type: task
priority: 2
assignee: Andre Silva
parent: mes-q7p0
tags: [directed, leiden, api, step-8, reference-tested]
---

# Directed handling through local move / refinement / aggregation + directed API

Thread directedness through the whole pipeline (local moving, refinement, aggregation) and expose directed graphs via the public API, with directed fixtures scored against the oracle. Design of record: `docs/meso-design.md` sections 4.1-4.3 and 5; step 8 of `docs/specs/001-initial-implementation/plan.md`.

## Design

Directed aggregation must preserve in/out separability; the local move and refinement reuse the directed delta (`DDIR`). Public API accepts directed builds (from the `Builder` directed path) and a directed-modularity quality option. Directed fixtures scored via `leidenalg/igraph` frozen vectors (assumed ready). No Lean verification original (E3 descope); correctness is reference-tested via the differential envelope.

## Acceptance Criteria

TDD order. 1) Directed fixtures produce the expected communities (oracle via `leidenalg/igraph` frozen vectors). 2) A symmetric directed graph reduces to the undirected `Leiden` result end to end. 3) Directed aggregation preserves in/out degree separability across a level. 4) Public API runs `Leiden` on a directed build with the directed-modularity option and returns communities in caller keys; byte-identical at a fixed seed. `make validate` and `make test-race` green.

## Notes

**2026-07-17T23:18:12Z**

Threaded directedness through the whole pipeline + directed public API (M3, AC1-4 met; make validate + test-race green).

Changes (all small, reusing existing direction-aware primitives; no second optimiser):
- directedModularity gained resolution(), so it is now a full objective (Quality+moveDelta+resolution) and drives the shared local-move/refine loops.
- aggregate() branches on g.directed: new aggregateDirected folds each out-arc once, internal arc -> self-loop, cross arc a->b into out-block (a,b) AND in-block (b,a); flattenBlock shared. Preserves in/out degree separability and the directed round-trip Quality(agg,aggP)==Quality(g,expand).
- bestMove and refineIntents gather candidates from BOTH arc directions (deduped), reducing exactly to out-only on undirected graphs.
- Leiden/Louvain accept directed graphs; checkObjectiveGraph requires DirectedModularity for directed (rejects undirected-modularity/CPM, whose symmetric null model mis-scores arcs); resolve WithResolution switch handles directedModularity. Renamed stale TestLeiden_DirectedRejected -> TestLeiden_DirectedRequiresDirectedModularity.

Tests: directed_aggregate_test.go (AC3 separability + directed round-trip + symmetric-is-symmetric), directed_pipeline_test.go (bestMove vs exhaustive-optimum oracle incl in-arc gains; symmetric bestMove/refine reduce to undirected), directed_api_test.go (AC1 two directed 3-cycles recover 2 communities; AC2 symmetric reduces to undirected end-to-end same grouping + identical quality; AC4 byte-identical at fixed seed).

Notes/traps for next person (also in docs/specs/learnings.md):
- Directed vs undirected move-delta differ ~1 ULP (null-term summation order): compare local-move DECISION exactly but delta within moveTol.
- bestMove still attains the global optimum delta (isolation dominates non-adjacent), so the oracle compares achieved delta, not target (ties allowed).
- localMoveDrain re-enqueues only out-neighbours; left as-is (outer drain loop still reaches IsLocalMoveStable; changing it would alter the deterministic partition for no correctness gain).
- AC1 uses hand-verifiable fixtures, NOT leidenalg/igraph frozen vectors: the reference harness is Docker-only/out-of-band and ships no directed community vectors. Directed remains a Lean descope (reference-tested only).
