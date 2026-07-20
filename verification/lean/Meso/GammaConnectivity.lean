/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.Refinement
import Meso.CPM

/-!
# γ-connectivity (second paper theorem)

The refinement guarantee upgraded from plain connectivity to the γ-density
bound the well-connectedness gate enforces. `Meso.Refinement` modelled the
gate by only one of its consequences — a shared edge — and proved that edge-merges
preserve *connectivity*. Its divergence note deferred the gate's γ-density content to
this tier; that is what lands here.

The honest inductive invariant is *internal γ-density* (`IsGammaDense`, `Meso.CPM`): a
community `C` is γ-dense when its internal weight covers the resolution term,
`e_C ≥ γ S_C²` (equivalently its CPM contribution is nonnegative). This is closed
under the gated merge, and the arithmetic is the CPM cousin of
`connected_induce_union_of_adj`:

`e(C₁ ∪ C₂) = e_{C₁} + e_{C₂} + 2·e(C₁,C₂) ≥ γS_{C₁}² + γS_{C₂}² + 2γS_{C₁}S_{C₂} = γS_C²`,

using that each piece is γ-dense and that the cut between them is γ-dense
(`gammaDense_union`). So the gate carries two consequences at once — a shared edge
and a γ-dense cut (density, here) — and a gated merge preserves
both, i.e. preserves *γ-well-connectedness* = connected ∧ γ-dense
(`GammaMergeStep.gammaWellConnectedCommunities`).

Two honest scope notes, recorded in the CORRESPONDENCE divergence register. First,
singletons are not γ-dense (a lone node's self-loop rarely covers `γ s²`), so unlike
connectivity there is no singleton base; the guarantee is preservation from a γ-dense
partition, which is what the gate bootstraps. Second, the paper's stronger "no sparse
cut" γ-connectivity is *not* closed under pairwise merges (the cross-cut terms are
unbounded by the piecewise hypotheses), so it is a convergence property and lands with
subset-optimality (C3), not here.
-/

namespace Meso

variable {n : ℕ}

/-- **γ-density is closed under a γ-dense cut.** If disjoint node sets `S` and `T` are
    each internally γ-dense and the cut weight between them covers `γ ‖S‖ ‖T‖`, then
    the union is internally γ-dense. This is the density arithmetic the refinement gate
    relies on: internal weight of a union is the two internal weights plus twice the
    cut, and squared size expands to match. The CPM analog of
    `connected_induce_union_of_adj`. -/
lemma gammaDense_union {G : WeightedGraph n} {γ : ℝ} {S T : Finset (Fin n)}
    (hdisj : Disjoint S T)
    (hS : γ * (∑ i ∈ S, G.nodeSize i) ^ 2 ≤ ∑ i ∈ S, ∑ j ∈ S, G.weight i j)
    (hT : γ * (∑ i ∈ T, G.nodeSize i) ^ 2 ≤ ∑ i ∈ T, ∑ j ∈ T, G.weight i j)
    (hcut : γ * (∑ i ∈ S, G.nodeSize i) * (∑ i ∈ T, G.nodeSize i)
              ≤ ∑ i ∈ S, ∑ j ∈ T, G.weight i j) :
    γ * (∑ i ∈ S ∪ T, G.nodeSize i) ^ 2 ≤ ∑ i ∈ S ∪ T, ∑ j ∈ S ∪ T, G.weight i j := by
  have hsize : ∑ i ∈ S ∪ T, G.nodeSize i
      = (∑ i ∈ S, G.nodeSize i) + (∑ i ∈ T, G.nodeSize i) := Finset.sum_union hdisj
  have hcross : ∑ i ∈ T, ∑ j ∈ S, G.weight i j = ∑ i ∈ S, ∑ j ∈ T, G.weight i j := by
    rw [Finset.sum_comm]
    exact Finset.sum_congr rfl fun x _ => Finset.sum_congr rfl fun y _ => G.weight_symm y x
  have hweight : ∑ i ∈ S ∪ T, ∑ j ∈ S ∪ T, G.weight i j
      = (∑ i ∈ S, ∑ j ∈ S, G.weight i j) + (∑ i ∈ T, ∑ j ∈ T, G.weight i j)
        + 2 * (∑ i ∈ S, ∑ j ∈ T, G.weight i j) := by
    simp_rw [Finset.sum_union hdisj, Finset.sum_add_distrib]
    rw [hcross]; ring
  rw [hsize, hweight]
  nlinarith [hS, hT, hcut]

/-- The merged community `a` (as a `Finset`) is the union of the old fibers `a` and
    `b`; the `Finset` counterpart of `merge_fiber_eq_union` used for the density
    arithmetic. -/
lemma merge_filter_eq_union (p : Partition n) (a b : ℕ) :
    Finset.univ.filter (fun i => mergeCommunities p a b i = a)
      = Finset.univ.filter (fun i => p i = a) ∪ Finset.univ.filter (fun i => p i = b) := by
  ext i
  simp only [Finset.mem_filter, Finset.mem_univ, true_and, Finset.mem_union, mergeCommunities]
  by_cases h : p i = b <;> simp [h]

/-- Fibers other than `a` and `b` are unchanged by the merge; the `Finset` counterpart
    of `merge_fiber_eq_of_ne`. -/
lemma merge_filter_eq_of_ne (p : Partition n) (a b c : ℕ) (hca : c ≠ a) (hcb : c ≠ b) :
    Finset.univ.filter (fun i => mergeCommunities p a b i = c)
      = Finset.univ.filter (fun i => p i = c) := by
  ext i
  simp only [Finset.mem_filter, Finset.mem_univ, true_and, mergeCommunities]
  by_cases h : p i = b <;> simp [h, Ne.symm hca, Ne.symm hcb]

/-- The γ-density guarantee as a partition predicate: every occupied community is
    internally γ-dense. The density analog of `ConnectedCommunities`. -/
def GammaDenseCommunities (G : WeightedGraph n) (γ : ℝ) (p : Partition n) : Prop :=
  ∀ c ∈ Finset.univ.image p, IsGammaDense G γ p c

/-- γ-well-connected communities: connected *and* internally γ-dense. -/
def GammaWellConnectedCommunities (G : WeightedGraph n) (γ : ℝ) (p : Partition n) : Prop :=
  ConnectedCommunities G p ∧ GammaDenseCommunities G γ p

/-- One γ-gated refinement merge. Beyond `MergeStep`'s shared edge (the connectivity
    gate), it carries the gate's γ-density content: the cut between the two merged
    communities is γ-dense, `γ S_a S_b ≤ e(a,b)` (`communitySize`, `blockWeight`). Both
    gate consequences appear, so a step is simultaneously an edge-merge and a
    density-preserving merge. -/
def GammaMergeStep (G : WeightedGraph n) (γ : ℝ) (p q : Partition n) : Prop :=
  ∃ a b, a ≠ b
    ∧ (∃ ia ib, p ia = a ∧ p ib = b ∧ (G.simpleGraph).Adj ia ib)
    ∧ (γ * communitySize G p a * communitySize G p b ≤ blockWeight G p a b)
    ∧ q = mergeCommunities p a b

/-- A γ-gated merge is in particular an edge-merge: drop the density gate, keep the
    shared edge. This routes the connectivity half through `MergeStep.connectedCommunities`. -/
theorem GammaMergeStep.mergeStep {G : WeightedGraph n} {γ : ℝ} {p q : Partition n}
    (h : GammaMergeStep G γ p q) : MergeStep G p q := by
  obtain ⟨a, b, hab, hedge, _, hq⟩ := h
  exact ⟨a, b, hab, hedge, hq⟩

/-- **A γ-gated merge preserves γ-density.** If every community of `p` is γ-dense and
    `q` merges two of them across a γ-dense cut, every community of `q` is γ-dense: the
    merged community is a union of two γ-dense fibers over a γ-dense cut
    (`gammaDense_union`), and all others are unchanged. This is the density cousin of
    `MergeStep.connectedCommunities`. -/
theorem GammaMergeStep.gammaDenseCommunities (G : WeightedGraph n) (γ : ℝ) (p q : Partition n)
    (hp : GammaDenseCommunities G γ p) (h : GammaMergeStep G γ p q) :
    GammaDenseCommunities G γ q := by
  obtain ⟨a, b, hab, ⟨ia, ib, hpa, hpb, _⟩, hcut, rfl⟩ := h
  intro c hc
  by_cases hca : c = a
  · rw [hca]
    have hSa : IsGammaDense G γ p a := hp a (Finset.mem_image.mpr ⟨ia, Finset.mem_univ _, hpa⟩)
    have hSb : IsGammaDense G γ p b := hp b (Finset.mem_image.mpr ⟨ib, Finset.mem_univ _, hpb⟩)
    have hdisj : Disjoint (Finset.univ.filter (fun i => p i = a))
        (Finset.univ.filter (fun i => p i = b)) := by
      rw [Finset.disjoint_left]
      intro i hia hib
      rw [Finset.mem_filter] at hia hib
      exact hab (hia.2.symm.trans hib.2)
    unfold IsGammaDense communityInternalWeight communitySize
    rw [merge_filter_eq_union p a b]
    exact gammaDense_union hdisj hSa hSb hcut
  · have hcb : c ≠ b := fun hcb' => merge_not_mem_image p a b hab (hcb' ▸ hc)
    have hpic : ∃ i, p i = c := by
      obtain ⟨i, -, hi⟩ := Finset.mem_image.mp hc
      refine ⟨i, ?_⟩
      simp only [mergeCommunities] at hi
      by_cases h : p i = b
      · rw [if_pos h] at hi; exact absurd hi.symm hca
      · rw [if_neg h] at hi; exact hi
    unfold IsGammaDense communityInternalWeight communitySize
    rw [merge_filter_eq_of_ne p a b c hca hcb]
    exact hp c (Finset.mem_image.mpr ⟨hpic.choose, Finset.mem_univ _, hpic.choose_spec⟩)

/-- **A γ-gated merge preserves γ-well-connectedness.** Connectivity via
    `MergeStep.connectedCommunities` through `GammaMergeStep.mergeStep` and γ-density
    via `GammaMergeStep.gammaDenseCommunities`, conjoined. This is the refinement
    guarantee at full strength for one step. -/
theorem GammaMergeStep.gammaWellConnectedCommunities (G : WeightedGraph n) (γ : ℝ)
    (p q : Partition n) (hp : GammaWellConnectedCommunities G γ p) (h : GammaMergeStep G γ p q) :
    GammaWellConnectedCommunities G γ q :=
  ⟨MergeStep.connectedCommunities G p q hp.1 h.mergeStep,
   GammaMergeStep.gammaDenseCommunities G γ p q hp.2 h⟩

/-- **γ-connectivity guarantee for a refinement run.** Any partition reachable by a
    sequence of γ-gated merges from a γ-well-connected partition is itself
    γ-well-connected — every community connected and internally γ-dense. This is the
    γ-connectivity guarantee for the gated refinement operator.

    Unlike the connectivity run (`connectedCommunities_of_mergeRun`, based at the
    singleton partition), the base here is a γ-dense partition rather than singletons:
    a lone node is not γ-dense, so the gate must bootstrap density, which it does by
    only ever merging across γ-dense cuts. The stronger "no sparse cut" reading of
    γ-connectivity is a convergence property and lands with subset-optimality (C3). -/
theorem gammaWellConnectedCommunities_of_gammaMergeRun (G : WeightedGraph n) (γ : ℝ)
    (p q : Partition n) (hp : GammaWellConnectedCommunities G γ p)
    (h : Relation.ReflTransGen (GammaMergeStep G γ) p q) :
    GammaWellConnectedCommunities G γ q := by
  induction h with
  | refl => exact hp
  | tail _ hstep ih => exact GammaMergeStep.gammaWellConnectedCommunities G γ _ _ ih hstep

end Meso
