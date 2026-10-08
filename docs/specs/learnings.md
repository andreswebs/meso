# Implementation learnings

Running notes on non-obvious decisions and traps hit while building `meso`,
kept for whoever picks up the next step.

## CSR model (mes-0isk, step 1)

- Self-loops are stored per node in a separate `selfLoops` array, out of the
  neighbour lists. This keeps the resolution term's internal-weight lookup and
  aggregation folding cheap, and it makes the `2m` convention explicit: an
  off-diagonal edge is counted from both endpoints (doubled), a self-loop is
  counted exactly once. `twoM = sum_i degree(i)` where `degree(i)` includes the
  self-loop once, matching the Lean model's `twoM` as the double sum of
  `weight i j`.
- The adjacency (offset/neighbour/weight arrays) is factored into its own
  struct so directed support (M3) can add a second in-arc adjacency without
  reshaping the layout, as the ticket required.
- `Partition` well-formedness is a length-and-range check: length `n` and every
  label in `[0, n)`, since `n` nodes form at most `n` communities. This is the
  Go realization of Lean's structural total-function well-formedness.
- Input validation (`weight_nonneg`, `nodeSize_nonneg`) is deliberately not in
  the low-level `newCSR` constructor. Per the correspondence divergence
  register, those are enforced at the public `Builder` boundary (mes-1ekt), not
  at the raw-array constructor, which is test-only.
- The representation rows in `verification/lean/CORRESPONDENCE.md` are left at
  status `planned` for now: they describe the whole public representation
  including the builder's key-to-index map, which lands with mes-1ekt. Flip
  them when the public core representation is complete.

## Public Builder (mes-1ekt, step 1)

- Keys are `string`, matching the design sketch (`AddEdge("a", "b", 1.0)`). Go
  generics were considered but rejected: `meso.NewBuilder()` cannot infer a type
  parameter, so a generic builder would force `NewBuilder[string]()` at every
  call site and diverge from the design of record. Callers stringify their own
  keys.
- Validation is deferred, not eager. `AddEdge`/`AddNodeWeight` return `*Builder`
  for chaining, so they cannot return an error; instead the first bad input is
  stored on `b.err` and surfaced by `Build()`. This is the only way to keep the
  fluent API and still enforce the boundary invariants.
- The non-negativity check is `!(w >= 0)`, not `w < 0`. This is the faithful
  negation of the Lean `weight_nonneg : 0 ≤ w`: it rejects negatives and NaN
  (`0 ≤ NaN` is false), which a bare `w < 0` would let through to poison `twoM`.
  `+Inf` passes (it is `>= 0`); that degenerate case is left for a later step if
  it ever matters.
- Determinism comes from sorting each node's neighbours by index in
  `flattenAdjacency`, so the CSR layout is independent of both map-iteration
  order and edge-insertion order. Parallel edges are folded in a
  `map[[2]int]float64` keyed by the canonical endpoint pair (unordered `min,max`
  for undirected, ordered `from,to` for directed) before the arrays are built,
  so folding never depends on iteration order either.
- Self-loops (`from == to`) go to `selfLoops[i]`, never the neighbour list, for
  both directed and undirected builders. A directed self-loop therefore counts
  in both `outDegree` and `inDegree`, matching the Leicht-Newman convention M3
  will need.
- The directed path populates a second `in` adjacency on the `csr`;
  `checkInvariants` now validates `in` too (only when `directed`), via a shared
  `adjacency.check` helper so the `out`-only undirected tests are unaffected.
  `degree`/`twoM` stay the undirected quantities; directed code uses
  `outDegree`/`inDegree`.
- Flipped the section-1 representation rows in `CORRESPONDENCE.md` from `planned`
  to a new `landed` status (Go symbol written and covered by tests), as the
  mes-0isk note asked once the public core representation was complete. The
  quality-function and algorithm rows stay `planned`.

## Quality functions - modularity (mes-orqz, step 2)

- The `QualityFunction` interface method takes the internal `*csr`
  (`Quality(g *csr, p Partition) float64`), not the public `*Graph`. That keeps
  quality functions meso-internal (external packages get a value from
  `Modularity(gamma)` and pass it to the options surface, but cannot call
  `Quality` or implement the interface themselves) and lets the move loop run
  directly on the compact model with no wrapping.
- `Modularity(gamma)` is a public constructor returning the interface; the
  concrete `modularity` struct stays unexported. The compile-time assertion is
  `var _ QualityFunction = modularity{}` on the concrete type (staticcheck
  QF1011 flags asserting `Modularity(1.0)`, whose static type is already the
  interface, so that form is not a useful assertion).
- Added `csr.weight(i, j)` mirroring Lean's `G.weight i j`: self-loop weight on
  the diagonal, off-diagonal scan otherwise, 0 when non-adjacent. It is
  O(deg i), used only by the from-scratch evaluator (the oracle), never the hot
  path.
- The evaluator is the literal O(n^2) double sum in row-major order, chosen for
  obvious correctness over speed. Row-major iteration is inherently canonical,
  which is what makes repeated evaluation bit-identical (acceptance criterion 5);
  no extra sorting is needed because the fixed index order already fixes the
  summation order.
- Clean closed forms verified by hand and used as test oracles (self-loop-free
  graphs): all-in-one-community `Q = 1 - gamma`; all-singletons
  `Q = (sum_i w_ii - gamma * sum_i k_i^2 / 2m) / 2m`. Both fall straight out of
  the double sum and make good gamma-sweep checks.
- The degenerate `2m == 0` (empty/edgeless) graph returns `Q = 0` explicitly to
  avoid a divide-by-zero, matching the "valid partition, no crash" fuzz
  expectation later tickets rely on.

## Move-delta - modularity (mes-vy3a, step 3)

- The delta of moving node `u` from its source community `s` to target `t` is
  `(2/2m) * [(W_ut - W_us) - gamma*k_u*(K_t - (K_s - k_u))/2m]`, where
  `W_ut`/`W_us` are the off-diagonal edge weight from `u` into each community and
  `K_t`/`K_s` the summed weighted degrees. It falls out of the double sum because
  moving only `u` touches just row `u` and column `u`, and
  `A_ij = w_ij - gamma*k_i*k_j/2m` is symmetric, so the change is
  `(2/2m) * sum_{j != u} A_uj (delta(t,c_j) - delta(s,c_j))`.
- The moved node's **self-loop never enters the edge term**. The `j == u` term of
  the double sum is `A_uu` with `delta(c_u,c_u) = 1` always, so it is constant
  across the move and cancels. The self-loop enters only through `k_u` in the
  null-model term. Because self-loops are stored off the neighbour list, summing
  `u`'s neighbours by community already excludes `j == u` for free - no special
  case needed. This is the classic off-by-a-factor trap the verification doc warns
  about, and getting it wrong is silent.
- `K_{s\{u}}` is `K_s - k_u`, not a separate scan: the source community's summed
  degree with `u` removed. Do not forget the `- k_u`, or a node moving within a
  large source community is mispriced.
- No-op (`target == s`) returns exactly `0` before any arithmetic; `2m == 0`
  returns `0`, matching `Quality`'s own guard so the identity holds on the empty
  graph too.
- Kept `moveDelta` an unexported method on the concrete `modularity` type, **not**
  on the `QualityFunction` interface. CPM's `Quality` (mes-qvav) lands before CPM's
  delta (mes-l38o); putting the method on the interface now would make qvav's CPM
  fail to satisfy `QualityFunction` and break its build. The Louvain ticket
  (mes-hcvp), which depends on both deltas, is the right place to unify them behind
  the interface.
- The property test's oracle is the from-scratch `Quality` (this ticket's stated
  oracle), asserting `|delta - (Q_after - Q_before)| <= 1e-9` over 5000 random
  trials. At `n <= 50` with weights in `[0.5, 5)` the two agree far tighter than
  `1e-9`; cancellation is not a problem at these magnitudes. Per-trial seeds are
  derived from one base RNG and logged on failure for reproducibility. Tightening
  this to the committed Lean exact deltas is deferred to the value-oracle harness
  (mes-nqky), which owns golden-vector loading.

## CPM(gamma) evaluator (mes-qvav, step 2)

- CPM has **no `2m` normalisation**: `Q = sum_ij (w_ij - gamma*s_i*s_j)*delta`,
  so it carries the units of edge weight, not the dimensionless `[-1, 1]` of
  modularity. Do not reflexively divide by `twoM` when porting from the
  modularity evaluator; the empty-graph guard is just `n == 0 -> sum stays 0`, no
  divide-by-zero case to special-case.
- The penalty is a **flat node-size** term `gamma*s_i*s_j`, which is why the model
  carries `nodeSize` from the start. Node sizes enter only the penalty, never the
  internal-weight term, so `TestCPM_NodeSizesAffectPenalty` scales one node's size
  and checks CPM shifts by exactly `-gamma*(S_c^2_new - S_c^2_old)`.
- Per-community decomposition `e_c - gamma*S_c^2` (Lean `cpm_eq_communitySum`):
  `communityInternalWeight` (`e_c`) is the block sum **including the diagonal**
  (self-loops count toward internal weight), and `communitySize` (`S_c`) is the
  summed node size. Kept both as unexported free functions (not methods) because
  the CPM move-delta (mes-l38o) and aggregation (mes-w60v) reuse them; they are
  the CPM analogue of modularity's per-community regrouping.
- `isGammaDense` is the predicate form `gamma*S_c^2 <= e_c`;
  `TestCPM_GammaDenseContribution` asserts it equals `contribution >= 0` (Lean
  `isGammaDense_iff`) per community over 3000 random trials. This is the Go image
  of the lemma, not a tautology: it pins that the predicate and the
  nonnegative-contribution phrasings never diverge.
- Property tests use a `randomCSRSized` wrapper over the existing `randomCSR` that
  overwrites `nodeSizes` with random values in `[0.5, 3)` (same-package access to
  the unexported field). The default `randomCSR` gives all-ones sizes, which would
  leave the node-size penalty path untested.
- Compared per-community sum to the from-scratch `Quality` with a **relative**
  tolerance `1e-12*(1+|want|)`: CPM values are unbounded (unlike modularity's
  `[-1, 1]`), so a purely absolute tolerance would be too strict on the
  larger-magnitude trials.

## Move-delta - CPM (mes-l38o, step 3)

- The CPM delta mirrors modularity's structure but drops the `2m` factor and
  swaps the null-model term: `2*[(W_ut - W_us) - gamma*s_u*(S_t - S_{s\{u}})]`,
  with `s_u*S` in place of `k_u*K/2m`. No `twoM == 0` guard is needed since there
  is no `2m` divisor.
- The correctness-critical subtlety is the **diagonal term**. In CPM the moved
  node's own cell is `B_uu = w_uu - gamma*s_u^2`; because `u` always shares a
  community with itself, this cell is unchanged by the move and drops out of the
  delta. So the moved node's self-loop weight and its own `s_u^2` penalty must
  **not** appear: `W_us`/`W_ut` sum only over `neighbors(u)`, and self-loops live
  in a separate `csr` field, so they are excluded naturally. `TestCPM_MoveRegressions`
  pins this with a self-loop-carrying moved node.
- `cpmMergeGain(g, gamma, a, b) = 2*(w_ab - gamma*s_a*s_b)` is the Go image of
  Lean `cpm_merge_two_singletons`: the general move-delta specialised to the
  singleton partition (each community one node). `TestCPM_MergeGain` checks it
  three ways - against the formula, the from-scratch oracle, and the general
  `moveDelta` - so the aggregate-level formula and the single-node delta can never
  silently diverge. Its sign is what gamma-separation reads off later.
- Property test (`TestCPM_MoveMatchesOracle`, 5000 trials) reuses `randomCSRSized`
  so non-unit node sizes exercise the penalty path; `cpmMoveTol = 1e-9` absolute
  holds comfortably (observed agreement ~1e-11 at `n <= 50`), so no relative
  tolerance is needed here even though CPM values are unbounded - the delta and
  the oracle difference are both small local quantities, unlike the whole-graph
  `Quality` compared in the community-sum test.

## Aggregation machinery (mes-w60v, step 4)

- The aggregate is built in a single pass over every incident arc, not the O(n^2)
  Lean `blockWeight` double sum. Because an undirected edge appears in both
  endpoints' adjacency, the pass yields the right numbers for free: an internal
  off-diagonal edge (both endpoints in the same community) is visited twice, so
  it adds `2*weight` to the super-node self-loop - exactly `blockWeight A A`, which
  counts `w_ij + w_ji`; a cross edge accumulates `blockWeight` symmetrically into
  both `(A,B)` and `(B,A)`. Members' own self-loops are folded in once. This is
  the correctness-critical folding from design section 4.1: internal weight must
  land on the self-loop and node sizes must sum, or the resolution term breaks
  silently.
- The community-to-dense-index relabel (`superOf`, the Go image of Lean
  `commLabel`) numbers communities by **ascending label**, not first-seen order.
  Canonical ordering keeps the aggregate independent of node/insertion order, per
  the determinism model (design 4.4).
- `TestAggregate_RoundTrip` proves the general level-composition property
  `Quality(agg, aggP) == Quality(g, expand(superOf, aggP))` for arbitrary aggregate
  partitions, not just the singleton. This is strictly stronger than the singleton
  invariance (`modularity_aggregate_eq` / `cpm_aggregate_eq`, which are the
  `aggP == identity` case) and is what lets Louvain/Leiden carry a partition found
  on the aggregate back down without re-scoring. It holds because grouping
  super-nodes is the same equivalence as grouping the underlying base communities,
  and `aggregate_degree` + `aggregate_twoM` make the block sums line up.
- Directed folding (separate in/out block sums) landed in M3 (mes-28rk):
  `aggregateDirected` runs one pass over each node's out-arcs, accumulating an
  internal arc onto the self-loop and a cross arc into both the out-block `(a, b)`
  and the in-block `(b, a)`, so out- and in-degree stay separable across levels.
  A symmetric directed graph folds to the same aggregate as the undirected case.

## Serial Louvain (mes-hcvp, step 4)

- The two move-deltas are unified behind an internal `objective` interface
  (`QualityFunction` plus `moveDelta`), exactly where mes-vy3a's note said to do
  it. `modularity` and `cpm` both satisfy it; the public `Modularity`/`CPM`
  constructors still hand back the `QualityFunction` view, and the optimizer
  narrows to `objective` internally, so the move loop is written once. `moveDelta`
  stays an unexported method, never on the public interface.
- Local moving considers three kinds of target: stay (delta 0), each distinct
  neighbour community, and **isolation** into a fresh singleton. Isolation is what
  makes a "stable" partition genuinely `IsLocalMoveStable` in the strong,
  all-targets Lean sense: moving a node into any community it has no edge to is
  never better than isolating it (edge term 0, null-model penalty only grows), so
  neighbour+isolate stability implies stability over every target up to sign. Drop
  isolation and `TestConverge_StableNoImprovement` fails on loosely-attached nodes.
- Isolation needs a **unique** fresh label per split so two isolated nodes never
  collide. A single counter, started at `numNodes` and incremented on each
  isolation (threaded through sweeps by a `*int`), does this. Labels then grow
  past `[0, n)`, which is fine internally: `Quality`, `moveDelta`, and `aggregate`
  all key on the grouping, not label magnitude, and `aggregate` canonicalises.
  The final result is run through `canonicalize` (ascending-label dense relabel)
  so the public `Partition` is well-formed.
- Moves are accepted only when `delta > moveImproveEps = 1e-12`. This is the
  float-noise guard, not just an optimisation: a true no-op whose incremental delta
  rounds to ~1e-14 must be rejected, or two communities can ping-pong a node
  forever and the "finite range of quality" termination argument
  (`no_infinite_acceptedMove_run`) stops applying. 1e-12 sits above the observed
  rounding floor at corpus scale and below both any real quality gap and the
  1e-9 tolerance the stability tests assert, so a loop-stable partition also
  passes those.
- The multilevel loop is deterministic with **no PRNG** (determinism
  scaffolding is the separate mes-z0pd): canonical node order, neighbour
  communities sorted so ties break to the smallest label, and the
  already-canonical `aggregate`. Randomised tie-breaking belongs to Leiden
  refinement, not here.
- The termination guard is `numCommunities(p) < workingNodes` after each level's
  local move - the exact negation of `RunningLevelStep` (CORRESPONDENCE.md section
  4). The loop stops the instant a level merges nothing (discrete partition), rather
  than aggregating unconditionally, so `no_infinite_runningLevel_run` is inherited:
  each running level strictly shrinks the working graph and the loop takes at
  most `n` levels. `louvainTrace` returns per-level `{base, working, merged}` so
  the five `Termination.lean` tests read the guard and the size descent off it.
- Karate at gamma=1 gives modularity 0.4198 with 4 communities - the known optimum
  and canonical Louvain shape - even with the simple deterministic tie-breaking.
- A minimal Newman-GML edge reader lives in `louvain_test.go` as a test helper,
  scoped to the karate shape check. The real golden-corpus loader is mes-5wqp; do
  not build on this throwaway parser.

## Determinism scaffolding (mes-z0pd, step 5)

- The owned PRNG (`prng.go`) is `splitmix64`, chosen for being tiny and fully
  specified by three constants so the stream is trivially portable. We do not use
  `math/rand`: its algorithm is not guaranteed stable across Go versions and its
  global source is shared with unrelated call sites, either of which would make
  a seeded run irreproducible. Owning the generator makes randomness a pure
  function of the seed (design 4.4).
- Our stream was checked against the canonical `splitmix64` reference values
  (Vigna) for seed 0 and those first outputs are pinned in
  `TestPRNG_GoldenStream`. This is a portability guard: an accidental change to
  a constant or the step would silently shift every seeded run's output, and the
  golden test catches it. It also documents that meso's PRNG *is* stock
  `splitmix64`, not a bespoke variant.
- Per-node randomness is `nodeSeed(globalSeed, u) = mix64(mix64(seed) ^
  mix64(u+goldenGamma))`, a pure function of `(seed, u)`. Avalanching the seed and
  the node index separately before combining keeps adjacent node indices (and
  adjacent seeds) from aliasing. Because it does not depend on visit order, the
  refinement (M2) and synchronous parallel round (M4) randomness is
  scheduling-independent - the property the parallel confluence work relies on.
- No `nodePRNG(seed, u)` convenience wrapper was shipped: with no consumer yet it
  trips golangci-lint's `unused` check, and the tdd/no-speculation rule says leave
  it out. Refinement composes `newPRNG(nodeSeed(seed, u))` at its first real call
  site. `nodeSeed` and `newPRNG` stay because the value-oracle-style tests exercise
  them directly.
- Order-independence of *inputs* lives at the Builder, since insertion order is
  where it enters. `Builder.Canonical()` is an opt-in that makes `Build` assign
  dense indices by ascending key instead of first-seen, via a `densePositions()`
  permutation threaded through the edge/self-loop/size/key writes. The default
  first-seen path is byte-for-byte unchanged (its documented contract and
  `builder_test` still hold); only the index *assignment* was insertion-order
  dependent, as adjacency was already sorted at flatten time. Under `Canonical()`,
  shuffled edge insertion yields an identical internal model and therefore an
  identical partition.
- Serial Louvain was already deterministic (no PRNG), so AC#1 is a locked-in
  regression guard rather than new behaviour: `TestLouvain_ByteIdenticalRepeated`
  asserts byte-identical partitions across repeated runs over the karate corpus
  plus 200 fuzzed graphs, for both modularity and CPM. It will catch any future
  non-determinism a seeded refinement layer might introduce.
- No Lean theorem-to-test row here. Determinism-by-sorting is the floating-point
  property the Lean model deliberately keeps out of scope (design section 7); this
  substrate underlies the M4 parallel confluence tests instead.

## Go value-oracle harness (mes-nqky, step F/G Go side)

- The harness lives in `_test.go` files in `package meso` (`oracle_test.go` for
  the loader and comparison policy, `oracle_harness_test.go` for the acceptance
  tests), not a separate `internal/` package. The quality functions take the
  unexported `*csr` and the move-deltas are methods on the unexported
  `modularity`/`cpm` types, so a distinct package could not reach them; a test
  file in `package meso` is genuinely test-only (never compiled into the shipped
  library) while still seeing the internals. This is the meaning of "internal
  test-only helper" for a library whose numeric core is unexported.
- CPM needed a convention bridge. The Go `cpm` evaluator computes the proof-side
  objective with the diagonal `i == j` terms included, but the oracle emits the
  canonical (leidenalg) convention. They differ by the partition-independent
  diagonal `D = sum_i (w_ii - gamma*s_i^2)`, so the harness compares against
  `canonicalCPM = cpmQ - D` (verified by hand on triangle: proof-side CPM -3 at
  gamma=1 becomes canonical 0, matching the golden). Because `D` is
  partition-independent it cancels in any move-delta, so `cpm.moveDelta` is
  compared to the golden delta with no correction. No canonical CPM was added to
  the production core: that is a corpus/output concern for a later ticket, and the
  correction is a test-side detail here.
- The comparison follows the decided ULP-plus-atol policy, not a single absolute
  epsilon. The reference is the correctly-rounded rational
  (`big.Rat.Float64()`), and a case passes iff ULP distance <= budget (16 for
  values, 4 for deltas) OR absolute error <= atol (1e-12). The atol floor is
  load-bearing for the exact-zero cases (square CPM, karate all-in-one
  modularity) where ULP distance near denormals is meaningless. All six fixtures,
  including the corpus graphs, land inside these budgets.
- The loader joins the input and golden files by case index and cross-checks that
  they agree on `quality` and `gamma` before trusting the join. Weights, sizes,
  and gamma parse through `big.Rat` (accepting both JSON numbers and `"p/q"`
  strings) so no float rounding enters the graph the oracle values were computed
  over. Nodes are registered 0..n-1 in order before edges so the Builder's
  first-seen dense index equals the input node index and the case partitions line
  up; `Canonical()` is deliberately not used (lexicographic key order would
  misalign "10" against "2").
- `subsetOptimal` is a `*bool`: null above the emitter's node bound (all corpus
  cases) and a genuine flag on the tiny fixtures. Consumers skip nil rather than
  reading it as false. The tiny fixtures carry deliberate false branches
  (path3 `[0,1,0]` disconnected, square all-in-one not subset-optimal, triangle
  `[0,0,1]` not gamma-separated) so the guarantee tests get a proved oracle value
  on the failing branch too.

## Leiden fast local move (mes-1lg6, step 6)

- The queue-driven fast local move lives in `leiden.go`, separate from Louvain's
  sweep in `louvain.go`, which is left unchanged: the design keeps Louvain on its
  own sweep and layers the queue in only for Leiden.
- `nodeQueue` is a FIFO with an in-queue bit set. `push` returns whether the node
  was newly added, so the move loop enqueues "exactly the un-queued neighbours"
  without a separate membership test; `pop` clears the mark so a node can be
  re-enqueued once processed but never while still pending. The bit set is sized
  `n` (node indices), not by community label - the queue only ever holds nodes,
  while isolation labels can exceed `n`.
- The literal design (section 4.1 phase 1) describes phase 1 as a single queue
  drain with neighbour re-enqueue. That single drain does NOT reach
  `IsLocalMoveStable` for modularity or CPM. A node's move gain has a null-model
  term over a whole community's aggregate degree/size (`K_c` / `S_c`); when a
  NON-adjacent node joins or leaves a community, every current member's gain
  shifts, but neighbour re-enqueue only ever revisits adjacent nodes. Confirmed
  empirically: a drained queue left residual improving moves of delta up to ~2.4
  (lesmis CPM) and ~0.94 (fuzzed CPM). Modularity is affected too but far less
  (its `k_i k_j / 2m` term is diluted by `2m`), so its residues are tiny or zero.
- Fix without abandoning the queue: `localMoveQueue` drains repeatedly until a
  whole drain applies no move. It is the queue-based drop-in for Louvain's
  `localMoveToStable` and reaches the same stopping condition - a clean drain pops
  every node once against a fixed partition and moves none, which is exactly a
  no-move exhaustive sweep, i.e. `IsLocalMoveStable`. This is what the ticket's
  AC1 ("both stop at IsLocalMoveStable") requires; the "neighbour re-enqueue" is
  the inner drain mechanism, the outer loop is the fixed-point guarantee.
- Termination is monotonicity plus a strict per-move floor: each accepted move
  raises quality by more than `moveImproveEps`, quality is bounded over the finite
  set of groupings, so finitely many moves are accepted; each drain pops a bounded
  number of nodes. No PRNG enters the phase - visiting order (ascending initial
  enqueue, FIFO re-enqueue) and `bestMove` tie-breaking are canonical - so the
  result is byte-identical across runs without any seed. Randomness stays a
  refinement concern (M2, `nodeSeed`), not local moving.

## Leiden refinement (mes-w7ko, step 6)

- The refinement operator lives in `refine.go` alongside the connectivity and
  gamma-density predicates it maintains: `mergeCommunities`, `communityConnected`,
  `connectedCommunities`, `gammaDenseCut`, `gammaDenseCommunities`, and the
  operator `refine`. `blockWeight` (the Lean `blockWeight`, generalising
  `communityInternalWeight` to the cut between two communities) went in `cpm.go`
  next to the other from-scratch density helpers.
- Scheduling-independence (design 4.4/4.5, AC7) is the design driver, and it is
  achieved by computing every node's merge intent up front against the fixed
  singleton partition (`refineIntents`), then resolving the intents with an
  order-independent union-find (`unionIntents`, smaller-root tie-break). This
  diverges from the sequential reference (which merges a singleton into an
  evolving sub-community, so its result depends on visiting order): here each node
  contributes exactly one intent from its own singleton state, so "only singletons
  are merged" holds and the outcome is a pure function of `(g, obj, outer, seed)`.
  The union step was factored out precisely so the test can apply the same intents
  in shuffled orders and confirm byte-identical output.
- Per-node randomness is `newPRNG(nodeSeed(seed, v))`, never a shared stream, so
  a node's gain-weighted draw is the same however nodes are ordered. Candidates
  are sorted ascending by node index before the weighted pick, otherwise the
  choice would depend on adjacency-discovery order and break the invariance.
- The well-connectedness gate is two conditions, matching the Lean
  `GammaMergeStep` (shared edge + gamma-dense cut): a candidate neighbour needs
  a positive-weight edge (connectivity, so no merge can ever disconnect a
  community), must sit in the same phase-1 outer community, and the
  singleton-level cut must be gamma-dense (`gamma*s_v*s_u <= w_vu`). Only
  strictly quality-improving merges
  (`moveDelta > moveImproveEps`) are candidates. For CPM the gain-positivity test
  and the gamma-dense-cut test coincide (`cpmMergeGain > 0` iff
  `w_uv > gamma*s_u*s_v`); for modularity they differ and both apply.
- The gate reads the objective's resolution, so `objective` grew a
  `resolution() float64` method (trivially returning `gamma` on both `modularity`
  and `cpm`). This keeps `refine` objective-generic without threading gamma
  through every call.
- Gamma-density is deliberately NOT guaranteed for the operator's output from
  singletons: a lone node is not gamma-dense, and the Lean
  `gammaWellConnectedCommunities_of_gammaMergeRun` bases its guarantee on a
  gamma-dense partition, not singletons (noted in its divergence register). So the
  landed rows are the step-level theorems (`gammaDense_union`,
  `GammaMergeStep.gammaDenseCommunities`) on fixtures; the full-operator run rows
  (`connectedCommunities_of_refineRun`,
  `gammaWellConnectedCommunities_of_gammaMergeRun`) stay `planned` for the Leiden
  assembly ticket.
- `communityConnected` treats the empty and singleton communities as connected
  (the Go image of `Preconnected.of_subsingleton`) and walks only positive-weight
  off-diagonal edges, so self-loops carrying internal weight are correctly not
  connectivity edges, matching `WeightedGraph.simpleGraph` (`fromRel`, which drops
  self-loops).

## Leiden assembly (mes-jbc7, step 6)

- The three phases are wired into a single `leiden(g, obj, seed)` in `leiden.go`,
  mirroring `louvainTrace`'s working-graph/`baseOf` structure but inserting
  refinement between local moving and aggregation. Each level: (1) `localMoveQueue`
  from the seeded partition to get the non-refined phase-1 `p`; (2) `refine(h, obj,
  p, seed)`; (3) `aggregate(h, refined)` and seed the next level's local move from
  the NON-refined `p` via `leidenLevelSeed`.
- The correctness-critical subtlety (design 4.1 phase 3): the aggregate is built
  from the refined sub-communities, but its initial partition is seeded from the
  non-refined phase-1 assignment. `leidenLevelSeed(superOf, nonRefined, k)` gives
  each super-node the phase-1 label of any member - well-defined because refinement
  only merges nodes already sharing a phase-1 community, so every base node mapping
  to one super-node carries the same non-refined label. Seeding from the refined
  partition instead would split those super-nodes apart and lose the phase-1
  grouping; `TestLeidenLevelSeed_FromNonRefined` pins the divergence.
- The returned communities are the non-refined phase-1 partition lifted to the
  base graph at the deepest level, not the refined one. Refinement exists to build
  a better aggregate and to guarantee connected sub-communities (`refine` unions
  only across shared edges); it does not define the final communities. The
  connectivity row `connectedCommunities_of_refineRun` is therefore about the
  refined partition at each level (`TestLeiden_CommunitiesConnected`), which is
  what CORRESPONDENCE names; whole-returned-partition connectivity on the corpus
  is a formal-guarantee concern for mes-wmzq (step 7).
- Termination has two exits: phase-1 merged nothing (discrete `p`, the negation
  of the `RunningLevelStep` guard, as in Louvain), or refinement did not shrink
  the graph (`aggG.numNodes() >= h.numNodes()`). The second exit is the
  load-bearing one for arbitrary inputs: refinement can, on a poorly-connected
  phase-1 community, rebuild it as all singletons, leaving the aggregate the same
  size as the working graph - without the guard that would loop forever (the
  seeded local move on an identical aggregate reproduces `p`, which is already
  stable). With the guard every continued level strictly shrinks the working
  graph, so the loop takes at most `n` levels. On real corpus graphs some
  community always refines, so the recursion runs to the phase-1-discrete exit and
  the guard never fires.
- AC4 (Leiden quality >= Louvain) is scoped to the corpus deliberately. On karate
  both hit the known optimum (0.4198). On arbitrary random graphs Leiden is
  monotone across its own levels (verified: no regression within a run) but can
  end a hair below Louvain, because Louvain re-explores from singletons on every
  aggregate level while Leiden resumes from the seeded phase-1 grouping. That
  is a legitimate heuristic trade, not a defect; the "Leiden >= Louvain" is
  empirical, not a theorem (the paper guarantees connectivity and monotone
  improvement, not beating Louvain). `TestLeiden_QualityGeLouvain` runs on karate
  only.
- Public `Leiden(g, opts...)`/`Louvain(g, opts...)` return a `*Result` (`api.go`)
  with `Communities() map[string]int` (caller keys via the Builder reverse map)
  and `Quality()`, matching the design section 3 sketch's `part.Communities()`/
  `part.Quality()`. The existing exported `Partition []int` stays the internal
  dense-label Lean mirror; `Result` is the caller-facing wrapper, so the ticket's
  shorthand `-> (Partition, error)` is realized as `-> (*Result, error)`: a bare
  `[]int` cannot also carry the quality and key mapping the design enumerates.
  `Levels()` (the hierarchy) is deferred - no AC needs it yet.
- Options are `WithQuality`/`WithSeed`/`WithResolution` (functional options over
  a `runConfig` defaulting to modularity gamma 1, seed 0). `WithResolution`
  overrides the objective's gamma by type-switching the internal `modularity`/
  `cpm` and rebuilding it; this is safe because `QualityFunction`'s method takes
  the unexported `*csr`, so external types cannot implement it and the objective
  assertion always holds. A directed graph must pair with `DirectedModularity`;
  `checkObjectiveGraph` rejects any other objective (the undirected null models
  mis-score directed arcs), while an undirected graph accepts any objective.

## Empirical invariants and formal-guarantee checks (mes-wmzq, step 7)

- This step builds no algorithm: it is the Go test layer
  (`guarantees_test.go`) that holds the run to the verified model, the
  empirical mirror of the Lean paper-theorems tier (`Guarantees.lean`,
  `Separation.lean`, `GammaConnectivity.lean`, `SubsetOptimality.lean`). Two
  predicate deciders that belong to the library's guarantee vocabulary went
  into production `refine.go` next to their existing peers
  `connectedCommunities`/`gammaDenseCommunities`: `gammaSeparatedCommunities`
  (`e(C,D) <= gamma S_C S_D` over distinct communities) and
  `gammaWellConnectedCommunities` (connected AND gamma-dense). Everything
  exponential or purely oracle-shaped (subset enumeration, the level-walker,
  the brute-force subset-stability check) stayed in the test file.
- The raw multilevel output is level-stable but NOT single-node stable on the
  base graph. Empirically, Louvain/Leiden CPM output at gamma >= ~0.3 leaves
  nodes with a strictly improving single-node move: a node stable on its
  level's working graph can become improvable once the community structure
  changes at a higher level. This drives each guarantee's hypothesis. The
  gamma-separation theorem (`gammaSeparated_of_converged`) rests only on
  level-stability (no community merge improves), which the multilevel loop DOES
  guarantee by construction (it stops when the top aggregate is
  singleton-stable), so `TestCPM_GammaSeparated` asserts `aggregateLevelStable`
  as the precondition and reads gamma-separation off it. The single-node "no
  better community" check (`cpm_noStrictlyBetterCommunity`) instead uses a
  base-level `localMoveQueue` result, whose local-move stability holds directly.
- The returned (non-refined) partition's communities ARE connected on the whole
  corpus and on 800+ fuzzed graphs for both objectives (probed before
  asserting), so `TestLeiden_OutputCommunitiesConnected` asserts whole-output
  connectivity, the guarantee mes-jbc7 deferred. It is not a standalone Lean
  theorem (the model proves connectivity for the refined partition), so this is
  an empirical invariant, held two ways: on the live output and, per committed
  case, against the proved `connected` oracle flag.
- The strongest guarantees (gamma-density of every community,
  subset-optimality) do NOT hold for arbitrary algorithm output: refinement
  from singletons is not gamma-dense, and the returned partition is the
  non-refined one. So the tests for those
  (`TestLeiden_CommunitiesGammaConnected`, `TestCPM_SubsetOptimal`,
  `TestLeiden_Guarantees`) assert the theorem's CONCLUSION from its actual
  HYPOTHESIS on fixtures: a gated `GammaMergeStep` run from a
  gamma-well-connected base (AC7), and the committed golden partitions whose
  proved `subsetOptimal` flag the enumerating oracle must match (AC8/AC9). The
  2026-07-17 ticket note is the guide: checks 2 and 6-9 assert Go deciders
  against committed proved booleans, not a retired differential envelope.
- Every predicate decider is cross-checked against the committed golden flags
  across all six fixtures (the `TestOracle_*Vector` tests) and matches exactly.
  The tiny fixtures carry deliberate false branches (path3 `[0,1,0]`
  disconnected, triangle `[0,0,1]` not gamma-separated, square all-in-one at
  gamma=1 not subset-optimal), so every vector test asserts two-sided teeth
  (`sawTrue`/`sawFalse`); mutating any decider (flip a comparison, drop the
  density conjunct, flip the split-gain sign) fails the suite.
- `subsetOptimal` is null above the emitter's node bound, so the enumerating
  `subsetOptimalOracle` (2^|C| per community) only ever runs on the tiny
  fixtures; corpus cases are skipped, never enumerated. `TestLeiden_Guarantees`
  conjoins the four deciders and compares against the AND of the four proved
  flags, with a committed case that satisfies all three guarantees at once
  (triangle CPM gamma=1/2 and square CPM gamma=1/3, both all-in-one) for
  positive teeth.
- `leidenTrace` in the test file re-runs the `leiden()` loop to expose
  per-level base partitions for the monotonicity and termination checks;
  `TestLeiden_TraceMatchesRun` pins its last level to `leiden()`'s return so the
  walked levels are the real run's, not a drifted copy. The names
  `TestLeiden_QualityMonotoneAcrossLevels` and `TestLeiden_LevelsTerminate` were
  already taken by the Louvain-trace (shared multilevel skeleton) tests, so the
  full-Leiden-run versions are `TestLeidenRun_*`.

## Golden corpus tests (mes-5wqp, step 5)

- `TestGoldenCorpus` (`golden_test.go`) pins meso's deterministic Louvain output
  on karate, dolphins, and Les Miserables as committed JSON goldens under
  `testdata/golden/`, using the standard Go `-update` idiom
  (`go test -run TestGoldenCorpus -update`) to regenerate after a reviewed
  change. It exercises the public API only (`NewBuilder`, `Louvain`,
  `Result.Communities`/`Quality`), so it survives internal refactors.
- The golden partition is canonicalised before committing: nodes are keyed by
  their GML id and listed in ascending numeric order, and community labels are
  renumbered in order of each community's smallest member. This makes the file a
  pure function of the partition, not of whatever internal label meso happened to
  assign, so a genuine drift shows as a diff while a mere relabelling does not.
  Modularity is stored as `strconv.FormatFloat(q, 'g', -1, 64)` (shortest
  round-trippable decimal) so equal float64s serialise to identical bytes.
- Achieved values match the literature: karate Q=0.4188 (4 communities; published
  optimum about 0.4198), dolphins Q=0.5185 (5), Les Miserables Q=0.5654 (6,
  weighted). AC#3's "reference envelope" is a loose published-literature band
  (bracketing the optimum, not meso's exact number) plus a community-count sanity
  check; it only guards against a grossly broken optimiser. Regenerating with
  -update a second time is byte-identical, and the test re-runs Louvain in-process
  to assert determinism (AC#4).
- The exact-rational value-oracle check on meso's OWN converged partition is NOT
  done here: the committed oracle vectors (`verification/oracle/golden/*.json`)
  only score the anchor partitions (all-in-one, singletons, karate's ground-truth
  split), never meso's Louvain output. Adding meso's converged partition to the
  oracle input set is the deferred mes-xb3w; this ticket's cross-check on those
  anchors is already covered by `TestOracleQualityValues`.
- GML loader gotcha: karate.gml is 1-based (node ids start at 1) while lesmis.gml
  is 0-based; the numeric-key sort handles both. lesmis edges carry weights in the
  `value` attribute, so the loader reads `value`/`weight` and the weighted
  modularity comes out at the published 0.5654.

## Directed modularity evaluator (mes-pgah, step 8)

- Leicht-Newman directed modularity lives in `directed_quality.go` as
  `DirectedModularity(gamma) QualityFunction`, the directed sibling of
  `Modularity`. Its null model uses each pair's separate degrees,
  `k_i^out * k_j^in / m`, instead of the symmetric `k_i * k_j / 2m`. The arc
  weight `w_ij` is read from the out-adjacency (`csr.weight` scans out-neighbours,
  giving the arc i to j) and `k_j^in` from the in-adjacency; both were already
  separable from the CSR/Builder tickets, so no graph-model change was needed.
- The normaliser `m` (total arc weight) is exactly what `csr.twoM()` returns for
  a directed graph: `twoM` sums `degree` = `outDegree`, so for directed it is
  `sum_i k_i^out` = total arc weight, and for a symmetric graph it is numerically
  the same `2m` the undirected evaluator uses. That is why the directed formula
  divides by `twoM()` and collapses to undirected modularity term-for-term.
- The symmetric-reduction test asserts EXACT float equality (`got != want`), not
  a tolerance: on a symmetric graph the two evaluators execute the identical
  row-major sum with identical operands, so the bits must match. `symmetricDirected`
  builds the directed csr by aliasing the undirected out-adjacency into both `out`
  and `in`.
- Deliberately did NOT add a `resolution()` method (modularity/cpm have one for
  the internal `objective` interface). It would be flagged unused by golangci-lint
  because directed is not yet a full `objective`: it has no `moveDelta`. Both land
  together in mes-gz8f (directed move-delta), which is when directed joins the move
  loop. Keeping the evaluator to just `Quality` now keeps the lint gate clean
  without an `_ =` silence.
- No Lean model by design (directed is a descope; weight-symmetry is load-bearing
  in the Lean development). Correctness rests on hand-computed fixtures plus the
  symmetric reduction here, and later on the leidenalg/igraph frozen-vector oracle
  (the `verification/reference/crosscheck` Docker harness), which is not wired into
  `make validate`. Louvain and Leiden still reject directed graphs until mes-28rk.

## Directed move-delta (mes-gz8f, step 8)

- `directedModularity.moveDelta` lives in `directed_move.go`, the directed
  sibling of the undirected/CPM deltas in `move.go`. Because the directed kernel
  `A_ij = w_ij - gamma*k_i^out*k_j^in/T` is asymmetric, moving node `u` perturbs
  BOTH row `u` (its out-arcs) and column `u` (its in-arcs), where the undirected
  delta only touches one symmetric row. The `j == u` diagonal is constant (`u`
  always shares a community with itself), so it drops out and `u`'s self-loop
  never enters the edge term, exactly as in the undirected case.
- Formula: `(1/T)[(Wout_ut - Wout_us) + (Win_ut - Win_us) - gamma*koutU*(Kin_t -
  Kin_{s\u})/T - gamma*kinU*(Kout_t - Kout_{s\u})/T]`, with `Wout` the arc weight
  from `u` into a community, `Win` the weight from a community into `u`, and
  `Kout`/`Kin` the summed out-/in-degrees. Iterating `u`'s in-arcs needed
  `csr.inNeighbors`/`inNeighborWeights` (added to `graph.go`), which fall back to
  the out-adjacency on an undirected graph, mirroring `inDegree`.
- Contrast with the evaluator's symmetric-reduction test above: that one asserts
  EXACT float equality because both evaluators run the identical row-major sum.
  The move-delta symmetric check must use a tolerance (`moveTol`), NOT `!=`: the
  directed and undirected delta formulas are algebraically equal but sum the null
  term in a different order (`koutU*.. + kinU*..` vs `2*ku*..`), so they land 1
  ULP apart. Do not tighten it back to bit-identical.
- The property test builds its own random ASYMMETRIC generator (`randomDirectedCSR`,
  per-direction independent weights, `in` = transpose of `out`) and checks the
  delta against the `DirectedModularity` oracle only. The undirected oracle is
  deliberately not reused (the directed note in
  `docs/research/move-delta-verification.md`). Hand value for the singleton-join
  case on the 3-node asymmetric fixture is 0.3125 at gamma=1.

## Directed pipeline and API (mes-28rk, step 8)

- The whole M3 pipeline was threaded directed by reusing the existing
  direction-aware primitives rather than forking a second optimiser. Four small,
  local changes: `directedModularity` gained `resolution()` (making it a full
  `objective` alongside `Quality`/`moveDelta`); `aggregate` branches on
  `g.directed`; `bestMove` and `refineIntents` gather candidates from both arc
  directions; the public `Leiden`/`Louvain` accept a directed graph with
  `DirectedModularity`. Louvain, local-move, refinement, and aggregation are all
  objective- and csr-generic, so nothing else needed touching.
- Directed aggregation (`aggregateDirected`) folds in a single pass over each
  node's out-arcs (a directed arc is stored once, from its source): an internal
  arc `a == b` lands once on the super-node self-loop, a cross arc `a -> b`
  accumulates into both the aggregate out-block `(a, b)` and the in-block
  `(b, a)`. This preserves in/out degree separability exactly: super-node A's
  aggregate out-degree is `selfLoop(A) + sum_b out[A][b] = sum_{i in A}
  outDegree(i)`, and likewise for in-degree, so `twoM` (total arc weight `m`) is
  preserved and the directed-modularity round-trip
  `Quality(agg, aggP) == Quality(g, expand(superOf, aggP))` holds. The undirected
  fold is unchanged; `flattenBlock` is the shared CSR-emit helper.
- The symmetric reduction is the load-bearing correctness anchor (AC2), and it
  falls out for free from three facts: on a symmetric directed graph in == out at
  every node, so (1) `aggregateDirected` sees each undirected edge as both `i->j`
  and `j->i`, landing `2*weight` on internal self-loops and a symmetric aggregate
  identical to `aggregateUndirected` (`TestAggregate_SymmetricDirectedIsSymmetric`);
  (2) `bestMove`'s in pass adds only communities the out pass already saw, so the
  candidate set and decision are unchanged; (3) `refineIntents`' in pass is fully
  deduped against the out pass and reads the same arc weight, so intents are
  byte-identical. End to end, directed Leiden on a symmetric graph recovers the
  same grouping and identical quality as undirected Leiden
  (`TestLeiden_SymmetricDirectedReducesToUndirected`).
- One trap: the directed and undirected move-*deltas* are algebraically equal but
  sum the null term in a different order (`koutU*.. + kinU*..` vs `2*ku*..`), so
  they land ~1 ULP apart, exactly as the mes-gz8f note records. The
  symmetric-reduction test compares the local-move *decision* (target) exactly but
  the delta only within `moveTol`; comparing deltas with `!=` is wrong. The
  end-to-end grouping still comes out identical because real quality gaps at
  corpus scale dwarf a ULP and no decision sat on a sub-ULP tie on the fixtures.
- `bestMove`'s exhaustiveness argument survives the directed generalisation:
  isolation still dominates any community `u` shares no arc with (its edge term
  is zero and its null penalty is smallest), so searching both-direction neighbour
  communities plus isolation attains the global optimum delta. The oracle test
  (`TestBestMove_DirectedMatchesBruteForce`) therefore compares the *achieved
  optimal delta* against an exhaustive scan of every label, not the target:
  several communities can tie for the optimum (a non-adjacent smaller community
  can tie isolation by lowering the null term), and bestMove may pick any of them.
  An earlier version comparing targets was wrong for this reason.
- `localMoveDrain` re-enqueues only a moved node's out-neighbours, not its
  in-neighbours whose gain also shifted. This is left as-is on purpose: it is an
  efficiency detail, not a correctness one, because the outer `localMoveQueue`
  loop drains repeatedly until a whole clean drain (every node popped once against
  a fixed partition) moves nothing, which is `IsLocalMoveStable` regardless of
  re-enqueue completeness. Adding directed in-neighbour re-enqueue would change
  the deterministic path (and thus the exact partition) for no correctness gain,
  so per the no-speculation rule it was not added. On symmetric graphs in == out
  so re-enqueue is already complete.
- API contract: a directed graph must be run with `DirectedModularity`.
  `checkObjectiveGraph` rejects directed + undirected-modularity/CPM (their
  symmetric null models mis-score arcs; directed CPM is out of scope by design),
  while an undirected graph accepts any objective, including
  `DirectedModularity` (which reduces to modularity there). `resolve`'s
  `WithResolution` switch grew a
  `directedModularity` case so gamma overrides rebuild it too. The stale
  `TestLeiden_DirectedRejected` (which asserted directed was unsupported) became
  `TestLeiden_DirectedRequiresDirectedModularity`.
- No Lean model backs any of this (directed is the deliberate E3 descope). AC1's
  directed oracle is hand-verifiable fixture structure, not the `leidenalg`/igraph
  frozen vectors: the reference harness (`verification/reference/crosscheck`) is
  Docker-only,
  out of band, and never emitted committed directed community vectors, so the
  fixtures are two directed 3-cycles weakly bridged, whose two-community ground
  truth is obvious by construction and which exercise both arc directions (a cycle
  node reaches its mates through one out- and one in-arc). If the frozen-vector
  oracle is wired up later, add it as a second, out-of-band check.

## Synchronous-round parallel local moving (mes-edpr, step 9)

- The confluence guarantee is structural, not something the code has to enforce
  at runtime. `applyRound(p, t, moved)` is a masked map (moved -> `t`, else `p`),
  so it is a pure function of the snapshot and the moved-node set: no apply order
  is even representable, which is `applyRound_perm` (schedule- and core-count
  independence) by construction. The Lean `applyRound` folds `move` over a
  work-list; the Go mask is that fold's proven closed form (`applyRound_eq`), so
  the code carries the stronger, order-free form directly.
- Parallelism is confined to `parallelBestMoves`: each node's best target and
  gain is computed against the frozen snapshot, split into contiguous node-index
  ranges across goroutines. Workers read only shared immutable state (`g`, `obj`,
  `p`) and write only their own output slots, so there is no lock and no race,
  and the output is byte-identical for any worker count. Everything after
  best-move computation (forming the moved set, the accept/resolve decision,
  applying) runs serially in a fixed order, so no decision can depend on the
  schedule. This is why `WithParallelism(w)` is byte-identical across `w` and
  clean under `-race`.
- Per-node isolation labels, not a shared counter. The serial sweep advances one
  `fresh` counter as nodes isolate, which cannot be evaluated independently per
  node. The parallel round hands node `u` the label `isolationBase(p, n) + u`
  (distinct, `> max(p)`, `>= n`), so two nodes splitting off in the same round
  never collide and the choice is a pure function of the snapshot. `canonicalize`
  compacts the sparse labels afterwards.
- Confluence gives determinism but NOT monotonicity, and this is the real design
  work the ticket left open (plan 4.5). Because every node prices its move against
  the snapshot, two independent snapshot moves interact through the modularity
  null-model degree sums, so applying the whole moved set at once can fail to
  improve; on a bipartite trap (an even cycle) a naive batch swaps partners each
  round and never converges. Graph-coloring by adjacency does not fully fix this:
  two non-adjacent nodes moving into the same community still lose a null-model
  cross term, so the batch is not provably monotone. The chosen mitigation is the
  other candidate in 4.5 - "strictly-positive-gain acceptance with deterministic
  conflict resolution": compute the round's exact realized delta as a telescoping
  sum of `moveDelta` in ascending node order (equal to
  `Quality(applyRound(...)) - Quality(p)` but O(moved) not O(n^2), and evaluated
  in a fixed order so it is byte-identical across cores); accept the whole round
  if it clears `moveImproveEps`, otherwise apply only the single highest-gain move
  (smallest index on a tie), whose realized delta is exactly its snapshot gain and
  so strictly improves. Either branch raises the objective by more than the eps,
  so termination is the same finite-range argument as the serial loop.
- The fallback keeps the round within the Lean model: applying only the best node
  is still `applyRound(p, t, {best})`, a smaller moved set, not a different
  operation. So the mitigation never leaves the proved-confluent shape; it only
  chooses which subset of the snapshot moves to commit.
- Level loop reuse over duplication. `louvainTrace` and `leiden` were refactored
  to take a `localMover` (`louvainTraceWith`, `leidenWith`); the serial movers
  wrap the existing sweep/queue phases and `parallelMover(workers)` wraps the
  synchronous-round phase, so the aggregation/refinement/guard/termination driver
  is written once and the parallel path is a one-line substitution. `louvain`,
  `leiden`, `louvainLevels` and every existing test keep their signatures.
- Parallel and serial reach DIFFERENT fixed points (the design doc says as much:
  synchronous updates converge differently from a sequential sweep), so
  `WithParallelism` is opt-in and the default stays serial - the golden corpus
  and every determinism test are untouched. "Byte-identical serial versus
  parallel" in the acceptance criteria means the parallel algorithm at 1 worker
  versus N
  workers, not parallel versus the default serial algorithm; both are valid
  partitions of possibly different modularity.
- `isLocalMoveStable` (the M2 invariant checker) uses `louvainTol = 1e-9` while
  the move loops accept at `moveImproveEps = 1e-12`. A partition the parallel loop
  calls stable (no gain `> 1e-12`) is therefore also stable under the checker
  (no gain `> 1e-9`), so `TestParallel_Convergence` can assert the converged
  partition passes `isLocalMoveStable` without a tolerance mismatch.

## Native fuzz targets (mes-y8ru, step 12)

- Go's `testing.F` seed corpus IS the CI-cheap fuzz run: `go test` (and so
  `make validate`) executes every `f.Add` seed once, without a mutation budget,
  so the fuzz targets need no separate wiring into the gate. A real mutation
  search is on-demand via `make fuzz` (added, one target per `go test -fuzz` run,
  `FUZZTIME` overridable). Failing inputs land in `testdata/fuzz/<Target>/`; a
  clean run writes nothing to the repo (the working corpus lives in the build
  cache), so the tree stays clean.
- The fuzz decoder maps raw bytes to a `Builder`: the first byte caps the node
  count in `[1, 24]` (keeps the O(n^2) connectivity/quality checks cheap under a
  mutation search) and the rest are read as 3-byte (from, to, control) records,
  so a byte mutation is a graph mutation and coverage-guided search explores real
  structure. Edge weights are made strictly positive (`1 + ctrl/8`): a zero-weight
  pair is a non-edge, and `communityConnected` follows only positive-weight edges,
  so allowing zero-weight edges would make connectivity ambiguous.
- Assert only what the algorithm actually guarantees. Connected communities is a
  Leiden guarantee, NOT a Louvain one (Louvain's disconnected-community defect is
  the paper's whole motivation), so the fuzz checker asserts connectivity on Leiden
  output only, never on Louvain's. The shared quality invariant used for both is
  "output quality >= the all-singletons baseline the run starts from", always true
  since local moving only accepts non-negative deltas and aggregation preserves
  quality.
- The fuzzer immediately found a real, deterministic bug (filed as a separate
  M2 ticket): Leiden under **modularity with non-unit node weights** returns a
  disconnected AND lower-quality community. Modularity's objective is blind to node
  sizes (only `cpm.moveDelta` reads them), but the refinement well-connectedness
  gate `gamma*s_v*s_u <= w_vu` (refine.go, the Lean GammaMergeStep density gate)
  reads them for every objective; large node sizes make the gate reject every
  merge, refinement becomes a no-op, and Leiden degenerates to Louvain. Because
  fixing the gate touches verified refinement code and the Lean CORRESPONDENCE,
  it is out of scope for the fuzz ticket; instead the fuzz targets exercise each
  objective over its meaningful input domain (modularity over unit node sizes,
  CPM over the fuzzed node weights where the size penalty is part of the objective)
  so the connectivity assertion is honest and green, and the bug is tracked rather
  than hidden. Once the gate is fixed, the modularity=unit-sizes split in
  `fuzz_test.go` can be removed.

## Objective-appropriate refinement gate (wor-w33p)

- The fix for the fuzz-found bug above: at the singleton base `refineIntents`
  runs over, the refinement well-connectedness density criterion IS the
  objective's own move-delta, so gating on `obj.moveDelta(base, v, u) >
  moveImproveEps` is automatically objective-appropriate. For CPM,
  `2*(w_vu - gamma*s_v*s_u) > 0` is the strict form of the `GammaMergeStep`
  gate `gamma*s_v*s_u <= w_vu`; for modularity, `(2/2m)*(w_vu -
  gamma*k_v*k_u/2m) > 0` is the degree-based analog. The old explicit
  `gamma*s_v*s_u > w` line was therefore redundant-and-correct for CPM (the
  gain check already subsumed it, since `gain > eps` implies the cut is
  gamma-dense) but wrong for modularity, which reads node sizes its objective
  ignores. Removing the line changes CPM behaviour by not one bit and fixes
  modularity, the whole diff being a deletion plus a comment.
- Connectivity and density are separate gate halves with separate owners. The
  Lean model already splits them (`Connectivity.lean` proves
  `connectedCommunities_of_refineRun` from a shared positive-weight edge ALONE,
  no node sizes; `GammaConnectivity.lean` owns the CPM `gamma`-density tier),
  so the objective-generic connectivity guarantee never depended on the density
  gate. Keeping `w > 0` as an unconditional candidate filter preserves it for
  every objective; only the density half needed to become objective-aware. When
  a gate enforces two guarantees at once, check whether the formal model already
  separates them before assuming both must be objective-generic.
- Removing the gate let `resolution()` fall out of the `objective` interface: it
  existed only to feed that gate, and nothing else called it. A gate rewrite that
  drops its last reader is a signal to prune the interface method too, not leave
  a dead accessor behind.
- With the gate fixed, the `modularity`-over-unit-sizes split in `fuzz_test.go`
  is gone: `FuzzLeidenLouvain` now runs both objectives over the node-weighted
  graph and asserts Leiden connectivity for modularity there. A 30s / ~900k-exec
  mutation run stayed green, including the deeper aggregation levels where
  super-node sizes grow from unit input (the path the ticket flagged as worth
  checking).

## Partition-comparison metrics: NMI, AMI, ARI (mes-e5yr)

- Normalization choice for NMI/AMI is arithmetic mean of the two entropies,
  matching scikit-learn's default. It is pinned by a clean analytic anchor: when
  `a` is a strict refinement of `b`, `MI = H(b)`, so
  `NMI = H(b)/((H(a)+H(b))/2)`. For `a=[0,1,2,3]`, `b=[0,0,1,1]` that is
  `ln2/(1.5 ln2) = 2/3` exactly, independent of the log base. A single hand
  value therefore both checks the code and fixes the normalization convention.
- "Independent partitions score near 0" (an acceptance criterion) is only true
  for genuinely random draws in expectation, NOT for structured, perfectly
  balanced contingency tables. A deterministic 10x10 Latin grid
  (`a=i//10`, `b=i%10`) has the maximally uniform table yet scores
  `AMI=-0.28`, `ARI=-0.10` - well below 0 - and the 2x2 checkerboard scores
  exactly `-0.5`. The chance model penalizes suspiciously even overlaps. The
  near-0 test must draw random labelings (meso's own PRNG, several seeds,
  n=200/k=5 where the empirical spread is ~0.03) and assert a loose bound, not
  pin a constant on a hand-built "independent-looking" table.
- AMI's expected mutual information is the fixed-margins hypergeometric sum
  (Vinh et al. 2010, eq. 24a). Evaluate its combinatorial weight through a
  log-factorial table plus `math.Exp`, never raw factorials, or `n` in the
  hundreds overflows float64. `entropy` never sees a zero bucket because the
  sizes come from compacted labels (every dense label occurs at least once), so
  there is no `log(0)` guard to get wrong.
- AMI has a genuine degeneracy at n=2 singletons: `MI == E[MI]`, so numerator
  and denominator are both 0. sklearn's fix - clamp the denominator to
  +/- machine epsilon - is what we use, and it means the AMI=1 identical-case
  test must use a partition with enough structure (>=3 nodes, >=2 real
  communities) rather than the trivial `[0,1]` where the adjustment cancels the
  whole signal.
- No sklearn/numpy in the toolchain, so trustworthy reference values came from a
  throwaway pure-Python (stdlib `math`) reimplementation of all three metrics -
  an independent second implementation, which is a stronger oracle than reusing
  the code under test. It immediately caught an arithmetic slip in a by-hand EMI
  computation (I had -0.309; the true value is -0.5).

## Converged-partition oracle case (mes-xb3w)

- The value-oracle input set now scores each corpus graph on meso's own
  converged Leiden partition, not only the degenerate anchors (all-in-one,
  singletons, karate's ground-truth split). At the default seed 0, gamma 1:
  karate reaches 4 communities at 49/117 (about 0.4188), dolphins 5 at
  26461/50562 (about 0.5233), Les Miserables 6 at 380779/672400 (about 0.5663).
  karate's 0.4188 corroborates the published optimum (about 0.4198) out of band;
  the optimum is never hand-entered, only meso's deterministic output is pinned.
- The Lean toolchain (mathlib) is not installed in this environment, so
  `make oracle-lean` cannot run here. The golden vectors were produced by
  a big.Rat mirror of Lean's `modularityQ` (verification/lean/Meso/Compute.lean:
  Q = (1/2m) sum_ij (w_ij - gamma k_i k_j / 2m) [c_i=c_j], self-loops counted
  once, edgeless 1/0 = 0). The mirror was validated by recomputing every
  pre-existing committed modularity value across all six fixtures and asserting
  byte-equal num/den before emitting the new cases, so a future confirmatory
  `make oracle-lean` will reproduce them.
- The predicate flags in a golden case must equal meso's Go deciders, because
  guarantees_test.go (TestOracle_ConnectivityVector, _GammaDenseVector,
  _GammaSeparatedVector) cross-checks the committed flag against the decider on
  every case of every fixture. Generating the new flags from
  connectedCommunities / gammaDenseCommunities / gammaSeparatedCommunities is
  therefore consistent by construction, and those deciders are the Lean-proved
  images, so the flags equal what the oracle emits. For a simple unit-weight
  graph at gamma 1 the flags are forced: connected true (Leiden guarantees it),
  gammaDense false (k^2 <= k^2-k never holds), gammaSeparated true (cross edges
  <= |C||D| always), subsetOptimal null (n > 16, emitter's maxSubsetOptimalN).
- The golden `approx` field is Lean's Float.toString, matched exactly by
  Go's "%.6f" on every committed value (1453/4056 -> "0.358235",
  -101/2028 -> "-0.049803", 0 -> "0.000000", -966 -> "-966.000000"). The Go
  harness ignores approx and reads only num/den, so it is review-only, but
  matching the format keeps a future oracle-lean regeneration a clean diff.
- Splice technique: oracle input files are Python json indent=2 and round-trip
  byte-identical, so a new case is appended via json without touching existing
  bytes. Golden files are Lean's pretty-printer (nonstandard layout, keys in the
  order value, quality, predicates, gamma, deltas), so a new case is inserted by
  exact text edit before the final `]}`, preserving existing bytes. Input and
  golden are joined by index in the harness, so the case must be appended at the
  same trailing position in both.

## gonum adapter (mes-b0id, step 10)

- The adapter is a nested module that imports the core, but the core is not yet
  published under a release tag and the repo has no `go.work`. Resolution
  therefore needs an explicit `replace github.com/andreswebs/meso => ../` in
  `gonum/go.mod` alongside the `require`. Without the replace, building or
  testing the adapter tries to fetch a nonexistent published version of the core
  from the proxy and fails.
- gonum's `Graph.From(id)` reports a neighbour from both endpoints of an
  undirected edge, and meso's undirected `AddEdge` folds parallel edges by
  summing. Adding every `From` neighbour would double each undirected edge's
  weight. The adapter adds only the `from <= to` direction for undirected graphs
  (which also lets a self-loop through exactly once); directed graphs use `From`
  directly as out-arcs.
- Node identity crosses the boundary as the decimal string of the int64 gonum
  ID (`strconv.FormatInt`). `Build` sets `Canonical()` so the meso graph is a
  pure function of the ID set and weights, independent of gonum's map-backed and
  therefore nondeterministic node-iteration order. `Communities` inverts the
  scheme with `strconv.ParseInt` and returns an error for a Result whose keys
  are not adapter-issued IDs, so a foreign Result fails loudly instead of
  silently mismapping.
- gonum's `Graph.Nodes()` and `From()` are contractually non-nil (an empty graph
  yields `graph.Empty`, not `nil`), so the conversion needs no nil guards; an
  empty graph converts to a valid empty meso graph.
- The core's dependency-free invariant is enforced from a core test
  (`dependency_test.go`) that runs `go list -deps -f '{{.ImportPath}}' ./...`
  and asserts no `gonum.org/` package appears. `./...` in the core module does
  not descend into `gonum/`, which is a separate module, so the adapter's gonum
  dependency never leaks into the core build graph.
- Trap: `go get <pkg>` run from the repo root edits `gonum/go.mod` (the module
  that will import the package), not the core `go.mod`. Confirm the core
  `go.mod` and absence of a core `go.sum` after any dependency operation to keep
  the core dependency-free.

## LFR accuracy sweep (mes-sotf, step 11)

- No LFR generator was written in Go. The 70 committed benchmark graphs under
  `datasets/lfr/` (regimes S and B, seven nominal mixing values, five
  realizations each) are the frozen vectors: `datasets/lfr/generate.py` produces
  them out of band with networkx's `LFR_benchmark_graph`, pinned by version and
  a seed derived from the grid indices. The plan explicitly allows "a bundled
  generator wrapper", and the design note scopes the accuracy tier to consuming
  frozen vectors, so the Go side is a loader plus scorer, never a generator.
- The loader (`lfr_test.go`, package `meso_test`, public API only) builds each
  graph through the public `Builder` with `Canonical()`, registering every
  ground-truth node before the edges so an isolated node (none today, since LFR
  `min_degree` is 10) would still exist. The planting is aligned back to dense
  indices through `Graph.Index`, so `NMI`/`ARI` compare like-indexed labelings
  regardless of build order.
- Recovery is textbook-monotone under Leiden at the default objective: regime B
  goes NMI 1.00 at mu 0.1 to 0.08 at mu 0.7; regime S goes 0.97 to 0.17. The
  realized mixing recorded in the manifest runs well above nominal (nominal 0.7
  reaches realized ~0.90), which is why recovery collapses by mu 0.6-0.7.
- The reference envelope (`testdata/lfr/envelope.json`) is a frozen vector in the
  golden idiom: regenerate with `go test -run TestLFRAccuracySweep -update-lfr`.
  The flag is `-update-lfr`, not `-update`, because `golden_test.go` already
  registers `-update` and both files compile into one test binary; a second
  `flag.Bool("update", ...)` would panic at init.
- Guards are layered like the golden corpus: a tight within-envelope band
  (tolerance 0.02, effectively exact since meso is deterministic) as the
  regression pin, plus two independent sanity checks a regenerated envelope
  cannot launder away, monotone degradation (noise tolerance 0.03, and strict
  easiest > hardest) and a literature-derived low-mixing NMI floor of 0.85.
- CI cost: scoring all 70 graphs is ~76s, far too slow for the always-on suite.
  The sweep scores realization 0 of every regime/mixing cell (14 Leiden runs,
  ~15s) and `-short` narrows to regime S. The other four realizations stay
  committed as frozen vectors for the benchmark and mutation tiers.

## Benchmark suite and regression gate (mes-qudi, step 12)

- The benchmark suite (`benchmark_test.go`) runs `Leiden` and `Louvain` over a
  small-to-large tier: the three corpus graphs (karate, dolphins, lesmis) for
  small, plus two deterministic planted-partition graphs (`planted-300`,
  `planted-800`) for medium and large. Graph construction happens once in
  `benchGraphs` before the timed loop; `b.ReportAllocs()` records allocs/op.
- `plantedGraph` wires each node to a fixed number of intra- and inter-community
  neighbours chosen from the package PRNG, so a given size yields the exact same
  graph every run and ns/op stays comparable. The `Builder` folds the duplicate
  edges the random wiring produces.
- Sizes were calibrated to this implementation's runtime, which is roughly
  O(n^2) (correctness/determinism over speed, per design section 4.5): n=1000
  Leiden is ~240ms and n=5000 is ~12s. planted-800 (~150ms) is the practical
  ceiling for an on-demand `make bench`; a truly large tier would make the gate
  too slow to run. If the core is ever optimised, bump these sizes.
- The regression gate is a Go test, not a shell script, because the core module
  is deliberately dependency-free (empty `go.mod`, no `go.sum`): adding
  `benchstat` or a benchmark-parsing library as a module dependency would break
  that invariant. The parser, median summary, and comparator live in
  `benchgate_test.go` (package `meso`, test-only), so nothing leaks into the
  library build graph.
- `benchstat` is still available for the human-facing diff via `make
  bench-compare`, which runs it with `go run golang.org/x/perf/cmd/benchstat@...`.
  The `pkg@version` form runs in isolated module-aware mode and never touches the
  core `go.mod`/`go.sum` (verified), so the dependency-free invariant holds.
- Thresholds split by metric because they have different noise profiles. ns/op
  gets generous cross-machine headroom (default 100%, i.e. a 2x slowdown fails)
  since a committed baseline is captured on one machine and CI runs on another;
  allocs/op is deterministic per graph and machine-independent, so its default
  tolerance is 0 (any growth fails). That allocs gate is the real hot-path
  allocation guard (acceptance criterion 4). Override via `MESO_BENCH_NS_FRACTION`
  and `MESO_BENCH_ALLOCS_FRACTION`; tighten ns for a same-machine before/after.
- `make validate` never runs the slow benchmarks. The gate's own logic is proved
  by a deterministic in-memory self-test (`TestDetectRegressions`) that feeds a
  slowed-down and a newly-allocating benchmark through the comparator, and
  `TestBenchmarkBaselineValid` guards that the committed baseline still parses and
  covers every tier. The env-gated `TestBenchmarkRegressionGate` is what `make
  bench-check` drives against a fresh run; it skips when `MESO_BENCH_CURRENT` is
  unset.

## Mutation testing (mes-aueb, step 12)

- The core scores **89.04% mutation efficacy** (390 killed / 438 killed+survived,
  ~99% mutator coverage) under `go-gremlins` v0.6.0. `make mutation` runs it on
  demand (like `fuzz` and `bench`, never in `make validate`) via `go run
  github.com/go-gremlins/gremlins/cmd/gremlins@<version> unleash .`. As with
  `benchstat` in `bench-compare`, `go run pkg@version` runs in isolated
  module-aware mode and never touches the core `go.mod`/`go.sum`, so the
  dependency-free-core invariant holds (verified: the module files are byte
  identical before and after a run).
- The efficacy gate lives in the committed `.gremlins.yaml`, not on the command
  line, because of a real v0.6.0 bug: `--threshold-efficacy`/`--threshold-mcover`
  arrive through viper as strings and fail an internal `float64`/`int` type
  assertion, so `configuration.Get` returns 0 and the gate is silently skipped.
  A YAML numeric value comes back as a concrete number, so only the config form
  fires. `make validate` never invokes this; the threshold sits at 85, a few
  points below the achieved 89.04% so a single new equivalent mutant or a
  load-induced timeout cannot flip the gate red. Note `go run` masks gremlins'
  exit code (10 for an efficacy breach, 11 for coverage) as a plain 1, which is
  fine for a pass/fail gate.
- Per-mutant timeout is `(2s + measured-suite-time) * timeout-coefficient`.
  gremlins measures the suite time during its one coverage-gathering run, and that
  measurement is only trustworthy on a cold test cache: run it right after
  another `go test` and the cached binary measures ~0s, making the timeout ~2s and
  turning the entire run into TIMED OUT. `MUTATION_TIMEOUT_COEFF` defaults to 8,
  so the timeout is comfortably above the ~25s suite even if the baseline is
  mis-measured. Infinite-loop mutants (an `INCREMENT_DECREMENT` flip of a loop
  counter's `i++` to `i--`, seen in `move.go` and `refine.go`) run to that ceiling
  and are reported TIMED OUT; they are effectively killed (a hang is a detectable
  behaviour change) but count toward neither side of efficacy.
- gremlins copies the whole module tree into a temp working dir per worker (once
  per worker, reused across that worker's mutants). Two traps followed from this:
  the gitignored Lean build output `verification/lean/.lake` is multiple GB, so
  each worker copy is that large and a few workers exhaust a small `/tmp` (the
  copy then fails with gremlins' placeholder `panic: error, this is temporary` out
  of `wdDealer.Get`); and pointing `TMPDIR` inside the repo makes gremlins try to
  copy the tree into itself. `MUTATION_WORKERS` therefore defaults to 1 (one copy,
  and no cross-worker resource pressure); raise it on a machine with spare cores
  and temp space. On a fresh checkout `.lake` does not exist, so the copy is small
  and the default is fine.
- The two real coverage gaps the survivors exposed both got a test back in the
  owning file. `builder.go` rejects edge and node weights with `!(w >= 0)`; only
  negatives were tested, so the `>=`-to-`>` boundary (which would reject a legal
  zero weight) survived - `TestBuilder_ZeroWeightsAccepted` pins the zero
  boundary. `parallel.go`'s `normalizeWorkers` (the worker-count clamp) and
  `isolationBase` (the per-round fresh-label base) are pure helpers whose output
  the core-count-invariance tests cannot see - a wrong clamp still yields
  byte-identical results, just with a different goroutine count - so their
  arithmetic and boundary mutants survived until `TestParallel_NormalizeWorkers`
  and `TestParallel_IsolationBase` asserted the clamped values and the
  fresh-label boundary directly.
- The remaining 48 survivors are documented equivalent or defensive mutants, in
  a handful of recurring shapes. (1) Epsilon boundaries: `> moveImproveEps` vs
  `>= moveImproveEps` (local-move acceptance in `leiden.go`, `louvain.go`,
  `parallel.go`; the density gate; `weightedChoice`) never differ because the
  compared quantity never equals the epsilon exactly. (2) Order-invariant sort
  comparators: `<` vs `<=` over distinct keys (the builder's CSR layout sorts, the
  contingency-table sort in `metrics.go`, the refinement candidate sort) produce
  the same order, and the metric sums they feed are commutative anyway. (3)
  Error-message-only arithmetic: an `i+1` inside a `checkInvariants` `Errorf`
  argument changes the reported index in a string no test asserts. (4) Validator
  boundaries unreachable for valid graphs: `checkInvariants`/`adjacency.check` only
  ever see well-formed CSRs, so `v >= n` vs `v > n` and the directed in-adjacency
  error branch never fire. (5) Canonical-partition-unreachable branches: the
  `fresh`/`isolationBase` "grow past the max label" loops never execute for a
  canonical partition (labels are already `< n`), and Leiden's
  refinement-did-not-shrink `break` is redundant with the discrete-partition guard
  above it. (6) Worker-count-invariant parallel mechanics: mutating the chunk size,
  the workers-equal-1 fast path, or the goroutine loop bounds in
  `parallelBestMoves` changes how the work is split but not the result, which is
  byte-identical across worker counts by construction (the code half of Lean's
  `applyRound_perm`), so no output or determinism test can distinguish them. A few
  survivors sit in code that is only reachable on degenerate inputs outside the
  algorithms' working domain (connectivity over a zero-weight edge, the
  conflict-resolution branch of `parallelRound` that a non-oscillating corpus never
  triggers, the AMI denominator clamp); these are noted as candidate future tests
  rather than pretended-equivalent.

## Directed modularity: Lean model and rational mirror (mes-yx9f, directed verification phase 1)

- Phase 1 of the directed-modularity verification project
  (`docs/research/directed-modularity-formal-verification.md`): a Lean model of a
  directed weighted graph, the real Leicht-Newman `directedModularity`, its
  computable rational mirror `directedModularityQ`, and the equivalence tying the
  two together. No guarantees, no oracle wiring, no Go changes. Two new files:
  `verification/lean/Meso/DirectedGraph.lean` and `Meso/DirectedCompute.lean`.
- The directed model is literally `WeightedGraph` minus the `weight_symm` field. A
  dense `weight : Fin n -> Fin n -> R` already carries direction (`weight i j` is
  the arc `i -> j`); the split out/in adjacency of the Go CSR is an implementation
  detail, not part of the mathematics. The model is deliberately standalone -
  shares no code, no typeclass, no common interface with the undirected model.
  Unifying them is an explicit open question, and keeping them separate keeps the
  Phase 2 statements direct.
- The term-for-term parallelism between `directedModularityQ` (Q) and
  `directedModularity` (R) is load-bearing, exactly as for the undirected pair: the
  F2 cast proof then closes with the mechanical `unfold; simp only [toReal_*];
  push_cast [apply_ite ...]; rfl`. Wrote the two definitions with identical shape,
  parenthesisation, and `if` indicator so nothing had to be fought in the proof.
- `ofRaw` must NOT symmetrise. The undirected `WeightedGraphQ.ofRaw` folds both
  orientations with `max` to manufacture symmetry; the directed one only clamps to
  nonnegative (`max 0 (raw i j)`), preserving asymmetry. This is the one place the
  mirror pattern deliberately diverges from `Compute.lean`.

### Deviation from the plan: fixture proofs could not use `decide`

- The plan's Task 4 primary route was `by decide` on the five fixture `example`s
  (four Q values, one move-delta). `decide` does not work on rational-arithmetic
  goals: `Rat` equality forces a gcd normalisation that the kernel does not reduce,
  so `decide` gets stuck (not slow - stuck) on `instDecidableEqRat`. This will bite
  any future ratio-valued Lean fixture; reach for `norm_num`, not `decide`.
- Used the plan's documented fallback (`norm_num`), packaged as a small local
  `check_fixture` macro: `simp only [<defs>, Fin.sum_univ_three, Matrix.*] <;>
  norm_num <;> simp <;> norm_num`. `simp only` unfolds the definitions, expands the
  `Fin 3` sums, and indexes the matrix/partition literals; `norm_num` does the Q
  arithmetic (including evaluating `max 0 k` from `ofRaw`). The trailing
  `<;> simp <;> norm_num` exists only for the move-delta example: `Function.update`
  leaves residual `if (label : N) = label` community comparisons that `norm_num`
  did not collapse on its own, so a `simp` pass reduces those ites before the final
  arithmetic. The four Q examples are closed by the first `norm_num`; the chained
  tactics then run on zero goals (no-ops).
- `native_decide` was NOT needed and is not used. It was the plan's last resort and
  is discouraged under the `mathlibStandardSet` linter; the `norm_num` route made
  it unnecessary.
- `totalWeight` is stated over out-degrees (`sum_i outDegree i`) with the column
  form as a lemma `totalWeight_eq_sum_inDegree` (proved by `Finset.sum_comm`),
  rather than as a flat double sum. This pins the out/in bookkeeping Phase 2 leans
  on and reads cleanest.
- Fixtures live at the end of `DirectedCompute.lean` (the plan's default); the file
  is well under the size of `Compute.lean`, so no separate `DirectedFixtures.lean`.
- `CORRESPONDENCE.md` divergence register: the old "directed is entirely outside the
  model" bullet was split, not deleted - one bullet now records the modelled
  objective (with `directedModularityQ_eq`), the other records that the guarantees
  remain unproved, directed stays outside the value oracle, and directed CPM is
  still unmodelled. Guarantee triage and oracle wiring are Phases 3-5.

## Directed modularity: objective identities in Lean (mes-z1f5, directed verification phase 2)

- Phase 2 of the directed-modularity verification project
  (`docs/research/directed-modularity-formal-verification.md`): prove, over the
  Phase 1 directed model, the algebraic facts the optimizer relies on. Additions to
  `verification/lean/Meso/DirectedGraph.lean` (basic value lemmas, the
  `WeightedGraph.toDirected` bridge, and the symmetric reduction), plus two new
  files: `Meso/DirectedAggregate.lean` (aggregation invariance) and
  `Meso/DirectedMove.lean` (the closed-form move-delta identity). No Go changes.
- Aggregation mirrored `Meso/Aggregate.lean` almost verbatim and built first try.
  The undirected file's partition-level helpers (`numComm`, `commLabel`,
  `sum_commLabel`, `sum_diagonal_eq_sum_fiber`) carry no symmetry assumption, so
  they were imported and reused, not re-proved. The directed aggregate is strictly
  easier to construct than the undirected one: there is no `weight_symm` field to
  discharge. The one genuinely new lemma is `directedAggregate_inDegree` (the
  transposed companion of the out-degree lemma); symmetry made one degree lemma
  suffice undirected.
- The move-delta identity (`directedModularity_move_eq`) was the hard item and has
  no undirected counterpart to copy (the undirected core never proved its
  closed-form ΔQ; its Go formula is property-tested only). It was proved in full,
  NOT descoped to the singleton fallback the ticket allowed. Structure that worked:
  isolate the partition combinatorics in a kernel-generic
  `sum_kernel_move_split` (how one move perturbs an indicator-weighted double sum),
  proved once independent of the modularity algebra; then read the perturbed degree
  sums off as full community degrees and close each row/column term with `ring`.

### Lean mechanics traps hit (all cost a build cycle each)

- `sum_kernel_move_split` needs NO hypothesis on the target `t`: for the no-op
  `t = p u` both correction sums vanish. The ticket stated `t ≠ p u`; keeping it
  triggered the unused-variable linter, so it was dropped. `t ≠ p u` enters only the
  main identity, where a source community's degree sum must exclude `u`'s own term.
- `move p u t u` is only DEFEQ to `Function.update p u t u`, not syntactically
  equal, so `Function.update_self`/`update_of_ne` cannot be applied to a `move` goal
  by `exact`; unfold first (`simp only [move]; exact Function.update_self ...`).
- Rewriting an equality inside an `if` with `rw [eq_comm ...]` fails: the `ite`'s
  `Decidable` instance depends on the proposition, so the motive is not type-correct
  ("Application type mismatch ... Decidable"). Use `simp only [eq_comm]`, which
  normalizes both orientations to a canonical form via its loop-guard.
- `simp only [] at h` to beta-reduce a hypothesis is unreliable here (reports "made
  no progress" and a spurious "function expected" under `instances` transparency).
  Two robust alternatives used instead: (1) state the `have` with the fully
  beta-reduced type and let defeq check the term against it (used to give
  `sum_kernel_move_split`'s application a `rw`-able shape); (2) restructure the proof
  to avoid needing beta at all (`sum_erase_mul_ind_eq` via
  `rw [eq_sub_iff_add_eq, add_comm]; exact Finset.add_sum_erase ...`).
- `Finset.add_sum_erase Finset.univ _ (mem_univ u)` with `_` for the summand leaves
  `AddCommMonoid ?m` stuck; supply the summand lambda explicitly.
- `#print axioms` confirmed `directedModularity_aggregate_eq`,
  `directedModularity_move_eq`, and `sum_kernel_move_split` depend only on
  `[propext, Classical.choice, Quot.sound]`: no `sorryAx`, no `native_decide`.

## Directed modularity: guarantee triage (mes-qmch, directed verification phase 3)

- Phase 3 was research, not proof engineering: the deliverable is the triage
  report `docs/research/directed-modularity-triage.md`, produced by an
  out-of-band scouting harness (`verification/reference/directed-scout/`, a
  standalone Go module in the `crosscheck.py` mold) plus derivations from the
  Phase 2 move identity. No Lean changes, no production Go changes.
- Verdicts: weak connectivity PROVABLE (0 violations in 15228 communities;
  strong connectivity fails in 39 percent, so weak is the right statement);
  gamma-separation conditional on level stability PROVABLE (0 violations in
  3240 runs, bound tight at equality, closed form matches from-scratch to
  4.3e-15); subset-optimality REFUTED as an output property (43 of 3240
  converged samples admit an improving subset split; minimal n=4 fixture
  hand-verified, reproducible via `go run . -hunt-subset`).
- The separate-module scout design is load-bearing, not packaging taste: as
  its own module it can only import meso's public API, which physically forces
  every bound to be recomputed independently from the edge list and returned
  partition. The per-sample guard "scout Q == Result.Quality()" (max err
  1.1e-16 over the grid) plus the igraph 1.0.0 exact match then close the
  convention loop from both sides.
- The biggest scout hazard is false counterexamples from non-convergence.
  meso's raw output admitted a further improving move or merge under the
  scout's exhaustive scan in 396 of 3240 samples (meso stops at
  moveImproveEps over neighbour candidates; the scout scans all targets
  including isolation, and all merge pairs). All gain-bound judgments were
  made only at scout-driven verified fixed points.
- Know which check is informative. At a driven fixed point the
  gamma-separation bound holds by construction (the bound IS "no merge
  improves" rearranged), so the informative gamma-separation checks are the
  raw meso output (all clean) and the closed-form-vs-from-scratch agreement.
  Subset splits are NOT covered by the driving dynamics, so subset violations
  at driven fixed points are genuine findings, not artifacts.
- The subset-optimality refutation is not a directed phenomenon: the
  symmetric regime (equivalent to undirected modularity by the proved
  reduction) shows the same violations. Converged-but-not-subset-stable
  outputs are inherited modularity behaviour; the undirected model never
  claimed modularity subset-optimality either (its guarantee is CPM-only,
  from an assumed IsSubsetStable). This context keeps the refutation from
  reading as a directed regression.
- Subset masks come in complement pairs and the directed subset bound is
  symmetric under S <-> C\S, so exhaustive mask enumeration counts every
  split exactly twice; report distinct splits, not raw mask hits.

## Leiden subset-optimality gap: mechanism and decision (mes-niic)

- The triage's subset-optimality refutation was investigated to a full
  mechanism on the n=4 fixture. It is a three-stage lock-in, not a refinement
  bug: (1) phase-1 greedy local moving settles in a wrong-pairing basin
  ({0,2},{1,3}); (2) refinement only splits within phase-1 communities, and
  both pairs are internally cohesive, so it keeps them whole and aggregation
  freezes that granularity; (3) the aggregate level merges to all-in-one, and
  the improving split {0,1},{2,3} crosses the frozen boundaries, permanently
  unreachable in meso's single pass. The paper's escape mechanism is iterated
  refinement at base granularity, which meso never runs.
- meso's phase-1 is completely seed-independent: only refinement consumes
  WithSeed, and on the fixture refinement has one forced candidate per node,
  so all 10000 swept seeds return the same stuck partition. Consequence:
  multi-seed restarts are NOT a fix for this class of local optimum; every
  seed replays the same phase-1 basin. Any future "just add restarts"
  suggestion should be checked against this finding.
- When reproducing a scout fixture through the public API, replicate the
  exact node registration order. The builder interns keys in first-seen order
  (Canonical() is the opt-in reordering), and the fixture flips between the
  stuck all-in-one and the optimal split depending on whether nodes are
  registered 0,1,2,3 (AddNodeWeight first, as the scout does) or interned
  from the edge list (0,1,3,2). The first reproduction attempt tested a
  relabeled isomorph and wrongly suggested the ticket's repro was invalid.
- The reference reproduces the gap per run: leidenalg 0.10.2 with
  n_iterations=-1 returns the suboptimal all-in-one on 9 of 200 seeds from a
  singleton start, and stays stuck on 57 of 200 seeds when started from
  all-in-one; 100 forced iterations always escape. The paper's
  subset-optimality guarantee is asymptotic over repeated randomized
  iterations, never a per-run property, and leidenalg's -1 semantics stop at
  the first non-improving pass, which can be an unlucky one.
- Decision: add WithIterations(k) with a fixed pass count only, default 1
  (byte-identical to today), each pass restarting from the previous base
  partition with a per-pass derived refinement seed. An until-stable mode was
  deliberately dropped as misleading. Rejected alternatives: multi-seed
  restarts (see above) and a subset-aware repair pass (exponential or an
  unformalizable heuristic that deviates from Leiden as specified). The
  phenomenon is surfaced in the option's godoc, where the reason to set k
  above 1 is the honest place to state it.

## Directed value-oracle (mes-43lb, directed verification phase 5)

- Phase 5 wired directed modularity into the standing Lean value-oracle at
  parity with the undirected core: `mesoOracle` now emits exact directed Q,
  move-delta, and guarantee-predicate vectors, and the Go harness
  (`directed_oracle_test.go`) asserts its float64 directed pipeline against
  them. New Lean file `Meso/DirectedPredicates.lean`; `Meso/OracleIO.lean` grew
  a parallel directed IO path; the generator got a `--directed` flag; four
  directed fixtures landed (`directed_asym3`, `directed_subset4`,
  `directed_cycles6`, `celegansneural`).
- The directed IO path is a *parallel* evaluator, not a generalization. The
  directed graph is a distinct Lean type (`DirectedWeightedGraphQ`, no
  `weight_symm`), so `runFile` branches on a top-level `"directed": true` flag:
  the directed branch builds via `DirectedWeightedGraphQ.ofRaw` (which clamps to
  nonnegative but never symmetrizes) and calls `directedModularityQ` /
  `moveDeltaDirectedModularityQ`. `evalQuality`/`evalDelta` had to change return
  type to `Except String` so the `.directedModularity` arm is an error, not a
  silent wrong value, if it ever reaches the undirected evaluator. The parser
  enforces the pairing both ways (directed input demands `directedModularity`;
  an undirected input rejects it), so a directed graph can never be scored by a
  symmetric objective.
- The `QKind` docstring said directed was out of scope; adding
  `.directedModularity` needed `DecidableEq` on `QKind` for the parser's
  pairing check (`c.quality ≠ .directedModularity`). `deriving Repr,
  DecidableEq`.
- Trap: two Lean files defining `WeightedGraphQ.simpleGraph` and
  `DirectedWeightedGraphQ.simpleGraph` both auto-generated an instance named
  `Meso.instDecidableRelFinAdjSimpleGraph._aux_1`, and importing both into
  `OracleIO.lean` failed with "environment already contains ...". Fix: give the
  directed instance an explicit name
  (`instDecidableRelDirectedSimpleGraphAdj`) so the auto-name collision
  disappears. Any `inferInstanceAs`-defined instance mirrored across the
  directed/undirected split risks this; name them.
- The connectivity fast decider ported verbatim from `communityConnectedFast` /
  `reflTransGen_radjQ_iff_reachable` with only the adjacency swapped to the
  directed `simpleGraph`; the `Reachability` layer is graph-agnostic and reused
  unchanged. This was the plan's one "substantial proof" and it was mechanical
  because the directed `simpleGraph` is still `fromRel`-symmetric (weak
  connectivity), so the induced-subgraph reachability argument is identical. All
  three `_iff` theorems are `native_decide`-free and depend only on `propext`,
  `Classical.choice`, `Quot.sound`.
- The Go weak-connectivity decider MUST traverse both out- and in-adjacency
  (`g.neighbors` and `g.inNeighbors`); the undirected `communityConnected` walks
  out-arcs only. A directed community `{u, v}` joined solely by `u -> v` is
  weakly connected, and an out-only DFS from `v` would never reach `u` and
  wrongly report it disconnected, disagreeing with the proved flag. The
  asym3 `[0,1,0]` case (community `{0,2}`, only arc `0->2`) is the committed
  witness that pins this.
- The directed predicate object omits `gammaDense` (CPM-only; directed CPM is
  unmodelled), so the Go loader's `GammaDense` field became `*bool` (nil on a
  directed case, non-nil on undirected). Every undirected consumer had to switch
  to `*flag`/nil-guard, mirroring how `SubsetOptimal` was already handled. The
  directed `subsetOptimal` flag is emitted as a *characterization*, not a
  guarantee: it reads `false` on `directed_subset4`'s converged all-in-one (the
  n=4 refutation fixture, cross-referenced to `mes-niic`) and `null` above the
  node bound (celegans). Its `_iff` holds regardless of whether the algorithm
  attains the subset-stability hypothesis.
- Fixture branch coverage was engineered from the generated goldens, not
  guessed: asym3 supplies both a weak-but-not-strong connected community
  (`[0,1,0]`, flag true) and a disconnected one (`[0,1,1]`, flag false) plus a
  false γ-separation case; subset4 supplies the false directed subset flag;
  cycles6 supplies a false γ-separation branch (grouping across the bridge arc).
  Every directed vector test asserts two-sided teeth.
- celegansneural (297 nodes, 2359 arcs, from the same Newman repository as the
  existing corpus, directed and weighted, cleanly attributed to White et al.
  1986 / Watts-Strogatz) exercises directed values, weak connectivity, and
  γ-separation at scale; its `subsetOptimal` is null. The exact-ℚ Lean oracle
  takes ~3.5 min to evaluate it (O(n^2) directed Q plus the polynomial
  connectivity decider over exact rationals) - slow but out-of-band only
  (`make oracle-lean`), never in CI; the Go harness reads the committed vectors
  in milliseconds.
- The generator's `--directed` flag is position-robust: flags are stripped from
  argv before the positional `gml/name/out` are read, so
  `gml_to_input.py --directed <gml> <name> <out>` and the trailing-flag form
  both work. It requires a GML `directed 1` header and errors otherwise, so a
  directed run cannot silently misread an undirected graph. celegans emits the
  two anchor partitions (all-in-one, singletons) only; no ground truth exists
  for it.

## Spec 002: Go 1.27 toolchain and benchmark baseline (mes-rfgu)

- The committed `benchmarks/baseline.txt` is recorded on linux/arm64 with
  GOMAXPROCS 4, in Docker on the maintainer's machine, not on the host. Keep
  regenerating it the same way so the benchmark names and machine profile stay
  comparable:
  `docker run --rm --cpus=4 -v "$PWD:/work" -w /work golang:<version>-bookworm go test -run='^$' -bench=. -benchmem -benchtime=10x -count=6 . > benchmarks/baseline.txt`.
- A baseline compared across months reads slower from machine drift alone
  (about 12% here). Attribute a change to the toolchain only with an
  interleaved A/B: the old tree on the old image and the new tree on the new
  image, run back to back. Go 1.26.5 to 1.27.1 showed no significant ns/op
  change; planted-300 lost a few allocations with bit-identical output.

## Spec 002: builder drops zero-weight edges (mes-dj0k)

- `Build` now omits any edge or arc whose folded weight is zero; `AddEdge`
  still registers both endpoints, so they survive as nodes.
- The change is invisible to the optimisers. A zero weight adds nothing to
  modularity or CPM, and `communityConnected` (the Lean `CommunityConnected`
  image) already followed only positive-weight edges. What it fixes is the
  structural view the v0.2.0 accessors and measures expose: without it a
  zero-weight entry would count as an edge in `NumEdges`, `Degree`,
  `Cohesion` and as a hop in `Betweenness`.
- Consequence for tests: the zero-weight-bridge test passes with and without
  the change, so it is an end-to-end guard, not a red-green proof. The
  discriminating tests are the adjacency assertions in `builder_test.go`.

## Spec 002: networkx betweenness references (mes-ar6b)

- meso's `Betweenness` agrees with networkx 3.4.2 to the last few bits on the
  corpus (worst 5.6e-17; lesmis bit-exact), because the accumulation uses
  networkx's own form `sigma[v] * (1 + delta[w]) / sigma[w]`. The Go test holds
  it to 1e-12, not the 1e-9 the plan allowed.
- The commonly quoted karate values (node 34 "0.3040") are truncated, not
  rounded: the true value is 0.30407. Compare quoted figures within one unit
  in the last quoted place.
- `celegansneural.gml` lists some arcs twice without declaring `multigraph 1`,
  so `networkx.read_gml` refuses it. `datasets/betweenness.py` retries with the
  header injected and collapses to a simple `DiGraph`, the same folding as
  meso's builder (2359 GML edge blocks become 2345 arcs).
- Typing a networkx script: pyright strict reads networkx's bundled stubs,
  where `Graph` is generic, while ty reads the runtime package, where it is
  not, so `nx.Graph[int]` satisfies one checker and fails the other. Follow
  `datasets/lfr/generate.py`: hold the graph as `cast("Any", ...)` at the
  library boundary, cast the values you read out to concrete types, and put a
  targeted `# pyright: ignore[reportUnknownMemberType]` on the stub-gap calls.
  The repo's Python scripts are not `ruff format`ted; formatting would also
  split those calls away from their ignore comments.

## Spec 002: benchmarks for the structural measures (mes-z1rf)

- A new benchmark is invisible to the regression gate until two things change.
  `make bench` only runs names matching `BENCH_RE` in the Makefile, and
  `detectRegressions` iterates over the baseline's names, so a benchmark absent
  from the baseline is never compared. Add the name to `BENCH_RE`, to the
  `want` list in `TestBenchmarkBaselineValid` (which then fails red until the
  baseline is regenerated), and regenerate the baseline.
- `Betweenness` allocates a constant 9 to 11 times per call from karate up to
  the 1000-node LFR graph, which is the allocs/op gate's real job here; its
  ns/op is about 100 ms on the LFR graph, the costliest benchmark in the suite.
