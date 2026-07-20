---
id: mes-qmch
status: closed
deps: [mes-z1f5]
links: [mes-niic]
created: 2026-07-18T11:46:41Z
type: task
priority: 2
assignee: Andre Silva
parent: mes-crz3
tags: [directed, lean, verification, research, phase-3]
---
# Directed-modularity verification phase 3: guarantee triage (research core)

## Execution plan: Phase 3 of the directed-modularity verification project

Self-contained plan for an executing agent. It implements Phase 3
("guarantee triage, the research core") of the research plan in
[docs/research/directed-modularity-formal-verification.md](../docs/research/directed-modularity-formal-verification.md),
tracked by ticket `mes-qmch` (parent epic `mes-crz3`).

**Read this first.** Phase 3 is unlike Phases 1 and 2. Those were deterministic
engineering with a known-true target and a green-`lake-build` acceptance. Phase 3
is *research*: its outcome is not known in advance, and its deliverable is a
**triage report**, not proofs. No Lean guarantee proofs land in Phase 3; the most
Lean it produces is *statements* (conjectures) and, at most, a proof *sketch* for
connectivity to hand to Phase 4. The bulk of the work is an empirical
counterexample search ("scouting") outside Lean, plus mathematical derivation on
paper. Do not force a proof; a well-evidenced counterexample or a sharply-stated
conjecture is a complete and successful result.

## 1. Context and what Phases 1-2 delivered

`meso` is a pure-Go community-detection library whose undirected numeric core is
formally verified in Lean 4 (`verification/lean/`). The directed model now exists:

- Phase 1 (`mes-yx9f`, closed): `DirectedWeightedGraph`, `directedModularity`,
  the rational mirror, and the equivalence theorems
  (`Meso/DirectedGraph.lean`, `Meso/DirectedCompute.lean`).
- Phase 2 (`mes-z1f5`, closed): the objective identities in
  `Meso/DirectedGraph.lean`, `Meso/DirectedAggregate.lean`, `Meso/DirectedMove.lean`:
  - `directedModularity_aggregate_eq`: aggregation invariance.
  - `directedModularity_move_eq`: the closed-form single-node move gain (needs
    `t ≠ p u`; holds with no `totalWeight` hypothesis). On the aggregate graph a
    community merge is a single-node move, so this already yields the directed
    **merge-gain** closed form.
  - `directedModularity_toDirected_eq`: on a symmetric graph, directed modularity
    equals undirected modularity.
  - `directedModularity_const` (all-in-one scores `1 − γ`), value lemmas.

These are the assets Phase 3 reasons from. In particular the merge/split gain
closed forms come from `directedModularity_move_eq`, not from any new algebra.

## 2. The finding that reshapes this phase (read before scoping anything)

Recorded in the research doc under "Finding (recorded after Phase 2): the
undirected guarantees are CPM-only". Summary, because it changes what Phase 3 can
aim for:

- **All three undirected guarantees are proved in CPM, not modularity.**
  `gammaSeparated_of_converged`, `isSubsetOptimal_of_stable`, and the bundle
  `leidenGuarantees_of_stable` all hypothesise CPM convergence/stability
  (`Meso/Separation.lean`, `Meso/SubsetOptimality.lean`, `Meso/Guarantees.lean`).
  There is **no modularity version** of γ-separation or subset-optimality in the
  model. Only connectivity is objective-agnostic (`Meso/Connectivity.lean`).
- **Directed support runs directed modularity; directed CPM is out of scope by
  design.** So for γ-separation and subset-optimality there is *no proof to
  transfer*: not a CPM one (wrong objective, and directed CPM is descoped) and not
  a modularity one (none exists). Phase 3 must establish a modularity-flavoured
  guarantee for the first time, directed.
- **Connectivity is separable and tractable.** Objective-agnostic, so no CPM
  coupling; its directed form is almost certainly *weak* connectivity, and the
  proof machinery likely transfers.

Consequence: **do not treat the three guarantees uniformly.** Connectivity is a
likely proof (sketch for Phase 4); γ-separation and subset-optimality are open
modularity questions to scout and, if they survive, state as conjectures. A very
plausible and acceptable Phase 3 outcome is "connectivity promotes; the other two
become stated, scout-tested directed-modularity conjectures." The epic's
acceptance sanctions this partial landing.

## 3. Scope and non-goals

In scope:

- A directed-graph **scouting harness** (Go, using meso's public directed API) that
  runs directed Leiden to convergence on many random directed graphs and checks
  candidate guarantee bounds, hunting counterexamples.
- Optional convention cross-check against igraph (Task 8) to confirm meso's
  directed-modularity value matches the standard tool, so a "no counterexample"
  result is not an artifact of a private convention.
- Mathematical derivation of the candidate directed *statements* from the Phase 2
  move identity.
- A **triage report** (the primary deliverable): per guarantee, one of
  {statement + Lean-ready proof sketch | concrete counterexample | precisely stated
  open conjecture}, with the evidence.
- At most: Lean *statements* (conjecture signatures, no proofs) if it helps Phase 4;
  and a connectivity proof *sketch*.

Out of scope (do NOT do):

- No Lean guarantee *proofs* (that is Phase 4). Do not commit any `theorem ... := by`
  that proves a guarantee; conjecture *signatures* ending in `:= by sorry` are also
  forbidden in committed files (a committed `sorry` breaks the "no sorry" invariant).
  If stating conjectures in Lean, keep them in the report as code blocks, or in a
  clearly-marked scratch file under `.local/tmp/` that is NOT imported by `Meso.lean`
  and NOT built by `make lean`.
- No changes to any committed Lean file, no new imports in `Meso.lean`.
- No changes to the directed model or the Phase 1-2 theorems.
- No Go production changes. The scout is a standalone out-of-band module, not
  library code (Section 4). Graduating its findings into committed CI tests is
  Phase 4, not this phase.
- No oracle/golden changes (Phase 5). No git commits, no ticket status changes.
- Do not "make the guarantee true" by weakening the convergence hypothesis until a
  bound holds vacuously; a vacuous statement is a negative result, report it as such.

## 4. Where the scouting code lives, and its lifecycle (fixed decision)

The scout is an **out-of-band, standalone Go module**, the direct analogue of the
existing reference cross-check `verification/reference/crosscheck.py` (a
self-contained `uv` script that is committed, reproducible, and never in CI).
Concretely:

- Location: `verification/reference/directed-scout/` (a new directory alongside
  `crosscheck.py`), with its own `go.mod` and a
  `replace github.com/andreswebs/meso => ../../..` directive so it resolves the
  core from the working tree.
- Invisible to the gate: the Makefile builds only `MODULES := . gonum`, and there
  is no `go.work`, so this module is never touched by `make validate`, `make lean`,
  `make test`, or any CI path. It is run manually with `go run .` from its own
  directory.
- **Public API only, by construction.** Because it is a separate module it can
  import only meso's exported surface (`NewDirectedBuilder`, `Leiden`,
  `DirectedModularity`, `Result.Communities()`). It therefore *cannot* reach meso's
  internal `directedModularity` formula, which is exactly what we want: the scout
  must recompute every candidate bound **independently** from the generated edge
  list plus the returned partition, so a shared formula bug cannot blind it. Use
  meso only to (a) build the graph and (b) run Leiden to a converged partition.
- A `verification/reference/directed-scout/README.md` records what it does, the
  parameter grid, and the recorded verdicts, mirroring
  `verification/reference/README.md` (the "Recorded result" pattern).
- Any counterexample it finds must be reduced to a small, seed-reproducible fixture
  and recorded in the triage report as an explicit edge list plus partition, so it
  survives independently of the harness.

Why a standalone module and not a build-tagged in-package test: it matches the
repo's out-of-band reference-tooling philosophy, it keeps the core module's
`go.mod` and gate untouched, and it enforces the independent-recompute discipline
physically. This is NOT a "submodule for dependency isolation" like `gonum/`: it
pulls no external dependency; it is an out-of-band tool that happens to be a Go
module.

### Lifecycle after Phase 3

- **The engine stays out-of-band forever.** It is a reproducibility aid, like
  `crosscheck.py`: committed, documented, re-runnable on demand (e.g. if the
  directed objective or the model changes), never shipped in the library surface and
  never in CI. It is not deleted and not promoted into the gate.
- **The findings graduate into CI, but not in Phase 3.** A REFUTED guarantee's
  minimal fixture becomes a committed regression test plus a CORRESPONDENCE
  divergence note; a confirmed durable invariant (e.g. directed output is always
  weakly connected) becomes a committed property test in the style of
  `directed_move_test.go`. Both of those are *Go production changes*, which Phase 3
  forbids (Section 5): they are performed in Phase 4 (`mes-jlz8`) or a follow-up,
  informed by this phase's report. Phase 3 produces the evidence and the fixtures;
  it does not commit them into the gated suite.
- The Python/igraph convention check (Task 8) likewise stays an out-of-band `uv`
  script under `verification/reference/`, not a module and not CI.

## 5. The triage report (primary deliverable)

Draft it in `.local/tmp/phase3-triage-report.md` during the work. Final home:
`docs/research/directed-modularity-triage.md` (a committed research doc, the
mathematical record of the verdicts and derivations). Additionally, the scout
module's `verification/reference/directed-scout/README.md` carries a short
"Recorded result" section and reproduce instructions, mirroring
`verification/reference/README.md`, so the out-of-band run is documented next to
its code. The full report is the research doc; the README is the tool-local
summary. Structure of the research doc:

- **Setup**: scout description, graph-generation parameters, number of graphs/seeds,
  the igraph convention-check result (Task 8).
- **Per guarantee** (connectivity, γ-separation, subset-optimality), a subsection
  with:
  - the candidate directed *statement(s)* considered, with the exact bound;
  - the derivation from `directedModularity_move_eq` (for the two gain guarantees);
  - the scouting result: verdict in {PROVABLE (proof sketch follows) | REFUTED
    (counterexample follows) | OPEN (conjecture, no counterexample found in N
    graphs)};
  - the evidence: for REFUTED, the minimal reproducing fixture; for OPEN/PROVABLE,
    the search breadth (graph sizes, seed count, μ/density range) and the
    symmetric-reduction sanity check (Task 3).
- **Handoff to Phase 4**: exactly what Phase 4 should attempt to prove, refute-record,
  or leave as a conjecture, and the modeling prerequisites (Section 9).

## 6. Candidate directed statements (derive precisely in Task 5-6)

These are the starting hypotheses; the exact constants are worked out from the
Phase 2 identity during execution. Notation: `m = totalWeight`; for a community `C`,
`Kout_C = ∑_{i∈C} kout_i`, `Kin_C = ∑_{i∈C} kin_i`; `e(A,B) = ∑_{i∈A,j∈B} w_{ij}`
(ordered, so `e(A,B) ≠ e(B,A)` in general).

### 6.1 Connectivity (expected: weak, provable)

Statement: every community meso's directed Leiden returns induces a **weakly**
connected subgraph (connected in the underlying undirected graph: an edge wherever
`w_{ij} > 0` or `w_{ji} > 0`). Rationale: the directed refinement gate
(`refine.go`, directed branch) admits a merge across a positive arc in *either*
direction, and the undirected `WeightedGraph.simpleGraph` is
`SimpleGraph.fromRel (0 < weight)`, which already symmetrises. So the directed
simple graph defined identically is the underlying undirected graph, and
`connectedCommunities_singleton` plus the merge-preservation argument transfer.
Scout: confirm no returned community is ever weakly disconnected (and, separately,
record how often it fails to be *strongly* connected, to justify "weak").

### 6.2 γ-separation (open; derive the bound, then scout)

At a level-stable partition no community merge raises directed modularity. From
`directedModularity_move_eq` on the aggregate (merge `C` into `D` is a single-node
move there), the merge gain is proportional to

```text
( e(C,D) + e(D,C) )  −  (γ/m) · ( Kout_C·Kin_D + Kout_D·Kin_C )
```

(work out the exact factor and self-loop treatment in Task 5). "Gain ≤ 0" then reads
as the candidate directed γ-separation bound

```text
e(C,D) + e(D,C)  ≤  (γ/m) · ( Kout_C·Kin_D + Kout_D·Kin_C )   for all C ≠ D.
```

Note the **bidirectional cut** on the left and **both degree-product orientations**
on the right; this is the directed bookkeeping the ticket flagged. Scout whether it
holds at meso's converged outputs. Warm-up worth doing first (open question 8): pin
the *undirected modularity* γ-separation bound, which is the same derivation without
the out/in split and is strictly simpler; if it is already vacuous or false
undirected (modularity's resolution limit), that predicts the directed answer.

### 6.3 Subset-optimality (hardest; open)

At a subset-stable partition, splitting `S ⊆ C` off raises no directed modularity.
Splitting flips the "same community" indicator at `S × (C\S)` and its transpose, so
the split gain involves `e(S, C\S) + e(C\S, S)` against
`(γ/m)(Kout_S·Kin_{C\S} + Kout_{C\S}·Kin_S)`. Candidate bound: for every subset `S`
of every returned community `C`,

```text
e(S, C\S) + e(C\S, S)  ≥  (γ/m) · ( Kout_S·Kin_{C\S} + Kout_{C\S}·Kin_S ).
```

This needs a directed analogue of `IsSubsetStable`, which is the stability Leiden
*refinement* targets, so the directed refinement gate must be examined for what
subset-level stability it actually attains. Scout over subsets (exhaustive for small
communities; sampled for larger). Highest chance of ending OPEN or REFUTED.

## 7. Tasks, in order

Ordered so the cheap, decisive checks (convention, symmetric reduction, connectivity)
come before the expensive open ones.

### Task 0: baseline and orientation

- `make lean` and `make validate` green on the unmodified tree (record; abort if not).
- Read: this plan; the research-doc finding section; `Meso/Separation.lean`,
  `Meso/SubsetOptimality.lean`, `Meso/Connectivity.lean`, `Meso/Convergence.lean`
  (stability defs); `Meso/DirectedMove.lean` (the gain identity to reason from);
  `refine.go` (directed branch) and `louvain.go` (directed `bestMove`) for what the
  directed gate/moves actually do.

### Task 1: build the scouting harness

Create the standalone scout module `verification/reference/directed-scout/`
(`go.mod` with `module github.com/andreswebs/meso/verification/reference/directed-scout`
and `replace github.com/andreswebs/meso => ../../..`; a `main.go`; a `README.md`).
It must:

- Generate random directed graphs across a parameter grid: sizes n in {5..40},
  arc density in {0.1, 0.3, 0.5}, weight distributions incl. integer and unit,
  optional self-loops, and deliberately asymmetric regimes (sources/sinks, DAG-like,
  strongly-connected). Seeds derived deterministically from grid indices; log the
  seed on any hit. Write the scout's own generator emitting an edge list: the
  in-package helper `randomDirectedCSR` (`directed_move_test.go`) is not importable
  from a separate module, and building via the public `NewDirectedBuilder` is the
  point.
- Run meso directed Leiden to convergence on each (public API:
  `NewDirectedBuilder` + `Leiden` with `DirectedModularity(γ)`, γ over {0.5, 1, 2}).
- **Ensure the tested partition is genuinely converged** before judging it: verify no
  single-node move and no two-community merge strictly improves directed modularity,
  using the scout's own independently-recomputed objective (not meso's, which is
  unexported here anyway) (a bound-violation at a *non*-converged partition is a
  convergence artifact, not a
  counterexample). If meso's output is not fully stable, drive extra local-move /
  merge sweeps to a fixed point, or discard that sample and note the rate.
- Recompute the candidate bounds (Section 6) **independently** from the edge list and
  the returned partition.
- Emit, per guarantee, the number of graphs checked and any violation with its seed.

Acceptance: `go run .` from `verification/reference/directed-scout/` runs and
reports per-guarantee counts; `make validate` and `make lean` still green (the
scout is a separate module, absent from `MODULES` and from any `go.work`).

### Task 2: convention self-consistency

Confirm the scout's independent bound recomputation agrees with meso's own
quantities on a few fixtures (e.g. the scout's `e`, `Kout`, `Kin`, and directed Q
recomputed by hand match `directed_quality_test.go`'s `asymmetricDirectedCSR` values
and `directedModularity.Quality`). This guards against a bug in the scout that would
manufacture false counterexamples or hide real ones.

### Task 3: symmetric-reduction sanity gate

For each candidate bound, verify on **symmetric** directed graphs (built via
`symmetricDirected`, `directed_quality_test.go`) that the directed bound reduces to
the undirected modularity statement. A candidate directed statement that fails on
symmetric inputs is wrong by construction (it must specialise to the proved
undirected case per `directedModularity_toDirected_eq`); fix the statement before
scouting further. This is the cheapest correctness filter; run it before Task 5-6.

### Task 4: connectivity triage (aim: PROVABLE)

- Scout: over all generated graphs, check every returned community for weak
  connectivity (BFS on `w_{ij}>0 ∨ w_{ji}>0`). Record any failure as a counterexample.
  Separately record the strong-connectivity failure rate (expected: common), which
  justifies stating the guarantee as *weak*.
- If no weak-connectivity counterexample appears, write the proof sketch: the directed
  simple graph = `SimpleGraph.fromRel (0 < weight)` on the directed weight (identical
  to the undirected construction), so `Connectivity.lean`'s framework applies; the
  gate's shared-positive-arc-in-either-direction condition is exactly an edge of that
  simple graph; the singleton base and merge-preservation transfer. Identify each spot
  the undirected proof uses `weight_symm` and confirm `fromRel`'s built-in
  symmetrisation absorbs it. Hand this sketch to Phase 4.

### Task 5: γ-separation triage

- Derive the exact merge-gain closed form from `directedModularity_move_eq` on the
  aggregate (Task: instantiate the identity for a merge; pin the factor and self-loop
  handling). State the candidate bound (Section 6.2).
- Optional warm-up (recommended): derive and scout the **undirected modularity**
  γ-separation bound first (open question 8); if it is vacuous or false undirected,
  record that as the likely directed verdict.
- Scout the directed bound. Verdict PROVABLE / REFUTED / OPEN with evidence. If
  REFUTED, minimise the counterexample to a committed fixture.

### Task 6: subset-optimality triage

- Derive the split-gain closed form and the candidate bound (Section 6.3); define the
  directed `IsSubsetStable` analogue on paper and check what the directed refinement
  gate attains.
- Scout over subsets (exhaustive for |C| ≤ ~12, sampled above). Verdict + evidence.

### Task 7: modeling-prerequisite note

The undirected guarantees are stated over `IsConverged`/`IsLevelStable`/`IsSubsetStable`,
which are defined via `QualityFamily := ∀ m, WeightedGraph m → Partition m → ℝ`
(`Meso/Convergence.lean`) — tied to the *undirected* `WeightedGraph`. Phase 4 will need
a directed `QualityFamily`/stability layer (or a generalization). Do NOT build it here;
just specify in the report exactly what Phase 4 must add, so the conjecture statements
have a home. Note whether a shared abstraction over both graph types is worth it or
whether a standalone directed copy is simpler (ties to open question 5, model unification).

### Task 8: igraph convention cross-check (optional but recommended)

Confirm meso's directed-modularity value matches igraph's directed modularity on a few
fixtures (a small out-of-band Python script under `.local/tmp/`, igraph
`Graph.modularity(..., directed=True)`; do not add to CI, do not commit an image). This
makes a "no counterexample found" result meaningful: it shows meso optimises the
standard objective, not a private one. Record versions and values in the report. If the
conventions differ (self-loop or resolution handling), pin the difference before trusting
any OPEN verdict. Ties to open question 4.

### Task 9: write the triage report

Draft in `.local/tmp/phase3-triage-report.md`, then write the final research doc
`docs/research/directed-modularity-triage.md` and the scout module's
`verification/reference/directed-scout/README.md` "Recorded result" summary
(Section 5). This is the deliverable. Every verdict carries evidence; every REFUTED
carries a minimal fixture; every OPEN carries the search breadth and the
symmetric-reduction check. Markdownlint the committed docs
(`markdownlint-cli2 --config ~/.markdownlint.yaml --fix <file>`).

### Task 10: ticket note and handoff

- `tk add-note mes-qmch "<verdicts per guarantee + where the report lives + what Phase 4 should attempt>"`.
- Do not change ticket status; the user closes tickets.
- If any guarantee is REFUTED, flag it prominently: a counterexample changes the epic's
  scope and the user may want to revisit the Phase 4 ticket (`mes-jlz8`) and the epic
  acceptance.

### Task 11: final report to the user

Report: the scout's location and how to run it; per-guarantee verdicts with evidence
summaries; the igraph convention result; the modeling prerequisites for Phase 4; and an
explicit statement of what landed vs. what is deferred. Note that `make validate` is
untouched-green and nothing was committed.

## 8. Decision criteria (how to reach a verdict)

- **REFUTED**: a single reproducible counterexample at a *verified-converged* partition,
  minimised to a small fixture and re-checked by hand. One is enough. Highest-value
  outcome to find early.
- **PROVABLE**: no counterexample across the full grid AND a concrete Lean-ready proof
  path (for connectivity, the transfer sketch; for a gain guarantee, a derivation whose
  every step has a known mathlib/undirected analogue). Absence of counterexamples alone
  is NOT PROVABLE; it is OPEN.
- **OPEN**: no counterexample across a documented search breadth, but no proof path
  either (e.g. the bound holds empirically but its proof needs a lemma with no undirected
  precedent). State it as a precise conjecture with the evidence.

Bias toward OPEN over PROVABLE when unsure: overclaiming a proof path that Phase 4 cannot
discharge is worse than a conservatively-stated conjecture.

## 9. Risks and notes

- **False counterexamples from non-convergence.** The single biggest scout hazard. A
  bound may "fail" only because meso's returned partition is not fully converged. Task 1's
  convergence verification is mandatory, not optional.
- **Scout sharing a bug with meso.** Mitigated structurally: as a separate module
  the scout can only use the public API and must recompute bounds independently
  (Section 4), and Task 2's self-consistency check plus Task 8's igraph cross-check
  catch a scout bug.
- **Scout bit-rot.** As an out-of-band module it is not in CI, so a public-API
  change could break it silently. This is acceptable and low-risk: it uses only the
  stable exported surface (`NewDirectedBuilder`, `Leiden`, `Result`), and it is a
  reproducibility aid re-run on demand, not a gate. The recorded minimal fixtures in
  the triage report are the durable artifact if the scout ever stops building.
- **Convention mismatch with igraph** could make an OPEN verdict meaningless. Task 8
  gates this.
- **Exhaustive subset search blows up.** Cap |C| for exhaustive checks; sample above the
  cap and log the cap so the report does not overclaim coverage (silent truncation reads
  as "covered everything").
- **The derivation factor/self-loop details are fiddly.** Cross-check every candidate
  bound against `directed_move.go`'s doc comment (which carries the same algebra) and
  against the Task 3 symmetric reduction before trusting a scout result over it.
- **Scope creep into Phase 4.** If connectivity looks provable, resist writing the full
  Lean proof; a sketch is the Phase 3 deliverable. Formalization is `mes-jlz8`.

## 10. Open questions (record answers/updates in the report; do not block)

- Which connectivity notion is the one practitioners want, not just the one the gate
  gives (research-doc open question 1)?
- Does the undirected *modularity* γ-separation bound (the warm-up) already fail or go
  vacuous, predicting the directed answer (open question 8)?
- Are there boundary degeneracies (sources/sinks with zero in- or out-degree, self-loop
  only nodes) that break a bound only at the edge of the parameter space (open question 3)?
- Should the directed stability layer share an abstraction with the undirected one, or be
  a standalone copy (open question 5)?

## 11. What Phase 4 will want from this work (context, not tasks)

Phase 4 (`mes-jlz8`, formalize the survivors) consumes the triage report's verdicts:

- For each PROVABLE guarantee: the proof sketch and the exact statement to formalize,
  plus the derivation from the Phase 2 identity.
- For each REFUTED guarantee: the minimal counterexample, to record as a documented
  divergence.
- For each OPEN guarantee: the precise conjecture, to state in the model (without proof)
  or leave in the divergence register.
- The directed stability/convergence definitions Phase 4 must add (Task 7), so the
  guarantee statements have a home.

Keep every statement in the report explicit and self-contained (exact bounds, exact
hypotheses), so Phase 4 can quote it without re-deriving.

## Notes

**2026-07-18T18:32:13Z**

Phase 3 triage complete. Verdicts: (1) weak connectivity PROVABLE - 0 violations in 15228 communities over 3240 runs; strong connectivity REFUTED (39% failure rate); proof sketch = fromRel simple graph transfer, Refinement.lean chain uses no weight_symm. (2) gamma-separation conditional on level stability PROVABLE - 0 violations raw and converged, bound tight (min slack 0), merge closed form from Phase 2 identity matches from-scratch to 4.3e-15; undirected warm-up (symmetric regime) also clean. (3) subset-optimality REFUTED as an output property - 43/3240 converged samples admit improving subset split, incl. 8 symmetric-regime samples (inherited modularity behaviour, not directed regression); minimal n=4 fixture hand-verified (arcs 0>1:1 0>3:1 1>0:3 1>2:1 1>3:5 2>0:2 3>2:5, gamma=1, all-in-one converged, split {0,1}|{2,3} gains 1/27); conditional-on-subset-stability sound but hypothesis not attained. Convention: igraph 1.0.0 exact match on all fixture cases; scout Q == Result.Quality() to 1.1e-16 on every sample. Deliverables: docs/research/directed-modularity-triage.md (report of record), verification/reference/directed-scout/ (out-of-band module, go run ., -hunt-subset repro). Phase 4 handoff in report section 8: formalize weak connectivity + gamma-separation (mechanical from Phase 2 assets, directed stability defs specified in section 7); record subset refutation as divergence + graduate n=4 fixture to regression test. make validate green; no Lean or production Go changes.
