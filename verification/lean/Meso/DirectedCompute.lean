/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.DirectedGraph
import Meso.Move

/-!
# Computable rational mirror of the directed model

The value-oracle mirror of the directed objective, for the future directed
value-oracle (Phase 5 of the research plan,
`docs/research/directed-modularity-formal-verification.md`). The model's
`directedModularity` is `noncomputable` over the reals; this file adds a parallel
development over the rationals that computes an exact fraction:
`DirectedWeightedGraphQ` (rational weights), `directedModularityQ`, and the
computable move-delta a future directed golden vector will carry.

Modelled on `Meso/Compute.lean`. Nothing here is `noncomputable` except `toReal`,
the bridge back to the real model; that the quality definitions are plain `def`s
is itself the proof they evaluate (`lake build` rejects a `def` the compiler
cannot produce code for). `directedModularityQ` is written to parallel
`directedModularity` term for term, over ℚ, so the F2 cast-pushing proof stays
mechanical.
-/

namespace Meso

/-- A finite, directed, weighted graph on `n` nodes with rational weights: the
    computable mirror of `DirectedWeightedGraph`. Same fields, `ℚ` in place of
    `ℝ`, and, like it, no symmetry field. -/
structure DirectedWeightedGraphQ (n : ℕ) where
  /-- Weight of the arc `i → j`. -/
  weight : Fin n → Fin n → ℚ
  /-- Arc weights are nonnegative. -/
  weight_nonneg : ∀ i j, 0 ≤ weight i j
  /-- Node size / weight (preserved through aggregation). -/
  nodeSize : Fin n → ℚ
  /-- Node sizes are nonnegative. -/
  nodeSize_nonneg : ∀ i, 0 ≤ nodeSize i

namespace DirectedWeightedGraphQ

variable {n : ℕ} (G : DirectedWeightedGraphQ n)

/-- Weighted out-degree `k_i^out = ∑_j w_{ij}` (row sum), over ℚ. -/
def outDegree (i : Fin n) : ℚ := ∑ j, G.weight i j

/-- Weighted in-degree `k_j^in = ∑_i w_{ij}` (column sum), over ℚ. -/
def inDegree (j : Fin n) : ℚ := ∑ i, G.weight i j

/-- Total arc weight `m = ∑_i k_i^out = ∑_{i,j} w_{ij}`, over ℚ. -/
def totalWeight : ℚ := ∑ i, G.outDegree i

/-- The real-valued directed graph this rational graph mirrors: cast every weight
    and node size into ℝ. The bridge the F2 equivalence theorem is stated across.

    Noncomputable because the cast `ℚ → ℝ` is; the one definition here not meant to
    be evaluated. -/
noncomputable def toReal : DirectedWeightedGraph n where
  weight i j := (G.weight i j : ℝ)
  weight_nonneg i j := by exact_mod_cast G.weight_nonneg i j
  nodeSize i := (G.nodeSize i : ℝ)
  nodeSize_nonneg i := by exact_mod_cast G.nodeSize_nonneg i

@[simp] lemma toReal_weight (i j : Fin n) : G.toReal.weight i j = (G.weight i j : ℝ) := rfl

@[simp] lemma toReal_nodeSize (i : Fin n) : G.toReal.nodeSize i = (G.nodeSize i : ℝ) := rfl

@[simp] lemma toReal_outDegree (i : Fin n) : G.toReal.outDegree i = (G.outDegree i : ℝ) := by
  simp only [DirectedWeightedGraph.outDegree, DirectedWeightedGraphQ.outDegree, toReal_weight,
    Rat.cast_sum]

@[simp] lemma toReal_inDegree (j : Fin n) : G.toReal.inDegree j = (G.inDegree j : ℝ) := by
  simp only [DirectedWeightedGraph.inDegree, DirectedWeightedGraphQ.inDegree, toReal_weight,
    Rat.cast_sum]

@[simp] lemma toReal_totalWeight : G.toReal.totalWeight = (G.totalWeight : ℝ) := by
  simp only [DirectedWeightedGraph.totalWeight, DirectedWeightedGraphQ.totalWeight,
    toReal_outDegree, Rat.cast_sum]

/-- Build a `DirectedWeightedGraphQ` from an arbitrary raw weight function and
    node-size function, normalising only to nonnegativity: `weight i j = max 0
    (raw i j)`.

    Unlike the undirected `WeightedGraphQ.ofRaw`, it does **not** symmetrise: it
    never folds the two orientations together, so an asymmetric `raw` produces an
    asymmetric graph (direction is preserved). On input that is already
    nonnegative it is the identity. -/
def ofRaw (raw : Fin n → Fin n → ℚ) (size : Fin n → ℚ) : DirectedWeightedGraphQ n where
  weight i j := max 0 (raw i j)
  weight_nonneg i j := le_max_left 0 (raw i j)
  nodeSize i := max 0 (size i)
  nodeSize_nonneg i := le_max_left 0 (size i)

end DirectedWeightedGraphQ

variable {n : ℕ}

/-- Directed modularity with resolution `γ`, over ℚ: the computable mirror of
    `directedModularity`.

    `Q = (1 / m) · ∑_{i,j} (w_{ij} − γ · k_i^out · k_j^in / m) · δ(c_i, c_j)`, term
    for term the real definition with ℚ arithmetic. On an arcless graph `m = 0` and
    ℚ division by zero is `0`, matching the real definition's `1 / 0 = 0`. -/
def directedModularityQ (G : DirectedWeightedGraphQ n) (γ : ℚ) (p : Partition n) : ℚ :=
  (1 / G.totalWeight) * ∑ i, ∑ j,
    (G.weight i j - γ * G.outDegree i * G.inDegree j / G.totalWeight) *
      (if p i = p j then (1 : ℚ) else 0)

/-- The exact directed modularity gain of moving node `v` to community `c`,
    computed the honest slow way as a difference of from-scratch scores: the
    oracle value a future directed golden vector will carry, not the incremental
    formula under test. That the incremental formula equals this is Phase 2 of the
    research plan. -/
def moveDeltaDirectedModularityQ (G : DirectedWeightedGraphQ n) (γ : ℚ) (p : Partition n)
    (v : Fin n) (c : ℕ) : ℚ :=
  directedModularityQ G γ (move p v c) - directedModularityQ G γ p

/-! ## F2: the mirror agrees with the real model

The load-bearing equivalences. Each says the computable rational value, cast to
ℝ, is exactly the real model's value on the cast graph, so the runnable directed
oracle is provably the same artifact future proofs reason about. The proof pushes
`Rat.cast` through the sums, products, division, and indicator; the `m = 0` edge
case is free, since `1 / 0 = 0` in both ℚ and ℝ. -/

theorem directedModularityQ_eq (G : DirectedWeightedGraphQ n) (γ : ℚ) (p : Partition n) :
    (directedModularityQ G γ p : ℝ) = directedModularity G.toReal (γ : ℝ) p := by
  unfold directedModularityQ directedModularity
  simp only [DirectedWeightedGraphQ.toReal_weight, DirectedWeightedGraphQ.toReal_outDegree,
    DirectedWeightedGraphQ.toReal_inDegree, DirectedWeightedGraphQ.toReal_totalWeight]
  push_cast [apply_ite ((↑) : ℚ → ℝ)]
  rfl

/-- The rational directed move-delta casts to the real directed move-delta: the
    oracle value a future directed golden vector carries is the genuine ΔQ. -/
theorem moveDeltaDirectedModularityQ_eq (G : DirectedWeightedGraphQ n) (γ : ℚ) (p : Partition n)
    (v : Fin n) (c : ℕ) :
    (moveDeltaDirectedModularityQ G γ p v c : ℝ)
      = directedModularity G.toReal (γ : ℝ) (move p v c)
        - directedModularity G.toReal (γ : ℝ) p := by
  unfold moveDeltaDirectedModularityQ
  rw [Rat.cast_sub, directedModularityQ_eq, directedModularityQ_eq]

/-! ## Fixture sanity checks

The Go tests pin hand-computed values on a 3-node asymmetric fixture
(`directed_quality_test.go`, `asymmetricDirectedCSR`). Reproducing them here pins
the Lean definitions to the same semantics as the Go code, until the Phase 5
oracle exists.

The fixture: arcs `0 → 1` weight 2, `0 → 2` weight 1, `1 → 0` weight 1; no
self-loops; unit node sizes. Out-degrees `[3, 1, 0]`, in-degrees `[1, 2, 1]`,
`totalWeight = 4`. -/

/-- The Go `asymmetricDirectedCSR` fixture as a `DirectedWeightedGraphQ`: raw
    weight matrix `!![0, 2, 1; 1, 0, 0; 0, 0, 0]`, unit node sizes. Built via
    `ofRaw`, which on this already-nonnegative input is the identity. -/
def asymmetricDirectedFixture : DirectedWeightedGraphQ 3 :=
  DirectedWeightedGraphQ.ofRaw ![![0, 2, 1], ![1, 0, 0], ![0, 0, 0]] ![1, 1, 1]

/-- Shared unfolding for the fixture checks: expand the objective, the fixture, and
    the derived quantities, then discharge the concrete ℚ arithmetic with `norm_num`.
    `decide` cannot close these: ℚ equality forces a gcd normalisation the kernel
    does not reduce. -/
syntax "check_fixture" : tactic
macro_rules
  | `(tactic| check_fixture) =>
    `(tactic|
      simp only [directedModularityQ, moveDeltaDirectedModularityQ, asymmetricDirectedFixture,
          DirectedWeightedGraphQ.ofRaw, DirectedWeightedGraphQ.totalWeight,
          DirectedWeightedGraphQ.outDegree, DirectedWeightedGraphQ.inDegree, move,
          Function.update, Fin.sum_univ_three, Matrix.cons_val_zero, Matrix.cons_val_one,
          Matrix.head_cons, Matrix.cons_val_two, Matrix.tail_cons, Matrix.cons_val_fin_one] <;>
        norm_num <;> simp <;> norm_num)

example : directedModularityQ asymmetricDirectedFixture 1 ![0, 0, 0] = 0 := by check_fixture

example : directedModularityQ asymmetricDirectedFixture 1 ![0, 1, 2] = -5 / 16 := by check_fixture

example : directedModularityQ asymmetricDirectedFixture 2 ![0, 1, 2] = -5 / 8 := by check_fixture

example : directedModularityQ asymmetricDirectedFixture 1 ![0, 0, 1] = 0 := by check_fixture

example : moveDeltaDirectedModularityQ asymmetricDirectedFixture 1 ![0, 1, 2] 1 0 = 5 / 16 := by
  check_fixture

end Meso
