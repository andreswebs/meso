---
id: mes-niic
status: closed
deps: []
links: [mes-crz3, mes-qmch, mes-orp6]
created: 2026-07-18T18:42:26Z
type: task
priority: 2
assignee: Andre Silva
tags: [leiden, refinement, subset-optimality, quality, research]
---
# Investigate: meso Leiden converged output is not subset-optimal in practice (affects undirected core too)

The directed-modularity Phase 3 triage (docs/research/directed-modularity-triage.md) found, via the out-of-band scout, that meso's Leiden returns CONVERGED partitions that still admit a strictly-improving subset split: 1.3% of directed samples (43 of 3240) and ~1.2% of the symmetric (undirected-modularity) samples (8 of 648). That is, Leiden's headline advantage over Louvain - subset-optimality via refinement - is not attained in practice by meso, and this is NOT directed-specific: it affects the undirected core too.

This is consistent with the formal model's honest scope: subset-optimality is proved (undirected, CPM) only FROM the assumed hypothesis IsSubsetStable (isSubsetOptimal_of_stable, verification/lean/Meso/SubsetOptimality.lean); the model never claims meso's algorithm attains that hypothesis. So this is a known-scope gap surfaced empirically, not a proven-theorem violation. But it is worth investigating whether it is expected or a refinement weakness.

## Minimal reproduction (from the triage, n=4 directed)

Arcs (i -> j : w): 0->1:1, 0->3:1, 1->0:3, 1->2:1, 1->3:5, 2->0:2, 3->2:5. gamma=1, m=18. meso returns the converged all-in-one partition [0,0,0,0] (Q=0), but the two-community split {0,1},{2,3} scores Q = 1/27 > 0. A subset move (S={0,1}) escapes a local optimum that meso's single-node moves and community merges do not. Reproduce with `go run . -hunt-subset` in verification/reference/directed-scout/.

## Questions to answer

1. Is this expected behaviour of Leiden's refinement as specified (refinement targets subset stability but does not guarantee it at every gamma / on every graph), or a gap in meso's refinement implementation?
2. Does the reference (leidenalg/igraph) exhibit the same on these inputs? (Cross-check out of band.)
3. If it is a meso weakness, what would strengthen it (e.g. a subset-aware refinement pass), and at what cost to determinism/performance?
4. Regardless of outcome, should this be surfaced to users? (Phase 4 decision was: document in CORRESPONDENCE + a characterization test, NOT user docs, for now.)

## Relationship to the directed epic

Discovered during the directed work (epic mes-crz3) but independent of it: the behaviour predates directed support and lives in the undirected core. Tracked separately so the directed verification phases are not blocked on it.

## Notes

**2026-07-19T01:00:21Z**

Investigation complete; all four questions answered.

Q1 (expected vs meso gap): Leiden-as-specified per pass, plus a meso descope; not a refinement bug. Level-by-level trace on the n=4 fixture (node registration order 0,1,2,3): phase-1 greedy local moving is seed-independent and settles in the wrong-pairing basin {0,2},{1,3} (Q ~ -0.049); refinement only splits within phase-1 communities and both pairs are internally cohesive (one forced candidate per node, so the seed never has a choice); aggregation freezes granularity at {02},{13}; the aggregate level merges to all-in-one (0 > -0.049). The improving split {0,1},{2,3} crosses the frozen boundaries and is unreachable in meso's single pass: 10000/10000 seeds return all-in-one. Caveat discovered on the way: under edge-only construction the builder interns nodes in first-seen order (0,1,3,2) and meso finds the optimal split on every seed; the fixture's outcome flips with registration order (documented behaviour, Builder.Canonical() is the opt-in reordering). Reproductions must pin the scout's AddNodeWeight-first order.

Q2 (reference cross-check): leidenalg 0.10.2 / igraph 0.11.8, 200 seeds, .local/tmp/leidenalg-subset-check.py (out of band, not committed). igraph's directed modularity matches our values exactly (all-in-one 0, split 1/27). Singleton start with n_iterations=-1: 9/200 seeds return the suboptimal all-in-one. Started from all-in-one: 57/200 stay stuck. 100 forced iterations: all 57 escape. So the reference exhibits the same per-run gap; the paper's subset-optimality guarantee is asymptotic over repeated randomized iterations, never per run. meso seed sweep is .local/tmp/seed-sweep/.

Q3 (strengthening): decided, tracked as mes-orp6. Add WithIterations(k), fixed pass count only, default 1 (byte-identical to today); each pass restarts from the previous base partition with a per-pass derived refinement seed; passes are monotone in Q. An until-stable mode was deliberately dropped as misleading (leidenalg's -1 stops at the first non-improving, possibly unlucky, pass). Rejected: multi-seed restarts (ineffective; phase-1 is seed-independent, every seed replays the same basin) and a subset-aware repair pass (exponential or an unformalizable heuristic deviating from Leiden as specified).

Q4 (surfacing): yes, scoped. The WithIterations godoc is the surface: the reason to set k above 1 is the phenomenon, stated plainly there. Phase 4 plan unchanged (CORRESPONDENCE divergence note + n=4 characterization test pinning registration order). Design doc gets the lock-in mechanism when mes-orp6 lands.

Learnings recorded in docs/specs/learnings.md (mes-niic section).

**2026-07-19T01:39:58Z**

The n=4 directed subset-suboptimality fixture is now a committed Go characterization test: TestLeiden_DirectedSubsetOptimalityLimitation in directed_guarantees_test.go. It pins meso's converged all-in-one output [0,0,0,0] (Q~0) on the fixture (arcs 0->1:1,0->3:1,1->0:3,1->2:1,1->3:5,2->0:2,3->2:5; m=18, gamma=1), which admits the improving split {0,1},{2,3} (Q=1/27>0). AddEdge order pins interning to 0,1,2,3 (the stuck basin); a different order lets meso escape to the split. Seed-independent (phase-1 basin). CORRESPONDENCE.md now records the ~1.3% directed / ~1.2% undirected-symmetric violation rates and that this is inherited from modularity, not directed-specific. When this investigation strengthens refinement to attain subset stability, that test will intentionally flip.
