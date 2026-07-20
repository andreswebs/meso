---
id: mes-685m
status: closed
deps: []
links: []
created: 2026-07-19T16:58:38Z
type: task
priority: 2
assignee: Andre Silva
parent: mes-dvte
tags: [verification, gobra, phase-d, spike]
---
# Gobra spike: toy fork-join proof through the pinned verifier (go/no-go)

Phase 0 of the Gobra verification plan (see mes-hret for the full plan; background: docs/research/gobra-setup-orientation.md). Surface verifier rejections before any real spec work and produce the go/no-go evidence for the fork-join proof. No dependency on mes-eqtz: the spike uses a synthetic example shaped like the post-refactor code, so it can run in parallel with the refactor. Touches no shipped code.

## Design

1. Pin the `ghcr.io/viperproject/gobra` Docker image by digest and stand up a minimal runner invocation; record the exact `docker run` command.
2. Write a toy fork-join shaped exactly like the post-refactor `parallelBestMoves`: goroutines writing disjoint chunks of two shared slices (one `[]int`, one `[]float64`), joined by `sync.WaitGroup`, exercising the shipped stub's ghost API (`Init`, `Add`, `Done`, `Wait`) end-to-end with per-chunk permission split at spawn and reassembly at `Wait`.
3. Probe every construct the real proof needs: `defer wg.Done()` against the stub, named return values, a `float64` slice as a pure write target (values never reasoned about), and passing `*sync.WaitGroup` to a named function.
4. Keep the toy example under `verification/gobra/spike/` so phase 1 can grow from it; primary deliverable is the findings note.

## Acceptance Criteria

- The toy fork-join verifies through the digest-pinned image, or every rejection is documented precisely (construct, error, workaround if any).
- Findings recorded as a ticket note: verifier version and image digest, exact invocation, wall-clock timings, rejected idioms.
- A clear go/no-go recommendation for the full fork-join proof; a no-go feeds the best-effort fallback (R8 in the mes-hret plan).
- No shipped source file modified.


## Notes

**2026-07-19T21:06:30Z**

Spike findings (2026-07-19)

Verdict: GO for the full fork-join proof, conditional on one upstream fix
(shared float64 encoding, details below). The complete WaitGroup
debt-and-token fork-join argument, in exactly the post-refactor shape of
parallelBestMoves, verifies end to end through the pinned verifier.

Pinned verifier

- Image: ghcr.io/viperproject/gobra@sha256:25e6c90c2f3f60ac6b51b23edd4d635dad13b96048ac7a4e9fc9ca607d72d967
  (tag latest, master build of 2026-07-16, Gobra 1.1-SNAPSHOT, linux/amd64).
- Runs fine under emulation on arm64 hosts (Docker prints a platform
  warning; harmless).
- Exact invocation (the image entrypoint is java -Xss128m -jar gobra.jar,
  with workdir /gobra, so do NOT mount over /gobra):
  docker run --rm -v "${SPIKE_DIR}:/work" ghcr.io/viperproject/gobra@sha256:25e6c90c2f3f60ac6b51b23edd4d635dad13b96048ac7a4e9fc9ca607d72d967 -i /work/bestmoves.go /work/spike.gobra

Toy example

- Lives in verification/gobra/spike/: bestmoves.go (annotated, verified),
  bestmove.go (opaque leaf with float literals, never fed to Gobra),
  spike.gobra (chunk predicate, reassembly lemma, trusted bodyless contract
  for bestMove), bestmoves_test.go (runtime sanity plus race-detector
  coverage of the same shape).
- Mirrors the real bestMovesRange / bestMovesWorker / parallelBestMoves,
  including defer wg.Done(), *sync.WaitGroup passed to a named function,
  named returns, make() in verified code, and disjoint chunks of []int and
  []float64.
- make validate and go test -race are green with the spike in the tree;
  gofmt and golangci-lint are indifferent to the annotations.

Wall-clock timings (docker on an arm64 laptop, amd64 emulation)

- Upstream parallel_sum.gobra example: about 18 s.
- Spike toy (int variant and, with patched jar, float64 variant): 17-18 s.
- Frontend-only failures return in about 5 s.

Verified idioms (all pass)

- sync.WaitGroup ghost protocol end to end: Init, Add(1, 1/2, PredTrue{}),
  Start, GenerateTokenAndDebt, fold TokenById, per-chunk UnitDebt handed to
  goroutines, PayDebt + Done in the worker, SetWaitMode, Wait, then a
  recursive ghost lemma redeeming InjEval tokens to reassemble full slice
  permissions.
- defer wg.Done(): verifies; the deferred call precondition is checked at
  function exit, after the ghost PayDebt lines.
- Ghost sequences (seq[int], seq[pred()]) as loop bookkeeping, ghost if,
  predicate constructors (chunk{t, gain, lo, hi}), conditional expressions
  in invariants.
- Mixed input: annotated .go file plus companion .gobra file of the same
  package passed together via -i.
- The /*@@@*/ shared-variable marker on var wg sync.WaitGroup survives
  gofmt.
- Variable-size chunking verifies without nonlinear-arithmetic pain by
  tracking ghost bounds sequences (los/his) instead of index arithmetic.

Rejected idioms and workarounds

1. Float literals are rejected by the frontend (ExprTyping: "floating point
   literals are not yet supported", tracked upstream as gobra issue 980).
   Workarounds, both validated: keep literal-bearing bodies in files never
   fed to the verifier and declare their contracts bodiless in the
   companion .gobra (this is the R3 architecture; the toy does exactly
   this), or mark the member trusted (confirmed: trusted suppresses the
   error).
2. BLOCKER, now with a confirmed fix: any permission to a shared float64
   location (acc(&gain[i]) on []float64) crashes the translator with
   "Logic error: cannot stringify type float64degree" (LogicException in
   Names.serializeType, reached from FieldsImpl.field). Crashes identically
   on latest master and v25.02. Not reported upstream yet. Root cause:
   Names.serializeType lacks Float32T/Float64T cases although
   FloatEncoding already maps shared floats to Ref. A two-line patch
   (adding those cases) was applied to master and built locally; with it,
   both a minimal repro and the full float64 toy verify with 0 errors.
   Issue draft ready for review before filing. Consequence for mes-hret:
   parallel.go cannot even be encoded until a fixed verifier is available
   (parallelRound reads gain[u]), so phase 3 needs either the upstream fix
   to land in the pinned image, or a temporary self-built patched image
   (which would amend decision R4).

Other operational notes for phase 1

- Gobra enforces termination measures on pure/ghost functions by default on
  current master; write decreases clauses (the toy does) rather than using
  --disablePureFunctsTerminationRequirement.
- Useful flags observed: --packageTimeout, --onlyFilesWithHeader (only
  verifies files with a "// +gobra" header; convenient once more packages
  exist), --parallelizeBranches.
- The image is amd64-only; CI on standard GitHub runners needs no special
  handling, local Apple Silicon runs use emulation.

**2026-07-19T21:27:17Z**

Post-spike promotion (2026-07-19): the spike directory is not committed
as-was. Its files were promoted from verification/gobra/spike/ to
verification/gobra/ and now seed the real verification tree. spike.gobra was
renamed to meso.gobra, its final name as the companion specification file
for package meso; it keeps "package spike" internally until the toy is
dissolved into the real proof (mes-hret phases 1-3). Re-verified after the
move: the int-variant toy passes through the pinned image with
-i /work/bestmoves.go /work/meso.gobra (0 errors), and
go test ./verification/gobra/ is green. Paths in the findings note above
predate the promotion.
