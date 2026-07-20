/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.DirectedCompute
import Meso.DirectedConnectivity
import Meso.DirectedSeparation
import Meso.DirectedSubsetOptimality
import Meso.Reachability

/-!
# Computable Bool mirrors of the directed guarantee predicates

The directed analogue of `Meso/Predicates.lean`, for the directed value-oracle
(Phase 5 of the research plan,
`docs/research/directed-modularity-formal-verification.md`). Each surviving
directed guarantee predicate — weak connectivity, directed γ-separation, and the
directed subset bound — gets a decidable rational form over
`DirectedWeightedGraphQ`, proved to hold iff the real predicate holds on
`G.toReal`, so the boolean the oracle emits is provably the guarantee's truth
value.

The mirrors target the Phase 4 real Props:

* weak connectivity: `DirectedConnectedCommunities`
  (`Meso/DirectedConnectivity.lean`), the conclusion of
  `directedConnectedCommunities_of_refineRun`;
* directed γ-separation: the "for all distinct occupied pairs" packaging of the
  bound `directedGammaSeparated_blockWeight_of_levelStable` establishes
  (`Meso/DirectedSeparation.lean`), defined here as
  `DirectedGammaSeparatedCommunities`;
* the directed subset bound: the "for all communities and subsets" packaging of
  the bound `directedSubsetGammaDense_of_subsetStable` establishes
  (`Meso/DirectedSubsetOptimality.lean`), defined here as
  `DirectedSubsetOptimal`. Per the Phase 3 triage this is a *characterization*:
  the algorithm does not always attain subset stability (the n=4 fixture, ticket
  `mes-niic`), so the emitted flag reads `false` at such outputs; the `_iff`
  holds regardless.

The γ-bounds keep the `/ totalWeight` division on both sides: ℚ and ℝ division
by zero are both `0`, so the predicates agree at `m = 0` too, and `Rat.cast_div`
closes the cast push. Connectivity reuses the graph-agnostic reachable-set
closure (`Meso.Reachability`) exactly as the undirected fast decider does; only
the adjacency changes, to the directed `simpleGraph`, whose `fromRel`
symmetrisation makes it the weak-connectivity edge set.
-/

namespace Meso

variable {n : ℕ}

/-! ## Rational directed block aggregates

The rational mirrors of `directedBlockWeight` (`Meso.DirectedAggregate`) and
`commOutDegree`/`commInDegree` (`Meso.DirectedMove`), with the `toReal` bridge
lemmas the γ-separation and subset mirrors are built from. -/

/-- Ordered block arc weight from community `a` into community `b`, over ℚ.
    Rational mirror of `directedBlockWeight`. -/
def directedBlockWeightQ (G : DirectedWeightedGraphQ n) (p : Partition n) (a b : ℕ) : ℚ :=
  ∑ i ∈ Finset.univ.filter (fun i => p i = a),
    ∑ j ∈ Finset.univ.filter (fun j => p j = b), G.weight i j

/-- Summed out-degree of community `c`, over ℚ. Rational mirror of
    `DirectedWeightedGraph.commOutDegree`. -/
def commOutDegreeQ (G : DirectedWeightedGraphQ n) (p : Partition n) (c : ℕ) : ℚ :=
  ∑ i ∈ Finset.univ.filter (fun i => p i = c), G.outDegree i

/-- Summed in-degree of community `c`, over ℚ. Rational mirror of
    `DirectedWeightedGraph.commInDegree`. -/
def commInDegreeQ (G : DirectedWeightedGraphQ n) (p : Partition n) (c : ℕ) : ℚ :=
  ∑ i ∈ Finset.univ.filter (fun i => p i = c), G.inDegree i

@[simp] lemma toReal_directedBlockWeight (G : DirectedWeightedGraphQ n) (p : Partition n)
    (a b : ℕ) : directedBlockWeight G.toReal p a b = (directedBlockWeightQ G p a b : ℝ) := by
  simp only [directedBlockWeight, directedBlockWeightQ, DirectedWeightedGraphQ.toReal_weight,
    Rat.cast_sum]

@[simp] lemma toReal_commOutDegree (G : DirectedWeightedGraphQ n) (p : Partition n) (c : ℕ) :
    G.toReal.commOutDegree p c = (commOutDegreeQ G p c : ℝ) := by
  simp only [DirectedWeightedGraph.commOutDegree, commOutDegreeQ,
    DirectedWeightedGraphQ.toReal_outDegree, Rat.cast_sum]

@[simp] lemma toReal_commInDegree (G : DirectedWeightedGraphQ n) (p : Partition n) (c : ℕ) :
    G.toReal.commInDegree p c = (commInDegreeQ G p c : ℝ) := by
  simp only [DirectedWeightedGraph.commInDegree, commInDegreeQ,
    DirectedWeightedGraphQ.toReal_inDegree, Rat.cast_sum]

/-! ## G1: weak connectivity

The mirror of the weak-connectivity guarantee
(`directedConnectedCommunities_of_refineRun`). The rational directed graph
carries the same underlying simple graph as the real one — `fromRel` over
`0 < weight`, which symmetrises the arc relation into the weak-connectivity edge
set — so the two induced-community connectivity predicates are literally the
same, and the efficient reachable-set decider ports from the undirected
`communityConnectedFast` with only the adjacency changed. -/

/-- The underlying (undirected) simple graph of a rational directed graph: an edge
    exactly where an arc runs in either direction, self-loops dropped. Computable
    counterpart of `DirectedWeightedGraph.simpleGraph`; its adjacency is decidable
    because `0 < q` is. -/
def DirectedWeightedGraphQ.simpleGraph (G : DirectedWeightedGraphQ n) : SimpleGraph (Fin n) :=
  SimpleGraph.fromRel (fun i j => 0 < G.weight i j)

instance instDecidableRelDirectedSimpleGraphAdj (G : DirectedWeightedGraphQ n) :
    DecidableRel G.simpleGraph.Adj :=
  inferInstanceAs (DecidableRel (SimpleGraph.fromRel _).Adj)

/-- **Weak-connectivity mirror (Bool-decidable), per community.** Community `c`
    induces a connected subgraph of the symmetrised positive-arc rational graph.
    Decidable rational form of `DirectedCommunityConnected`. -/
def DirectedCommunityConnectedQ (G : DirectedWeightedGraphQ n) (p : Partition n) (c : ℕ) :
    Prop :=
  ((G.simpleGraph).induce {i | p i = c}).Connected

instance (G : DirectedWeightedGraphQ n) (p : Partition n) (c : ℕ) :
    Decidable (DirectedCommunityConnectedQ G p c) :=
  inferInstanceAs (Decidable (SimpleGraph.Connected _))

/-- **Weak-connectivity mirror (Bool-decidable).** Every occupied community induces
    a weakly connected subgraph. Decidable rational form of
    `DirectedConnectedCommunities`. -/
def DirectedConnectedCommunitiesQ (G : DirectedWeightedGraphQ n) (p : Partition n) : Prop :=
  ∀ c ∈ Finset.univ.image p, DirectedCommunityConnectedQ G p c

instance (G : DirectedWeightedGraphQ n) (p : Partition n) :
    Decidable (DirectedConnectedCommunitiesQ G p) :=
  inferInstanceAs (Decidable (∀ _ ∈ _, _))

/-- The real and rational underlying simple graphs coincide: casting a weight into
    ℝ preserves positivity, so the two `fromRel` graphs have the same adjacency.
    The directed twin of `toReal_simpleGraph`. -/
theorem toReal_directedSimpleGraph (G : DirectedWeightedGraphQ n) :
    G.toReal.simpleGraph = G.simpleGraph := by
  ext i j
  simp only [DirectedWeightedGraph.simpleGraph, DirectedWeightedGraphQ.simpleGraph,
    SimpleGraph.fromRel_adj, DirectedWeightedGraphQ.toReal_weight, Rat.cast_pos]

/-- The weak-connectivity mirror decides `DirectedConnectedCommunities` on the real
    graph. Both sides induce over the identical graph (`toReal_directedSimpleGraph`),
    so the predicates are the same. -/
theorem directedConnectedCommunitiesQ_iff (G : DirectedWeightedGraphQ n) (p : Partition n) :
    DirectedConnectedCommunitiesQ G p ↔ DirectedConnectedCommunities G.toReal p := by
  unfold DirectedConnectedCommunitiesQ DirectedConnectedCommunities
    DirectedCommunityConnectedQ DirectedCommunityConnected
  rw [toReal_directedSimpleGraph]

/-! ### G1: the efficient connectivity decider

As undirected, `DirectedConnectedCommunitiesQ`'s `Decidable` instance is Mathlib's
exponential walk enumeration. The emitted flag uses the polynomial reachable-set
closure (`Meso.Reachability`, reused unchanged) over the within-community
adjacency `radjDirQ` instead, mirroring `communityConnectedFast` line for line. -/

/-- Within-community adjacency on `Fin n`: an edge of the symmetrised positive-arc
    graph whose both endpoints lie in community `c`. Its reflexive-transitive
    closure is reachability inside the induced community subgraph. Directed twin of
    `radjQ`. -/
def radjDirQ (G : DirectedWeightedGraphQ n) (p : Partition n) (c : ℕ) (i j : Fin n) : Prop :=
  p i = c ∧ p j = c ∧ G.simpleGraph.Adj i j

instance (G : DirectedWeightedGraphQ n) (p : Partition n) (c : ℕ) :
    DecidableRel (radjDirQ G p c) :=
  fun i j => inferInstanceAs (Decidable (p i = c ∧ p j = c ∧ G.simpleGraph.Adj i j))

/-- `radjDirQ`-reachability on `Fin n` is exactly reachability in the induced
    community subgraph. Directed twin of `reflTransGen_radjQ_iff_reachable`; the
    proof is identical, since it touches only the abstract adjacency. -/
theorem reflTransGen_radjDirQ_iff_reachable (G : DirectedWeightedGraphQ n) (p : Partition n)
    (c : ℕ) {u v : Fin n} (hu : p u = c) (hv : p v = c) :
    Relation.ReflTransGen (radjDirQ G p c) u v
      ↔ (G.simpleGraph.induce {i | p i = c}).Reachable ⟨u, hu⟩ ⟨v, hv⟩ := by
  have fwd : ∀ {a b : Fin n}, Relation.ReflTransGen (radjDirQ G p c) a b →
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

/-- **Efficient per-community weak connectivity (Bool).** Whether the community's
    fiber is contained in the reachable-set closure from a representative node.
    Directed twin of `communityConnectedFast`. -/
def directedCommunityConnectedFast (G : DirectedWeightedGraphQ n) (p : Partition n)
    (c : ℕ) : Bool :=
  if h : (Finset.univ.filter (fun i => p i = c)).Nonempty then
    decide ((Finset.univ.filter (fun i => p i = c))
      ⊆ reachableFinset (radjDirQ G p c) ((Finset.univ.filter (fun i => p i = c)).min' h))
  else false

/-- The efficient per-community decider agrees with `DirectedCommunityConnectedQ`
    on an occupied community. Directed twin of `communityConnectedFast_iff`. -/
theorem directedCommunityConnectedFast_iff (G : DirectedWeightedGraphQ n) (p : Partition n)
    {c : ℕ} (hc : c ∈ Finset.univ.image p) :
    directedCommunityConnectedFast G p c = true ↔ DirectedCommunityConnectedQ G p c := by
  have hF : (Finset.univ.filter (fun i => p i = c)).Nonempty := by
    obtain ⟨i, -, hi⟩ := Finset.mem_image.mp hc
    exact ⟨i, Finset.mem_filter.mpr ⟨Finset.mem_univ _, hi⟩⟩
  rw [directedCommunityConnectedFast, dif_pos hF, decide_eq_true_iff]
  set r := (Finset.univ.filter (fun i => p i = c)).min' hF with hr
  have hrc : p r = c :=
    (Finset.mem_filter.mp ((Finset.univ.filter (fun i => p i = c)).min'_mem hF)).2
  rw [DirectedCommunityConnectedQ]
  constructor
  · intro hsub
    rw [SimpleGraph.connected_iff_exists_forall_reachable]
    refine ⟨⟨r, hrc⟩, ?_⟩
    rintro ⟨w, hw⟩
    have hwF : w ∈ Finset.univ.filter (fun i => p i = c) :=
      Finset.mem_filter.mpr ⟨Finset.mem_univ _, hw⟩
    have hmem := mem_reachableFinset_iff (radjDirQ G p c) |>.mp (hsub hwF)
    exact (reflTransGen_radjDirQ_iff_reachable G p c hrc hw).mp hmem
  · intro hconn x hxF
    have hx : p x = c := (Finset.mem_filter.mp hxF).2
    have hreach := hconn.preconnected ⟨r, hrc⟩ ⟨x, hx⟩
    rw [← reflTransGen_radjDirQ_iff_reachable G p c hrc hx] at hreach
    exact (mem_reachableFinset_iff (radjDirQ G p c)).mpr hreach

/-- **Efficient weak-connectivity mirror (Bool-decidable).** Every occupied
    community is weakly connected, decided by the polynomial reachable-set closure
    per community. This is the flag the directed value-oracle emits. Directed twin
    of `ConnectedCommunitiesFast`. -/
def DirectedConnectedCommunitiesFast (G : DirectedWeightedGraphQ n) (p : Partition n) :
    Prop :=
  ∀ c ∈ Finset.univ.image p, directedCommunityConnectedFast G p c = true

instance (G : DirectedWeightedGraphQ n) (p : Partition n) :
    Decidable (DirectedConnectedCommunitiesFast G p) :=
  inferInstanceAs (Decidable (∀ _ ∈ _, _))

/-- The efficient weak-connectivity mirror decides `DirectedConnectedCommunities`
    on the real graph; it composes the per-community agreement with
    `directedConnectedCommunitiesQ_iff`. Directed twin of
    `connectedCommunitiesFast_iff`. -/
theorem directedConnectedCommunitiesFast_iff (G : DirectedWeightedGraphQ n)
    (p : Partition n) :
    DirectedConnectedCommunitiesFast G p ↔ DirectedConnectedCommunities G.toReal p := by
  rw [← directedConnectedCommunitiesQ_iff]
  unfold DirectedConnectedCommunitiesFast DirectedConnectedCommunitiesQ
  exact forall_congr' fun c => imp_congr_right fun hc =>
    directedCommunityConnectedFast_iff G p hc

/-! ## G2: directed γ-separation

The bidirectional block bound of `directedGammaSeparated_blockWeight_of_levelStable`,
packaged over all distinct occupied pairs. Both sides keep the `/ totalWeight`
division (`0` at `m = 0` in both ℚ and ℝ), so the equivalence is a `Rat.cast`
push through the block-aggregate bridges. -/

/-- **Directed γ-separation as a real partition predicate.** Any two distinct
    occupied communities satisfy the triage's directed separation bound
    `e(C,D) + e(D,C) ≤ γ (Kout_C·Kin_D + Kout_D·Kin_C) / m`; the partition-predicate
    form of the conclusion `directedGammaSeparated_blockWeight_of_levelStable`
    proves for a directed level-stable partition. -/
def DirectedGammaSeparatedCommunities (G : DirectedWeightedGraph n) (γ : ℝ)
    (p : Partition n) : Prop :=
  ∀ C ∈ Finset.univ.image p, ∀ D ∈ Finset.univ.image p, C ≠ D →
    directedBlockWeight G p C D + directedBlockWeight G p D C
      ≤ γ * (G.commOutDegree p C * G.commInDegree p D
             + G.commOutDegree p D * G.commInDegree p C) / G.totalWeight

/-- **Directed γ-separation mirror (Bool-decidable).** The same bound over ℚ.
    Decidable rational form of `DirectedGammaSeparatedCommunities`. -/
def DirectedGammaSeparatedCommunitiesQ (G : DirectedWeightedGraphQ n) (γ : ℚ)
    (p : Partition n) : Prop :=
  ∀ C ∈ Finset.univ.image p, ∀ D ∈ Finset.univ.image p, C ≠ D →
    directedBlockWeightQ G p C D + directedBlockWeightQ G p D C
      ≤ γ * (commOutDegreeQ G p C * commInDegreeQ G p D
             + commOutDegreeQ G p D * commInDegreeQ G p C) / G.totalWeight

instance (G : DirectedWeightedGraphQ n) (γ : ℚ) (p : Partition n) :
    Decidable (DirectedGammaSeparatedCommunitiesQ G γ p) :=
  inferInstanceAs (Decidable (∀ _ ∈ _, _))

/-- The directed γ-separation mirror decides `DirectedGammaSeparatedCommunities`
    on the real graph. -/
theorem directedGammaSeparatedCommunitiesQ_iff (G : DirectedWeightedGraphQ n) (γ : ℚ)
    (p : Partition n) :
    DirectedGammaSeparatedCommunitiesQ G γ p
      ↔ DirectedGammaSeparatedCommunities G.toReal (γ : ℝ) p := by
  unfold DirectedGammaSeparatedCommunitiesQ DirectedGammaSeparatedCommunities
  refine forall_congr' fun C => imp_congr_right fun _ => forall_congr' fun D =>
    imp_congr_right fun _ => imp_congr_right fun _ => ?_
  rw [toReal_directedBlockWeight, toReal_directedBlockWeight, toReal_commOutDegree,
    toReal_commOutDegree, toReal_commInDegree, toReal_commInDegree,
    DirectedWeightedGraphQ.toReal_totalWeight, ← Rat.cast_add, ← Rat.cast_mul, ← Rat.cast_mul,
    ← Rat.cast_add, ← Rat.cast_mul, ← Rat.cast_div, Rat.cast_le]

/-! ## G3: the directed subset bound

The bidirectional no-sparse-cut bound of `directedSubsetGammaDense_of_subsetStable`,
packaged over every community and every subset of its fiber. As undirected, the
mirror enumerates each fiber's powerset, so its decision is exponential in
community size — fine for the small fixtures, `null` above the emitter's node
bound. Per the triage, this is a characterization flag, not an output guarantee:
meso's converged output can violate the bound (the committed n=4 fixture does). -/

/-- **The directed subset bound as a real partition predicate.** For every
    community `C` and every subset `S` of its fiber (with `T = C \ S`),
    `γ (Kout_S·Kin_T + Kout_T·Kin_S) / m ≤ e(S,T) + e(T,S)`; the
    partition-predicate form of the conclusion
    `directedSubsetGammaDense_of_subsetStable` proves for a directed subset-stable
    partition. Directed analogue of `IsSubsetOptimal`. -/
def DirectedSubsetOptimal (G : DirectedWeightedGraph n) (γ : ℝ) (p : Partition n) : Prop :=
  ∀ (C : ℕ) (S : Finset (Fin n)), (∀ i ∈ S, p i = C) →
    γ * ((∑ i ∈ S, G.outDegree i)
            * (∑ j ∈ Finset.univ.filter (fun j => p j = C ∧ j ∉ S), G.inDegree j)
         + (∑ i ∈ Finset.univ.filter (fun j => p j = C ∧ j ∉ S), G.outDegree i)
            * (∑ j ∈ S, G.inDegree j)) / G.totalWeight
      ≤ (∑ i ∈ S, ∑ j ∈ Finset.univ.filter (fun j => p j = C ∧ j ∉ S), G.weight i j)
        + (∑ i ∈ Finset.univ.filter (fun j => p j = C ∧ j ∉ S), ∑ j ∈ S, G.weight i j)

/-- **Directed subset-bound mirror (Bool-decidable).** The same bound over ℚ, with
    `C` ranging over occupied labels and `S` over each fiber's powerset, so the
    decision enumerates subsets. Decidable rational form of
    `DirectedSubsetOptimal`. -/
def DirectedSubsetOptimalQ (G : DirectedWeightedGraphQ n) (γ : ℚ) (p : Partition n) : Prop :=
  ∀ C ∈ Finset.univ.image p,
    ∀ S ∈ (Finset.univ.filter (fun i => p i = C)).powerset,
      γ * ((∑ i ∈ S, G.outDegree i)
              * (∑ j ∈ Finset.univ.filter (fun j => p j = C ∧ j ∉ S), G.inDegree j)
           + (∑ i ∈ Finset.univ.filter (fun j => p j = C ∧ j ∉ S), G.outDegree i)
              * (∑ j ∈ S, G.inDegree j)) / G.totalWeight
        ≤ (∑ i ∈ S, ∑ j ∈ Finset.univ.filter (fun j => p j = C ∧ j ∉ S), G.weight i j)
          + (∑ i ∈ Finset.univ.filter (fun j => p j = C ∧ j ∉ S), ∑ j ∈ S, G.weight i j)

instance (G : DirectedWeightedGraphQ n) (γ : ℚ) (p : Partition n) :
    Decidable (DirectedSubsetOptimalQ G γ p) :=
  inferInstanceAs (Decidable (∀ _ ∈ _, _))

/-- The directed subset-bound mirror decides `DirectedSubsetOptimal` on the real
    graph. The mirror ranges `C` over occupied labels and `S` over each fiber's
    powerset; the real predicate ranges over all `C : ℕ` and all `S`, but the two
    agree: a nonempty subset contained in some community forces that community into
    the image, and the empty subset makes both sides of the bound `0`. Directed
    twin of `subsetOptimalQ_iff`. -/
theorem directedSubsetOptimalQ_iff (G : DirectedWeightedGraphQ n) (γ : ℚ) (p : Partition n) :
    DirectedSubsetOptimalQ G γ p ↔ DirectedSubsetOptimal G.toReal (γ : ℝ) p := by
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
      simp only [DirectedWeightedGraphQ.toReal_outDegree, DirectedWeightedGraphQ.toReal_inDegree,
        DirectedWeightedGraphQ.toReal_weight, DirectedWeightedGraphQ.toReal_totalWeight]
      exact_mod_cast h
    · rw [Finset.not_nonempty_iff_eq_empty] at hS
      subst hS
      simp
  · intro hOpt C hC S hSpow
    rw [Finset.mem_powerset] at hSpow
    have hSC : ∀ i ∈ S, p i = C := fun i hi => (Finset.mem_filter.mp (hSpow hi)).2
    have h := hOpt C S hSC
    simp only [DirectedWeightedGraphQ.toReal_outDegree, DirectedWeightedGraphQ.toReal_inDegree,
      DirectedWeightedGraphQ.toReal_weight, DirectedWeightedGraphQ.toReal_totalWeight] at h
    exact_mod_cast h

end Meso
