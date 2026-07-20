/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.Move
import Meso.Aggregate
import Meso.Level

/-!
# Termination

The algorithm halts (plan section 4.1). Its iteration is two nested loops, and
each has its own well-founded measure, exactly as the plan describes:

* **The fast local-move phase halts** because every *accepted* move strictly
  raises modularity, and modularity takes only finitely many values. The fiddly
  point is the finiteness: `Partition n = Fin n → ℕ` is an infinite type, so the
  measure is not the partition but its *quality*. Modularity depends on a partition
  only through the equivalence "same community" (`p i = p j`), which ranges over a
  finite type; hence `modularity G γ` has finite range, and a strictly-increasing
  sequence of accepted moves would be an injection from `ℕ` into a finite set.

* **The multilevel recursion halts** because the number of communities `numComm`
  is bounded (`numComm p ≤ n`, the partition lattice is finite) and strictly
  decreases on each productive level; a strictly-decreasing `ℕ`-measure cannot run
  forever.

That a level *is* productive is no longer assumed: `levelStep_size_lt_or_injective`
derives the dichotomy — a level either strictly lowers the node count or its
partition is discrete (injective, every node its own community), the fixed point at
which aggregation does nothing and the algorithm stops. `levelRun_reaches_fixedPoint`
then reads whole-algorithm halting off it: an infinite level run must reach such a
fixed point, since the node count cannot strictly descend forever. This unifies the
size-bundled `Config` / `LevelStep` of `Meso.Level` with the `numComm` descent here.

Modelling the convergence guard explicitly closes the last residual: `RunningLevelStep`
is a level the loop takes *only while* its partition is non-discrete, and
`no_infinite_runningLevel_run` proves the guarded loop cannot run forever — outright
termination, no "reaches a fixed point" gloss. This is what a real implementation's
stop-when-unchanged check buys; the Go correspondence is in CORRESPONDENCE.md.

The whole algorithm's termination is the lexicographic combination of the two:
within a level the quality measure strictly climbs to a local-move-stable point,
and across levels the community count strictly drops. This file proves each half.
-/

namespace Meso

variable {n : ℕ}

/-- Modularity written against an explicit "same community" kernel `k : Fin n →
    Fin n → Bool` in place of the partition's own `if p i = p j`. Its domain is a
    finite type, which is what makes the range of modularity finite. -/
noncomputable def modularityKernel (G : WeightedGraph n) (γ : ℝ)
    (k : Fin n → Fin n → Bool) : ℝ :=
  (1 / G.twoM) * ∑ i, ∑ j,
    (G.weight i j - γ * G.degree i * G.degree j / G.twoM) *
      (if k i j then (1 : ℝ) else 0)

/-- Modularity factors through the finite "same community" kernel: it is
    `modularityKernel` applied to `decide (p i = p j)`. -/
lemma modularity_eq_kernel (G : WeightedGraph n) (γ : ℝ) (p : Partition n) :
    modularity G γ p = modularityKernel G γ (fun i j => decide (p i = p j)) := by
  unfold modularity modularityKernel
  simp only [decide_eq_true_eq]

/-- **Modularity has finite range.** It factors through the finite kernel type
    `Fin n → Fin n → Bool`, so its image is contained in that of `modularityKernel`,
    which is finite because its domain is. This is the measure's well-foundedness:
    quality can strictly increase only finitely often. -/
lemma finite_range_modularity (G : WeightedGraph n) (γ : ℝ) :
    (Set.range (modularity G γ)).Finite := by
  apply Set.Finite.subset (Set.finite_range (modularityKernel G γ))
  rintro _ ⟨p, rfl⟩
  exact ⟨fun i j => decide (p i = p j), (modularity_eq_kernel G γ p).symm⟩

/-- **The improving dynamics halt.** There is no infinite sequence of partitions
    whose modularity strictly increases at every step: such a sequence would make
    `modularity G γ` strictly monotone along `ℕ`, hence injective, with range in the
    finite set `Set.range (modularity G γ)` — impossible. This is the termination of
    the fast local-move phase (each accepted move strictly improves quality). -/
theorem no_infinite_improving_run (G : WeightedGraph n) (γ : ℝ) (f : ℕ → Partition n)
    (h : ∀ k, modularity G γ (f k) < modularity G γ (f (k + 1))) : False := by
  have hmono : StrictMono (fun k => modularity G γ (f k)) := strictMono_nat_of_lt_succ h
  have hfin : (Set.range (fun k => modularity G γ (f k))).Finite :=
    (finite_range_modularity G γ).subset (by rintro _ ⟨k, rfl⟩; exact ⟨f k, rfl⟩)
  exact absurd hfin (Set.infinite_range_of_injective hmono.injective)

/-- An *accepted* local move: a local-move step that strictly raises modularity.
    The fast phase only ever applies these (a non-improving candidate is rejected in
    favour of staying put, which is a no-op). -/
def IsAcceptedMove (G : WeightedGraph n) (γ : ℝ) (p q : Partition n) : Prop :=
  IsLocalMove (modularity G γ) p q ∧ modularity G γ p < modularity G γ q

/-- **The local-move phase halts.** No infinite run of accepted moves exists: each
    strictly improves modularity, so `no_infinite_improving_run` applies. -/
theorem no_infinite_acceptedMove_run (G : WeightedGraph n) (γ : ℝ) (f : ℕ → Partition n)
    (h : ∀ k, IsAcceptedMove G γ (f k) (f (k + 1))) : False :=
  no_infinite_improving_run G γ f fun k => (h k).2

/-- **The community count is bounded by the node count** (`numComm p ≤ n`): the
    partition lattice is finite, so the level measure is bounded below-and-above and
    can strictly decrease only finitely often. -/
lemma numComm_le (p : Partition n) : numComm p ≤ n := by
  calc numComm p ≤ (Finset.univ : Finset (Fin n)).card := Finset.card_image_le
    _ = n := by rw [Finset.card_univ, Fintype.card_fin]

/-- **The multilevel recursion halts.** A `ℕ`-valued level measure that strictly
    decreases at every level cannot run forever. Instantiated with `numComm` (which
    `numComm_le` bounds and which strictly drops on each productive aggregation
    level), this is the termination of the outer loop. -/
theorem no_infinite_descending_levels (m : ℕ → ℕ) (h : ∀ k, m (k + 1) < m k) : False := by
  have key : ∀ k, m k + k ≤ m 0 := by
    intro k
    induction k with
    | zero => omega
    | succ j ih => have := h j; omega
  have := key (m 0 + 1)
  omega

/-- **The community count characterises the discrete partition.** `numComm p = n`
    exactly when `p` is injective — every node in its own community, the discrete
    (all-singleton) partition. The boundary case of `numComm_le`: the image of `p`
    fills all `n` labels iff `p` collapses nothing. -/
lemma numComm_eq_iff_injective (p : Partition n) :
    numComm p = n ↔ Function.Injective p := by
  rw [← Set.injOn_univ, ← Finset.coe_univ, ← Finset.card_image_iff, Finset.card_univ,
    Fintype.card_fin]

/-- **The level measure descends or the partition is discrete (derived, not assumed).**
    One level, from a configuration `⟨m, G, p⟩`, aggregates along `p` to a graph on
    `numComm p` nodes. Either the node count strictly drops (`numComm p < m`, the level
    merged at least two nodes) or `p` is injective — the discrete partition on which
    aggregation does nothing, the fixed point where the algorithm stops. This replaces
    the former assumption that a productive level lowers `numComm`: the dichotomy is now
    read off `numComm_le` and `numComm_eq_iff_injective`. -/
theorem levelStep_size_lt_or_injective
    (Qf : ∀ m, WeightedGraph m → Partition m → ℝ) (c d : Config) (h : LevelStep Qf c d) :
    d.1 < c.1 ∨ Function.Injective c.2.2 := by
  obtain ⟨qs, q, hchain, hmem, rfl⟩ := h
  rcases lt_or_eq_of_le (numComm_le c.2.2) with hlt | heq
  · exact Or.inl hlt
  · exact Or.inr ((numComm_eq_iff_injective c.2.2).mp heq)

/-- **No infinite run of productive levels.** If every level strictly shrinks the node
    count, the run is finite: `fun k => (f k).1` is a strictly-decreasing ℕ measure,
    which `no_infinite_descending_levels` forbids. The node count is the level measure,
    now used concretely rather than as an abstract `m : ℕ → ℕ`. -/
theorem no_infinite_productiveLevel_run (f : ℕ → Config)
    (h : ∀ k, (f (k + 1)).1 < (f k).1) : False :=
  no_infinite_descending_levels (fun k => (f k).1) h

/-- **The multilevel recursion reaches a fixed point.** In any infinite sequence of
    levels, some configuration's partition is injective — the discrete partition, the
    fixed point at which aggregation does nothing and the algorithm halts. The node
    count cannot strictly drop forever, so by `levelStep_size_lt_or_injective` an
    unproductive (discrete-partition) level must occur. This derives whole-algorithm
    halting from the level dichotomy, closing the former "a productive level lowers
    `numComm`" assumption. -/
theorem levelRun_reaches_fixedPoint
    (Qf : ∀ m, WeightedGraph m → Partition m → ℝ) (f : ℕ → Config)
    (hstep : ∀ k, LevelStep Qf (f k) (f (k + 1))) :
    ∃ k, Function.Injective (f k).2.2 := by
  by_contra hcon
  rw [not_exists] at hcon
  refine no_infinite_productiveLevel_run f fun k => ?_
  rcases levelStep_size_lt_or_injective Qf (f k) (f (k + 1)) (hstep k) with hlt | hinj
  · exact hlt
  · exact absurd hinj (hcon k)

/-- A **running level**: a `LevelStep` the algorithm's convergence check still permits.
    The guard is that the source partition is not yet discrete (`¬ Injective c.2.2`) — the
    previous level actually merged something, so the loop's "continue while the partition
    changed" test fires. This is the control-flow guard the bare `LevelStep` relation
    omits. `LevelStep` alone permits idling at the fixed point (aggregating a discrete
    partition returns a same-size graph); the guard forbids it, so folding it in removes
    the last interpretive residual from termination. A discrete partition is exactly the
    fixed point at which no further merge helps; matching this guard in Go is spelled out
    in CORRESPONDENCE.md. -/
def RunningLevelStep (Qf : ∀ m, WeightedGraph m → Partition m → ℝ) (c d : Config) : Prop :=
  LevelStep Qf c d ∧ ¬ Function.Injective c.2.2

/-- A running level strictly lowers the node count: the guard `¬ Injective c.2.2` rules
    out the discrete-partition branch of `levelStep_size_lt_or_injective`, leaving the
    strict descent. -/
theorem RunningLevelStep.size_lt
    {Qf : ∀ m, WeightedGraph m → Partition m → ℝ} {c d : Config}
    (h : RunningLevelStep Qf c d) : d.1 < c.1 := by
  rcases levelStep_size_lt_or_injective Qf c d h.1 with hlt | hinj
  · exact hlt
  · exact absurd hinj h.2

/-- **The guarded multilevel loop halts outright.** No infinite sequence of running
    levels exists: each strictly lowers the node count (`RunningLevelStep.size_lt`), and a
    strictly-decreasing ℕ measure cannot descend forever. Unlike
    `levelRun_reaches_fixedPoint`, this needs *no* "reaches a fixed point" interpretation —
    the convergence guard is part of the relation, so the loop cannot run forever, full
    stop. It is the level-loop analogue of `no_infinite_acceptedMove_run` for the fast
    phase: an "accepted" fast-phase step strictly improves quality, a "running" level
    strictly shrinks the graph, and neither can repeat forever. -/
theorem no_infinite_runningLevel_run
    (Qf : ∀ m, WeightedGraph m → Partition m → ℝ) (f : ℕ → Config)
    (h : ∀ k, RunningLevelStep Qf (f k) (f (k + 1))) : False :=
  no_infinite_productiveLevel_run f fun k => (h k).size_lt

end Meso
