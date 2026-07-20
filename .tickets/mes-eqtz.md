---
id: mes-eqtz
status: closed
deps: []
links: []
created: 2026-07-19T16:16:00Z
type: task
priority: 2
assignee: Andre Silva
parent: mes-dvte
tags: [verification, gobra, phase-d]
---
# Gobra pre-work: lift parallel closures into named functions

Audience: an AI agent implementing this change without prior context on the
project. Read this whole document before editing anything.

## Context and goal

`meso` is a pure-Go, deterministic community-detection library (Leiden and
Louvain). Ticket `mes-hret` will later verify data-race freedom of the
parallel hot loop with Gobra (`https://github.com/viperproject/gobra`), a
separation-logic verifier that reads Go files annotated with `// @` comments.
Background: `docs/research/gobra-setup-orientation.md` and section 7 of
`docs/meso-design.md`.

Gobra verifies function literals (closures) only through manually proved
specification entailments, which is expensive. The parallel hot loop in
`parallel.go` currently spawns goroutines through two nested function
literals. This refactor lifts them into named, package-level functions so the
later annotation work never has to specify a closure. That is the entire goal.

This is a behavior-preserving refactor. No public API change, no algorithmic
change, no new dependency, no annotation added yet. Every existing test must
pass unchanged, including the byte-identical determinism tests.

## Scope

In scope: `parallel.go` only, plus any doc-comment updates inside that file
that the refactor makes stale.

Out of scope, do not touch:

- Gobra annotations, stubs, or any `verification/gobra/` scaffolding.
- CI workflows, Makefile, `verification/lean/`, tickets, or design docs.
- The serial code paths (`louvain.go`, `refine.go`, `move.go`, etc.).
- `parallelRound`, `parallelLocalMoveToStable`, `applyRound`,
  `isolationBase`, `normalizeWorkers`: these contain no function literals and
  stay exactly as they are.
- Git: do not commit, branch, or stage anything. The user handles git.

## Current code (the only part that changes)

`parallelBestMoves` in `parallel.go` currently reads:

```go
func parallelBestMoves(g *csr, obj objective, p Partition, workers int) (t []int, gain []float64) {
 n := g.numNodes()
 t = make([]int, n)
 gain = make([]float64, n)
 if n == 0 {
  return t, gain
 }
 base := isolationBase(p, n)

 compute := func(lo, hi int) {
  for u := lo; u < hi; u++ {
   t[u], gain[u] = bestMove(g, obj, p, u, base+u)
  }
 }

 workers = normalizeWorkers(workers, n)
 if workers == 1 {
  compute(0, n)
  return t, gain
 }

 var wg sync.WaitGroup
 chunk := (n + workers - 1) / workers
 for lo := 0; lo < n; lo += chunk {
  hi := lo + chunk
  if hi > n {
   hi = n
  }
  wg.Add(1)
  go func(lo, hi int) {
   defer wg.Done()
   compute(lo, hi)
  }(lo, hi)
 }
 wg.Wait()
 return t, gain
}
```

There are two function literals: the `compute` closure (captures `t`, `gain`,
`g`, `obj`, `p`, `base`) and the goroutine body (captures `wg` and `compute`).
Both must become named functions whose entire state arrives through
parameters.

## Target design

Introduce two package-level functions in `parallel.go`, placed directly above
`parallelBestMoves`:

```go
func bestMovesRange(g *csr, obj objective, p Partition, t []int, gain []float64, base, lo, hi int) {
 for u := lo; u < hi; u++ {
  t[u], gain[u] = bestMove(g, obj, p, u, base+u)
 }
}

func bestMovesWorker(wg *sync.WaitGroup, g *csr, obj objective, p Partition, t []int, gain []float64, base, lo, hi int) {
 defer wg.Done()
 bestMovesRange(g, obj, p, t, gain, base, lo, hi)
}
```

Then `parallelBestMoves` becomes:

```go
func parallelBestMoves(g *csr, obj objective, p Partition, workers int) (t []int, gain []float64) {
 n := g.numNodes()
 t = make([]int, n)
 gain = make([]float64, n)
 if n == 0 {
  return t, gain
 }
 base := isolationBase(p, n)

 workers = normalizeWorkers(workers, n)
 if workers == 1 {
  bestMovesRange(g, obj, p, t, gain, base, 0, n)
  return t, gain
 }

 var wg sync.WaitGroup
 chunk := (n + workers - 1) / workers
 for lo := 0; lo < n; lo += chunk {
  hi := lo + chunk
  if hi > n {
   hi = n
  }
  wg.Add(1)
  go bestMovesWorker(&wg, g, obj, p, t, gain, base, lo, hi)
 }
 wg.Wait()
 return t, gain
}
```

Design constraints behind this shape, respect them exactly:

- All shared state reaches the workers as explicit parameters. No named
  function may read package-level mutable state.
- `bestMovesRange` writes only `t[lo:hi]` and `gain[lo:hi]` and reads `g`,
  `obj`, `p` immutably. That disjoint-write discipline is what the later
  Gobra proof will formalize; do not weaken it (for example, do not merge the
  two output slices into a struct that workers share, and do not add any
  write outside `[lo, hi)`).
- Keep `defer wg.Done()` in `bestMovesWorker`. Do not convert it to a direct
  call; the defer preserves current panic semantics and Gobra supports defer.
- The goroutine spawn must be a direct `go bestMovesWorker(...)` call with no
  wrapping literal.
- `wg` is passed as `*sync.WaitGroup`. Do not copy the WaitGroup by value.
- Keep the `workers == 1` fast path calling `bestMovesRange` synchronously,
  so the serial path stays goroutine-free.
- Do not reorder, rename, or restructure anything else in the function: the
  chunking arithmetic, `normalizeWorkers` call site, and early return for
  `n == 0` stay byte-for-byte as they are.

## Doc comments

The existing doc comment on `parallelBestMoves` describes the chunking, the
race-freedom argument, and the link to the Lean proof (`applyRound_perm`).
Keep that comment on `parallelBestMoves` and adjust wording only where it
refers to the removed closure structure.

Write short doc comments for the two new functions in the style of the file:
they explain why, not what. Suggested content, adapt freely to match the
surrounding voice:

- `bestMovesRange`: states the write footprint contract, that is, it computes
  the snapshot best move for every node in `[lo, hi)`, writing only
  `t[lo:hi]` and `gain[lo:hi]` and reading `g`, `obj`, `p` immutably, and
  notes this is the unit the Gobra data-race proof will specify (node `u`'s
  isolation candidate is `base+u`).
- `bestMovesWorker`: one goroutine's share of the fan-out; a named function
  rather than a literal so the later Gobra annotation never specifies a
  closure.

Follow the project comment rule: no comments narrating the edit or the
refactor session, only comments that help the next reader of the code.

## Step-by-step

1. Read `parallel.go` in full. Confirm the current code matches the "Current
   code" section above; if it has drifted, stop and report instead of
   guessing.
2. Apply the refactor exactly as specified in "Target design".
3. Update the doc comments per the "Doc comments" section.
4. Run `gofmt` via `make fmt`.
5. Run the validation suite below.

## Validation and acceptance criteria

All commands run from the project root. All must pass:

1. `make validate` (fmt-check, vet, golangci-lint, all tests in every
   module). Do not silence lint findings with `_ =`; fix them properly.
2. `make test-race` (race detector). The parallel tests in
   `parallel_test.go` are the point of this gate:
   `TestParallel_RoundCoreCountInvariant` sweeps worker counts 1, 2, 4, 8,
   GOMAXPROCS and asserts byte-identical results, and
   `TestParallel_WholeRunCoreCountInvariant` plus
   `TestParallel_PublicOptionCoreCountInvariant` assert whole-run identity.
   They must pass with zero diffs, which confirms the refactor is
   behavior-identical.
3. Golden tests (`golden_test.go`, run as part of the suite) must pass
   untouched: do not regenerate any golden or corpus fixture. If a golden
   test fails, the refactor changed behavior; fix the refactor, never the
   fixture.
4. No test file is modified. If a test seems to require changing, the
   refactor is wrong.

Optional sanity check, not a gate: `make bench-compare` to confirm no
meaningful benchmark movement (a named call in place of a closure call should
be noise; the committed baseline is not to be regenerated).

## Report back

When done, report: the diff summary of `parallel.go`, the outcome of
`make validate` and `make test-race`, and explicitly confirm that no other
file changed and nothing was committed or staged.


## Notes

**2026-07-19T20:47:12Z**

Refactor applied: compute closure lifted to bestMovesRange, goroutine literal to bestMovesWorker. make validate and make test-race pass, zero races, core-count-invariance tests byte-identical. Only parallel.go changed for the ticket; Makefile test targets made verbose (-v) separately at user request.
