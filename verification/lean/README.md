# meso formal verification (Lean 4)

Model-level formal verification of the Leiden/Louvain guarantees, per section 7
of [the plan](../../docs/meso-design.md). This is the optional depth tier;
it does not gate the Go library.

## The one thing to understand first

**Lean verifies a model, not the Go binary.** Lean cannot read or emit Go, so
this development is a standalone mathematical description of the algorithm and its
properties, derived from the Traag-Waltman-van Eck (2019) paper. The shipped Go is
held faithful to this model by the section-6 test suite (the Lean value-oracle,
property, and formal-guarantee checks), not by a compiler. Only data-race freedom
is verified on the real Go, via Gobra (`verification/gobra/`). The model is also `meso`'s numeric
oracle: a computable rational mirror of the quality functions (Phase F) emits
exact golden vectors the Go tests check against, which
retires the earlier cross-language differential harness (see
[../../docs/specs/001-initial-implementation/meso-oracle.md](../../docs/specs/001-initial-implementation/meso-oracle.md)). The
model-to-code bridge itself (which Lean definition maps to which Go symbol, and
which theorem to which Go test) is tracked in
[CORRESPONDENCE.md](CORRESPONDENCE.md).

Both the Lean model and the Go implementation derive from the same source (the
paper's pseudocode), so they converge on a shared spec rather than one driving the
other.

## Layout

| File                          | Contents                                                                                                                                                                 |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `Meso.lean`                   | Root; imports every module.                                                                                                                                              |
| `Meso/Graph.lean`             | Weighted-graph model over `Fin n`: symmetric nonnegative weights, node sizes, weighted degree, `2m`.                                                                     |
| `Meso/Quality.lean`           | `Partition`, modularity with resolution parameter, and stepwise-to-global monotonicity for any quality function.                                                         |
| `Meso/Move.lean`              | The single-node local-move operator, parameterised by a quality function; a best move never lowers quality.                                                              |
| `Meso/Aggregate.lean`         | The δ-regrouping identity, modularity as a community sum, and the re-indexed aggregate graph with its invariance.                                                        |
| `Meso/Level.lean`             | Per-level and whole-algorithm monotonicity, generic over the quality family: quality threaded across every level.                                                        |
| `Meso/Connectivity.lean`      | Underlying simple graph, the community-connectivity predicate, and the singleton base case.                                                                              |
| `Meso/Refinement.lean`        | The edge-merge and refinement operators; every refinement run from singletons yields connected communities.                                                              |
| `Meso/Termination.lean`       | Both loops halt: the local-move phase (modularity's finite range) and the multilevel recursion (community count).                                                        |
| `Meso/CPM.lean`               | The Constant Potts Model quality function, its `e_c − γ S_c²` community decomposition, and γ-density.                                                                    |
| `Meso/Convergence.lean`       | The converged partition: local-move + level stability over any quality family, and fixed-point adequacy lemmas.                                                          |
| `Meso/Separation.lean`        | γ-separation: the CPM merge-gain formula and that a converged partition's communities are γ-separated.                                                                   |
| `Meso/GammaConnectivity.lean` | γ-connectivity: the γ-density arithmetic and that a gated refinement run keeps communities connected and γ-dense.                                                        |
| `Meso/SubsetOptimality.lean`  | subset-optimality: the subset split-off gain formula and that a subset-stable partition has no sparse cut.                                                               |
| `Meso/Guarantees.lean`        | the combined guarantee: a converged Leiden output satisfies γ-separation, γ-connectivity, and subset-optimality.                                                         |
| `Meso/Round.lean`             | synchronous-round confluence: a parallel local-move round's outcome is schedule- and core-count-independent.                                                             |
| `Meso/Compute.lean`           | Computable rational mirror of the quality functions (value-oracle F1) and its proved equivalence to the model (F2).                                                      |
| `Meso/Reachability.lean`      | A computable, proved reachable-set closure (fixpoint of a BFS step) — the efficient connectivity decider's engine.                                                       |
| `Meso/Predicates.lean`        | Computable Bool mirrors of the guarantee predicates (connectivity, γ-density, γ-separation, subset-optimality), each proved equal to its predicate (value-oracle G1-G3). |
| `Meso/OracleIO.lean`          | The `mesoOracle` executable's IO: reads fixtures, evaluates the mirrors, emits exact golden vectors and predicate flags (F3, G4).                                        |
| `CORRESPONDENCE.md`           | Audited model-to-code bridge: representation map, theorem-to-test table, and divergence register.                                                                        |

The paper-theorems tier is complete (`Meso/Guarantees.lean` conjoins the three paper
guarantees), and both halves of the concurrency track are done: the design half is
proved here (`Meso/Round.lean`), and the code half - data-race freedom and memory
safety of the parallel Go fork-join - is machine-checked by Gobra against the shipped
`parallel.go` (see `verification/gobra/README.md` for scope, trusted assumptions, and
how to run it). A set of
follow-up hardening items (Phase E) are all now closed: CPM
aggregation invariance is proved (`cpm_aggregate_eq`) and CPM is threaded through whole-run
monotonicity (`cpm_le_of_algorithmRun`, via a quality-family-generic `Meso/Level.lean`); the
whole-algorithm termination measure is derived rather than assumed, and the guarded loop
halts outright once the convergence check is modelled (`no_infinite_runningLevel_run`); and
the directed-model question is decided — deliberately descoped (directed stays
reference-tested, not verified; see [CORRESPONDENCE.md](CORRESPONDENCE.md)). Two further phases
are complete: Phase F, the computable value-oracle (promoted when the Lean model became
`meso`'s sole standing numeric oracle, retiring the cross-language differential harness), and
Phase G, the predicate vectors (computable Bool mirrors of the guarantee predicates, each
proved equal to its predicate and emitted as boolean golden vectors).

## Roadmap (plan section 7)

- **Design invariants.** _Complete._ Partition well-formedness (structural: a `Partition`
  is a total function), quality monotone across aggregation levels (stepwise non-decrease
  within a local-move run, aggregation invariance, and the whole-algorithm thread
  `modularity_le_of_algorithmRun`), connectivity of the refinement operator's output, and
  termination of both loops.
- **Concurrency.** _Complete._ _Design:_ a synchronous round is confluent
  (schedule- and core-count-independent), proved in `Meso/Round.lean`; it informed the
  chosen parallel scheme (plan section 4.5). _Code:_ data-race freedom and memory safety
  of the parallel Go fork-join, machine-checked in the Gobra track
  (`verification/gobra/README.md`; the only proof touching the shipped artifact).
- **The paper theorems.** _Complete._ γ-separation (`gammaSeparated_of_converged`),
  γ-connectivity (`gammaWellConnectedCommunities_of_gammaMergeRun`), and subset-optimality
  (`isSubsetOptimal_of_stable`) are all machine-checked, and `leidenGuarantees_of_stable`
  conjoins the three for a converged Leiden output. A whole-development `#print axioms`
  sweep confirms no `sorryAx` anywhere.

## Toolchain

- Lean `v4.31.0` (pinned in `lean-toolchain`).
- Mathlib `v4.31.0` (pinned in `lakefile.toml`), installed via `elan` / `lake`.

Mathlib's release cadence drives the Lean version: to bump, change the Mathlib
`rev` in `lakefile.toml`, run `lake update`, then re-fetch the cache.

## Build

```sh
cd verification/lean
lake exe cache get   # download Mathlib's prebuilt oleans (a few GB); first time only
lake build           # type-check the whole development
```

A green `lake build` with no `sorry` warnings means every stated theorem is
proved relative to its definitions. To confirm a specific result depends on no
placeholder axioms:

```sh
echo 'import Meso
#print axioms Meso.quality_monotone_of_stepwise' | lake env lean --stdin
```

You want to see only `propext`, `Classical.choice`, `Quot.sound`, never
`sorryAx`.

## Working style

- Develop interactively in VS Code with the `leanprover.lean4` extension: the live
  goal state is the whole experience.
- The Lean kernel is the external authority. A wrong proof does not compile, so
  AI-drafted tactic blocks are safe to iterate against `lake build` output. The
  real risk is a wrong or vacuous _theorem statement_, not a wrong proof, so
  review the statements carefully; that is where the human judgement goes.
- Find Mathlib lemmas with `exact?`, `apply?`, `rw?`, and Loogle / LeanSearch.
- GPLv3 headers, not Mathlib's Apache headers; the Apache header linter is
  disabled in `lakefile.toml`.

## Status

Graph and quality models in place. **The local-move phase is proved monotone end
to end**, along with the supporting results, all resting only on the standard
axioms (no `sorryAx`):

The local-move machinery is parameterised by a quality function `Q : Partition n →
ℝ`, so modularity and CPM share one proof of monotonicity:

- `quality_monotone_of_stepwise` - global monotonicity from stepwise non-decrease,
  for any `Q` (uses only transitivity of `≤`).
- `move_self` - moving a node to its own community is a no-op.
- `move_best_ge` - a best single-node move never lowers `Q` (staying put is always
  an option); `modularity_bestMove_ge` / `cpm_bestMove_ge` are its instances.
- `IsLocalMove Q` / `IsLocalMove.le` - one local-move step and its non-decrease, for
  any `Q`; `IsLocalMove.modularity_le` is the modularity specialisation.
- `localMoveRun_monotone` - any run (chain) of local-move steps is globally monotone
  in `Q`; discharges the `IsChain` hypothesis of `quality_monotone_of_stepwise`.

**Aggregation is proved invariant end to end** (the heavy sum-algebra plus the
`Fin m` re-indexing bookkeeping):

- `sum_diagonal_eq_sum_fiber` - the Kronecker-δ regrouping: a double sum over
  same-community node pairs equals a sum over communities of within-community
  block sums.
- `modularity_eq_communitySum` - modularity as a sum of within-community
  contributions, the form the aggregate graph's self-loops realise.
- `aggregate` - the re-indexed aggregate `WeightedGraph` on `Fin (numComm p)`,
  with `aggregate_degree` (derived degree = community degree) and
  `aggregate_twoM` (total weight preserved).
- `modularity_aggregate_eq` - the graph-to-graph invariance: the singleton
  partition on the aggregate has exactly the modularity of `p` on `G`. Both
  halves of stepwise monotonicity (local move and aggregation) are now proved.

**The two halves are joined across the aggregation boundary** (generic over the
quality family, so modularity and CPM share it):

- `IsAggregationInvariant` - the one graph-to-graph fact a level boundary consumes:
  scoring the aggregate's singleton partition returns the original score. Both
  `modularity_aggregate_eq` and `cpm_aggregate_eq` discharge it.
- `le_aggregate_run` - after aggregating `p` and running local moves on the aggregate,
  quality never drops below `Qf G p`. This is one level: aggregation invariance pins
  the aggregate's singleton to `p`'s value, and `localMoveRun_monotone` shows the run
  only climbs from there. `le_modularity_aggregate_run` / `le_cpm_aggregate_run` are
  the instances.
- `Config` / `configQuality` / `LevelStep` / `configQuality_monotone_of_levelRun` -
  the multilevel recursion as a relation on size-bundled configurations, with quality
  threaded across a whole run by transitivity, parameterised by the quality family.
- `qualityFamily_le_of_algorithmRun` - **the whole-algorithm guarantee: the
  recursion's output never lowers quality**, from an initial `(G, p)` to any final
  iterated-aggregate `(G', q)`. `modularity_le_of_algorithmRun` and
  `cpm_le_of_algorithmRun` are the modularity and CPM instances.

The monotonicity story is now complete end to end: monotone within a
local-move run, invariant across aggregation, non-decreasing through the level
boundary that joins them, and threaded across all levels into one whole-algorithm
theorem.

**The connectivity framework is in place** (on Mathlib's `SimpleGraph`):

- `WeightedGraph.simpleGraph` - the underlying simple graph (edge where weight is
  positive; self-loops excluded).
- `CommunityConnected` / `ConnectedCommunities` - a community induces a connected
  subgraph; the guarantee as a partition predicate.
- `communityConnected_of_subsingleton` and `connectedCommunities_singleton` - the
  base case: single-node communities (hence the singleton partition) are
  connected.

**Refinement preserves connectivity, so the guarantee holds for any refinement
run:**

- `connected_induce_union_of_adj` - two connected induced subgraphs sharing an
  edge form a connected whole (the graph-theoretic core).
- `mergeCommunities` / `MergeStep` - the edge-merge operator: join two distinct
  communities that share an edge.
- `MergeStep.connectedCommunities` - a merge preserves `ConnectedCommunities`.
- `connectedCommunities_of_mergeRun` - any partition reachable from singletons by
  edge-merges has connected communities.
- `RefineStep` / `RefineStep.mergeStep` - the refinement operator's per-step gate
  (outer-locality plus a shared edge), and that every such step is an edge-merge.
- `refineRun_isMergeRun` - the operator's output is a valid edge-merge run.
- `connectedCommunities_of_refineRun` - the refinement operator run from singletons
  has connected communities. **This closes the connectivity guarantee for the
  operator, not merely for abstract edge-merge runs.** The gate's γ-density content
  (beyond the shared edge) governs quality and the γ-bounds, not connectivity, and
  lands with CPM in the paper-theorems tier.

**Termination — both loops halt:**

- `modularityKernel` / `modularity_eq_kernel` / `finite_range_modularity` -
  modularity factors through the finite "same community" kernel `Fin n → Fin n →
Bool`, so it has finite range. This is the local-move measure's well-foundedness.
- `no_infinite_improving_run` - no infinite strictly-improving sequence of
  partitions exists (an injection from `ℕ` into that finite range).
- `IsAcceptedMove` / `no_infinite_acceptedMove_run` - the fast local-move phase
  halts: each accepted move strictly improves quality.
- `numComm_le` - the community count is bounded (`numComm p ≤ n`); the partition
  lattice is finite.
- `no_infinite_descending_levels` - the multilevel recursion halts: a
  strictly-decreasing `ℕ` level measure (instantiated by `numComm`) cannot run
  forever. Whole-algorithm halting is the lexicographic combination of the two.
- `numComm_eq_iff_injective` / `levelStep_size_lt_or_injective` - the level dichotomy,
  **derived rather than assumed**: `numComm p = n` exactly when `p` is discrete
  (injective), so a level either strictly lowers the node count or its partition is that
  discrete fixed point where aggregation does nothing.
- `no_infinite_productiveLevel_run` / `levelRun_reaches_fixedPoint` - the node count is the
  concrete level measure, and any infinite level run must reach a discrete-partition fixed
  point (the node count cannot strictly descend forever). This closes the former "a
  productive level lowers `numComm`" assumption.
- `RunningLevelStep` / `no_infinite_runningLevel_run` - **the guarded loop halts outright.**
  Folding the convergence check into the model (a level is "running" only while its
  partition is non-discrete) removes even the control-flow residual: the guarded loop
  provably cannot run forever, no "reaches a fixed point" interpretation needed. This is the
  level-loop analogue of `no_infinite_acceptedMove_run`. How the Go loop must implement this
  guard to inherit termination is spelled out in
  [CORRESPONDENCE.md](CORRESPONDENCE.md) section 4.

**CPM — the second quality function (Phase B):**

- `cpm` - the Constant Potts Model quality, with a node-size penalty `γ s_i s_j` in
  place of modularity's degree-based term.
- `cpm_bestMove_ge` / `cpm_localMoveRun_monotone` - CPM local-move monotonicity, free
  from the shared quality-function interface (no re-proof).
- `communityInternalWeight` / `communitySize` / `cpm_eq_communitySum` - CPM as a sum
  of per-community contributions `e_c − γ S_c²`, reusing `sum_diagonal_eq_sum_fiber`
  and `block_sub_sq`.
- `cpm_aggregate_eq` - CPM aggregation invariance (the CPM twin of
  `modularity_aggregate_eq`): the singleton partition on the aggregate has exactly the CPM
  of `p` on `G`, so a recursion may continue on the aggregate without losing CPM.
- `le_cpm_aggregate_run` / `cpm_le_of_algorithmRun` - CPM joins the whole-run monotonicity
  story: one level does not lose CPM, and the whole multilevel run never lowers it. Both are
  instances of the quality-family-generic `le_aggregate_run` /
  `qualityFamily_le_of_algorithmRun` (see below), discharging aggregation invariance with
  `cpm_aggregate_eq`.
- `IsGammaDense` / `isGammaDense_iff` - the γ-density notion (community contributes
  nonnegatively to CPM), the object the γ-connectivity guarantee will build on.

**The converged partition — the fixed point the guarantees are stated about (Phase B):**

- `IsLocalMoveStable Q p` - no single-node move strictly improves `Q` (the `∀ v c`
  local optimum); `IsLevelStable` / `IsConverged` add "no aggregate move" via the
  same notion on the aggregate's singleton partition. Defined over a `QualityFamily`
  (`modularityF`, `cpmF`) so modularity and CPM share one convergence definition.
- `IsLocalMove.quality_eq_of_stable` / `IsLocalMoveStable.no_strict_improvement` -
  adequacy: a stable partition is a genuine fixed point of the move dynamics, so the
  definition is not vacuous (the judgement-critical check for Phase C).

**γ-separation - the first paper theorem (Phase C):**

- `cpm_merge_two_singletons` - the CPM move-gain formula at the aggregate level:
  merging two singleton communities changes CPM by exactly `2·(w_AB − γ s_A s_B)`.
  The only "same community" cells that change are the merged pair `(A,B)` and `(B,A)`
  (`ind_singletonMerge`).
- `cpm_gammaSeparated_of_stable` - if a graph's singleton partition is CPM
  local-move stable (no merge helps), any two distinct nodes are γ-separated:
  `w_AB ≤ γ s_A s_B`.
- `gammaSeparated_of_converged` / `gammaSeparated_blockWeight_of_converged` - **the
  γ-separation guarantee**: a converged partition's distinct communities satisfy
  `e(C,D) ≤ γ S_C S_D`. It rests on _level_ stability (no community merge improves),
  the second half of `IsConverged`; single-node stability alone gives only the weaker
  relative `cpm_noStrictlyBetterCommunity` (no node has a strictly better community).

**γ-connectivity - the second paper theorem (Phase C):**

- `gammaDense_union` - the density arithmetic: a γ-dense cut joins two internally
  γ-dense node sets into a γ-dense union (`e(C₁∪C₂) = e_{C₁} + e_{C₂} + 2 e(C₁,C₂)`),
  the CPM cousin of `connected_induce_union_of_adj`.
- `GammaMergeStep` / `GammaMergeStep.gammaDenseCommunities` - the γ-gated refinement
  merge (shared edge + γ-dense cut) and that it preserves internal γ-density of every
  community; `GammaMergeStep.mergeStep` routes connectivity through Phase A.
- `gammaWellConnectedCommunities_of_gammaMergeRun` - **the γ-connectivity
  guarantee**: a gated refinement run keeps every community connected and internally
  γ-dense. The base is a γ-dense partition, not singletons (a lone node is not
  γ-dense); the gate bootstraps density by only merging across γ-dense cuts. The
  paper's stronger "no sparse cut" reading is a convergence property, deferred to
  subset-optimality (C3).

**subset-optimality - the third paper theorem (Phase C):**

- `moveSubset` / `cpm_moveSubset_split` - the subset move (reassign a whole node set)
  and its CPM gain when splitting a subset `S` of community `C` off to a fresh label:
  exactly `−2·(e(S, C\S) − γ ‖S‖ ‖C\S‖)`, since only the cross pairs `S × (C\S)` leave
  the community (`ind_subsetSplit`).
- `IsSubsetStable` / `IsSubsetStable.isLocalMoveStable` - the subset-level fixed point
  (no subset reassignment improves CPM), strictly stronger than single-node stability
  (recovered as the singleton case) and the stability the Leiden refinement targets.
- `cpm_subsetGammaDense_of_stable` / `isSubsetOptimal_of_stable` - **the
  subset-optimality guarantee**: at a subset-stable partition every subset `S` of every
  community `C` satisfies `e(S, C\S) ≥ γ ‖S‖ ‖C\S‖` (`IsSubsetOptimal`). No subset can be
  split off to raise quality, so the community is γ-densely connected at every scale.
  This is the "no sparse cut" reading deferred from C2. The external absolute bound
  `e(S, D) ≤ γ ‖S‖ ‖D‖` against another community `D` does not follow from subset
  stability of `C` alone and is not claimed (divergence register).

**The combined guarantee (Phase C):**

- `IsLeidenStable` - a converged Leiden output: converged (`IsConverged`, for
  γ-separation), arisen from a gated refinement run out of a γ-well-connected base (for
  γ-connectivity), and subset-stable (`IsSubsetStable`, for subset-optimality). One
  hypothesis per guarantee, each what a phase of the algorithm establishes.
- `LeidenGuarantees` / `leidenGuarantees_of_stable` - **the combined guarantee**: a
  converged Leiden output satisfies all three paper guarantees at once - γ-separation,
  γ-well-connectedness, and subset-optimality - each discharged from the matching
  hypothesis by the first, second, and third paper theorems.

**Synchronous-round confluence (concurrency track, design half):**

- `move_comm` - two single-node moves at distinct nodes commute (`Function.update_comm`),
  the conflict-free case underlying round confluence.
- `applyRound` / `applyRound_eq` - a synchronous round folds the snapshot-decided moves
  `move · v (t v)` over a work-list; its closed form is `t` on the moved nodes and `p`
  elsewhere, a function of the moved-node _set_ and the snapshot alone.
- `applyRound_perm` - **the confluence guarantee**: a round's outcome is independent of the
  schedule, hence of core count (any two schedules of the same work are permutations, and
  membership is permutation-invariant). This proves determinism, not quality improvement;
  the snapshot semantics and the still-open parallel scheme (plan 4.5) are noted in the
  divergence register.

**The value-oracle — computable mirrors and predicate vectors (Phases F and G):**

- `WeightedGraphQ` / `modularityQ` / `cpmQ` / `cpmCanonicalQ` and `modularityQ_eq` /
  `cpmQ_eq` / `cpmCanonicalQ_eq` (`Meso/Compute.lean`) - computable rational mirrors of the
  quality functions, proved equal to the real model, so the runnable oracle is the proved
  artifact. `mesoOracle` (`Meso/OracleIO.lean`) emits exact golden vectors from committed
  fixtures.
- `GammaDenseCommunitiesQ` / `GammaSeparatedCommunitiesQ` / `SubsetOptimalQ` and their
  `_iff` theorems (`Meso/Predicates.lean`) - decidable Bool mirrors of the guarantee
  predicates over rational block aggregates, each proved to decide the real predicate.
- `ConnectedCommunitiesFast` / `connectedCommunitiesFast_iff` - the connectivity flag,
  decided by a proved reachable-set closure (`reachableFinset`, `mem_reachableFinset_iff` in
  `Meso/Reachability.lean`) that runs at corpus scale where Mathlib's walk-enumeration
  `Decidable Connected` cannot. `mesoOracle` emits all four predicate flags per case; the
  connectivity flag is cross-checked against igraph (`verification/reference/crosscheck`).

The paper-theorems tier is complete. Design invariants (monotonicity end to end, connectivity, termination), CPM and the converged partition, γ-separation, γ-connectivity, subset-optimality, and the combined guarantee are all proved, and a whole-development `#print axioms` sweep confirms every result rests only on `propext`, `Classical.choice`, and `Quot.sound` - no `sorryAx`. On the concurrency track, the design half (synchronous-round confluence, `Meso/Round.lean`) is proved, and the code half (data-race freedom of the parallel fork-join) is machine-checked in the Gobra track (`verification/gobra/README.md`).
