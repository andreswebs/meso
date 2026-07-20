/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.DirectedAggregate
import Meso.Convergence
import Meso.SubsetOptimality

/-!
# Directed convergence: the fixed point the directed guarantees are stated about

The directed analogue of `Meso/Convergence.lean`. The directed guarantees
(weak connectivity, γ-separation, the subset conditional) are, like their
undirected CPM counterparts, not about "the output of the algorithm" as a
process but about a partition the algorithm has driven to a fixed point,
characterised by stability properties.

Per the research plan's open question 5 and the Phase 3 triage recommendation,
this file does NOT introduce a shared quality typeclass over both graph types. It
reuses the graph-agnostic `IsLocalMoveStable` (which constrains an abstract
`Q : Partition n → ℝ`), instantiated at `directedModularity`, and adds standalone
directed variants of level stability, convergence, and subset stability. Keeping
the directed stability definitions standalone keeps the directed statements direct
and avoids touching the undirected core.

The undirected file states level stability over a `QualityFamily` so modularity
and CPM share one definition; here there is a single objective
(`directedModularity`), so the family abstraction is unnecessary and the
aggregate objective is named directly.
-/

namespace Meso

variable {n : ℕ}

/-- **Directed level stable.** No improving aggregate move under directed
    modularity: the directed aggregate graph's singleton partition (each community
    collapsed to its own node) is itself local-move stable. A single move there
    merges two communities, so this says no community-merge improves directed
    modularity. Directed analogue of `IsLevelStable`; because there is a single
    directed objective, the aggregate quality is named directly rather than via a
    `QualityFamily`. -/
def IsDirectedLevelStable (G : DirectedWeightedGraph n) (γ : ℝ) (p : Partition n) : Prop :=
  IsLocalMoveStable (directedModularity (directedAggregate G p) γ) (fun A => (A : ℕ))

/-- **Directed converged.** The fixed point the directed guarantees are stated
    about: a partition that is both local-move stable (no single-node move raises
    directed modularity) and level stable (no aggregate move / community merge).
    Directed analogue of `IsConverged`. -/
def IsDirectedConverged (G : DirectedWeightedGraph n) (γ : ℝ) (p : Partition n) : Prop :=
  IsLocalMoveStable (directedModularity G γ) p ∧ IsDirectedLevelStable G γ p

/-- A directed-converged partition is in particular local-move stable (first
    projection), the hypothesis single-node reasoning is stated from. Mirrors
    `IsConverged.localMoveStable`. -/
theorem IsDirectedConverged.localMoveStable {G : DirectedWeightedGraph n} {γ : ℝ}
    {p : Partition n} (h : IsDirectedConverged G γ p) :
    IsLocalMoveStable (directedModularity G γ) p :=
  h.1

/-- A directed-converged partition is in particular level stable (second
    projection), the hypothesis directed γ-separation is stated from. Mirrors
    `IsConverged.levelStable`. -/
theorem IsDirectedConverged.levelStable {G : DirectedWeightedGraph n} {γ : ℝ}
    {p : Partition n} (h : IsDirectedConverged G γ p) : IsDirectedLevelStable G γ p :=
  h.2

/-- **Directed subset stable.** No reassignment of a whole node set strictly
    improves directed modularity. The subset-level fixed point: strictly stronger
    than single-node local-move stability (recovered as the singleton case,
    `IsDirectedSubsetStable.isLocalMoveStable`), and the stability the Leiden
    refinement targets. Directed analogue of `IsSubsetStable`; reuses the
    graph-agnostic `moveSubset`. -/
def IsDirectedSubsetStable (G : DirectedWeightedGraph n) (γ : ℝ) (p : Partition n) : Prop :=
  ∀ (S : Finset (Fin n)) (c : ℕ),
    directedModularity G γ (moveSubset p S c) ≤ directedModularity G γ p

/-- Directed subset stability implies single-node local-move stability (the
    singleton case), so it is a genuine strengthening of the first half of
    `IsDirectedConverged`. Mirrors `IsSubsetStable.isLocalMoveStable`. -/
theorem IsDirectedSubsetStable.isLocalMoveStable {G : DirectedWeightedGraph n} {γ : ℝ}
    {p : Partition n} (h : IsDirectedSubsetStable G γ p) :
    IsLocalMoveStable (directedModularity G γ) p := by
  intro v c
  have hstep := h {v} c
  rwa [moveSubset_singleton] at hstep

end Meso
