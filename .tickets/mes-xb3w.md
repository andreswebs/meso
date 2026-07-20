---
id: mes-xb3w
status: closed
deps: [mes-jbc7, mes-nqky]
links: [mes-5wqp]
created: 2026-07-17T18:37:09Z
type: task
priority: 3
assignee: Andre Silva
parent: mes-t76u
tags: [oracle, golden, corpus, deferred]
---

# Add a modularity-optimal partition per corpus graph to the oracle input set

Extend the committed value-oracle input set so each corpus graph (karate, dolphins, Les Miserables) is also scored on a modularity-maximizing partition, not only the current anchors (all-in-one, all-singletons, and karate's ground-truth two-faction split).

Today the oracle never scores the partition meso's Leiden is meant to approximate. karate's modularity optimum is about 4 communities at Q approximately 0.4198, whereas the pinned partitions top out at the ground-truth split (Q = 1453/4056 approximately 0.3582), all-in-one (0), and singletons (about -0.05). The same gap exists for dolphins and Les Miserables.

This was the sole remaining item under "Open questions" in docs/specs/001-initial-implementation/meso-oracle.md. It is not a design fork but a deferred task gated on the Go Leiden core existing, so it is moved out of the doc and tracked here. Recorded from the 2026-07-17 oracle-harnesses discussion (Q1 tolerance, Q3 predicate-flip fixtures, and Q2 LFR sweep were locked in that same pass).

## Design

Mechanism. Once the public Leiden() API can produce a converged partition, run it deterministically (fixed, documented seed) on each corpus graph, take the converged high-quality partition, add it as a new case in verification/oracle/inputs/{karate,dolphins,lesmis}.json, regenerate the golden vector with `make oracle-lean` (the mesoOracle executable already emits the exact rational quality and the predicate vectors), and have the Go value-oracle harness assert meso's float64 quality of that partition lands within tolerance of the exact rational value.

What it tests, and what it does NOT. The Lean value-oracle blesses the exact quality VALUE of whatever partition it is given; it does not certify that the partition is optimal. So this vector serves two honest purposes:
(1) a determinism/regression pin: meso is deterministic, so its converged partition per graph is stable, can be committed, and re-checked for drift;
(2) value-correctness on a real, non-degenerate output: meso computing the modularity of its own rich output correctly to float tolerance, a path the degenerate anchors (Q=0, singletons) do not exercise.
It must NOT be framed as an optimality assertion. Confidence that the output is actually good rests on the proven invariants (correctness) and the LFR NMI/ARI accuracy sweep (accuracy), never on this value-oracle vector. Modularity maximization is NP-hard, so meso's converged partition is a strong local optimum, not a certified global optimum.

Naming. Pin it as meso's converged / best-known partition per graph, not "the optimum". For karate the published optimum (Q approximately 0.4198) can corroborate the value out of band, but do not hand-enter optima for dolphins/lesmis; use meso's own converged output uniformly across the corpus.

Tolerance. Reuse the locked Go-vs-Lean policy in docs/specs/001-initial-implementation/meso-oracle.md (ULP distance against the correctly-rounded rational, per-function budget, 1e-12 absolute floor).

Scope. Modularity first. A CPM analogue (a converged CPM partition per graph at a fixed gamma) can follow the same recipe later if wanted.

## Acceptance Criteria

1. Each of verification/oracle/inputs/{karate,dolphins,lesmis}.json carries a new case holding meso's converged modularity partition (documented fixed seed), scored for modularity at gamma=1 (optionally a CPM analogue at a fixed gamma).
2. `make oracle-lean` regenerates the golden vectors; each new case has an exact rational value ({num,den,approx}) and its predicate vector.
3. The Go value-oracle harness (mes-nqky) asserts meso's float64 modularity of that partition is within the committed tolerance of the golden exact value.
4. The committed partition is meso's deterministic output for the documented seed, and a corpus/regression test pins it so drift is caught (relates to mes-5wqp).
5. For karate, the emitted quality is corroborated against the published optimum (about 0.4198) in a comment or the reference cross-check note, without hand-entering it as the input partition.
6. docs/specs/001-initial-implementation/meso-oracle.md no longer lists this under Open questions; this ticket closing is what settles the residual.
7. `make validate` green.

## Notes

**2026-07-18T00:41:59Z**

Added meso's converged modularity partition as a new oracle case for each corpus graph (karate/dolphins/lesmis), at the documented default seed 0, gamma 1. Exact values: karate 49/117 (~0.4188, corroborates published ~0.4198), dolphins 26461/50562 (~0.5233), lesmis 380779/672400 (~0.5663). Predicates per case: connected=true, gammaDense=false, gammaSeparated=true, subsetOptimal=null (n>16). New drift/regression pin TestOracleConvergedPartitionPinned re-runs Leiden and matches the committed input case; existing TestOracleQualityValues now asserts the float64 modularity within tolerance of the exact value; guarantees_test vectors validate the predicate flags. meso-oracle.md 'Deferred work' section replaced with 'Converged-partition case'; stale golden_test.go comment updated. CAVEAT: the Lean toolchain (mathlib) is not installed in this environment, so 'make oracle-lean' could not be run. Golden vectors were produced by a big.Rat mirror of Lean's modularityQ, validated to reproduce byte-equal num/den for every pre-existing committed modularity value across all six fixtures, with predicates taken from meso's Lean-proved-equal Go deciders and approx via %.6f (matches all committed approx). A confirmatory 'make oracle-lean' should be run when Lean is available; it is expected to reproduce these vectors byte-identically. make validate green.
