# Verifying directed modularity: options, trade-offs, and a research plan

`meso` ships directed community detection (Leicht-Newman directed modularity)
but, unlike the undirected core, it is **not formally verified**: directed
support is reference-tested only (hand-computed fixtures, an internal move-delta
property test, and a planned one-time `leidenalg`/igraph cross-check). This was a
deliberate, recorded decision (2026-07-14, Lean TODO Phase E3; see
[verification/lean/CORRESPONDENCE.md](../../verification/lean/CORRESPONDENCE.md)
and [docs/specs/001-initial-implementation/meso-oracle.md](../specs/001-initial-implementation/meso-oracle.md)).

This document explains why, lays out the options and their trade-offs, and drafts
a research plan for the one option that would close the gap: formally verifying
the directed case. It is a research statement, not an implementation guide. Its
purpose is to be turnable into a research project that eventually performs the
original mathematics the verification would require.

## Background: what the undirected oracle is, and why it is strong

`meso`'s numeric core is validated against a **Lean value-oracle**. Lean 4 is a
compiled functional language as well as a proof assistant, so the same
definitions the proofs reason about can be executed to emit expected values. A
`lake exe mesoOracle` run evaluates the proved model over a committed input set
and emits exact rational quality, move-delta, and predicate values as golden
vectors; Go then asserts its `float64` results land within a float-rounding
tolerance of those exact rationals.

The strength of this arrangement is that the oracle is the *proved* artifact.
Using it makes the Go code faithful to the verified design, not merely to a
second unproven implementation that could share a misconception with the code
under test. That is a stronger guarantee than any differential harness against an
independent reference.

The undirected model rests on one structural assumption: symmetry. In the Lean
model, symmetry is not an incidental hypothesis but a required field of the graph
structure:

```lean
structure WeightedGraph (n : ℕ) where
  weight      : Fin n → Fin n → ℝ
  weight_symm : ∀ i j, weight i j = weight j i   -- required to construct one
  weight_nonneg : ∀ i j, 0 ≤ weight i j
  nodeSize    : Fin n → ℝ
  nodeSize_nonneg : ∀ i, 0 ≤ nodeSize i
```

To build a `WeightedGraph` you must supply a proof that the weight function is
symmetric. For a genuinely directed graph, that proposition is false, so no such
proof exists, so the object cannot be constructed.

## Why the directed case is not simply "the same, with more arcs"

Two facts make directed a different problem, not an extension of the existing
one.

1. **Different objective.** Directed (Leicht-Newman) modularity uses a null model
   with separate out- and in-degrees, `k_i^out k_j^in / m`, rather than the
   symmetric `k_i k_j / 2m`. The theorem *statements* change, not just their
   proofs.

2. **Symmetry is load-bearing in the guarantees themselves.** The three paper
   guarantees `meso` proves for the undirected case (every returned community is
   well-connected, γ-separated, and subset-optimal) use `weight_symm` inside
   their proofs: the γ-separation merge gain, the subset split-off gain,
   `gammaDense_union`, and the aggregate's own `weight_symm` all depend on it, and
   `weight_symm` appears across nearly every proof file. Remove symmetry and these
   arguments do not merely need re-checking; the results they establish are not
   known to hold.

The reference paper (Traag, Waltman, van Eck, 2019) states the guarantees for the
**symmetric case only**. For directed modularity, whether the analogous
guarantees hold, and in what form, is not established in the literature. This is
the deep blocker: it is a mathematics gap, not a tooling gap.

## Finding (recorded after Phase 2): the undirected guarantees are CPM-only

A review of the undirected Lean model while planning Phase 3 surfaced a fact that
reshapes the guarantee work, and it is recorded here because it changes what
Phase 3 and Phase 4 can aim for.

**Every one of the three undirected guarantees is proved in CPM, not modularity.**
`gammaSeparated_of_converged` and `cpm_gammaSeparated_of_stable`
(`verification/lean/Meso/Separation.lean`), `isSubsetOptimal_of_stable` and
`cpm_subsetGammaDense_of_stable` (`Meso/SubsetOptimality.lean`), and the conjoined
`leidenGuarantees_of_stable` (`Meso/Guarantees.lean`) all hypothesise CPM
convergence (`IsConverged (cpmF γ)`) and CPM subset-stability. There is **no
modularity version of γ-separation or subset-optimality** anywhere in the model.
Only connectivity is objective-agnostic: it rests on the refinement gate's shared
positive-weight edge and the positive-weight simple graph
(`Meso/Connectivity.lean`), not on the objective.

The reason is structural. CPM's merge and split gains have the clean,
normalisation-free form `e(S,T) − γ‖S‖‖T‖`, and the guarantee *is* that structural
bound. Modularity carries the `1/2m` normalisation and the `k_i k_j / 2m` null
term, so "gain ≤ 0" reads as a degree-and-`m` bound rather than a clean
size-product, and modularity additionally has the resolution-limit pathology. The
undirected model therefore states the γ-guarantees where they are cleanest, in
CPM, and never states a modularity analogue.

Two consequences for the directed work:

1. **For γ-separation and subset-optimality, there is no proof to "transfer."**
   Directed support runs directed *modularity*, and directed CPM is out of scope
   by design (CPM stays undirected). Since no *modularity* guarantee proof exists
   even in the undirected case, Phase 3 is not "port a CPM proof past
   `weight_symm`"; it is "establish a modularity-flavoured guarantee for the first
   time, and do it directed." That is more original than a straight port, and it
   is the same task the undirected model declined to take on.

2. **Connectivity is the separable, tractable guarantee.** Because it is
   objective-agnostic, it does not inherit the CPM coupling. Its directed form is
   almost certainly *weak* connectivity (meso's directed refinement gathers
   candidates from both in- and out-neighbours, and the undirected
   `WeightedGraph.simpleGraph` is built with `SimpleGraph.fromRel`, which already
   symmetrises), and the shared-positive-weight-edge preservation argument should
   transfer nearly verbatim. It is the one guarantee that looks promotable to a
   proof rather than merely triageable.

The net effect is that Phase 3 should not treat the three guarantees uniformly:
connectivity is a likely proof, while γ-separation and subset-optimality are
genuinely open modularity questions to be scouted for counterexamples and, if they
survive, stated as directed-modularity conjectures. See also open question 8.

## The options, and their trade-offs

There are four distinct things one could mean by "give directed a Lean-style
oracle." They are not interchangeable.

### Option A: reuse the existing proved model to emit directed goldens

**Verdict: impossible.** The proved `WeightedGraph` requires a proof of symmetry
to construct. A directed graph cannot supply one, so the oracle has nothing to
run on. This is impossible in the strong sense (the required proof obligation is
unsatisfiable), not merely unsupported by tooling.

### Option B: write a new directed Lean definition, use it only as a calculator

**Verdict: possible, but dominated.** One could define a fresh directed graph
structure and a `directedModularityQ` over it, and execute it to emit exact
rational values, without proving anything about it. This works, but it defeats
the point of using Lean: the only reason the Lean oracle beats an independent
implementation is that it is *proved*. A directed definition with no theorems
attached is just an ordinary program that happens to be written in Lean. It
yields exact fractions and nothing more, and a self-contained `fractions.Fraction`
script in Python yields the same exact fractions for a small fraction of the
effort (see Option D). So Option B pays Lean's cost and delivers Python's value.

### Option C: write a new directed model and prove the guarantees about it

**Verdict: possible, valuable, expensive, and partly open research.** This is the
only option that would actually make directed a verified feature at parity with
the undirected core. It requires (1) a directed graph model in Lean, (2)
statements of the directed analogues of the objective identities and the three
guarantees, and (3) proofs of them, which in turn requires first determining
whether those directed guarantees are even true. The rest of this document drafts
how to approach it.

### Option D: self-contained exact-rational golden, no Lean (the pragmatic middle)

**Verdict: possible, moderate effort, weaker guarantee.** A pure-standard-library
Python script computes exact directed Q and move-deltas with `fractions.Fraction`
and emits the same `{num, den}` golden format the Go harness already consumes
(mirroring the style of
[verification/oracle/tools/gml_to_input.py](../../verification/oracle/tools/gml_to_input.py)).
This gives corpus-scale committed vectors re-checked on every Go test run.

The cost is honesty about the guarantee: this is a *second unproven
implementation*, exactly the shared-misconception risk the Lean oracle was
designed to avoid. It is acceptable for directed (the design already accepts
hand-computed fixtures) and strictly stronger than the single 3-node fixture
present today, but it does not reach the undirected oracle's bar. Consuming it
also requires extending the Go oracle harness, which is currently hard-wired to
undirected inputs (an undirected `Builder`, and a quality switch over only
`modularity`/`cpm`).

### Option E: one-time `leidenalg`/igraph cross-check (the design's stated plan)

**Verdict: possible, this is what the design already prescribes.** igraph exposes
directed modularity. Run it out of band on a handful of asymmetric fixtures,
confirm agreement within float tolerance, and commit a dated note, exactly the
mechanism already used for the undirected spec-blessing cross-check. This
validates against a genuinely independent reference (guarding against a wrong
*specification*, which Options B and D cannot), but produces float agreement, not
a committed exact vector, and is a one-off, not a standing check.

### How the options relate

- Options D and E are the near-term, pragmatic ways to raise directed confidence
  without new mathematics. They are complementary: E blesses the specification
  once; D gives standing, corpus-scale value-checking. Neither is verification.
- Options A and B are dead ends for verification (impossible, and pointless,
  respectively).
- Option C is verification. It is the subject of the research plan below.

## What a reference implementation can and cannot do for a proof

Because it is a natural question: a Python (or igraph) reference **cannot build
the proof**, and, more importantly, it does not remove the blocker.

- A reference implementation produces **examples**: one input, one output. You
  can generate arbitrarily many.
- A proof is a **universally quantified** statement ("for every directed graph and
  every partition ..."). No finite set of examples can establish a "for all"; the
  infinite remainder is covered by reasoning, not enumeration.

A reference is still genuinely useful in two supporting roles: as a **test
oracle** (Option D/E), and as a **scout** that tries to *break* a conjecture on
thousands of graphs before effort is spent proving it. A found counterexample
saves a wasted proof; the absence of one is encouragement, not proof. What a
reference cannot supply is the missing mathematics: whether the directed
guarantees are true at all. That has to be settled by proof or disproof, and only
then can the Lean formalization proceed.

## Option C in detail: a research plan for formal verification of directed modularity

The goal: bring directed modularity to parity with the undirected core, i.e. a
Lean model whose objective identities and applicable guarantees are proved, feeding
a directed value-oracle the Go implementation is checked against.

The plan separates the tractable engineering (define the objective, prove the
algebraic identities) from the open research (establish which structural
guarantees survive the loss of symmetry).

### Strategy

Proceed in order of increasing mathematical risk, so the project produces
value early and fails fast if a guarantee turns out to be false.

1. **Model and identities first (low risk).** These mirror what already exists
   undirected and do not depend on the paper's guarantees.
2. **Guarantee triage next (the research core).** For each of the three
   guarantees, determine the directed truth *before* attempting a Lean proof,
   using the reference implementations as scouts.
3. **Formalize only what survives triage (variable risk).** Prove the directed
   analogues that triage supports; record precise counterexamples for those it
   refutes; leave genuinely open ones as stated conjectures.

Throughout, keep the undirected model untouched: introduce the directed model
alongside it rather than generalizing the existing structure, to avoid disturbing
proved undirected results. Whether to later unify them behind a common interface
is an explicit open question (below).

### Tactics, phase by phase

**Phase 1: directed graph model.** Add a Lean structure for a directed weighted
graph, with separate treatment of out- and in-arcs, nonnegative weights, and node
sizes, but *without* a symmetry field. Define directed out-degree, in-degree, and
total arc weight `m`. Define the computable rational mirror
(`directedModularityQ : ... → ℚ`) for the value-oracle, alongside the real-valued
definition for the proofs, and prove they agree on rational inputs (the directed
analogue of `modularityQ_eq`). This phase is deterministic engineering.

**Phase 2: objective identities (low risk).** Prove the algebraic facts the
optimizer relies on, none of which need the guarantees:

- The move-delta identity: the incremental directed gain equals
  `Q(after) − Q(before)` exactly. This is the directed analogue of the identity in
  [move-delta-verification.md](move-delta-verification.md) and is almost certainly
  true (it is pure algebra on the objective); `meso`'s directed move-delta already
  passes a 5000-graph property test, which is strong scouting evidence.
- Aggregation invariance: directed modularity is preserved when a community is
  collapsed to a super-node with the out/in block sums folded correctly (the
  directed analogue of `modularity_aggregate_eq`). `meso`'s `aggregateDirected`
  is the operational counterpart; the identity is what would make it verified.
- Single-community modularity is zero, and the symmetric reduction: on a symmetric
  graph directed modularity equals undirected modularity. The reduction is a
  valuable consistency theorem tying the new model to the proved one.

**Phase 3: guarantee triage (the research core).** The three guarantees are
`connectivity` (every returned community is a connected subgraph), `γ-separation`
(no two communities can be merged for a gain), and `subset-optimality` (no subset
of a community can be split off for a gain). For each, the questions are:

- What is the correct directed *statement*? Connectivity has at least two candidate
  directed meanings (weakly connected vs strongly connected); the right one is the
  one the algorithm's gate actually establishes. γ-separation and subset-optimality
  are gain conditions, so their directed statements follow the directed move-delta,
  but the direction bookkeeping (out-cut vs in-cut) must be worked out.
- Is that statement *true*? Use the reference implementations and `meso`'s own
  directed pipeline as scouts: search for counterexamples over many random directed
  graphs at converged partitions. A counterexample settles it negatively and is
  itself a publishable result; sustained failure to find one is the green light for
  a proof attempt.
- If true, does an undirected proof *transfer*? For connectivity, yes in
  principle: identify where the objective-agnostic proof invokes `weight_symm`
  (mainly the `fromRel` symmetrisation) and confirm the weak-connectivity
  substitute closes the same step. For γ-separation and subset-optimality there is
  no modularity proof to transfer (see "Finding: the undirected guarantees are
  CPM-only" above); the closed-form gain comes from the Phase 2 directed
  move-delta identity, and triage must derive what structural bound "gain ≤ 0"
  yields for the directed-modularity null model (both cross-directions `e(C,D)`
  and `e(D,C)`, both degree-product orientations).

**Phase 4: formalize the survivors.** Prove in Lean the directed guarantees that
triage supports, as real-valued Props. Record refuted guarantees as documented
divergences with their counterexamples. Leave anything still open as a precisely
stated conjecture. (Boundary decision D1: the computable Bool *mirrors* of those
Props, and their `_iff` equivalences, belong to Phase 5, alongside the oracle
wiring they feed; Phase 4 produces the Props and stops there.)

**Phase 5: wire the directed value-oracle.** Build the computable Bool mirrors of
the Phase 4 Props (each proved `_iff` its Prop on `.toReal`), extend `mesoOracle`
to emit directed vectors for the proved quantities and predicates, add directed
fixtures to the committed input set, and extend the Go oracle harness to consume
them (directed `Builder` path, a `directedModularity` quality case). At this point
directed has whatever subset of undirected's guarantees survived, verified end to
end.

### Deliverables

- A Lean directed model and the Phase 2 identity proofs (tractable regardless of
  how triage goes).
- A triage report: for each guarantee, the directed statement, a proof, a
  counterexample, or a stated open conjecture, with the scouting evidence.
- Whatever Phase 4 proofs the triage supports, plus a directed value-oracle
  (Phase 5) covering the proved quantities.
- If any guarantee is genuinely novel mathematics, a write-up suitable for
  external review; the directed guarantees of Leiden-style refinement appear to be
  unaddressed in the current literature.

## Open questions (for further research)

These are deliberately unresolved and are the substance a research project would
tackle.

1. **Connectivity semantics.** Which notion of directed connectivity (weak or
   strong) does Leiden's refinement gate actually guarantee on a directed graph,
   and is it the notion practitioners want? The gate is stated over a shared
   positive-weight arc; does "shared arc in either direction" yield weak
   connectivity only?
2. **Do the guarantees survive asymmetry at all?** Is there a directed graph and a
   converged partition where two communities are mergeable for a gain
   (γ-separation fails) or a subset is splittable for a gain (subset-optimality
   fails)? A single counterexample reshapes the whole project.
3. **Resolution-limit and null-model subtleties.** The directed null model
   `k_i^out k_j^in / m` interacts with self-loops and with the resolution
   parameter differently from the symmetric case; are there degeneracies (e.g.
   sources/sinks with zero in- or out-degree) that break the objective or the
   move-delta at the boundary?
4. **Convention alignment with igraph/leidenalg.** Exactly which directed
   modularity convention (normalization, self-loop treatment, resolution scaling)
   do the standard tools implement, and does it match `meso`'s? This must be
   pinned before any cross-check (Option E) is meaningful, and before a Lean
   statement can claim to formalize "the" directed modularity.
5. **Model unification.** Should the directed model be introduced standalone or
   should the undirected `WeightedGraph` be refactored into a common interface with
   a symmetry-carrying specialization? The latter is cleaner but risks disturbing a
   large body of proved undirected results; the trade-off needs its own analysis.
6. **Directed CPM.** CPM is currently undirected by design. Is a directed CPM
   analogue meaningful, and if so does it share the directed modularity guarantee
   story or need its own triage?
7. **Scope of "verified directed."** If only some guarantees survive (say, the
   objective identities and connectivity, but not subset-optimality), is a
   partially-verified directed feature worth shipping as such, and how should the
   surviving-vs-refuted status be surfaced to users?
8. **What is a modularity guarantee, even undirected?** The finding above shows the
   model proves γ-separation and subset-optimality only in CPM. Before the directed
   modularity versions can be stated, the modularity-flavoured statement itself must
   be pinned down: what structural bound does "no merge / no split raises modularity"
   give, given the `k_i k_j / 2m` null term, and is it meaningful or does modularity's
   resolution limit make it vacuous or false? This question exists undirected too; the
   directed case only adds the asymmetric null model on top. It may be worth answering
   the undirected modularity version first, as a strictly simpler warm-up.

## Recommendation

For raising confidence without new mathematics, pursue Option E (the design's
stated one-time cross-check) and, if standing corpus-scale value-checking is
wanted, Option D alongside it. Reserve Option C for when directed becomes a hard
product requirement or a deliberate research investment: it is the only route to
genuine parity with the undirected core, but its core phase is original
mathematics whose outcome is not known in advance. This document is the starting
point for that investment.
