# meso: design for a best-in-world Go community-detection library

Status: draft.

`meso` is a standalone, GPLv3, pure-Go community-detection library
(Leiden and Louvain), published in its own repository at
`github.com/andreswebs/meso`. This document is the plan of record until the meso
repository exists and carries its own docs.

## 1. Summary and goals

`meso` implements graph community detection in pure Go, correct and deterministic,
tested exhaustively against the canonical reference implementations and the
literature. The name refers to the mesoscale structure of a network (the level
between individual nodes and the whole graph), which is exactly what community
detection recovers.

Goals:

- A faithful, best-in-class Leiden implementation (Traag, Waltman, van Eck, 2019),
  with Louvain as the baseline and a shared quality-function core.
- Bit-reproducible output for a given input, seed, and parameters, including under
  parallelism.
- Correctness anchored by a proven Lean value-oracle for the numeric core and by
  empirical verification of the algorithm's formal guarantees, with the reference
  implementations used for porting and a one-time cross-check.
- Idiomatic, dependency-free public API with an optional gonum adapter.

Non-goals: graph layout/visualization (the VOS technique in the Java reference is
out of scope), graph storage, or anything beyond partitioning a weighted graph
into communities.

## 2. Scope

In scope:

- Algorithms: Leiden and Louvain, plus hierarchical/multi-level output.
- Quality functions: modularity (with a resolution parameter) and CPM (Constant
  Potts Model) for undirected graphs; directed modularity for directed graphs.
- Graph types: undirected weighted and directed weighted, with node weights/sizes;
  self-loops and parallel edges folded into edge weights.

Out of scope: layout, directed CPM (CPM stays undirected, matching convention),
overlapping communities, dynamic/streaming updates (possible later).

## 3. Public API and data model

- Dependency-free core. Internally a compact CSR representation (offset, neighbor,
  and weight arrays) over dense integer node indices, which is what makes the
  fast-local-move queue and aggregation cheap.
- A builder accepts weighted edges and optional node weights, mapping arbitrary
  caller keys to dense indices. The result is a `Partition` (community per node),
  the level hierarchy, and the achieved quality score.
- Options via functional options: quality function (modularity or CPM), resolution
  parameter, random seed, iteration limits, parallelism degree, and the CPM/
  modularity/directed selection.
- An optional gonum adapter ships as a nested module
  (`github.com/andreswebs/meso/gonum`, its own go.mod) so the core never pulls
  gonum into consumers that do not want it.

Sketch:

```go
g := meso.NewBuilder().
    AddEdge("a", "b", 1.0).
    AddEdge("b", "c", 2.0).
    Build()

part, err := meso.Leiden(g, meso.WithQuality(meso.Modularity(1.0)), meso.WithSeed(42))
// part.Communities(), part.Levels(), part.Quality()
```

## 4. Algorithms

### 4.1 Leiden

Three phases iterated to stability:

1. Fast local moving: a queue-driven local move. Pop a node, move it to the
   neighboring community with the best quality gain; if it moved, re-enqueue its
   neighbors not already queued. Repeat until the queue is empty.
2. Refinement: within each community from phase 1, every node starts as its own
   singleton; nodes are merged by a randomized, quality-gain-weighted choice, but
   only singletons are merged and only into sub-communities that are well-connected
   to the rest of their community. This is what guarantees connected communities
   and lifts modularity above Louvain.
3. Aggregation: build an aggregate network whose nodes are the refined
   sub-communities, but seed the aggregate's initial partition from the
   non-refined phase-1 partition. Recurse from phase 1 on the aggregate.

Correctness-critical subtleties (the parts that fail silently, not loudly): the
aggregate's initial partition must come from the non-refined partition; the
refinement's well-connectedness gate and randomized selection must be exact; and
self-loops carrying internal edge weight plus node sizes must be preserved through
aggregation for the resolution term.

An optional outer iteration (`WithIterations(k)`, default 1) reruns the whole
three-phase pass from the previous pass's partition, with a per-pass derived
refinement seed. A single pass has a structural blind spot: refinement only
re-divides the communities of the pass that discovered them, so a community
assembled across aggregation levels is never re-examined at node granularity,
and a converged output can still admit a strictly improving split. This is
Leiden as specified, not an implementation gap: the paper's subset-optimality
guarantee is asymptotic over repeated randomized iterations, never a per-run
property, and reference implementations exhibit the same per-run behavior.
Extra passes re-enter phase 1 at base granularity, giving refinement fresh
randomness over the final communities. Quality is non-decreasing in the pass
count, the run stays a pure function of (graph, options, seed), and the count
is deliberately fixed rather than "until no pass improves": a non-improving
pass is no evidence that the next randomized pass cannot improve, so an
until-stable stopping rule would suggest a convergence that does not exist.

### 4.2 Louvain

The baseline: local moving plus aggregation, without refinement. Shares the CSR,
quality-function, and aggregation machinery with Leiden. Shipped both as a useful
algorithm in its own right and as an internal differential baseline.

### 4.3 Quality functions

A `QualityFunction` interface with three implementations at launch: modularity
with a resolution parameter, CPM (undirected), and directed modularity (the
Leicht-Newman formulation, for directed graphs). The interface leaves room for
significance, surprise, and RBER variants later.

### 4.4 Determinism model

Bit-reproducible as a pure function of (input representation, seed, params). We own
a seedable PRNG, canonicalize internal iteration order (sorted adjacency, stable
tie-breaks), and derive each node's refinement randomness from
`hash(globalSeed, nodeID)` so it is independent of scheduling. A canonicalization
helper lets callers get order-independence across differently-ordered inputs. We do
not attempt to match any reference implementation's PRNG stream; that is infeasible
cross-language and brittle.

### 4.5 Deterministic parallelism

Parallel from the start, without sacrificing reproducibility. The fast-local-move
runs in synchronous rounds: within a round, every active node computes its best
move against the frozen previous-round community assignment, and moves are then
applied in a deterministic order (node-index tie-break). Because each round's
decisions depend only on the prior round's state plus fixed tie-breaks, output is
identical regardless of core count. Refinement randomness is per-node seeded (see
4.4), so it too is scheduling-independent. Synchronous updates can oscillate and
converge differently from a sequential sweep; candidate mitigations to settle in
implementation are a graph-coloring partition of independent (non-adjacent) nodes
that can move simultaneously without conflict, or strictly-positive-gain acceptance
with deterministic conflict resolution. The deep test suite (section 6) is what
guards this.

Priorities: correctness and determinism first, then speed. Target: competitive
with the Java reference and far faster than a naive implementation. We do not claim
to beat the C++ `libleidenalg`.

## 5. Reference implementations and porting strategy

Because meso is GPLv3 (section 9), we may study and port directly from GPL sources,
not merely treat them as black-box oracles. The reference set:

- The paper: Traag, V. A., Waltman, L., van Eck, N. J. (2019). "From Louvain to
  Leiden: guaranteeing well-connected communities." Scientific Reports.
- The Java reference `networkanalysis`
  (<https://github.com/CWTSLeiden/networkanalysis>), the canonical clean
  implementation by the algorithm's authors.
- The Python reference `leidenalg` (<https://github.com/vtraag/leidenalg>) and
  igraph (<https://github.com/igraph/igraph>), which is what graphify uses via
  graspologic.

Strategy: derive the algorithm structure from the paper, port data structures and
phase logic from the Java `networkanalysis` (clean OOP, close to the paper), and
cross-check semantics against `leidenalg`/igraph, especially for directed graphs
where the Java reference does not help. Beyond porting, the references serve one
standing purpose: a one-time spec-blessing cross-check (section 6.2) that confirms
the Lean formalization of the quality functions matches the references' own
definitions. They are not run as a continuous differential oracle.

## 6. Testing strategy

Exhaustive. Correctness of community detection has no single right answer, and the
hard bugs are silent (a worse partition, not a crash), so validation is the bulk of
the work.

### 6.1 Test corpus

Canonical graphs with known structure:

- Zachary's Karate Club: a 34-node, 78-edge social network from Wayne Zachary's
  1977 study of a university karate club that split into two factions after a
  dispute between the instructor and the administrator. The friendship graph
  predicts the real split, so it is the canonical ground-truth benchmark for
  community detection (built into networkx, igraph, graspologic). Modularity
  maximization finds a handful of fine communities that merge into the two real
  factions.
- Dolphins (62-node dolphin social network), Les Miserables (character
  co-occurrence), and other standard small graphs with published partitions.
- LFR benchmark graphs (Lancichinetti, Fortunato, Radicchi): synthetic graphs with
  planted communities and a tunable mixing parameter, generated across a difficulty
  sweep. Recovery is scored with NMI (normalized mutual information) and ARI
  (adjusted Rand index) against the planting.

### 6.2 Oracle: the Lean model as a proven value-oracle

The numeric core is validated against the formally verified Lean model, not
against a running cross-language harness. For a verified library the proven model
is a stronger oracle than a second unproven implementation: it is the exact
artifact the proofs reason about. See
[specs/001-initial-implementation/meso-oracle.md](specs/001-initial-implementation/meso-oracle.md) for the full design.

- Lean value-oracle: a computable rational mirror of the quality functions
  (`Meso/Compute.lean`, proved equal to the real model) emits exact expected
  values as committed golden vectors. Go asserts its `float64` results land within
  a float-rounding tolerance of those exact rationals. This covers quality (`Q`,
  CPM), the move-delta, and the invariant predicates. It does not cover directed
  modularity, which the Lean model deliberately descopes.
- Output correctness is checked against the proven invariants (every community
  connected, quality monotone across levels, subset-optimality at convergence),
  not against an empirical envelope. This is the actual Leiden specification, not
  a proxy for it.
- Output accuracy is scored by NMI/ARI against planted ground truth on LFR graphs
  (which carry their ground truth by construction) and against published
  literature values on the canonical corpus. Neither needs a running reference.
- The Java and Python references are retained for porting and a one-time
  spec-blessing cross-check (confirming the Lean formalization matches their
  definitions, and validating the directed evaluator Lean does not model). They
  run out of band, never in CI. The main Go CI stays pure-Go, reading the
  committed Lean golden vectors.

### 6.3 Test tiers

Baseline, always in CI:

- Property-based invariants: every community connected; quality monotonic
  non-decreasing across aggregation levels; deterministic output for a fixed seed;
  degenerate graphs (empty, single node, disconnected, isolated nodes) handled.
- Native fuzzing: random, degenerate, weighted, self-loop, and multi-edge graphs
  never panic, always yield a valid partition, and always satisfy the invariants.

Deeper tiers (all committed):

- LFR benchmark plus NMI/ARI accuracy scoring against planted ground truth,
  within literature-derived recovery thresholds across the mixing-parameter
  sweep.
- Formal-guarantee verification: empirically assert the paper's guarantees on
  fixtures (gamma-separation, gamma-connectivity, and subset-optimality at
  convergence). This is the essence of a best-in-world Leiden.
- Benchmark plus performance-regression tracking (benchstat) across small to large
  graphs, versus Louvain and versus the references' runtime, with allocation
  tracking and a CI guard.
- Mutation testing on the core, targeting a high mutation score, to prove the suite
  actually kills injected bugs.

### 6.4 Determinism tests

Run every algorithm several times at a fixed seed and assert byte-identical
partitions, across serial and parallel execution and across core counts.

## 7. Formal verification

meso's correctness properties are unusually well specified (the Leiden paper
states them as theorems), which makes it a strong candidate for formal
verification in the sense of Graciolli and Amin ("You Don't Know Jack About
Formal Verification," ACM Queue, 2026): choose the properties worth guaranteeing,
express them in a verification-aware language, and let a verifier (increasingly
with AI-drafted proofs, with the verifier as the external authority) establish
them for all reachable states. This is an optional depth tier layered on top of
the test suite in section 6, not a v1 gate.

The essential boundary first: formal verification can guarantee meso's structural
and safety properties and its concurrency determinism, but it cannot establish
that the communities are good. Modularity maximization is NP-hard and Leiden is a
heuristic, so there is no optimality theorem to prove; partition quality stays
empirical and remains the job of the LFR and NMI/ARI benchmarks (section 6).
Verification replaces "we tested the invariants" with "the invariants cannot
break"; it does not replace accuracy benchmarking. Floating-point exactness is
likewise not worth proving: model the quality function over rationals/reals and
keep floating point an implementation detail (our determinism-by-sorting already
makes summation order canonical, which is the floating-point property that
matters).

Chosen toolset: Lean is the anchor prover (it covers V1, the V2 design proof, and
V3 in one system with Mathlib's graph-theory and real-analysis scaffolding); Gobra
verifies the actual Go for concurrency safety; TLA+ is an optional design
bug-finder. Lean is chosen over Dafny deliberately, accepting two costs: proofs are
tactic-driven and more manual than Dafny's SMT automation (AI-assisted per the
article), and Lean cannot emit Go, so its proofs stay model-level.

What is worth proving, in tiers:

- Tier V0 (baseline, already planned): the property-based, fuzz,
  value-oracle, and formal-guarantee-checking tests of section 6 are lightweight
  verification, since they sample and empirically check the same invariants a
  proof would guarantee.
- Tier V1 (verified design; recommended if we invest): model-level, unbounded
  proofs in Lean of the exact invariants: that any returned partition is
  well-formed (each node in exactly one community, indices in range); that every
  community is a connected subgraph (the Leiden connectivity guarantee); that
  quality is monotonically non-decreasing across aggregation levels; and that the
  iteration terminates.
- Tier V2 (concurrency; highest value per unit effort): the deterministic-parallel
  scheme (section 4.5) is where interleavings hide bugs property tests
  statistically miss. Split across model and code:
  - Design: prove in Lean that a synchronous round is confluent (its outcome is
    independent of the order moves are applied, hence of interleaving and core
    count). As a full logic Lean gives an unbounded proof, strictly stronger than
    bounded model checking, discharging the "same partition serial or parallel,
    any core count" promise. TLA+ is optional here: it proves nothing Lean cannot,
    but TLC finds an interleaving bug in minutes with a concrete counterexample
    trace during design iteration, which a failed Lean proof does not give. Use it
    as a cheap bug-finder before committing to the Lean proof, not as a required
    obligation.
  - Code: prove data-race freedom and memory safety of the parallel core on the
    real Go with Gobra (the separation-logic verifier that targets Go). This is the
    only proof in the stack that touches the shipped artifact rather than a model.
    Scope it to the parallel hot loops and treat it as best-effort: Gobra is
    research-grade and may not handle every Go feature.
- Tier V3 (research-grade; skip unless meso becomes a formal-methods showcase): a
  machine-checked proof in Lean that the algorithm realizes the paper's full
  guarantees (gamma-separation, gamma-connectivity, subset-optimality at
  convergence). Paper- or thesis-scale, but it lives in the same Lean development
  as V1.

The verified-design, tested-code gap: Lean and TLA+ reason about models, not Go, so
functional correctness is verified at the design level and the Go implementation is
held faithful to it by the section 6 tests (the Lean value-oracle, property, and
formal-guarantee checks). Only data-race freedom is verified on the actual Go, via
Gobra. This is a deliberate posture: for a heuristic library, a verified design plus
an exhaustively tested implementation plus proven race-freedom is the right
cost/assurance trade. AI-assisted proof drafting (per the article) lowers the labor
of the Lean and Gobra proofs; the human contribution is choosing the properties,
which for meso the literature has largely pre-decided.

## 8. Performance and concurrency

See 4.5. Deterministic-parallel from the start; CSR layout; reused buffers and no
hot-path allocation; correctness and determinism prioritized over raw throughput.

## 9. Repository, license, and release

- Repository and module: `github.com/andreswebs/meso`, package `meso`. Standalone
  module, go-gettable, independently versioned. Optional gonum adapter as a nested
  module `github.com/andreswebs/meso/gonum`.
- License: GPLv3 (full copyleft). This binds importers: any Go program linking meso
  becomes a derivative work and must be GPLv3-compatible. The upside is that we may
  derive from GPL references (section 5).
- Release and CI, full library-grade works:
  - Fast CI on every push: Go version matrix, race detector, `golangci-lint`,
    coverage gate, short fuzz, and the Lean golden-vector tests.
  - Nightly / on-demand heavy jobs: mutation testing, long fuzz, and benchmarks.
    The one-time reference spec-blessing cross-check (section 6.2) is manual, run
    only on a reference or formalization change, not scheduled.
  - Supply chain: SLSA build provenance and cosign keyless signing on release
    module artifacts and checksums, SHA-pinned actions, and Dependabot.
  - Docs: pkg.go.dev with runnable examples (karate club, LFR); a README covering
    the theory, the guarantees, and benchmarks.
  - Versioning: 0.x until the guarantee and accuracy suites are green, the
    value-oracle checks pass, and the API is stable, then a 1.0 with a
    semantic-versioning promise.

## 10. Milestones

1. Core: CSR graph, builder, modularity and CPM quality functions, Louvain
   (serial), determinism scaffolding, karate/dolphins/Les Mis golden tests.
2. Leiden (serial): all three phases, connectivity and quality invariants,
   formal-guarantee checks, Lean golden-vector tests for the quality functions.
3. Directed support: directed modularity, directed fixtures, hand-computed and
   `leidenalg`/igraph-checked (Lean does not model directed).
4. Deterministic parallelism: synchronous-round local moving, per-node seeded
   refinement, determinism-across-cores tests.
5. Depth: LFR plus NMI/ARI, fuzzing, mutation testing, benchmark suite and
   regression gate.
6. Release engineering: supply chain (SLSA, cosign, pinned actions, Dependabot),
   docs and examples, 1.0 once parity and API are proven.
7. Optional depth: formal verification tiers (section 7) - invariants, parallel
   confluence, and the paper theorems in Lean; data-race freedom of the parallel
   core in Gobra; TLA+ optional as a design bug-finder.

## 11. References

- Traag, V. A., Waltman, L., van Eck, N. J. (2019). From Louvain to Leiden:
  guaranteeing well-connected communities. Scientific Reports, 9, 5233.
- Blondel, V. D., Guillaume, J.-L., Lambiotte, R., Lefebvre, E. (2008). Fast
  unfolding of communities in large networks (Louvain).
- Leicht, E. A., Newman, M. E. J. (2008). Community structure in directed networks.
- Lancichinetti, A., Fortunato, S., Radicchi, F. (2008). Benchmark graphs for
  testing community detection algorithms (LFR).
- Zachary, W. W. (1977). An information flow model for conflict and fission in
  small groups. (The karate club.)
- Java reference `networkanalysis`: <https://github.com/CWTSLeiden/networkanalysis>
- Python reference `leidenalg`: <https://github.com/vtraag/leidenalg>
- igraph: <https://github.com/igraph/igraph>
- gonum: <https://github.com/gonum/gonum>
