---
id: mes-crz3
status: closed
deps: []
links: [mes-niic]
created: 2026-07-18T11:20:02Z
type: epic
priority: 2
assignee: Andre Silva
tags: [directed, lean, verification, research]
---
# Formal verification of directed modularity (research)

Bring directed (Leicht-Newman) modularity to parity with meso's undirected core: a Lean model whose objective identities and applicable guarantees are proved, feeding a directed value-oracle the Go implementation is checked against.

Directed community detection ships in Go but is deliberately outside the Lean model. The proved `WeightedGraph` structure requires a symmetry proof (`weight_symm`) a directed graph cannot supply, and symmetry is load-bearing inside the undirected guarantee proofs. The Traag-Waltman-van Eck (2019) reference states the guarantees for the symmetric case only, so whether their directed analogues even hold is not established in the literature. That mathematics gap, not a tooling gap, is why directed is reference-tested rather than verified today.

Research statement of record: `docs/research/directed-modularity-formal-verification.md`. It also weighs the alternatives (a one-time igraph/leidenalg cross-check, a self-contained exact-rational golden) that raise confidence without new mathematics; this epic tracks the verification route (the doc's Option C) specifically.

## Strategy

Proceed in order of increasing mathematical risk, so the project delivers value early and fails fast if a guarantee turns out false. Keep the directed model standalone alongside the undirected one; whether to later unify them behind a common interface is an explicit open question.

## Phases

1. Directed graph model and rational mirror. Define the directed weighted graph in Lean (the undirected structure minus symmetry), directed out/in-degree and total arc weight, the real-valued directed modularity objective, its computable rational mirror, and the equivalence theorem tying the two together. Deterministic engineering, no open mathematics.

2. Objective identities (low risk). Prove the algebraic facts the optimizer relies on, none of which need the guarantees: the move-delta identity (incremental gain equals the from-scratch difference), directed aggregation invariance, single-community modularity is zero, and the symmetric reduction (directed Q equals undirected Q on a symmetric graph).

3. Guarantee triage (the research core). For each of connectivity, gamma-separation, and subset-optimality: determine the correct directed statement, use the reference implementations and meso's own pipeline as scouts to search for counterexamples, and, where a guarantee survives, identify whether the undirected proof transfers or needs an asymmetric substitute. A single counterexample reshapes the project; this is where the original mathematics lives.

4. Formalize the survivors. Prove in Lean the directed guarantees triage supports, as computable Bool mirrors proved equal to the directed predicates. Record refuted guarantees as documented divergences with their counterexamples; leave genuinely open ones as precisely stated conjectures.

5. Wire the directed value-oracle. Extend the mesoOracle executable to emit directed vectors for the proved quantities and predicates, add directed fixtures to the committed input set, and extend the Go oracle harness to consume them. At this point directed has whatever subset of the undirected guarantees survived, verified end to end.

## Open questions (see the research doc for detail)

Connectivity semantics (weak vs strong); whether the guarantees survive asymmetry at all; null-model/self-loop degeneracies at the boundary; convention alignment with igraph/leidenalg; standalone model vs unification with the undirected model; directed CPM; and the scope of a partially-verified "verified directed" feature.

## Acceptance

The epic is complete when directed modularity's objective identities and every guarantee that triage establishes as true are machine-checked in Lean and emitted by the directed value-oracle, with refuted or open guarantees documented as such. Partial completion (identities plus a subset of guarantees) is an acceptable landing point if triage refutes or leaves open the rest; the epic then closes against the recorded triage outcome, not against proving all three guarantees.

## Notes

**2026-07-19T15:37:44Z**

Epic complete. All five phases closed (mes-yx9f, mes-z1f5, mes-qmch, mes-jlz8, mes-43lb).

Landing outcome against the acceptance criterion: directed modularity's objective identities plus every guarantee triage established as true are machine-checked in Lean and emitted by the directed value-oracle; refuted/open guarantees are documented as such.

- Objective + identities (Phases 1-2): directedModularity, its rational mirror, aggregation invariance, closed-form move-delta, symmetric reduction. Proved.
- Guarantees (Phases 3-4): weak connectivity PROVED; gamma-separation PROVED conditional on directed level stability; strong connectivity REFUTED by design; subset-optimality REFUTED as an output property (n=4 fixture) but the subset-stability conditional PROVED.
- Value-oracle (Phase 5): mesoOracle emits directed quantity + {connected, gammaSeparated, subsetOptimal} vectors behind the "directed": true flag, from Bool mirrors proved _iff their real Props (axiom-clean). Go harness checks values, move-deltas, weak connectivity, and gamma-separation against the proved vectors on every run; celegansneural covers scale. subsetOptimal emitted as a characterization flag (false on the refutation fixture), cross-referenced to mes-niic. Directed CPM stays unmodelled by design.

This closes against the recorded triage outcome (identities + weak connectivity + gamma-separation verified end to end; subset-optimality documented as refuted-output), which the epic explicitly accepts as a landing point. No git commits, no code left broken (make lean, oracle-lean, validate all green).
