/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.CPM

/-!
# Computable rational mirror of the quality functions

The value-oracle mirror. The model's `modularity` and `cpm` are
`noncomputable` and live over the reals, so they cannot be evaluated to a number.
This file adds a parallel development over the rationals that computes an exact
fraction: `WeightedGraphQ` (rational weights), `modularityQ`, `cpmQ`, and the
computable move-deltas the Go move-delta property test consumes.

Nothing here is `noncomputable` except `toReal`, the bridge back to the real
model. `toReal` exists only to state the F2 equivalence theorem
(`(modularityQ G γ p : ℝ) = modularity G.toReal γ p`), which is what makes this
runnable mirror provably the same as the proved model. That the quality
definitions below are plain `def`s (not `noncomputable def`s) is itself the
guarantee that they evaluate: `lake build` rejects a `def` the compiler cannot
produce code for.

The formulas are written to parallel `Meso.Quality.modularity` and `Meso.CPM.cpm`
term for term, over ℚ instead of ℝ, so the F2 cast-pushing proof stays mechanical.
-/

namespace Meso

/-- A finite, undirected, weighted graph on `n` nodes with rational weights: the
    computable mirror of `WeightedGraph`. Same fields, `ℚ` in place of `ℝ`. -/
structure WeightedGraphQ (n : ℕ) where
  /-- Edge weight between two nodes. -/
  weight : Fin n → Fin n → ℚ
  /-- The graph is undirected. -/
  weight_symm : ∀ i j, weight i j = weight j i
  /-- Weights are nonnegative. -/
  weight_nonneg : ∀ i j, 0 ≤ weight i j
  /-- Node size / weight (preserved through aggregation). -/
  nodeSize : Fin n → ℚ
  /-- Node sizes are nonnegative. -/
  nodeSize_nonneg : ∀ i, 0 ≤ nodeSize i

namespace WeightedGraphQ

variable {n : ℕ} (G : WeightedGraphQ n)

/-- Weighted degree of a node: `k_i = ∑_j w_{ij}`, over ℚ. -/
def degree (i : Fin n) : ℚ := ∑ j, G.weight i j

/-- `2m`: twice the total edge weight, `∑_i k_i`, over ℚ. -/
def twoM : ℚ := ∑ i, G.degree i

/-- The real-valued graph this rational graph mirrors: cast every weight and node
    size into ℝ. The bridge the F2 equivalence theorem is stated across.

    Noncomputable because the cast `ℚ → ℝ` is; this is the one definition here that
    is not meant to be evaluated. -/
noncomputable def toReal : WeightedGraph n where
  weight i j := (G.weight i j : ℝ)
  weight_symm i j := by exact_mod_cast G.weight_symm i j
  weight_nonneg i j := by exact_mod_cast G.weight_nonneg i j
  nodeSize i := (G.nodeSize i : ℝ)
  nodeSize_nonneg i := by exact_mod_cast G.nodeSize_nonneg i

@[simp] lemma toReal_weight (i j : Fin n) : G.toReal.weight i j = (G.weight i j : ℝ) := rfl

@[simp] lemma toReal_nodeSize (i : Fin n) : G.toReal.nodeSize i = (G.nodeSize i : ℝ) := rfl

@[simp] lemma toReal_degree (i : Fin n) : G.toReal.degree i = (G.degree i : ℝ) := by
  simp only [WeightedGraph.degree, WeightedGraphQ.degree, toReal_weight, Rat.cast_sum]

@[simp] lemma toReal_twoM : G.toReal.twoM = (G.twoM : ℝ) := by
  simp only [WeightedGraph.twoM, WeightedGraphQ.twoM, toReal_degree, Rat.cast_sum]

/-- Build a `WeightedGraphQ` from an arbitrary raw weight function and node-size
    function, normalising to the model's structural invariants. Each edge weight
    is `max (max 0 (raw i j)) (max 0 (raw j i))`: the `max 0` clamps to
    nonnegative, and taking the max of both orientations makes it symmetric while
    folding a once-listed edge into both directions (and leaving a self-loop `i = i`
    untouched, not doubled). On input that is already symmetric and nonnegative it
    is the identity.

    Its purpose is to let a runtime parser produce a graph without discharging any
    proof obligation itself: the three structural proofs are machine-checked here,
    once, so the evaluated value is a genuine `modularityQ G γ p` that F2 covers. -/
def ofRaw (raw : Fin n → Fin n → ℚ) (size : Fin n → ℚ) : WeightedGraphQ n where
  weight i j := max (max 0 (raw i j)) (max 0 (raw j i))
  weight_symm _ _ := max_comm _ _
  weight_nonneg i j := le_max_of_le_left (le_max_left 0 (raw i j))
  nodeSize i := max 0 (size i)
  nodeSize_nonneg i := le_max_left 0 (size i)

end WeightedGraphQ

variable {n : ℕ}

/-- Modularity with resolution `γ`, over ℚ: the computable mirror of `modularity`.

    `Q = (1 / 2m) · ∑_{i,j} (w_{ij} − γ · k_i · k_j / 2m) · δ(c_i, c_j)`, term for
    term the real definition with ℚ arithmetic. On an edgeless graph `2m = 0` and
    ℚ division by zero is `0`, matching the real definition's `1 / 0 = 0`. -/
def modularityQ (G : WeightedGraphQ n) (γ : ℚ) (p : Partition n) : ℚ :=
  (1 / G.twoM) * ∑ i, ∑ j,
    (G.weight i j - γ * G.degree i * G.degree j / G.twoM) *
      (if p i = p j then (1 : ℚ) else 0)

/-- The Constant Potts Model quality with resolution `γ`, over ℚ: the computable
    mirror of `cpm`. Flat node-size penalty `γ s_i s_j`, no `2m` normalisation.
    Sums over all ordered pairs, including the diagonal `i = j`. -/
def cpmQ (G : WeightedGraphQ n) (γ : ℚ) (p : Partition n) : ℚ :=
  ∑ i, ∑ j,
    (G.weight i j - γ * G.nodeSize i * G.nodeSize j) * (if p i = p j then (1 : ℚ) else 0)

/-- The canonical (leidenalg) CPM value: `cpmQ` restricted to distinct pairs
    `i ≠ j`, i.e. with the self-pair diagonal excluded. This is the value the
    reference implementations report; the F4 spec-blessing cross-check confirmed
    `cpmCanonicalQ` equals leidenalg's `quality()` on the corpus. It differs from
    `cpmQ` only by the diagonal (`cpmCanonicalQ_eq`), a partition-independent
    quantity, so it shares every optimum and guarantee. The Go public `Quality()`
    reports this, so meso's CPM numbers match the published literature. -/
def cpmCanonicalQ (G : WeightedGraphQ n) (γ : ℚ) (p : Partition n) : ℚ :=
  ∑ i, ∑ j, if i = j then 0 else
    (G.weight i j - γ * G.nodeSize i * G.nodeSize j) * (if p i = p j then (1 : ℚ) else 0)

/-- The exact modularity gain of moving node `v` to community `c`: the value the Go
    incremental move-delta is checked against. Computed the honest slow way, as a
    difference of from-scratch scores, so it is the oracle, not the formula under
    test. -/
def moveDeltaModularityQ (G : WeightedGraphQ n) (γ : ℚ) (p : Partition n)
    (v : Fin n) (c : ℕ) : ℚ :=
  modularityQ G γ (move p v c) - modularityQ G γ p

/-- The exact CPM gain of moving node `v` to community `c`; the CPM twin of
    `moveDeltaModularityQ`. -/
def moveDeltaCpmQ (G : WeightedGraphQ n) (γ : ℚ) (p : Partition n)
    (v : Fin n) (c : ℕ) : ℚ :=
  cpmQ G γ (move p v c) - cpmQ G γ p

/-! ## F2: the mirror agrees with the real model

The load-bearing equivalences. Each says the computable rational value, cast to
ℝ, is exactly the real model's value on the cast graph, so the runnable oracle is
provably the same artifact the proofs reason about. The proofs push `Rat.cast`
through the sums, products, division, and indicator; the `twoM = 0` edge case is
free, since `1 / 0 = 0` in both ℚ and ℝ. -/

theorem modularityQ_eq (G : WeightedGraphQ n) (γ : ℚ) (p : Partition n) :
    (modularityQ G γ p : ℝ) = modularity G.toReal (γ : ℝ) p := by
  unfold modularityQ modularity
  simp only [WeightedGraphQ.toReal_weight, WeightedGraphQ.toReal_degree,
    WeightedGraphQ.toReal_twoM]
  push_cast [apply_ite ((↑) : ℚ → ℝ)]
  rfl

theorem cpmQ_eq (G : WeightedGraphQ n) (γ : ℚ) (p : Partition n) :
    (cpmQ G γ p : ℝ) = cpm G.toReal (γ : ℝ) p := by
  unfold cpmQ cpm
  simp only [WeightedGraphQ.toReal_weight, WeightedGraphQ.toReal_nodeSize]
  push_cast [apply_ite ((↑) : ℚ → ℝ)]
  rfl

/-- **The canonical CPM is meso's CPM minus its self-pair diagonal**, the F4
    spec-blessing finding as a machine-checked identity:
    `cpmCanonicalQ = cpmQ − ∑_i (w_ii − γ s_i²)`. The subtracted term is exactly
    the diagonal `i = j` contribution `cpmQ` carries and the canonical convention
    drops; for a graph without self-loops it is `−γ ∑_i s_i²`, the constant offset
    the cross-check measured. Since it does not depend on the partition `p`, the
    two CPMs share every optimum and every guarantee. -/
theorem cpmCanonicalQ_eq (G : WeightedGraphQ n) (γ : ℚ) (p : Partition n) :
    cpmCanonicalQ G γ p
      = cpmQ G γ p - ∑ i, (G.weight i i - γ * G.nodeSize i ^ 2) := by
  unfold cpmCanonicalQ cpmQ
  rw [← Finset.sum_sub_distrib]
  refine Finset.sum_congr rfl (fun i _ => ?_)
  have h : ∀ j,
      (if i = j then (0 : ℚ) else
        (G.weight i j - γ * G.nodeSize i * G.nodeSize j) * (if p i = p j then 1 else 0))
      = (G.weight i j - γ * G.nodeSize i * G.nodeSize j) * (if p i = p j then 1 else 0)
        - (if i = j then
            (G.weight i j - γ * G.nodeSize i * G.nodeSize j) * (if p i = p j then 1 else 0)
          else 0) := by
    intro j; split <;> simp
  simp_rw [h]
  rw [Finset.sum_sub_distrib, Finset.sum_ite_eq]
  simp [pow_two, mul_assoc]

/-- The rational modularity move-delta casts to the real move-delta: the oracle
    value the Go incremental gain is checked against is the genuine ΔQ. -/
theorem moveDeltaModularityQ_eq (G : WeightedGraphQ n) (γ : ℚ) (p : Partition n)
    (v : Fin n) (c : ℕ) :
    (moveDeltaModularityQ G γ p v c : ℝ)
      = modularity G.toReal (γ : ℝ) (move p v c) - modularity G.toReal (γ : ℝ) p := by
  unfold moveDeltaModularityQ
  rw [Rat.cast_sub, modularityQ_eq, modularityQ_eq]

/-- The CPM twin of `moveDeltaModularityQ_eq`. -/
theorem moveDeltaCpmQ_eq (G : WeightedGraphQ n) (γ : ℚ) (p : Partition n)
    (v : Fin n) (c : ℕ) :
    (moveDeltaCpmQ G γ p v c : ℝ)
      = cpm G.toReal (γ : ℝ) (move p v c) - cpm G.toReal (γ : ℝ) p := by
  unfold moveDeltaCpmQ
  rw [Rat.cast_sub, cpmQ_eq, cpmQ_eq]

end Meso
