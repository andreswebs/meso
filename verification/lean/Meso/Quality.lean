/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.Graph

/-!
# Quality functions and the first monotonicity target

Modularity with a resolution parameter, plus the first invariant obligation from
the plan (section 7): quality is monotone non-decreasing across the algorithm's
aggregation levels.

The theorem below is deliberately split at the honest seam. Global monotonicity
follows from *stepwise* non-decrease by transitivity; that easy half is proved
here. The substance is the hard half: proving the engine actually
produces stepwise-non-decreasing runs (each local move and each aggregation does
not lower quality). That half needs the move and aggregation operators, which are
not modelled yet, and is the next obligation.
-/

namespace Meso

/-- A partition assigns each node a community label. Well-formedness (each node in
    exactly one community) is structural: a `Partition` is a total function, so it
    needs no side condition. -/
def Partition (n : ℕ) := Fin n → ℕ

variable {n : ℕ}

/-- Modularity with resolution parameter `γ`:
    `Q = (1 / 2m) · ∑_{i,j} (w_{ij} − γ · k_i · k_j / 2m) · δ(c_i, c_j)`.

    Noncomputable because it is defined over the reals; floating point is an
    implementation detail of the Go core, not of the model (plan section 7). -/
noncomputable def modularity (G : WeightedGraph n) (γ : ℝ) (p : Partition n) : ℝ :=
  (1 / G.twoM) * ∑ i, ∑ j,
    (G.weight i j - γ * G.degree i * G.degree j / G.twoM) *
      (if p i = p j then (1 : ℝ) else 0)

/-- **Monotonicity target — global monotonicity from stepwise non-decrease.**

    If a quality function `Q` does not decrease between consecutive partitions of a
    run, then it does not decrease between any earlier and any later partition. This
    is the easy half (transitivity of `≤` along an `IsChain`), and it is proved.

    Stated for an arbitrary `Q : Partition n → ℝ`, not just `modularity`: it uses
    only transitivity of `≤`, so every quality function (modularity, CPM, ...)
    shares it. The hard half, the real obligation, is discharging the
    `IsChain` hypothesis: proving that the local-move and aggregation steps each
    produce a next partition of no-lower quality. That lands as those operators are
    modelled. -/
theorem quality_monotone_of_stepwise
    (Q : Partition n → ℝ) (ps : List (Partition n))
    (h : ps.IsChain (fun a b => Q a ≤ Q b)) :
    ps.Pairwise (fun a b => Q a ≤ Q b) := by
  haveI : IsTrans (Partition n) (fun a b => Q a ≤ Q b) :=
    ⟨fun _ _ _ hab hbc => le_trans hab hbc⟩
  exact List.isChain_iff_pairwise.mp h

end Meso
