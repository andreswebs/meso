/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.Quality

/-!
# The connectivity guarantee

Leiden's defining property over Louvain: every community it returns is a
*connected* subgraph (plan section 4.1). This file builds the machinery to state
that — the underlying simple graph of a `WeightedGraph`, and what it means for a
community to be connected — and proves the base case.

The underlying simple graph has an edge exactly where the weight is positive;
self-loops (which carry internal edge weight through aggregation) are not edges
for connectivity. A community is connected when the subgraph induced on its nodes
is connected in Mathlib's sense: any two of its members are joined by a walk that
stays inside the community.

The substance — that Leiden's refinement phase establishes this while Louvain does
not — needs the refinement operator, which is future work. What is proved here is
the framework plus the trivial end of the spectrum: the singleton partition, whose
communities are single nodes, is connected.
-/

namespace Meso

variable {n : ℕ}

/-- The underlying simple graph of `G`: an edge exactly where the weight is
    positive. Built with `fromRel`, which symmetrizes and drops self-loops; since
    `weight` is symmetric the symmetrization is a no-op, and self-loops (which
    carry internal weight) are correctly excluded from connectivity edges. -/
def WeightedGraph.simpleGraph (G : WeightedGraph n) : SimpleGraph (Fin n) :=
  SimpleGraph.fromRel (fun i j => 0 < G.weight i j)

/-- Community `c` of `p` induces a connected subgraph of `G`: any two nodes both
    assigned to `c` are joined by a walk that stays within `c`. -/
def CommunityConnected (G : WeightedGraph n) (p : Partition n) (c : ℕ) : Prop :=
  ((G.simpleGraph).induce {i | p i = c}).Connected

/-- The connectivity guarantee as a property of a partition: every occupied
    community is a connected subgraph. Leiden's refinement establishes this;
    Louvain does not. -/
def ConnectedCommunities (G : WeightedGraph n) (p : Partition n) : Prop :=
  ∀ c ∈ Finset.univ.image p, CommunityConnected G p c

/-- A community containing at most one node is connected: preconnected because its
    vertex set is a subsingleton, and nonempty by the witness. -/
lemma communityConnected_of_subsingleton (G : WeightedGraph n) (p : Partition n) (c : ℕ)
    (hne : ∃ i, p i = c) (hsub : ∀ i j : Fin n, p i = c → p j = c → i = j) :
    CommunityConnected G p c := by
  haveI : Nonempty ↥{i | p i = c} := ⟨⟨hne.choose, hne.choose_spec⟩⟩
  haveI : Subsingleton ↥{i | p i = c} :=
    ⟨fun a b => Subtype.ext (hsub a b a.2 b.2)⟩
  exact ⟨SimpleGraph.Preconnected.of_subsingleton⟩

/-- **Base case: the singleton partition has connected communities.** With every
    node its own community (`p = Fin.val`), each community is a single node and a
    one-vertex subgraph is trivially connected. The real content is future work:
    Leiden's refinement preserves connectivity while merging singletons upward. -/
theorem connectedCommunities_singleton (G : WeightedGraph n) :
    ConnectedCommunities G (fun i => (i : ℕ)) := by
  intro c hc
  obtain ⟨i, -, hi⟩ := Finset.mem_image.mp hc
  exact communityConnected_of_subsingleton G _ c ⟨i, hi⟩
    (fun a b ha hb => Fin.val_injective (ha.trans hb.symm))

end Meso
