# meso oracle: the Lean value-oracle

> Describes how `meso` validates its numeric core against an external reference.
> This is the supporting side-track referenced from
> [docs/specs/001-initial-implementation/plan.md](plan.md) as "(consumes Lean
> golden vectors)". Derived from [docs/meso-design.md](../../meso-design.md) sections 6.2
> and 7.

`meso` has a formally verified Lean model of its quality functions and
guarantees, and for a verified library the proven model is a stronger oracle than
a second unproven implementation: it is the exact artifact the proofs reason
about, so using it as the test oracle makes the tested code faithful to the
verified design rather than to an independent implementation that could share a
misconception with the code under test.

The approach:

- The Lean model is the sole standing oracle, for the pure quantities and
  predicates it covers.
- Output correctness is checked against the proven invariants, not an empirical
  envelope.
- Output accuracy is scored against planted ground truth (LFR) and published
  literature values, neither of which needs a running reference.
- The Java and Python implementations are used as porting references
  (design section 5) plus a one-time spec-blessing cross-check; they never run in
  CI.

## Why an oracle at all

Community detection has no single right answer and its hard bugs are silent: a
worse partition, not a crash. A pure-Go unit test cannot tell a subtly wrong
partition from a correct one, and it cannot tell a subtly wrong quality
computation from a correct one either. The oracle establishes an external
reference for the parts that have one.

Two things need an external reference, and they are different:

1. The numeric core: the quality functions (`Q`, `Q_gamma`, CPM, directed
   modularity), the move-delta, and the invariant predicates. These are pure
   functions of `(graph, partition)`. A wrong formula here is silent.
2. The search result: whether the partition `meso` returns is a good one. This is
   the heuristic's job and has no closed-form right answer.

The Lean value-oracle covers (1). Nothing external produces a partition to
compare against for (2); instead (2) is pinned by the proven invariants and by
accuracy benchmarks (see below).

## What the Lean value-oracle is

Lean 4 is a compiled functional language, not only a proof assistant: a
definition can be evaluated and serialized to JSON. The tier V1 through V3
development already contains Lean definitions of the quality functions and the
guarantee predicates that the proofs reason about. Executing those same
definitions to emit expected values makes the test oracle identical to the
artifact that was proven correct. No other reference gives that: the Java and
Python references are independent implementations, not proven ones.

This is the concrete mechanism for the design's "verified design, tested code"
gap (section 7): running the Lean model as an oracle is how the Go implementation
is held faithful to the verified model, rather than to a second unproven oracle.

### The one obstacle: the model is noncomputable

The model is deliberately noncomputable. `modularity` (`Meso/Quality.lean`) and
`cpm` (`Meso/CPM.lean`) are defined over the reals (`noncomputable def ... : ℝ`),
matching the design's decision to model quality over rationals/reals and keep
floating point an implementation detail. You cannot evaluate them to a number as
they stand.

Bridging this is the promoted work (see
[verification/lean/README.md](../../../verification/lean/README.md)). The standard move:

1. Add a computable rational mirror in Lean, `Meso/Compute.lean`:
   `def modularityQ (G : WeightedGraphQ n) (γ : ℚ) (p : Partition n) : ℚ` over ℚ,
   with `Fin n → Fin n → ℚ` weights, and the CPM twin `cpmQ`.
2. Prove it agrees with the real model on rational inputs:
   `theorem modularityQ_eq : (modularityQ G γ p : ℝ) = modularity G.toReal γ p`.
   This is the load-bearing step: it makes the runnable thing provably the same
   as the proved thing.
3. A `lake exe mesoOracle` reads a committed input set (corpus graphs and fixture
   partitions with rational weights) and emits the exact rational quality,
   move-delta, and predicate values as committed golden vectors.
4. Go asserts its `float64` result lands within a float-rounding tolerance of the
   exact rational value.

The Go-vs-Lean comparison is then "does Go's `float64` land within tolerance of
the exact rational value," a meaningful check, rather than comparing two float
pipelines.

### Scope it to what Lean wins at

- In scope (Lean is the oracle): the pure quantities and predicates. Quality
  functions (`Q`, `Q_gamma`, CPM), the move-delta expected value (see
  [docs/research/move-delta-verification.md](../../research/move-delta-verification.md)),
  and invariant predicates (community connected, partition well-formed,
  γ-separation, γ-connectivity, subset-optimality).
- Out of scope (Lean adds nothing here): running Leiden/Louvain in Lean to
  compare partitions. It hits the same cross-language divergence as any other
  implementation (different PRNG, tie-breaks, iteration order), and would mean
  maintaining a second production algorithm in lockstep with `meso`'s exact
  determinism model. Skip it.
- Now covered by Lean: directed modularity. The directed objective, its
  algebraic identities, and the surviving guarantees (weak connectivity,
  γ-separation, and the directed subset bound as a characterization) are modelled
  and proved standalone, and the value-oracle emits directed quantity and
  predicate vectors behind the `"directed": true` input flag (directed
  verification epic `mes-crz3`; see the directed input-schema note under "Golden
  vector format"). Directed CPM stays unmodelled by design.

Division of labor: Go owns the inputs, Lean owns the expected values for the
parts that are pure math.

- Go generates and owns the fixtures and fuzz inputs (weighted, self-looped,
  directed random graphs); Go's native fuzzing and `testing/quick` are the right
  tools, not Lean generators.
- Lean evaluates the quality functions and deltas over the committed input set
  and emits golden vectors; Go asserts equality against them.

## Pinning output correctness and accuracy without a differential harness

The retired harness defined an envelope: `meso`'s partition quality had to fall
within the range two independent implementations produced. That envelope was
always a weak proxy. The real specification of a correct Leiden output is the
paper's guarantees, and `meso` has them proven.

- Output correctness: the proven invariants, checked empirically in Go on the
  corpus and on fuzzed inputs. A partition that is not locally optimal, not
  connected, or not subset-optimal violates a proved theorem (see the
  theorem-to-test table in
  [verification/lean/CORRESPONDENCE.md](../../../verification/lean/CORRESPONDENCE.md)).
  This is stronger than "quality landed in a band": it is the actual Leiden spec.
- Output accuracy: NMI and ARI against planted ground truth on LFR benchmark
  graphs. LFR graphs ship their ground truth by construction, so this needs no
  running reference. Recovery thresholds across the mixing-parameter sweep come
  from the LFR and Leiden literature.
- The canonical corpus: karate, dolphins, Les Miserables have published
  partitions and published quality values (for example karate modularity around
  0.4198). These are pinned in-repo as literature values, and `meso`'s own exact
  partition is pinned in-repo because `meso` is deterministic.

The one thing the envelope caught that these do not: `meso` could satisfy every
invariant yet settle in a worse local optimum than a great implementation would.
That is an accuracy concern, and the LFR NMI/ARI sweep is the measure for it.

## The one-time spec-blessing cross-check (in scope)

The one thing the Java and Python references validate that Lean cannot: that the
Lean formalization of modularity and CPM matches the world's definition. If
`Meso/Quality.lean` encoded a subtly wrong definition, `meso` (held faithful to
Lean) and Lean would agree on the wrong thing and nothing would notice. An
independent implementation guards against a wrong specification, not just wrong
code.

So the references are kept for a one-time, out-of-band cross-check, not a
standing harness:

- Scope: a handful of fixtures. Run the Java `networkanalysis` CLI and Python
  `leidenalg`/igraph on the same `(graph, partition)` inputs, and confirm their
  computed quality values agree with Lean's exact rational values within
  tolerance. For directed modularity, where Lean has no model, confirm Go's
  from-scratch directed evaluator agrees with `leidenalg`/igraph.
- Trigger: once, and again only when a reference version or the Lean
  formalization changes. Never in CI.
- Output: a short recorded note (reference versions, date, that the values
  agreed), committed alongside the golden vectors. After it passes, the
  references revert to being porting references (design section 5) and
  documentation.
- Mechanism: a Dockerfile per reference (one for the JVM plus the pinned
  `nl.cwts:networkanalysis` jar from Maven Central, one for Python plus pinned
  `leidenalg` and igraph) makes the one-time run reproducible. These images are
  reproducibility aids for the cross-check, not a committed vector-generating
  pipeline.

Licensing note: the references are GPL; the cross-check invokes them as separate
processes and records only derived numbers, not their code.

## Corpus covered

- Canonical small graphs with known structure: Zachary karate club, dolphins,
  Les Miserables. Pinned against published literature values.
- LFR benchmark graphs across a mixing-parameter difficulty sweep, with planted
  ground truth. Scored by NMI/ARI against the planting.

## Golden vector format

Realized by the `mesoOracle` executable. JSON, committed under a
fixed path so it diffs cleanly in review.

- Inputs live under `verification/oracle/inputs/*.json`, the single source of
  truth: `{ n, edges: [[i,j,w]], nodeSizes?, directed?, cases: [{quality, gamma,
partition, deltas?}] }`. Weights and sizes are integers or exact `"p/q"` strings,
  never floats; each undirected edge is listed once; a self-loop is `i == j`.
- A top-level `"directed": true` marks a directed input. Each edge is then a
  single arc `from -> to`, listed once and never symmetrized, and the only
  accepted `quality` is `"directedModularity"` (the parser rejects the pairing
  either way, so a directed graph is never scored by an undirected objective or
  the reverse). The graph is built through the verified
  `DirectedWeightedGraphQ.ofRaw`, which clamps to nonnegative but does not
  symmetrize. The output envelope echoes `"directed": true`; undirected outputs
  are unchanged. The generator `gml_to_input.py --directed` honors a GML
  `directed 1` header and emits this shape.
- Outputs live under `verification/oracle/golden/*.json`: per case, the exact
  value as `{ "num", "den" }` (strings, so no JSON-number precision is involved)
  plus a float `"approx"` for review, and one record per requested move-delta.
- A `provenance` block records the generator, the Lean toolchain, and the
  input-set identifier. Generation date and git commit are added by the
  `make oracle-lean` wrapper, which the executable does not read.

CPM is emitted in the canonical (leidenalg) convention (`cpmCanonicalQ`), so
meso's CPM numbers match the published literature. It equals the proof-side
`cpmQ` minus a partition-independent diagonal (`cpmCanonicalQ_eq`), so the
optimum and the guarantees are unchanged; the spec-blessing cross-check compares
it directly to leidenalg. Modularity is emitted as is and matches igraph
directly. The committed input set covers the tiny fixtures plus the corpus graphs
karate, dolphins, and Les Miserables.

Predicate vectors are also emitted: each case carries a `predicates`
object with the boolean guarantee values for its partition, `connected`, `gammaDense`,
`gammaSeparated`, and `subsetOptimal`. Each flag is the value of a computable Bool
mirror proved equal to the corresponding paper predicate (`Meso/Predicates.lean`), so it
is a proved oracle value, not an independent check. Connectivity uses an efficient
reachable-set closure (`Meso/Reachability.lean`) rather than Mathlib's exponential
walk-enumeration decision, so it is emitted at corpus scale; `subsetOptimal` enumerates
community subsets, so it is emitted only below a node-count bound and is `null` above it.
Well-formedness is not a vector: a `Partition` is total by construction, so it is a
Go-side range check, not an oracle value.

The tiny fixtures carry deliberate predicate-flipping cases so every flag emits both
branches, not only the passing one: a disconnected community (path3 `[0,1,0]`,
`connected` false), a subset with a sparse internal cut (square all-in-one at γ=1,
`subsetOptimal` false), and a dense cut between communities (triangle `[0,0,1]` at
γ=1/2, `gammaSeparated` false); `gammaDense` already emits both (square at γ=1/3). This
gives the Go guarantee tests a proved oracle value on the failing branch too, so a check
hardwired to the passing value cannot slip through.

A directed case's `predicates` object is `{connected, gammaSeparated, subsetOptimal}`,
with **no `gammaDense` key**: γ-density is a CPM guarantee and directed CPM is
unmodelled. `connected` is weak connectivity (`DirectedConnectedCommunitiesFast`, the same
efficient reachable-set closure over the `fromRel`-symmetrised arc relation), `gammaSeparated`
is the bidirectional directed separation bound, and `subsetOptimal` is the directed subset
bound. Each is a computable Bool mirror proved equal to its Phase 4 real predicate
(`Meso/DirectedPredicates.lean`). The directed `subsetOptimal` flag is a
*characterization*, not an output guarantee: per the Phase 3 triage the algorithm does not
always attain subset stability, so it reads `false` on `directed_subset4`'s converged
all-in-one (the n=4 refutation fixture, cross-referenced to `mes-niic`) and `null` above the
node bound (celegans). The directed fixtures carry deliberate flag-flipping cases the same
way: `directed_asym3` supplies a weakly-but-not-strongly connected community (`[0,1,0]`,
`connected` true) and a disconnected one (`[0,1,1]`, `connected` false) plus a
`gammaSeparated` false branch; `directed_subset4` supplies the false `subsetOptimal` branch;
`directed_cycles6` supplies a `gammaSeparated` false branch across the bridge arc.

## Determinism and reproducibility caveats

- `meso` does not attempt to match any reference PRNG stream; that is infeasible
  cross-language and brittle (design section 4.4). Where the oracle compares a
  produced partition (accuracy, corpus), it compares invariants and scores, never
  exact membership.
- The Lean value-oracle is exact: over rationals it emits the exact value, and Go
  is compared within a float-rounding tolerance. There is no reference-version
  drift to track, because the oracle is the proved model.
- The Lean oracle scopes to small and medium fixtures: rational arithmetic will
  not run over large graphs. Big-graph confidence rests on the proven invariants
  checked in Go and on fuzzing, not on the value-oracle.

## Go-vs-Lean comparison tolerance (decided)

The policy for asserting Go's `float64` against the exact rational is fixed; the
per-function integer budgets are calibrated against the corpus once the Go core
exists (gated on it), but the shape below does not change.

- Reference value: the correctly-rounded rational, not an arbitrary float. Build a
  `big.Rat` from the golden `num`/`den` and round it once
  (`new(big.Rat).SetFrac(num, den).Float64()`, round-to-nearest-even). This is the
  best any `float64` pipeline can produce, so the test measures how far Go is from
  optimal rounding rather than comparing two unrelated floats.
- Metric: ULP distance with an absolute floor. A case passes iff
  `ulpDiff(got, want) <= B_q` OR `|got - want| <= atol`. ULP distance is inherently
  relative, so it judges modularity (order 1) and CPM (values in the thousands) on
  the same significant-digit scale with one constant; a fixed absolute epsilon
  cannot. On the tiny fixtures this lands at 0 to 2 ULP, effectively bit-exact, a
  strong regression signal.
- The absolute floor `atol = 1e-12` is load-bearing. ULP distance explodes near
  zero (adjacent floats there are denormals), so every exact-zero case, square's
  `CPM = 0`, karate's all-in-one `modularity = 0`, and the no-op move-delta, would
  fail a pure ULP check without it. `1e-12` sits well above `float64` noise (about
  `1e-16`) and well below any partition-quality difference that could matter.
- Per quality function: modularity and CPM take a small budget (about 16 ULP);
  CPM's large magnitude is irrelevant because the metric is relative, and the floor
  covers `CPM = 0`. The move-delta takes the tightest budget (about 4 ULP) because
  it is a difference of quantities with the most cancellation risk and is the
  highest-value bug catch (an incremental formula, a different computation than
  recompute-before-after); the no-op move is asserted exactly `== 0`, not within
  tolerance.

## LFR accuracy sweep (decided)

The accuracy benchmark is a fixed, versioned suite of LFR graphs committed under
`datasets/lfr/`, generated once and out of band, scored in CI. The graphs are not
generated at test time: a runtime generator would put its own unverified
correctness in the trusted path and make the realized difficulty depend on it. A
committed suite is reproducible, variance-free, and matches how the corpus and the
reference cross-check are already handled (own the inputs; generation is a pinned,
documented, out-of-band step).

- Generator: networkx `LFR_benchmark_graph`, pinned by version, driven by
  `datasets/lfr/generate.py` (a self-contained `uv` script). Every seed is derived
  deterministically from the grid indices, so re-running reproduces the suite. Each
  graph is written as a 1-based edge list plus a `-ground-truth.csv` planted
  partition, matching the `datasets/` convention; `datasets/lfr/manifest.json`
  records per graph the parameters, seed, edge count, community count, realized
  average degree, and realized mixing.
- Grid: `N = 1000`, degree exponent `τ1 = 2`, community-size exponent `τ2 = 1.5`
  (networkx requires `τ2 > 1`, so not the classic `1`), degree floor
  `min_degree = 10` and `max_degree = 50` (networkx's `average_degree` sampler is
  unreliable at `τ1 = 2`; the realized average degree lands near 23 to 26 and is
  recorded), two community regimes S (10 to 50) and B (20 to 100), nominal mixing
  `μ ∈ {0.1 … 0.7}` step `0.1`, and 5 realizations per grid point (70 graphs, about
  7 MB). Self-loops that networkx emits are stripped; the committed graph is simple
  and undirected.
- Nominal vs realized mixing: networkx's `μ` knob runs low, so nominal `0.1 … 0.7`
  realizes about `0.15 … 0.90` (recorded per graph). This is fine because the
  planted ground truth is exact regardless of `μ` and the difficulty axis is smooth
  and monotone; thresholds key off the recorded realized `μ`, not the nominal knob.
- Metrics: NMI (normalized by the arithmetic mean of entropies, the Danon/igraph
  and LFR-literature convention), plus chance-corrected AMI (Vinh et al. 2010) and
  ARI (Hubert-Arabie 1985). Gate primarily on NMI, secondarily on ARI; report AMI.
  The metric implementations keep their own unit tests (identical labelings give 1,
  independent give about 0 for the adjusted variants, ARI against hand-computed
  values), separate from the sweep.
- Thresholds: conservative lower bounds taken from the LFR/Leiden literature, never
  from meso's own output, asserted on the mean over the 5 realizations at each grid
  point (per-realization variance near the detectability limit is too high to gate
  on). Keyed to realized `μ`: `≤ 0.3` requires NMI `≥ 0.95`; about `0.45` requires
  `≥ 0.90`; about `0.60` requires `≥ 0.70`; about `0.72` requires `≥ 0.30`; `≥ 0.82`
  is report-only (beyond the detectability limit for these parameters). The B
  regime's floors are a touch higher than S and are pinned from the same literature.
  These sit well below what Leiden achieves, so realization variance will not cause
  flaky failures, yet a genuine regression still trips.
- Placement: generation is out of band (never in CI); scoring is in CI, since it is
  deterministic (fixed committed graphs, fixed meso seed) and cheap (Leiden on 1000
  nodes is sub-millisecond). The labeled real-world graphs already in `datasets/`
  (football's 12 conferences, polbooks' leanings) fold in as extra NMI/ARI points
  under the same lower-bound discipline.
- Gated on the Go core: only the design, the committed suite, and the manifest can
  land now; the scorer and the threshold assertions arrive with the Leiden
  implementation (plan Step 11). Open sub-choices left to that step: 5 vs 3
  realizations, and whether to add an `N = 10000` tier behind a build tag.

## Converged-partition case

Each corpus graph (karate, dolphins, Les Miserables) carries a committed case
holding meso's converged modularity partition, alongside the degenerate anchors
(all-in-one, all-singletons, and karate's ground-truth split). It is generated by
running Leiden deterministically at a documented fixed seed (the default seed 0;
see `oracleConvergedSeed`), scored for modularity at gamma 1. This serves two
purposes and only these two: a determinism/regression pin (meso is deterministic,
so its converged partition per graph is stable, committed, and re-checked for
drift by `TestOracleConvergedPartitionPinned`), and value-correctness on a real,
non-degenerate output (meso computing the modularity of its own rich output
correctly to float tolerance, a path the degenerate anchors do not exercise).

It is not an optimality assertion; the value-oracle blesses a partition's quality,
not its optimality (that is what the proven invariants and the LFR accuracy sweep
are for). Modularity maximization is NP-hard, so meso's converged partition is a
strong local optimum. For karate the achieved value (49/117, about 0.4188)
corroborates the published optimum (about 0.4198) out of band; the published
optimum is never hand-entered as the input partition. A CPM analogue at a fixed
gamma can follow the same recipe later if wanted.
