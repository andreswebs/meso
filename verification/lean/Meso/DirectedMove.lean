/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.DirectedGraph
import Meso.Move

/-!
# The directed move-delta identity

The closed-form incremental gain of a single-node move, proved equal to the
from-scratch difference of directed modularities. This is the identity Go's
`directed_move.go` (`directedModularity.moveDelta`) implements; its doc comment
carries the same derivation, and this file is its machine-checked form.

Two facts about the statement matter. It holds for every graph, including
`totalWeight = 0` (both sides collapse to `0` because Lean's `1 / 0 = 0`), so no
hypothesis on `totalWeight` is imposed. It does NOT hold for the no-op move
`t = p u` (the null term would spuriously contribute `−2γ·k_u^out·k_u^in/m²`), so
the hypothesis `t ≠ p u` is required; Go's early `if target == src { return 0 }`
is the operational counterpart.

Unlike the aggregation and symmetric-reduction results, this identity has no
undirected counterpart in the model: the undirected core deliberately never proved
its closed-form ΔQ (its Go incremental formula is validated by property test only,
see `Meso/Move.lean`). It is proved directed-side because Phase 3 (guarantee
triage) states directed γ-separation and subset-optimality as sign conditions on
move gains, and needs a closed form to reason about.

The partition combinatorics (how a single move perturbs an indicator-weighted
double sum) are isolated in the kernel-generic `sum_kernel_move_split`, so they are
proved once, independent of the modularity algebra.
-/

namespace Meso

namespace DirectedWeightedGraph

variable {n : ℕ} (G : DirectedWeightedGraph n)

/-- Arc weight from `u` into community `c` under `p`, self-loop excluded (the sum
    ranges over `j ≠ u`). Go counterpart: `woutTarget`/`woutSrc`, which sum
    `g.neighbors(u)`, a list that never contains `u` (self-loops are stored
    separately in the CSR). -/
noncomputable def outWeightTo (p : Partition n) (u : Fin n) (c : ℕ) : ℝ :=
  ∑ j ∈ (Finset.univ.erase u).filter (fun j => p j = c), G.weight u j

/-- Arc weight from community `c` into `u` under `p`, self-loop excluded. Go
    counterpart: `winTarget`/`winSrc` (over `g.inNeighbors(u)`). -/
noncomputable def inWeightFrom (p : Partition n) (u : Fin n) (c : ℕ) : ℝ :=
  ∑ i ∈ (Finset.univ.erase u).filter (fun i => p i = c), G.weight i u

/-- Summed out-degree of community `c` (the full fiber, self-loop contributions
    included). Go counterpart: `koutTarget`/`koutSrc`. -/
noncomputable def commOutDegree (p : Partition n) (c : ℕ) : ℝ :=
  ∑ i ∈ Finset.univ.filter (fun i => p i = c), G.outDegree i

/-- Summed in-degree of community `c` (the full fiber). Go counterpart:
    `kinTarget`/`kinSrc`. -/
noncomputable def commInDegree (p : Partition n) (c : ℕ) : ℝ :=
  ∑ i ∈ Finset.univ.filter (fun i => p i = c), G.inDegree i

/-- `outWeightTo` as an indicator-weighted sum over `j ≠ u`. -/
lemma outWeightTo_eq (p : Partition n) (u : Fin n) (c : ℕ) :
    G.outWeightTo p u c
      = ∑ j ∈ Finset.univ.erase u, G.weight u j * (if p j = c then (1 : ℝ) else 0) := by
  unfold outWeightTo
  rw [Finset.sum_filter]
  refine Finset.sum_congr rfl fun j _ => ?_
  rw [mul_ite, mul_one, mul_zero]

/-- `inWeightFrom` as an indicator-weighted sum over `i ≠ u`. -/
lemma inWeightFrom_eq (p : Partition n) (u : Fin n) (c : ℕ) :
    G.inWeightFrom p u c
      = ∑ i ∈ Finset.univ.erase u, G.weight i u * (if p i = c then (1 : ℝ) else 0) := by
  unfold inWeightFrom
  rw [Finset.sum_filter]
  refine Finset.sum_congr rfl fun i _ => ?_
  rw [mul_ite, mul_one, mul_zero]

/-- `commOutDegree` as an indicator-weighted sum over all nodes. -/
lemma commOutDegree_eq_sum (p : Partition n) (c : ℕ) :
    G.commOutDegree p c = ∑ i, G.outDegree i * (if p i = c then (1 : ℝ) else 0) := by
  unfold commOutDegree
  rw [Finset.sum_filter]
  refine Finset.sum_congr rfl fun i _ => ?_
  rw [mul_ite, mul_one, mul_zero]

/-- `commInDegree` as an indicator-weighted sum over all nodes. -/
lemma commInDegree_eq_sum (p : Partition n) (c : ℕ) :
    G.commInDegree p c = ∑ i, G.inDegree i * (if p i = c then (1 : ℝ) else 0) := by
  unfold commInDegree
  rw [Finset.sum_filter]
  refine Finset.sum_congr rfl fun i _ => ?_
  rw [mul_ite, mul_one, mul_zero]

end DirectedWeightedGraph

variable {n : ℕ}

/-- An indicator-weighted sum over `j ≠ u` equals the full sum minus the `u` term.
    Used to read the perturbed degree sums off as full community degrees. -/
lemma sum_erase_mul_ind_eq (g : Fin n → ℝ) (p : Partition n) (u : Fin n) (c : ℕ) :
    (∑ j ∈ Finset.univ.erase u, g j * (if p j = c then (1 : ℝ) else 0))
      = (∑ j, g j * (if p j = c then (1 : ℝ) else 0)) - g u * (if p u = c then (1 : ℝ) else 0) := by
  rw [eq_sub_iff_add_eq, add_comm]
  exact Finset.add_sum_erase Finset.univ
    (fun j => g j * (if p j = c then (1 : ℝ) else 0)) (Finset.mem_univ u)

/-- **How a single move perturbs an indicator-weighted double sum.** For any kernel
    `F` (independent of the partition), moving `u` from `s = p u` to `t ≠ s` changes
    `∑_{i,j} F i j · δ` only on row `u` and column `u` (the diagonal cell `(u,u)` is
    unchanged and cancels): each off-`u` cell in row `u` swaps its indicator from
    `χ(p j = s)` to `χ(p j = t)`, and symmetrically for column `u`.

    No hypothesis on `t` is needed: for the no-op `t = p u` both correction sums
    vanish. The `t ≠ p u` requirement enters only the modularity identity below,
    when the perturbed degree sums are read off as full community degrees. -/
lemma sum_kernel_move_split (F : Fin n → Fin n → ℝ) (p : Partition n) (u : Fin n)
    (t : ℕ) :
    (∑ i, ∑ j, F i j * (if move p u t i = move p u t j then (1 : ℝ) else 0))
      = (∑ i, ∑ j, F i j * (if p i = p j then (1 : ℝ) else 0))
        + (∑ j ∈ Finset.univ.erase u,
            F u j * ((if p j = t then (1 : ℝ) else 0) - (if p j = p u then (1 : ℝ) else 0)))
        + (∑ i ∈ Finset.univ.erase u,
            F i u * ((if p i = t then (1 : ℝ) else 0) - (if p i = p u then (1 : ℝ) else 0))) := by
  have hmu : move p u t u = t := by simp only [move]; exact Function.update_self u t p
  have hmv : ∀ v : Fin n, v ≠ u → move p u t v = p v := fun v hv => by
    simp only [move]; exact Function.update_of_ne hv t p
  rw [add_assoc, ← sub_eq_iff_eq_add']
  rw [← Finset.sum_sub_distrib]
  simp_rw [← Finset.sum_sub_distrib, ← mul_sub]
  rw [← Finset.add_sum_erase _ _ (Finset.mem_univ u)]
  congr 1
  · -- row u
    rw [← Finset.add_sum_erase _ _ (Finset.mem_univ u)]
    rw [show F u u * ((if move p u t u = move p u t u then (1 : ℝ) else 0)
          - (if p u = p u then (1 : ℝ) else 0)) = 0 by simp, zero_add]
    refine Finset.sum_congr rfl fun j hj => ?_
    rw [hmu, hmv j (Finset.ne_of_mem_erase hj)]
    simp only [eq_comm]
  · -- columns, i ≠ u
    refine Finset.sum_congr rfl fun i hi => ?_
    rw [hmv i (Finset.ne_of_mem_erase hi), ← Finset.add_sum_erase _ _ (Finset.mem_univ u), hmu]
    rw [show (∑ j ∈ Finset.univ.erase u, F i j * ((if p i = move p u t j then (1 : ℝ) else 0)
          - (if p i = p j then (1 : ℝ) else 0))) = 0 from
        Finset.sum_eq_zero fun j hj => by rw [hmv j (Finset.ne_of_mem_erase hj)]; simp, add_zero]

/-- **The directed move-delta identity.** For a real move (`t ≠ p u`), the
    closed-form incremental gain computed by Go's `directed_move.go`
    (`directedModularity.moveDelta`) is exactly the difference of from-scratch
    directed modularities. Holds with no hypothesis on `totalWeight`: on an arcless
    graph both sides are `0` (Lean's `1 / 0 = 0`). The no-op move `t = p u` is
    excluded; Go returns `0` for it before evaluating the formula. The `edge` term is
    `(outWeightTo t − outWeightTo (p u)) + (inWeightFrom t − inWeightFrom (p u))`, the
    `null` term the two degree products; each source community degree drops `u`'s own
    contribution (Go's `koutSrcWithoutU`/`kinSrcWithoutU`). -/
theorem directedModularity_move_eq (G : DirectedWeightedGraph n) (γ : ℝ)
    (p : Partition n) (u : Fin n) (t : ℕ) (ht : t ≠ p u) :
    directedModularity G γ (move p u t)
      = directedModularity G γ p
        + (1 / G.totalWeight) *
            (((G.outWeightTo p u t - G.outWeightTo p u (p u))
                + (G.inWeightFrom p u t - G.inWeightFrom p u (p u)))
              - γ * G.outDegree u *
                  (G.commInDegree p t - (G.commInDegree p (p u) - G.inDegree u)) / G.totalWeight
              - γ * G.inDegree u *
                  (G.commOutDegree p t - (G.commOutDegree p (p u) - G.outDegree u))
                    / G.totalWeight) := by
  have hput : (if p u = t then (1 : ℝ) else 0) = 0 := if_neg fun h => ht h.symm
  have hInDt : (∑ j ∈ Finset.univ.erase u, G.inDegree j * (if p j = t then (1 : ℝ) else 0))
      = G.commInDegree p t := by
    rw [sum_erase_mul_ind_eq G.inDegree p u t, ← G.commInDegree_eq_sum p t, hput, mul_zero,
      sub_zero]
  have hInDs : (∑ j ∈ Finset.univ.erase u, G.inDegree j * (if p j = p u then (1 : ℝ) else 0))
      = G.commInDegree p (p u) - G.inDegree u := by
    rw [sum_erase_mul_ind_eq G.inDegree p u (p u), ← G.commInDegree_eq_sum p (p u), if_pos rfl,
      mul_one]
  have hOutDt : (∑ i ∈ Finset.univ.erase u, G.outDegree i * (if p i = t then (1 : ℝ) else 0))
      = G.commOutDegree p t := by
    rw [sum_erase_mul_ind_eq G.outDegree p u t, ← G.commOutDegree_eq_sum p t, hput, mul_zero,
      sub_zero]
  have hOutDs : (∑ i ∈ Finset.univ.erase u, G.outDegree i * (if p i = p u then (1 : ℝ) else 0))
      = G.commOutDegree p (p u) - G.outDegree u := by
    rw [sum_erase_mul_ind_eq G.outDegree p u (p u), ← G.commOutDegree_eq_sum p (p u), if_pos rfl,
      mul_one]
  have hrow : (∑ j ∈ Finset.univ.erase u,
        (G.weight u j - γ * G.outDegree u * G.inDegree j / G.totalWeight)
          * ((if p j = t then (1 : ℝ) else 0) - (if p j = p u then (1 : ℝ) else 0)))
      = (G.outWeightTo p u t - G.outWeightTo p u (p u))
        - γ * G.outDegree u
            * (G.commInDegree p t - (G.commInDegree p (p u) - G.inDegree u)) / G.totalWeight := by
    rw [G.outWeightTo_eq p u t, G.outWeightTo_eq p u (p u), ← hInDt, ← hInDs,
      ← Finset.sum_sub_distrib, ← Finset.sum_sub_distrib, Finset.mul_sum, Finset.sum_div,
      ← Finset.sum_sub_distrib]
    exact Finset.sum_congr rfl fun j _ => by ring
  have hcol : (∑ i ∈ Finset.univ.erase u,
        (G.weight i u - γ * G.outDegree i * G.inDegree u / G.totalWeight)
          * ((if p i = t then (1 : ℝ) else 0) - (if p i = p u then (1 : ℝ) else 0)))
      = (G.inWeightFrom p u t - G.inWeightFrom p u (p u))
        - γ * G.inDegree u
            * (G.commOutDegree p t - (G.commOutDegree p (p u) - G.outDegree u))
              / G.totalWeight := by
    rw [G.inWeightFrom_eq p u t, G.inWeightFrom_eq p u (p u), ← hOutDt, ← hOutDs,
      ← Finset.sum_sub_distrib, ← Finset.sum_sub_distrib, Finset.mul_sum, Finset.sum_div,
      ← Finset.sum_sub_distrib]
    exact Finset.sum_congr rfl fun i _ => by ring
  have hsplit :
      (∑ i, ∑ j, (G.weight i j - γ * G.outDegree i * G.inDegree j / G.totalWeight)
          * (if move p u t i = move p u t j then (1 : ℝ) else 0))
        = (∑ i, ∑ j, (G.weight i j - γ * G.outDegree i * G.inDegree j / G.totalWeight)
            * (if p i = p j then (1 : ℝ) else 0))
          + (∑ j ∈ Finset.univ.erase u,
              (G.weight u j - γ * G.outDegree u * G.inDegree j / G.totalWeight)
                * ((if p j = t then (1 : ℝ) else 0) - (if p j = p u then (1 : ℝ) else 0)))
          + (∑ i ∈ Finset.univ.erase u,
              (G.weight i u - γ * G.outDegree i * G.inDegree u / G.totalWeight)
                * ((if p i = t then (1 : ℝ) else 0) - (if p i = p u then (1 : ℝ) else 0))) :=
    sum_kernel_move_split
      (fun i j => G.weight i j - γ * G.outDegree i * G.inDegree j / G.totalWeight) p u t
  unfold directedModularity
  rw [hsplit, hrow, hcol]
  ring

end Meso
