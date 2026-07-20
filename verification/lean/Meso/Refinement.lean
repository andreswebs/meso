/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.Connectivity

/-!
# Refinement and the connectivity invariant

Leiden's refinement phase is what makes its communities connected (plan section
4.1): starting from singletons (each node alone, trivially connected), it merges
sub-communities, but only ones that are well-connected and sit inside the same
outer community. The well-connectedness gate and the randomized, gain-weighted
choice govern *quality* and the γ-guarantees; the *connectivity* of the result is
a simpler invariant, and that is what this file isolates.

The invariant: a merge only ever joins two sub-communities that share an edge, and
joining two connected pieces across an edge yields a connected piece. So starting
from singletons, connectivity is preserved through every merge. The graph-theoretic
core is `connected_induce_union_of_adj`; the operator side is `mergeCommunities` /
`MergeStep`, and `MergeStep.connectedCommunities` is the preservation theorem.
-/

namespace Meso

/-- **Two connected induced subgraphs sharing an edge form a connected induced
    subgraph.** The vertex-set union is connected: within each side reach the
    endpoint of the shared edge (transporting reachability up the inclusion
    `induceHomOfLE`), then cross the edge. This is the reason merging across an
    edge preserves connectivity. -/
lemma connected_induce_union_of_adj {V : Type*} (H : SimpleGraph V) (Sa Sb : Set V)
    (ca : (H.induce Sa).Connected) (cb : (H.induce Sb).Connected)
    {ia ib : V} (hia : ia ∈ Sa) (hib : ib ∈ Sb) (hadj : H.Adj ia ib) :
    (H.induce (Sa ∪ Sb)).Connected := by
  rw [SimpleGraph.connected_iff_exists_forall_reachable]
  refine ⟨⟨ia, Or.inl hia⟩, ?_⟩
  rintro ⟨w, hw⟩
  have hedge : (H.induce (Sa ∪ Sb)).Adj ⟨ia, Or.inl hia⟩ ⟨ib, Or.inr hib⟩ :=
    SimpleGraph.induce_adj.mpr hadj
  rcases hw with hwa | hwb
  · have hr : (H.induce Sa).Reachable ⟨ia, hia⟩ ⟨w, hwa⟩ := ca.preconnected _ _
    exact hr.map (H.induceHomOfLE Set.subset_union_left).toHom
  · have hr : (H.induce Sb).Reachable ⟨ib, hib⟩ ⟨w, hwb⟩ := cb.preconnected _ _
    exact hedge.reachable.trans (hr.map (H.induceHomOfLE Set.subset_union_right).toHom)

/-- Merge community `b` into community `a`: relabel every node of `b` to `a`. -/
def mergeCommunities (p : Partition n) (a b : ℕ) : Partition n :=
  fun i => if p i = b then a else p i

/-- The merged community `a` is exactly the union of the old communities `a` and
    `b`. -/
lemma merge_fiber_eq_union (p : Partition n) (a b : ℕ) :
    {i | mergeCommunities p a b i = a} = {i | p i = a} ∪ {i | p i = b} := by
  ext i
  simp only [mergeCommunities, Set.mem_setOf_eq, Set.mem_union]
  by_cases h : p i = b <;> simp [h]

/-- Communities other than `a` and `b` are unchanged by the merge. -/
lemma merge_fiber_eq_of_ne (p : Partition n) (a b c : ℕ) (hca : c ≠ a) (hcb : c ≠ b) :
    {i | mergeCommunities p a b i = c} = {i | p i = c} := by
  ext i
  simp only [mergeCommunities, Set.mem_setOf_eq]
  by_cases h : p i = b <;> simp [h, Ne.symm hca, Ne.symm hcb]

/-- After merging `b` into `a`, community `b` is empty. -/
lemma merge_not_mem_image (p : Partition n) (a b : ℕ) (hab : a ≠ b) :
    b ∉ Finset.univ.image (mergeCommunities p a b) := by
  simp only [Finset.mem_image, Finset.mem_univ, true_and, not_exists]
  intro i hi
  simp only [mergeCommunities] at hi
  by_cases h : p i = b
  · rw [if_pos h] at hi; exact hab hi
  · rw [if_neg h] at hi; exact h hi

/-- One refinement merge: join two distinct communities that share an edge. The
    shared edge is what the well-connectedness gate guarantees; here we keep only
    the part connectivity needs. -/
def MergeStep (G : WeightedGraph n) (p q : Partition n) : Prop :=
  ∃ a b, a ≠ b ∧ (∃ ia ib, p ia = a ∧ p ib = b ∧ (G.simpleGraph).Adj ia ib)
    ∧ q = mergeCommunities p a b

/-- **A merge preserves the connectivity guarantee.** If every community of `p` is
    connected and `q` merges two of them across a shared edge, every community of
    `q` is connected: the merged community is a union of two connected pieces
    joined by an edge (`connected_induce_union_of_adj`), and all others are
    unchanged. Starting from the singleton partition (`connectedCommunities_singleton`),
    this is the invariant that makes Leiden's refined communities connected. -/
theorem MergeStep.connectedCommunities (G : WeightedGraph n) (p q : Partition n)
    (hp : ConnectedCommunities G p) (h : MergeStep G p q) :
    ConnectedCommunities G q := by
  obtain ⟨a, b, hab, ⟨ia, ib, hpa, hpb, hadj⟩, rfl⟩ := h
  intro c hc
  by_cases hca : c = a
  · unfold CommunityConnected
    rw [hca, merge_fiber_eq_union p a b]
    exact connected_induce_union_of_adj (G.simpleGraph) {i | p i = a} {i | p i = b}
      (hp a (Finset.mem_image.mpr ⟨ia, Finset.mem_univ _, hpa⟩))
      (hp b (Finset.mem_image.mpr ⟨ib, Finset.mem_univ _, hpb⟩)) hpa hpb hadj
  · have hcb : c ≠ b := fun hcb' => merge_not_mem_image p a b hab (hcb' ▸ hc)
    unfold CommunityConnected
    rw [merge_fiber_eq_of_ne p a b c hca hcb]
    obtain ⟨i, -, hi⟩ := Finset.mem_image.mp hc
    have hpic : p i = c := by
      simp only [mergeCommunities] at hi
      by_cases h : p i = b
      · rw [if_pos h] at hi; exact absurd hi.symm hca
      · rw [if_neg h] at hi; exact hi
    exact hp c (Finset.mem_image.mpr ⟨i, Finset.mem_univ _, hpic⟩)

/-- **A refinement run yields connected communities.** Any partition reachable
    from the singleton partition by a sequence of edge-merges has every community
    connected. This is the connectivity guarantee for refinement modelled as
    edge-merges: `connectedCommunities_singleton` is the base and
    `MergeStep.connectedCommunities` the inductive step. The remaining gap to the
    literal Leiden guarantee is only that the refinement operator (with its
    well-connectedness gate) produces such a run; the gate governs quality and the
    γ-bounds, not connectivity. -/
theorem connectedCommunities_of_mergeRun (G : WeightedGraph n) (q : Partition n)
    (h : Relation.ReflTransGen (MergeStep G) (fun i => (i : ℕ)) q) :
    ConnectedCommunities G q := by
  induction h with
  | refl => exact connectedCommunities_singleton G
  | tail _ hstep ih => exact MergeStep.connectedCommunities G _ _ ih hstep

/-- **The refinement operator, one step.** Leiden refines each outer community
    independently: from singletons it merges two sub-communities that (i) lie
    inside a single outer community `o` and (ii) share an edge. The shared edge is
    the connectivity-relevant consequence of the well-connectedness gate; the
    gate's γ-density content — which governs quality and the γ-guarantees, not
    connectivity — is abstracted to that consequence here and revisited with CPM in
    the γ-guarantee tier. Compared with `MergeStep`, this adds only the
    outer-locality constraint, so it is a strictly more constrained merge and
    distinguishes refinement from generic merging. -/
def RefineStep (G : WeightedGraph n) (outer p q : Partition n) : Prop :=
  ∃ a b, a ≠ b
    ∧ (∃ o, (∀ i, p i = a → outer i = o) ∧ (∀ i, p i = b → outer i = o))
    ∧ (∃ ia ib, p ia = a ∧ p ib = b ∧ (G.simpleGraph).Adj ia ib)
    ∧ q = mergeCommunities p a b

/-- Every refinement step is in particular an edge-merge: drop the outer-locality
    witness and keep the shared edge. -/
theorem RefineStep.mergeStep {G : WeightedGraph n} {outer p q : Partition n}
    (h : RefineStep G outer p q) : MergeStep G p q := by
  obtain ⟨a, b, hab, _, hedge, hq⟩ := h
  exact ⟨a, b, hab, hedge, hq⟩

/-- **The refinement operator's output is a valid edge-merge run.** A run of the
    operator (a `RefineStep` sequence) is a fortiori a `MergeStep` run, lifting each
    step through `RefineStep.mergeStep`. This is the statement the connectivity
    guarantee row names. -/
theorem refineRun_isMergeRun {G : WeightedGraph n} {outer s q : Partition n}
    (h : Relation.ReflTransGen (RefineStep G outer) s q) :
    Relation.ReflTransGen (MergeStep G) s q :=
  Relation.ReflTransGen.mono (fun _ _ hstep => hstep.mergeStep) h

/-- **Connectivity guarantee, closed for the refinement operator.** Running the
    refinement operator from the singleton partition yields connected communities:
    its output is an edge-merge run from singletons (`refineRun_isMergeRun`), and
    every such run has connected communities (`connectedCommunities_of_mergeRun`).
    Together with the base and inductive steps this closes the connectivity invariant
    end to end for the operator, not merely for abstract edge-merge runs. -/
theorem connectedCommunities_of_refineRun {G : WeightedGraph n} {outer q : Partition n}
    (h : Relation.ReflTransGen (RefineStep G outer) (fun i => (i : ℕ)) q) :
    ConnectedCommunities G q :=
  connectedCommunities_of_mergeRun G q (refineRun_isMergeRun h)

end Meso
