---
id: mes-hret
status: closed
deps: [mes-f1b3, mes-edpr, mes-eqtz, mes-685m]
links: []
created: 2026-07-17T02:40:47Z
type: task
priority: 2
assignee: Andre Silva
parent: mes-dvte
tags: [verification, gobra, phase-d]
---
# Data-race freedom (Gobra)

A machine-checked Gobra proof that the parallel hot loop of `meso` is
data-race free and memory safe, wired into the repo so it is reproducible
locally and checked in CI. The only proof in the verification stack that
touches the shipped artifact. Design decisions were settled in a design
review on 2026-07-19 and are recorded below (R1 through R11, with R4
amended after the spike); implement to them, do not relitigate.

Preconditions, all met: the closure-lifting refactor landed in
`parallel.go` (`bestMovesRange`, `bestMovesWorker`, and a literal-free
`parallelBestMoves` exist; `mes-eqtz`), and the phase 0 spike (`mes-685m`,
closed) returned a GO verdict for the full fork-join proof, conditional on
one upstream verifier fix handled by the R4 amendment below. The spike's
findings notes on `mes-685m` are the operational reference for this
ticket: exact invocation, timings, verified and rejected idioms.

Background: `docs/research/gobra-setup-orientation.md` (tool orientation),
`docs/meso-design.md` section 7 (verification posture: Gobra is
best-effort, scoped to the parallel hot loops), and
`verification/lean/README.md` (status conventions of the Lean tier, which
this track mirrors).

## Current state (updated 2026-07-20, phases 1 through 3 done)

The proof is live: `make verify-gobra` feeds
`-i parallel.go verification/gobra/meso.gobra` to the pinned interim
image and reports 0 errors in about 18 seconds. The toy is dissolved.

- `parallel.go`: fully annotated. Verified: `parallelBestMoves`,
  `bestMovesWorker`, `bestMovesRange` (the fork-join, transcribed from
  the toy choreography with the predicate renamed `rangeAcc`), plus
  `applyRound`, `normalizeWorkers`, `isolationBase`. Trusted markers on
  `parallelRound` and `parallelLocalMoveToStable` (R2 as amended). No
  function literals: `localMover`/`parallelMover` live in `mover.go`,
  outside the verifier's input.
- `meso.gobra` (`package meso`): proof vocabulary (`csrMem`,
  `partitionMem`, `rangeAcc`, `redeem`) plus the trusted frontier
  (mirror types with the documented `inAdj` rename, `objective` narrowed
  to `moveDelta`, `moveImproveEps` as uninitialized `var`, pure bodiless
  `numNodes`, trusted bodiless `bestMove` footprint contract).
- `stubs/runtime/runtime.gobra`: trusted `GOMAXPROCS` stub
  (`ensures res >= 1`), resolved via the `-I` include path
  (`GOBRA_INCLUDES` in the Makefile).

## Spike results this ticket builds on

- The complete WaitGroup debt-and-token fork-join argument verifies end
  to end on the toy: per-chunk permission split at spawn
  (`Add`/`Start`/`GenerateTokenAndDebt`/`TokenById`), `PayDebt` plus
  `Done` in the worker, `SetWaitMode`/`Wait` at the join, then the
  `redeem` lemma reassembles full slice permissions. About 18 s
  wall-clock per run through Docker.
- `defer wg.Done()` verifies: the deferred precondition is checked at
  function exit, after the ghost `PayDebt` lines. No workaround needed.
- Variable-size chunking verifies without nonlinear-arithmetic trouble by
  tracking ghost bounds sequences (`los`/`his`) instead of
  `index * chunkSize` arithmetic. Reuse this pattern.
- Mixed input works: an annotated `.go` file plus a companion `.gobra`
  file of the same package, passed together via `-i`. Mount sources at
  `/work`, never over `/gobra` (the image keeps `gobra.jar` there).
- Float literals are rejected by the Gobra frontend (upstream issue 980).
  Validated workarounds: keep literal-bearing files out of the verifier's
  input entirely (the R3 architecture), or mark members `trusted` (also
  confirmed to suppress the error).
- Blocker with a confirmed fix: upstream Gobra crashes on any permission
  to a shared float64 location (`acc(&gain[i])` on `[]float64`), with
  `Logic error: cannot stringify type float64` from
  `Names.serializeType`. Confirmed on both the `latest` and `v25.02`
  images. A two-line patch (adding the missing `Float32T`/`Float64T`
  cases) was built and validated in the spike: minimal repro and full
  float64 toy both verify with 0 errors. Without the fix, `parallel.go`
  cannot even be encoded (`parallelRound` reads `gain[u]`), so the
  interim image below is a hard prerequisite for phases 2 onward.
- Gobra enforces termination measures on pure and ghost members by
  default: write `decreases` clauses, do not reach for
  `--disablePureFunctsTerminationRequirement`.

## Goal and non-goals

Non-goals:

- No functional-correctness proof in Gobra. Functional properties are the
  Lean tier's job; Gobra proves safety (races, memory, crashes) only.
- No termination proof of `parallelLocalMoveToStable`. Its termination
  argument rests on strict float improvement, out of reach for the SMT
  float encoding and already covered by the Lean model plus tests.
- No verification of the serial algorithm files, of `parallelRound`, of
  `parallelLocalMoveToStable`, or of the `gonum/` module (per R2). Note:
  those `parallel.go` members are still encoded by the verifier even
  though they are not verified, so their references must type-check (see
  phase 2).
- No behavior change to any shipped code beyond adding annotation
  comments.
- No code rewrites forced by verifier limitations (per R8).

## Resolved decisions

- R1 (annotation placement): inline `// @` comments in `parallel.go`,
  with package-level predicates in the companion
  `verification/gobra/meso.gobra`. The proof attaches to the shipped
  artifact. No mirror copy.
- R2 (proof scope): the fork-join only: `bestMovesRange`,
  `bestMovesWorker`, `parallelBestMoves`, plus the cheap sequential
  warm-ups (`applyRound`, `isolationBase`, `normalizeWorkers`). This is
  100 percent of the concurrency. `parallelRound` and
  `parallelLocalMoveToStable` stay unverified; amended 2026-07-20 after
  the phase 2 encoding probe: they carry a one-line `// @ trusted` marker
  rather than staying bare, because their bodies use `copy` on a defined
  slice type and float compound assignment, both rejected by the frontend
  even for unverified members. `trusted` skips the body entirely and
  suppresses both (probe-confirmed).
- R3 (objective interface): opaque behind the trusted frontier.
  `bestMove` gets a trusted footprint-only contract (reads `g`, `obj`,
  `p` immutably, writes nothing shared); the interface is never called
  from verified code, so no interface specs or conformance proofs. The
  spike validated the mechanism: the contract lives as a bodiless
  declaration in the companion `.gobra` file, and `louvain.go` (which
  holds the float-literal-bearing body) is never fed to the verifier.
- R4 (verifier toolchain), amended 2026-07-19 after the spike: the
  original decision (official `ghcr.io/viperproject/gobra` image, pinned
  by digest) is blocked by the shared-float64 crash. Until the upstream
  fix lands, the pin is an interim self-built image: fork
  `viperproject/gobra`, apply the two-line `Names.serializeType` patch
  validated in the spike, build an image `FROM` the official pinned base
  that replaces only `gobra.jar`, push it to a registry CI can pull, and
  pin that digest. Everything else stands: wrapped by a `verify-gobra`
  Make target, one Makefile variable holds the digest, contributors need
  only Docker. Exit condition: when the upstream fix reaches
  `ghcr.io/viperproject/gobra`, re-pin the official digest and re-run
  `make verify-gobra` as the canary. The upstream issue draft was
  prepared during the spike and is filed separately by the maintainer.
- R5 (CI lane and failure policy): nightly plus manual dispatch,
  non-blocking, while the proof stabilizes; promoted later per R6. Never
  part of `make validate`.
- R6 (promotion trigger): promote to an additional blocking,
  path-filtered PR job (paths: `parallel.go`, `verification/gobra/`)
  after 10 consecutive green nightlies with no timeout flakes and
  verification wall-clock under roughly 10 minutes.
- R7 (CI runner mechanism): CI runs the same `make verify-gobra` target
  and pinned image digest as local dev. No `gobra-action`: it bundles its
  own Gobra version, which would break local/CI symmetry. We forgo its
  cache and stats output, acceptable for a nightly lane; add our own
  caching later only if timings demand it.
- R8 (best-effort floor): if the WaitGroup fork-join proof is infeasible
  due to a verifier limitation, land whatever verifies (leaf contracts,
  the `bestMovesRange` footprint, sequential warm-ups), record the
  precise blocker in the assumption register and this ticket, and close
  as best-effort-complete. The spike has already de-risked the protocol
  itself; the residual risk is the real code's contracts, not the
  choreography.
- R9 (ticket mapping): implemented: `mes-685m` was the spike (closed,
  findings in its notes); this ticket holds the full plan.
- R10 (slice-range predicates): written in-repo in
  `verification/gobra/meso.gobra`: range-access predicates plus
  split/join reasoning for `[0, n)` into chunks. The spike's `chunk`
  predicate and `redeem` lemma are the seed. Small, self-contained,
  GPLv3-clean by construction. External projects' predicates are reading
  material only, not vendored.
- R11 (bookkeeping): cross-link only. All Gobra records live in
  `verification/gobra/README.md` (status table, assumption register);
  `verification/lean/README.md`'s D2 entry gets updated with a pointer.
  `CORRESPONDENCE.md` keeps its crisp theorem-to-test contract, a
  one-line pointer at most.

## Phases

Phase 0 (the spike) is done: `mes-685m`, closed 2026-07-19. Phases 1
through 3 are done and phase 4 is implemented pending its first green
scheduled run (2026-07-20); the summaries below record what was built
and the findings later phases build on. Remaining: phase 5
(documentation and bookkeeping) and phase 4's first green nightly.

### Phase 1: interim verifier image and scaffolding (done)

All landed: the interim image is built from the `andreswebs/gobra` fork
by its `publish-image` workflow and digest-pinned in the Makefile
(`GOBRA_IMAGE`); `make verify-gobra` wraps it (repo mounted at `/work`,
inputs in `GOBRA_INPUTS`, include path in `GOBRA_INCLUDES`) and is listed
in the agent-instructions Make table; `verification/gobra/README.md`
carries the status table, assumption register, and the R4 exit
condition; `stubs/runtime/runtime.gobra` declares trusted `GOMAXPROCS`
with `ensures res >= 1`, resolved via `-I` (probe-validated: the image
bundles no `runtime` stub, the contract proves `n >= 1` and rejects
`n >= 2`).

### Phase 2: leaf contracts, the trusted frontier (done)

Everything `parallel.go` references from files never fed to the verifier
is declared in `meso.gobra` (now `package meso`; the toy declarations
were split into `toy.gobra` to resolve the `bestMove` name conflict):
the real `bestMove` trusted footprint contract (wildcard reads of
`csrMem(g)` and `partitionMem(p)`, `0 <= u < len(p)`, no ensures); the
`csr`/`adjacency`/`Partition` mirrors with read predicates; `objective`
narrowed to `moveDelta`; `moveImproveEps` as an uninitialized `var`;
pure bodiless `numNodes` with `ensures 0 <= res`. All registered in the
README's assumption register. Exit test passed: feeding
`-i parallel.go meso.gobra` with every member trusted encodes with
0 errors, so every cross-file reference resolves and phase 3 is
contract-writing only. Probe findings folded into R2 (trusted markers)
and phase 3 below.

### Phase 3: the proof proper, bottom-up over the fork-join (done)

All four steps landed; `GOBRA_INPUTS` is now
`parallel.go verification/gobra/meso.gobra` and the toy (all four files
and its register row) is dissolved. The proof verified with 0 errors on
the first full run; a negative control (worker loop bound widened to
`u <= hi`) fails as it must, so the proof is live, not vacuous. Findings
beyond the plan:

- The chunk predicate is named `rangeAcc`, not `chunk`:
  `parallelBestMoves` has a local variable `chunk` that would shadow the
  predicate constructor inside the ghost annotations.
- `normalizeWorkers` carries the proof's single non-comment source
  change: its result is named (`res int`) so `ensures 1 <= res` can bind
  it. That bound plus the `GOMAXPROCS` stub discharges the
  division-by-zero check on the chunk-size computation. No behavior
  change.
- Contracts added beyond the toy's: `parallelBestMoves` requires
  `len(p) == g.numNodes()` (drives `hi <= len(p)` for `bestMove`'s
  bounds) and wildcard `csrMem`/`partitionMem` fractions thread through
  every fork-join contract and loop invariant.
- Range loops over slices (`for w := range p`, `for _, c := range p`)
  verify with ordinary invariants; no rewrite to indexed loops needed.
- `unfold acc(partitionMem(p), _)` (wildcard unfold) works and is how
  `isolationBase` reads elements while holding only a wildcard
  predicate fraction.

### Phase 4: CI (implemented, first green run pending)

Implemented 2026-07-20 as specified below; the lane goes live when the
workflow is committed and pushed, and the acceptance criterion (nightly
green per R5) is met by its first green scheduled run. Decisions were
settled in a design discussion on 2026-07-20. Context: the
repo's other workflows are parked in `.github/workflows.disabled/`; this
lane is deliberately enabled ahead of them and must stay self-contained.

1. `.github/workflows/gobra.yml` (name `gobra-nightly`): the repo's only
   active workflow, created directly in the live directory. The parked
   workflows stay parked; nothing in this lane depends on them.
2. Triggers per R5: `schedule` with cron `17 3 * * *` (daily 03:17 UTC;
   an off-peak minute, since GitHub delays or drops top-of-the-hour
   crons) plus `workflow_dispatch`. No push or PR triggers until the R6
   promotion.
3. One job on `ubuntu-latest` (amd64, so the image runs natively, no
   emulation): SHA-pinned `actions/checkout`, then exactly
   `make verify-gobra` (R7 symmetry: the digest pin and invocation live
   only in the Makefile). No setup steps; Docker is preinstalled and the
   interim image is a public ghcr package, pullable without auth.
4. `timeout-minutes: 10`, deliberately equal to the R6 promotion
   ceiling: a timeout is itself the not-promotable signal. Typical runs
   (~2-3 min including pull) keep 3x headroom. `permissions:
   contents: read`.
5. Failure signal: GitHub's built-in email for scheduled-run failures
   (goes to the workflow's last committer); the R6 streak of 10
   consecutive greens is read off the Actions run history. No issue
   automation, no badge, while the lane stabilizes.
6. The promotion trigger (R6) recorded in the workflow's comment header
   so the future promotion PR is mechanical.
7. Operational note: GitHub suspends cron triggers after 60 days of repo
   inactivity; if development pauses, the streak pauses silently with
   it.

### Phase 5: documentation and bookkeeping (done)

1. Done: `verification/gobra/README.md` status table all green;
   assumption register complete (four entries: `bestMove` footprint,
   `numNodes`, mirror-declaration fidelity, `GOMAXPROCS` stub).
2. Done: `verification/lean/README.md` concurrency-track status updated
   in all three places (intro, status paragraph, roadmap bullet),
   pointing at `verification/gobra/README.md` (R11).
   `CORRESPONDENCE.md` untouched: race-freedom is not a theorem-to-test
   mapping, and R11 allows at most a one-line pointer there.
3. Standing item that outlives this ticket: the upstream issue (drafted
   during the spike) is filed by the maintainer; once the fix reaches
   `ghcr.io/viperproject/gobra`, execute the R4 exit condition
   documented in the Makefile comment and
   `verification/gobra/README.md` (re-pin the official digest, run
   `make verify-gobra` as the canary, drop the fork).
4. Done: closed with the summary note on this ticket.

## Acceptance Criteria

- `make verify-gobra` passes locally from a clean checkout with only
  Docker installed, against the digest-pinned interim image (R4 as
  amended), with the exit condition to the official image documented.
- The Gobra run covers `bestMovesRange`, `bestMovesWorker`, and
  `parallelBestMoves` (plus warm-ups), establishing data-race freedom and
  memory safety of the fork-join under the registered trusted
  assumptions; or the R8 fallback is invoked with the blocker recorded.
- Nightly CI lane green per R5; every trusted contract listed in the
  assumption register.
- `make validate`, `make test-race`, and the golden suite untouched and
  green (annotations are comments; nothing runtime-visible changed).
- The toy scaffolding is gone (phase 3 step 4); `verification/gobra/`
  contains only the companion file, stubs, and README.
- Lean README D2 status updated; ticket closed with the summary note.

## Notes

**2026-07-20T14:01:50Z**

Closing summary.

PROVEN (machine-checked by Gobra, 0 errors, ~18 s/run): data-race freedom, memory safety, and crash safety (no panics, no out-of-bounds, no division by zero) of the complete fork-join in parallel.go - parallelBestMoves, bestMovesWorker, bestMovesRange - plus the sequential warm-ups applyRound, normalizeWorkers, isolationBase. The argument is the WaitGroup debt-and-token protocol: disjoint rangeAcc write chunks per worker, wildcard csrMem/partitionMem read fractions, redeem lemma reassembles full permissions at the join. Negative control confirmed the proof is live (off-by-one chunk write fails verification).

TRUSTED (all registered in verification/gobra/README.md): bestMove footprint contract (reads g/obj/p immutably, writes nothing shared, promises nothing about values - the single hot-path axiom); numNodes >= 0; runtime.GOMAXPROCS >= 1 (stubs/runtime/runtime.gobra); mirror-declaration fidelity in meso.gobra (documented inAdj rename). parallelRound and parallelLocalMoveToStable carry trusted markers per amended R2; localMover/parallelMover live in mover.go, outside the verifier input.

PINNED VERIFIER: ghcr.io/andreswebs/gobra@sha256:d3fc8fd7b6d8ae1cbd5adcc2d4f93e770ce2b3f3140a68af5bd8052a6f8d6f76 (interim fork image carrying the two-line Names.serializeType float fix; R4 as amended). Exit condition documented in Makefile and README: when the upstream fix ships, re-pin the official digest, run make verify-gobra as canary, drop the fork. Upstream issue draft prepared; filing is the maintainer's action.

CI: gobra-nightly workflow (.github/workflows/gobra.yml), daily 03:17 UTC + dispatch, timeout 10 min = R6 promotion ceiling, non-blocking. It is the repo's only active workflow (rest of CI parked). First green scheduled run pending push; a manual dispatch after push is the quick way to get it. R6 promotion (blocking path-filtered PR job after 10 consecutive green nightlies) is recorded in the workflow header.

The one non-comment source change of the whole proof: normalizeWorkers result named (res int) so ensures 1 <= res can bind it. Everything else is annotation comments. make validate, make test-race, make verify-gobra all green at close.
