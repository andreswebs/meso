/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.Move

/-!
# Synchronous-round confluence (concurrency track)

The design half of the concurrency tier (plan sections 4.5, 7): a parallel local-move
round should produce the same partition no matter how its work is scheduled across cores.
This file models such a round and proves it is *confluent* — its outcome is independent of
the order the moves are applied, hence of the number of cores.

The model is what makes "synchronous" precise. Every node's target is decided from the
*round-start snapshot* `p`, packaged as a fixed function `t : Fin n → ℕ` (node `v` wants to
be in community `t v`). A round then applies, over a work-list `vs` of the nodes that move,
the single-node reassignments `move · v (t v)`. Crucially `t` does not change as moves land
— a node never reads a neighbour's mid-round write — which is exactly the synchronous
(snapshot) semantics.

Under those semantics confluence is automatic and needs no disjointness hypothesis:

* two moves at distinct nodes commute (`move_comm`, i.e. `Function.update_comm`), and
* two moves at the *same* node both write that node's single snapshot target, so they are
  idempotent.

So the round collapses to a closed form — `applyRound_eq`: node `w` ends at `t w` if it is
in the work-list and `p w` otherwise — which is manifestly a function of `(p, t, set of
moved nodes)` alone. Order- and core-count-independence (`applyRound_perm`) falls out: any
two schedules of the same work are list permutations of each other, and membership in the
work-list is permutation-invariant.

Honest scope, for the CORRESPONDENCE divergence register. This proves *determinism* of a
synchronous round (same result under any schedule / core count), not that the round
*improves* quality: independent snapshot moves can conflict, and a synchronous round can
oscillate. Handling that (and the concrete parallel scheme) is the still-open design choice
of plan section 4.5; the monotonicity story lives with the sequential local move
(`Meso.Move`). Confluence is the property that lets the parallel scheme be chosen freely
without changing the result.
-/

namespace Meso

variable {n : ℕ}

/-- **Distinct-node moves commute.** Reassigning `v` then `w` equals reassigning `w` then
    `v` when `v ≠ w`, since the two writes touch different coordinates. The
    single-conflict-free case underlying round confluence (`Function.update_comm`). -/
lemma move_comm {p : Partition n} {v w : Fin n} (hvw : v ≠ w) (c d : ℕ) :
    move (move p v c) w d = move (move p w d) v c := by
  unfold move
  exact Function.update_comm hvw c d p

/-- A synchronous local-move round: fold the reassignments `move · v (t v)` over the
    work-list `vs`, each node going to its snapshot-decided target `t v`. The list order is
    the schedule; `applyRound_perm` shows the result does not depend on it. -/
def applyRound (p : Partition n) (t : Fin n → ℕ) (vs : List (Fin n)) : Partition n :=
  vs.foldl (fun q v => move q v (t v)) p

/-- Processing the head of the work-list first: a round over `a :: vs` is the move of `a`
    followed by the round over `vs`. Definitional, the induction step for the closed
    form. -/
lemma applyRound_cons (p : Partition n) (t : Fin n → ℕ) (a : Fin n) (vs : List (Fin n)) :
    applyRound p t (a :: vs) = applyRound (move p a (t a)) t vs :=
  rfl

/-- **Closed form of a synchronous round.** After a round with work-list `vs`, node `w`
    sits at its snapshot target `t w` if it was in `vs`, and is left at `p w` otherwise.
    The right-hand side mentions neither the order nor the multiplicity of `vs`, so this is
    the confluence fact in its strongest form: the outcome is a function of the moved-node
    *set* and the snapshot decisions alone. -/
theorem applyRound_eq (p : Partition n) (t : Fin n → ℕ) (vs : List (Fin n)) :
    applyRound p t vs = fun w => if w ∈ vs then t w else p w := by
  induction vs generalizing p with
  | nil => funext w; simp [applyRound]
  | cons a vs ih =>
    rw [applyRound_cons, ih]
    funext w
    by_cases hw : w ∈ vs
    · simp [List.mem_cons, hw]
    · rw [if_neg hw]
      simp only [List.mem_cons, hw, or_false, move, Function.update_apply]
      by_cases hwa : w = a <;> simp [hwa]

/-- **Round confluence: the outcome is schedule-independent.** Any two work-lists that are
    permutations of each other — the same moved nodes scheduled in any order, split across
    any number of cores — yield the same partition. Immediate from the closed form
    (`applyRound_eq`), since list-membership is permutation-invariant. This is the
    concurrency guarantee: the parallel round's result does not depend on the core count. -/
theorem applyRound_perm (p : Partition n) (t : Fin n → ℕ) {vs ws : List (Fin n)}
    (h : vs.Perm ws) : applyRound p t vs = applyRound p t ws := by
  rw [applyRound_eq, applyRound_eq]
  funext w
  simp only [h.mem_iff]

/-- Splitting a round's work across two cores (process `vs`, then `ws`) is the same as
    handing one core the concatenation. With `applyRound_perm`, any distribution of the
    work over cores and any interleaving of their results agree, since all are permutations
    of the full work-list. -/
lemma applyRound_append (p : Partition n) (t : Fin n → ℕ) (vs ws : List (Fin n)) :
    applyRound p t (vs ++ ws) = applyRound (applyRound p t vs) t ws := by
  simp only [applyRound, List.foldl_append]

end Meso
