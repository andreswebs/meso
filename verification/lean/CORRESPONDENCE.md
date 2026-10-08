# meso model-to-code correspondence

This file is the audited bridge between the Lean model in this directory and the
Go library at the repository root. Lean verifies a model, not the Go binary (see
[README.md](README.md)); this document is where the claim "the Go code
implements the same objects and preserves the same invariants" is made explicit
and kept reviewable.

It is a reviewed artifact. A wrong or stale row here is exactly the residual
trust the formal-verification tier exists to shrink, so treat edits to these
tables with the same care as a proof.

## The rule that keeps this honest

A proved Lean theorem, a row in the theorem-to-test table, and a green Go
guarantee test are added together. Any one of the three without the other two is
a gap, and deliberate gaps live in the divergence register so they cannot
masquerade as coverage.

Status legend: `proved` (Lean, no `sorryAx`), `open` (Lean obligation not yet
discharged), `planned` (Go symbol or test not yet written), `landed` (Go symbol
written and covered by tests).

## 1. Representation correspondence

How each mathematical object is realized in Go. Rows should be "obviously the
same" so a reviewer can check them by eye. Go symbols are proposed until the core
lands; mark them `planned` and correct on arrival.

| Lean (`Meso`)                                    | Go (`package meso`)                                             | Status  | Notes on the encoding gap                                                                                                                    |
| ------------------------------------------------ | --------------------------------------------------------------- | ------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| `WeightedGraph n` over `Fin n`                   | internal CSR (offset/neighbor/weight arrays), `int`             | landed  | `Fin n` maps to dense indices `[0, n)`; the builder's key-to-index map is outside the model                                                  |
| `weight : Fin n → Fin n → ℝ`                     | CSR adjacency lookup, `float64`                                 | landed  | dense function vs sparse CSR; `weight_symm` is a builder invariant asserted in the parallel-edge test                                        |
| `weight_nonneg`, `nodeSize_nonneg`               | validated in `Build()`, error on violation                      | landed  | the model assumes these as fields; Go enforces them at the input boundary (`!(w >= 0)` catches NaN too)                                      |
| `nodeSize : Fin n → ℝ`                           | per-node `[]float64`                                            | landed  | preserved through aggregation (plan 4.1); re-check the correspondence for the aggregate graph                                                |
| `degree`, `twoM`                                 | CSR `degree(i)`, `twoM()`                                       | landed  | real sum vs `float64` sum; canonical (sorted) summation order is the FP property that matters                                                |
| `Partition n := Fin n → ℕ`                       | `[]int` (community label per node)                              | landed  | total function vs length-`n` slice; well-formedness is structural in Lean, a length+range check in Go                                        |
| `modularity G γ p` (ℝ, noncomputable)            | quality function, `float64`                                     | planned | reals vs float; compared within a delta, never bit-identical                                                                                 |
| `cpm G γ p` (ℝ, noncomputable)                   | CPM quality function, `float64`                                 | planned | node-size penalty `γ s_i s_j`, no `2m` term; shares the `QualityFunc` interface with modularity in Go                                        |
| `DirectedWeightedGraph.weight i j` (arc `i → j`) | directed CSR out/in adjacency (`csr.out`, `csr.in`), `float64`  | landed  | undirected structure minus `weight_symm`; Go's split out/in adjacency is a CSR detail, the model is the dense arc function                   |
| `directedModularity G γ p` (ℝ, noncomputable)    | `DirectedModularity(γ).Quality`, `float64`                      | landed  | Leicht-Newman null model, separate in/out degrees `k_i^out k_j^in / m`; reduces to `modularity` on a symmetric graph                         |
| `directedModularityQ G γ p` (ℚ, computable)      | directed value-oracle vector (`directed_oracle_test.go`)        | landed  | rational mirror, `directedModularityQ_eq`-proved equal to `directedModularity`; emitted by `mesoOracle` behind the `directed` flag (Phase 5) |
| `IsLocalMove Q` (Q a quality function)           | fast-move loop, parameterised by quality function               | planned | `Q` is `modularity`/`cpm`; Go passes a `QualityFunc` so one move loop serves both                                                            |
| `IsLocalMoveStable Q p`                          | fast-move loop reached a fixed point (no node moves)            | planned | Go's loop-until-no-move termination flag; the `∀ v c` optimum the flag asserts                                                               |
| `IsConverged Qf G p`                             | full run converged (node- and aggregate-stable)                 | planned | Go's outer-loop stop condition: neither the move loop nor the aggregate move loop changed anything                                           |
| `move p v c = Function.update p v c`             | single-node reassignment                                        | planned | `Function.update` maps to one slice write; direct                                                                                            |
| `aggregate G p` over `Fin (numComm p)`           | aggregated CSR (one node per community)                         | planned | `commLabel` bijection = the community-to-dense-index relabel Go builds during aggregation                                                    |
| `G.simpleGraph` / `ConnectedCommunities`         | unweighted view (weight > 0 edges); connectivity check          | planned | Go checks connectivity per community by BFS/union-find on the CSR; self-loops excluded, as in `fromRel`                                      |
| `IsGammaDense` / `GammaDenseCommunities`         | refined community passes the γ-density gate (`e_c ≥ γ S_c²`)    | planned | Go's well-connectedness gate check during refinement; the CPM density bound the merge must clear                                             |
| `GammaMergeStep`                                 | one gated refinement merge (shared edge + γ-dense cut)          | planned | Go merges two sub-communities only across a γ-dense cut inside one outer community                                                           |
| `moveSubset p S c`                               | reassign a whole node set to one community                      | planned | subset generalisation of `move`; one masked pass over the label slice                                                                        |
| `IsSubsetStable` / `IsSubsetOptimal`             | no subset reassignment improves; every subset well-connected    | planned | the refinement fixed point Go reaches; the no-sparse-cut check the gate enforces at every scale                                              |
| `IsLeidenStable` / `LeidenGuarantees`            | a converged Leiden output; the three paper guarantees conjoined | planned | Go's whole-run stop state and the three guarantee assertions checked together on the output                                                  |
| `applyRound` (snapshot decision `t`)             | `applyRound` / `parallelRound` (snapshot-decide, then apply)    | landed  | Go's parallel round; `t` is each node's best target from the round-start snapshot (`parallelBestMoves`)                                      |
| `RunningLevelStep` (guard: non-discrete)         | the outer loop's "continue while the level changed" condition   | planned | Go's stop-when-unchanged check: continue only while a node moved / `numComm` dropped (see section 4)                                         |

## 2. Theorem-to-test correspondence

Every proved or planned Lean theorem gets a named Go guarantee test that checks
the same proposition on the corpus and on fuzzed inputs. No theorem is "landed"
until its row names a green Go test.

| Lean theorem                                     | Statement                                                                                                                                                | Go guarantee test                                     | Lean   | Go      |
| ------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------- | ------ | ------- |
| `move_self`                                      | moving a node to its own community is a no-op                                                                                                            | `TestLocalMove_SelfIsNoOp`                            | proved | planned |
| `modularity_bestMove_ge`                         | a best single-node move never lowers modularity (staying is an option)                                                                                   | `TestLocalMove_BestMoveNonDecreasing`                 | proved | planned |
| `IsLocalMove.modularity_le`                      | one local-move step is monotone                                                                                                                          | folded into the per-step assertion above              | proved | planned |
| `localMoveRun_monotone`                          | a whole local-move sweep is monotone                                                                                                                     | `TestLocalMove_SweepMonotone`                         | proved | planned |
| `quality_monotone_of_stepwise`                   | global monotonicity across levels from stepwise non-decrease                                                                                             | `TestLeiden_QualityMonotoneAcrossLevels`              | proved | planned |
| `modularity_eq_communitySum`                     | modularity equals the sum of within-community block contributions                                                                                        | `TestAggregate_ModularityCommunitySum`                | proved | planned |
| `modularity_aggregate_eq`                        | aggregation preserves modularity (graph-to-graph invariance)                                                                                             | `TestAggregate_QualityPreserved`                      | proved | planned |
| `le_modularity_aggregate_run`                    | a level does not lose quality across the aggregation boundary                                                                                            | `TestLeiden_LevelNonDecreasing`                       | proved | planned |
| `modularity_le_of_algorithmRun`                  | the whole multilevel run never lowers modularity (all levels threaded)                                                                                   | `TestLeiden_QualityMonotoneWholeRun`                  | proved | planned |
| `connectedCommunities_singleton`                 | singleton-partition communities (single nodes) are connected                                                                                             | `TestConnectivity_SingletonConnected`                 | proved | landed  |
| `MergeStep.connectedCommunities`                 | an edge-merge preserves connected communities                                                                                                            | `TestRefine_MergePreservesConnectivity`               | proved | landed  |
| `connectedCommunities_of_mergeRun`               | any edge-merge run from singletons yields connected communities                                                                                          | `TestRefine_RunConnected`                             | proved | landed  |
| `connectedCommunities_of_refineRun`              | the refinement operator run from singletons has connected communities                                                                                    | `TestLeiden_CommunitiesConnected`                     | proved | landed  |
| `no_infinite_acceptedMove_run`                   | the fast local-move phase halts (no infinite strictly-improving run)                                                                                     | `TestLocalMove_TerminatesBounded`                     | proved | planned |
| `no_infinite_descending_levels`                  | the multilevel recursion halts (a strictly-decreasing level measure)                                                                                     | `TestLeiden_LevelsTerminate`                          | proved | planned |
| `levelStep_size_lt_or_injective`                 | a level strictly lowers the node count or its partition is discrete                                                                                      | `TestLeiden_LevelSizeDichotomy`                       | proved | planned |
| `levelRun_reaches_fixedPoint`                    | any infinite level run reaches a discrete-partition fixed point                                                                                          | `TestLeiden_LevelsReachFixedPoint`                    | proved | planned |
| `RunningLevelStep.size_lt`                       | a guarded (running) level strictly shrinks the node count                                                                                                | `TestLeiden_RunningLevelShrinks`                      | proved | planned |
| `no_infinite_runningLevel_run`                   | the guarded multilevel loop halts outright (no infinite running run)                                                                                     | `TestLeiden_GuardedLevelsTerminate`                   | proved | planned |
| `cpm_localMoveRun_monotone`                      | a CPM local-move sweep is monotone (shared interface, CPM instance)                                                                                      | `TestCPM_SweepMonotone`                               | proved | planned |
| `cpm_eq_communitySum`                            | CPM equals the sum of per-community contributions `e_c − γ S_c²`                                                                                         | `TestCPM_CommunitySum`                                | proved | planned |
| `cpm_aggregate_eq`                               | aggregation preserves CPM (graph-to-graph invariance)                                                                                                    | `TestAggregate_CPMPreserved`                          | proved | planned |
| `le_cpm_aggregate_run`                           | a level does not lose CPM across the aggregation boundary                                                                                                | `TestLeiden_CPMLevelNonDecreasing`                    | proved | planned |
| `cpm_le_of_algorithmRun`                         | the whole multilevel run never lowers CPM (all levels threaded)                                                                                          | `TestLeiden_CPMMonotoneWholeRun`                      | proved | planned |
| `isGammaDense_iff`                               | a community is γ-dense iff its CPM contribution is nonnegative                                                                                           | `TestCPM_GammaDenseContribution`                      | proved | planned |
| `IsLocalMove.quality_eq_of_stable`               | a local-move step out of a stable partition leaves quality unchanged                                                                                     | `TestConverge_StableIsMoveFixedPoint`                 | proved | planned |
| `IsLocalMoveStable.no_strict_improvement`        | no single-node move strictly improves a stable partition                                                                                                 | `TestConverge_StableNoImprovement`                    | proved | planned |
| `cpm_merge_two_singletons`                       | merging two singletons changes CPM by `2·(w_AB − γ s_A s_B)`                                                                                             | `TestCPM_MergeGain`                                   | proved | planned |
| `cpm_noStrictlyBetterCommunity`                  | at convergence no node has a strictly better community (CPM)                                                                                             | `TestConverge_NoBetterCommunity`                      | proved | landed  |
| `gammaSeparated_of_converged`                    | converged communities are γ-separated: `e(C,D) ≤ γ S_C S_D`                                                                                              | `TestCPM_GammaSeparated`                              | proved | landed  |
| `gammaDense_union`                               | a γ-dense cut joins two γ-dense sets into a γ-dense union                                                                                                | `TestCPM_GammaDenseUnion`                             | proved | landed  |
| `GammaMergeStep.gammaDenseCommunities`           | a γ-gated merge preserves internal γ-density of every community                                                                                          | `TestRefine_MergePreservesGammaDensity`               | proved | landed  |
| `gammaWellConnectedCommunities_of_gammaMergeRun` | a gated refinement run keeps communities connected and γ-dense                                                                                           | `TestLeiden_CommunitiesGammaConnected`                | proved | landed  |
| `cpm_moveSubset_split`                           | splitting a subset off changes CPM by `−2·(e(S,C\S) − γ ‖S‖ ‖C\S‖)`                                                                                      | `TestCPM_SubsetSplitGain`                             | proved | landed  |
| `IsSubsetStable.isLocalMoveStable`               | subset stability implies single-node local-move stability                                                                                                | `TestConverge_SubsetStableIsMoveStable`               | proved | landed  |
| `isSubsetOptimal_of_stable`                      | a subset-stable partition has no sparse cut: `e(S,C\S) ≥ γ ‖S‖ ‖C\S‖`                                                                                    | `TestCPM_SubsetOptimal`                               | proved | landed  |
| `leidenGuarantees_of_stable`                     | a converged Leiden output satisfies all three paper guarantees at once                                                                                   | `TestLeiden_Guarantees`                               | proved | landed  |
| `applyRound_eq`                                  | a synchronous round is `t` on moved nodes, `p` elsewhere (closed form)                                                                                   | `TestParallel_RoundClosedForm`                        | proved | landed  |
| `applyRound_perm`                                | a synchronous round's outcome is schedule- and core-count-independent                                                                                    | `TestParallel_RoundCoreCountInvariant`                | proved | landed  |
| `connectedCommunitiesFast_iff`                   | the emitted `connected` flag decides `ConnectedCommunities` (efficiently)                                                                                | `TestOracle_ConnectivityVector`                       | proved | landed  |
| `gammaDenseCommunitiesQ_iff`                     | the emitted `gammaDense` flag decides `GammaDenseCommunities`                                                                                            | `TestOracle_GammaDenseVector`                         | proved | landed  |
| `gammaSeparatedCommunitiesQ_iff`                 | the emitted `gammaSeparated` flag decides γ-separation of communities                                                                                    | `TestOracle_GammaSeparatedVector`                     | proved | landed  |
| `subsetOptimalQ_iff`                             | the emitted `subsetOptimal` flag decides `IsSubsetOptimal`                                                                                               | `TestOracle_SubsetOptimalVector`                      | proved | landed  |
| `mem_reachableFinset_iff`                        | the reachable-set closure equals `ReflTransGen`-reachability                                                                                             | folded into `TestOracle_ConnectivityVector`           | proved | landed  |
| `directedModularity_toDirected_eq`               | directed Q equals undirected `modularity` on a symmetric graph                                                                                           | `TestDirectedModularity_SymmetricReducesToUndirected` | proved | landed  |
| `directedModularity_aggregate_eq`                | directed aggregation preserves directed modularity (Go `aggregateDirected`)                                                                              | `TestAggregate_DirectedRoundTrip`                     | proved | landed  |
| `directedModularity_move_eq`                     | closed-form directed move gain equals the from-scratch difference; the Go incremental formula is now backed by a proved identity, not property-test only | `TestDirectedMove_MatchesOracle`                      | proved | landed  |
| `directedModularity_const`                       | the all-in-one partition scores `1 − γ` (`0` at `γ = 1`)                                                                                                 | `TestDirectedModularity_HandComputed`                 | proved | landed  |
| `directedConnectedCommunities_of_refineRun`      | a directed refinement run from singletons has weakly connected communities (`fromRel` symmetrization)                                                    | `TestLeiden_DirectedFixtureCommunities`               | proved | landed  |
| `directedModularity_merge_two_singletons`        | directed-modularity gain of merging two singletons (both arc directions, `1/m` factor)                                                                   | folded into the separation proof                      | proved | n/a     |
| `directedGammaSeparated_of_levelStable`          | directed γ-separation, conditional on directed level stability: `e(C,D)+e(D,C) ≤ (γ/m)(Kout_C·Kin_D + Kout_D·Kin_C)`                                     | `TestLeiden_DirectedFixtureCommunities`               | proved | planned |
| `directedModularity_moveSubset_split`            | closed-form directed subset split gain (both cross blocks, `1/m` factor)                                                                                 | folded into the subset proof                          | proved | n/a     |
| `directedSubsetGammaDense_of_subsetStable`       | directed subset bound, conditional on directed subset stability (hypothesis not attained by the algorithm)                                               | `TestLeiden_DirectedSubsetOptimalityLimitation`       | proved | landed  |
| `directedConnectedCommunitiesFast_iff`           | the emitted directed `connected` flag decides `DirectedConnectedCommunities` (weak connectivity, efficiently)                                            | `TestOracle_DirectedConnectivityVector`               | proved | landed  |
| `directedGammaSeparatedCommunitiesQ_iff`         | the emitted directed `gammaSeparated` flag decides the directed separation bound                                                                         | `TestOracle_DirectedGammaSeparatedVector`             | proved | landed  |
| `directedSubsetOptimalQ_iff`                     | the emitted directed `subsetOptimal` flag decides the directed subset bound (a characterization; `null` above the node bound)                            | `TestOracle_DirectedSubsetVector`                     | proved | landed  |

## 3. Divergence register

Places where Go intentionally departs from the model. These are decisions, not
bugs; listing them keeps known gaps from masquerading as coverage.

- Go uses `float64`; the model uses ℝ. Consequence: comparisons are delta-based,
  and summation order is canonicalized (sorted adjacency) so the delta stays
  tight and stable. The reference for that delta is the Lean value-oracle: a
  computable rational mirror of the quality functions (`Meso/Compute.lean`:
  `modularityQ`, `cpmQ`), proved equal to the real model (`modularityQ_eq`,
  `cpmQ_eq`, both `sorryAx`-free) so the runnable artifact is the proved one,
  emits the exact expected value, and Go is checked within a float-rounding
  tolerance of it. This is the numeric oracle of record; the cross-language
  differential harness is retired (decided 2026-07-16, TODO Phase F1/F2 done; see
  [../../docs/specs/001-initial-implementation/meso-oracle.md](../../docs/specs/001-initial-implementation/meso-oracle.md)).
- The v0.2.0 structural measures are outside the model: the `Graph` accessors
  (`Keys`, `NumEdges`, `Degree`, `Neighbors`, `Weight`), the `Result` accessors
  and `Cohesion`, `Subgraph`, and `Betweenness`. No theorem, mirror or golden
  vector covers them. They are validated empirically instead: a definitional
  brute-force oracle on random small graphs, closed forms, committed networkx
  references on the corpus, insertion-order determinism, and fuzzing (design
  section 4.6 and 6.2). Their one contact with the model is the builder rule
  that drops zero-weight edges: the model's `weight i j = 0` already means "no
  edge" for connectivity (`CommunityConnected` follows only positive weights),
  so the rule changes no modelled quantity.
- The model assumes symmetry, nonnegative weights, and nonnegative node sizes as
  structure. Go validates them at `Build()` and returns an error; the malformed
  inputs the model never sees are Go's responsibility.
- CPM is now modelled (`Meso/CPM.lean`): quality function, its per-community
  decomposition `e_c − γ S_c²`, γ-density, and the shared local-move monotonicity. Its
  aggregation graph-to-graph invariance (the CPM analogue of `modularity_aggregate_eq`) is
  now derived too: `cpm_aggregate_eq` (TODO Phase E1). CPM is also threaded through the
  whole-run monotonicity story modularity has: `Meso/Level.lean` was generalized over the
  quality family (`IsAggregationInvariant`, `qualityFamily_le_of_algorithmRun`), and
  `cpm_le_of_algorithmRun` is the CPM instance alongside `modularity_le_of_algorithmRun`.
- meso reports CPM in the canonical (leidenalg) convention, decided 2026-07-16. The
  proof-side `cpm` (`Meso/CPM.lean`, and its rational mirror `cpmQ`) sums over all ordered
  pairs including the diagonal `i = j`; the canonical value `cpmCanonicalQ`
  (`Meso/Compute.lean`) drops that diagonal (`i ≠ j`), matching leidenalg. `cpmCanonicalQ_eq`
  proves the exact identity `cpmCanonicalQ = cpmQ − ∑_i (w_ii − γ s_i²)`; the subtracted
  diagonal is partition-independent (`−γ·N` for unit sizes, no self-loops), so the two share
  every optimum and guarantee. The value-oracle emits `cpmCanonicalQ`, the F4 cross-check
  compares it directly to leidenalg's `quality()` (all PASS), and the Go public `Quality()`
  reports it, so meso's CPM numbers match published literature. The guarantees
  (γ-separation, subset-optimality) stay stated in the proof-side convention, where they are
  cleanest; they are over distinct communities, so the diagonal never enters them. modularity
  matches igraph directly with no such adjustment. Recorded in
  `verification/reference/crosscheck/README.md`.
- Directed modularity's **objective and its algebraic identities are now modelled; the
  guarantees follow in Phases 3 and 4 (next bullet)** (Phases 1 and 2 of
  `../../docs/research/directed-modularity-formal-verification.md`, landed 2026-07-18;
  supersedes the 2026-07-14 full descope). `Meso/DirectedGraph.lean` defines
  `DirectedWeightedGraph` (the undirected structure minus the `weight_symm` field, so
  `weight i j` is the arc `i → j`), the directed out/in degrees and total arc weight, and
  the real-valued `directedModularity` (Leicht-Newman, separate in/out degrees), matching
  the Go `DirectedModularity` term for term. `Meso/DirectedCompute.lean` adds the rational
  mirror `directedModularityQ` and its move-delta `moveDeltaDirectedModularityQ`, proved
  equal to the real model by `directedModularityQ_eq` and `moveDeltaDirectedModularityQ_eq`
  (both `sorryAx`-free), plus five machine-checked fixture values against
  `directed_quality_test.go`. Phase 2 proved the objective identities the optimizer relies
  on, all `sorryAx`-free: the all-in-one value `directedModularity_const` (`1 − γ`, hence
  `0` at `γ = 1`); the symmetric reduction `directedModularity_toDirected_eq` (directed Q
  equals undirected `modularity` on a symmetric graph, the model-consistency anchor);
  directed aggregation invariance `directedModularity_aggregate_eq`
  (`Meso/DirectedAggregate.lean`, Go counterpart `aggregateDirected`); and the closed-form
  move-delta `directedModularity_move_eq` (`Meso/DirectedMove.lean`, Go counterpart
  `directedModularity.moveDelta`, requiring `t ≠ p u`). The directed model is standalone: it
  shares no code with the undirected `WeightedGraph`, and unification behind a common
  interface is an explicit open question of the research doc, not decided here.
- Directed **guarantees are now triaged and the survivors formalized; directed stays outside
  the value oracle** (Phases 3 and 4 of the research doc, landed 2026-07-18; supersedes the
  earlier "guarantees remain unproved" status). Phase 3 triage
  (`../../docs/research/directed-modularity-triage.md`) established which guarantees survive
  asymmetry; Phase 4 formalized the survivors in Lean, all `sorryAx`-free (the three top
  theorems depend only on `propext`, `Classical.choice`, `Quot.sound`). The earlier descope
  reasoning was too coarse: symmetry is not load-bearing for weak connectivity or for the
  directed γ-bounds, whose sub-lemmas are graph-agnostic; it is load-bearing only for the
  guarantees that genuinely fail (strong connectivity, and subset-optimality as an output
  property). Concretely:
  - **Weak connectivity: proved.** `directedConnectedCommunities_of_refineRun`
    (`Meso/DirectedConnectivity.lean`): every community of a directed refinement run from
    singletons is weakly connected, over `DirectedWeightedGraph.simpleGraph :=
SimpleGraph.fromRel (fun i j => 0 < weight i j)`, whose symmetrization combines both arc
    directions. The proofs port from the undirected `Refinement.lean` / `Connectivity.lean`
    verbatim (no `weight_symm`); the partition-level operators and
    `connected_induce_union_of_adj` are reused directly. Go counterpart: the directed
    refinement gate (`refine.go`), exercised by `TestLeiden_DirectedFixtureCommunities`.
  - **γ-separation: proved, conditional on directed level stability.**
    `directedGammaSeparated_of_levelStable` and its community-cut form
    `directedGammaSeparated_blockWeight_of_levelStable` (`Meso/DirectedSeparation.lean`):
    `e(C,D) + e(D,C) ≤ (γ/m)(Kout_C·Kin_D + Kout_D·Kin_C)`, the asymmetric kernel keeping
    both arc directions. As in the undirected case, the theorem is about the hypothesis:
    meso stops at `moveImproveEps`, so raw output is not always exactly level-stable. The
    directed stability predicates (`IsDirectedLevelStable`, `IsDirectedConverged`,
    `IsDirectedSubsetStable`) are standalone in `Meso/DirectedConvergence.lean` (no shared
    quality typeclass, per the research doc's open question 5).
  - **Subset-optimality: two-sided.** (a) The **conditional is proved**:
    `directedSubsetGammaDense_of_subsetStable` (`Meso/DirectedSubsetOptimality.lean`), from
    `IsDirectedSubsetStable`, gives `e(S,T) + e(T,S) ≥ (γ/m)(Kout_S·Kin_T + Kout_T·Kin_S)`
    for every proper nonempty subset `S` of a community `C` (`T = C \ S`). (b) As an
    **output property it is REFUTED**: the n=4 fixture (arcs 0→1:1, 0→3:1, 1→0:3, 1→2:1,
    1→3:5, 2→0:2, 3→2:5; `m = 18`, `γ = 1`) converges all-in-one `[0,0,0,0]` (Q = 0) while
    `{0,1},{2,3}` scores `1/27 > 0`; taking `S = {0,1}`, `T = {2,3}`,
    `e(S,T) + e(T,S) = 9 < 174/18`. The violation rates are about 1.3 percent directed and
    about 1.2 percent undirected-symmetric, so it is inherited from modularity, not
    directed-specific, and is consistent with the undirected model proving subset-optimality
    only from the `IsSubsetStable` hypothesis. Recorded as the Go characterization test
    `TestLeiden_DirectedSubsetOptimalityLimitation` and tracked by ticket `mes-niic` (the
    refinement investigation, which affects the undirected core too).
  - **Strong connectivity: refuted by design.** About 39 percent of communities in the
    triage sweep fail it; a single-arc pair `{u, v}` with only `u → v` is a legitimate
    community yet not strongly connected. No formal work; weak connectivity is the directed
    connectivity notion.

- Directed modularity now has a **standing value-oracle** (Phase 5, `mes-43lb`, landed
  2026-07-19; supersedes the "directed stays outside the value oracle" status above).
  `Meso/DirectedPredicates.lean` adds the computable Bool mirrors — weak connectivity
  (`DirectedConnectedCommunitiesFast`, the polynomial reachable-set decider),
  directed γ-separation (`DirectedGammaSeparatedCommunitiesQ`), and the directed subset
  bound (`DirectedSubsetOptimalQ`) — each proved `_iff` its Phase 4 real Prop on `.toReal`
  (`directedConnectedCommunitiesFast_iff`, `directedGammaSeparatedCommunitiesQ_iff`,
  `directedSubsetOptimalQ_iff`; all three depend only on `propext`, `Classical.choice`,
  `Quot.sound`). `Meso/OracleIO.lean` emits directed quantity + `{connected, gammaSeparated,
subsetOptimal}` vectors behind a top-level `"directed": true` flag (single arcs, no
  symmetrization, `directedModularity` the only quality); the undirected path is unchanged
  and its goldens byte-identical. The Go harness (`directed_oracle_test.go`) builds through
  `NewDirectedBuilder` and asserts the directed quantity, move-delta, weak-connectivity, and
  γ-separation deciders against the proved vectors over the committed fixtures
  (`directed_asym3`, `directed_subset4`, `directed_cycles6`, and the `celegansneural` corpus
  graph, ~297 nodes). The **directed `subsetOptimal` flag is a characterization, not an
  output guarantee**: it reads `false` on `directed_subset4`'s converged all-in-one (the
  Phase 3 refutation, cross-referenced to `mes-niic`) and is `null` above the emitter's node
  bound (celegans). The directed predicate object has **no `gammaDense` key**: γ-density is a
  CPM guarantee and **directed CPM remains unmodelled by design**.

- The refinement operator (`RefineStep`) models the well-connectedness gate by its
  connectivity-relevant consequence only: a shared positive-weight edge between the
  two merged sub-communities, plus outer-locality (a merge stays within one outer
  community). The gate's γ-density consequence is now modelled separately by
  `GammaMergeStep` (`Meso/GammaConnectivity.lean`), which adds the γ-dense-cut gate
  `γ S_a S_b ≤ e(a,b)`; the outer-locality witness is still recorded in `RefineStep`
  but not yet consumed by either proof.
- Go's refinement gate (`refineIntents`, `refine.go`) is objective-appropriate,
  which keeps the two Lean tiers separate on the Go side. Connectivity (a shared
  positive-weight edge, `w > 0`) is enforced for every objective, so the
  objective-generic connectivity guarantee (`connectedCommunities_of_refineRun`)
  holds regardless of the quality function. The γ-density gate is the CPM
  instantiation only: at the singleton base `refineIntents` runs over,
  `obj.moveDelta(base, v, u) > 0` is exactly the well-connectedness density
  criterion in the objective's own currency, and for CPM it is `γ s_v s_u < w_vu`,
  the strict form of the `GammaMergeStep` gate `γ S_a S_b ≤ e(a,b)`. For modularity
  it is the degree-based `γ k_v k_u / 2m < w_vu` instead, matching that
  modularity's objective is blind to node sizes; there is no modularity γ-density
  theorem in the Lean model, and none is claimed. Reading node sizes under
  modularity (the pre-fix behaviour, bug `wor-w33p`) had no Lean referent and broke
  the objective-generic connectivity guarantee whenever node weights were non-unit.
- γ-connectivity (C2) is modelled as internal γ-density (`IsGammaDense`) and its
  preservation under gated merges, not as the paper's stronger "no sparse cut"
  reading. Two deliberate consequences: (i) there is no singleton base (a lone node
  is not γ-dense), so the guarantee is preservation from a γ-dense partition, which is
  what the gate bootstraps; (ii) full no-sparse-cut γ-connectivity is not closed under
  pairwise merges (the cross-cut terms escape the piecewise bounds), so it is a
  convergence property and lands with subset-optimality (C3), not C2.
- Subset-optimality (C3) is stated from `IsSubsetStable` (no whole-subset reassignment
  improves CPM), a strictly stronger fixed point than the single-node local-move
  stability of `IsConverged` (`IsSubsetStable.isLocalMoveStable` recovers the latter as
  the singleton case). This is deliberate and faithful: it is the stability the Leiden
  _refinement_ targets by considering subset moves, which is why Leiden attains
  subset-optimality where plain Louvain does not. The delivered bound is the split-off
  half, `e(S, C\S) ≥ γ ‖S‖ ‖C\S‖` for every subset (the "no sparse cut" property C2
  deferred). The external absolute bound `e(S, D) ≤ γ ‖S‖ ‖D‖` for a subset against a
  different community `D` does not follow from subset stability of `C` alone (only the
  relative form does), so it is not claimed.
- The combined guarantee (`leidenGuarantees_of_stable`, C4) bundles three hypotheses
  (`IsLeidenStable`), one per guarantee, rather than deriving all three from one minimal
  fixed point: γ-separation
  needs level stability on the aggregate, γ-connectivity is a provenance property (a gated
  refinement run from a γ-well-connected base), and subset-optimality needs the stronger
  subset-stable fixed point. These constrain different structures, so a single minimal
  condition entailing all three is not claimed; each conjunct of `IsLeidenStable` is what
  the corresponding phase of the algorithm establishes.
- Synchronous-round confluence (`applyRound_perm`) proves _determinism_ — a round's
  outcome is independent of the schedule and core count — not that a round _improves_
  quality. The model is snapshot semantics: each node's target `t` is fixed from the
  round-start partition and never reads a mid-round write, which is what makes confluence
  hold with no disjointness hypothesis. Independent snapshot moves can conflict, so a
  synchronous round can oscillate; handling that and choosing the concrete parallel scheme
  is the still-open design of plan section 4.5. Quality monotonicity lives with the
  sequential local move (`Meso.Move`), not here. The Go round (`parallelRound`, mes-edpr)
  closes the oscillation gap the model leaves open with the "positive-gain acceptance with
  deterministic conflict resolution" mitigation of section 4.5: it uses `applyRound`
  verbatim for confluence, but accepts the whole moved set only when its exact realized
  delta strictly improves, otherwise deterministically falls back to the single
  highest-gain move (which is exactly monotone). Every changing round then raises the
  objective by more than `moveImproveEps`, so termination follows the same finite-range
  argument as the sequential loop. This monotonicity is a Go-side property beyond what the
  model proves; it is covered by `TestParallel_Convergence` on the corpus and on
  bipartite even-cycle swap traps, not by a Lean theorem.
- Termination is modelled as two well-founded-descent facts, not one whole-algorithm
  measure: no infinite strictly-improving local-move run (`no_infinite_improving_run`,
  resting on modularity's finite range) and no infinite strictly-decreasing level
  measure (`no_infinite_descending_levels`, with `numComm_le` bounding it by the node
  count). The level dichotomy is derived, not assumed (TODO Phase E2): a `LevelStep`
  either strictly lowers `numComm` or its partition is discrete
  (`levelStep_size_lt_or_injective`), and any infinite level run reaches such a fixed
  point (`levelRun_reaches_fixedPoint`). The former control-flow residual is now also
  closed by modelling the convergence guard: `RunningLevelStep` is a level the loop takes
  only while its partition is non-discrete, and `no_infinite_runningLevel_run` proves the
  guarded loop halts outright — no "reaches a fixed point" interpretation needed. The Go
  loop must implement that guard to inherit termination; section 4 spells this out.
  Whole-algorithm halting is the lexicographic combination of the two descents.

## 4. Matching the termination guard in Go

Termination has two Lean statements, and the Go implementation must match the stronger
one. `levelRun_reaches_fixedPoint` proves the recursion _reaches_ a discrete-partition
fixed point (the node count cannot descend forever). `no_infinite_runningLevel_run`
proves the _guarded_ loop halts outright, by making the convergence check part of the
model: `RunningLevelStep` fires only while the source partition is non-discrete. The bare
`LevelStep` relation permits idling at the fixed point (aggregating a discrete partition
returns a same-size graph and could recurse forever); the guard is what forbids it. The Go
outer loop must implement that guard, or it does not inherit the halting proof.

### What the guard is

A level is "running" (the loop may take another iteration) exactly while its local-move
phase produced a _non-discrete_ partition — at least one pair of nodes merged. These three
conditions coincide in the model, so Go may test whichever is cheapest:

- a node moved during the local-move sweep (the usual `moved` flag is `true`), or
- the produced partition has fewer communities than nodes (`numComm p < len(nodes)`), or
- the partition is not the all-singletons (discrete) partition.

When none holds — no node moved, equivalently `numComm p == len(nodes)`, equivalently the
partition is discrete — the level is a fixed point and the loop must **stop**.

### What Go must do to be faithful

1. The outer (aggregation) loop keeps an explicit stop condition and breaks as soon as a
   level's local-move phase changes nothing. Do not aggregate-and-recurse
   unconditionally: that is the unguarded `LevelStep` reading, which the model does not
   claim halts.
2. The stop test is the negation of the `RunningLevelStep` guard — continue only while the
   partition is non-discrete. A `moved` boolean threaded out of the local-move sweep, or a
   `numComm` before/after comparison, both match the model exactly.
3. Termination is then inherited, not re-argued: each running level strictly lowers the
   node count (`RunningLevelStep.size_lt`), so the loop runs at most `n` times for `n`
   input nodes. This bound is an assertable postcondition, not a hope.

### Guarantee tests

- `TestLeiden_RunningLevelShrinks` (planned): each level that the loop actually takes
  reduces the node count of the working graph by at least one — the Go image of
  `RunningLevelStep.size_lt`.
- `TestLeiden_GuardedLevelsTerminate` (planned): on the corpus and on fuzzed inputs, the
  loop (a) stops, (b) stops at a level whose local-move phase moved no node (the
  discrete-partition fixed point), and (c) executes at most `len(nodes)` levels — the Go
  image of `no_infinite_runningLevel_run`.

## See also

- [../../docs/specs/001-initial-implementation/meso-oracle.md](../../docs/specs/001-initial-implementation/meso-oracle.md) for the oracle
  design of record: the Lean value-oracle, the proven-invariant and accuracy
  checks that replace the differential envelope, and the one-time reference
  cross-check.
- [../../docs/research/meso-correspondence-and-differential-harness.md](../../docs/research/meso-correspondence-and-differential-harness.md)
  for the earlier harness sketch (the three bridges A/B/C and the computable-Lean
  oracle B'). Bridge C, the cross-language differential harness, is superseded by
  the decision recorded above; the computable-Lean oracle (B') is now the numeric
  oracle of record.
