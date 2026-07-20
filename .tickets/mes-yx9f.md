---
id: mes-yx9f
status: closed
deps: []
links: []
created: 2026-07-18T11:16:35Z
type: task
priority: 2
assignee: Andre Silva
parent: mes-crz3
tags: [directed, lean, verification, research, phase-1]
---
# Directed-modularity verification phase 1: directed graph model and rational mirror in Lean

## Execution plan: Phase 1 of the directed-modularity verification project

Self-contained plan for an executing agent. It implements Phase 1 ("directed
graph model") of the research plan in
[docs/research/directed-modularity-formal-verification.md](../docs/research/directed-modularity-formal-verification.md):
a Lean model of a directed weighted graph, the real-valued Leicht-Newman
directed modularity definition, its computable rational mirror, and the
equivalence theorem tying the two together. No guarantee proofs, no oracle
wiring, no Go changes.

Everything needed to execute is in this document plus the referenced repo
files. Read the "Reference patterns" section before writing any Lean.

## 1. Context

`meso` is a pure-Go community-detection library whose undirected numeric core
is formally verified in Lean 4 (under `verification/lean/`). The Lean model is
executed as a value-oracle: computable rational mirrors of the proved
definitions emit exact golden vectors that the Go tests assert against.

Directed modularity shipped in Go (`directed_quality.go`,
`directed_move.go`) but is deliberately outside the Lean model: the proved
`WeightedGraph` structure requires a symmetry proof (`weight_symm`) that a
directed graph cannot supply. The research doc lays out a five-phase plan to
close this; Phase 1 builds the directed model and its mirror, which is
deterministic engineering with no open mathematics.

### Phase 1 scope (from the research doc)

- A Lean structure for a directed weighted graph: no symmetry field,
  nonnegative weights, node sizes.
- Directed out-degree, in-degree, and total arc weight `m`.
- The real-valued (noncomputable) directed modularity definition for future
  proofs.
- The computable rational mirror `directedModularityQ` for the value-oracle.
- A proof that the mirror agrees with the real definition on rational inputs
  (the directed analogue of `modularityQ_eq`).

## 2. Repo orientation

| Path | What it is |
| --- | --- |
| `verification/lean/` | The Lean package (`lakefile.toml`, toolchain `leanprover/lean4:v4.31.0`, mathlib pinned `v4.31.0`) |
| `verification/lean/Meso.lean` | Root module: one `import` line per model file; new files must be registered here |
| `verification/lean/Meso/Graph.lean` | Undirected `WeightedGraph` (the structure to mirror, minus symmetry) |
| `verification/lean/Meso/Quality.lean` | `Partition` and the real `modularity` (the definition style to parallel) |
| `verification/lean/Meso/Compute.lean` | `WeightedGraphQ`, `modularityQ`, `toReal`, `ofRaw`, and the F2 equivalence proofs (the mirror pattern to follow) |
| `verification/lean/Meso/Move.lean` | `move` operator used by the move-delta definitions |
| `verification/lean/CORRESPONDENCE.md` | Model-to-Go correspondence and divergence register; must be updated (Task 6) |
| `directed_quality.go` (repo root) | Go directed modularity; the semantics the Lean definitions must match |
| `directed_quality_test.go` (repo root) | Hand-computed fixture values reused as Lean sanity checks (Task 4) |
| `docs/research/directed-modularity-formal-verification.md` | The research plan this executes Phase 1 of |

Build commands (from the repo root):

```sh
make lean       # cd verification/lean && lake build (type-checks everything)
make validate   # Go quality gate; must stay green (this plan touches no Go)
```

## 3. Prerequisites and environment check

1. Lean toolchain: `elan` must provide `leanprover/lean4:v4.31.0` (pinned in
   `verification/lean/lean-toolchain`). Check: `cd verification/lean && lake --version`.
2. Mathlib: already vendored under `verification/lean/.lake/packages/mathlib`.
   If starting from a clean checkout, run `lake exe cache get` inside
   `verification/lean/` first; building mathlib from source takes hours,
   the cache download takes minutes.
3. Confirm the baseline builds before changing anything: `make lean` must
   succeed on the unmodified tree. If it does not, stop and report; do not
   proceed on a broken baseline.

## 4. Fixed design decisions

These are settled; do not revisit them during execution.

### 4.1 The directed model is the undirected structure minus symmetry

At the mathematical level a directed graph needs no separate in-adjacency:
a dense weight function `weight : Fin n → Fin n → ℝ` where `weight i j` is
the weight of the arc `i → j` carries direction by itself. Dropping the
`weight_symm` field IS the directed model. (Separate out/in adjacency lists
are a CSR implementation detail of the Go core, not of the mathematics.)

### 4.2 Degree and total-weight conventions (must match the Go core)

- `outDegree i = ∑_j weight i j` (row sum). A self-loop `weight i i`
  contributes once.
- `inDegree j = ∑_i weight i j` (column sum). A self-loop contributes once.
- `totalWeight = ∑_i outDegree i = ∑_{i,j} weight i j` (the paper's `m`,
  written `T` in the Go comments). Self-loops count once. This equals what
  the Go `csr.twoM()` returns on a directed graph.
- Do not name it `m` in Lean (too short, shadows conventions); use
  `totalWeight`.

### 4.3 The objective (Leicht-Newman directed modularity)

```text
Q = (1 / m) * ∑_{i,j} (w_ij − γ · k_i^out · k_j^in / m) · δ(c_i, c_j)
```

Matches `directedModularity.Quality` in `directed_quality.go` term for term.

### 4.4 Division-by-zero convention

An arcless graph has `totalWeight = 0`. In both ℚ and ℝ, Lean defines
`x / 0 = 0` and `1 / 0 = 0`, so the definition evaluates to `0` with no side
condition, matching the Go early-return (`if total == 0 { return 0 }`) and
matching how the undirected `modularity`/`modularityQ` already handle
`twoM = 0`. Write the definitions with plain division; add no hypotheses.

### 4.5 Standalone model; existing files untouched

Introduce the directed model alongside the undirected one. Do not modify
`WeightedGraph`, `WeightedGraphQ`, or any existing proof. Model unification
is an explicit open question of the research doc, out of scope here.

### 4.6 Naming

- Real side: `DirectedWeightedGraph`, `outDegree`, `inDegree`,
  `totalWeight`, `directedModularity`.
- Rational side: `DirectedWeightedGraphQ`, same member names,
  `directedModularityQ`, `moveDeltaDirectedModularityQ`.
- Equivalence theorems: `directedModularityQ_eq`,
  `moveDeltaDirectedModularityQ_eq`.

### 4.7 Term-for-term parallelism is load-bearing

Write `directedModularityQ` so it is the ℚ transliteration of
`directedModularity`, same shape, same parenthesisation, same `if` indicator.
The equivalence proof then stays mechanical (`unfold`, `simp` with the
`toReal` simp lemmas, `push_cast`, `rfl`), exactly like `modularityQ_eq` in
`Meso/Compute.lean`. If the proof does not close with that pattern, first
suspect a shape mismatch between the two definitions.

### 4.8 `ofRaw` does not symmetrise

The undirected `WeightedGraphQ.ofRaw` folds both orientations with `max`
because it must manufacture symmetry. The directed `ofRaw` must NOT do that:
it only clamps to nonnegative (`max 0 (raw i j)`), preserving asymmetry.

## 5. Non-goals (do not do these)

- No guarantee statements or proofs (connectivity, gamma-separation,
  subset-optimality): Phase 3/4.
- No move-delta *identity* (incremental formula equals difference of
  from-scratch scores): Phase 2. The from-scratch *difference definition*
  (`moveDeltaDirectedModularityQ`) IS in scope; the incremental formula is
  not.
- No symmetric-reduction theorem (directed equals undirected on symmetric
  graphs): Phase 2.
- No aggregation model or invariance: Phase 2.
- No oracle executable changes (`Main.lean`, `Meso/OracleIO.lean`), no golden
  vectors, no `verification/oracle/` changes: Phase 5.
- No Go changes at all.
- No edits to `lakefile.toml`, `lean-toolchain`, or `lake-manifest.json`.
- No git commits, no branching; leave version control to the user.

## 6. Reference patterns

### 6.1 Structure and file conventions

Every Lean file in the package starts with this exact header:

```lean
/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
```

followed by imports, then a module docstring (`/-! # Title ... -/`), then
`namespace Meso ... end Meso`. The linter option
`weak.linter.mathlibStandardSet = true` is on: every structure field and
public `def` needs a docstring (`/-- ... -/`). `relaxedAutoImplicit = false`
is set, so declare `variable {n : ℕ}` explicitly.

### 6.2 The undirected structure being mirrored (from `Meso/Graph.lean`)

```lean
structure WeightedGraph (n : ℕ) where
  weight : Fin n → Fin n → ℝ
  weight_symm : ∀ i j, weight i j = weight j i   -- the field to DROP
  weight_nonneg : ∀ i j, 0 ≤ weight i j
  nodeSize : Fin n → ℝ
  nodeSize_nonneg : ∀ i, 0 ≤ nodeSize i
```

### 6.3 The F2 equivalence pattern (from `Meso/Compute.lean`)

The mirror declares `toReal` plus `@[simp]` cast lemmas for every derived
quantity, and the equivalence proof is:

```lean
theorem modularityQ_eq (G : WeightedGraphQ n) (γ : ℚ) (p : Partition n) :
    (modularityQ G γ p : ℝ) = modularity G.toReal (γ : ℝ) p := by
  unfold modularityQ modularity
  simp only [WeightedGraphQ.toReal_weight, WeightedGraphQ.toReal_degree,
    WeightedGraphQ.toReal_twoM]
  push_cast [apply_ite ((↑) : ℚ → ℝ)]
  rfl
```

The directed proof follows this shape with the directed simp lemma set
(`toReal_weight`, `toReal_outDegree`, `toReal_inDegree`,
`toReal_totalWeight`, `toReal_nodeSize`).

### 6.4 `Partition` and `move`

`Partition n := Fin n → ℕ` lives in `Meso/Quality.lean`. `move p v c` is
`Function.update p v c` in `Meso/Move.lean`. Import them; do not redefine.

## 7. Tasks, in order

### Task 0: baseline

Run `make lean` on the unmodified tree. Record that it passes. Abort with a
report if it does not.

### Task 1: `verification/lean/Meso/DirectedGraph.lean` (real model)

Create the file with the standard header and a module docstring explaining:
the directed weighted graph `meso`'s directed support operates on; that it is
`WeightedGraph` without the symmetry field, so `weight i j` is the arc
`i → j`; that it deliberately shares no code with `WeightedGraph` (model
unification is an open question of the research doc); and that no guarantees
are proved over it yet (they are Phases 3 and 4).

Contents:

1. `structure DirectedWeightedGraph (n : ℕ)` with fields `weight`
   (`Fin n → Fin n → ℝ`, docstring: weight of the arc `i → j`),
   `weight_nonneg`, `nodeSize`, `nodeSize_nonneg`. No symmetry field.
2. In `namespace DirectedWeightedGraph`, with
   `variable {n : ℕ} (G : DirectedWeightedGraph n)`:
   - `def outDegree (i : Fin n) : ℝ := ∑ j, G.weight i j`
   - `def inDegree (j : Fin n) : ℝ := ∑ i, G.weight i j`
   - `def totalWeight : ℝ := ∑ i, G.outDegree i`
   - `lemma outDegree_nonneg`, `lemma inDegree_nonneg`,
     `lemma totalWeight_nonneg` (via `Finset.sum_nonneg`, mirroring
     `degree_nonneg`/`twoM_nonneg` in `Meso/Graph.lean`).
   - `lemma totalWeight_eq_sum_inDegree : G.totalWeight = ∑ j, G.inDegree j`
     (unfold and `Finset.sum_comm`). This pins the out/in bookkeeping and
     will be used repeatedly in Phase 2.
3. After the namespace, the objective (import `Meso.Quality` for
   `Partition`):

   ```lean
   noncomputable def directedModularity (G : DirectedWeightedGraph n)
       (γ : ℝ) (p : Partition n) : ℝ :=
     (1 / G.totalWeight) * ∑ i, ∑ j,
       (G.weight i j - γ * G.outDegree i * G.inDegree j / G.totalWeight) *
         (if p i = p j then (1 : ℝ) else 0)
   ```

   Docstring: the Leicht-Newman directed modularity with resolution `γ`;
   the null model uses separate out- and in-degrees; cite that it matches
   the Go `DirectedModularity`; note the `totalWeight = 0` graph evaluates
   to `0` because Lean division by zero is zero.

Acceptance: `lake build` succeeds; every def and field has a docstring; no
linter warnings.

### Task 2: `verification/lean/Meso/DirectedCompute.lean` (rational mirror)

Create the file (imports: `Meso.DirectedGraph`, `Meso.Move`). Module
docstring: the computable rational mirror of the directed model for the
future directed value-oracle (Phase 5 of the research doc), written term for
term against `directedModularity` so the cast proof stays mechanical;
modelled on `Meso/Compute.lean`.

Contents, paralleling `Compute.lean` exactly:

1. `structure DirectedWeightedGraphQ (n : ℕ)`: same fields as
   `DirectedWeightedGraph` with ℚ in place of ℝ.
2. In its namespace: `outDegree`, `inDegree`, `totalWeight` over ℚ.
3. `noncomputable def toReal : DirectedWeightedGraph n` casting weight and
   nodeSize, discharging `weight_nonneg`/`nodeSize_nonneg` by
   `exact_mod_cast`.
4. `@[simp]` lemmas: `toReal_weight`, `toReal_nodeSize`, `toReal_outDegree`,
   `toReal_inDegree`, `toReal_totalWeight` (the degree/total ones via
   `simp only [...]` with `Rat.cast_sum`, mirroring `toReal_degree` and
   `toReal_twoM`).
5. `def ofRaw (raw : Fin n → Fin n → ℚ) (size : Fin n → ℚ)`:
   clamp only, `weight i j := max 0 (raw i j)`,
   `weight_nonneg i j := le_max_left 0 (raw i j)`, same for `nodeSize`.
   Docstring must state explicitly that, unlike the undirected
   `WeightedGraphQ.ofRaw`, it does NOT symmetrise, and that on already
   nonnegative input it is the identity.
6. `def directedModularityQ (G : DirectedWeightedGraphQ n) (γ : ℚ)
   (p : Partition n) : ℚ`: the ℚ transliteration of `directedModularity`
   (same shape; see decision 4.7).
7. `def moveDeltaDirectedModularityQ (G) (γ) (p) (v : Fin n) (c : ℕ) : ℚ :=
   directedModularityQ G γ (move p v c) - directedModularityQ G γ p`.
   Docstring: the exact directed gain computed the honest slow way, the
   oracle value a future directed golden vector will carry; the incremental
   formula's identity against it is Phase 2.
8. The equivalence theorems:
   - `theorem directedModularityQ_eq (G : DirectedWeightedGraphQ n) (γ : ℚ)
     (p : Partition n) : (directedModularityQ G γ p : ℝ)
     = directedModularity G.toReal (γ : ℝ) p`
     by the F2 pattern (section 6.3).
   - `theorem moveDeltaDirectedModularityQ_eq` casting the difference, by
     `rw [Rat.cast_sub, directedModularityQ_eq, directedModularityQ_eq]`,
     mirroring `moveDeltaModularityQ_eq`.

Acceptance: `lake build` succeeds; `directedModularityQ` and
`moveDeltaDirectedModularityQ` are plain `def`s (NOT `noncomputable`), which
is itself the proof they evaluate; only `toReal` is `noncomputable`; both
equivalence theorems are `sorry`-free.

### Task 3: register the modules

Add to `verification/lean/Meso.lean`, after `import Meso.Compute`:

```lean
import Meso.DirectedGraph
import Meso.DirectedCompute
```

Acceptance: `make lean` builds the whole package.

### Task 4: fixture sanity checks (Lean-side, machine-checked)

The Go tests pin hand-computed values on a 3-node asymmetric fixture
(`directed_quality_test.go`, `asymmetricDirectedCSR`). Reproduce them in
Lean so the Lean definitions are pinned to the same semantics as the Go
code, in a closing section of `DirectedCompute.lean` (or a separate
`Meso/DirectedFixtures.lean` if the file gets long; if separate, register it
in `Meso.lean` too).

The fixture: arcs `0 → 1` weight 2, `0 → 2` weight 1, `1 → 0` weight 1;
no self-loops; unit node sizes. As a raw weight function this is the matrix
(row `i` = arcs out of `i`):

```text
row 0: [0, 2, 1]
row 1: [1, 0, 0]
row 2: [0, 0, 0]
```

Out-degrees `[3, 1, 0]`, in-degrees `[1, 2, 1]`, `totalWeight = 4`.

Exact expected values (all derived by hand in the Go test, restated as
exact rationals):

| Case | γ | Partition | Exact Q |
| --- | --- | --- | --- |
| all-in-one | 1 | `![0, 0, 0]` | `0` |
| singletons | 1 | `![0, 1, 2]` | `-5/16` |
| singletons | 2 | `![0, 1, 2]` | `-5/8` |
| two communities | 1 | `![0, 0, 1]` | `0` |

And one move-delta: from singletons at γ = 1, moving node 1 to community 0
has exact gain `5/16` (it lands on the two-community partition with Q = 0).

Encode each as an `example` over `DirectedWeightedGraphQ`, e.g. built via
`ofRaw` from a `Matrix (Fin 3) (Fin 3) ℚ` literal (`!![0, 2, 1; 1, 0, 0;
0, 0, 0]`) or a `Fin.cons`/pattern-match function, with partitions as
`![0, 0, 0] : Fin 3 → ℕ` literals. Prove by `decide` (ℚ equality is
decidable and the sums are over `Fin 3`). If `decide` is too slow or the
elaborator struggles, fall back in order: `norm_num [directedModularityQ,
DirectedWeightedGraphQ.totalWeight, DirectedWeightedGraphQ.outDegree,
DirectedWeightedGraphQ.inDegree, ...]`, then `native_decide` (last resort;
check the linter set accepts it before keeping it).

Acceptance: at least the four Q values and the one move-delta are
machine-checked `example`s that build. These five checks are what ties the
Lean definitions to the Go semantics until the Phase 5 oracle exists; do
not skip them.

### Task 5: full build and Go gate

- `make lean` from the repo root: full Lean package type-checks.
- `make validate` from the repo root: the Go gate must be untouched-green
  (this plan changes no Go; run it to prove that).

### Task 6: update the correspondence register

`verification/lean/CORRESPONDENCE.md` currently records (in the divergence
register, the bullet beginning "Directed modularity and directed CPM (plan
4.3) are **deliberately not modelled**") that directed is entirely outside
the model. That statement is now partially stale. Update that bullet (do
not delete it; amend it) to record:

- The directed *objective* is now modelled: `DirectedWeightedGraph`,
  `directedModularity`, the rational mirror `directedModularityQ`, and the
  equivalence `directedModularityQ_eq` (Phase 1 of
  `docs/research/directed-modularity-formal-verification.md`).
- The *guarantees* remain unproved and directed remains outside the value
  oracle; the original descope reasoning (symmetry is load-bearing in the
  guarantee proofs; the reference paper states them for the symmetric case
  only) still stands for Phases 3 and 4.
- Directed CPM remains unmodelled by design.

If the correspondence table at the top of the file maps model artifacts to
Go artifacts, add rows for `DirectedWeightedGraph.weight` (Go directed CSR
out/in adjacency), `directedModularity` (Go `DirectedModularity.Quality`),
and `directedModularityQ` (status: landed, oracle wiring pending Phase 5).
Match the file's existing table format and prose style.

After editing, lint it: `markdownlint-cli2 --config ~/.markdownlint.yaml --fix
verification/lean/CORRESPONDENCE.md` and fix anything reported. (Check first
for a project-root markdownlint config; there is none as of this writing, so
the user-global config applies.)

### Task 7: final report

Report to the user: files created, theorem names proved, the fixture checks
and their values, `make lean` and `make validate` output status, and the
CORRESPONDENCE.md edit. List anything skipped or weakened, explicitly. Do
not commit anything.

## 8. Acceptance checklist

- [ ] `make lean` green on the final tree.
- [ ] `make validate` green (no Go touched).
- [ ] `Meso/DirectedGraph.lean`: structure without symmetry field;
      `outDegree`, `inDegree`, `totalWeight`; nonneg lemmas;
      `totalWeight_eq_sum_inDegree`; `noncomputable directedModularity`.
- [ ] `Meso/DirectedCompute.lean`: `DirectedWeightedGraphQ`; `toReal` plus
      simp lemmas; non-symmetrising `ofRaw`; computable
      `directedModularityQ` and `moveDeltaDirectedModularityQ`;
      `directedModularityQ_eq` and `moveDeltaDirectedModularityQ_eq`
      proved without `sorry`.
- [ ] Both files registered in `Meso.lean`.
- [ ] Five fixture `example`s machine-checked (four Q values, one
      move-delta).
- [ ] Every public def/field/theorem has a docstring; GPL header on new
      files; no new linter warnings.
- [ ] CORRESPONDENCE.md divergence bullet amended; markdownlint clean.
- [ ] No changes to: existing Lean files (other than `Meso.lean` imports and
      CORRESPONDENCE.md), Go sources, oracle inputs/goldens, lakefile,
      toolchain.
- [ ] Nothing committed to git.

## 9. Known risks and fallbacks

- **The `push_cast; rfl` proof does not close.** Almost always a shape
  mismatch between `directedModularityQ` and `directedModularity`. Diff the
  two definitions token by token first. If a cast lemma is missing from the
  simp set, add the corresponding `@[simp] toReal_*` lemma rather than
  fighting the main proof. `apply_ite ((↑) : ℚ → ℝ)` handles the indicator.
- **`decide` on the fixture examples is slow or fails to elaborate.** Use
  the fallback ladder in Task 4. If `ofRaw` around a `Matrix` literal
  resists `decide`, construct the fixture graph directly (explicit
  `weight`/`nodeSize` functions with `weight_nonneg` by `decide` or by
  `Fin.cases`), avoiding `max` unfolding entirely.
- **Mathlib linter complaints** (missing docstrings, style): fix them; do
  not disable linters. The lakefile already disables the Apache-header
  linter for GPL reasons; nothing else should need touching.
- **Name clashes inside `namespace Meso`** (e.g. `outDegree` also existing
  elsewhere someday): the definitions live inside
  `namespace DirectedWeightedGraph`/`DirectedWeightedGraphQ`, so they are
  fully qualified; only `directedModularity`, `directedModularityQ`, and
  the theorems sit at `Meso` level, and none of those names exist yet.
- **Long build times.** Incremental `lake build` after the mathlib cache is
  in place takes minutes. If the environment has no cache and no network,
  stop and report rather than attempting a source build of mathlib.

## 10. Open questions (record answers in the final report; do not block)

- Whether the fixture examples belong at the end of `DirectedCompute.lean`
  or in their own `Meso/DirectedFixtures.lean`. Default: same file unless it
  exceeds roughly the size of `Compute.lean`.
- Whether `native_decide` is acceptable under the configured linter set if
  the `decide`/`norm_num` routes both fail. Default: avoid it; report if it
  was needed.
- Whether `totalWeight` should be stated over out-degrees (as specified) or
  as the flat double sum `∑ i, ∑ j, weight i j` with the out-degree form as
  a lemma. Either is fine; keep whichever makes
  `totalWeight_eq_sum_inDegree` and the Phase 2 proofs cleanest, and note
  the choice.

## 11. What Phase 2 will want from this work (context, not tasks)

Phase 2 proves the move-delta identity, aggregation invariance, and the
symmetric reduction (directed Q equals undirected Q on symmetric graphs).
The deliverables here that Phase 2 leans on: `totalWeight_eq_sum_inDegree`
(out/in bookkeeping), the term-for-term parallel mirror (so identities can
be proved once over ℝ and transported), and `moveDeltaDirectedModularityQ`
(the oracle side of the move-delta identity). Keeping definitions simple and
unbundled (no typeclasses, no common interface with the undirected model)
is deliberate: Phase 2 statements stay direct, and unification remains a
separate, explicit decision.

## Notes

**2026-07-18T12:02:53Z**

Phase 1 landed: DirectedGraph.lean (DirectedWeightedGraph, outDegree/inDegree/totalWeight, totalWeight_eq_sum_inDegree, noncomputable directedModularity) and DirectedCompute.lean (DirectedWeightedGraphQ, toReal + simp lemmas, non-symmetrising ofRaw, computable directedModularityQ and moveDeltaDirectedModularityQ, directedModularityQ_eq and moveDeltaDirectedModularityQ_eq, 5 fixture examples). Registered in Meso.lean. make lean and make validate green. CORRESPONDENCE.md divergence bullet amended + 3 section-1 rows. Deviation: fixtures use norm_num not decide (Rat gcd normalisation stalls the kernel); no native_decide. Learnings recorded in docs/specs/learnings.md.
