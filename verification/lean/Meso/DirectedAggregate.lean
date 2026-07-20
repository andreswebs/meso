/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.DirectedGraph
import Meso.Aggregate

/-!
# Directed aggregation: directed modularity as a sum over communities

The directed analogue of `Meso/Aggregate.lean`. Collapsing each community to a
single super-node preserves directed modularity, so a Louvain/Leiden recursion on
a directed graph may continue on the aggregate without losing quality.

The development reuses, verbatim, the partition-level machinery of the undirected
file: `numComm`, `commLabel`, `sum_commLabel`, and the kernel-generic Kronecker-δ
regrouping `sum_diagonal_eq_sum_fiber`. None of those carry a symmetry assumption,
so they apply unchanged to the asymmetric directed kernel.

Two things differ from the undirected case. First, the directed block sum is
ordered: `directedBlockWeight a b` (arcs from community `a` into community `b`) is
not `directedBlockWeight b a` in general, and the diagonal block `a a` counts each
internal arc exactly once (each directed arc is stored once), matching the
self-loop that Go's `aggregateDirected` (`aggregate.go`) folds, in contrast to the
undirected model whose diagonal block double-counts an internal edge. Second, the
aggregate out-degree and in-degree need two separate lemmas
(`directedAggregate_outDegree`, `directedAggregate_inDegree`); symmetry made one
suffice undirected.
-/

namespace Meso

variable {n : ℕ}

/-- **Directed modularity is a sum of within-community contributions.** Each
    community `c` contributes the block sum of `w_{ij} − γ k_i^out k_j^in / m` over
    ordered node pairs both assigned to `c`. Directed analogue of
    `modularity_eq_communitySum`. -/
theorem directedModularity_eq_communitySum (G : DirectedWeightedGraph n) (γ : ℝ)
    (p : Partition n) :
    directedModularity G γ p
      = (1 / G.totalWeight) * ∑ c ∈ Finset.univ.image p,
          ∑ i ∈ Finset.univ.filter (fun i => p i = c),
            ∑ j ∈ Finset.univ.filter (fun j => p j = c),
              (G.weight i j - γ * G.outDegree i * G.inDegree j / G.totalWeight) := by
  unfold directedModularity
  congr 1
  simp_rw [mul_ite, mul_one, mul_zero]
  rw [sum_diagonal_eq_sum_fiber p
    (fun i j => G.weight i j - γ * G.outDegree i * G.inDegree j / G.totalWeight)]

/-- Ordered block sum of arc weight from the community labelled `a` into the
    community labelled `b`. Unlike the undirected `blockWeight`, this is directional
    (`a → b`); the diagonal block `a a` sums each internal arc once, matching the
    single-storage self-loop of Go's `aggregateDirected`. -/
noncomputable def directedBlockWeight (G : DirectedWeightedGraph n) (p : Partition n)
    (a b : ℕ) : ℝ :=
  ∑ i ∈ Finset.univ.filter (fun i => p i = a),
    ∑ j ∈ Finset.univ.filter (fun j => p j = b), G.weight i j

/-- The directed aggregate graph: one node per community. Its arc weight `A → B` is
    the directed block sum between the two communities, its node size the summed
    member sizes. There is no symmetry obligation to discharge, so the directed
    aggregate is strictly easier to construct than the undirected one. Go
    counterpart: `aggregateDirected` (`aggregate.go`). -/
noncomputable def directedAggregate (G : DirectedWeightedGraph n) (p : Partition n) :
    DirectedWeightedGraph (numComm p) where
  weight A B := directedBlockWeight G p (commLabel p A) (commLabel p B)
  weight_nonneg A B := by
    unfold directedBlockWeight
    exact Finset.sum_nonneg fun i _ => Finset.sum_nonneg fun j _ => G.weight_nonneg i j
  nodeSize A := ∑ i ∈ Finset.univ.filter (fun i => p i = commLabel p A), G.nodeSize i
  nodeSize_nonneg A := Finset.sum_nonneg fun i _ => G.nodeSize_nonneg i

/-- The out-degree of an aggregate node is its community's total out-degree. Mirrors
    `aggregate_degree`. -/
lemma directedAggregate_outDegree (G : DirectedWeightedGraph n) (p : Partition n)
    (A : Fin (numComm p)) :
    (directedAggregate G p).outDegree A
      = ∑ i ∈ Finset.univ.filter (fun i => p i = commLabel p A), G.outDegree i := by
  unfold DirectedWeightedGraph.outDegree
  simp only [directedAggregate, directedBlockWeight]
  rw [Finset.sum_comm]
  refine Finset.sum_congr rfl fun i _ => ?_
  rw [sum_commLabel p (fun c => ∑ j ∈ Finset.univ.filter (fun j => p j = c), G.weight i j)]
  exact Finset.sum_fiberwise_of_maps_to
    (fun k _ => Finset.mem_image_of_mem p (Finset.mem_univ k)) (G.weight i)

/-- The in-degree of an aggregate node is its community's total in-degree. The
    transposed companion of `directedAggregate_outDegree`; it has no undirected
    counterpart, since symmetry made one degree lemma suffice. -/
lemma directedAggregate_inDegree (G : DirectedWeightedGraph n) (p : Partition n)
    (B : Fin (numComm p)) :
    (directedAggregate G p).inDegree B
      = ∑ j ∈ Finset.univ.filter (fun j => p j = commLabel p B), G.inDegree j := by
  unfold DirectedWeightedGraph.inDegree
  simp only [directedAggregate, directedBlockWeight]
  rw [sum_commLabel p (fun c => ∑ i ∈ Finset.univ.filter (fun i => p i = c),
      ∑ j ∈ Finset.univ.filter (fun j => p j = commLabel p B), G.weight i j)]
  rw [Finset.sum_fiberwise_of_maps_to
    (fun i _ => Finset.mem_image_of_mem p (Finset.mem_univ i))
    (fun i => ∑ j ∈ Finset.univ.filter (fun j => p j = commLabel p B), G.weight i j)]
  rw [Finset.sum_comm]

/-- Directed aggregation preserves total arc weight: `m' = m`. Mirrors
    `aggregate_twoM`. -/
lemma directedAggregate_totalWeight (G : DirectedWeightedGraph n) (p : Partition n) :
    (directedAggregate G p).totalWeight = G.totalWeight := by
  simp only [DirectedWeightedGraph.totalWeight]
  simp_rw [directedAggregate_outDegree]
  rw [sum_commLabel p (fun c => ∑ i ∈ Finset.univ.filter (fun i => p i = c), G.outDegree i)]
  exact Finset.sum_fiberwise_of_maps_to
    (fun i _ => Finset.mem_image_of_mem p (Finset.mem_univ i)) G.outDegree

/-- A community's directed block contribution splits into internal arc weight minus
    the resolution term built from its total out- and in-degrees. Directed analogue
    of `block_contribution` (the proof is the same `sum_mul_sum`/`mul_sum`/`sum_div`
    plumbing; nothing used symmetry). -/
lemma directed_block_contribution (G : DirectedWeightedGraph n) (γ : ℝ)
    (s : Finset (Fin n)) :
    ∑ i ∈ s, ∑ j ∈ s, (G.weight i j - γ * G.outDegree i * G.inDegree j / G.totalWeight)
      = (∑ i ∈ s, ∑ j ∈ s, G.weight i j)
        - γ * ((∑ i ∈ s, G.outDegree i) * (∑ j ∈ s, G.inDegree j)) / G.totalWeight := by
  simp_rw [Finset.sum_sub_distrib]
  congr 1
  rw [Finset.sum_mul_sum, Finset.mul_sum, Finset.sum_div]
  refine Finset.sum_congr rfl fun i _ => ?_
  rw [Finset.mul_sum, Finset.sum_div]
  refine Finset.sum_congr rfl fun j _ => ?_
  ring

/-- **Directed aggregation invariance.** Running the singleton partition (each
    aggregate node its own community) on the directed aggregate yields exactly the
    directed modularity of `p` on the original graph, so a directed
    Louvain/Leiden recursion may continue on the aggregate without losing quality.
    Directed analogue of `modularity_aggregate_eq`; Go counterpart
    `aggregateDirected`. -/
theorem directedModularity_aggregate_eq (G : DirectedWeightedGraph n) (γ : ℝ)
    (p : Partition n) :
    directedModularity (directedAggregate G p) γ (fun A => (A : ℕ))
      = directedModularity G γ p := by
  rw [directedModularity_eq_communitySum G γ p]
  simp only [directedModularity]
  rw [directedAggregate_totalWeight]
  congr 1
  rw [← sum_commLabel p fun c => ∑ i ∈ Finset.univ.filter (fun i => p i = c),
        ∑ j ∈ Finset.univ.filter (fun j => p j = c),
          (G.weight i j - γ * G.outDegree i * G.inDegree j / G.totalWeight)]
  refine Finset.sum_congr rfl fun A _ => ?_
  rw [directed_block_contribution,
    Finset.sum_eq_single A
      (fun b _ hb => by rw [if_neg fun h => hb (Fin.val_injective h).symm]; exact mul_zero _)
      (fun h => absurd (Finset.mem_univ A) h),
    if_pos rfl, mul_one, directedAggregate_outDegree, directedAggregate_inDegree]
  simp only [directedAggregate, directedBlockWeight]
  ring

end Meso
