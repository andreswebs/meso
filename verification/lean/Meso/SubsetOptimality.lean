/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.Convergence

/-!
# Subset-optimality (third paper theorem)

The strongest of the three Traag-Waltman-van Eck (2019) guarantees and the essence of a
best-in-world Leiden: at a fixed point, no *subset* of any community would raise quality
by splitting off. Where γ-separation (C1) and γ-connectivity (C2) reason about whole
communities and whole merges, subset-optimality reasons about every subset `S` of every
community `C` at once.

The honest fixed-point hypothesis here is *subset stability* (`IsSubsetStable`): no
reassignment of a whole node set improves CPM. This is strictly stronger than the
single-node local-move stability of `IsConverged` (recovered as the singleton case,
`IsSubsetStable.isLocalMoveStable`), and it is exactly the stability the Leiden
*refinement* targets — refinement considers moving subsets, not just single nodes, which
is why Leiden attains subset-optimality where plain Louvain does not.

The computational heart is the CPM gain of splitting a subset off into a fresh community
(`cpm_moveSubset_split`). Moving `S ⊆ C` to an unused label flips the "same community"
indicator at exactly the cross pairs `S × (C \ S)` (and their transpose), so the gain is

`ΔQ = −2·(e(S, C\S) − γ ‖S‖ ‖C\S‖)`,

via the indicator-change lemma `ind_subsetSplit` (the subset cousin of C1's
`ind_singletonMerge`). Subset stability forces `ΔQ ≤ 0`, hence

`e(S, C\S) ≥ γ ‖S‖ ‖C\S‖`  for every subset `S` of every community `C`

(`cpm_subsetGammaDense_of_stable`, packaged as `isSubsetOptimal_of_stable`). This is the
"no sparse cut" reading of γ-connectivity that C2 deferred: a community cannot be cut
into two γ-separated pieces, so it is well-connected not merely as a graph but densely,
at every scale.

One honest scope note, recorded in the CORRESPONDENCE divergence register: this delivers
the *split-off* half of subset-optimality (no subset wants to leave its community, the
no-sparse-cut bound). The *external* absolute bound `e(S, D) ≤ γ ‖S‖ ‖D‖` for a subset
against another community `D` does not follow from subset stability of `C` alone — only
the relative form does — so it is not claimed here.
-/

namespace Meso

variable {n : ℕ}

/-- Reassign every node of a set `S` to community `c`, leaving all other nodes fixed. The
    subset generalisation of `move`; `moveSubset p {v} c = move p v c`
    (`moveSubset_singleton`). -/
def moveSubset (p : Partition n) (S : Finset (Fin n)) (c : ℕ) : Partition n :=
  fun i => if i ∈ S then c else p i

/-- A subset move of a singleton is a single-node move. -/
lemma moveSubset_singleton (p : Partition n) (v : Fin n) (c : ℕ) :
    moveSubset p {v} c = move p v c := by
  funext i
  simp only [moveSubset, move, Function.update_apply, Finset.mem_singleton]

/-- There is always a community label unused by `p`: the image of `p` is finite and `ℕ`
    is infinite. Provides the fresh target the split-off move reassigns `S` to. -/
lemma exists_fresh_label (p : Partition n) : ∃ f : ℕ, ∀ i, p i ≠ f := by
  refine ⟨(Finset.univ.image p).sup id + 1, fun i h => ?_⟩
  have hle : p i ≤ (Finset.univ.image p).sup id :=
    Finset.le_sup (f := id) (Finset.mem_image_of_mem p (Finset.mem_univ i))
  omega

/-- **The indicator change of splitting a subset off.** With `S` contained in community
    `C` (`hS`) and `f` a fresh label (`hf`), reassigning all of `S` to `f` flips the
    "same community" indicator `δ` from `1` to `0` at exactly the cross pairs
    `S × (C \ S)` and their transpose `(C \ S) × S` (`T` collecting `C \ S`), leaving
    every other cell fixed. Written as a real identity to push through the CPM sum; the
    subset cousin of `ind_singletonMerge`. -/
lemma ind_subsetSplit {p : Partition n} {C : ℕ} {S T : Finset (Fin n)} {f : ℕ}
    (hS : ∀ i ∈ S, p i = C) (hf : ∀ k, p k ≠ f)
    (hT : ∀ j, j ∈ T ↔ p j = C ∧ j ∉ S) (i j : Fin n) :
    (if moveSubset p S f i = moveSubset p S f j then (1 : ℝ) else 0)
      = (if p i = p j then (1 : ℝ) else 0)
        - (if i ∈ S ∧ j ∈ T then (1 : ℝ) else 0)
        - (if i ∈ T ∧ j ∈ S then (1 : ℝ) else 0) := by
  by_cases hi : i ∈ S <;> by_cases hj : j ∈ S
  · have hmi : moveSubset p S f i = f := if_pos hi
    have hmj : moveSubset p S f j = f := if_pos hj
    rw [hmi, hmj, if_pos rfl,
      if_pos (show p i = p j by rw [hS i hi, hS j hj]),
      if_neg (show ¬(i ∈ S ∧ j ∈ T) from fun h => ((hT j).mp h.2).2 hj),
      if_neg (show ¬(i ∈ T ∧ j ∈ S) from fun h => ((hT i).mp h.1).2 hi)]
    ring
  · have hmi : moveSubset p S f i = f := if_pos hi
    have hmj : moveSubset p S f j = p j := if_neg hj
    rw [hmi, hmj,
      if_neg (show ¬(f = p j) from fun h => hf j h.symm),
      if_neg (show ¬(i ∈ T ∧ j ∈ S) from fun h => hj h.2)]
    by_cases hC : p j = C
    · rw [if_pos (show p i = p j by rw [hS i hi]; exact hC.symm),
        if_pos (show i ∈ S ∧ j ∈ T from ⟨hi, (hT j).mpr ⟨hC, hj⟩⟩)]
      ring
    · rw [if_neg (show ¬(p i = p j) by rw [hS i hi]; exact fun h => hC h.symm),
        if_neg (show ¬(i ∈ S ∧ j ∈ T) from fun h => hC ((hT j).mp h.2).1)]
      ring
  · have hmi : moveSubset p S f i = p i := if_neg hi
    have hmj : moveSubset p S f j = f := if_pos hj
    rw [hmi, hmj,
      if_neg (show ¬(p i = f) from fun h => hf i h),
      if_neg (show ¬(i ∈ S ∧ j ∈ T) from fun h => hi h.1)]
    by_cases hC : p i = C
    · rw [if_pos (show p i = p j by rw [hS j hj]; exact hC),
        if_pos (show i ∈ T ∧ j ∈ S from ⟨(hT i).mpr ⟨hC, hi⟩, hj⟩)]
      ring
    · rw [if_neg (show ¬(p i = p j) by rw [hS j hj]; exact fun h => hC h),
        if_neg (show ¬(i ∈ T ∧ j ∈ S) from fun h => hC ((hT i).mp h.1).1)]
      ring
  · have hmi : moveSubset p S f i = p i := if_neg hi
    have hmj : moveSubset p S f j = p j := if_neg hj
    rw [hmi, hmj,
      if_neg (show ¬(i ∈ S ∧ j ∈ T) from fun h => hi h.1),
      if_neg (show ¬(i ∈ T ∧ j ∈ S) from fun h => hj h.2)]
    ring

/-- A double sum weighted by a rectangular membership indicator collapses to the double
    sum over the two sets. The sum-algebra step that turns the indicator-change lemma
    into a block sum over `S × T`. -/
lemma sum_sum_mul_ite_mem (S T : Finset (Fin n)) (g : Fin n → Fin n → ℝ) :
    ∑ i, ∑ j, g i j * (if i ∈ S ∧ j ∈ T then (1 : ℝ) else 0)
      = ∑ i ∈ S, ∑ j ∈ T, g i j := by
  simp_rw [mul_ite, mul_one, mul_zero, ite_and, Finset.sum_ite_irrel, Finset.sum_const_zero,
    Finset.sum_ite_mem, Finset.univ_inter]

/-- Rectangular block algebra: a block sum of `w − γ f_i f_j` over `S × T` factors into
    the block weight minus `γ` times the product of the two marginal `f`-sums. The
    `S ≠ T` cousin of `block_sub_sq`. -/
lemma block_sub_prod (γ : ℝ) (S T : Finset (Fin n)) (w : Fin n → Fin n → ℝ) (f : Fin n → ℝ) :
    ∑ i ∈ S, ∑ j ∈ T, (w i j - γ * f i * f j)
      = (∑ i ∈ S, ∑ j ∈ T, w i j) - γ * ((∑ i ∈ S, f i) * (∑ j ∈ T, f j)) := by
  simp_rw [Finset.sum_sub_distrib]
  congr 1
  rw [Finset.sum_mul_sum, Finset.mul_sum]
  refine Finset.sum_congr rfl fun i _ => ?_
  rw [Finset.mul_sum]
  refine Finset.sum_congr rfl fun j _ => ?_
  ring

/-- **CPM gain of splitting a subset off.** Reassigning `S ⊆ C` to a fresh label `f`
    (`hS`, `hf`, with `T` collecting `C \ S`) changes CPM by exactly
    `−2·(e(S, C\S) − γ ‖S‖ ‖C\S‖)`: only the cross pairs between `S` and `C \ S` leave the
    community, and the term is symmetric. The subset-level move-gain formula whose sign
    subset-optimality reads off. -/
theorem cpm_moveSubset_split {H : WeightedGraph n} {γ : ℝ} {p : Partition n} {C : ℕ}
    {S T : Finset (Fin n)} {f : ℕ}
    (hS : ∀ i ∈ S, p i = C) (hf : ∀ k, p k ≠ f)
    (hT : ∀ j, j ∈ T ↔ p j = C ∧ j ∉ S) :
    cpm H γ (moveSubset p S f)
      = cpm H γ p
        - 2 * ∑ i ∈ S, ∑ j ∈ T, (H.weight i j - γ * H.nodeSize i * H.nodeSize j) := by
  have hsplit : cpm H γ (moveSubset p S f)
      = ∑ i, ∑ j,
          ((H.weight i j - γ * H.nodeSize i * H.nodeSize j) * (if p i = p j then (1 : ℝ) else 0)
            - (H.weight i j - γ * H.nodeSize i * H.nodeSize j)
                * (if i ∈ S ∧ j ∈ T then (1 : ℝ) else 0)
            - (H.weight i j - γ * H.nodeSize i * H.nodeSize j)
                * (if i ∈ T ∧ j ∈ S then (1 : ℝ) else 0)) := by
    unfold cpm
    refine Finset.sum_congr rfl fun i _ => Finset.sum_congr rfl fun j _ => ?_
    rw [ind_subsetSplit hS hf hT]; ring
  have hsym : ∑ i ∈ T, ∑ j ∈ S, (H.weight i j - γ * H.nodeSize i * H.nodeSize j)
      = ∑ i ∈ S, ∑ j ∈ T, (H.weight i j - γ * H.nodeSize i * H.nodeSize j) := by
    rw [Finset.sum_comm]
    exact Finset.sum_congr rfl fun x _ => Finset.sum_congr rfl fun y _ => by
      rw [H.weight_symm y x]; ring
  have hcpm : cpm H γ p
      = ∑ i, ∑ j, (H.weight i j - γ * H.nodeSize i * H.nodeSize j)
          * (if p i = p j then (1 : ℝ) else 0) := rfl
  rw [hsplit]
  simp_rw [Finset.sum_sub_distrib]
  rw [sum_sum_mul_ite_mem S T (fun i j => H.weight i j - γ * H.nodeSize i * H.nodeSize j),
    sum_sum_mul_ite_mem T S (fun i j => H.weight i j - γ * H.nodeSize i * H.nodeSize j), hsym,
    ← hcpm]
  simp_rw [Finset.sum_sub_distrib]
  ring

/-- **Subset stable.** No reassignment of a whole node set strictly improves CPM. The
    subset-level fixed point: strictly stronger than single-node local-move stability
    (`IsSubsetStable.isLocalMoveStable`), and the stability the Leiden refinement
    targets. -/
def IsSubsetStable (H : WeightedGraph n) (γ : ℝ) (p : Partition n) : Prop :=
  ∀ (S : Finset (Fin n)) (c : ℕ), cpm H γ (moveSubset p S c) ≤ cpm H γ p

/-- Subset stability implies single-node local-move stability (the singleton case), so it
    is a genuine strengthening of the first half of `IsConverged`. -/
theorem IsSubsetStable.isLocalMoveStable {H : WeightedGraph n} {γ : ℝ} {p : Partition n}
    (h : IsSubsetStable H γ p) : IsLocalMoveStable (cpm H γ) p := by
  intro v c
  have hstep := h {v} c
  rwa [moveSubset_singleton] at hstep

/-- **No subset splits off (the per-subset bound).** At a subset-stable partition, every
    subset `S` of a community `C` is γ-densely connected to the rest of the community:
    `e(S, C\S) ≥ γ ‖S‖ ‖C\S‖`. Split `S` off to a fresh community; subset stability makes
    the gain `≤ 0`, and the gain is `−2·(e(S, C\S) − γ ‖S‖ ‖C\S‖)`. This is the
    no-sparse-cut reading of γ-connectivity deferred from C2. -/
theorem cpm_subsetGammaDense_of_stable {H : WeightedGraph n} {γ : ℝ} {p : Partition n}
    (hstab : IsSubsetStable H γ p) {C : ℕ} {S : Finset (Fin n)} (hS : ∀ i ∈ S, p i = C) :
    γ * (∑ i ∈ S, H.nodeSize i)
        * (∑ j ∈ Finset.univ.filter (fun j => p j = C ∧ j ∉ S), H.nodeSize j)
      ≤ ∑ i ∈ S, ∑ j ∈ Finset.univ.filter (fun j => p j = C ∧ j ∉ S), H.weight i j := by
  obtain ⟨f, hf⟩ := exists_fresh_label p
  set T := Finset.univ.filter (fun j => p j = C ∧ j ∉ S) with hTdef
  have hT : ∀ j, j ∈ T ↔ p j = C ∧ j ∉ S := by
    intro j; rw [hTdef]; simp [Finset.mem_filter]
  have hstep := hstab S f
  rw [cpm_moveSubset_split hS hf hT] at hstep
  have hbracket : 0 ≤ ∑ i ∈ S, ∑ j ∈ T, (H.weight i j - γ * H.nodeSize i * H.nodeSize j) := by
    linarith
  rw [block_sub_prod] at hbracket
  rw [mul_assoc]
  linarith

/-- **Subset-optimality as a partition predicate.** Every subset of every community is
    γ-densely connected to the rest of its community (the no-sparse-cut bound for all
    subsets). The strongest of the three paper guarantees. -/
def IsSubsetOptimal (H : WeightedGraph n) (γ : ℝ) (p : Partition n) : Prop :=
  ∀ (C : ℕ) (S : Finset (Fin n)), (∀ i ∈ S, p i = C) →
    γ * (∑ i ∈ S, H.nodeSize i)
        * (∑ j ∈ Finset.univ.filter (fun j => p j = C ∧ j ∉ S), H.nodeSize j)
      ≤ ∑ i ∈ S, ∑ j ∈ Finset.univ.filter (fun j => p j = C ∧ j ∉ S), H.weight i j

/-- **Subset-optimality at a subset-stable partition (the paper theorem).** A subset-stable
    partition is subset-optimal: no subset of any community can be split off to raise
    quality, equivalently every community is γ-densely connected at every scale (no sparse
    cut). This is the strongest paper guarantee and the property distinguishing Leiden. -/
theorem isSubsetOptimal_of_stable {H : WeightedGraph n} {γ : ℝ} {p : Partition n}
    (hstab : IsSubsetStable H γ p) : IsSubsetOptimal H γ p :=
  fun _ _ hS => cpm_subsetGammaDense_of_stable hstab hS

end Meso
