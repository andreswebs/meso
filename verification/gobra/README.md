# Gobra verification

Machine-checked data-race freedom and memory safety of the parallel hot
loop, verified with [Gobra](https://github.com/viperproject/gobra), the
deductive verifier for Go from ETH Zurich. This is the only proof in the
verification stack that touches the shipped artifact.

## What is proven

`make verify-gobra` feeds the verifier the annotated `parallel.go` plus
the companion `meso.gobra` and establishes, for every verified member,
memory safety, crash safety (no panics, no out-of-bounds accesses, no
division by zero), and data-race freedom, modulo the trusted assumptions
registered below.

Verified members: `parallelBestMoves`, `bestMovesWorker`, and
`bestMovesRange` (the complete fork-join: 100 percent of the concurrency
in the library), plus the sequential warm-ups `applyRound`,
`normalizeWorkers`, and `isolationBase`. The race-freedom argument is the
WaitGroup debt-and-token protocol: at spawn, each worker goroutine
receives full write permission to exactly its disjoint slots `t[lo:hi]`
and `gain[lo:hi]` (the `rangeAcc` predicate) and wildcard read fractions
of the graph and snapshot partition (`csrMem`, `partitionMem`); at the
join, `wg.Wait` redeems each worker's paid-back `rangeAcc` token and the
`redeem` lemma reassembles full permission over both slices. Two writers
to the same slot are unrepresentable.

Not verified, by decision: `parallelRound` and
`parallelLocalMoveToStable` carry a one-line `trusted` marker (their
bodies also use `copy` on a defined slice type and float compound
assignment, which the frontend rejects even unverified), and `localMover`
and `parallelMover` live in `mover.go`, outside the verifier's input, so
the proof never sees a function literal. The serial algorithm and the
`gonum/` module are out of scope.

## What is here now

- `meso.gobra`: the companion specification file (`package meso`). The
  proof vocabulary: the `csrMem`/`partitionMem` read predicates, the
  `rangeAcc` per-worker write predicate, and the `redeem`
  permission-reassembly lemma. The trusted frontier: mirror declarations
  of `adjacency`, `csr`, `Partition`, the `objective` interface narrowed
  to `moveDelta`, `moveImproveEps` as an uninitialized `var`, the pure
  bodiless `numNodes`, and the trusted bodiless footprint contract of
  `bestMove`. The mirrors must stay type-identical to their Go
  definitions on everything `parallel.go` touches (drift is caught only
  by the Gobra run, not the Go compiler), with one documented divergence:
  the in-adjacency field is renamed `inAdj` because `in` is a reserved
  keyword of Gobra's specification syntax, which is safe because no file
  fed to the verifier accesses `csr` fields.
- `stubs/`: specification-only stand-ins for standard-library packages
  that Gobra's bundled stubs do not cover, organized one directory per
  package so imports resolve through the include path (the `-I` flag,
  held in the `GOBRA_INCLUDES` Makefile variable). Currently one file,
  `stubs/runtime/runtime.gobra`, declaring `GOMAXPROCS` with the
  postcondition `res >= 1`. Every declaration in `stubs/` is an axiom and
  has a row in the trusted assumption register below.

## The interim verifier image

Upstream Gobra crashes on any permission to a shared float64 location,
such as `acc(&gain[i])` on a `[]float64`:

```text
Logic error: cannot stringify type float64°
```

The crash reproduces on the official `latest` and `v25.02` images. Root
cause: `Names.serializeType` lacks cases for `Float32T`/`Float64T`,
although the float encoding otherwise supports shared floats. Without the
fix, `parallel.go` cannot even be encoded, so the proof runs against an
interim image built from a patched fork:

- Fork: <https://github.com/andreswebs/gobra>, carrying a two-line patch
  that adds the missing cases to
  `src/main/scala/viper/gobra/translator/Names.scala`.
- The image is built with the upstream recipe
  (`workflow-container/Dockerfile`, which runs `sbt assembly` in a build
  stage), targeting `linux/amd64` because the bundled Z3 and Boogie
  binaries are x64:

```sh
git submodule update --init --recursive
docker build --platform linux/amd64 \
  -f workflow-container/Dockerfile \
  -t "${IMAGE_REPO}:${IMAGE_TAG}" .
docker push "${IMAGE_REPO}:${IMAGE_TAG}"
```

The digest of the pushed image is pinned in the `GOBRA_IMAGE` variable in
the root Makefile and used identically by local runs and CI. The image is
built by the fork's `publish-image` workflow (manual dispatch), which
prints the digest to pin in its job summary.

Exit condition: the bug and patch are being reported upstream. Once the
fix reaches `ghcr.io/viperproject/gobra`, re-pin the official digest, run
`make verify-gobra` as the canary, and drop the fork.

## Running the verifier

The verifier runs in Docker; no JVM or Z3 is needed on the host. From the
project root:

```sh
make verify-gobra
```

The target mounts the repository at `/work` and feeds the verifier the
files listed in the `GOBRA_INPUTS` Makefile variable, with the
`GOBRA_INCLUDES` directories on the include path (`-I`) so imports of
locally stubbed packages resolve (the image entrypoint is
`java -Xss128m -jar gobra.jar` with workdir `/gobra`, so sources must not
be mounted over `/gobra`). The run exits nonzero on any verification
failure.

A full run of the proof takes roughly 20 seconds. On arm64 hosts Docker
prints a platform warning and runs the amd64 image under emulation; this
is harmless.

CI runs the identical target: the `gobra-nightly` workflow
(`.github/workflows/gobra.yml`) executes `make verify-gobra` every night
plus on manual dispatch, non-blocking while the proof stabilizes. The
promotion condition to a blocking PR job is recorded in the workflow's
header comment.

## Trusted assumption register

Every `trusted` or bodiless declaration the proof relies on, with its
justification. Additions to this list happen in the same change that
introduces the assumption.

| Assumption                                                                                | Where                         | Justification                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| ----------------------------------------------------------------------------------------- | ----------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bestMove(g *csr, obj objective, p Partition, u, isolateLabel int)` footprint contract    | `meso.gobra`                  | The real leaf of the fork-join. Its body (`louvain.go`) contains float literals and stays out of the verifier's input per the architecture. The contract grants only wildcard read access to `csrMem(g)` and `partitionMem(p)` plus `0 <= u < len(p)`, and promises nothing about the results: by inspection the implementation reads `g`, `obj`, and `p` immutably and writes nothing shared, which is all the race-freedom proof needs. The single trusted assumption on the hot path. |
| `numNodes()` pure bodiless, `ensures 0 <= res`                                            | `meso.gobra`                  | Mirrors `graph.go`: it returns `g.n`, the node count, never negative by construction. The bound discharges the `make([]int, n)` allocations in `parallelBestMoves`.                                                                                                                                                                                                                                                                                                                      |
| Mirror type declarations (`adjacency`, `csr`, `Partition`, `objective`, `moveImproveEps`) | `meso.gobra`                  | Not contracts but a trust point: they must stay type-identical to the Go definitions on everything `parallel.go` touches. Divergences: `csr`'s in-adjacency field is renamed `inAdj` (`in` is a reserved Gobra keyword), `objective` omits the embedded `QualityFunction`, and `moveImproveEps` drops its value (a float literal) - all safe because no file fed to the verifier touches those parts.                                                                                    |
| `runtime.GOMAXPROCS(n int) (res int)` with `ensures res >= 1`                             | `stubs/runtime/runtime.gobra` | Documented Go runtime behavior: with `n <= 0` the call reports the current setting without changing it, and the setting is never below 1. The bound is load-bearing: `normalizeWorkers` needs it to establish `workers >= 1`, which discharges the division-by-zero check on the chunk-size computation in `parallelBestMoves`.                                                                                                                                                          |
| `min(a int, b int) (res int)` pure bodiless, postconditions `a <= b ==> res == a` and `b <= a ==> res == b` | `meso.gobra`                  | Go's `min` builtin (go1.21), unknown to Gobra's frontend (`got unknown identifier min`). The companion declaration is what the identifier resolves to under verification; the Go compiler keeps resolving to the builtin. The postconditions are the builtin's complete two-argument semantics for `int`. Bodiless of necessity: a defining body cannot be written, because the conditional expression is ghost-only syntax and cannot produce a non-ghost result.                       |

## Verified idioms worth knowing

Findings the proof relies on, accumulated over the spike, the encoding
probe, and the proof itself:

- `defer wg.Done()` verifies: the deferred call's precondition is checked
  at function exit, after the ghost `PayDebt` statements.
- Variable-size chunking avoids nonlinear arithmetic by tracking ghost
  bounds sequences (`los`/`his`) instead of `index * chunkSize`.
- Ghost and pure functions must carry `decreases` termination measures;
  the `redeem` lemma shows the pattern.
- `gofmt`, `golangci-lint`, and `make validate` are indifferent to the
  `// @` annotations and the `/*@@@*/` shared-variable marker.
- The frontend rejects `copy` on defined slice types (`copy(q, p)` where
  both are `Partition`) and float compound assignment (`realized += ...`).
  Both occur only in `parallelRound` and `parallelLocalMoveToStable`,
  whose one-line `// @ trusted` marker skips the body entirely and
  suppresses both.
- `sync.WaitGroup` methods require a shared receiver: the declaration in
  `parallelBestMoves` needs the `/*@@@*/` marker
  (`var wg /*@@@*/ sync.WaitGroup`).
- Range loops over slices verify with ordinary invariants, in both the
  index form (`for w := range p`) and the value form
  (`for _, c := range p`).
- Wildcard predicate instances unfold: `unfold acc(partitionMem(p), _)`
  yields wildcard fractions of the elements while a positive fraction of
  the predicate instance remains, which is how `isolationBase` reads the
  partition it is handed read-only.
- A postcondition can only name a named result: `normalizeWorkers` has
  the one non-comment source change of the proof, its result named `res`
  so `ensures 1 <= res` can bind it (no behavior change).
- The go1.21 `min`/`max` builtins are unknown to the frontend
  (`got unknown identifier min`). A package-level declaration in the
  companion file captures the identifier under verification while the
  Go compiler keeps the builtin. It must be bodiless: the conditional
  expression (`a < b ? a : b`) is ghost-only syntax and cannot produce
  a non-ghost result, so the builtin's semantics go in postconditions
  instead of a body.
