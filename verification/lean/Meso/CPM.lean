/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.Move
import Meso.Aggregate
import Meso.Level

/-!
# The Constant Potts Model quality function

The second launch quality function (plan section 4.2). Where modularity penalises
a within-community pair by `γ k_i k_j / 2m` (a degree-based, graph-size-dependent
null model), the Constant Potts Model (CPM) penalises it by `γ s_i s_j` — a flat,
node-size-based term with no `2m` normalisation:

`Q_CPM = ∑_{i,j} (w_{ij} − γ s_i s_j) · δ(c_i, c_j)`.

This is why the model carries `nodeSize` from the start (see `Meso.Graph`). CPM is
where the paper's γ-guarantees are cleanest: its per-community contribution is
`e_c − γ S_c²` (internal weight minus γ times the squared community size), so
"γ-dense" and "γ-separated" are stated directly in terms of γ.

Two reuses keep this from duplicating the modularity development:

* **Local-move monotonicity** is already generic over any quality function
  (`move_best_ge`, `IsLocalMove`, `localMoveRun_monotone`), so CPM inherits it by
  instantiation — no new sign argument.
* **The community-sum regrouping** is the kernel-generic `sum_diagonal_eq_sum_fiber`,
  applied here to the CPM summand exactly as `modularity_eq_communitySum` applies it
  to the modularity summand.
-/

namespace Meso

variable {n : ℕ}

/-- The Constant Potts Model quality with resolution `γ`: within-community pairs
    contribute `w_{ij} − γ s_i s_j`, a flat node-size penalty rather than
    modularity's degree-based one. Noncomputable (over ℝ), like `modularity`. -/
noncomputable def cpm (G : WeightedGraph n) (γ : ℝ) (p : Partition n) : ℝ :=
  ∑ i, ∑ j,
    (G.weight i j - γ * G.nodeSize i * G.nodeSize j) * (if p i = p j then (1 : ℝ) else 0)

/-- **A best single-node move never lowers CPM.** CPM instance of the generic
    `move_best_ge`: staying put is always an option. -/
theorem cpm_bestMove_ge (G : WeightedGraph n) (γ : ℝ) (p : Partition n) (v : Fin n)
    (S : Finset ℕ) (c : ℕ) (hcurr : p v ∈ S)
    (hmax : ∀ c' ∈ S, cpm G γ (move p v c') ≤ cpm G γ (move p v c)) :
    cpm G γ p ≤ cpm G γ (move p v c) :=
  move_best_ge p v S c hcurr hmax

/-- **A CPM local-move sweep is monotone.** CPM instance of the generic
    `localMoveRun_monotone`: the whole fast phase inherits monotonicity for CPM with
    no re-proof, via the shared `IsLocalMove (cpm G γ)` interface. -/
theorem cpm_localMoveRun_monotone (G : WeightedGraph n) (γ : ℝ) (ps : List (Partition n))
    (h : ps.IsChain (IsLocalMove (cpm G γ))) :
    ps.Pairwise (fun a b => cpm G γ a ≤ cpm G γ b) :=
  localMoveRun_monotone (cpm G γ) ps h

/-- A block sum of `w_{ij} − γ f_i f_j` over a set factors into the block weight
    minus `γ` times the product of the marginal `f`-sums. This is the sum algebra
    behind the per-community CPM contribution (the node-size cousin of
    `block_contribution`, which carries modularity's `/2m`). -/
lemma block_sub_sq (γ : ℝ) (s : Finset (Fin n)) (w : Fin n → Fin n → ℝ) (f : Fin n → ℝ) :
    ∑ i ∈ s, ∑ j ∈ s, (w i j - γ * f i * f j)
      = (∑ i ∈ s, ∑ j ∈ s, w i j) - γ * ((∑ i ∈ s, f i) * (∑ j ∈ s, f j)) := by
  simp_rw [Finset.sum_sub_distrib]
  congr 1
  rw [Finset.sum_mul_sum, Finset.mul_sum]
  refine Finset.sum_congr rfl fun i _ => ?_
  rw [Finset.mul_sum]
  refine Finset.sum_congr rfl fun j _ => ?_
  ring

/-- Community `c`'s internal edge weight: the block sum of `w` over node pairs both
    assigned to `c`. -/
noncomputable def communityInternalWeight (G : WeightedGraph n) (p : Partition n) (c : ℕ) : ℝ :=
  ∑ i ∈ Finset.univ.filter (fun i => p i = c),
    ∑ j ∈ Finset.univ.filter (fun j => p j = c), G.weight i j

/-- Community `c`'s size: the sum of the node sizes of its members. -/
noncomputable def communitySize (G : WeightedGraph n) (p : Partition n) (c : ℕ) : ℝ :=
  ∑ i ∈ Finset.univ.filter (fun i => p i = c), G.nodeSize i

/-- **CPM is a sum of per-community contributions `e_c − γ S_c²`.** Each community's
    term is its internal weight minus `γ` times its squared size — the form in which
    the γ-guarantees are stated. Reuses `sum_diagonal_eq_sum_fiber` (the shared
    regrouping) and `block_sub_sq` (the node-size block algebra). -/
theorem cpm_eq_communitySum (G : WeightedGraph n) (γ : ℝ) (p : Partition n) :
    cpm G γ p
      = ∑ c ∈ Finset.univ.image p,
          (communityInternalWeight G p c - γ * communitySize G p c ^ 2) := by
  unfold cpm
  simp_rw [mul_ite, mul_one, mul_zero]
  rw [sum_diagonal_eq_sum_fiber p (fun i j => G.weight i j - γ * G.nodeSize i * G.nodeSize j)]
  refine Finset.sum_congr rfl fun c _ => ?_
  rw [block_sub_sq]
  unfold communityInternalWeight communitySize
  ring

/-- **CPM aggregation invariance.** Running the singleton partition (each aggregate
    node its own community) on the aggregate graph yields exactly the CPM of `p` on
    the original graph — the CPM twin of `modularity_aggregate_eq`, and simpler for
    want of the `2m` normalisation. Each aggregate node's self-loop `weight A A`
    carries its community's internal weight and its `nodeSize` carries the community's
    summed size, so the aggregate's diagonal CPM term `weight A A − γ · nodeSize A²`
    is exactly the community contribution `e_c − γ S_c²`; summing over aggregate nodes
    reindexes (`sum_commLabel`) to the community sum of `cpm_eq_communitySum`.
    Aggregation neither raises nor lowers CPM, so a Leiden/Louvain recursion may
    continue on the aggregate without losing CPM. -/
theorem cpm_aggregate_eq (G : WeightedGraph n) (γ : ℝ) (p : Partition n) :
    cpm (aggregate G p) γ (fun A => (A : ℕ)) = cpm G γ p := by
  rw [cpm_eq_communitySum G γ p]
  simp only [cpm]
  rw [← sum_commLabel p fun c => communityInternalWeight G p c - γ * communitySize G p c ^ 2]
  refine Finset.sum_congr rfl fun A _ => ?_
  rw [Finset.sum_eq_single A
      (fun b _ hb => by rw [if_neg fun h => hb (Fin.val_injective h).symm]; exact mul_zero _)
      (fun h => absurd (Finset.mem_univ A) h),
    if_pos rfl, mul_one]
  simp only [aggregate, blockWeight]
  unfold communityInternalWeight communitySize
  ring

/-- `le_aggregate_run` specialised to CPM (aggregation invariance via
    `cpm_aggregate_eq`): one level of the recursion does not lower CPM. -/
theorem le_cpm_aggregate_run
    (G : WeightedGraph n) (γ : ℝ) (p : Partition n)
    (qs : List (Partition (numComm p))) (q : Partition (numComm p))
    (hchain : (((fun A => (A : ℕ)) : Partition (numComm p)) :: qs).IsChain
                (IsLocalMove (cpm (aggregate G p) γ)))
    (hmem : q ∈ ((fun A => (A : ℕ)) : Partition (numComm p)) :: qs) :
    cpm G γ p ≤ cpm (aggregate G p) γ q :=
  le_aggregate_run (fun _ H r => cpm H γ r)
    (fun G p => cpm_aggregate_eq G γ p) G p qs q hchain hmem

/-- **The algorithm's output never lowers CPM.** The CPM instance of
    `qualityFamily_le_of_algorithmRun`, discharging aggregation invariance with
    `cpm_aggregate_eq`: from an initial `(G, p)` to any final iterated-aggregate
    `(G', q)`, `cpm G γ p ≤ cpm G' γ q`. CPM now joins the whole-run monotonicity
    story modularity has (`modularity_le_of_algorithmRun`). -/
theorem cpm_le_of_algorithmRun {m k : ℕ} (γ : ℝ)
    (G : WeightedGraph m) (p : Partition m) (G' : WeightedGraph k) (q : Partition k)
    (h : Relation.ReflTransGen (LevelStep (fun _ H r => cpm H γ r))
          ⟨m, G, p⟩ ⟨k, G', q⟩) :
    cpm G γ p ≤ cpm G' γ q :=
  qualityFamily_le_of_algorithmRun (fun _ H r => cpm H γ r)
    (fun G p => cpm_aggregate_eq G γ p) G p G' q h

/-- **γ-density of a community.** Community `c` is γ-dense when its internal weight
    covers the resolution term `γ S_c²`; equivalently its CPM contribution
    `e_c − γ S_c²` is nonnegative (`isGammaDense_iff`). This is the CPM form of "the
    community is internally well-knit relative to γ", the notion the γ-connectivity
    guarantee is built on. -/
def IsGammaDense (G : WeightedGraph n) (γ : ℝ) (p : Partition n) (c : ℕ) : Prop :=
  γ * communitySize G p c ^ 2 ≤ communityInternalWeight G p c

/-- A community is γ-dense iff its CPM contribution is nonnegative. -/
lemma isGammaDense_iff (G : WeightedGraph n) (γ : ℝ) (p : Partition n) (c : ℕ) :
    IsGammaDense G γ p c
      ↔ 0 ≤ communityInternalWeight G p c - γ * communitySize G p c ^ 2 := by
  rw [IsGammaDense, ← sub_nonneg]

end Meso
