/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.Quality

/-!
# Directed weighted graph model

The finite directed weighted graph `meso`'s directed support operates on. It is
`WeightedGraph` (`Meso/Graph.lean`) without the symmetry field: a dense weight
function `weight i j` is the weight of the arc `i → j`, and direction is carried
by the asymmetry of that function alone. No separate in-adjacency is needed at
the mathematical level (the split out/in adjacency lists of the Go CSR are an
implementation detail, not part of the mathematics).

This model deliberately shares no code with `WeightedGraph`: whether to later
unify the two behind a common interface is an explicit open question of the
research plan (`docs/research/directed-modularity-formal-verification.md`),
resolved separately. Keeping it standalone keeps the directed statements direct.

Only the directed *objective* (`directedModularity`, the Leicht-Newman directed
modularity) is modelled here. No guarantees (connectivity, γ-separation,
subset-optimality) are proved over it yet: those are Phases 3 and 4 of the
research plan, gated on triage of whether they survive asymmetry at all.
-/

namespace Meso

/-- A finite, directed, weighted graph on `n` nodes: `WeightedGraph` without the
    symmetry field. `weight i j` is the weight of the arc `i → j`, so asymmetry
    (`weight i j ≠ weight j i`) is exactly what makes the graph directed. -/
structure DirectedWeightedGraph (n : ℕ) where
  /-- Weight of the arc `i → j`. -/
  weight : Fin n → Fin n → ℝ
  /-- Arc weights are nonnegative. -/
  weight_nonneg : ∀ i j, 0 ≤ weight i j
  /-- Node size / weight (preserved through aggregation). -/
  nodeSize : Fin n → ℝ
  /-- Node sizes are nonnegative. -/
  nodeSize_nonneg : ∀ i, 0 ≤ nodeSize i

namespace DirectedWeightedGraph

variable {n : ℕ} (G : DirectedWeightedGraph n)

/-- Weighted out-degree of a node: total weight of its outgoing arcs,
    `k_i^out = ∑_j w_{ij}` (row sum). A self-loop `weight i i` contributes once. -/
def outDegree (i : Fin n) : ℝ := ∑ j, G.weight i j

/-- Weighted in-degree of a node: total weight of its incoming arcs,
    `k_j^in = ∑_i w_{ij}` (column sum). A self-loop `weight j j` contributes once. -/
def inDegree (j : Fin n) : ℝ := ∑ i, G.weight i j

/-- `m`: the total arc weight of the graph, `∑_i k_i^out = ∑_{i,j} w_{ij}`. Each
    arc (self-loops included) counts once. Equals what the Go `csr.twoM()` returns
    for a directed graph. -/
def totalWeight : ℝ := ∑ i, G.outDegree i

lemma outDegree_nonneg (i : Fin n) : 0 ≤ G.outDegree i :=
  Finset.sum_nonneg fun j _ => G.weight_nonneg i j

lemma inDegree_nonneg (j : Fin n) : 0 ≤ G.inDegree j :=
  Finset.sum_nonneg fun i _ => G.weight_nonneg i j

lemma totalWeight_nonneg : 0 ≤ G.totalWeight :=
  Finset.sum_nonneg fun i _ => G.outDegree_nonneg i

/-- Total arc weight computed by columns equals total arc weight computed by rows:
    `∑_i k_i^out = ∑_j k_j^in`, both being `∑_{i,j} w_{ij}`. Pins the out/in
    bookkeeping for the Phase 2 identity proofs. -/
lemma totalWeight_eq_sum_inDegree : G.totalWeight = ∑ j, G.inDegree j := by
  unfold totalWeight outDegree inDegree
  rw [Finset.sum_comm]

/-- `totalWeight` as the flat double sum `∑_{i,j} w_{ij}`. -/
lemma totalWeight_eq_sum_sum : G.totalWeight = ∑ i, ∑ j, G.weight i j := rfl

end DirectedWeightedGraph

variable {n : ℕ}

/-- Leicht-Newman directed modularity with resolution `γ`:
    `Q = (1 / m) · ∑_{i,j} (w_{ij} − γ · k_i^out · k_j^in / m) · δ(c_i, c_j)`.

    The null model uses each pair's separate out- and in-degrees
    (`k_i^out · k_j^in / m`) rather than the symmetric undirected `k_i · k_j / 2m`,
    so an arc `i → j` is credited against `i`'s out-strength and `j`'s in-strength.
    Matches the Go `DirectedModularity` (`directed_quality.go`) term for term.

    Noncomputable because it is defined over the reals. On an arcless graph
    `m = 0` and Lean's `1 / 0 = 0`, so `Q = 0` with no side condition, matching the
    Go early-return `if total == 0 { return 0 }`. -/
noncomputable def directedModularity (G : DirectedWeightedGraph n)
    (γ : ℝ) (p : Partition n) : ℝ :=
  (1 / G.totalWeight) * ∑ i, ∑ j,
    (G.weight i j - γ * G.outDegree i * G.inDegree j / G.totalWeight) *
      (if p i = p j then (1 : ℝ) else 0)

/-- On a graph with no arc weight, directed modularity is `0` for every partition
    and resolution: the leading `1 / m` factor is `1 / 0 = 0`. -/
lemma directedModularity_of_totalWeight_eq_zero
    (G : DirectedWeightedGraph n) (γ : ℝ) (p : Partition n)
    (h : G.totalWeight = 0) : directedModularity G γ p = 0 := by
  unfold directedModularity
  rw [h]
  simp

/-- **The all-in-one partition scores `1 − γ`** (on a graph with arcs): every pair
    is within-community, so the edge term sums to `m` and the null term to `γ · m`
    (`∑_i k_i^out = ∑_j k_j^in = m`). At `γ = 1` this is `0`, matching the Go
    fixture case and the undirected convention that one community carries no
    structure signal. -/
theorem directedModularity_const (G : DirectedWeightedGraph n) (γ : ℝ)
    (h : G.totalWeight ≠ 0) (c : ℕ) :
    directedModularity G γ (fun _ => c) = 1 - γ := by
  have hblock :
      (∑ i, ∑ j, (G.weight i j - γ * G.outDegree i * G.inDegree j / G.totalWeight))
        = (∑ i, ∑ j, G.weight i j)
          - γ * ((∑ i, G.outDegree i) * (∑ j, G.inDegree j)) / G.totalWeight := by
    simp_rw [Finset.sum_sub_distrib]
    congr 1
    rw [Finset.sum_mul_sum, Finset.mul_sum, Finset.sum_div]
    refine Finset.sum_congr rfl fun i _ => ?_
    rw [Finset.mul_sum, Finset.sum_div]
    refine Finset.sum_congr rfl fun j _ => ?_
    ring
  unfold directedModularity
  simp only [if_true, mul_one]
  rw [hblock, ← G.totalWeight_eq_sum_sum, show (∑ i, G.outDegree i) = G.totalWeight from rfl,
    ← G.totalWeight_eq_sum_inDegree]
  field_simp

/-- At γ = 1 the all-in-one partition scores exactly `0`. -/
theorem directedModularity_const_one (G : DirectedWeightedGraph n)
    (h : G.totalWeight ≠ 0) (c : ℕ) :
    directedModularity G 1 (fun _ => c) = 0 := by
  rw [directedModularity_const G 1 h c]; ring

/-- View an undirected graph as a (symmetric) directed graph: same weight function,
    symmetry forgotten. The bridge the symmetric-reduction theorem is stated across;
    a constructor, not a model unification. -/
def WeightedGraph.toDirected (G : WeightedGraph n) : DirectedWeightedGraph n where
  weight := G.weight
  weight_nonneg := G.weight_nonneg
  nodeSize := G.nodeSize
  nodeSize_nonneg := G.nodeSize_nonneg

@[simp] lemma WeightedGraph.toDirected_weight (G : WeightedGraph n) (i j : Fin n) :
    G.toDirected.weight i j = G.weight i j := rfl

@[simp] lemma WeightedGraph.toDirected_nodeSize (G : WeightedGraph n) (i : Fin n) :
    G.toDirected.nodeSize i = G.nodeSize i := rfl

@[simp] lemma WeightedGraph.toDirected_outDegree (G : WeightedGraph n) (i : Fin n) :
    G.toDirected.outDegree i = G.degree i := rfl

@[simp] lemma WeightedGraph.toDirected_inDegree (G : WeightedGraph n) (j : Fin n) :
    G.toDirected.inDegree j = G.degree j := by
  change ∑ i, G.weight i j = ∑ i, G.weight j i
  exact Finset.sum_congr rfl fun i _ => G.weight_symm i j

@[simp] lemma WeightedGraph.toDirected_totalWeight (G : WeightedGraph n) :
    G.toDirected.totalWeight = G.twoM :=
  Finset.sum_congr rfl fun i _ => G.toDirected_outDegree i

/-- **Symmetric reduction.** On a symmetric graph, Leicht-Newman directed modularity
    coincides with undirected modularity: out- and in-degree both collapse to the
    degree and `m = 2m`. The directed model is a conservative extension of the proved
    undirected one; the Go counterpart is
    `TestDirectedModularity_SymmetricReducesToUndirected`. -/
theorem directedModularity_toDirected_eq (G : WeightedGraph n) (γ : ℝ)
    (p : Partition n) :
    directedModularity G.toDirected γ p = modularity G γ p := by
  unfold directedModularity modularity
  simp only [WeightedGraph.toDirected_weight, WeightedGraph.toDirected_outDegree,
    WeightedGraph.toDirected_inDegree, WeightedGraph.toDirected_totalWeight]

end Meso
