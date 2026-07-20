/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.Separation
import Meso.GammaConnectivity
import Meso.SubsetOptimality

/-!
# The Leiden guarantees, conjoined

The terminus of the paper-theorems tier: a single theorem certifying that a converged
Leiden output satisfies all three Traag-Waltman-van Eck (2019) guarantees at once —
γ-separation, γ-connectivity, and subset-optimality.

The three guarantees constrain genuinely different structures, so the bundle collects the
three fixed-point / provenance hypotheses each one needs rather than deriving them from
one minimal condition (recorded in the CORRESPONDENCE divergence register):

* **γ-separation** reasons about the *aggregate* graph and needs `IsConverged` (its level
  stability: no community merge improves).
* **γ-connectivity** is a *provenance* property: it holds because the partition was built
  by a gated refinement run from a γ-well-connected base, and gated merges preserve it. So
  the honest hypothesis is that such a run exists.
* **subset-optimality** reasons about *subsets of the base graph* and needs the stronger
  `IsSubsetStable` fixed point (which the refinement targets).

`IsLeidenStable` collects exactly these three, one per guarantee; each is what the
corresponding phase of the real algorithm establishes. `leidenGuarantees_of_stable` then
discharges all three guarantee statements from that bundle, reusing the per-theorem
results unchanged. This is the whole point of the file, and — as everywhere in this
development — the judgement is in the statement: `IsLeidenStable` must describe the actual
converged output, and `LeidenGuarantees` must be the paper's three conclusions, not weaker
surrogates.
-/

namespace Meso

variable {n : ℕ}

/-- **A converged Leiden output.** The three fixed-point / provenance conditions the
    guarantees rest on, one per guarantee: the partition is converged (`IsConverged`, for
    γ-separation), it arose from a gated refinement run out of a γ-well-connected base (for
    γ-connectivity), and it is subset-stable (`IsSubsetStable`, for subset-optimality).
    Each is what a phase of the algorithm establishes. -/
structure IsLeidenStable (γ : ℝ) (G : WeightedGraph n) (p : Partition n) : Prop where
  /-- No node move and no community merge improves CPM (the γ-separation hypothesis). -/
  converged : IsConverged (cpmF γ) G p
  /-- `p` was produced by a gated refinement run from a γ-well-connected base (the
      γ-connectivity provenance). -/
  refinedFrom : ∃ p₀, GammaWellConnectedCommunities G γ p₀
    ∧ Relation.ReflTransGen (GammaMergeStep G γ) p₀ p
  /-- No subset reassignment improves CPM (the subset-optimality hypothesis). -/
  subsetStable : IsSubsetStable G γ p

/-- **The three Leiden guarantees, conjoined.** γ-separation of distinct communities on
    the aggregate, γ-well-connectedness (connected and internally γ-dense) of every
    community, and subset-optimality (no subset of any community can be split off to raise
    quality — no sparse cut at any scale). The paper's terminal characterisation of a
    Leiden partition. -/
structure LeidenGuarantees (γ : ℝ) (G : WeightedGraph n) (p : Partition n) : Prop where
  /-- Distinct communities are γ-separated: `e(C,D) ≤ γ S_C S_D`. -/
  gammaSeparated : ∀ {A B : Fin (numComm p)}, A ≠ B →
    (aggregate G p).weight A B ≤ γ * (aggregate G p).nodeSize A * (aggregate G p).nodeSize B
  /-- Every community is connected and internally γ-dense. -/
  gammaWellConnected : GammaWellConnectedCommunities G γ p
  /-- Every subset of every community is γ-densely connected to the rest (no sparse cut). -/
  subsetOptimal : IsSubsetOptimal G γ p

/-- **The Leiden guarantee theorem.** A converged Leiden output satisfies all three paper
    guarantees: γ-separation, γ-connectivity, and subset-optimality. Each conjunct is
    discharged by the corresponding result — `gammaSeparated_of_converged`,
    `gammaWellConnectedCommunities_of_gammaMergeRun`, and `isSubsetOptimal_of_stable` —
    from the matching hypothesis in `IsLeidenStable`. -/
theorem leidenGuarantees_of_stable {γ : ℝ} {G : WeightedGraph n} {p : Partition n}
    (h : IsLeidenStable γ G p) : LeidenGuarantees γ G p where
  gammaSeparated := fun {_ _} hAB => gammaSeparated_of_converged h.converged hAB
  gammaWellConnected := by
    obtain ⟨p₀, hbase, hrun⟩ := h.refinedFrom
    exact gammaWellConnectedCommunities_of_gammaMergeRun G γ p₀ p hbase hrun
  subsetOptimal := isSubsetOptimal_of_stable h.subsetStable

end Meso
