/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.Compute
import Meso.Separation
import Meso.GammaConnectivity
import Meso.SubsetOptimality
import Meso.Connectivity
import Meso.Reachability

/-!
# Computable Bool mirrors of the guarantee predicates

The value-oracle gets its numbers from computable rational mirrors of
`modularity` and `cpm`, proved equal to the real model (`Meso.Compute`). This file
does the same for the *predicates* the paper theorems establish — connectivity,
γ-separation, γ-density, and subset-optimality — so the value-oracle can emit
boolean golden vectors the Go guarantee tests check against a proved oracle.

Each guarantee predicate is `noncomputable` (defined over ℝ, or over Mathlib's
`SimpleGraph` reachability). The pattern here is F1/F2 for booleans:

1. A decidable rational predicate over `WeightedGraphQ`, matching the real one term
   for term (the F1 analogue). Decidability makes it a `Bool` via `decide`, which is
   what the executable emits.
2. A theorem that it holds iff the noncomputable real predicate holds on `G.toReal`
   (the F2 analogue), so the emitted boolean is provably the guarantee's truth value.

The bridge is that `ℚ → ℝ` preserves both order (`Rat.cast_le`, for the inequality
predicates) and positivity (`Rat.cast_pos`, for the adjacency relation behind
connectivity), so the rational decision and the real predicate always agree.

Well-formedness is deliberately absent: a `Partition` is a total function by
construction, so "every node has a community" is vacuous in Lean; it is a Go-side
range check, not an oracle value.
-/

namespace Meso

variable {n : ℕ}

/-! ## Rational block aggregates

The rational mirrors of `communitySize`, `communityInternalWeight`, and
`blockWeight` (`Meso.CPM`, `Meso.Aggregate`), with the `toReal` bridge lemmas that
turn each real aggregate into the cast of its rational twin. These are the pieces
the γ-density, γ-separation, and subset-optimality mirrors are built from. -/

/-- Community `c`'s size over ℚ: the sum of its members' node sizes. Rational mirror
    of `communitySize`. -/
def communitySizeQ (G : WeightedGraphQ n) (p : Partition n) (c : ℕ) : ℚ :=
  ∑ i ∈ Finset.univ.filter (fun i => p i = c), G.nodeSize i

/-- Community `c`'s internal edge weight over ℚ: the block sum of `weight` over node
    pairs both assigned to `c`. Rational mirror of `communityInternalWeight`. -/
def communityInternalWeightQ (G : WeightedGraphQ n) (p : Partition n) (c : ℕ) : ℚ :=
  ∑ i ∈ Finset.univ.filter (fun i => p i = c),
    ∑ j ∈ Finset.univ.filter (fun j => p j = c), G.weight i j

/-- Block edge weight between communities `a` and `b` over ℚ. Rational mirror of
    `blockWeight`. -/
def blockWeightQ (G : WeightedGraphQ n) (p : Partition n) (a b : ℕ) : ℚ :=
  ∑ i ∈ Finset.univ.filter (fun i => p i = a),
    ∑ j ∈ Finset.univ.filter (fun j => p j = b), G.weight i j

@[simp] lemma toReal_communitySize (G : WeightedGraphQ n) (p : Partition n) (c : ℕ) :
    communitySize G.toReal p c = (communitySizeQ G p c : ℝ) := by
  simp only [communitySize, communitySizeQ, WeightedGraphQ.toReal_nodeSize, Rat.cast_sum]

@[simp] lemma toReal_communityInternalWeight (G : WeightedGraphQ n) (p : Partition n) (c : ℕ) :
    communityInternalWeight G.toReal p c = (communityInternalWeightQ G p c : ℝ) := by
  simp only [communityInternalWeight, communityInternalWeightQ, WeightedGraphQ.toReal_weight,
    Rat.cast_sum]

@[simp] lemma toReal_blockWeight (G : WeightedGraphQ n) (p : Partition n) (a b : ℕ) :
    blockWeight G.toReal p a b = (blockWeightQ G p a b : ℝ) := by
  simp only [blockWeight, blockWeightQ, WeightedGraphQ.toReal_weight, Rat.cast_sum]

/-! ## G1: connectivity

The connectivity guarantee Leiden adds over Louvain, and the hardest mirror: it
relates a decidable check back to Mathlib's `SimpleGraph` reachability rather than to
an arithmetic inequality. The rational graph carries the same edge relation as the
real one (`0 < q ↔ 0 < (q : ℝ)`, `Rat.cast_pos`), so `G.toReal.simpleGraph` and
`G.simpleGraph` are the *same* `SimpleGraph (Fin n)` (`toReal_simpleGraph`). The
rational graph's adjacency is decidable, and Mathlib decides `Connected` for a finite
graph with decidable adjacency, so the induced-community connectivity check is
decidable and the equivalence is that graph equality. -/

/-- The underlying simple graph of a rational graph: an edge exactly where the
    rational weight is positive, symmetrised with self-loops dropped. Computable
    counterpart of `WeightedGraph.simpleGraph`; its adjacency is decidable because
    `0 < q` is. -/
def WeightedGraphQ.simpleGraph (G : WeightedGraphQ n) : SimpleGraph (Fin n) :=
  SimpleGraph.fromRel (fun i j => 0 < G.weight i j)

instance (G : WeightedGraphQ n) : DecidableRel G.simpleGraph.Adj :=
  inferInstanceAs (DecidableRel (SimpleGraph.fromRel _).Adj)

/-- **Connectivity mirror (Bool-decidable), per community.** Community `c` induces a
    connected subgraph of the positive-weight rational graph. Decidable rational form
    of `CommunityConnected`. -/
def CommunityConnectedQ (G : WeightedGraphQ n) (p : Partition n) (c : ℕ) : Prop :=
  ((G.simpleGraph).induce {i | p i = c}).Connected

instance (G : WeightedGraphQ n) (p : Partition n) (c : ℕ) :
    Decidable (CommunityConnectedQ G p c) :=
  inferInstanceAs (Decidable (SimpleGraph.Connected _))

/-- **Connectivity mirror (Bool-decidable).** Every occupied community induces a
    connected subgraph. Decidable rational form of `ConnectedCommunities`. -/
def ConnectedCommunitiesQ (G : WeightedGraphQ n) (p : Partition n) : Prop :=
  ∀ c ∈ Finset.univ.image p, CommunityConnectedQ G p c

instance (G : WeightedGraphQ n) (p : Partition n) :
    Decidable (ConnectedCommunitiesQ G p) :=
  inferInstanceAs (Decidable (∀ _ ∈ _, _))

/-- The real and rational underlying simple graphs coincide: casting a weight into ℝ
    preserves positivity, so the two `fromRel` graphs have the same adjacency. This is
    the whole content of the connectivity equivalence. -/
theorem toReal_simpleGraph (G : WeightedGraphQ n) : G.toReal.simpleGraph = G.simpleGraph := by
  ext i j
  simp only [WeightedGraph.simpleGraph, WeightedGraphQ.simpleGraph, SimpleGraph.fromRel_adj,
    WeightedGraphQ.toReal_weight, Rat.cast_pos]

/-- The connectivity mirror decides `ConnectedCommunities` on the real graph. Both
    sides induce over the identical graph (`toReal_simpleGraph`), so the predicates are
    the same. -/
theorem connectedCommunitiesQ_iff (G : WeightedGraphQ n) (p : Partition n) :
    ConnectedCommunitiesQ G p ↔ ConnectedCommunities G.toReal p := by
  unfold ConnectedCommunitiesQ ConnectedCommunities CommunityConnectedQ CommunityConnected
  rw [toReal_simpleGraph]

/-! ### G1: the efficient connectivity decider

`ConnectedCommunitiesQ` is proved correct but its `Decidable` instance is Mathlib's
walk enumeration, which is exponential and hangs past ~10 nodes. The emitted flag uses
`connectedCommunitiesFast` instead: a breadth-first reachable-set closure
(`Meso.Reachability`) that runs in polynomial time and is proved to decide the same
predicate. The within-community adjacency `radjQ` keeps walks inside the community, so
its `ReflTransGen` matches reachability in the induced subgraph
(`reflTransGen_radjQ_iff_reachable`). -/

/-- Within-community adjacency on `Fin n`: an edge of the positive-weight graph whose
    both endpoints lie in community `c`. Its reflexive-transitive closure is reachability
    inside the induced community subgraph. -/
def radjQ (G : WeightedGraphQ n) (p : Partition n) (c : ℕ) (i j : Fin n) : Prop :=
  p i = c ∧ p j = c ∧ G.simpleGraph.Adj i j

instance (G : WeightedGraphQ n) (p : Partition n) (c : ℕ) : DecidableRel (radjQ G p c) :=
  fun i j => inferInstanceAs (Decidable (p i = c ∧ p j = c ∧ G.simpleGraph.Adj i j))

/-- `radjQ`-reachability on `Fin n` is exactly reachability in the induced community
    subgraph: forward, each `radjQ` step carries both endpoints' membership and an
    induced edge; backward, an induced walk lifts along `Subtype.val`. -/
theorem reflTransGen_radjQ_iff_reachable (G : WeightedGraphQ n) (p : Partition n) (c : ℕ)
    {u v : Fin n} (hu : p u = c) (hv : p v = c) :
    Relation.ReflTransGen (radjQ G p c) u v
      ↔ (G.simpleGraph.induce {i | p i = c}).Reachable ⟨u, hu⟩ ⟨v, hv⟩ := by
  have fwd : ∀ {a b : Fin n}, Relation.ReflTransGen (radjQ G p c) a b →
      ∀ (ha : p a = c) (hb : p b = c),
        (G.simpleGraph.induce {i | p i = c}).Reachable ⟨a, ha⟩ ⟨b, hb⟩ := by
    intro a b h
    induction h with
    | refl => intro ha hb; exact SimpleGraph.Reachable.refl _
    | @tail x y _ hxy ih =>
      intro ha hy
      have hEdge : (G.simpleGraph.induce {i | p i = c}).Adj ⟨x, hxy.1⟩ ⟨y, hy⟩ :=
        SimpleGraph.induce_adj.mpr hxy.2.2
      exact (ih ha hxy.1).trans hEdge.reachable
  constructor
  · intro h; exact fwd h hu hv
  · intro h
    rw [SimpleGraph.reachable_eq_reflTransGen] at h
    exact Relation.ReflTransGen.lift (Subtype.val)
      (fun a b hab => ⟨a.2, b.2, SimpleGraph.induce_adj.mp hab⟩) h

/-- **Efficient per-community connectivity (Bool).** Whether the community's fiber is
    contained in the reachable-set closure from a representative node. Polynomial where
    `CommunityConnectedQ`'s decision procedure is exponential. -/
def communityConnectedFast (G : WeightedGraphQ n) (p : Partition n) (c : ℕ) : Bool :=
  if h : (Finset.univ.filter (fun i => p i = c)).Nonempty then
    decide ((Finset.univ.filter (fun i => p i = c))
      ⊆ reachableFinset (radjQ G p c) ((Finset.univ.filter (fun i => p i = c)).min' h))
  else false

/-- The efficient per-community decider agrees with `CommunityConnectedQ` on an occupied
    community: the fiber is reachable-closed from any representative iff the induced
    subgraph is connected. -/
theorem communityConnectedFast_iff (G : WeightedGraphQ n) (p : Partition n) {c : ℕ}
    (hc : c ∈ Finset.univ.image p) :
    communityConnectedFast G p c = true ↔ CommunityConnectedQ G p c := by
  have hF : (Finset.univ.filter (fun i => p i = c)).Nonempty := by
    obtain ⟨i, -, hi⟩ := Finset.mem_image.mp hc
    exact ⟨i, Finset.mem_filter.mpr ⟨Finset.mem_univ _, hi⟩⟩
  rw [communityConnectedFast, dif_pos hF, decide_eq_true_iff]
  set r := (Finset.univ.filter (fun i => p i = c)).min' hF with hr
  have hrc : p r = c :=
    (Finset.mem_filter.mp ((Finset.univ.filter (fun i => p i = c)).min'_mem hF)).2
  rw [CommunityConnectedQ]
  constructor
  · intro hsub
    rw [SimpleGraph.connected_iff_exists_forall_reachable]
    refine ⟨⟨r, hrc⟩, ?_⟩
    rintro ⟨w, hw⟩
    have hwF : w ∈ Finset.univ.filter (fun i => p i = c) :=
      Finset.mem_filter.mpr ⟨Finset.mem_univ _, hw⟩
    have hmem := mem_reachableFinset_iff (radjQ G p c) |>.mp (hsub hwF)
    exact (reflTransGen_radjQ_iff_reachable G p c hrc hw).mp hmem
  · intro hconn x hxF
    have hx : p x = c := (Finset.mem_filter.mp hxF).2
    have hreach := hconn.preconnected ⟨r, hrc⟩ ⟨x, hx⟩
    rw [← reflTransGen_radjQ_iff_reachable G p c hrc hx] at hreach
    exact (mem_reachableFinset_iff (radjQ G p c)).mpr hreach

/-- **Efficient connectivity mirror (Bool-decidable).** Every occupied community is
    connected, decided by the polynomial reachable-set closure per community. This is
    the flag the value-oracle emits; unlike `ConnectedCommunitiesQ` its decision runs at
    corpus scale. -/
def ConnectedCommunitiesFast (G : WeightedGraphQ n) (p : Partition n) : Prop :=
  ∀ c ∈ Finset.univ.image p, communityConnectedFast G p c = true

instance (G : WeightedGraphQ n) (p : Partition n) :
    Decidable (ConnectedCommunitiesFast G p) :=
  inferInstanceAs (Decidable (∀ _ ∈ _, _))

/-- The efficient connectivity mirror decides `ConnectedCommunities` on the real graph;
    it composes the per-community agreement with `connectedCommunitiesQ_iff`. -/
theorem connectedCommunitiesFast_iff (G : WeightedGraphQ n) (p : Partition n) :
    ConnectedCommunitiesFast G p ↔ ConnectedCommunities G.toReal p := by
  rw [← connectedCommunitiesQ_iff]
  unfold ConnectedCommunitiesFast ConnectedCommunitiesQ
  exact forall_congr' fun c => imp_congr_right fun hc => communityConnectedFast_iff G p hc

/-! ## G2: γ-density and γ-separation

Two inequality predicates over the block aggregates, so both mirrors are finite
loops over community labels with a decidable ℚ comparison at the leaf. The
equivalences push a single `Rat.cast_le` through the bridge lemmas. -/

/-- **γ-density mirror (Bool-decidable).** Every occupied community's rational CPM
    contribution is nonnegative: `γ S_c² ≤ e_c`. Decidable rational form of
    `GammaDenseCommunities`. -/
def GammaDenseCommunitiesQ (G : WeightedGraphQ n) (γ : ℚ) (p : Partition n) : Prop :=
  ∀ c ∈ Finset.univ.image p, γ * communitySizeQ G p c ^ 2 ≤ communityInternalWeightQ G p c

instance (G : WeightedGraphQ n) (γ : ℚ) (p : Partition n) :
    Decidable (GammaDenseCommunitiesQ G γ p) :=
  inferInstanceAs (Decidable (∀ _ ∈ _, _))

/-- The γ-density mirror decides `GammaDenseCommunities` on the real graph. -/
theorem gammaDenseCommunitiesQ_iff (G : WeightedGraphQ n) (γ : ℚ) (p : Partition n) :
    GammaDenseCommunitiesQ G γ p ↔ GammaDenseCommunities G.toReal (γ : ℝ) p := by
  unfold GammaDenseCommunitiesQ GammaDenseCommunities IsGammaDense
  refine forall_congr' fun c => imp_congr_right fun _ => ?_
  rw [toReal_communitySize, toReal_communityInternalWeight, ← Rat.cast_pow, ← Rat.cast_mul,
    Rat.cast_le]

/-- **γ-separation mirror (Bool-decidable).** Any two distinct occupied communities
    are weakly connected relative to γ: `e(C,D) ≤ γ S_C S_D`. Decidable rational form
    of the paper's γ-separation (`gammaSeparated_blockWeight_of_converged`'s
    conclusion). -/
def GammaSeparatedCommunitiesQ (G : WeightedGraphQ n) (γ : ℚ) (p : Partition n) : Prop :=
  ∀ C ∈ Finset.univ.image p, ∀ D ∈ Finset.univ.image p, C ≠ D →
    blockWeightQ G p C D ≤ γ * communitySizeQ G p C * communitySizeQ G p D

instance (G : WeightedGraphQ n) (γ : ℚ) (p : Partition n) :
    Decidable (GammaSeparatedCommunitiesQ G γ p) :=
  inferInstanceAs (Decidable (∀ _ ∈ _, _))

/-- **γ-separation as a real partition predicate.** Any two distinct occupied
    communities satisfy `e(C,D) ≤ γ S_C S_D`; the partition-predicate form of the
    conclusion `gammaSeparated_blockWeight_of_converged` proves for a converged
    partition. -/
def GammaSeparatedCommunities (G : WeightedGraph n) (γ : ℝ) (p : Partition n) : Prop :=
  ∀ C ∈ Finset.univ.image p, ∀ D ∈ Finset.univ.image p, C ≠ D →
    blockWeight G p C D ≤ γ * communitySize G p C * communitySize G p D

/-- The γ-separation mirror decides `GammaSeparatedCommunities` on the real graph. -/
theorem gammaSeparatedCommunitiesQ_iff (G : WeightedGraphQ n) (γ : ℚ) (p : Partition n) :
    GammaSeparatedCommunitiesQ G γ p ↔ GammaSeparatedCommunities G.toReal (γ : ℝ) p := by
  unfold GammaSeparatedCommunitiesQ GammaSeparatedCommunities
  refine forall_congr' fun C => imp_congr_right fun _ => forall_congr' fun D =>
    imp_congr_right fun _ => imp_congr_right fun _ => ?_
  rw [toReal_blockWeight, toReal_communitySize, toReal_communitySize, ← Rat.cast_mul,
    ← Rat.cast_mul, Rat.cast_le]

/-! ## G3: subset-optimality

The no-sparse-cut bound for every subset of every community: `e(S, C\S) ≥ γ ‖S‖ ‖C\S‖`.
The mirror enumerates each community's subsets (via `Finset.powerset` of its fiber),
so the decision is exponential in community size — fine for the small fixtures, out of
reach for a large corpus graph. -/

/-- **Subset-optimality mirror (Bool-decidable).** For every occupied community `C`
    and every subset `S` of its fiber, `γ ‖S‖ ‖C\S‖ ≤ e(S, C\S)`. Decidable rational
    form of `IsSubsetOptimal`; the outer `S` ranges over `Finset.powerset`, so the
    decision enumerates subsets. -/
def SubsetOptimalQ (G : WeightedGraphQ n) (γ : ℚ) (p : Partition n) : Prop :=
  ∀ C ∈ Finset.univ.image p,
    ∀ S ∈ (Finset.univ.filter (fun i => p i = C)).powerset,
      γ * (∑ i ∈ S, G.nodeSize i)
          * (∑ j ∈ Finset.univ.filter (fun j => p j = C ∧ j ∉ S), G.nodeSize j)
        ≤ ∑ i ∈ S, ∑ j ∈ Finset.univ.filter (fun j => p j = C ∧ j ∉ S), G.weight i j

instance (G : WeightedGraphQ n) (γ : ℚ) (p : Partition n) :
    Decidable (SubsetOptimalQ G γ p) :=
  inferInstanceAs (Decidable (∀ _ ∈ _, _))

/-- The subset-optimality mirror decides `IsSubsetOptimal` on the real graph. The
    mirror ranges `C` over occupied labels and `S` over each fiber's powerset; the
    real predicate ranges over all `C : ℕ` and all `S`, but the two agree: a subset
    contained in some community forces that community into the image, and the empty
    subset makes the bound `0 ≤ 0`. -/
theorem subsetOptimalQ_iff (G : WeightedGraphQ n) (γ : ℚ) (p : Partition n) :
    SubsetOptimalQ G γ p ↔ IsSubsetOptimal G.toReal (γ : ℝ) p := by
  constructor
  · intro hQ C S hSC
    by_cases hS : S.Nonempty
    · obtain ⟨i0, hi0⟩ := hS
      have hCimg : C ∈ Finset.univ.image p :=
        Finset.mem_image.mpr ⟨i0, Finset.mem_univ _, hSC i0 hi0⟩
      have hSpow : S ∈ (Finset.univ.filter (fun i => p i = C)).powerset := by
        rw [Finset.mem_powerset]
        intro i hi
        exact Finset.mem_filter.mpr ⟨Finset.mem_univ _, hSC i hi⟩
      have h := hQ C hCimg S hSpow
      simp only [WeightedGraphQ.toReal_nodeSize, WeightedGraphQ.toReal_weight]
      exact_mod_cast h
    · rw [Finset.not_nonempty_iff_eq_empty] at hS
      subst hS
      simp
  · intro hOpt C hC S hSpow
    rw [Finset.mem_powerset] at hSpow
    have hSC : ∀ i ∈ S, p i = C := fun i hi => (Finset.mem_filter.mp (hSpow hi)).2
    have h := hOpt C S hSC
    simp only [WeightedGraphQ.toReal_nodeSize, WeightedGraphQ.toReal_weight] at h
    exact_mod_cast h

end Meso
