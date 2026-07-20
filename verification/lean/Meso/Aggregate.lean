/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.Quality

/-!
# Aggregation: modularity as a sum over communities

The second half of stepwise monotonicity is aggregation (the plan's
"correctness-critical, fails-silently" phase, section 4.1): collapse each
community to a single aggregate node, folding a community's internal edge weight
into a self-loop and summing node sizes, then recurse.

The mathematical heart of *why aggregation preserves modularity* is that
modularity depends on a partition only through community-level aggregates. The
Kronecker-δ in the modularity sum keeps exactly the within-community node pairs,
so the double sum over nodes regroups into a sum over communities of
within-community sums, which is precisely the internal weight the aggregate
graph's self-loops carry.

This file proves that regrouping (`sum_diagonal_eq_sum_fiber`) and specialises it
to modularity (`modularity_eq_communitySum`). Constructing the re-indexed
aggregate `WeightedGraph` and stating invariance as a graph-to-graph equality
(`modularity G γ p = modularity (aggregate G p) γ singleton`) is the next step;
this decomposition is its content, stripped of the re-indexing bookkeeping.
-/

namespace Meso

variable {n : ℕ}

/-- The Kronecker-δ regrouping. Summing `F i j` over all node pairs in the same
    community (the diagonal picked out by `if p i = p j`) equals summing, over
    each community `c`, the block sum of `F` over pairs both assigned to `c`.

    This is the aggregation identity at the level of an arbitrary kernel `F`;
    with `F` the modularity summand it becomes `modularity_eq_communitySum`. -/
theorem sum_diagonal_eq_sum_fiber (p : Fin n → ℕ) (F : Fin n → Fin n → ℝ) :
    ∑ i, ∑ j, (if p i = p j then F i j else 0)
      = ∑ c ∈ Finset.univ.image p,
          ∑ i ∈ Finset.univ.filter (fun i => p i = c),
            ∑ j ∈ Finset.univ.filter (fun j => p j = c), F i j := by
  have inner : ∀ i : Fin n,
      (∑ j, if p i = p j then F i j else 0)
        = ∑ j ∈ Finset.univ.filter (fun j => p i = p j), F i j :=
    fun i => (Finset.sum_filter (fun j => p i = p j) (F i)).symm
  simp_rw [inner]
  rw [← Finset.sum_fiberwise_of_maps_to (g := p) (t := Finset.univ.image p)
        (fun i _ => Finset.mem_image_of_mem p (Finset.mem_univ i))]
  refine Finset.sum_congr rfl fun c _ => Finset.sum_congr rfl fun i hi => ?_
  have hpi : p i = c := (Finset.mem_filter.mp hi).2
  refine Finset.sum_congr (Finset.filter_congr fun j _ => ?_) fun _ _ => rfl
  rw [hpi]; exact eq_comm

/-- **Modularity is a sum of within-community contributions.** Each community `c`
    contributes the block sum of `w_{ij} − γ k_i k_j / 2m` over node pairs both
    assigned to `c`. This is the form the aggregate graph realises: a community's
    block sum is the internal weight its aggregate self-loop carries. -/
theorem modularity_eq_communitySum (G : WeightedGraph n) (γ : ℝ) (p : Partition n) :
    modularity G γ p
      = (1 / G.twoM) * ∑ c ∈ Finset.univ.image p,
          ∑ i ∈ Finset.univ.filter (fun i => p i = c),
            ∑ j ∈ Finset.univ.filter (fun j => p j = c),
              (G.weight i j - γ * G.degree i * G.degree j / G.twoM) := by
  unfold modularity
  congr 1
  simp_rw [mul_ite, mul_one, mul_zero]
  rw [sum_diagonal_eq_sum_fiber p
    (fun i j => G.weight i j - γ * G.degree i * G.degree j / G.twoM)]

/-!
## The re-indexed aggregate graph

The decomposition above is the content of aggregation invariance; what remains is
the bookkeeping of collapsing each community to a single node on a fresh `Fin m`
index. `aggregate G p` is that graph: `m = numComm p` distinct communities, edge
weight the block sum between two communities, node size the summed sizes, and a
self-loop `weight A A` carrying community `A`'s internal weight. The payoff is
`modularity_aggregate_eq`: running the singleton partition on the aggregate gives
exactly the modularity of `p` on `G`, so a recursion may continue on the aggregate
without ever losing quality.
-/

/-- Number of distinct communities under `p`; the aggregate's node count. -/
abbrev numComm (p : Partition n) : ℕ := (Finset.univ.image p).card

/-- Block sum of edge weight between the communities labelled `a` and `b`. -/
noncomputable def blockWeight (G : WeightedGraph n) (p : Partition n) (a b : ℕ) : ℝ :=
  ∑ i ∈ Finset.univ.filter (fun i => p i = a),
    ∑ j ∈ Finset.univ.filter (fun j => p j = b), G.weight i j

/-- The community label carried by aggregate node `A`, via a fixed bijection
    between `Fin (numComm p)` and the community labels present in `p`. -/
noncomputable def commLabel (p : Partition n) (A : Fin (numComm p)) : ℕ :=
  ↑((Finset.univ.image p).equivFin.symm A)

/-- The aggregate graph: one node per community. Its self-loop `weight A A` is
    community `A`'s internal edge weight, its derived `degree` is the community's
    total degree (`aggregate_degree`), and its total weight is preserved
    (`aggregate_twoM`). -/
noncomputable def aggregate (G : WeightedGraph n) (p : Partition n) :
    WeightedGraph (numComm p) where
  weight A B := blockWeight G p (commLabel p A) (commLabel p B)
  weight_symm A B := by
    unfold blockWeight
    rw [Finset.sum_comm]
    exact Finset.sum_congr rfl fun x _ => Finset.sum_congr rfl fun y _ => G.weight_symm y x
  weight_nonneg A B := by
    unfold blockWeight
    exact Finset.sum_nonneg fun i _ => Finset.sum_nonneg fun j _ => G.weight_nonneg i j
  nodeSize A := ∑ i ∈ Finset.univ.filter (fun i => p i = commLabel p A), G.nodeSize i
  nodeSize_nonneg A := Finset.sum_nonneg fun i _ => G.nodeSize_nonneg i

/-- Reindexing: a sum of `g` over aggregate nodes equals the sum of `g` over the
    community labels present in `p`, via the fixed bijection. -/
lemma sum_commLabel (p : Partition n) (g : ℕ → ℝ) :
    ∑ A, g (commLabel p A) = ∑ c ∈ Finset.univ.image p, g c := by
  calc ∑ A, g (commLabel p A)
      = ∑ x : ↥(Finset.univ.image p), g ↑x :=
        Equiv.sum_comp (Finset.univ.image p).equivFin.symm (fun x => g ↑x)
    _ = ∑ c ∈ Finset.univ.image p, g c := Finset.sum_coe_sort (Finset.univ.image p) g

/-- The derived degree of an aggregate node is its community's total degree. -/
lemma aggregate_degree (G : WeightedGraph n) (p : Partition n) (A : Fin (numComm p)) :
    (aggregate G p).degree A
      = ∑ i ∈ Finset.univ.filter (fun i => p i = commLabel p A), G.degree i := by
  unfold WeightedGraph.degree
  simp only [aggregate, blockWeight]
  rw [Finset.sum_comm]
  refine Finset.sum_congr rfl fun i _ => ?_
  rw [sum_commLabel p (fun c => ∑ j ∈ Finset.univ.filter (fun j => p j = c), G.weight i j)]
  exact Finset.sum_fiberwise_of_maps_to
    (fun k _ => Finset.mem_image_of_mem p (Finset.mem_univ k)) (G.weight i)

/-- Aggregation preserves total edge weight: `2m' = 2m`. -/
lemma aggregate_twoM (G : WeightedGraph n) (p : Partition n) :
    (aggregate G p).twoM = G.twoM := by
  simp only [WeightedGraph.twoM]
  simp_rw [aggregate_degree]
  rw [sum_commLabel p (fun c => ∑ i ∈ Finset.univ.filter (fun i => p i = c), G.degree i)]
  exact Finset.sum_fiberwise_of_maps_to
    (fun i _ => Finset.mem_image_of_mem p (Finset.mem_univ i)) G.degree

/-- A community's block contribution splits into internal weight minus the
    resolution term built from its total degree. -/
lemma block_contribution (G : WeightedGraph n) (γ : ℝ) (s : Finset (Fin n)) :
    ∑ i ∈ s, ∑ j ∈ s, (G.weight i j - γ * G.degree i * G.degree j / G.twoM)
      = (∑ i ∈ s, ∑ j ∈ s, G.weight i j)
        - γ * ((∑ i ∈ s, G.degree i) * (∑ j ∈ s, G.degree j)) / G.twoM := by
  simp_rw [Finset.sum_sub_distrib]
  congr 1
  rw [Finset.sum_mul_sum, Finset.mul_sum, Finset.sum_div]
  refine Finset.sum_congr rfl fun i _ => ?_
  rw [Finset.mul_sum, Finset.sum_div]
  refine Finset.sum_congr rfl fun j _ => ?_
  ring

/-- **Aggregation invariance.** Running the singleton partition (each aggregate
    node its own community) on the aggregate graph yields exactly the modularity
    of `p` on the original graph. This is the graph-to-graph form of
    `modularity_eq_communitySum` and the second half of stepwise monotonicity:
    aggregation neither raises nor lowers quality, so a Louvain/Leiden recursion
    may continue on the aggregate. -/
theorem modularity_aggregate_eq (G : WeightedGraph n) (γ : ℝ) (p : Partition n) :
    modularity (aggregate G p) γ (fun A => (A : ℕ)) = modularity G γ p := by
  rw [modularity_eq_communitySum G γ p]
  simp only [modularity]
  rw [aggregate_twoM]
  congr 1
  rw [← sum_commLabel p fun c => ∑ i ∈ Finset.univ.filter (fun i => p i = c),
        ∑ j ∈ Finset.univ.filter (fun j => p j = c),
          (G.weight i j - γ * G.degree i * G.degree j / G.twoM)]
  refine Finset.sum_congr rfl fun A _ => ?_
  rw [block_contribution,
    Finset.sum_eq_single A
      (fun b _ hb => by rw [if_neg fun h => hb (Fin.val_injective h).symm]; exact mul_zero _)
      (fun h => absurd (Finset.mem_univ A) h),
    if_pos rfl, mul_one, aggregate_degree]
  simp only [aggregate, blockWeight]
  ring

end Meso
