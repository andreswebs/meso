/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Mathlib.Logic.Relation
import Mathlib.Data.Fintype.Card
import Mathlib.Order.Iterate

/-!
# A computable reachable-set closure

An efficient, provably-correct reachability decision over a finite type, used by the
connectivity value-oracle. Mathlib decides `SimpleGraph.Reachable` by
enumerating walks up to length `card V`, which is exponential and unusable past a
handful of nodes. This file gives the standard fixpoint alternative: iterate a
neighbour-expanding step `card α` times from the start vertex, and prove the resulting
`Finset` is exactly the set of `Relation.ReflTransGen`-reachable vertices.

The one nontrivial fact is that an inflationary `Finset` map reaches a fixed point
within `card α` iterations (`inflationary_iterate_fixed`): a strictly growing chain of
subsets of a `card α`-element type cannot outrun `card α` steps.
-/

namespace Meso

section Fixpoint

variable {α : Type*} [Fintype α]

/-- Iterating an inflationary `Finset` map (`S ⊆ f S`) reaches a fixed point within
    `Fintype.card α` steps: the chain `f^[k] s` strictly grows until it stabilises, and
    a strictly growing chain of subsets of a `card α`-element type stabilises by step
    `card α`. -/
theorem inflationary_iterate_fixed (f : Finset α → Finset α) (hf : ∀ S, S ⊆ f S)
    (s : Finset α) : f (f^[Fintype.card α] s) = f^[Fintype.card α] s := by
  set N := Fintype.card α with hN
  -- once two consecutive iterates coincide, the chain is constant from there on
  have stab : ∀ k m, f^[k] s = f^[k + 1] s → f^[k + m] s = f^[k] s := by
    intro k m hk
    induction m with
    | zero => rfl
    | succ m ih =>
      have e : k + (m + 1) = (k + m) + 1 := by omega
      rw [e, Function.iterate_succ_apply', ih, ← Function.iterate_succ_apply' f k s, ← hk]
  -- some consecutive pair among the first N+1 iterates coincides
  have exStable : ∃ k ≤ N, f^[k] s = f^[k + 1] s := by
    by_contra hcon
    push Not at hcon
    have hgrow : ∀ k ≤ N, (f^[k] s).card + 1 ≤ (f^[k + 1] s).card := by
      intro k hk
      have hsub : f^[k] s ⊆ f^[k + 1] s := by
        rw [Function.iterate_succ_apply']; exact hf _
      exact Finset.card_lt_card (hsub.ssubset_of_ne (hcon k hk))
    have hbig : ∀ k ≤ N + 1, k ≤ (f^[k] s).card := by
      intro k
      induction k with
      | zero => intro _; exact Nat.zero_le _
      | succ k ih =>
        intro hk
        have hik : k ≤ (f^[k] s).card := ih (by omega)
        have := hgrow k (by omega)
        omega
    have hbN := hbig (N + 1) le_rfl
    have hle : (f^[N + 1] s).card ≤ N := by rw [hN]; exact Finset.card_le_univ _
    omega
  obtain ⟨k, hkN, hk⟩ := exStable
  have h1 : f^[N] s = f^[k] s := by
    have := stab k (N - k) hk; rwa [Nat.add_sub_cancel' hkN] at this
  have h2 : f^[N + 1] s = f^[k] s := by
    have := stab k (N + 1 - k) hk; rwa [Nat.add_sub_cancel' (by omega)] at this
  rw [← Function.iterate_succ_apply' f N s, h2, h1]

end Fixpoint

section Reachable

variable {α : Type*} [Fintype α] [DecidableEq α] (r : α → α → Prop) [DecidableRel r]

/-- One breadth-first expansion: add every vertex adjacent to the current set. -/
def stepRel (R : Finset α) : Finset α :=
  R ∪ Finset.univ.filter (fun j => ∃ i ∈ R, r i j)

/-- `stepRel` is inflationary. -/
theorem subset_stepRel (R : Finset α) : R ⊆ stepRel r R := by
  rw [stepRel]; exact Finset.subset_union_left

/-- Every iterate of `stepRel` contains its starting set. -/
theorem subset_iterate_stepRel (k : ℕ) (S : Finset α) : S ⊆ (stepRel r)^[k] S := by
  induction k generalizing S with
  | zero => simp
  | succ k ih =>
    rw [Function.iterate_succ_apply]
    exact (subset_stepRel r S).trans (ih _)

/-- The computable set of vertices reachable from `a`: iterate `stepRel` `card α` times.
    `card α` steps suffice because the reachable set stabilises by then
    (`inflationary_iterate_fixed`). -/
def reachableFinset (a : α) : Finset α :=
  (stepRel r)^[Fintype.card α] {a}

/-- Everything in `reachableFinset` is genuinely reachable. Proved by induction on the
    iteration count: a freshly added vertex has a `stepRel`-predecessor already known
    reachable. -/
theorem reachableFinset_sound {a x : α} (hx : x ∈ reachableFinset r a) :
    Relation.ReflTransGen r a x := by
  have key : ∀ k, ∀ y ∈ (stepRel r)^[k] ({a} : Finset α), Relation.ReflTransGen r a y := by
    intro k
    induction k with
    | zero => intro y hy; rw [Finset.mem_singleton.mp hy]
    | succ k ih =>
      intro y hy
      rw [Function.iterate_succ_apply', stepRel, Finset.mem_union] at hy
      rcases hy with hy | hy
      · exact ih y hy
      · rw [Finset.mem_filter] at hy
        obtain ⟨i, hi, hri⟩ := hy.2
        exact (ih i hi).tail hri
  exact key _ x hx

/-- `reachableFinset` is closed under `r`: it is a fixed point of `stepRel`, so any
    `r`-successor of a member is itself a member. -/
theorem reachableFinset_closed {a i j : α} (hi : i ∈ reachableFinset r a) (hij : r i j) :
    j ∈ reachableFinset r a := by
  have hfix : stepRel r (reachableFinset r a) = reachableFinset r a :=
    inflationary_iterate_fixed (stepRel r) (subset_stepRel r) {a}
  rw [← hfix, stepRel, Finset.mem_union]
  exact Or.inr (Finset.mem_filter.mpr ⟨Finset.mem_univ _, i, hi, hij⟩)

/-- The start vertex is reachable from itself. -/
theorem self_mem_reachableFinset (a : α) : a ∈ reachableFinset r a :=
  subset_iterate_stepRel r (Fintype.card α) {a} (Finset.mem_singleton_self a)

/-- Every reachable vertex is in `reachableFinset` — completeness via closure under `r`
    from the start vertex. Together with soundness this is the correctness spec. -/
theorem reachableFinset_complete {a x : α} (h : Relation.ReflTransGen r a x) :
    x ∈ reachableFinset r a := by
  induction h with
  | refl => exact self_mem_reachableFinset r a
  | tail _ hij ih => exact reachableFinset_closed r ih hij

/-- **Correctness of the reachable-set closure.** A vertex is in `reachableFinset` iff
    it is `ReflTransGen`-reachable from the start. -/
theorem mem_reachableFinset_iff {a x : α} :
    x ∈ reachableFinset r a ↔ Relation.ReflTransGen r a x :=
  ⟨reachableFinset_sound r, reachableFinset_complete r⟩

end Reachable

end Meso
