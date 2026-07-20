/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.Convergence

/-!
# γ-separation (first paper theorem)

The first of the three Traag-Waltman-van Eck (2019) guarantees, stated in CPM about
a converged partition (`Meso.Convergence`). A partition is *γ-separated* when any two
distinct communities `C ≠ D` are only weakly connected relative to the resolution:

`e(C, D) ≤ γ · S_C · S_D`,

the between-community edge weight is below the γ threshold set by the two community
sizes. This is exactly the statement that no merge of `C` and `D` would raise CPM.

The honest route is *level stability*, not single-node stability. On the aggregate
graph each community is one node, so merging two communities is a single-node move
there; its CPM gain is `2·(e(C,D) − γ S_C S_D)` (`cpm_merge_two_singletons`). Level
stability forces that gain to be `≤ 0`, which is γ-separation
(`gammaSeparated_of_converged`). Single-node stability gives only the weaker relative
statement "no node has a strictly better community" (`cpm_noStrictlyBetterCommunity`),
because a node's own community is not empty, so its pull to it is not zero.

The computational core is the CPM gain of merging two singletons, and it needs the
move-gain formula: the only pairs whose "same community" indicator changes when node
`A` joins node `B`'s singleton are `(A,B)` and `(B,A)` (`ind_singletonMerge`).
-/

namespace Meso

variable {n m : ℕ}

/-- **The indicator change of merging two singletons.** Under the singleton partition
    `k ↦ k`, moving node `A` into `B`'s (singleton) community changes the "same
    community" indicator `δ` at exactly the two off-diagonal cells `(A,B)` and `(B,A)`
    (each rising from `0` to `1`), leaving every other cell fixed. Written as a real
    identity so it can be pushed through the CPM double sum. -/
lemma ind_singletonMerge (A B : Fin m) (hAB : A ≠ B) (i j : Fin m) :
    (if move (fun k => (k : ℕ)) A (B : ℕ) i = move (fun k => (k : ℕ)) A (B : ℕ) j
        then (1 : ℝ) else 0)
      = (if (i : ℕ) = (j : ℕ) then (1 : ℝ) else 0)
        + (if i = A ∧ j = B then (1 : ℝ) else 0)
        + (if i = B ∧ j = A then (1 : ℝ) else 0) := by
  have hAB' : (A : ℕ) ≠ (B : ℕ) := fun h => hAB (Fin.val_injective h)
  simp only [move, Function.update_apply, Fin.ext_iff]
  split_ifs <;> first | (exfalso; omega) | norm_num

/-- **CPM gain of merging two singletons.** Starting from the singleton partition
    `k ↦ k` on any graph `H`, reassigning node `A` to `B`'s community (a single
    local move) raises CPM by exactly `2·(w_{AB} − γ s_A s_B)`. This is the move-gain
    formula specialised to the aggregate level, where communities are single nodes;
    its sign is what γ-separation reads off. -/
theorem cpm_merge_two_singletons (H : WeightedGraph m) (γ : ℝ) (A B : Fin m) (hAB : A ≠ B) :
    cpm H γ (move (fun k => (k : ℕ)) A (B : ℕ))
      = cpm H γ (fun k => (k : ℕ))
        + 2 * (H.weight A B - γ * H.nodeSize A * H.nodeSize B) := by
  have h1 : ∑ i : Fin m, ∑ j : Fin m,
      (H.weight i j - γ * H.nodeSize i * H.nodeSize j) * (if i = A ∧ j = B then (1 : ℝ) else 0)
      = H.weight A B - γ * H.nodeSize A * H.nodeSize B := by
    rw [Finset.sum_eq_single_of_mem A (Finset.mem_univ A) (fun b _ hb => by simp [hb])]
    rw [Finset.sum_eq_single_of_mem B (Finset.mem_univ B) (fun b _ hb => by simp [hb])]
    simp
  have h2 : ∑ i : Fin m, ∑ j : Fin m,
      (H.weight i j - γ * H.nodeSize i * H.nodeSize j) * (if i = B ∧ j = A then (1 : ℝ) else 0)
      = H.weight B A - γ * H.nodeSize B * H.nodeSize A := by
    rw [Finset.sum_eq_single_of_mem B (Finset.mem_univ B) (fun b _ hb => by simp [hb])]
    rw [Finset.sum_eq_single_of_mem A (Finset.mem_univ A) (fun b _ hb => by simp [hb])]
    simp
  unfold cpm
  simp_rw [ind_singletonMerge A B hAB, mul_add, Finset.sum_add_distrib, h1, h2]
  rw [H.weight_symm B A]
  ring

/-- **γ-separation from level stability (general form).** If the singleton partition
    of a graph `H` is local-move stable under CPM (no merge of two nodes improves
    quality), then any two distinct nodes are γ-separated: their edge weight is below
    `γ` times the product of their node sizes. Applied to the aggregate graph, the
    "nodes" are communities and this is the paper's γ-separation. -/
theorem cpm_gammaSeparated_of_stable {H : WeightedGraph m} {γ : ℝ}
    (hstab : IsLocalMoveStable (cpm H γ) (fun k => (k : ℕ)))
    {A B : Fin m} (hAB : A ≠ B) :
    H.weight A B ≤ γ * H.nodeSize A * H.nodeSize B := by
  have h := hstab A (B : ℕ)
  rw [cpm_merge_two_singletons H γ A B hAB] at h
  linarith

/-- **γ-separation of a converged partition (the paper theorem).** At a partition that
    has converged under CPM, any two distinct communities `A ≠ B` are γ-separated: the
    aggregate edge weight between them (the total between-community edge weight
    `e(A,B)`) is at most `γ` times the product of their sizes. The hypothesis used is
    exactly level stability — no community merge improves quality — which is the second
    half of `IsConverged`. -/
theorem gammaSeparated_of_converged {γ : ℝ} {G : WeightedGraph n} {p : Partition n}
    (h : IsConverged (cpmF γ) G p) {A B : Fin (numComm p)} (hAB : A ≠ B) :
    (aggregate G p).weight A B
      ≤ γ * (aggregate G p).nodeSize A * (aggregate G p).nodeSize B := by
  have hstab : IsLocalMoveStable (cpm (aggregate G p) γ) (fun k => (k : ℕ)) := h.levelStable
  exact cpm_gammaSeparated_of_stable hstab hAB

/-- **γ-separation in community vocabulary.** The same guarantee as
    `gammaSeparated_of_converged`, phrased with the CPM community aggregates: the
    between-community block weight is below `γ` times the product of the two community
    sizes. `blockWeight` and `communitySize` are definitionally the aggregate graph's
    edge weight and node size, so this is the paper's `e(C,D) ≤ γ S_C S_D`. -/
theorem gammaSeparated_blockWeight_of_converged {γ : ℝ} {G : WeightedGraph n}
    {p : Partition n} (h : IsConverged (cpmF γ) G p) {A B : Fin (numComm p)} (hAB : A ≠ B) :
    blockWeight G p (commLabel p A) (commLabel p B)
      ≤ γ * communitySize G p (commLabel p A) * communitySize G p (commLabel p B) :=
  gammaSeparated_of_converged h hAB

/-- **No node has a strictly better community (single-node form).** At a converged
    partition, reassigning any node `v` to any community `c` does not raise CPM. This
    is the direct unfolding of local-move stability (the first half of `IsConverged`),
    the relative companion to the absolute γ-separation bound above. -/
theorem cpm_noStrictlyBetterCommunity {γ : ℝ} {G : WeightedGraph n} {p : Partition n}
    (h : IsConverged (cpmF γ) G p) (v : Fin n) (c : ℕ) :
    cpm G γ (move p v c) ≤ cpm G γ p :=
  h.localMoveStable v c

end Meso
