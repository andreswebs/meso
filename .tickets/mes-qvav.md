---
id: mes-qvav
status: closed
deps: [mes-orqz]
links: []
created: 2026-07-14T03:40:29Z
type: task
priority: 1
assignee: Andre Silva
parent: mes-t76u
tags: [core, quality, cpm, step-2, verified]
---

# CPM(gamma) from-scratch evaluator (undirected) + gamma-density

The Constant Potts Model objective with a node-size penalty and no `2m` normalization, as a from-scratch evaluator, plus its per-community decomposition and the gamma-density notion. Design of record: `docs/meso-design.md` section 4.3; step 2 of `docs/specs/001-initial-implementation/plan.md`.

## Design

Model `verification/lean/Meso/CPM.lean`: `cpm = sum_ij (w_ij - gamma * s_i s_j) * delta(c_i,c_j)`, and `cpm_eq_communitySum = sum_c (e_c - gamma * S_c^2)` where `e_c` is internal weight and `S_c` the summed node size. Reuse the `QualityFunction` interface from the modularity ticket; CPM shares the move loop. gamma-density (`IsGammaDense`): a community with `e_c >= gamma S_c^2`, i.e. nonnegative CPM contribution (`isGammaDense_iff`). Verification originals: `CPM.lean` theorems `cpm_eq_communitySum`, `isGammaDense_iff`.

## Acceptance Criteria

TDD order. 1) CPM of the triangle / path / two-disjoint-edges fixtures matches by-hand values at `gamma=1` and other gamma. 2) `TestCPM_CommunitySum`: CPM equals the sum of per-community contributions `e_c - gamma S_c^2` on random fixtures (Go image of `cpm_eq_communitySum`). 3) `TestCPM_GammaDenseContribution`: a community is gamma-dense iff its contribution `e_c - gamma S_c^2` is nonnegative (Go image of `isGammaDense_iff`). 4) CPM satisfies the `QualityFunction` interface (compile-time assertion). 5) Node sizes survive into the penalty term (a graph with non-unit node sizes changes CPM as predicted). `make validate` green.

## Notes

**2026-07-17T20:30:56Z**

Implemented CPM(gamma) from-scratch evaluator in cpm.go (undirected). Q = sum_ij (w_ij - gamma*s_i*s_j)*delta(c_i,c_j), no 2m normalization (units of edge weight). Added unexported helpers communityInternalWeight (e_c), communitySize (S_c), and isGammaDense (gamma*S_c^2 <= e_c) - Go images of the CPM.lean defs; these are reused by the CPM move-delta (mes-l38o) and aggregation (mes-w60v). Tests in cpm_test.go: hand-computed values on triangle/path/two-edges at multiple gamma, self-loop diagonal, node-sizes-affect-penalty (AC5), determinism, empty graph, plus two 3000-trial property tests over random graphs with non-unit node sizes: TestCPM_CommunitySum (cpm_eq_communitySum) and TestCPM_GammaDenseContribution (isGammaDense_iff). QualityFunction compile-time assertion present. make validate green.
