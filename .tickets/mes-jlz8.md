---
id: mes-jlz8
status: closed
deps: [mes-qmch]
links: []
created: 2026-07-18T11:46:41Z
type: task
priority: 2
assignee: Andre Silva
parent: mes-crz3
tags: [directed, lean, verification, research, phase-4]
---
# Directed-modularity verification phase 4: formalize the surviving guarantees

## Execution plan: Phase 4 of the directed-modularity verification project

Self-contained plan for an executing agent. It implements Phase 4 ("formalize the
survivors") of the research plan in
[docs/research/directed-modularity-formal-verification.md](../docs/research/directed-modularity-formal-verification.md),
tracked by ticket `mes-jlz8` (parent epic `mes-crz3`). It consumes the Phase 3
triage report `docs/research/directed-modularity-triage.md`.

Phase 4 turns the triage verdicts into Lean. Three of the survivors are, per the
triage, mechanical: their sub-lemmas are already in the tree and are
graph-agnostic. The scoping decisions the triage deferred have been made by the
user and are fixed in this plan (Section 4).

## 1. Context and inputs

`meso` is a pure-Go community-detection library whose undirected numeric core is
formally verified in Lean 4 (`verification/lean/`). Phases 1-3 delivered:

- **Phase 1-2 assets** (proved, in `verification/lean/Meso/`):
  `DirectedWeightedGraph`, `directedModularity`, `directedModularity_const`,
  `WeightedGraph.toDirected`, `directedModularity_toDirected_eq`
  (`DirectedGraph.lean`); `directedAggregate` with
  `directedAggregate_outDegree`/`_inDegree`/`_totalWeight`,
  `directed_block_contribution`, `directedModularity_aggregate_eq`
  (`DirectedAggregate.lean`); `outWeightTo`, `inWeightFrom`, `commOutDegree`,
  `commInDegree`, `sum_kernel_move_split`, `directedModularity_move_eq`
  (`DirectedMove.lean`); the rational mirror and equivalence theorems
  (`DirectedCompute.lean`).
- **Phase 3 verdicts** (`docs/research/directed-modularity-triage.md`):

  | Guarantee | Verdict | Phase 4 action |
  | --- | --- | --- |
  | Weak connectivity | PROVABLE (mechanical transfer) | Formalize (Task 3) |
  | Strong connectivity | REFUTED by design | Record only (Task 7) |
  | γ-separation (given level stability) | PROVABLE (mechanical) | Formalize (Task 4) |
  | Subset-optimality as output property | REFUTED (n=4 fixture) | Record + regression test (Task 6, 7) |
  | Subset-optimality (given subset stability) | Provable-shaped | Formalize (Task 5) — user chose to do it |

The two gain closed forms are already derived and numerically verified in the
triage (its section 3); this plan formalizes them, it does not re-derive them.

## 2. User decisions fixed for this phase

1. **Formalize the subset-optimality conditional** (Task 5), despite its hypothesis
   not being attained by the algorithm. It is the phase's most involved proof, but
   its sub-lemmas are graph-agnostic and reusable (Section 5.5).
2. **Bool-mirror predicates and all oracle wiring are Phase 5, not here.** Phase 4
   produces real-valued Lean *theorems* only. Do NOT add computable `...Q` mirror
   predicates or touch `Meso/OracleIO.lean`/`Main.lean`.
3. **The subset-split limitation is documented, and a separate investigation is
   opened.** Document it in the CORRESPONDENCE divergence register and as a Go
   characterization test (Task 6). The separate investigation into whether meso's
   refinement should be strengthened is ticket `mes-niic` (already created; affects
   the undirected core too). Do NOT surface it in user-facing docs (`doc.go`) in
   this phase.

## 3. Repo orientation

| Path | What it is |
| --- | --- |
| `verification/lean/Meso.lean` | Root module; register new files here after `import Meso.DirectedMove` |
| `verification/lean/Meso/Convergence.lean` | `IsLocalMoveStable` (graph-agnostic, REUSE), `IsLevelStable`/`IsConverged`/`QualityFamily` (undirected, do not touch) |
| `verification/lean/Meso/Connectivity.lean` | `WeightedGraph.simpleGraph`, `CommunityConnected`, `ConnectedCommunities`, `connectedCommunities_singleton` (the templates for Task 3) |
| `verification/lean/Meso/Refinement.lean` | `mergeCommunities`, `MergeStep`, `MergeStep.connectedCommunities`, `connectedCommunities_of_mergeRun`, `RefineStep`, `refineRun_isMergeRun`, `connectedCommunities_of_refineRun`; also the graph-agnostic `connected_induce_union_of_adj`. Zero `weight_symm` uses (verified) |
| `verification/lean/Meso/Separation.lean` | `ind_singletonMerge` (graph-agnostic, REUSE), `cpm_merge_two_singletons`, `gammaSeparated_of_converged` (the templates for Task 4) |
| `verification/lean/Meso/SubsetOptimality.lean` | `moveSubset`, `ind_subsetSplit`, `sum_sum_mul_ite_mem`, `block_sub_prod` (all graph-agnostic, REUSE), `IsSubsetStable`, `cpm_moveSubset_split`, `isSubsetOptimal_of_stable` (templates for Task 5) |
| `verification/lean/Meso/Directed{Graph,Aggregate,Move,Compute}.lean` | Phase 1-2 directed assets |
| `verification/lean/CORRESPONDENCE.md` | Divergence register; update (Task 7) |
| `directed_quality_test.go`, `directed_move_test.go` | Go directed tests; the n=4 fixture regression test (Task 6) sits alongside |
| `docs/research/directed-modularity-triage.md` | The triage report this phase formalizes |
| `.tickets/mes-jlz8.md` | The Phase 4 ticket (acceptance this plan discharges) |
| `.tickets/mes-niic.md` | The separate refinement-investigation ticket (cross-reference, do not work it here) |

Build commands (from repo root): `make lean` (type-check all), `make validate`
(Go gate). Fast iteration from `verification/lean/`: `lake build Meso.DirectedSeparation`.

## 4. Key insight: the survivors are mechanical because the sub-lemmas are graph-agnostic

Confirmed by inspection of the tree, and the reason the triage rated these
PROVABLE:

- **Connectivity** (`Refinement.lean`, `Connectivity.lean`): uses only
  `G.simpleGraph.Adj` and partition/fiber combinatorics; `weight_symm` appears
  nowhere in `Refinement.lean`. `connected_induce_union_of_adj` is stated over an
  abstract `SimpleGraph V`. So the transfer is: define the directed `simpleGraph`,
  restate the definitions and the merge-run chain over `DirectedWeightedGraph`, and
  the proofs port with `WeightedGraph → DirectedWeightedGraph` and
  `G.simpleGraph → directed simpleGraph` substitutions.
- **γ-separation** (`Separation.lean`): `ind_singletonMerge` is over
  `Partition`/`Fin` with no graph; reuse it directly. The merge lemma pushes the
  directed kernel through, exactly as `cpm_merge_two_singletons` pushes the CPM
  kernel, but the directed algebra is already packaged in
  `directedModularity_move_eq`.
- **Subset conditional** (`SubsetOptimality.lean`): `ind_subsetSplit`,
  `sum_sum_mul_ite_mem`, `block_sub_prod` are all graph-agnostic; import and reuse
  them. Only the directed-kernel assembly (the `1/m` factor and `kout·kin`) is new,
  parallel to `cpm_moveSubset_split`.

So no guarantee needs a new proof *idea*; the work is restatement plus directed
kernel bookkeeping. Budget the most time on Task 5 (the subset identity assembly),
but expect it mechanical, not open.

## 5. Deliverables and design decisions

### 5.1 File layout (all new; existing files untouched except `Meso.lean` and CORRESPONDENCE)

- `Meso/DirectedConvergence.lean` — directed stability definitions (Task 2).
- `Meso/DirectedConnectivity.lean` — weak connectivity (Task 3).
- `Meso/DirectedSeparation.lean` — γ-separation (Task 4).
- `Meso/DirectedSubsetOptimality.lean` — the subset conditional (Task 5).
- Register all four in `Meso.lean` after `import Meso.DirectedMove`, in dependency
  order (Convergence, then Connectivity, Separation, SubsetOptimality).
- Optional `Meso/DirectedGuarantees.lean` bundling the survivors, parallel to
  `Meso/Guarantees.lean` (Section 5.6; low priority).

Do NOT modify any undirected Lean file, the Phase 1-2 directed files,
`DirectedCompute.lean`, the lakefile, or the toolchain.

### 5.2 Directed stability definitions (standalone; triage section 7)

Per the triage's recommendation and open question 5, do NOT introduce a shared
`QualityFamily` typeclass over both graph types. Reuse the graph-agnostic
`IsLocalMoveStable` (it constrains `Q : Partition n → ℝ`), instantiated at
`directedModularity G γ`. Add standalone directed variants:

- `IsDirectedLevelStable (G) (γ) (p) : Prop :=
  IsLocalMoveStable (directedModularity (directedAggregate G p) γ) (fun A => (A : ℕ))`
  (no aggregate-singleton merge improves — the directed analogue of `IsLevelStable`).
- `IsDirectedConverged (G) (γ) (p) : Prop :=
  IsLocalMoveStable (directedModularity G γ) p ∧ IsDirectedLevelStable G γ p`.
- `IsDirectedSubsetStable (G) (γ) (p) : Prop` — no `moveSubset` improves
  `directedModularity`; the directed analogue of `IsSubsetStable`
  (`SubsetOptimality.lean`). Needed by Task 5.

### 5.3 Naming

Mirror the undirected names with a `directed` prefix / `Directed` infix:
`directedModularity_merge_two_singletons`, `directedGammaSeparated_of_levelStable`,
`directedGammaSeparated_blockWeight_of_levelStable` (if the community-cut form is
wanted), `DirectedWeightedGraph.simpleGraph`, `DirectedCommunityConnected`
(or reuse the name in the directed namespace), `directedConnectedCommunities_of_refineRun`,
`directedModularity_moveSubset_split`, `directedSubsetGammaDense_of_subsetStable`.

### 5.4 Statements to prove (exact, from the triage)

Notation: `m = totalWeight`; `Kout_C = ∑_{i∈C} kout_i`, `Kin_C = ∑_{i∈C} kin_i`;
`e(A,B) = ∑_{i∈A,j∈B} w_{ij}`.

- **Weak connectivity** (Task 3): every community of a directed refine-run output
  induces a connected subgraph of `DirectedWeightedGraph.simpleGraph :=
  SimpleGraph.fromRel (fun i j => 0 < G.weight i j)` — the directed analogue of
  `connectedCommunities_of_refineRun`.
- **γ-separation** (Task 4): `IsDirectedLevelStable G γ p → ∀ C D, C ≠ D (both
  occupied) → e(C,D) + e(D,C) ≤ (γ/m)·(Kout_C·Kin_D + Kout_D·Kin_C)`.
- **Subset conditional** (Task 5): `IsDirectedSubsetStable G γ p → ∀ C, ∀ S ⊊ C
  nonempty, T = C\S → e(S,T) + e(T,S) ≥ (γ/m)·(Kout_S·Kin_T + Kout_T·Kin_S)`.

### 5.5 Subset-move identity (the one new assembly, Task 5)

The directed split closed form (triage section 3):

```text
gain_split(S) = -(1/m)·[ e(S,T) + e(T,S) - (γ/m)·(Kout_S·Kin_T + Kout_T·Kin_S) ]
```

Derivation (record in the file docstring): splitting `S` to a fresh label flips the
same-community indicator at exactly `S×T` and `T×S` (`ind_subsetSplit`, reused
directly). Pushing the directed kernel `F i j = w_{ij} − γ·kout_i·kin_j/m` through
that indicator change and factoring the degree products with `block_sub_prod`
(reused) and `sum_sum_mul_ite_mem` (reused) gives the closed form, exactly as
`cpm_moveSubset_split` does for the CPM kernel. The `1/m` factor rides along from
`directedModularity`. The conditional theorem is then "`gain_split(S) ≤ 0`
rearranged," discharged from `IsDirectedSubsetStable`.

### 5.6 Optional bundle

If time permits, `Meso/DirectedGuarantees.lean` conjoining weak connectivity +
γ-separation (+ the subset conditional), parallel to `Meso/Guarantees.lean`. Low
priority; skip if it adds friction. Note it cannot mirror the undirected bundle's
CPM hypotheses; it uses the directed stability defs from 5.2.

## 6. Non-goals (do not do these)

- No Bool-mirror predicates (`...Q`), no `Meso/OracleIO.lean` or `Main.lean`
  changes, no golden vectors: all Phase 5 (user decision 2).
- No user-facing doc changes (`doc.go`): the subset limitation is internal +
  investigation-tracked this phase (user decision 3).
- No work on the refinement investigation itself (`mes-niic`): only cross-reference
  it.
- No changes to undirected Lean files, Phase 1-2 directed files,
  `DirectedCompute.lean`, lakefile, toolchain.
- No `sorry` anywhere, including no "conjecture stubs": everything stated in a
  committed file is proved (the subset conditional included, per user decision 1).
- No git commits, no ticket status changes.

## 7. Tasks, in order

Ordered by increasing effort; every Lean task ends green before the next.

### Task 0: baseline

`make lean` and `make validate` green on the unmodified tree. Read the triage
report end to end, and the template files in Section 3. Abort with a report if the
baseline is not green.

### Task 1: read and confirm the assets

Confirm the Phase 1-2 directed names in Section 1 exist as written (verified at
plan time; adapt and note any drift). Confirm the graph-agnostic sub-lemmas
(`ind_singletonMerge`, `ind_subsetSplit`, `sum_sum_mul_ite_mem`, `block_sub_prod`,
`connected_induce_union_of_adj`) are importable and take no `WeightedGraph`
argument.

### Task 2: directed stability definitions (`Meso/DirectedConvergence.lean`)

GPL header; imports `Meso.DirectedGraph`, `Meso.DirectedAggregate`,
`Meso.Convergence` (for `IsLocalMoveStable`), `Meso.SubsetOptimality` (for
`moveSubset`). Define `IsDirectedLevelStable`, `IsDirectedConverged`,
`IsDirectedSubsetStable` (Section 5.2), each docstringed with its undirected
analogue named. Add the trivial adequacy lemmas the later files need (e.g.
`IsDirectedConverged.localMoveStable`, `.levelStable`), mirroring
`IsConverged.localMoveStable`. Register in `Meso.lean`. Acceptance:
`lake build Meso.DirectedConvergence` green.

### Task 3: weak connectivity (`Meso/DirectedConnectivity.lean`)

Imports `Meso.DirectedGraph`, `Meso.Connectivity`, `Meso.Refinement`. Steps
(triage section 4 sketch):

1. `DirectedWeightedGraph.simpleGraph := SimpleGraph.fromRel (fun i j => 0 < G.weight i j)`.
   Docstring: `fromRel` symmetrises and drops loops, so adjacency is
   `(0 < w_ij ∨ 0 < w_ji) ∧ i ≠ j`, the weak-connectivity edge set; textually
   identical to the undirected `simpleGraph`, but here the symmetrisation does the
   work.
2. Restate `CommunityConnected`, `ConnectedCommunities`,
   `communityConnected_of_subsingleton`, `connectedCommunities_singleton` over
   `DirectedWeightedGraph` (they mention only `simpleGraph` and fibers).
3. Restate the merge-run chain from `Refinement.lean`: `MergeStep`, its
   `.connectedCommunities`, `connectedCommunities_of_mergeRun`, `RefineStep`,
   `refineRun_isMergeRun`, `connectedCommunities_of_refineRun`, substituting the
   directed graph and its `simpleGraph`. Proofs port verbatim (no `weight_symm`);
   where a proof calls `connected_induce_union_of_adj`, reuse it directly (abstract
   over `SimpleGraph`).

Acceptance: `lake build Meso.DirectedConnectivity` green; the top theorem is the
directed `connectedCommunities_of_refineRun`. Register in `Meso.lean`.

### Task 4: γ-separation (`Meso/DirectedSeparation.lean`)

Imports `Meso.DirectedAggregate`, `Meso.DirectedMove`, `Meso.DirectedConvergence`,
`Meso.Separation` (for `ind_singletonMerge`). Steps:

1. `directedModularity_merge_two_singletons (H : DirectedWeightedGraph m) (γ) (A B,
   A ≠ B)`: from the singleton partition, moving `A` into `B`'s community raises
   `directedModularity` by `(1/m)·[w_{AB} + w_{BA} − (γ/m)(kout_A·kin_B +
   kout_B·kin_A)]`. Prove by reusing `ind_singletonMerge` and pushing the directed
   kernel, mirroring `cpm_merge_two_singletons`; or by instantiating
   `directedModularity_move_eq` at the singleton partition. Prefer whichever closes
   cleaner; the `ind_singletonMerge` route parallels the undirected proof exactly.
2. `directedGammaSeparated_of_levelStable`: apply step 1 on `directedAggregate G p`
   (communities are single nodes there), rewrite the aggregate degrees/weight with
   `directedAggregate_outDegree`/`_inDegree`/`_totalWeight` and the aggregate edge
   with `directedBlockWeight`, and read `gain ≤ 0` off `IsDirectedLevelStable`.
   Mirrors `gammaSeparated_of_converged`.
3. Optional community-cut restatement `directedGammaSeparated_blockWeight_of_levelStable`
   if the `e(C,D)+e(D,C)` form is wanted explicitly (parallels
   `gammaSeparated_blockWeight_of_converged`).

Acceptance: `lake build Meso.DirectedSeparation` green. Register in `Meso.lean`.

### Task 5: subset conditional (`Meso/DirectedSubsetOptimality.lean`)

Imports `Meso.DirectedGraph`, `Meso.DirectedConvergence`, `Meso.SubsetOptimality`
(for `moveSubset`, `ind_subsetSplit`, `sum_sum_mul_ite_mem`, `block_sub_prod`).
Steps:

1. `directedModularity_moveSubset_split`: the split closed form (Section 5.5),
   proved by reusing `ind_subsetSplit` and pushing the directed kernel with
   `sum_sum_mul_ite_mem` + `block_sub_prod`, parallel to `cpm_moveSubset_split`.
2. `directedSubsetGammaDense_of_subsetStable`: from `IsDirectedSubsetStable`,
   `gain_split(S) ≤ 0` gives the bound `e(S,T)+e(T,S) ≥ (γ/m)(Kout_S·Kin_T +
   Kout_T·Kin_S)` for every proper nonempty `S ⊆ C`. Mirror
   `cpm_subsetGammaDense_of_stable` / `isSubsetOptimal_of_stable`.

Acceptance: `lake build Meso.DirectedSubsetOptimality` green; no `sorry`. Register
in `Meso.lean`.

### Task 6: subset-limitation Go characterization test

Add a test (in `directed_quality_test.go` or a new `directed_guarantees_test.go`)
that builds the n=4 fixture from the triage (section 6: arcs 0→1:1, 0→3:1, 1→0:3,
1→2:1, 1→3:5, 2→0:2, 3→2:5), runs `Leiden` with `DirectedModularity(1)`, and
documents the known limitation: meso's converged output does not split
`{0,1},{2,3}` even though that split scores higher (`Q = 1/27 > 0`). Assert the
current behaviour (whatever meso returns and its quality), and comment that this is
a documented, investigation-tracked limitation (`mes-niic`), not a regression, so a
future refinement improvement will intentionally flip this test. Keep it a
characterization test: it pins current behaviour and cross-references the ticket.
This is the only Go change in the phase. Acceptance: `make validate` green with the
new test.

### Task 7: CORRESPONDENCE and divergence register

In `verification/lean/CORRESPONDENCE.md`, amend the directed section to record:

- Weak connectivity: proved (`directedConnectedCommunities_of_refineRun`), directed
  `simpleGraph` via `fromRel`; Go counterpart the directed refinement gate
  (`refine.go`).
- γ-separation: proved conditional on `IsDirectedLevelStable`
  (`directedGammaSeparated_of_levelStable`); note the level-stability gap the triage
  recorded (meso stops at `moveImproveEps`, so raw output is not always exactly
  level-stable; the theorem is about the hypothesis, as undirected).
- Subset-optimality: two entries. (a) The conditional is proved
  (`directedSubsetGammaDense_of_subsetStable`). (b) As an *output property* it is
  REFUTED: record the n=4 fixture, the 1.3% directed / ~1.2% undirected-symmetric
  rates, that it is inherited from modularity (not directed-specific) and consistent
  with the undirected model proving subset-optimality only from the hypothesis, and
  cross-reference the characterization test and investigation ticket `mes-niic`.
- Strong connectivity: refuted by design (39% of communities; single-arc pairs are
  legitimate communities); no formal work.

If the file has a model-to-Go table, add rows for the new theorems. Match the
existing prose/table style, no em-dashes. Then markdownlint:
`markdownlint-cli2 --config ~/.markdownlint.yaml --fix verification/lean/CORRESPONDENCE.md`.

### Task 8: full validation and hygiene

- `make lean` green (all four new files build with the package).
- `grep -rn "sorry" verification/lean/Meso/` returns nothing.
- Optional: `#print axioms` on the three top theorems in a scratch buffer; expect
  only `propext`, `Classical.choice`, `Quot.sound` (no `sorryAx`, no
  `native_decide`). Remove the scratch lines.
- `make validate` green (including the new Go characterization test).

### Task 9: ticket notes

- `tk add-note mes-jlz8 "<files added, theorems proved, the subset conditional, the characterization test, CORRESPONDENCE updates, and that Bool mirrors/oracle are deferred to Phase 5>"`.
- `tk add-note mes-niic "<the n=4 fixture is now a committed characterization test; CORRESPONDENCE records the rates>"`.
- Do not change any ticket status; the user closes tickets.

### Task 10: final report

Report: files created, every theorem name proved, the characterization test, the
CORRESPONDENCE edits, and an explicit statement that weak connectivity,
γ-separation, and the subset conditional are proved while strong connectivity and
output-subset-optimality are recorded as refuted. Note `make lean`/`make validate`
status and that nothing was committed.

## 8. Acceptance checklist

- [ ] `make lean` green; `make validate` green; no `sorry` in `verification/lean/Meso/`.
- [ ] `Meso/DirectedConvergence.lean`: `IsDirectedLevelStable`,
      `IsDirectedConverged`, `IsDirectedSubsetStable` + adequacy lemmas.
- [ ] `Meso/DirectedConnectivity.lean`: directed `simpleGraph`, restated
      definitions, `directedConnectedCommunities_of_refineRun`.
- [ ] `Meso/DirectedSeparation.lean`: `directedModularity_merge_two_singletons`,
      `directedGammaSeparated_of_levelStable`.
- [ ] `Meso/DirectedSubsetOptimality.lean`: `directedModularity_moveSubset_split`,
      `directedSubsetGammaDense_of_subsetStable`.
- [ ] All four registered in `Meso.lean`; no undirected/Phase-1-2/`DirectedCompute`
      files modified; no lakefile/toolchain changes.
- [ ] Go characterization test for the n=4 subset fixture, cross-referencing
      `mes-niic`.
- [ ] CORRESPONDENCE.md updated (four entries) and markdownlint-clean.
- [ ] No Bool mirrors, no `OracleIO`/`Main` changes, no `doc.go` change.
- [ ] `tk add-note` on `mes-jlz8` and `mes-niic`; no status changes; nothing committed.

## 9. Risks and fallbacks

- **The restated refinement chain (Task 3) is more copying than proving.** If the
  volume is large, that is expected; do not try to abstract `Refinement.lean` over a
  shared interface (that would touch undirected files and is open question 5). Copy
  and substitute. If a ported proof fails, the cause is almost always a `simpleGraph`
  definitional mismatch, not a missing hypothesis.
- **The subset identity (Task 5) is the one place with real assembly.** Mitigations:
  `ind_subsetSplit`/`sum_sum_mul_ite_mem`/`block_sub_prod` are graph-agnostic and
  reused directly; the target closed form is given exactly (Section 5.5) and was
  numerically verified in the triage to 7.1e-15; cross-check the final `ring` against
  `cpm_moveSubset_split`'s shape. If it resists, the split-gain sign can also be
  obtained by summing the single-node `directedModularity_move_eq` over the members of
  `S` is NOT valid (subset moves are not sequential single-node moves at fixed
  partition); stay with the direct indicator-change proof.
- **γ-separation degree bookkeeping (Task 4).** The aggregate rewrite must use the
  Phase 2 `directedAggregate_*` lemmas exactly; a factor mismatch (`γ*a*b/m` vs
  `γ*(a*b)/m`) is the usual `ring_nf` fix. Verify against the triage's merge closed
  form and the symmetric reduction (both sides halve to the undirected bound).
- **Long build times.** Iterate per-module; full `make lean` at task boundaries only.
  If no mathlib cache and no network, stop and report.

## 10. Open questions (record in the final report; do not block)

- Whether to build the optional `DirectedGuarantees.lean` bundle now or defer to
  Phase 5 alongside the oracle predicates. Default: defer unless it is trivial.
- Whether the community-cut form of γ-separation
  (`..._blockWeight_of_levelStable`) is worth stating now or only when Phase 5's
  predicate needs it. Default: state it if it falls out; otherwise defer.
- Exactly what the Task 6 characterization test should assert about meso's returned
  partition (the specific labels vs just "not the better split"): pick the assertion
  that is stable under meso's determinism and clearly documents the limitation; record
  the choice.

## 11. What Phase 5 will want from this work (context, not tasks)

Phase 5 (`mes-43lb`, wire the directed value-oracle) will add computable Bool-mirror
predicates for weak connectivity and γ-separation (parallel to
`Meso/Predicates.lean`), prove them equal to the Props this phase defines, and emit
them from `mesoOracle` alongside the directed quantity vectors, plus extend the Go
oracle harness for directed inputs. Keep this phase's Prop statements clean and their
hypotheses explicit so the Phase 5 Bool mirrors can be proved equal to them directly.
The subset conditional's predicate, if emitted, must be labelled as conditional on a
hypothesis the algorithm does not attain (Task 7 divergence note).

## Notes

**2026-07-19T01:39:51Z**

Phase 4 implemented. Four new Lean files under verification/lean/Meso/ (registered in Meso.lean, no undirected/Phase-1-2/DirectedCompute/lakefile changes):

- DirectedConvergence.lean: IsDirectedLevelStable, IsDirectedConverged, IsDirectedSubsetStable + adequacy lemmas (IsDirectedConverged.localMoveStable/.levelStable, IsDirectedSubsetStable.isLocalMoveStable). Standalone directed stability, no shared quality typeclass (open question 5).
- DirectedConnectivity.lean: DirectedWeightedGraph.simpleGraph (fromRel), directed CommunityConnected/ConnectedCommunities/MergeStep/RefineStep restated, top theorem directedConnectedCommunities_of_refineRun (weak connectivity). Partition-only lemmas + connected_induce_union_of_adj reused directly.
- DirectedSeparation.lean: directedModularity_merge_two_singletons, directedGammaSeparated_of_stable/_of_levelStable/_blockWeight_of_levelStable. Bound e(C,D)+e(D,C) <= (g/m)(KoutC*KinD+KoutD*KinC); m=0 branch handled via directedWeight_eq_zero_of_totalWeight_eq_zero.
- DirectedSubsetOptimality.lean: directed_block_sub_prod (rectangular two-degree factoring), directedModularity_moveSubset_split, directedSubsetGammaDense_of_subsetStable (conditional on IsDirectedSubsetStable). User decision 1 honored: conditional formalized despite unattained hypothesis.

Guarantees status: weak connectivity + gamma-separation + subset conditional PROVED; strong connectivity + output-subset-optimality RECORDED AS REFUTED. No sorry; axioms only propext/Classical.choice/Quot.sound (native_decide-free).

Go: TestLeiden_DirectedSubsetOptimalityLimitation in directed_guarantees_test.go pins meso's all-in-one converged output on the n=4 fixture (Q~0) and documents the better {0,1},{2,3} split (1/27>0); registration order pinned to intern 0,1,2,3 (the stuck basin). CORRESPONDENCE.md: added 6 theorem-to-test rows and rewrote the directed divergence bullet (four entries). Bool mirrors + oracle wiring deferred to Phase 5 (mes-43lb, user decision 2). make lean and make validate green; nothing committed.
