# Directed modularity: guarantee triage (Phase 3)

The triage report of Phase 3 of the directed-modularity verification project
(research plan: `directed-modularity-formal-verification.md`; ticket
`mes-qmch`, parent epic `mes-crz3`). Phase 3 is research, not proof
engineering: for each of the three candidate directed guarantees it establishes
a verdict in {PROVABLE, REFUTED, OPEN} by mathematical derivation from the
Phase 2 identities plus an empirical counterexample search, and hands the
result to Phase 4 (`mes-jlz8`).

Verdict summary:

| Guarantee             | Directed statement                             | Verdict                                                         |
| --------------------- | ---------------------------------------------- | --------------------------------------------------------------- |
| Connectivity          | every returned community is weakly connected   | PROVABLE (sketch in section 4)                                  |
| Connectivity (strong) | every returned community is strongly connected | REFUTED (section 4)                                             |
| Gamma-separation      | conditional on level stability                 | PROVABLE (derivation in section 5)                              |
| Subset-optimality     | as a property of meso's converged output       | REFUTED (4-node fixture, section 6)                             |
| Subset-optimality     | conditional on subset stability                | PROVABLE-shaped, but the hypothesis is not attained (section 6) |

## 1. Setup

### The scout

`verification/reference/directed-scout/` is a standalone out-of-band Go module
(the Go analogue of `verification/reference/crosscheck/crosscheck.py`): committed and
reproducible, never in CI or the build gate. It uses meso only through the
public API (`NewDirectedBuilder`, `Leiden`, `DirectedModularity`,
`Result.Communities`, `Result.Quality`) and recomputes every quality value and
every candidate bound independently from the generated edge list and the
returned partition, so a shared formula bug cannot blind it. Run it with
`go run .` from its directory; `go run . -hunt-subset` reproduces the
counterexample of section 6.

### Parameter grid

3240 samples: node counts {5, 8, 12, 16, 24, 40}, arc densities
{0.1, 0.3, 0.5}, weight distributions {unit, integer 1..5, continuous (0,1]},
regimes {uniform, DAG (acyclic, maximal asymmetry), source/sink (nodes with no
in- or no out-arcs), cycle-seeded (strongly-connected-ish), symmetric}, with
and without self-loops, gamma in {0.5, 1, 2}, 2 seeds per cell. Seeds derive
deterministically from the grid coordinates (FNV-1a of the config string), so
every logged hit is reproducible from its config line alone.

### Convergence verification

A bound violation at a non-converged partition is a convergence artifact, not
a counterexample. The scout therefore verifies every judged partition at its
own (independently recomputed) objective: it applies best strictly-improving
single-node moves (to any occupied community or a fresh label, covering
isolation) and whole-community merges, priced from scratch, until a fixed
point. 396 of 3240 samples needed extra driving beyond meso's raw output (742
moves, 39 merges; 35 samples had an improving merge). Zero samples were
discarded. Gain guarantees are judged only at these verified fixed points;
connectivity is judged at meso's raw output, since it is a claim about what
Leiden returns.

### Self-consistency and convention checks

- The scout's independent directed modularity matches the hand-computed
  fixture values of `directed_quality_test.go` (also machine-checked in Lean,
  `verification/lean/Meso/DirectedCompute.lean`) to 1e-12.
- On every one of the 3240 samples the scout's Q of the returned partition
  agrees with meso's reported `Result.Quality()`: max absolute error 1.1e-16.
- igraph cross-check (python-igraph 1.0.0, `Graph.modularity(membership,
weights, resolution, directed=True)`): exact agreement on all four fixture
  cases (partitions all-in-one, singletons at gamma 1 and 2, two-community;
  values 0, -0.3125, -0.625, 0). meso optimises the standard Leicht-Newman
  objective, not a private convention, so "no counterexample found" verdicts
  below are about the standard objective.
- Symmetric-reduction gate: on symmetric directed graphs the directed bound
  expressions reduce to exactly twice the undirected modularity expressions on
  both sides (verified numerically), so every candidate statement below
  specialises to the classic undirected bound, as
  `directedModularity_toDirected_eq` requires.

## 2. Notation

`m` is the total arc weight (`totalWeight`; self-loops count once). For a
community `C`: `Kout_C = sum_{i in C} kout_i`, `Kin_C = sum_{i in C} kin_i`.
`e(A,B) = sum_{i in A, j in B} w_ij` is the ordered cut (in general
`e(A,B) != e(B,A)`). All statements are at resolution `gamma`.

## 3. The two gain closed forms (derived from Phase 2)

Both derive from the proved move identity `directedModularity_move_eq`
(`verification/lean/Meso/DirectedMove.lean`); neither needs new algebra.

**Merge gain.** On the directed aggregate (`directedAggregate`, whose
out/in-degrees are the community degrees and whose total weight is `m`,
`Meso/DirectedAggregate.lean`), merging community `C` into `D` is a single-node
move under the singleton partition. Instantiating the move identity there
(source is a singleton, so the source terms vanish and the self-loop is erased
by construction):

```text
gain_merge(C,D) = (1/m) * [ e(C,D) + e(D,C)
                            - (gamma/m) * (Kout_C*Kin_D + Kout_D*Kin_C) ]
```

**Split gain.** Splitting `S` (a proper nonempty subset of community `C`) off
to a fresh label flips the same-community indicator at exactly the cross pairs
`S x T` and `T x S`, `T = C \ S` (the directed analogue of `ind_subsetSplit`,
`Meso/SubsetOptimality.lean`), so with the directed kernel:

```text
gain_split(S) = -(1/m) * [ e(S,T) + e(T,S)
                           - (gamma/m) * (Kout_S*Kin_T + Kout_T*Kin_S) ]
```

Both forms were cross-checked against from-scratch Q differences on every
sample: maximum discrepancy 4.3e-15 (merge) and 7.1e-15 (split) across the
grid. The derivations are numerically exact.

## 4. Connectivity: PROVABLE as weak connectivity; strong is refuted

**Statement.** Every community meso's directed Leiden returns induces a weakly
connected subgraph: connected in the underlying undirected graph, with an edge
wherever `w_ij > 0` or `w_ji > 0` (self-loops are not connectivity edges).

**Scouting result.** 0 weak-connectivity violations across all 15228
communities returned in 3240 runs. Strong connectivity fails in 5880 of those
15228 communities (39 percent), decisively refuting any strong-connectivity
reading; the trivial witness is any community `{a, b}` joined by a single arc
`a -> b`, which the DAG regime returns routinely.

**Proof sketch (the Phase 4 handoff).** The undirected connectivity tier
transfers essentially verbatim, because it never touches edge weights beyond
the simple graph and never uses `weight_symm`:

1. Define `DirectedWeightedGraph.simpleGraph :=
SimpleGraph.fromRel (fun i j => 0 < G.weight i j)`. `fromRel` symmetrises
   and drops loops, so adjacency is exactly `(0 < w_ij or 0 < w_ji) and
i != j` - the weak-connectivity edge set. The undirected definition is
   textually identical (`Meso/Connectivity.lean`); there the symmetrisation is
   a no-op, here it does the work.
2. `CommunityConnected`, `ConnectedCommunities`, and
   `connectedCommunities_singleton` mention only the simple graph and
   partition combinatorics; restate over `DirectedWeightedGraph` unchanged.
3. The refinement chain (`Meso/Refinement.lean`: `mergeCommunities`,
   `MergeStep`, `RefineStep`, `connectedCommunities_of_mergeRun`,
   `connectedCommunities_of_refineRun`) uses no weight algebra at all, only
   `G.simpleGraph.Adj` and fiber bookkeeping; a grep confirms `weight_symm`
   appears nowhere in it. Re-parametrising by the directed graph goes through
   unchanged.
4. Go correspondence: the directed refinement gate (`refineIntents`,
   `refine.go`) admits a merge across a positive arc in either direction,
   which is precisely `fromRel`-adjacency, so the modelled gate matches the
   implemented one.

The only genuinely new Lean artifact Phase 4 needs is the directed
`simpleGraph` definition and the restatements; no new proof idea.

## 5. Gamma-separation: PROVABLE, conditional on level stability

**Statement.** At a level-stable partition (no community merge improves
directed modularity), any two distinct communities satisfy

```text
e(C,D) + e(D,C) <= (gamma/m) * (Kout_C*Kin_D + Kout_D*Kin_C).
```

Note the bidirectional cut on the left and both degree-product orientations on
the right; on a symmetric graph both sides halve into the classic undirected
bound `e(C,D) <= gamma * K_C * K_D / 2m`.

**Derivation.** Level stability says `gain_merge(C,D) <= 0` for every pair;
by the merge closed form of section 3 that inequality is literally the bound
(the `1/m` factor is nonnegative). The proof path is fully mechanical:
instantiate `directedModularity_move_eq` on `directedAggregate G p` at the
singleton partition, rewrite with `directedAggregate_outDegree`,
`directedAggregate_inDegree`, `directedAggregate_totalWeight`, and read the
sign. This mirrors exactly how the undirected CPM separation proof runs
(`Meso/Separation.lean`: `cpm_merge_two_singletons` then
`gammaSeparated_of_converged`), with the Phase 2 directed move identity in
place of the CPM merge lemma. Every step has a proved analogue in the tree.

**Scouting result.** 0 violations at all 3240 raw outputs and 0 at all
verified-converged partitions. The bound is tight, not vacuous: minimum
observed slack is 0 (equality is achieved, up to 1.1e-16 float noise). The
closed form matched from-scratch differences to 4.3e-15 everywhere.

**Undirected warm-up (research-doc open question 8).** The symmetric regime
(648 samples) is, by the reduction, exactly the undirected-modularity
gamma-separation question: 0 violations, minimum slack 0. So undirected
modularity gamma-separation from level stability is also empirically solid and
provable by the same one-step argument; it was never in the undirected model
only because the undirected guarantees were built in CPM (see the research
doc's CPM-only finding). The directed answer does not hinge on any undirected
failure.

**Caveat recorded.** meso's raw output is not always level-stable against the
scout's exhaustive merge scan: 35 of 3240 samples had an improving merge at
the raw output (meso stops at `moveImproveEps` and considers
neighbour-community candidates; the scout scans all pairs and all single-node
targets exhaustively). At none of those raw outputs was the separation bound
itself violated (0 raw violations), so the gap is between "stable up to meso's
epsilon and candidate set" and the model's exact `IsLevelStable`; the same gap
already exists undirected and is priced into stating guarantees from the
stability hypothesis rather than from "whatever the binary returns".

## 6. Subset-optimality: REFUTED as an output property

**Candidate statement.** For every proper nonempty subset `S` of every
returned community `C`, with `T = C \ S`:

```text
e(S,T) + e(T,S) >= (gamma/m) * (Kout_S*Kin_T + Kout_T*Kin_S).
```

Equivalently (split closed form, section 3): no subset split improves directed
modularity, `gain_split(S) <= 0`.

**Scouting result: REFUTED at meso's converged output.** 110 violating
subsets (55 distinct splits; each split is counted once as `S` and once as
`T`) in 43 communities across 43 of 3240 samples (1.3 percent), at partitions
verified stable under every single-node move and every community merge. Worst
observed violation slack -3.62 (split gain about +2.6e-3 in Q), far above
float noise. Subset checks were exhaustive for communities up to 12 members
and sampled (500 random subsets) above; the count therefore lower-bounds the
true violation rate for large communities.

**Minimal fixture (n = 4, hand-verified).** Reproduce with
`go run . -hunt-subset`. Arcs (`i -> j : w`):

```text
0 -> 1 : 1
0 -> 3 : 1
1 -> 0 : 3
1 -> 2 : 1
1 -> 3 : 5
2 -> 0 : 2
3 -> 2 : 5
```

`m = 18`, `gamma = 1`, converged partition all-in-one `[0,0,0,0]` (verified:
no single-node move, including isolation, and no merge improves; its Q is 0,
consistent with `directedModularity_const`). Take `S = {0,1}`, `T = {2,3}`:
`Kout_S = 11`, `Kin_S = 6`, `Kout_T = 7`, `Kin_T = 12`, `e(S,T) = 7`,
`e(T,S) = 2`, so the bound reads `9 >= 174/18 = 9.667`, which is false; the
split gain is `(174/18 - 9)/18 = 1/27 ~ +0.0370`, and the two-community
partition `{0,1},{2,3}` indeed scores `1/27 > 0`. The partition is a genuine
local optimum of the move-and-merge dynamics that a subset move escapes.

**This is not a directed phenomenon.** The symmetric regime alone (the
undirected-modularity case, by the reduction) shows 18 violating subsets in 8
of 648 samples. Converged-but-not-subset-stable outputs are a modularity
behaviour meso's directed support inherits, not a directed regression. It is
also consistent with the model's undirected scope: subset-optimality is proved
there in CPM only, from the assumed hypothesis `IsSubsetStable`
(`isSubsetOptimal_of_stable`, `Meso/SubsetOptimality.lean`); no
modularity-flavoured subset-optimality was ever claimed, undirected or
directed.

**The conditional statement survives.** "Directed subset stability implies the
bound" is sound and essentially definitional given the split closed form: the
bound is `gain_split(S) <= 0` rearranged, and the closed form was verified to
7.1e-15 against from-scratch differences. Phase 4 can formalize it mechanically
by proving the subset analogue of the Phase 2 move identity (a
`moveSubset` version of `sum_kernel_move_split` and
`directedModularity_move_eq`, following `ind_subsetSplit`). But the hypothesis
is not attained by the algorithm on 1.3 percent of scouted samples, so the
conditional would be a theorem about a fixed point meso does not reach in
general; whether that is worth formalizing is a Phase 4 scoping decision, not
a mathematical one.

## 7. Modeling prerequisites for Phase 4

The undirected stability layer is tied to the undirected graph type:
`QualityFamily := forall m, WeightedGraph m -> Partition m -> R`
(`Meso/Convergence.lean`), and `IsLocalMoveStable`, `IsLevelStable`,
`IsConverged`, `IsSubsetStable` are built on it. Phase 4 needs directed
homes for the surviving statements:

- `IsLocalMoveStable` is already graph-agnostic (it constrains
  `Q : Partition n -> R`); it can be reused as-is with
  `Q := directedModularity G gamma`.
- `IsLevelStable`/`IsConverged` need a directed variant, since they mention
  `aggregate`. Recommendation: a standalone
  `IsDirectedLevelStable (G) (p) := IsLocalMoveStable
(directedModularity (directedAggregate G p) gamma) (fun A => A)` (and the
  conjunction for converged), not a shared `QualityFamily` abstraction over
  both graph types. The directed development has deliberately stayed
  standalone (research-doc open question 5, model unification), only two
  quality functions exist, and a two-instance typeclass would be an
  unrequested abstraction; revisit only if a third graph model appears.
- Gamma-separation (section 5) then needs: the merge-gain instantiation of
  `directedModularity_move_eq` at the aggregate singleton partition (a small
  lemma in the `Separation.lean` style), plus the sign-reading theorem.
- Weak connectivity (section 4) needs: `DirectedWeightedGraph.simpleGraph`
  and directed restatements of the `Connectivity.lean`/`Refinement.lean`
  chain, no new proof ideas.
- Subset-optimality: if Phase 4 chooses to state the conditional, it needs
  `moveSubset`-analogues of the Phase 2 identity; the REFUTED-as-output
  verdict and the n=4 fixture should be recorded in the CORRESPONDENCE
  divergence register either way.

## 8. Handoff to Phase 4 (`mes-jlz8`)

1. **Formalize weak connectivity** (section 4 sketch): directed `simpleGraph`,
   restated definitions, transferred merge-run chain. Expected mechanical.
2. **Formalize gamma-separation** (section 5): directed stability definitions
   (section 7), merge-gain lemma, sign theorem. Expected mechanical from the
   Phase 2 assets.
3. **Record subset-optimality as a divergence**: the n=4 fixture and the
   1.3 percent rate, in the CORRESPONDENCE divergence register; optionally
   formalize the conditional (subset identity first). The fixture should also
   graduate into a committed Go regression test documenting the behaviour
   (converged output admits an improving subset split), which is a Phase 4 Go
   change, out of Phase 3 scope.
4. **Strong connectivity**: record as refuted-by-design (39 percent failure
   rate; single-arc pairs are legitimate communities); no formal work.

Every statement above is self-contained (exact bounds, exact hypotheses) so
Phase 4 can quote it without re-deriving.
