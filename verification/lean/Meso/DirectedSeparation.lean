/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.DirectedAggregate
import Meso.DirectedMove
import Meso.DirectedConvergence
import Meso.Separation

/-!
# Directed γ-separation

The directed analogue of `Meso/Separation.lean`'s first paper guarantee, stated in
Leicht-Newman directed modularity about a directed level-stable partition. Per the
Phase 3 triage (`docs/research/directed-modularity-triage.md`, section 3), the
directed separation bound is

`e(C, D) + e(D, C) ≤ (γ / m) · (Kout_C · Kin_D + Kout_D · Kin_C)`

for any two distinct occupied communities `C ≠ D`: the total between-community arc
weight in both directions is below the resolution threshold set by the two
communities' out- and in-degrees. This is exactly the statement that no merge of
`C` and `D` raises directed modularity.

The honest route is *level stability*, as undirected. On the directed aggregate
graph each community is one node, so merging two communities is a single-node move
there; its directed-modularity gain is
`(1/m)·[(w_{AB}+w_{BA}) − (γ/m)(kout_A·kin_B + kout_B·kin_A)]`
(`directedModularity_merge_two_singletons`). Level stability forces that gain to be
`≤ 0`, which is directed γ-separation (`directedGammaSeparated_of_levelStable`).

The proof mirrors `cpm_merge_two_singletons` structurally: the only pairs whose
"same community" indicator changes when node `A` joins node `B`'s singleton are
`(A,B)` and `(B,A)`, via the graph-agnostic `ind_singletonMerge` (reused directly).
Two things differ from CPM. The directed kernel `w_{ij} − γ·kout_i·kin_j/m` is not
symmetric, so `w_{AB}` and `w_{BA}` do not collapse; both survive. And directed
modularity carries a leading `1/m` factor, so reading the bound off the sign of the
gain needs the resolution scale `m > 0`; on an arcless graph (`m = 0`) both sides
of the bound are zero and it holds trivially.
-/

namespace Meso

variable {n m : ℕ}

/-- On a directed graph with no arc weight (`totalWeight = 0`), every arc weight is
    zero: the total is a sum of nonnegative arc weights, so each vanishes. The
    degenerate case the `m = 0` branch of γ-separation discharges. -/
lemma directedWeight_eq_zero_of_totalWeight_eq_zero (H : DirectedWeightedGraph m)
    (h : H.totalWeight = 0) (i j : Fin m) : H.weight i j = 0 := by
  have hsum : ∑ i, ∑ j, H.weight i j = 0 := by rw [← H.totalWeight_eq_sum_sum]; exact h
  have hrow : ∀ i ∈ Finset.univ, ∑ j, H.weight i j = 0 :=
    (Finset.sum_eq_zero_iff_of_nonneg
      (fun i _ => Finset.sum_nonneg fun j _ => H.weight_nonneg i j)).mp hsum
  exact (Finset.sum_eq_zero_iff_of_nonneg
    (fun j _ => H.weight_nonneg i j)).mp (hrow i (Finset.mem_univ i)) j (Finset.mem_univ j)

/-- **Directed-modularity gain of merging two singletons.** Starting from the
    singleton partition `k ↦ k` on any directed graph `H`, reassigning node `A` to
    `B`'s community (a single local move) raises directed modularity by exactly
    `(1/m)·[(w_{AB}+w_{BA}) − (γ/m)(kout_A·kin_B + kout_B·kin_A)]`. This is the
    directed move-gain formula specialised to the aggregate level, where communities
    are single nodes; its sign is what γ-separation reads off. Mirrors
    `cpm_merge_two_singletons`; the asymmetric kernel keeps both `w_{AB}` and
    `w_{BA}`, and the `1/m` factor rides along from `directedModularity`. -/
theorem directedModularity_merge_two_singletons (H : DirectedWeightedGraph m) (γ : ℝ)
    (A B : Fin m) (hAB : A ≠ B) :
    directedModularity H γ (move (fun k => (k : ℕ)) A (B : ℕ))
      = directedModularity H γ (fun k => (k : ℕ))
        + (1 / H.totalWeight)
            * ((H.weight A B + H.weight B A)
               - γ * (H.outDegree A * H.inDegree B + H.outDegree B * H.inDegree A)
                 / H.totalWeight) := by
  have h1 : ∑ i : Fin m, ∑ j : Fin m,
      (H.weight i j - γ * H.outDegree i * H.inDegree j / H.totalWeight)
        * (if i = A ∧ j = B then (1 : ℝ) else 0)
      = H.weight A B - γ * H.outDegree A * H.inDegree B / H.totalWeight := by
    rw [Finset.sum_eq_single_of_mem A (Finset.mem_univ A) (fun b _ hb => by simp [hb])]
    rw [Finset.sum_eq_single_of_mem B (Finset.mem_univ B) (fun b _ hb => by simp [hb])]
    simp
  have h2 : ∑ i : Fin m, ∑ j : Fin m,
      (H.weight i j - γ * H.outDegree i * H.inDegree j / H.totalWeight)
        * (if i = B ∧ j = A then (1 : ℝ) else 0)
      = H.weight B A - γ * H.outDegree B * H.inDegree A / H.totalWeight := by
    rw [Finset.sum_eq_single_of_mem B (Finset.mem_univ B) (fun b _ hb => by simp [hb])]
    rw [Finset.sum_eq_single_of_mem A (Finset.mem_univ A) (fun b _ hb => by simp [hb])]
    simp
  unfold directedModularity
  simp_rw [ind_singletonMerge A B hAB, mul_add, Finset.sum_add_distrib, h1, h2]
  ring

/-- **Directed γ-separation from level stability (general form).** If the singleton
    partition of a directed graph `H` is local-move stable under directed modularity
    (no merge of two nodes improves quality), then any two distinct nodes are
    γ-separated: their two-directional arc weight is below `γ/m` times the sum of
    cross out/in-degree products. Applied to the directed aggregate graph, the
    "nodes" are communities and this is directed γ-separation. Mirrors
    `cpm_gammaSeparated_of_stable`; the `m = 0` branch is degenerate (all arc weight
    zero). -/
theorem directedGammaSeparated_of_stable {H : DirectedWeightedGraph m} {γ : ℝ}
    (hstab : IsLocalMoveStable (directedModularity H γ) (fun k => (k : ℕ)))
    {A B : Fin m} (hAB : A ≠ B) :
    H.weight A B + H.weight B A
      ≤ γ * (H.outDegree A * H.inDegree B + H.outDegree B * H.inDegree A) / H.totalWeight := by
  have hmove := hstab A (B : ℕ)
  rw [directedModularity_merge_two_singletons H γ A B hAB] at hmove
  rcases eq_or_ne H.totalWeight 0 with hm | hm
  · rw [directedWeight_eq_zero_of_totalWeight_eq_zero H hm A B,
        directedWeight_eq_zero_of_totalWeight_eq_zero H hm B A, hm]
    simp
  · have hinv : 0 < 1 / H.totalWeight :=
      one_div_pos.mpr (lt_of_le_of_ne H.totalWeight_nonneg (Ne.symm hm))
    have hbr : (1 / H.totalWeight)
        * ((H.weight A B + H.weight B A)
           - γ * (H.outDegree A * H.inDegree B + H.outDegree B * H.inDegree A) / H.totalWeight)
        ≤ (1 / H.totalWeight) * 0 := by rw [mul_zero]; linarith
    have := le_of_mul_le_mul_left hbr hinv
    linarith

/-- **Directed γ-separation of a level-stable partition (the paper theorem).** At a
    partition that is directed level-stable, any two distinct communities `A ≠ B` of
    the aggregate are γ-separated: the two-directional aggregate arc weight is at
    most `γ/m` times the sum of cross out/in-degree products. The hypothesis used is
    exactly directed level stability. Mirrors `gammaSeparated_of_converged`. -/
theorem directedGammaSeparated_of_levelStable {γ : ℝ} {G : DirectedWeightedGraph n}
    {p : Partition n} (h : IsDirectedLevelStable G γ p)
    {A B : Fin (numComm p)} (hAB : A ≠ B) :
    (directedAggregate G p).weight A B + (directedAggregate G p).weight B A
      ≤ γ * ((directedAggregate G p).outDegree A * (directedAggregate G p).inDegree B
             + (directedAggregate G p).outDegree B * (directedAggregate G p).inDegree A)
        / (directedAggregate G p).totalWeight :=
  directedGammaSeparated_of_stable h hAB

/-- **Directed γ-separation in community vocabulary.** The same guarantee as
    `directedGammaSeparated_of_levelStable`, phrased with the directed community
    aggregates: the two-directional between-community block weight is below `γ/m`
    times the sum of cross community out/in-degree products,
    `e(C,D) + e(D,C) ≤ (γ/m)(Kout_C·Kin_D + Kout_D·Kin_C)`. `directedBlockWeight`,
    `commOutDegree`, `commInDegree`, and `totalWeight` are definitionally the
    aggregate graph's arc weight, out/in-degrees, and total, so this is the triage's
    directed separation bound. Mirrors `gammaSeparated_blockWeight_of_converged`. -/
theorem directedGammaSeparated_blockWeight_of_levelStable {γ : ℝ}
    {G : DirectedWeightedGraph n} {p : Partition n} (h : IsDirectedLevelStable G γ p)
    {A B : Fin (numComm p)} (hAB : A ≠ B) :
    directedBlockWeight G p (commLabel p A) (commLabel p B)
        + directedBlockWeight G p (commLabel p B) (commLabel p A)
      ≤ γ * (G.commOutDegree p (commLabel p A) * G.commInDegree p (commLabel p B)
             + G.commOutDegree p (commLabel p B) * G.commInDegree p (commLabel p A))
        / G.totalWeight := by
  have hbase := directedGammaSeparated_of_levelStable h hAB
  rw [directedAggregate_outDegree, directedAggregate_outDegree,
    directedAggregate_inDegree, directedAggregate_inDegree, directedAggregate_totalWeight] at hbase
  exact hbase

end Meso
