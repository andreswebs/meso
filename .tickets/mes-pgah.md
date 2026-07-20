---
id: mes-pgah
status: closed
deps: [mes-orqz, mes-1ekt]
links: []
created: 2026-07-14T03:40:30Z
type: task
priority: 2
assignee: Andre Silva
parent: mes-q7p0
tags: [directed, quality, modularity, step-8, reference-tested]
---

# Directed modularity (Leicht-Newman) from-scratch evaluator

The Leicht-Newman directed modularity objective and its from-scratch evaluator, using separate in- and out-degrees. CPM stays undirected by design. Design of record: `docs/meso-design.md` sections 4.3 and 5; step 8 of `docs/specs/001-initial-implementation/plan.md`.

## Design

Directed modularity null model uses `k_i^out k_j^in / m` (not the symmetric `k_i k_j / 2m`). Reuse the `QualityFunction` interface; add a directed evaluator that reads the in/out neighbor lists kept separable since the CSR/`Builder` tickets. Cross-check semantics against `leidenalg/igraph` (the Java reference does not cover directed). NO Lean verification original: directed is a deliberate descope (Lean Phase E3, ticket mes-aoug; CORRESPONDENCE divergence register) because `weight_symm` is structural and load-bearing; directed is reference-tested only.

## Acceptance Criteria

TDD order. 1) Directed `Q` of a hand-computed asymmetric fixture matches by hand (separate in/out degrees). 2) A symmetric directed graph reduces to the undirected modularity result. 3) Directed evaluator satisfies the `QualityFunction` interface. 4) Canonical summation order keeps it bit-identical. `make validate` green. NOTE: no `#print axioms` obligation - directed has no Lean model by decision; correctness rests on the directed oracle (`leidenalg/igraph` frozen vectors, assumed ready).

## Notes

**2026-07-17T22:50:18Z**

Added Leicht-Newman directed modularity evaluator in directed_quality.go: DirectedModularity(gamma) QualityFunction. Null model uses separate out/in degrees k_i^out*k_j^in/T (T = total arc weight = csr.twoM() for a directed graph, which sums outDegree). weight(i,j) reads the out-adjacency (arc i->j), inDegree(j) reads the in-adjacency; both already existed from the CSR/Builder tickets. Row-major canonical summation, mirrors modularity/cpm. TDD in directed_quality_test.go: (1) hand-computed asymmetric 3-node fixture 0->1(2),0->2(1),1->0(1) [singletons Q=-0.3125, all-in-one Q=0]; (2) symmetric directed reduces to undirected modularity with EXACT equality across triangle/path/two-edges x gamma{0.5,1,2}; (3) compile-time QualityFunction assertion; (4) bit-identical repeated evaluation. Dropped the resolution() method for now (would be lint-unused): it belongs with the directed moveDelta in mes-gz8f, where directed becomes a full internal 'objective'. No Lean model by design; leidenalg/igraph frozen-vector cross-check remains for the oracle infra (Docker harness, not run here). Louvain/Leiden still reject directed graphs (mes-28rk). make validate + make build green.
