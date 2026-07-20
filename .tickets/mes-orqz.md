---
id: mes-orqz
status: closed
deps: [mes-1ekt]
links: []
created: 2026-07-14T03:40:29Z
type: task
priority: 1
assignee: Andre Silva
parent: mes-t76u
tags: [core, quality, modularity, step-2, verified]
---

# QualityFunction interface + modularity(gamma) from-scratch evaluator

The `QualityFunction` interface and the modularity objective with a resolution parameter, as a full from-scratch (simple, slow, obviously-correct) evaluator - not yet the incremental delta. This is the oracle the move-delta property test (later) checks against. Design of record: `docs/meso-design.md` section 4.3; step 2 of `docs/specs/001-initial-implementation/plan.md`.

## Design

Model `verification/lean/Meso/Quality.lean` modularity: `Q = (1/2m) * sum_ij (w_ij - gamma * k_i k_j / 2m) * delta(c_i,c_j)`. Define a `QualityFunction` interface (`Quality(g, partition) float64`) so modularity and CPM (next ticket) and directed modularity (M3) share one shape; Go passes it into the move loop later. Implement the double-sum definition literally; correctness over speed. Resolution gamma is a field of the modularity value. Sum in canonical (sorted) order so floating-point summation is deterministic (`docs/meso-design.md` section 4.4; CORRESPONDENCE divergence register on `float64` vs R). Go conventions: `interfaces.md` (small interface), `documentation.md` (doc the formula).

## Acceptance Criteria

TDD order. 1) Modularity of a hand-computed triangle at `gamma=1` matches the by-hand value. 2) Same for a path graph and for two disjoint edges, at `gamma=1` and at a gamma above and below 1. 3) Q of the all-in-one-community partition and the all-singletons partition equal their known closed forms. 4) The `QualityFunction` interface is satisfied by the modularity type (compile-time assertion `var _ QualityFunction = ...`). 5) Summation order is canonical so repeated evaluation is bit-identical. `make validate` green. (Guarantee tests that build on this evaluator land in later tickets: `TestAggregate_ModularityCommunitySum` in the aggregation ticket, `TestLocalMove_*` in the move-delta ticket.)

## Notes

**2026-07-17T20:20:57Z**

Added QualityFunction interface (Quality(g *csr, p Partition) float64) and the from-scratch modularity(gamma) evaluator in quality.go, plus csr.weight(i,j) in graph.go mirroring Lean G.weight. Evaluator is the literal O(n^2) double sum from verification/lean/Meso/Quality.lean in canonical row-major order (bit-identical repeats). Public constructor Modularity(gamma) returns the interface; concrete modularity type unexported; compile-time assertion var _ QualityFunction = modularity{}. Tests (quality_test.go, internal package): hand-computed triangle/path/two-disjoint-edges at gamma below/at/above 1; all-in-one closed form Q=1-gamma and all-singletons Q=(sum w_ii - gamma*sum k_i^2/2m)/2m across gammas; self-loop diagonal; determinism; empty-graph 2m==0 -> Q=0. make validate + test-race green. Unblocks mes-vy3a, mes-w60v, mes-qvav, mes-nqky, mes-pgah. Note the QualityFunction method takes *csr, so it is meso-internal by design; CORRESPONDENCE modularity row can flip planned->landed once the oracle harness (mes-nqky) lands.
