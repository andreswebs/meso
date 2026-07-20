---
id: mes-0isk
status: closed
deps: []
links: []
created: 2026-07-14T03:40:29Z
type: task
priority: 1
assignee: Andre Silva
parent: mes-t76u
tags: [core, csr, step-1, verified]
---

# CSR weighted-graph model + Partition type

The internal compact CSR representation (offset, neighbor, weight arrays) over dense integer node indices, plus per-node sizes/weights, weighted degree, and `2m`; and the `Partition` type (community label per node) with a well-formedness check. This is the data model everything else stands on. Design of record: `docs/meso-design.md` sections 3 and 4; step 1 of `docs/specs/001-initial-implementation/plan.md`.

## Design

Mirror the Lean model in `verification/lean/Meso/Graph.lean`: `WeightedGraph` over `Fin n` (symmetric nonnegative weights `weight_symm`, `nodeSize`, `degree`, `twoM`) maps to CSR arrays over dense indices `[0,n)`; `Partition n := Fin n -> Nat` maps to `[]int`. Store offsets (`[]int`, len `n+1`, monotonic), neighbors (`[]int`), weights (`[]float64`) aligned with neighbors; node sizes `[]float64`; internal self-loop weight per node kept separate for the resolution term. Keep in/out neighbor lists separable from the start so directed (M3, step 8) reuses the layout. Provide an unexported test-only constructor from raw arrays so the type is testable before the `Builder` lands. `CORRESPONDENCE.md` rows: `WeightedGraph n`, `weight`, `nodeSize`, `degree`/`twoM`, `Partition n`. Go conventions: package-private struct, doc comments on exported symbols, no hot-path allocation (`data-structures.md` capacity hints).

## Acceptance Criteria

TDD vertical slices, one test then one implementation, in this order. 1) A CSR built from a tiny fixed adjacency exposes the right `degree(i)` and `neighbors(i)`. 2) `twoM()` equals the summed edge weight times two (self-loops counted once as internal weight, not doubled). 3) CSR invariants hold: offsets monotonic non-decreasing, `len(offsets)==n+1`, neighbor and weight slices equal length, sum of degrees consistent with `twoM`. 4) `nodeSize(i)` returns the stored per-node size; default size `1.0` when unset. 5) `Partition` well-formedness: a `[]int` of length n with every entry in a valid community range validates; wrong length or an out-of-range label is rejected (the structural invariant Lean treats as total-function well-formedness). 6) Degenerate shapes build a valid CSR: empty graph (`n=0`), single node, isolated node (degree 0). `make validate` green.

## Notes

**2026-07-17T20:06:08Z**

Implemented the internal CSR weighted-graph model (graph.go) and the Partition type (partition.go), mirroring verification/lean/Meso/Graph.lean. TDD, 6 vertical slices per the acceptance criteria.

Delivered:
- csr struct over dense indices [0,n): out adjacency (offsets/neighbors/weights, factored into an 'adjacency' sub-struct so directed M3 can add an in-arc adjacency without reshaping), separate per-node selfLoops, and nodeSizes.
- newCSR(offsets, neighbors, weights, selfLoops, nodeSizes) low-level test-only constructor; nil selfLoops -> zeros, nil nodeSizes -> 1.0 default.
- Accessors: numNodes, neighbors(i)/neighborWeights(i) (subslice views, no alloc), degree(i) (includes self-loop once), twoM() (off-diagonal doubled, self-loops once), nodeSize(i), checkInvariants().
- Partition []int (exported per design) with wellFormed(n): length n and labels in [0,n).
- Tests cover degree/neighbors, twoM with self-loops, structural invariants (valid + 5 malformed rejections), nodeSize default/explicit, partition well-formedness (valid + 4 invalid), and degenerate shapes (n=0, single node, isolated node).

Decisions for the next person (see docs/specs/learnings.md):
- Nonneg validation is NOT in newCSR; it belongs at the public Builder boundary (mes-1ekt) per the CORRESPONDENCE divergence register.
- Left CORRESPONDENCE.md representation rows at 'planned' since they describe the full public representation (incl. builder key map) landing in mes-1ekt.
- Left meso_test.go (blank-import smoke test) in place; retire it when the public API lands.

make validate, make build, and make test-race all green.
