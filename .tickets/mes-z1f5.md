---
id: mes-z1f5
status: closed
deps: [mes-yx9f]
links: []
created: 2026-07-18T11:46:41Z
type: task
priority: 2
assignee: Andre Silva
parent: mes-crz3
tags: [directed, lean, verification, research, phase-2]
---
# Directed-modularity verification phase 2: objective identities in Lean

## Execution plan: Phase 2 of the directed-modularity verification project

Self-contained plan for an executing agent. It implements Phase 2 ("objective
identities") of the research plan in
[docs/research/directed-modularity-formal-verification.md](../docs/research/directed-modularity-formal-verification.md),
tracked by ticket `mes-z1f5` (parent epic `mes-crz3`): prove, over the Lean
directed model that Phase 1 delivered, the algebraic facts the optimizer relies
on. None of them need the paper guarantees, and all of them are believed-true
pure algebra; this phase contains no open mathematics.

Everything needed to execute is in this document plus the referenced repo
files. Read the "Reference patterns" and "Mathematical appendix" sections
before writing any Lean.

## 1. Context

`meso` is a pure-Go community-detection library whose undirected numeric core
is formally verified in Lean 4 (under `verification/lean/`). Phase 1
(ticket `mes-yx9f`, closed) added the directed model:

- `verification/lean/Meso/DirectedGraph.lean`: `DirectedWeightedGraph n`
  (dense `weight : Fin n → Fin n → ℝ`, no symmetry field, nonnegative weights,
  node sizes), `outDegree` (row sum), `inDegree` (column sum), `totalWeight`
  (`∑ i, outDegree i`), nonnegativity lemmas, `totalWeight_eq_sum_inDegree`,
  and the noncomputable objective `directedModularity`.
- `verification/lean/Meso/DirectedCompute.lean`: the rational mirror
  `DirectedWeightedGraphQ`, `directedModularityQ`,
  `moveDeltaDirectedModularityQ` (honest from-scratch difference), `toReal`
  plus `@[simp]` cast lemmas, the equivalence theorems
  `directedModularityQ_eq` and `moveDeltaDirectedModularityQ_eq`, and five
  fixture `example`s pinned to the Go hand-computed values (via a
  `check_fixture` tactic macro; note `decide` cannot close ℚ fixture goals,
  `norm_num` does).

Phase 2 proves four identities over this model:

1. Basic value facts: `directedModularity` on a zero-weight graph is `0` for
   every partition, and on the all-in-one partition equals `1 − γ` (so `0` at
   `γ = 1`).
2. Symmetric reduction: on a symmetric graph, directed modularity equals
   undirected modularity, tying the new model to the proved one.
3. Directed aggregation invariance: collapsing each community to a super-node
   preserves directed modularity (directed analogue of
   `modularity_aggregate_eq`; operational counterpart is Go's
   `aggregateDirected` in `aggregate.go`).
4. The move-delta identity: the closed-form incremental gain that Go's
   `directed_move.go` computes equals
   `directedModularity(after) − directedModularity(before)` exactly. This is
   the heaviest item and it has NO undirected pattern to copy: the undirected
   model deliberately never proved its closed-form ΔQ (see `Meso/Move.lean`'s
   module docstring, "the heavier algebra ... comes later"); Go's undirected
   incremental formula is validated by property test against the honest
   difference only. Proving the directed closed form therefore goes beyond
   undirected parity. It is in scope because the research plan and ticket
   `mes-z1f5` name it, and because Phase 3's separation and subset-optimality
   triage will be stated over move gains; a documented de-scope fallback is
   defined in Task 6.

## 2. Repo orientation

| Path | What it is |
| --- | --- |
| `verification/lean/` | The Lean package (toolchain `leanprover/lean4:v4.31.0`, mathlib pinned `v4.31.0`) |
| `verification/lean/Meso.lean` | Root module: one `import` per model file; new files must be registered here |
| `verification/lean/Meso/DirectedGraph.lean` | Phase 1 directed model and objective (this phase adds to it) |
| `verification/lean/Meso/DirectedCompute.lean` | Phase 1 rational mirror and fixtures (unchanged this phase) |
| `verification/lean/Meso/Graph.lean` | Undirected `WeightedGraph`, `degree`, `twoM` |
| `verification/lean/Meso/Quality.lean` | `Partition n` and the real `modularity` |
| `verification/lean/Meso/Move.lean` | `move` operator (`Function.update`), local-move monotonicity |
| `verification/lean/Meso/Aggregate.lean` | The undirected aggregation development this phase parallels; also holds partition-level helpers to REUSE |
| `verification/lean/Meso/Separation.lean` | `ind_singletonMerge` and `cpm_merge_two_singletons`: the only existing indicator-change proof, the pattern for Task 6 |
| `verification/lean/CORRESPONDENCE.md` | Model-to-Go correspondence and divergence register; must be updated (Task 8) |
| `directed_move.go` (repo root) | Go incremental directed move-delta; the formula Task 6 proves (its doc comment contains the same derivation) |
| `aggregate.go` (repo root) | Go `aggregateDirected`; the operational counterpart of Task 5 |
| `.tickets/mes-z1f5.md` | The Phase 2 ticket (acceptance criteria this plan discharges) |

Build commands (from the repo root):

```sh
make lean       # cd verification/lean && lake build (type-checks everything)
make validate   # Go quality gate; must stay green (this plan touches no Go)
```

For fast iteration, build a single module from `verification/lean/`:

```sh
lake build Meso.DirectedAggregate   # or Meso.DirectedMove, etc.
```

## 3. Prerequisites and environment check

1. Lean toolchain: `elan` must provide `leanprover/lean4:v4.31.0` (pinned in
   `verification/lean/lean-toolchain`).
2. Mathlib: vendored under `verification/lean/.lake/packages/mathlib`. From a
   clean checkout run `lake exe cache get` inside `verification/lean/` first
   (a source build of mathlib takes hours; the cache takes minutes). If there
   is no cache and no network, stop and report.
3. Confirm the baseline builds: `make lean` must succeed on the unmodified
   tree. Abort with a report if it does not.
4. Read fully before writing any Lean: `Meso/DirectedGraph.lean`,
   `Meso/DirectedCompute.lean`, `Meso/Aggregate.lean`, `Meso/Move.lean`,
   `Meso/Separation.lean` (at least through `cpm_merge_two_singletons`), and
   the doc comment of `directed_move.go`.

## 4. Fixed design decisions

These are settled; do not revisit them during execution.

### 4.1 What is reusable from the undirected development, verbatim

`Meso/Aggregate.lean` contains four artifacts that carry NO symmetry
assumption and must be reused, not re-proved:

- `sum_diagonal_eq_sum_fiber (p) (F : Fin n → Fin n → ℝ)`: the Kronecker-δ
  regrouping of a double sum into per-community block sums, stated for an
  arbitrary kernel `F`. It is the heart of aggregation invariance and works
  unchanged for the asymmetric directed kernel.
- `numComm p`: the aggregate node count.
- `commLabel p A`: the fixed bijection from aggregate nodes to community
  labels.
- `sum_commLabel (p) (g : ℕ → ℝ)`: reindexing a sum over aggregate nodes to a
  sum over present community labels.

Import `Meso.Aggregate` and use them. Do not modify `Aggregate.lean`.

### 4.2 File layout

- Basic value lemmas, the `toDirected` bridge, and the symmetric reduction go
  INTO `Meso/DirectedGraph.lean` (the directed model's home; Phase 1's "do
  not touch existing files" rule protected the undirected files, not the
  directed ones, which this phase owns).
- Aggregation gets a new file `Meso/DirectedAggregate.lean` (imports
  `Meso.DirectedGraph`, `Meso.Aggregate`).
- The move-delta identity gets a new file `Meso/DirectedMove.lean` (imports
  `Meso.DirectedGraph`, `Meso.Move`).
- Both new files are registered in `Meso.lean` after
  `import Meso.DirectedCompute`.
- `Meso/DirectedCompute.lean` is NOT modified (no new mirror definitions are
  needed this phase; the oracle wiring is Phase 5).

### 4.3 Naming

Top-level names mirror the `directedModularity` pattern (undirected name with
a `directed` prefix); community-relative quantities live in the
`DirectedWeightedGraph` namespace for dot notation:

- Task 3 (in `DirectedGraph.lean`): `directedModularity_of_totalWeight_eq_zero`,
  `directedModularity_const`, `directedModularity_const_one`,
  `WeightedGraph.toDirected` (with `@[simp]` lemmas `toDirected_weight`,
  `toDirected_nodeSize`, `toDirected_outDegree`, `toDirected_inDegree`,
  `toDirected_totalWeight`), `directedModularity_toDirected_eq`.
- Task 5 (in `DirectedAggregate.lean`): `directedBlockWeight`,
  `directedAggregate`, `directedAggregate_outDegree`,
  `directedAggregate_inDegree`, `directedAggregate_totalWeight`,
  `directed_block_contribution`, `directedModularity_eq_communitySum`,
  `directedModularity_aggregate_eq`.
- Task 6 (in `DirectedMove.lean`): `DirectedWeightedGraph.outWeightTo`,
  `DirectedWeightedGraph.inWeightFrom`, `DirectedWeightedGraph.commOutDegree`,
  `DirectedWeightedGraph.commInDegree`, `sum_kernel_move_split`,
  `directedModularity_move_eq`.

### 4.4 The move identity requires `t ≠ p u` and nothing else

The closed form holds for every graph including `totalWeight = 0` (both sides
collapse to `0` because Lean's `1 / 0 = 0`), so state NO hypothesis on
`totalWeight`. It does NOT hold for the no-op move `t = p u` (the null term
would spuriously contribute `−2γ·k_u^out·k_u^in/m²`), so the hypothesis
`t ≠ p u` is required; Go's early `if target == src { return 0 }` is the
operational counterpart. Record this correspondence in the docstring.

### 4.5 Definitions used in statements are `noncomputable def`s over ℝ

`outWeightTo`, `inWeightFrom`, `commOutDegree`, `commInDegree`,
`directedBlockWeight`, `directedAggregate` are proof-side real-valued
definitions (like the undirected `blockWeight`/`aggregate`). No ℚ mirrors
this phase: the oracle consumes only `directedModularityQ` and
`moveDeltaDirectedModularityQ`, which exist.

### 4.6 Style

Every new file carries the exact 5-line GPL header used by every Lean file in
the package, then imports, then a `/-! # ... -/` module docstring. Every
public def, field, lemma, and theorem gets a docstring (the
`mathlibStandardSet` linter is on). `relaxedAutoImplicit = false`: declare
`variable {n : ℕ}` explicitly. Do not use em-dashes in new comments; use
hyphens, colons, or parentheses. Mathlib lemma names drift between versions:
verify every candidate name suggested below by grepping
`verification/lean/.lake/packages/mathlib/` before relying on it, and prefer
patterns already used in this repo (listed per task).

## 5. Non-goals (do not do these)

- No guarantee statements or proofs (connectivity, gamma-separation,
  subset-optimality): Phases 3 and 4. In particular do NOT state the directed
  analogue of `cpm_merge_two_singletons` as a guarantee; if Task 6 lands, the
  merge gain is its trivial specialisation and Phase 3 will derive it.
- No `IsLocalMove` instances or monotonicity plumbing for directed: the
  generic `move_best_ge` in `Move.lean` is quality-function-agnostic and
  already covers `directedModularity`; nothing to add.
- No changes to `Meso/DirectedCompute.lean`, no new ℚ mirrors, no oracle or
  golden-vector changes (Phase 5), no Go changes.
- No changes to any undirected Lean file (`Graph`, `Quality`, `Move`,
  `Aggregate`, `Compute`, ...). Reuse is by import only.
- No model unification (`WeightedGraph.toDirected` is a bridge, not an
  interface; keep it a plain constructor).
- No git commits, no branching, no ticket status changes; leave version
  control and ticket lifecycle to the user.

## 6. Reference patterns

### 6.1 The undirected aggregation proof chain (`Meso/Aggregate.lean`)

The directed development in Task 5 mirrors this chain one for one:

| Undirected artifact | Directed analogue (Task 5) | Notes |
| --- | --- | --- |
| `sum_diagonal_eq_sum_fiber` | reused verbatim | kernel-generic, no symmetry |
| `modularity_eq_communitySum` | `directedModularity_eq_communitySum` | same `congr 1; simp_rw [mul_ite ...]` shape |
| `blockWeight` | `directedBlockWeight` | identical definition, `DirectedWeightedGraph` argument |
| `aggregate` | `directedAggregate` | one FEWER proof obligation: no `weight_symm` field to discharge |
| `aggregate_degree` | `directedAggregate_outDegree` and `directedAggregate_inDegree` | two lemmas; the in-degree one needs the transposed regroup (see appendix A.3) |
| `aggregate_twoM` | `directedAggregate_totalWeight` | same `sum_commLabel` + `sum_fiberwise_of_maps_to` plumbing |
| `block_contribution` | `directed_block_contribution` | same `ring`-closing proof with `kout_i · kin_j` in place of `k_i · k_j` |
| `modularity_aggregate_eq` | `directedModularity_aggregate_eq` | same final assembly, with TWO degree rewrites instead of one |

Key mathlib pieces the undirected proofs use (all already exercised in this
repo, so they exist at the pinned version): `Finset.sum_comm`,
`Finset.sum_filter`, `Finset.sum_fiberwise_of_maps_to`,
`Finset.mem_image_of_mem`, `Finset.sum_sub_distrib`, `Finset.sum_mul_sum`,
`Finset.mul_sum`, `Finset.sum_div`, `Finset.sum_eq_single`,
`Equiv.sum_comp`, `Finset.sum_coe_sort`.

### 6.2 The indicator-change pattern (`Meso/Separation.lean`)

`ind_singletonMerge` proves an indicator decomposition for a move on the
singleton partition with `simp only [move, Function.update_apply,
Fin.ext_iff]` then `split_ifs <;> first | (exfalso; omega) | norm_num`. Task
6's `sum_kernel_move_split` needs the general-partition analogue; the same
`Function.update_apply` + `split_ifs` discipline closes the pointwise cases
(`omega` is unavailable for `ℕ`-valued community labels compared with `=`,
plain `simp_all`/contradiction handles them).

### 6.3 The equivalence-proof pattern (`Meso/DirectedCompute.lean`)

Not needed this phase (no new mirrors), but if a proof accidentally needs a
cast identity, follow `directedModularityQ_eq`: `unfold`, `simp only` with
the `toReal_*` simp set, `push_cast [apply_ite ((↑) : ℚ → ℝ)]`, `rfl`.

## 7. Tasks, in order

Ordered by increasing proof risk, so partial completion still lands value.

### Task 0: baseline

Run `make lean` on the unmodified tree. Record that it passes. Abort with a
report if it does not.

### Task 1: read the references

Read the files listed in section 3 item 4. Confirm the Phase 1 names in this
plan match the tree (they were verified at plan-writing time; if the tree has
drifted, adapt and note the drift in the final report).

### Task 2: trivial totalWeight lemma (in `DirectedGraph.lean`)

Add to the `DirectedWeightedGraph` namespace:

```lean
/-- `totalWeight` as the flat double sum `∑_{i,j} w_{ij}`. -/
lemma totalWeight_eq_sum_sum : G.totalWeight = ∑ i, ∑ j, G.weight i j := rfl
```

(`totalWeight` unfolds to `∑ i, outDegree i` which unfolds to the double sum;
if `rfl` does not close it, `by unfold totalWeight outDegree` then `rfl`.)
Task 3 and the appendix derivations use it.

### Task 3: basic value lemmas (in `DirectedGraph.lean`)

After `directedModularity`, add:

```lean
/-- On a graph with no arc weight, directed modularity is `0` for every
    partition and resolution: the leading `1 / m` factor is `1 / 0 = 0`. -/
lemma directedModularity_of_totalWeight_eq_zero
    (G : DirectedWeightedGraph n) (γ : ℝ) (p : Partition n)
    (h : G.totalWeight = 0) : directedModularity G γ p = 0 := by
  unfold directedModularity
  rw [h]
  simp
```

```lean
/-- **The all-in-one partition scores `1 − γ`** (on a graph with arcs): every
    pair is within-community, so the edge term sums to `m` and the null term
    to `γ · m` (`∑_i k_i^out = ∑_j k_j^in = m`). At `γ = 1` this is `0`,
    matching the Go fixture case and the undirected convention that one
    community carries no structure signal. -/
theorem directedModularity_const (G : DirectedWeightedGraph n) (γ : ℝ)
    (h : G.totalWeight ≠ 0) (c : ℕ) :
    directedModularity G γ (fun _ => c) = 1 - γ := ...
```

Proof sketch: `unfold directedModularity`; the indicator is uniformly `1`
(`if_pos rfl`, via `simp only`), leaving
`(1/m) * ∑ i, ∑ j, (w i j − γ * kout i * kin j / m)`. Split with
`Finset.sum_sub_distrib`; identify `∑∑ w = m` by `totalWeight_eq_sum_sum`;
factor the null sum into `γ * (∑ i, kout i) * (∑ j, kin j) / m` with
`Finset.sum_mul_sum` / `Finset.mul_sum` / `Finset.sum_div` (exactly the
`block_contribution` manipulation in `Aggregate.lean`, with `s = univ`);
rewrite `∑ kout = m` (definition) and `∑ kin = m`
(`totalWeight_eq_sum_inDegree`); close with `field_simp [h]` and `ring`.

Add the corollary:

```lean
/-- At γ = 1 the all-in-one partition scores exactly `0`. -/
theorem directedModularity_const_one (G : DirectedWeightedGraph n)
    (h : G.totalWeight ≠ 0) (c : ℕ) :
    directedModularity G 1 (fun _ => c) = 0 := by
  rw [directedModularity_const G 1 h c]; ring
```

Acceptance: `lake build Meso.DirectedGraph` green, docstrings on all three.

### Task 4: the symmetric bridge and reduction (in `DirectedGraph.lean`)

The bridge, in the `WeightedGraph` namespace (an undirected graph IS a
symmetric directed graph; the constructor just forgets the symmetry proof):

```lean
/-- View an undirected graph as a (symmetric) directed graph: same weight
    function, symmetry forgotten. The bridge the symmetric-reduction theorem
    is stated across; a constructor, not a model unification. -/
def WeightedGraph.toDirected (G : WeightedGraph n) : DirectedWeightedGraph n where
  weight := G.weight
  weight_nonneg := G.weight_nonneg
  nodeSize := G.nodeSize
  nodeSize_nonneg := G.nodeSize_nonneg
```

`@[simp]` lemmas: `toDirected_weight`, `toDirected_nodeSize` (both `rfl`);
`toDirected_outDegree : G.toDirected.outDegree i = G.degree i` (`rfl`: both
are `∑ j, G.weight i j`); `toDirected_inDegree : G.toDirected.inDegree j =
G.degree j` (NOT `rfl`: needs `weight_symm`, via
`Finset.sum_congr rfl fun i _ => G.weight_symm i j`);
`toDirected_totalWeight : G.toDirected.totalWeight = G.twoM` (`rfl` should
work since both unfold to the same double sum through definitionally equal
degree sums; fall back to `unfold` + `sum_congr`).

The reduction theorem:

```lean
/-- **Symmetric reduction.** On a symmetric graph, Leicht-Newman directed
    modularity coincides with undirected modularity: out- and in-degree both
    collapse to the degree and `m = 2m`. The directed model is a conservative
    extension of the proved undirected one; the Go counterpart is
    `TestDirectedModularity_SymmetricReducesToUndirected`. -/
theorem directedModularity_toDirected_eq (G : WeightedGraph n) (γ : ℝ)
    (p : Partition n) :
    directedModularity G.toDirected γ p = modularity G γ p := by
  unfold directedModularity modularity
  simp only [toDirected_weight, toDirected_outDegree, toDirected_inDegree,
    toDirected_totalWeight]
```

If the trailing goal is not closed by the `simp only`, finish with `rfl`.

Acceptance: `lake build Meso.DirectedGraph` green. This theorem is the
model-consistency anchor: cite it in the CORRESPONDENCE.md update (Task 8).

### Task 5: directed aggregation invariance (new file `Meso/DirectedAggregate.lean`)

Create the file (GPL header; imports `Meso.DirectedGraph`, `Meso.Aggregate`).
Module docstring: the directed analogue of `Meso/Aggregate.lean`; reuses its
partition-level helpers (`numComm`, `commLabel`, `sum_commLabel`) and its
kernel-generic regrouping (`sum_diagonal_eq_sum_fiber`); the Go operational
counterpart is `aggregateDirected` (`aggregate.go`); note the self-loop
convention correspondence (below).

Contents, mirroring the undirected chain (section 6.1):

1. `directedModularity_eq_communitySum`: apply `sum_diagonal_eq_sum_fiber`
   with the directed kernel
   `F i j = G.weight i j − γ * G.outDegree i * G.inDegree j / G.totalWeight`.
   Copy the `modularity_eq_communitySum` proof, adjusting names.
2. `directedBlockWeight G p a b := ∑ i ∈ filter (p · = a), ∑ j ∈ filter
   (p · = b), G.weight i j` (noncomputable def). Docstring: for a directed
   graph the block is ORDERED, `blockWeight a b ≠ blockWeight b a` in
   general; the diagonal block `a a` counts each internal arc once, which is
   exactly the self-loop Go's `aggregateDirected` folds (each directed arc is
   stored once), versus the undirected model where the diagonal block counts
   each off-diagonal internal edge twice.
3. `directedAggregate G p : DirectedWeightedGraph (numComm p)`:
   `weight A B := directedBlockWeight G p (commLabel p A) (commLabel p B)`;
   `weight_nonneg` by double `Finset.sum_nonneg`; `nodeSize A` the fiber sum
   of `G.nodeSize`; `nodeSize_nonneg` by `Finset.sum_nonneg`. There is no
   `weight_symm` field: the directed structure makes the directed aggregate
   STRICTLY EASIER to construct than the undirected one.
4. `directedAggregate_outDegree`: the aggregate's out-degree at `A` is the
   fiber sum of members' out-degrees. Mirror `aggregate_degree` (its proof
   regroups the inner sum over all nodes by their community via
   `sum_commLabel` + `Finset.sum_fiberwise_of_maps_to`).
5. `directedAggregate_inDegree`: the transposed statement, fiber sum of
   members' in-degrees. Same skeleton with the roles of the two indices
   swapped; see appendix A.3 for the exact sum manipulation. This lemma has
   no undirected counterpart (symmetry made one lemma suffice); it is new but
   mechanical.
6. `directedAggregate_totalWeight`: mirrors `aggregate_twoM`, summing
   `directedAggregate_outDegree` over aggregate nodes and regrouping.
7. `directed_block_contribution`: for any `s : Finset (Fin n)`,
   `∑ i ∈ s, ∑ j ∈ s, (w i j − γ kout i *kin j / m) = (∑∑ w) − γ* ((∑ kout)
   - (∑ kin)) / m`. Copy`block_contribution`'s proof verbatim (it is pure
   `sum_mul_sum`/`mul_sum`/`sum_div` plumbing plus `ring`; nothing used
   symmetry).
8. The payoff:

```lean
/-- **Directed aggregation invariance.** Running the singleton partition on
    the directed aggregate yields exactly the directed modularity of `p` on
    the original graph, so a Louvain/Leiden recursion on a directed graph may
    continue on the aggregate without losing quality. Directed analogue of
    `modularity_aggregate_eq`; Go counterpart `aggregateDirected`. -/
theorem directedModularity_aggregate_eq (G : DirectedWeightedGraph n) (γ : ℝ)
    (p : Partition n) :
    directedModularity (directedAggregate G p) γ (fun A => (A : ℕ))
      = directedModularity G γ p := ...
```

Copy the `modularity_aggregate_eq` assembly: rewrite by
`directedModularity_eq_communitySum`, unfold the aggregate's objective,
rewrite `directedAggregate_totalWeight`, reindex with `sum_commLabel`, kill
the off-diagonal indicator with the same `Finset.sum_eq_single A` step
(unchanged: the singleton-partition indicator logic is graph-agnostic), then
`directed_block_contribution` + `directedAggregate_outDegree` +
`directedAggregate_inDegree` + `ring`.

Acceptance: `lake build Meso.DirectedAggregate` green; every def/lemma
docstringed; no modification to `Aggregate.lean`.

### Task 6: the move-delta identity (new file `Meso/DirectedMove.lean`)

Create the file (GPL header; imports `Meso.DirectedGraph`, `Meso.Move`).
Module docstring: the closed-form directed move gain, the identity Go's
`directed_move.go` implements; state the `t ≠ p u` requirement and the Go
early-return correspondence; note this identity has no undirected
counterpart in the model (the undirected Go formula is property-tested only)
and record why the directed one is proved (Phase 3 consumes move gains).

Read appendix A before starting: it derives the identity and pins the exact
correspondence to `directed_move.go`, so the statement below is known-true on
paper; the work is only Lean mechanics.

Definitions (all `noncomputable def` in the `DirectedWeightedGraph`
namespace, each with a docstring naming its Go counterpart from
`directed_move.go`):

```lean
/-- Arc weight from `u` into community `c` under `p`, self-loop excluded:
    Go's `woutTarget`/`woutSrc`. -/
noncomputable def outWeightTo (G : DirectedWeightedGraph n) (p : Partition n)
    (u : Fin n) (c : ℕ) : ℝ :=
  ∑ j ∈ (Finset.univ.erase u).filter (fun j => p j = c), G.weight u j

/-- Arc weight from community `c` into `u` under `p`, self-loop excluded:
    Go's `winTarget`/`winSrc`. -/
noncomputable def inWeightFrom ... : ℝ :=
  ∑ i ∈ (Finset.univ.erase u).filter (fun i => p i = c), G.weight i u

/-- Summed out-degree of community `c`: Go's `koutTarget`/`koutSrc`. -/
noncomputable def commOutDegree (G : DirectedWeightedGraph n) (p : Partition n)
    (c : ℕ) : ℝ :=
  ∑ i ∈ Finset.univ.filter (fun i => p i = c), G.outDegree i

/-- Summed in-degree of community `c`: Go's `kinTarget`/`kinSrc`. -/
noncomputable def commInDegree ... : ℝ :=
  ∑ i ∈ Finset.univ.filter (fun i => p i = c), G.inDegree i
```

The workhorse lemma (kernel-generic, so the partition logic is proved once,
separate from the modularity algebra):

```lean
/-- **How a single move perturbs an indicator-weighted double sum.** For any
    kernel `F` (independent of the partition), moving `u` from `s = p u` to
    `t ≠ s` changes `∑_{i,j} F i j · δ` only on row `u` and column `u`, minus
    the fixed diagonal: each off-`u` cell in row `u` swaps its indicator from
    `χ(p j = s)` to `χ(p j = t)`, and symmetrically for column `u`. -/
lemma sum_kernel_move_split (F : Fin n → Fin n → ℝ) (p : Partition n)
    (u : Fin n) (t : ℕ) (ht : t ≠ p u) :
    (∑ i, ∑ j, F i j * (if move p u t i = move p u t j then (1:ℝ) else 0))
      = (∑ i, ∑ j, F i j * (if p i = p j then (1:ℝ) else 0))
        + (∑ j ∈ Finset.univ.erase u, F u j *
            ((if p j = t then (1:ℝ) else 0) - (if p j = p u then (1:ℝ) else 0)))
        + (∑ i ∈ Finset.univ.erase u, F i u *
            ((if p i = t then (1:ℝ) else 0) - (if p i = p u then (1:ℝ) else 0)))
```

Proof strategy (decompose; do not attempt one big `split_ifs`):

- Row split: for the outer sum use `Finset.add_sum_erase` (or
  `Finset.sum_eq_add_sum_diff_singleton`; verify the exact name by grepping
  mathlib) to peel `i = u`, on both sides.
- Pointwise facts, each a tiny lemma proved by
  `simp [move, Function.update_apply]` with `split_ifs` and the `ht`/`hne`
  hypotheses:
  (a) for `i ≠ u`, `j ≠ u`: `move p u t i = p i` and `move p u t j = p j`,
  indicator unchanged;
  (b) `move p u t u = t` (`Function.update_self` or `Function.update_same`;
  verify name);
  (c) row `u`, `j ≠ u`: indicator is `if t = p j`; rewrite to `if p j = t`
  via `eq_comm` (`if_congr (eq_comm ...)` or `simp [eq_comm]`);
  (d) the diagonal cell `(u, u)` is `1` before and after (both `if rfl`),
  so it cancels in the difference.
- Peel `j = u` inside the `i ∈ erase u` sum the same way for the column
  terms, and regroup with `Finset.sum_add_distrib` /
  `Finset.sum_sub_distrib` and `ring_nf`.

Then the main theorem:

```lean
/-- **The directed move-delta identity.** For a real move (`t ≠ p u`), the
    closed-form incremental gain computed by Go's `directed_move.go` is
    exactly the difference of from-scratch directed modularities. Holds with
    no hypothesis on `totalWeight`: on an arcless graph both sides are `0`
    (Lean's `1 / 0 = 0`). The no-op move `t = p u` is excluded; Go returns
    `0` for it before evaluating the formula. -/
theorem directedModularity_move_eq (G : DirectedWeightedGraph n) (γ : ℝ)
    (p : Partition n) (u : Fin n) (t : ℕ) (ht : t ≠ p u) :
    directedModularity G γ (move p u t)
      = directedModularity G γ p
        + (1 / G.totalWeight) *
            (((G.outWeightTo p u t - G.outWeightTo p u (p u))
                + (G.inWeightFrom p u t - G.inWeightFrom p u (p u)))
              - γ * G.outDegree u *
                  (G.commInDegree p t
                    - (G.commInDegree p (p u) - G.inDegree u)) / G.totalWeight
              - γ * G.inDegree u *
                  (G.commOutDegree p t
                    - (G.commOutDegree p (p u) - G.outDegree u)) / G.totalWeight)
```

Assembly (appendix A.2 has the full paper derivation):

1. `unfold directedModularity`; both sides share the factor
   `1 / G.totalWeight`; reduce to the double-sum difference via
   `sum_kernel_move_split` applied at the directed kernel
   `F i j = G.weight i j − γ * G.outDegree i * G.inDegree j / G.totalWeight`
   (the kernel does not depend on `p`, which is the whole reason the split
   applies; say so in a comment).
2. Convert the two erase-sums to the named quantities. Each splits linearly
   (`Finset.sum_sub_distrib`, `Finset.mul_sum`) into a weight part and a
   degree part:
   - weight part over `{j ≠ u, p j = c}` is `outWeightTo`/`inWeightFrom`
     by definition (the erase-filter shape matches; keep the definitions in
     exactly the `(univ.erase u).filter` form above so this is `rfl`-ish);
   - degree part: `∑_{j ≠ u, p j = t} G.inDegree j = G.commInDegree p t`
     because `u` is not in the `t` fiber (`p u ≠ t` from `ht`; lemma:
     `Finset.filter_erase`/`Finset.erase_eq_of_not_mem` route, or sum over
     `filter` minus a term the guard excludes), and
     `∑_{j ≠ u, p j = p u} G.inDegree j = G.commInDegree p (p u) −
     G.inDegree u` because `u` IS in its own fiber (candidate:
     `Finset.sum_erase_eq_sub`; verify name, else `Finset.add_sum_erase`
     rearranged).
3. Finish with `field_simp`-free `ring_nf` (everything is already over a
   common shape; avoid `field_simp` since `totalWeight` may be zero).

Fallback (invoke only after a genuine attempt, and document in the report):
if the general `sum_kernel_move_split` will not close after decomposing as
above, DESCOPE to the singleton-partition specialisation
`directedModularity_merge_two_singletons` (the directed analogue of
`cpm_merge_two_singletons`, whose `ind_singletonMerge` pattern is proven
technology), which is the minimum Phase 3 needs; record the general identity
as an explicitly stated `theorem ... := sorry`-FREE omission (that is: do not
commit a `sorry`; state the descope in the module docstring, the final
report, and a `tk add-note` on `mes-z1f5`). The ticket's acceptance is then
renegotiated by the user, not silently weakened.

Acceptance: `lake build Meso.DirectedMove` green; the identity (or the
documented descope) in place; `grep -rn "sorry" verification/lean/Meso/`
returns nothing.

### Task 7: register the new modules

Add to `verification/lean/Meso.lean`, after `import Meso.DirectedCompute`:

```lean
import Meso.DirectedAggregate
import Meso.DirectedMove
```

Acceptance: `make lean` builds the whole package.

### Task 8: update the correspondence register

In `verification/lean/CORRESPONDENCE.md`:

- Amend the directed divergence bullet (the one updated by Phase 1) to
  record: the directed objective identities are now proved
  (`directedModularity_toDirected_eq`, `directedModularity_aggregate_eq`,
  `directedModularity_move_eq`, `directedModularity_const`); the guarantees
  (connectivity, γ-separation, subset-optimality) remain unproved pending
  Phase 3 triage; directed remains outside the value-oracle until Phase 5;
  directed CPM remains unmodelled by design.
- If the correspondence table maps theorems to Go artifacts, add rows:
  `directedModularity_aggregate_eq` ↔ `aggregateDirected` (`aggregate.go`),
  `directedModularity_move_eq` ↔ `directedModularity.moveDelta`
  (`directed_move.go`) noting the Go incremental formula is now backed by a
  proved identity (an upgrade from property-test-only),
  `directedModularity_toDirected_eq` ↔
  `TestDirectedModularity_SymmetricReducesToUndirected`.
- Match the file's existing table format and prose style; no em-dashes.

Then lint: check for a project-root markdownlint config first (none exists
as of this writing, so the user-global config applies):
`markdownlint-cli2 --config ~/.markdownlint.yaml --fix
verification/lean/CORRESPONDENCE.md`, and fix anything reported.

### Task 9: ticket note

Append an implementation note to the Phase 2 ticket (matching the repo's
timestamped-notes convention, e.g. `.tickets/mes-pgah.md`):

```sh
tk add-note mes-z1f5 "<summary: files added, theorems proved, any descope>"
```

Do NOT change the ticket status; the user closes tickets.

### Task 10: full validation and axiom hygiene

- `make lean` from the repo root: green.
- `grep -rn "sorry" verification/lean/Meso/`: no output.
- Optional but recommended: in a scratch buffer (do not commit), check
  `#print axioms directedModularity_aggregate_eq` and
  `#print axioms directedModularity_move_eq`; expected axioms are at most
  `propext`, `Classical.choice`, `Quot.sound` (no `native_decide`, no
  `sorryAx`). Remove the scratch lines afterwards.
- `make validate` from the repo root: the Go gate must be untouched-green.

### Task 11: final report

Report: files created/modified, every theorem name proved, whether the Task
6 fallback was used (and exactly what was descoped), `make lean` /
`make validate` status, the CORRESPONDENCE.md edit, and the ticket note. List
anything skipped or weakened, explicitly. Do not commit anything.

## 8. Mathematical appendix (read before Task 5 and Task 6)

### A.1 Conventions recap

`m = totalWeight = ∑_i kout_i = ∑_j kin_j` (Phase 1's
`totalWeight_eq_sum_inDegree`). Self-loops count once in `kout_i`, once in
`kin_i`, once in `m`. The objective:
`Q(p) = (1/m) ∑_{i,j} (w_{ij} − γ kout_i kin_j / m) · χ(p i = p j)`.

### A.2 Derivation of the move identity (Task 6)

Move `u` from `s = p u` to `t ≠ s`. The kernel
`F i j = w_{ij} − γ kout_i kin_j / m` does not depend on `p`; only the
indicator changes, and only where `i = u` or `j = u`:

- Cell `(u, u)`: indicator `1` before and after. No contribution.
- Row `u`, `j ≠ u`: indicator `χ(s = p j)` before, `χ(t = p j)` after.
- Column `u`, `i ≠ u`: symmetric.
- All other cells: unchanged.

So `m · (Q(after) − Q(before))` equals

```text
  ∑_{j ≠ u} F u j · (χ(p j = t) − χ(p j = s))
+ ∑_{i ≠ u} F i u · (χ(p i = t) − χ(p i = s))
```

Expand the row sum with `F u j = w_{uj} − γ kout_u kin_j / m`:

```text
  ∑_{j ≠ u, p j = t} w_{uj} − ∑_{j ≠ u, p j = s} w_{uj}          -- Wout_t − Wout_s
− (γ kout_u / m) · (∑_{j ≠ u, p j = t} kin_j − ∑_{j ≠ u, p j = s} kin_j)
```

Since `p u = s ≠ t`, node `u` is not in the `t` fiber, so
`∑_{j ≠ u, p j = t} kin_j = Kin_t` (the full community in-degree), while `u`
is in its own fiber, so `∑_{j ≠ u, p j = s} kin_j = Kin_s − kin_u`. The row
sum is therefore
`(Wout_t − Wout_s) − γ kout_u (Kin_t − (Kin_s − kin_u)) / m`, and the column
sum symmetrically
`(Win_t − Win_s) − γ kin_u (Kout_t − (Kout_s − kout_u)) / m`. Dividing by `m`
gives exactly the theorem statement, and exactly Go's
`(edge − null) / total` with `edge = (woutTarget − woutSrc) + (winTarget −
winSrc)` and `null = γ·koutU·(kinTarget − kinSrcWithoutU)/total +
γ·kinU·(koutTarget − koutSrcWithoutU)/total` (`directed_move.go`). The same
derivation appears in that file's doc comment; the Lean theorem is its
machine-checked form.

Correspondence details worth pinning in docstrings: Go's `woutTarget` sums
`g.neighbors(u)` which never contains `u` (self-loops are stored separately
in the CSR), matching the `erase u` in `outWeightTo`; Go's `koutTarget` sums
`outDegree(v)` over ALL `v` in the community including self-loop
contributions, matching `commOutDegree` over the full fiber.

### A.3 The aggregate in-degree regroup (Task 5, item 5)

`(directedAggregate G p).inDegree B = ∑_A directedBlockWeight (label A)
(label B)`. Push `sum_commLabel` through the OUTER index this time: the sum
over `A` of block weights into `B` regroups, via
`Finset.sum_fiberwise_of_maps_to` applied to the FIRST index's community
membership, into `∑_{i} ∑_{j ∈ fiber B} w i j` restricted appropriately,
which is `∑_{j ∈ fiber B} G.inDegree j` after `Finset.sum_comm`. Mirror
`aggregate_degree`'s proof and swap the roles of the two indices; the
`sum_comm` that the undirected proof already performs is the template.

### A.4 Why the constant-partition value is `1 − γ` (Task 3)

All pairs are within-community:
`m · Q = ∑∑ w − (γ/m)(∑ kout)(∑ kin) = m − (γ/m)·m·m = m(1 − γ)`, so
`Q = 1 − γ` when `m ≠ 0`. The undirected model has no such lemma; adding it
directed-only is intentional (the research plan lists it), and at `γ = 1` it
generalises the fixture check `directedModularityQ fixture 1 ![0,0,0] = 0`.

## 9. Acceptance checklist

- [ ] `make lean` green on the final tree; `make validate` green (no Go
      touched).
- [ ] `grep -rn "sorry" verification/lean/Meso/` returns nothing.
- [ ] `DirectedGraph.lean` additions: `totalWeight_eq_sum_sum`,
      `directedModularity_of_totalWeight_eq_zero`,
      `directedModularity_const`, `directedModularity_const_one`,
      `WeightedGraph.toDirected` + simp lemmas,
      `directedModularity_toDirected_eq`.
- [ ] New `DirectedAggregate.lean`: `directedBlockWeight`,
      `directedAggregate` (no symmetry obligation),
      `directedAggregate_outDegree`, `directedAggregate_inDegree`,
      `directedAggregate_totalWeight`, `directed_block_contribution`,
      `directedModularity_eq_communitySum`,
      `directedModularity_aggregate_eq`. `Aggregate.lean` helpers reused,
      not re-proved; `Aggregate.lean` unmodified.
- [ ] New `DirectedMove.lean`: `outWeightTo`, `inWeightFrom`,
      `commOutDegree`, `commInDegree`, `sum_kernel_move_split`,
      `directedModularity_move_eq` with hypothesis `t ≠ p u` and no
      `totalWeight` hypothesis (OR the documented singleton descope per Task
      6's fallback, with no `sorry` anywhere).
- [ ] Both new files registered in `Meso.lean`.
- [ ] Every public def/lemma/theorem docstringed; GPL header on new files; no
      new linter warnings; no em-dashes in new comments.
- [ ] CORRESPONDENCE.md updated and markdownlint-clean.
- [ ] `tk add-note mes-z1f5` appended; ticket status untouched.
- [ ] No changes to: undirected Lean files, `DirectedCompute.lean`, Go
      sources, oracle inputs/goldens, lakefile, toolchain. Nothing committed.

## 10. Known risks and fallbacks

- **`sum_kernel_move_split` is the riskiest proof.** Mitigations are built
  into Task 6: kernel-generic statement (partition logic isolated from
  modularity algebra), row/column decomposition into pointwise lemmas, the
  `Function.update_apply` + `split_ifs` discipline from
  `ind_singletonMerge`, and the documented singleton descope as a last
  resort. Budget the most time here.
- **Mathlib name drift.** Candidate names in this plan
  (`Finset.add_sum_erase`, `Finset.sum_erase_eq_sub`,
  `Function.update_self`/`update_same`, `Finset.sum_add_distrib`) must be
  verified against the pinned mathlib by grep before use; the names actually
  exercised in this repo (section 6.1 list) are safe.
- **`rfl` expectations may not hold** (`totalWeight_eq_sum_sum`,
  `toDirected_totalWeight`): definitional unfolding through two `def` layers
  usually reduces, but if not, `unfold` + `Finset.sum_congr` closes them;
  do not restructure definitions to force `rfl`.
- **The final `ring` in `directedModularity_aggregate_eq` or
  `..._move_eq` fails to close.** Usually a factor placement mismatch
  (`γ * a * b / m` vs `γ * (a * b) / m`). Normalise with `ring_nf` on both
  sides first, or adjust the statement's parenthesisation to match the
  definition's; the statement shapes given above copy the definitions'
  shapes deliberately.
- **Long build times.** Iterate per-module (`lake build Meso.DirectedMove`);
  a full `make lean` only at task boundaries. If the environment has no
  mathlib cache and no network, stop and report.

## 11. Open questions (record answers in the final report; do not block)

- Whether `directedModularity_move_eq` should also be stated as a
  `moveDeltaDirectedModularityQ` corollary on the ℚ side (cast the closed
  form through `moveDeltaDirectedModularityQ_eq`). Default: skip; the real
  identity plus the existing cast theorem already pin the oracle value, and
  Phase 5 can add the ℚ corollary if the golden format wants it.
- Whether the merge-gain specialisation
  (`directedModularity_merge_two_singletons`, the directed analogue of
  `cpm_merge_two_singletons`) should be derived NOW as a corollary of the
  move identity, ready for Phase 3. Default: skip unless it falls out in
  under an hour; Phase 3 owns its own statements.
- Whether `WeightedGraph.toDirected` should get a ℚ-side twin
  (`WeightedGraphQ.toDirectedQ`) for future oracle cross-checks. Default:
  skip; note it as a Phase 5 candidate.

## 12. What Phase 3 will want from this work (context, not tasks)

Phase 3 (guarantee triage) states directed γ-separation and
subset-optimality as sign conditions on move gains at converged partitions,
and scouts them for counterexamples. It will lean on: the move identity (or
its singleton specialisation) for merge-gain closed forms on the aggregate,
`directedModularity_aggregate_eq` for the level-stability argument (the
undirected separation proof runs through aggregate-level single-node moves,
see `Meso/Separation.lean`), and `directedModularity_toDirected_eq` to
sanity-check every directed conjecture against its proved undirected
specialisation (a directed statement that fails on symmetric graphs is
wrong). Keep statements unbundled and hypothesis-light so Phase 3 can quote
them directly.

## Notes

**2026-07-18T14:41:31Z**

Phase 2 landed. DirectedGraph.lean: totalWeight_eq_sum_sum, directedModularity_of_totalWeight_eq_zero, directedModularity_const (1-gamma), directedModularity_const_one, WeightedGraph.toDirected + simp lemmas, directedModularity_toDirected_eq (symmetric reduction). New Meso/DirectedAggregate.lean: directedModularity_eq_communitySum, directedBlockWeight, directedAggregate (no symmetry obligation), directedAggregate_outDegree/inDegree/totalWeight, directed_block_contribution, directedModularity_aggregate_eq. New Meso/DirectedMove.lean: outWeightTo/inWeightFrom/commOutDegree/commInDegree + readout lemmas, sum_erase_mul_ind_eq, sum_kernel_move_split (kernel-generic, needs no t-hypothesis), directedModularity_move_eq (full closed-form move-delta identity, hyp t != p u, no totalWeight hyp). Both new files registered in Meso.lean. NO descope: the full move identity is proved, not the singleton fallback. make lean + make validate green; grep sorry empty; #print axioms of aggregate_eq/move_eq/sum_kernel_move_split = [propext, Classical.choice, Quot.sound] only (no sorryAx/native_decide). CORRESPONDENCE.md divergence bullet amended + 4 theorem-to-test rows. Status left open (user closes).
