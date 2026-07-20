/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.Move
import Meso.Aggregate

/-!
# Per-level monotonicity across the aggregation boundary

This ties the two halves of stepwise monotonicity together across the one
place they meet: the aggregation boundary of a Louvain/Leiden level.

Within a level, a local-move run is monotone on whichever graph it runs
(`localMoveRun_monotone`, feeding `quality_monotone_of_stepwise`). The subtlety is
the boundary: aggregation moves to a *different* graph on a *different* index set,
so the two runs cannot live in one `List (Partition n)`. Aggregation invariance is
the bridge: the aggregate's singleton partition starts at exactly the quality of `p`
on `G`, so a local-move run on the aggregate can only climb from there. Chaining the
two gives multilevel monotonicity: quality never drops as the recursion descends
through aggregation levels.

Everything here is generic over the quality function. The one graph-to-graph fact a
level boundary consumes is aggregation invariance (`IsAggregationInvariant`), and
`localMoveRun_monotone` is already generic, so the whole-run monotonicity is proved
once and instantiated: for modularity below (`modularity_aggregate_eq`), and for CPM
in `Meso.CPM` (`cpm_aggregate_eq`).
-/

namespace Meso

variable {n : ℕ}

/-- **Aggregation invariance of a quality family.** Scoring the aggregate graph's
    singleton partition (each community collapsed to its own node) returns the score
    of `p` on the original graph. This is the one graph-to-graph fact the level
    boundary needs — it pins the aggregate's starting value — and both quality
    functions satisfy it (`modularity_aggregate_eq`, `cpm_aggregate_eq`), so the
    whole-run monotonicity below holds for each. `Qf` is a graph-indexed quality
    function (the `QualityFamily` of `Meso.Convergence`), kept abstract so one proof
    serves both. -/
def IsAggregationInvariant (Qf : ∀ m, WeightedGraph m → Partition m → ℝ) : Prop :=
  ∀ {m : ℕ} (G : WeightedGraph m) (p : Partition m),
    Qf (numComm p) (aggregate G p) (fun A => (A : ℕ)) = Qf m G p

/-- **A level does not lose quality, for any aggregation-invariant quality family.**
    Starting from partition `p` on `G` (the result of the previous level), aggregate
    and run local moves on the aggregate: for every partition `q` reached in that run,
    the aggregate's quality at `q` is at least `Qf n G p`.

    The run is a chain of `IsLocalMove` steps starting at the singleton partition
    (each aggregate node its own community). Aggregation invariance pins the
    singleton's value to `Qf n G p`; `localMoveRun_monotone` shows the run never
    descends below its start. -/
theorem le_aggregate_run
    (Qf : ∀ m, WeightedGraph m → Partition m → ℝ) (hinv : IsAggregationInvariant Qf)
    (G : WeightedGraph n) (p : Partition n)
    (qs : List (Partition (numComm p))) (q : Partition (numComm p))
    (hchain : (((fun A => (A : ℕ)) : Partition (numComm p)) :: qs).IsChain
                (IsLocalMove (Qf (numComm p) (aggregate G p))))
    (hmem : q ∈ ((fun A => (A : ℕ)) : Partition (numComm p)) :: qs) :
    Qf n G p ≤ Qf (numComm p) (aggregate G p) q := by
  have hpair := localMoveRun_monotone (Qf (numComm p) (aggregate G p)) _ hchain
  rw [← hinv G p]
  rcases List.mem_cons.mp hmem with rfl | hq
  · exact le_refl _
  · exact (List.pairwise_cons.mp hpair).1 q hq

/-- `le_aggregate_run` specialised to modularity (aggregation invariance via
    `modularity_aggregate_eq`). -/
theorem le_modularity_aggregate_run
    (G : WeightedGraph n) (γ : ℝ) (p : Partition n)
    (qs : List (Partition (numComm p))) (q : Partition (numComm p))
    (hchain : (((fun A => (A : ℕ)) : Partition (numComm p)) :: qs).IsChain
                (IsLocalMove (modularity (aggregate G p) γ)))
    (hmem : q ∈ ((fun A => (A : ℕ)) : Partition (numComm p)) :: qs) :
    modularity G γ p ≤ modularity (aggregate G p) γ q :=
  le_aggregate_run (fun _ H r => modularity H γ r)
    (fun G p => modularity_aggregate_eq G γ p) G p qs q hchain hmem

/-!
## Whole-algorithm monotonicity

`le_aggregate_run` is one level. The recursion iterates it, each level aggregating to
a *smaller* graph on a *different* index set, so the levels cannot share a `Fin`. A
configuration is therefore bundled with its size, and a level is a step of a relation
on those bundles; monotonicity threads across a whole run by transitivity of `≤` on
the quality value.
-/

/-- A configuration the multilevel recursion passes through: a graph paired with the
    partition that level produced. The node count shrinks each aggregation, so the
    size is bundled in. -/
abbrev Config := Σ m : ℕ, WeightedGraph m × Partition m

/-- The quality of a configuration under a quality family `Qf`: its partition's score
    on its graph. -/
def configQuality (Qf : ∀ m, WeightedGraph m → Partition m → ℝ) (c : Config) : ℝ :=
  Qf c.1 c.2.1 c.2.2

/-- **One level of the recursion.** From configuration `c`, aggregate along its
    partition and run local moves on the aggregate to a new partition `q` (a chain
    of `IsLocalMove` from the singleton, `q` one of its members); the next
    configuration carries the aggregate graph and `q`. Parameterised by the quality
    family the local moves optimise. -/
def LevelStep (Qf : ∀ m, WeightedGraph m → Partition m → ℝ) (c d : Config) : Prop :=
  ∃ (qs : List (Partition (numComm c.2.2))) (q : Partition (numComm c.2.2)),
    (((fun A => (A : ℕ)) : Partition (numComm c.2.2)) :: qs).IsChain
        (IsLocalMove (Qf (numComm c.2.2) (aggregate c.2.1 c.2.2)))
      ∧ q ∈ (((fun A => (A : ℕ)) : Partition (numComm c.2.2)) :: qs)
      ∧ d = ⟨numComm c.2.2, aggregate c.2.1 c.2.2, q⟩

/-- A single level does not lower quality: this is `le_aggregate_run` lifted to
    configurations. -/
theorem LevelStep.le_configQuality
    (Qf : ∀ m, WeightedGraph m → Partition m → ℝ) (hinv : IsAggregationInvariant Qf)
    (c d : Config) (h : LevelStep Qf c d) :
    configQuality Qf c ≤ configQuality Qf d := by
  obtain ⟨qs, q, hchain, hmem, rfl⟩ := h
  exact le_aggregate_run Qf hinv c.2.1 c.2.2 qs q hchain hmem

/-- **Whole-algorithm monotonicity (consolidation).** Along any run of the
    multilevel recursion, quality never decreases: an induction on recursion depth
    (`ReflTransGen`) that chains the per-level `LevelStep.le_configQuality` by
    transitivity. -/
theorem configQuality_monotone_of_levelRun
    (Qf : ∀ m, WeightedGraph m → Partition m → ℝ) (hinv : IsAggregationInvariant Qf)
    (c d : Config) (h : Relation.ReflTransGen (LevelStep Qf) c d) :
    configQuality Qf c ≤ configQuality Qf d := by
  induction h with
  | refl => exact le_refl _
  | tail _ hstep ih => exact le_trans ih (LevelStep.le_configQuality Qf hinv _ _ hstep)

/-- **The algorithm's output never lowers quality, for any aggregation-invariant
    quality family.** For an initial graph `G` and partition `p`, if the recursion
    reaches a final (iterated-aggregate) graph `G'` with output partition `q`, then
    `Qf m G p ≤ Qf k G' q`. This is the top-level monotonicity guarantee, threaded
    across all aggregation levels; `modularity_le_of_algorithmRun` and
    `cpm_le_of_algorithmRun` (in `Meso.CPM`) are its instances. -/
theorem qualityFamily_le_of_algorithmRun {m k : ℕ}
    (Qf : ∀ j, WeightedGraph j → Partition j → ℝ) (hinv : IsAggregationInvariant Qf)
    (G : WeightedGraph m) (p : Partition m) (G' : WeightedGraph k) (q : Partition k)
    (h : Relation.ReflTransGen (LevelStep Qf) ⟨m, G, p⟩ ⟨k, G', q⟩) :
    Qf m G p ≤ Qf k G' q :=
  configQuality_monotone_of_levelRun Qf hinv _ _ h

/-- **The algorithm's output never lowers modularity.** For an initial graph `G`
    and partition `p`, if the recursion reaches a final (iterated-aggregate) graph
    `G'` with output partition `q`, then `modularity G γ p ≤ modularity G' γ q`. The
    modularity instance of `qualityFamily_le_of_algorithmRun`. -/
theorem modularity_le_of_algorithmRun {m k : ℕ} (γ : ℝ)
    (G : WeightedGraph m) (p : Partition m) (G' : WeightedGraph k) (q : Partition k)
    (h : Relation.ReflTransGen (LevelStep (fun _ H r => modularity H γ r))
          ⟨m, G, p⟩ ⟨k, G', q⟩) :
    modularity G γ p ≤ modularity G' γ q :=
  qualityFamily_le_of_algorithmRun (fun _ H r => modularity H γ r)
    (fun G p => modularity_aggregate_eq G γ p) G p G' q h

end Meso
