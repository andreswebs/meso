---
id: mes-wmzq
status: closed
deps: [mes-jbc7]
links: [mes-5wqp, mes-nqky, mes-vy3a, wor-w33p, mes-y8ru]
created: 2026-07-14T03:40:30Z
type: task
priority: 1
assignee: Andre Silva
parent: mes-mklx
tags: [leiden, invariants, guarantees, step-7, verified]
---

# Empirical invariants + formal-guarantee checks on corpus and fuzzed inputs

The empirical mirror of the Lean paper-theorems tier: assert the invariants and the three paper guarantees on the whole corpus and on fuzzed graphs. Nothing new is built; this is the test layer that holds the Go faithful to the verified model. Design of record: `docs/meso-design.md` sections 6.3 and 7; step 7 of `docs/specs/001-initial-implementation/plan.md`.

## Design

Assert the structural invariants (well-formed partition, every community a connected subgraph, quality monotone across levels, termination within limits) and the three formal guarantees on fixtures. Verification originals: `verification/lean/Meso/Guarantees.lean` (`leidenGuarantees_of_stable`) conjoining `Separation.lean` (`gammaSeparated_of_converged`), `GammaConnectivity.lean` (`gammaWellConnectedCommunities_of_gammaMergeRun`), `SubsetOptimality.lean` (`isSubsetOptimal_of_stable`, `cpm_moveSubset_split`, `IsSubsetStable.isLocalMoveStable`), plus Convergence/Separation (`cpm_noStrictlyBetterCommunity`). The differential envelope check consumes frozen reference vectors (oracle harness assumed ready; not built here). CORRESPONDENCE rows listed in acceptance.

## Acceptance Criteria

TDD order, each an empirical property over corpus + fuzzed graphs. 1) Well-formed partition: each node in exactly one community, labels in range. 2) Every community is a connected subgraph. 3) Quality monotonic non-decreasing across aggregation levels. 4) Termination within iteration limits. 5) `TestConverge_NoBetterCommunity`: at convergence no node has a strictly better community (`cpm_noStrictlyBetterCommunity`). 6) `TestCPM_GammaSeparated`: converged communities are gamma-separated `e(C,D) <= gamma S_C S_D` (`gammaSeparated_of_converged`). 7) `TestLeiden_CommunitiesGammaConnected`: a gated refinement run keeps communities connected and gamma-dense (`gammaWellConnectedCommunities_of_gammaMergeRun`). 8) `TestCPM_SubsetSplitGain` / `TestConverge_SubsetStableIsMoveStable` / `TestCPM_SubsetOptimal`: subset split-off gain, subset-stability implies move-stability, and no sparse cut `e(S,C\S) >= gamma |S| |C\S|` (`SubsetOptimality.lean`). 9) `TestLeiden_Guarantees`: a converged Leiden output satisfies all three guarantees at once (`leidenGuarantees_of_stable`). 10) Differential envelope: quality, community count, connectivity within the reference envelope (consumes frozen vectors). `make validate` and `make test-race` green.

## Notes

**2026-07-17T02:29:41Z**

Guarantee predicates now have a proved oracle (2026-07-16, Lean Phase G). Each committed
golden case carries a `predicates` object (`connected`, `gammaDense`, `gammaSeparated`,
`subsetOptimal`) in `verification/oracle/golden/*.json`, each the value of a Lean Bool mirror
proved equal to the paper predicate (`Meso/Predicates.lean`; connectivity via an efficient
reachable-set closure in `Meso/Reachability.lean`, corpus-scale). So acceptance checks 2 and
6-9 can assert the Go result against a committed, proved oracle boolean, not only recompute
independently. The "differential envelope consuming frozen vectors" language is retired: the
connectivity flag is cross-checked against igraph (`verification/reference`, ALL PASS); the
gamma-predicates are meso-specific guarantees the references do not compute, emitted for
these tests. `subsetOptimal` is null above a small node bound (its decision enumerates subsets).

**2026-07-17T22:38:29Z**

Landed the empirical invariants + formal-guarantee test layer (step 7) in guarantees_test.go, plus two production predicate deciders in refine.go: gammaSeparatedCommunities and gammaWellConnectedCommunities (peers of connectedCommunities/gammaDenseCommunities). All 10 ACs covered: AC1 TestLeiden_OutputWellFormed; AC2 TestLeiden_OutputCommunitiesConnected (whole returned partition, corpus+800 fuzzed, both objectives) + TestOracle_ConnectivityVector; AC3 TestLeidenRun_QualityMonotoneAcrossLevels; AC4 TestLeidenRun_LevelsTerminate; AC5 TestConverge_NoBetterCommunity; AC6 TestCPM_GammaSeparated (asserts aggregateLevelStable precondition then gamma-separation); AC7 TestLeiden_CommunitiesGammaConnected (gated GammaMergeStep run from a gamma-well-connected K6 base); AC8 TestCPM_SubsetSplitGain, TestConverge_SubsetStableIsMoveStable, TestCPM_SubsetOptimal; AC9 TestLeiden_Guarantees (conjunction vs AND of proved flags); AC10 the four TestOracle_*Vector deciders cross-checked against committed golden predicate booleans. Key finding: raw multilevel output is level-stable but not single-node-stable on the base graph at gamma>=~0.3, so each guarantee test asserts the theorem's actual hypothesis. Two-sided teeth on every vector/guarantee test; mutation-checked. make validate, make test-race, make build all green. Flipped 12 CORRESPONDENCE.md section-2 rows from planned to landed. Names TestLeiden_QualityMonotoneAcrossLevels/TestLeiden_LevelsTerminate were already taken by the Louvain-trace tests, hence TestLeidenRun_* for the full-Leiden-run versions.
