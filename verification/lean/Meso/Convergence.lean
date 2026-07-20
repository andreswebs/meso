/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.CPM

/-!
# The converged partition

The fixed point the paper guarantees (γ-separation, γ-connectivity, subset-optimality)
are stated about. The paper's theorems are not about "the output of the algorithm"
as a process; they are about a partition that the algorithm has driven to a fixed
point, characterised by two stability properties (plan section 4.1):

* **Local-move stable** — no single-node reassignment strictly improves quality.
  Each node already sits in a locally optimal community. This is the premise of
  γ-separation: if a node had a strictly better community it would move, so at a
  stable partition distinct communities are γ-separated.

* **Level stable** — no improving *aggregate* move. Collapsing each community to a
  node and asking whether merging two of them helps: at convergence it does not.
  This is exactly local-move stability of the aggregate graph's singleton partition,
  so it reuses the same notion one level up.

The definitions are the whole point of this file, and the judgement is in getting
them right — a too-weak "stable" makes the paper theorems vacuous. The adequacy lemmas
below pin the meaning down: a local-move-stable partition is a genuine fixed point
of the move dynamics (no accepted move exists, and any local-move step leaves
quality unchanged), so "stable" is not vacuously satisfiable.

Everything is stated for an arbitrary quality function, so modularity and CPM share
one definition of convergence (`modularityF`, `cpmF` are the instances). The
γ-guarantees are cleanest in CPM.
-/

namespace Meso

variable {n : ℕ}

/-- **Local-move stable.** No single-node move strictly improves `Q`: for every node
    `v` and every target community `c`, reassigning `v` to `c` does not raise `Q`.
    Equivalently, `Q p` is maximal over all single-node reassignments — the fixed
    point of the fast local-move phase. Quantifying `c` over all of `ℕ` covers every
    option at once: staying (`c = p v`), isolating `v` (a fresh label), and joining
    any existing community. -/
def IsLocalMoveStable (Q : Partition n → ℝ) (p : Partition n) : Prop :=
  ∀ (v : Fin n) (c : ℕ), Q (move p v c) ≤ Q p

/-- **Adequacy: a stable partition is a fixed point of the local-move relation.** Any
    `IsLocalMove` step out of a local-move-stable `p` leaves quality unchanged. A
    move picks the best target in its candidate set, and staying put is in that set,
    so the move is at least as good as staying (`≥`); stability makes it at most as
    good (`≤`). Hence equal — a stable partition cannot be strictly improved by the
    very steps the algorithm takes, which is what makes the definition non-vacuous. -/
theorem IsLocalMove.quality_eq_of_stable {Q : Partition n → ℝ} {p q : Partition n}
    (hstab : IsLocalMoveStable Q p) (h : IsLocalMove Q p q) : Q q = Q p := by
  obtain ⟨v, S, c, hcurr, hmax, rfl⟩ := h
  refine le_antisymm (hstab v c) ?_
  have := hmax (p v) hcurr
  rwa [move_self] at this

/-- **Adequacy: no strictly-improving local move exists from a stable partition.**
    Restates `IsLocalMove.quality_eq_of_stable` as the impossibility of strict
    improvement — the direct sense in which local-move stability is a local optimum. -/
theorem IsLocalMoveStable.no_strict_improvement {Q : Partition n → ℝ} {p : Partition n}
    (h : IsLocalMoveStable Q p) : ¬ ∃ q, IsLocalMove Q p q ∧ Q p < Q q := by
  rintro ⟨q, hmove, hlt⟩
  exact absurd (le_of_eq (hmove.quality_eq_of_stable h)) (not_le.mpr hlt)

/-- A graph-indexed quality function: it scores a partition on a graph of *any*
    size, the graph explicit so it can also be read off an aggregate. `modularityF γ`
    and `cpmF γ` are the two instances; keeping the family abstract is what lets
    convergence be defined once for both. -/
def QualityFamily := ∀ (m : ℕ), WeightedGraph m → Partition m → ℝ

/-- Modularity as a quality family (at fixed resolution `γ`). -/
noncomputable def modularityF (γ : ℝ) : QualityFamily := fun _ G p => modularity G γ p

/-- CPM as a quality family (at fixed resolution `γ`). -/
noncomputable def cpmF (γ : ℝ) : QualityFamily := fun _ G p => cpm G γ p

/-- **Level stable.** No improving aggregate move: the aggregate graph's singleton
    partition (each community collapsed to its own node) is itself local-move stable.
    A single move there merges two communities, so this says no community-merge
    improves quality — the convergence condition one level up. -/
def IsLevelStable (Qf : QualityFamily) (G : WeightedGraph n) (p : Partition n) : Prop :=
  IsLocalMoveStable (Qf (numComm p) (aggregate G p)) (fun A => (A : ℕ))

/-- **Converged.** The fixed point the paper guarantees are stated about: a partition
    that is both local-move stable (no node moves) and level stable (no aggregate
    move / community merge). Instantiate `Qf` with `modularityF γ` or `cpmF γ`. -/
def IsConverged (Qf : QualityFamily) (G : WeightedGraph n) (p : Partition n) : Prop :=
  IsLocalMoveStable (Qf n G) p ∧ IsLevelStable Qf G p

/-- A converged partition is in particular local-move stable (first projection),
    the hypothesis γ-separation is stated from. -/
theorem IsConverged.localMoveStable {Qf : QualityFamily} {G : WeightedGraph n}
    {p : Partition n} (h : IsConverged Qf G p) : IsLocalMoveStable (Qf n G) p :=
  h.1

/-- A converged partition is in particular level stable (second projection). -/
theorem IsConverged.levelStable {Qf : QualityFamily} {G : WeightedGraph n}
    {p : Partition n} (h : IsConverged Qf G p) : IsLevelStable Qf G p :=
  h.2

end Meso
