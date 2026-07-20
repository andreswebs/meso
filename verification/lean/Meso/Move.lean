/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.Quality

/-!
# The local-move operator and its monotonicity

The first piece of the "hard half" of monotonicity (see `Meso.Quality`): the
fast local-move phase reassigns one node at a time to a neighbouring community.
This file models a single reassignment (`move`) and proves the key monotonicity
fact: choosing the best available target never lowers modularity, because leaving
the node where it is ("staying") is always one of the options.

This is *why* local moving is monotone, and it needs no expansion of the
modularity-gain formula: it is purely that a maximum over a set containing the
current configuration is at least the current value. The heavier algebra (the
closed-form ΔQ of a move) comes later, when we bound the gain rather than merely
sign it.
-/

namespace Meso

variable {n : ℕ}

/-- Reassign node `v` to community `c`, leaving every other node unchanged. -/
def move (p : Partition n) (v : Fin n) (c : ℕ) : Partition n :=
  Function.update p v c

/-- Moving a node to the community it is already in is a no-op. -/
@[simp] lemma move_self (p : Partition n) (v : Fin n) : move p v (p v) = p :=
  Function.update_eq_self v p

/-- **A best single-node move does not decrease quality, for any quality function.**

    If, for node `v`, the target community `c` is at least as good under `Q` as
    every candidate in `S` (`hmax`), and `v`'s current community is itself a
    candidate (`hcurr`), then moving `v` to `c` does not lower `Q`.

    The proof is exactly the observation that staying put is always available:
    instantiate maximality at the current community, where `move` is the identity
    (`move_self`). It uses nothing about `Q`, so modularity and CPM both inherit
    local-move monotonicity from it. This is the sign of the local-move gain;
    bounding its magnitude is a later obligation. -/
theorem move_best_ge {Q : Partition n → ℝ} (p : Partition n) (v : Fin n)
    (S : Finset ℕ) (c : ℕ) (hcurr : p v ∈ S)
    (hmax : ∀ c' ∈ S, Q (move p v c') ≤ Q (move p v c)) :
    Q p ≤ Q (move p v c) := by
  have h := hmax (p v) hcurr
  rwa [move_self] at h

/-- `move_best_ge` specialised to modularity. -/
theorem modularity_bestMove_ge
    (G : WeightedGraph n) (γ : ℝ) (p : Partition n) (v : Fin n)
    (S : Finset ℕ) (c : ℕ) (hcurr : p v ∈ S)
    (hmax : ∀ c' ∈ S, modularity G γ (move p v c') ≤ modularity G γ (move p v c)) :
    modularity G γ p ≤ modularity G γ (move p v c) :=
  move_best_ge p v S c hcurr hmax

/-- One step of the fast local-move phase, for a quality function `Q`: `q` is `p`
    with a single node `v` reassigned to a community `c` that is best under `Q`
    among a candidate set `S` containing `v`'s current community. The candidate set
    abstracts the concrete "neighbouring communities plus stay" choice; all the
    monotonicity argument needs is that the current community is in it.

    Parameterising by `Q` rather than by `(G, γ)` is the shared local-move
    interface: modularity and CPM are the instances `IsLocalMove (modularity G γ)`
    and `IsLocalMove (cpm G γ)`. -/
def IsLocalMove (Q : Partition n → ℝ) (p q : Partition n) : Prop :=
  ∃ (v : Fin n) (S : Finset ℕ) (c : ℕ),
    p v ∈ S ∧
    (∀ c' ∈ S, Q (move p v c') ≤ Q (move p v c)) ∧
    q = move p v c

/-- A single local-move step does not decrease `Q`. -/
theorem IsLocalMove.le {Q : Partition n → ℝ} {p q : Partition n}
    (h : IsLocalMove Q p q) : Q p ≤ Q q := by
  obtain ⟨v, S, c, hcurr, hmax, rfl⟩ := h
  exact move_best_ge p v S c hcurr hmax

/-- `IsLocalMove.le` specialised to modularity. -/
theorem IsLocalMove.modularity_le {G : WeightedGraph n} {γ : ℝ} {p q : Partition n}
    (h : IsLocalMove (modularity G γ) p q) : modularity G γ p ≤ modularity G γ q :=
  h.le

/-- **The local-move sweep is monotone, for any quality function.** Along any run
    of local-move steps (a chain of `IsLocalMove Q`), `Q` does not decrease from any
    earlier partition to any later one.

    This discharges the `IsChain` hypothesis of `quality_monotone_of_stepwise` for
    the local-move phase end to end: a chain of `IsLocalMove Q` steps weakens to a
    chain of `Q`-non-decrease (`IsChain.imp`), which is globally monotone by
    transitivity. It is the whole point of the fast local move — every sweep only
    ever helps. -/
theorem localMoveRun_monotone (Q : Partition n → ℝ) (ps : List (Partition n))
    (h : ps.IsChain (IsLocalMove Q)) :
    ps.Pairwise (fun a b => Q a ≤ Q b) :=
  quality_monotone_of_stepwise Q ps (h.imp fun _ _ hm => hm.le)

end Meso
