# meso v0.2.0 structural measures plan

A TDD-oriented step plan for the second `meso` release, derived from the
graphomania structural-analysis requirements (graphomania spec 003) and from
[meso-design.md](../../meso-design.md). As in
[spec 001](../001-initial-implementation/plan.md), each step names what to
build and the tests to write first (red), worked through the red-green-refactor
loop via `/tdd`.

v0.2.0 widens meso from community detection to mesoscale structure: node
betweenness centrality, community cohesion, the induced subgraph, and the
graph and result accessors a consumer needs to reconcile those answers with
its own edge set. Everything is additive; v0.1.0 callers keep compiling.

## Assumptions and decisions

- Verification tier: empirical. The new measures sit at the same tier as the
  NMI/AMI/ARI metrics (design section 7): brute-force Go oracle on small
  graphs, closed forms, committed networkx reference values, determinism
  tests. No Lean obligation is added in this spec. The Lean track for these
  measures (a computable rational Brandes and cohesion in `Meso/Compute.lean`
  emitting golden vectors) is sketched separately and deliberately deferred;
  the correspondence divergence register records the gap (Step 8).
- Scope: all four handoff deliverables, in priority order: `Betweenness`,
  `Graph` accessors, `Result` accessors with `Cohesion`, `Subgraph`. Plus the
  Go 1.27 toolchain upgrade and a builder change that drops zero-weight edges.
- Zero-weight edges: decided with the plan owner on 2026-10-07 that the
  builder drops them, so every new symbol can equate "edge" with "entry in the
  adjacency" without a weight test. This is a deliberate v0.1.0 behaviour
  change (Step 1).
- The int64-keyed gonum helpers are decided (2026-10-07) to follow the core
  `v0.2.0` tag rather than live in this plan: `gonum/go.mod` requires the
  published core with no `replace` and the repo has no `go.work`, so helpers
  calling the new core API cannot compile under `make validate` until the tag
  exists. See "Follow-up after the core tag".
- Directed graphs are supported by every new symbol from the start. Only the
  normalization constants and the arc-counting rules differ, and they are
  pinned below so a later change is not needed.
- `Betweenness` returns `map[string]float64`, symmetric with
  `Result.Communities()`. It takes no options; a `WithParallelism` variant,
  if ever wanted, is a later additive change (callers of a function with a
  new variadic parameter keep compiling).
- Release engineering is out of scope. The plan ends feature-complete and
  documented; tagging `v0.2.0` and `gonum/v0.2.0` follows as a separate
  user-driven checklist (see exit criteria).
- The level hierarchy (Step 9) and the mutation-target changes (Step 10)
  were added on 2026-10-08, after steps 0 to 8 had landed, and decided with
  the plan owner: both block the `v0.2.0` tag.
- Every step ends with `make validate` green before moving on.

## Pinned semantics

These are the definitions the tests encode. They are restated in the design
doc in Step 8 so the design of record describes the whole public API.

Builder:

- An edge or arc whose folded weight is zero is not stored: `Build` omits it
  from the adjacency. Its endpoints are still registered as nodes, so they
  survive as isolated nodes. A self-loop of folded weight zero is likewise
  absent. Negative and NaN weights stay errors. Every rule below therefore
  reads "edge" as "entry in the adjacency", which always has positive weight.

Graph accessors:

- `Keys()` returns every key in dense index order. With `Canonical()` that is
  ascending key order. The slice is a copy.
- `NumEdges()` counts distinct unordered pairs (undirected) or distinct arcs
  (directed), self-loops excluded.
- `Degree(key)` counts distinct neighbours, self excluded. On a directed graph
  it is the size of the union of out-neighbours and in-neighbours, so a node
  reachable both ways counts once and `len(Neighbors(k)) == Degree(k)` holds
  on every graph.
- `Neighbors(key)` returns distinct neighbours in dense index order, self
  excluded; nil for an absent key. On a directed graph it is the sorted union
  of out- and in-neighbours.
- `Weight(a, b)` returns the folded weight and `true` when the edge exists.
  On a directed graph it follows the arc `a -> b`, so `Weight(a, b)` and
  `Weight(b, a)` may differ. `Weight(a, a)` reports the folded self-loop
  weight and `true` when it is positive. An absent key yields
  `(0, false)`.

Result accessors:

- `NumCommunities()` returns the label count; labels are dense in
  `[0, NumCommunities)`.
- `Members(label)` returns the member keys in dense index order; nil for a
  label outside the range.
- `Cohesion(label)` is the internal density of the community: distinct edges
  with both endpoints inside it, self-loops excluded, divided by `k(k-1)/2`
  for `k` members on an undirected graph and by `k(k-1)` (distinct arcs) on a
  directed one. The directed form reduces to the undirected one on a
  symmetric graph. Fewer than two members, or an unknown label, gives 0.
  Cohesion counts edges, never weights.

Subgraph:

- `Subgraph(g, keys)` returns the graph induced by `keys`: those nodes, every
  edge or arc between them with its folded weight, their self-loops, and
  their node weights, with the same directedness as `g`.
- The result is always canonically indexed, whatever `g`'s indexing, so a
  Leiden pass on it is a pure function of the member set.
- An unknown key is an error wrapping `ErrUnknownKey`; a key listed twice is
  an error wrapping `ErrDuplicateKey`. Both carry the offending key in the
  message. An empty key list yields an empty graph, not an error.

Level hierarchy (Step 9):

- `NumLevels()` returns the number of levels of the run; `Level(i)` returns
  level `i`'s community label per caller key, labels dense, nil for `i`
  outside `[0, NumLevels)`; `LevelQuality(i)` returns that level's quality
  under the run's objective (0 out of range, documented). The last level
  equals `Communities()` and its quality equals `Quality()`.
- A level is the base-graph partition the run reports after that level's
  local moving: Louvain's level partitions, and Leiden's non-refined phase-1
  partition lifted to the base graph. Quality is non-decreasing along the
  levels.
- Louvain's levels nest: every community of level `i+1` is a union of level
  `i` communities. Leiden's levels are not promised to nest, because each
  level after the first is computed on an aggregate of the refined
  sub-communities, which a coarser level can split differently; the godoc
  says so.
- Under `WithIterations(k)`, the levels are the final pass's only, so the
  last level is always the reported result.
- Levels are stored internally as dense partitions with their qualities, so a
  later `LevelResult(i) *Result` can wrap one through the same constructor as
  the run's own result without an API break. That accessor is not part of
  v0.2.0.
- Deterministic like the result: byte-identical across runs for a fixed seed,
  and across worker counts under `WithParallelism`.

Betweenness:

- Brandes (2001) over unweighted shortest paths: every edge is one hop, edge
  weights are ignored, self-loops contribute nothing. Path counts are
  `float64` (as in networkx), so large graphs cannot overflow an integer.
- Undirected: each unordered pair counts once; the raw dependency sum is
  halved and divided by `(n-1)(n-2)/2`. Directed: paths follow out-arcs; the
  raw sum is divided by `(n-1)(n-2)`. Values lie in `[0, 1]`.
- Fewer than three nodes: zero for every node. Zero nodes: an empty, non-nil
  map.
- Determinism: sources iterate in ascending dense index, neighbours are
  visited in adjacency order (the CSR lists are already sorted by dense
  index), the dependency accumulation walks the Brandes stack in reverse
  discovery order. No map iteration on the hot path. The result is therefore
  byte-identical across runs and machines for the same graph.

## Step 0: Go 1.27 toolchain

Build: bump the `go` directive in both `go.mod` files (core and `gonum/`) to
the installed 1.27 patch release, run `go fix ./...` in each module to apply
the modernizers, `make tidy`, and regenerate the committed benchmark baseline
(`make bench-baseline`) so later bench gates compare like with like. CI reads
`go-version-file: go.mod`, so no workflow edit is needed; confirm the pinned
`golangci-lint` action version supports Go 1.27 and bump it (by commit SHA) if
not.

Test-first / verify: `make validate` and `make test-race` green on 1.27 in
both modules before any feature code; `make bench-check` green against the
regenerated baseline; `make vulncheck` green.

## Milestone 1: accessors

### Step 1: builder drops zero-weight edges

Build: in `Build`, skip any folded edge or arc whose total weight is zero when
filling the adjacency lists. Endpoint registration in `AddEdge` is unchanged.
Update the godoc of `AddEdge` and `Builder` to state the rule.

Test-first:

- Rewrite `TestBuilder_ZeroWeightsAccepted`: a zero-weight edge still builds
  without error and registers both endpoints, but leaves no neighbour entry;
  a zero-size node keeps size 0 (unchanged).
- Directed: a zero-weight arc leaves neither an out- nor an in-entry; a
  zero-weight arc beside a positive reverse arc keeps only the positive one.
- Parallel edges `0` and `w > 0` fold to one entry of weight `w`.
- No churn: golden corpus, oracle, determinism and benchmark suites unchanged
  (the corpus and fuzz generators carry no zero edge weights).
- Leiden and Louvain on a graph with a zero-weight bridge treat the two sides
  as disconnected (the connectivity guarantee now matches the fuzz
  generator's existing "a zero-weight pair is a non-edge" convention).

### Step 2: Graph accessors

Build: `Keys`, `NumEdges`, `Degree`, `Neighbors`, `Weight` on `*Graph`,
backed by the CSR without allocation beyond the returned slices. `Weight`
uses the existing CSR point lookup; `Neighbors` and `Degree` on a directed
graph merge the two sorted adjacency lists.

Test-first:

- `Keys()` is in dense order and equals ascending key order under
  `Canonical()`; mutating the returned slice does not affect the graph.
- `NumEdges()` on hand-built fixtures: parallel edges fold to one, self-loops
  excluded; directed reciprocal arcs count two.
- `Degree`/`Neighbors` agree (`len(Neighbors(k)) == Degree(k)`) on every
  corpus graph and on fuzzed graphs; directed union counts a two-way
  neighbour once; absent key gives `(0, false)` and nil.
- `Weight` round-trips every folded edge of a fixture; asymmetric on a
  directed fixture; self-loop weight reported for `(k, k)`; absent for a
  non-edge and for an unknown key.
- Reconciliation: for every corpus GML, the sum of `Degree` over `Keys()`
  equals `2 * NumEdges()` (undirected) and the GML edge count matches
  `NumEdges()` after folding.

### Step 3: Result accessors and Cohesion

Build: `NumCommunities`, `Members`, `Cohesion` on `*Result`. `Members`
builds the per-label key lists lazily once (dense order falls out of a single
pass over the partition) and caches them on the result.

Test-first:

- `NumCommunities()` equals `max(label)+1` and the distinct-label count of
  `Communities()` on every corpus result.
- `Members(l)` for all labels partitions `Keys()` exactly; each list is in
  dense order; an out-of-range label gives nil.
- `Cohesion` closed forms: a clique community gives 1; a community with no
  internal edges gives 0; a singleton gives 0; a hand-built 4-node community
  with 3 internal edges gives 0.5; the directed form on a symmetric graph
  equals the undirected form; a one-way arc pair gives `1/(k(k-1))` per arc.
- Cohesion ignores weights: the same topology with unit and random weights
  gives identical values.
- Cohesion ignores self-loops: adding self-loops to members changes nothing.
- Brute force: on small random graphs and random partitions, compare with a
  direct count over all member pairs.

## Milestone 2: Subgraph

### Step 4: Subgraph

Build: `Subgraph(g, keys) (*Graph, error)` and the sentinel errors
`ErrUnknownKey`, `ErrDuplicateKey`. Implementation feeds a `Canonical()`
builder of the same directedness: for each member in dense order, add every
edge or arc to a member (undirected: `j > i` only, so each pair is added
once), add its self-loop via `AddEdge(k, k, w)` when present, and set its
node weight. Going through the public builder means the result carries every
builder invariant and validation for free.

Test-first:

- Induced edges: for every pair of members `Weight` agrees between `g` and
  the subgraph; no non-member key is present; `NumEdges` equals the count of
  `g`'s edges with both endpoints in the set.
- Self-loops and node weights survive; directedness is preserved and arc
  direction is kept.
- Canonical regardless of source: a non-canonical `g` built in two insertion
  orders gives subgraphs with identical `Keys()` and identical Leiden output
  under a fixed seed.
- Errors: unknown key wraps `ErrUnknownKey` and names it; a duplicate wraps
  `ErrDuplicateKey`; the empty list builds an empty graph.
- Identity: `Subgraph(g, g.Keys())` on a canonical `g` equals `g` in keys,
  edges, weights, and Leiden output.
- Re-split use case: on karate, take the largest Leiden community, subgraph
  it, run Leiden again; the inner labels are dense and the inner graph is
  connected (the invariant graphomania's re-split relies on).

## Milestone 3: Betweenness

### Step 5: Brandes betweenness, undirected and directed

Build: `Betweenness(g *Graph) map[string]float64` in a new `centrality.go`,
with the scratch arrays (distance, path count, dependency, predecessor
lists, queue, stack) allocated once per call and reused across sources.
Normalization constants per the pinned semantics.

Test-first, in this order:

- Brute-force oracle: on random undirected and directed graphs of up to 12
  nodes (including disconnected ones and ones with self-loops), enumerate all
  shortest paths per pair by BFS and compare with a tolerance of 1e-12.
- Closed forms: a star of `n` leaves gives the centre 1.0 and the leaves 0; a
  path of `n` nodes gives node `i` (0-based) the value
  `2 i (n-1-i) / ((n-1)(n-2))`; a complete graph gives 0 everywhere; a
  directed cycle of `n` nodes gives every node exactly 1/2 (each node lies
  strictly between `(n-1)(n-2)/2` of the `(n-1)(n-2)` ordered pairs).
- Degenerate: zero, one, and two nodes give all-zero (or empty) maps; weights
  are ignored (random reweighting of a fixture changes nothing); self-loops
  change nothing; node weights change nothing.
- Determinism: the same graph built in two insertion orders with
  `Canonical()` yields maps equal under `math.Float64bits`; repeated calls
  are bit-identical.
- Allocation: a benchmark-backed assertion that the per-source loop does not
  allocate (allocations are proportional to `n`, not to `n * m`).

### Step 6: reference values from networkx

Build: a one-off PEP 723 script under `datasets/` (run with `uv run`, depends
on networkx) that reads a corpus GML and writes `betweenness.csv` (key,
value at full float precision) next to it, for karate, dolphins, lesmis, and
the directed celegansneural (weights ignored in both tools). Commit the
script and the four CSVs; the Go tests read the CSVs with the existing GML
loader's conventions. The script is out-of-band tooling like the oracle
converters; CI stays pure Go.

Test-first:

- All 34 karate values within 1e-9 of networkx
  `betweenness_centrality(normalized=True)`; spot-check that node 1 is about
  0.4376, node 34 about 0.3040, node 33 about 0.1452, as published.
- dolphins and lesmis undirected, celegansneural directed, each within 1e-9.
- The CSV round-trip is itself tested (every key in the CSV is a graph key
  and vice versa), so a stale reference file fails loudly.

## Milestone 4: hardening

### Step 7: benchmarks, fuzz, mutation

Build: `BenchmarkBetweenness` on karate, lesmis, and an LFR corpus graph;
`BenchmarkSubgraph` on the largest karate community; baseline entries added
by rerunning `make bench-baseline` (the Step 0 baseline predates these
names). New fuzz targets: `FuzzBetweenness` (values in `[0, 1]`, bit-identical
across two insertion orders, equal to the brute-force oracle when `n <= 10`)
and `FuzzSubgraph` (induced weights match `g.Weight`, canonical keys,
builder invariants hold). Then `make mutation`.

Test-first / verify:

- `make bench-check` green with the new entries; `make test-race` green.
- Fuzz corpora seeded with the closed-form fixtures; no panics across a
  timed run.
- Mutation efficacy stays above the committed threshold; survivors in the new
  files are triaged into `docs/specs/learnings.md` as spec 001 did, with real
  gaps turned into tests in the owning step.

## Milestone 5: documentation

### Step 8: design of record and package docs

Build:

- `docs/meso-design.md`: a new "Structural measures" section carrying the
  pinned semantics above (definitions, normalization, self-loop and weight
  rules, the determinism argument), plus updates to section 2 (scope now
  covers mesoscale structure, not only community detection), section 3 (the
  accessor and `Subgraph` API), section 6 (empirical tier for these
  measures), and section 10 (a v0.2.0 milestone).
- `doc.go`: the scope statement and an example of `Betweenness` and
  `Subgraph` alongside Leiden; `README.md` likewise.
- `verification/lean/CORRESPONDENCE.md`: a divergence-register entry stating
  that betweenness, cohesion, and the induced subgraph are outside the Lean
  model and validated empirically, so the gap cannot pass as coverage.

Verify: `markdownlint-cli2` clean on every touched markdown file; `go vet`
and the doc examples compile as `Example` tests; `make validate` green in
both modules.

## Milestone 6: additions before the tag

### Step 9: level hierarchy on Result

Build: record each level's base partition and quality in the Leiden and
Louvain level loops (Leiden: the final pass of `WithIterations`), store them
on `Result` through `newResult`, and add `NumLevels`, `Level`, `LevelQuality`
with the pinned semantics. Update the design of record: section 2 keeps
hierarchical output in scope, section 3's sketch and API bullet gain the
level accessors and drop the "not yet public" sentence, section 4.6 gains the
level semantics (including the Leiden nesting caveat). Package doc and
correspondence register as needed.

Test-first:

- Last level equals `Communities()` and `LevelQuality(last) == Quality()`,
  for Leiden and Louvain, serial and parallel, on the corpus.
- Every level is a well-formed dense partition over all keys; quality is
  non-decreasing along the levels (matching the existing internal
  `louvainLevels` and Leiden level-monotonicity tests).
- Louvain levels nest; Leiden levels are not asserted to nest.
- `Level` and `LevelQuality` out of range return nil and 0.
- `WithIterations(k)`: levels are those of the final pass (the last level
  equals the result for k > 1).
- Determinism: levels byte-identical across repeated runs and across
  `WithParallelism` worker counts.
- A graph whose first local move merges nothing reports one level, equal to
  the all-singletons result.

### Step 10: mutation target on a clean export

Build: change the `mutation` target in the Makefile so it exports the
committed tree (`git archive HEAD`) to a temp directory outside the repo and
runs gremlins there, excludes `verification/` alongside `gonum/`, and
defaults `MUTATION_WORKERS` to 4. Document in the target's comment that it
mutates committed code only. Run it after Step 9 so the confirmation run also
covers the level code.

Verify: `make mutation` passes the committed threshold; mutator coverage is
reported without the reference tool (about 97% before Step 9); the temp
footprint stays in the low hundreds of MB; the `.gremlins.yaml` score comment
and `docs/specs/learnings.md` record the new figures; survivors in the Step 9
code are killed or triaged.

## Exit criteria for this list

All four deliverables present on the public API with the pinned semantics,
undirected and directed; the toolchain at Go 1.27 in both modules; test tier
green: brute-force oracles, closed forms, networkx references, determinism,
reconciliation against the corpus, fuzz, mutation above threshold, and
benchmarks under the regression guard; the design of record, package docs,
and correspondence register updated; the level hierarchy on `Result` (Step 9)
and the mutation target running on a clean export (Step 10).

Tagging follows as a user action: tag the core `v0.2.0`. The graphomania
v0.0.3 gate closes when the core tag exists and its library module pins it.

## Follow-up after the core tag

The gonum adapter helpers, deferred here because they cannot compile against
the published core until `v0.2.0` exists.

Build: in `github.com/andreswebs/meso/gonum`, bump the core `require` to
`v0.2.0` and `make tidy`; add `Betweenness(g *meso.Graph)
(map[int64]float64, error)` and `Members(r *meso.Result, label int)
([]int64, error)`, inverting the decimal key scheme the way `Communities`
does, with the same error on a non-numeric key. The adapter's `Build` already
returns a `*meso.Graph`, so the core accessors and `Subgraph` need no mirror.
Correct the stale `gonum/go.mod` comment that still describes a local
`replace`. Mention the helpers in the package doc. Tag `gonum/v0.2.0`.

Test-first:

- Helpers agree with the core on a gonum-built karate graph (same values,
  int64 keys); a non-numeric-key graph gives the documented error.
- The core module's dependency graph is still free of gonum.
