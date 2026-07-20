# meso implementation plan

A TDD-oriented step plan for building the `meso` Go library, derived from
[meso-design.md](../../meso-design.md). Each step names what to build and,
more importantly, the tests to write first (red), since we are working the
red-green-refactor loop via `/tdd`.

## Assumptions

- The Lean formal-verification setup is done first: tier V1 (well-formed
  partition, connectivity, quality monotonicity, termination) and tier V2 design
  proof (synchronous-round confluence) exist as model-level proofs before the Go
  work in this list begins. The Go steps below hold the implementation faithful
  to that verified design through tests; they do not re-derive it.
- Scope of this list: milestones 1 through 5 of the plan, ending when the
  library is feature-complete, deterministic, and exhaustively tested. Release
  engineering (M6) is out of scope here.
- The oracle is the proven Lean value-oracle: a computable rational mirror of the
  quality functions emits committed golden vectors, and Go checks its `float64`
  output within tolerance (see [specs/001-initial-implementation/meso-oracle.md](../../specs/001-initial-implementation/meso-oracle.md)).
  Building the mirror (`Meso/Compute.lean` plus its equivalence proof) is a
  supporting side-track, not a main step. Where a step consumes those vectors it
  is noted as "(consumes Lean golden vectors)". The Java/Python references are not
  a standing oracle; they serve porting and a one-time spec-blessing cross-check.
- Every step ends with `make validate` green before moving on.

## How the move-delta property test fits

[move-delta-verification.md](../../research/move-delta-verification.md)
defines the property test proving the incremental move gain equals
`Q(after) - Q(before)`. It lands in **Step 3**, immediately after the quality
functions exist and before any optimizer relies on the incremental delta. It is
the gate that lets Louvain (Step 4) and Leiden (Step 6) trust their hot-path
gain computation. It is re-run for every objective added later (CPM in Step 3,
directed modularity in Step 8). It is the Go-side counterpart of the Lean V1
move-delta target.

## Step 0: repo and module scaffolding

Build: the multi-module workspace (`.` core and `gonum/` adapter), `Makefile`
targets, `golangci-lint` config, CI skeleton. No algorithm code yet.

Test-first: a trivial package-level test so `make validate` (fmt, vet, lint,
test) runs green end to end across both modules. This proves the quality gate
before real code exists.

## Milestone 1: core

### Step 1: CSR graph and builder

Build: the compact CSR representation (offset, neighbor, weight arrays over dense
integer indices); the `Builder` that maps arbitrary caller keys to dense indices,
accepts weighted edges and optional node weights, folds self-loops and parallel
edges into edge weights.

Test-first:

- Builder maps keys to stable dense indices; round-trips keys back out.
- Parallel edges sum into one weight; self-loops recorded as node-internal weight.
- Degenerate inputs: empty graph, single node, isolated nodes, disconnected
  components all build a valid CSR.
- CSR invariants: offsets monotonic, neighbor/weight arrays aligned, degree sums
  consistent with `2m`.
- Directed builds keep in/out separable (foundation for Step 8).

### Step 2: quality function interface, modularity, CPM

Build: the `QualityFunction` interface; modularity with a resolution parameter;
CPM (undirected). Full from-scratch evaluators (the simple, slow definitions),
not yet the incremental deltas.

Test-first:

- Modularity `Q` of hand-computed tiny graphs matches by hand (a triangle, a
  path, two disjoint edges) at gamma = 1 and other gamma.
- CPM of the same fixtures matches by hand.
- `Q` of the all-in-one-community and all-singletons partitions equal their known
  closed forms.
- Resolution behaves monotonically: higher gamma never yields fewer/larger
  communities at the objective level (property check on the score).

### Step 3: incremental move-delta (move-delta-verification.md)

Build: the incremental gain functions used by the optimizers: gain of moving a
node into a candidate community, for modularity and CPM.

Test-first: the property test specified in
[docs/research/move-delta-verification.md](research/move-delta-verification.md).
Random weighted graphs with self-loops, disconnected parts, and isolated nodes;
random partitions and legal moves (including no-op and empty-target); assert the
incremental delta equals the from-scratch `Q(after) - Q(before)` within
tolerance. Explicit cases: no-op move equals exactly zero; singleton in/out;
self-loop on the moved node. One test per objective. This step gates all
optimizer work.

### Step 4: Louvain (serial) and aggregation

Build: local moving plus aggregation (no refinement). The aggregation machinery
(super-nodes, preserved self-loops and node sizes) shared later with Leiden.

Test-first:

- Aggregation preserves total edge weight and the objective score across a level
  (quality of the aggregate equals quality of the expanded partition).
- Local moving never decreases the objective (monotonic per pass).
- Louvain on the corpus yields the expected community count / known partition
  shape.
- Round-trip: expanding an aggregate partition to the base graph is consistent
  with community assignments.

### Step 5: determinism scaffolding and golden corpus

Build: the seedable PRNG we own; canonical iteration order (sorted adjacency,
stable tie-breaks); per-node refinement randomness derived from
`hash(globalSeed, nodeID)`; the canonicalization helper for order-independent
inputs.

Test-first:

- Byte-identical partition across repeated runs at a fixed seed.
- Differently-ordered inputs (shuffled edge insertion) canonicalize to identical
  output.
- Golden tests: Karate, dolphins, Les Miserables produce committed golden
  partitions/quality at fixed seed. The exact Go partition is pinned in-repo
  (deterministic); the quality value is checked against the Lean golden vector and
  the published literature value for each graph.

## Milestone 2: Leiden (serial)

### Step 6: the three phases

Build: fast local moving (queue-driven, re-enqueue neighbors); refinement
(singletons merged by randomized quality-gain-weighted choice, only into
well-connected sub-communities); aggregation seeded from the non-refined phase-1
partition. Reuse Step 3 deltas and Step 4 aggregation.

Test-first (the correctness-critical subtleties that fail silently):

- The aggregate's initial partition comes from the non-refined partition, not the
  refined one (fixture that distinguishes the two).
- Well-connectedness gate: a fixture where a naive merge would create a
  disconnected community; assert Leiden refuses it.
- Refinement randomness is per-node seeded and scheduling-independent (same
  result regardless of node processing order).
- Self-loops carrying internal weight and node sizes survive aggregation so the
  resolution term stays correct.
- Leiden quality is greater than or equal to Louvain quality on the corpus.

### Step 7: invariants and formal-guarantee checks

Build: nothing new; this is the empirical mirror of the Lean V1/V3 properties.

Test-first:

- Every community is a connected subgraph (the guarantee Leiden adds over
  Louvain) on the whole corpus and on fuzzed graphs.
- Well-formed partition: each node in exactly one community, indices in range.
- Quality monotonic non-decreasing across aggregation levels.
- Termination within iteration limits.
- Formal-guarantee fixtures: gamma-separation, gamma-connectivity, and
  subset-optimality at convergence, asserted empirically on fixtures.
- Value-oracle check: modularity/CPM of fixtures matches the Lean golden vector
  within tolerance (consumes Lean golden vectors); connectivity and community
  count asserted directly as proven invariants, not against a reference envelope.

## Milestone 3: directed support

### Step 8: directed modularity

Build: the Leicht-Newman directed modularity objective and its incremental delta;
directed handling through local moving, refinement, and aggregation. CPM stays
undirected by design.

Test-first:

- Directed `Q` of hand-computed asymmetric fixtures matches by hand.
- Re-run the Step 3 move-delta property test with the directed oracle and the
  directed delta (do not reuse the undirected oracle).
- Directed `Q` of fixtures matches hand-computed values; directed communities
  cross-checked against `leidenalg`/igraph in the one-time spec-blessing pass
  (Lean does not model directed, so there is no Lean golden vector here).
- A symmetric directed graph reduces to the undirected result.

## Milestone 4: deterministic parallelism

### Step 9: synchronous-round local moving

Build: synchronous-round fast-local-move (each active node computes its best move
against the frozen previous-round assignment; moves applied in deterministic
node-index order); settle the confluence mitigation (graph-coloring of
independent nodes, or positive-gain acceptance with deterministic conflict
resolution) chosen to match the Lean V2 design proof.

Test-first:

- Byte-identical partition serial versus parallel, and across core counts
  (1, 2, 4, 8, GOMAXPROCS).
- Race detector clean (`make test-race`) on the parallel core.
- A round's outcome is independent of the order moves are applied (the confluence
  property the V2 proof establishes at the model level, checked here on the code).
- Convergence: synchronous updates reach a stable partition without oscillation on
  the corpus and on adversarial fixtures.

## Milestone 5: depth

### Step 10: gonum adapter

Build: the optional `github.com/andreswebs/meso/gonum` adapter (nested module) so
the core stays dependency-free. Slot here once the public `Builder`/`Partition`
API is stable.

Test-first: adapter converts a gonum graph to a `meso` build and back; produces
the same partition as the core API on shared fixtures; core module's dependency
graph asserted free of gonum.

### Step 11: LFR benchmark and NMI/ARI accuracy

Build: LFR generator (or a bundled generator wrapper) and the NMI, AMI, and ARI
comparison metrics.

Test-first:

- NMI/AMI implemented per Vinh et al. (normalization and chance-adjustment chosen
  deliberately); metric unit tests against hand-computed contingency tables and
  known edge cases (identical partitions score 1; independent partitions score
  near 0 for adjusted variants).
- ARI unit tests against hand-computed values.
- Accuracy sweep: recovery scored by NMI/ARI against planted ground truth across
  a mixing-parameter sweep, staying above literature-derived thresholds. LFR
  graphs carry their ground truth by construction, so no external reference is
  consumed.

### Step 12: fuzzing, mutation testing, benchmarks

Build: native fuzz targets, benchmark suite, benchstat regression tracking.

Test-first / verify:

- Fuzz: random, degenerate, weighted, self-loop, multi-edge graphs never panic,
  always yield a valid partition satisfying all Step 7 invariants.
- Mutation testing on the core reaches a high mutation score (the suite kills
  injected bugs); gaps drive new tests back into the relevant step above.
- Benchmarks across small-to-large graphs versus Louvain and the reference
  runtimes, with allocation tracking and a CI regression guard.

## Exit criteria for this list

Feature-complete Leiden and Louvain (undirected and directed), deterministic
serial and parallel with byte-identical output across cores, the gonum adapter,
and the full test tier green: golden corpus, invariants and formal-guarantee
checks, the Lean value-oracle checks, LFR NMI/ARI above the literature-derived
thresholds, fuzz, mutation score, and benchmarks under the regression guard.
Release engineering (M6) follows next.
