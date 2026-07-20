/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.DirectedGraph
import Meso.Refinement

/-!
# Directed weak connectivity

The directed analogue of `Meso/Connectivity.lean` and `Meso/Refinement.lean`:
every community a directed refinement run returns is *weakly* connected. Per the
Phase 3 triage (`docs/research/directed-modularity-triage.md`, section 4), weak
connectivity is the connectivity notion that survives asymmetry. Strong
connectivity is refuted by design: a single-arc pair `{u, v}` with only `u → v`
is a legitimate community yet not strongly connected, so no strong-connectivity
guarantee is stated (triage section 5).

The transfer is mechanical, and the reason is structural. The undirected
connectivity proofs (`Connectivity.lean`, `Refinement.lean`) never touch
`weight_symm`: they reason only about the underlying `SimpleGraph`, partition
fibers, and `numComm`/image combinatorics. So all the partition-level operators
and the graph-theoretic core carry over verbatim:

* `mergeCommunities`, `merge_fiber_eq_union`, `merge_fiber_eq_of_ne`,
  `merge_not_mem_image` are partition-only and reused directly (imported).
* `connected_induce_union_of_adj` is stated over an abstract `SimpleGraph V` and
  reused directly.

Only the definitions that mention the graph are restated over
`DirectedWeightedGraph`. The single mathematical difference is the underlying
simple graph: `DirectedWeightedGraph.simpleGraph` uses `fromRel`, which
symmetrises the arc relation, so its edge set is exactly the weak-connectivity
edge set `(0 < w_ij ∨ 0 < w_ji) ∧ i ≠ j`. Textually the definition is identical
to the undirected `simpleGraph`; here the symmetrisation is what does the work
rather than being a no-op.
-/

namespace Meso

variable {n : ℕ}

/-- The underlying (undirected) simple graph of a directed graph `G`, for weak
    connectivity: an edge exactly where an arc runs in either direction. Built with
    `fromRel`, which symmetrises the arc relation `0 < weight i j` and drops
    self-loops, so `Adj i j ↔ (0 < w_ij ∨ 0 < w_ji) ∧ i ≠ j`. Textually identical
    to `WeightedGraph.simpleGraph`, but here the symmetrisation genuinely combines
    the two arc directions rather than acting as a no-op on a symmetric weight. -/
def DirectedWeightedGraph.simpleGraph (G : DirectedWeightedGraph n) : SimpleGraph (Fin n) :=
  SimpleGraph.fromRel (fun i j => 0 < G.weight i j)

/-- Community `c` of `p` induces a connected subgraph of `G.simpleGraph`: any two
    nodes both assigned to `c` are joined by a walk (through the symmetrised arc
    relation) that stays within `c`. Directed analogue of `CommunityConnected`. -/
def DirectedCommunityConnected (G : DirectedWeightedGraph n) (p : Partition n) (c : ℕ) : Prop :=
  ((G.simpleGraph).induce {i | p i = c}).Connected

/-- The weak-connectivity guarantee as a property of a partition: every occupied
    community induces a weakly connected subgraph. Directed refinement establishes
    this. Directed analogue of `ConnectedCommunities`. -/
def DirectedConnectedCommunities (G : DirectedWeightedGraph n) (p : Partition n) : Prop :=
  ∀ c ∈ Finset.univ.image p, DirectedCommunityConnected G p c

/-- A community containing at most one node is weakly connected: preconnected
    because its vertex set is a subsingleton, and nonempty by the witness. Directed
    analogue of `communityConnected_of_subsingleton` (the proof mentions only
    `simpleGraph` and fibers, so it ports verbatim). -/
lemma directedCommunityConnected_of_subsingleton (G : DirectedWeightedGraph n) (p : Partition n)
    (c : ℕ) (hne : ∃ i, p i = c) (hsub : ∀ i j : Fin n, p i = c → p j = c → i = j) :
    DirectedCommunityConnected G p c := by
  haveI : Nonempty ↥{i | p i = c} := ⟨⟨hne.choose, hne.choose_spec⟩⟩
  haveI : Subsingleton ↥{i | p i = c} :=
    ⟨fun a b => Subtype.ext (hsub a b a.2 b.2)⟩
  exact ⟨SimpleGraph.Preconnected.of_subsingleton⟩

/-- **Base case: the singleton partition has weakly connected communities.** Each
    community is a single node and a one-vertex subgraph is trivially connected.
    Directed analogue of `connectedCommunities_singleton`. -/
theorem directedConnectedCommunities_singleton (G : DirectedWeightedGraph n) :
    DirectedConnectedCommunities G (fun i => (i : ℕ)) := by
  intro c hc
  obtain ⟨i, -, hi⟩ := Finset.mem_image.mp hc
  exact directedCommunityConnected_of_subsingleton G _ c ⟨i, hi⟩
    (fun a b ha hb => Fin.val_injective (ha.trans hb.symm))

/-- One directed refinement merge: join two distinct communities sharing a
    (symmetrised, i.e. either-direction) arc. Directed analogue of `MergeStep`;
    reuses the partition-only `mergeCommunities`. -/
def DirectedMergeStep (G : DirectedWeightedGraph n) (p q : Partition n) : Prop :=
  ∃ a b, a ≠ b ∧ (∃ ia ib, p ia = a ∧ p ib = b ∧ (G.simpleGraph).Adj ia ib)
    ∧ q = mergeCommunities p a b

/-- **A directed merge preserves the weak-connectivity guarantee.** If every
    community of `p` is weakly connected and `q` merges two of them across a shared
    (either-direction) arc, every community of `q` is weakly connected: the merged
    community is a union of two connected pieces joined by an edge
    (`connected_induce_union_of_adj`, reused directly), and all others are
    unchanged. Directed analogue of `MergeStep.connectedCommunities`; the proof
    ports verbatim because it uses only `simpleGraph` and fiber combinatorics. -/
theorem DirectedMergeStep.connectedCommunities (G : DirectedWeightedGraph n) (p q : Partition n)
    (hp : DirectedConnectedCommunities G p) (h : DirectedMergeStep G p q) :
    DirectedConnectedCommunities G q := by
  obtain ⟨a, b, hab, ⟨ia, ib, hpa, hpb, hadj⟩, rfl⟩ := h
  intro c hc
  by_cases hca : c = a
  · unfold DirectedCommunityConnected
    rw [hca, merge_fiber_eq_union p a b]
    exact connected_induce_union_of_adj (G.simpleGraph) {i | p i = a} {i | p i = b}
      (hp a (Finset.mem_image.mpr ⟨ia, Finset.mem_univ _, hpa⟩))
      (hp b (Finset.mem_image.mpr ⟨ib, Finset.mem_univ _, hpb⟩)) hpa hpb hadj
  · have hcb : c ≠ b := fun hcb' => merge_not_mem_image p a b hab (hcb' ▸ hc)
    unfold DirectedCommunityConnected
    rw [merge_fiber_eq_of_ne p a b c hca hcb]
    obtain ⟨i, -, hi⟩ := Finset.mem_image.mp hc
    have hpic : p i = c := by
      simp only [mergeCommunities] at hi
      by_cases h : p i = b
      · rw [if_pos h] at hi; exact absurd hi.symm hca
      · rw [if_neg h] at hi; exact hi
    exact hp c (Finset.mem_image.mpr ⟨i, Finset.mem_univ _, hpic⟩)

/-- **A directed refinement run yields weakly connected communities.** Any
    partition reachable from the singleton partition by a sequence of arc-merges has
    every community weakly connected. Directed analogue of
    `connectedCommunities_of_mergeRun`. -/
theorem directedConnectedCommunities_of_mergeRun (G : DirectedWeightedGraph n) (q : Partition n)
    (h : Relation.ReflTransGen (DirectedMergeStep G) (fun i => (i : ℕ)) q) :
    DirectedConnectedCommunities G q := by
  induction h with
  | refl => exact directedConnectedCommunities_singleton G
  | tail _ hstep ih => exact DirectedMergeStep.connectedCommunities G _ _ ih hstep

/-- **The directed refinement operator, one step.** From singletons, merge two
    sub-communities that (i) lie inside a single outer community `o` and (ii) share
    a (symmetrised) arc. Directed analogue of `RefineStep`; adds only the
    outer-locality constraint to `DirectedMergeStep`. -/
def DirectedRefineStep (G : DirectedWeightedGraph n) (outer p q : Partition n) : Prop :=
  ∃ a b, a ≠ b
    ∧ (∃ o, (∀ i, p i = a → outer i = o) ∧ (∀ i, p i = b → outer i = o))
    ∧ (∃ ia ib, p ia = a ∧ p ib = b ∧ (G.simpleGraph).Adj ia ib)
    ∧ q = mergeCommunities p a b

/-- Every directed refinement step is in particular an arc-merge: drop the
    outer-locality witness and keep the shared arc. Directed analogue of
    `RefineStep.mergeStep`. -/
theorem DirectedRefineStep.mergeStep {G : DirectedWeightedGraph n} {outer p q : Partition n}
    (h : DirectedRefineStep G outer p q) : DirectedMergeStep G p q := by
  obtain ⟨a, b, hab, _, hedge, hq⟩ := h
  exact ⟨a, b, hab, hedge, hq⟩

/-- **The directed refinement operator's output is a valid arc-merge run.** A run of
    the operator is a fortiori a `DirectedMergeStep` run. Directed analogue of
    `refineRun_isMergeRun`. -/
theorem directedRefineRun_isMergeRun {G : DirectedWeightedGraph n} {outer s q : Partition n}
    (h : Relation.ReflTransGen (DirectedRefineStep G outer) s q) :
    Relation.ReflTransGen (DirectedMergeStep G) s q :=
  Relation.ReflTransGen.mono (fun _ _ hstep => hstep.mergeStep) h

/-- **Weak-connectivity guarantee, closed for the directed refinement operator.**
    Running the directed refinement operator from the singleton partition yields
    weakly connected communities: its output is an arc-merge run from singletons
    (`directedRefineRun_isMergeRun`), and every such run has weakly connected
    communities (`directedConnectedCommunities_of_mergeRun`). The top theorem of
    Task 3; directed analogue of `connectedCommunities_of_refineRun`. -/
theorem directedConnectedCommunities_of_refineRun {G : DirectedWeightedGraph n}
    {outer q : Partition n}
    (h : Relation.ReflTransGen (DirectedRefineStep G outer) (fun i => (i : ℕ)) q) :
    DirectedConnectedCommunities G q :=
  directedConnectedCommunities_of_mergeRun G q (directedRefineRun_isMergeRun h)

end Meso
