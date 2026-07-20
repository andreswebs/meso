/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.DirectedGraph
import Meso.DirectedConvergence
import Meso.SubsetOptimality

/-!
# The directed subset conditional

The directed analogue of `Meso/SubsetOptimality.lean`'s third paper guarantee,
stated in Leicht-Newman directed modularity. Per the Phase 3 triage
(`docs/research/directed-modularity-triage.md`, sections 3 and 7),
subset-optimality as an *output property* is REFUTED for directed modularity (the
n=4 fixture, tracked as a Go characterization test and in `mes-niic`): the
algorithm does not always attain the hypothesis. What survives, and is proved
here, is the *conditional*: from directed subset stability
(`IsDirectedSubsetStable`), every proper nonempty subset `S` of a community `C` is
γ-densely connected to the rest `T = C \ S` in both arc directions,

`e(S, T) + e(T, S) ≥ (γ / m) · (Kout_S · Kin_T + Kout_T · Kin_S)`.

The computational heart is the directed-modularity gain of splitting a subset off
(`directedModularity_moveSubset_split`). Moving `S ⊆ C` to a fresh label flips the
"same community" indicator at exactly the cross pairs `S × T` and `T × S`, via the
graph-agnostic `ind_subsetSplit` (reused directly). Pushing the directed kernel
`w_{ij} − γ·kout_i·kin_j/m` through that indicator change, with the graph-agnostic
`sum_sum_mul_ite_mem` (reused directly) collapsing each indicator block, gives the
split gain

`ΔQ = −(1/m)·[ (e(S,T) + e(T,S)) − (γ/m)(Kout_S·Kin_T + Kout_T·Kin_S) ]`.

Two things differ from CPM. The directed kernel is asymmetric, so the `S × T` and
`T × S` blocks do not collapse to a factor of two; both survive. And factoring a
rectangular block of the directed kernel needs a two-degree analogue of
`block_sub_prod` (`directed_block_sub_prod` below), since the null term pairs `S`'s
out-degrees with `T`'s in-degrees rather than a single node-size function. Directed
subset stability forces `ΔQ ≤ 0`, which is the bound above.
-/

namespace Meso

variable {n : ℕ}

/-- **Rectangular directed block algebra.** A block sum of `w_{ij} − γ·g_i·h_j/tw`
    over `S × T` factors into the block weight minus `γ/tw` times the product of the
    two marginal sums (`g` over `S`, `h` over `T`). The two-degree, rectangular
    cousin of `block_sub_prod` (which uses a single node-size function on a diagonal
    block) and of `directed_block_contribution` (which is the diagonal `S = T`
    directed case). The directed null term pairs out-degrees on the left with
    in-degrees on the right, so both a separate `g`, `h` and the `/tw` scale are
    needed. -/
lemma directed_block_sub_prod (γ tw : ℝ) (S T : Finset (Fin n)) (w : Fin n → Fin n → ℝ)
    (g h : Fin n → ℝ) :
    ∑ i ∈ S, ∑ j ∈ T, (w i j - γ * g i * h j / tw)
      = (∑ i ∈ S, ∑ j ∈ T, w i j) - γ * ((∑ i ∈ S, g i) * (∑ j ∈ T, h j)) / tw := by
  simp_rw [Finset.sum_sub_distrib]
  congr 1
  rw [Finset.sum_mul_sum, Finset.mul_sum, Finset.sum_div]
  refine Finset.sum_congr rfl fun i _ => ?_
  rw [Finset.mul_sum, Finset.sum_div]
  refine Finset.sum_congr rfl fun j _ => ?_
  ring

/-- **Directed-modularity gain of splitting a subset off.** Reassigning `S ⊆ C` to a
    fresh label `f` (`hS`, `hf`, with `T` collecting `C \ S`) changes directed
    modularity by exactly `−(1/m)·[ block(S,T) + block(T,S) ]`, where each block is
    the directed kernel summed over the cross pairs: only the cross pairs between `S`
    and `C \ S` leave the community. Directed analogue of `cpm_moveSubset_split`;
    the asymmetric kernel keeps the `S × T` and `T × S` blocks separate, and the
    `1/m` factor rides along from `directedModularity`. -/
theorem directedModularity_moveSubset_split {H : DirectedWeightedGraph n} {γ : ℝ}
    {p : Partition n} {C : ℕ} {S T : Finset (Fin n)} {f : ℕ}
    (hS : ∀ i ∈ S, p i = C) (hf : ∀ k, p k ≠ f)
    (hT : ∀ j, j ∈ T ↔ p j = C ∧ j ∉ S) :
    directedModularity H γ (moveSubset p S f)
      = directedModularity H γ p
        - (1 / H.totalWeight) *
            ((∑ i ∈ S, ∑ j ∈ T, (H.weight i j - γ * H.outDegree i * H.inDegree j / H.totalWeight))
             + (∑ i ∈ T, ∑ j ∈ S,
                 (H.weight i j - γ * H.outDegree i * H.inDegree j / H.totalWeight))) := by
  have hsplit :
      (∑ i, ∑ j, (H.weight i j - γ * H.outDegree i * H.inDegree j / H.totalWeight)
          * (if moveSubset p S f i = moveSubset p S f j then (1 : ℝ) else 0))
        = ∑ i, ∑ j,
            ((H.weight i j - γ * H.outDegree i * H.inDegree j / H.totalWeight)
                * (if p i = p j then (1 : ℝ) else 0)
              - (H.weight i j - γ * H.outDegree i * H.inDegree j / H.totalWeight)
                  * (if i ∈ S ∧ j ∈ T then (1 : ℝ) else 0)
              - (H.weight i j - γ * H.outDegree i * H.inDegree j / H.totalWeight)
                  * (if i ∈ T ∧ j ∈ S then (1 : ℝ) else 0)) := by
    refine Finset.sum_congr rfl fun i _ => Finset.sum_congr rfl fun j _ => ?_
    rw [ind_subsetSplit hS hf hT]; ring
  unfold directedModularity
  rw [← mul_sub]
  congr 1
  rw [hsplit]
  simp_rw [Finset.sum_sub_distrib]
  rw [sum_sum_mul_ite_mem S T
      (fun i j => H.weight i j - γ * H.outDegree i * H.inDegree j / H.totalWeight),
    sum_sum_mul_ite_mem T S
      (fun i j => H.weight i j - γ * H.outDegree i * H.inDegree j / H.totalWeight)]
  simp_rw [Finset.sum_sub_distrib]
  ring

/-- **The directed subset conditional (the surviving guarantee).** At a directed
    subset-stable partition, every proper nonempty subset `S` of a community `C` is
    γ-densely connected to the rest `T = C \ S` in both arc directions:
    `e(S,T) + e(T,S) ≥ (γ/m)·(Kout_S·Kin_T + Kout_T·Kin_S)`. Split `S` off to a fresh
    community; directed subset stability makes the gain `≤ 0`, and the gain is
    `−(1/m)·[ (e(S,T)+e(T,S)) − (γ/m)(...) ]`. Directed analogue of
    `cpm_subsetGammaDense_of_stable`; per the triage this is a *conditional* on a
    hypothesis the algorithm does not always attain (subset-optimality as an output
    property is refuted). -/
theorem directedSubsetGammaDense_of_subsetStable {G : DirectedWeightedGraph n} {γ : ℝ}
    {p : Partition n} (hstab : IsDirectedSubsetStable G γ p)
    {C : ℕ} {S : Finset (Fin n)} (hS : ∀ i ∈ S, p i = C) :
    γ * ((∑ i ∈ S, G.outDegree i)
            * (∑ j ∈ Finset.univ.filter (fun j => p j = C ∧ j ∉ S), G.inDegree j)
         + (∑ i ∈ Finset.univ.filter (fun j => p j = C ∧ j ∉ S), G.outDegree i)
            * (∑ j ∈ S, G.inDegree j)) / G.totalWeight
      ≤ (∑ i ∈ S, ∑ j ∈ Finset.univ.filter (fun j => p j = C ∧ j ∉ S), G.weight i j)
        + (∑ i ∈ Finset.univ.filter (fun j => p j = C ∧ j ∉ S), ∑ j ∈ S, G.weight i j) := by
  obtain ⟨f, hf⟩ := exists_fresh_label p
  set T := Finset.univ.filter (fun j => p j = C ∧ j ∉ S) with hTdef
  have hT : ∀ j, j ∈ T ↔ p j = C ∧ j ∉ S := by
    intro j; rw [hTdef]; simp [Finset.mem_filter]
  have hstep := hstab S f
  rw [directedModularity_moveSubset_split hS hf hT,
    directed_block_sub_prod γ G.totalWeight S T G.weight G.outDegree G.inDegree,
    directed_block_sub_prod γ G.totalWeight T S G.weight G.outDegree G.inDegree] at hstep
  rcases eq_or_ne G.totalWeight 0 with hm | hm
  · rw [hm, div_zero]
    exact add_nonneg
      (Finset.sum_nonneg fun i _ => Finset.sum_nonneg fun j _ => G.weight_nonneg i j)
      (Finset.sum_nonneg fun i _ => Finset.sum_nonneg fun j _ => G.weight_nonneg i j)
  · have hinv : 0 < 1 / G.totalWeight :=
      one_div_pos.mpr (lt_of_le_of_ne G.totalWeight_nonneg (Ne.symm hm))
    have hb : (1 / G.totalWeight) * 0
        ≤ (1 / G.totalWeight)
            * (((∑ i ∈ S, ∑ j ∈ T, G.weight i j)
                  - γ * ((∑ i ∈ S, G.outDegree i) * (∑ j ∈ T, G.inDegree j)) / G.totalWeight)
               + ((∑ i ∈ T, ∑ j ∈ S, G.weight i j)
                  - γ * ((∑ i ∈ T, G.outDegree i) * (∑ j ∈ S, G.inDegree j)) / G.totalWeight)) := by
      rw [mul_zero]; linarith
    have hbr := le_of_mul_le_mul_left hb hinv
    have hexp : γ * ((∑ i ∈ S, G.outDegree i) * (∑ j ∈ T, G.inDegree j)
                     + (∑ i ∈ T, G.outDegree i) * (∑ j ∈ S, G.inDegree j)) / G.totalWeight
        = γ * ((∑ i ∈ S, G.outDegree i) * (∑ j ∈ T, G.inDegree j)) / G.totalWeight
          + γ * ((∑ i ∈ T, G.outDegree i) * (∑ j ∈ S, G.inDegree j)) / G.totalWeight := by ring
    linarith

end Meso
