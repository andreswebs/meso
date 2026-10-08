---
id: mes-slkl
status: open
deps: [mes-6mfk, mes-i21e]
links: []
created: 2026-10-07T16:34:16Z
type: task
priority: 2
assignee: Andre Silva
parent: mes-0jzi
tags: [fuzz, step-7, implementation]
---
# Fuzz targets `FuzzBetweenness` and `FuzzSubgraph`

Step 7 (fuzz part) of `docs/specs/002-structural-measures/plan.md`, parent mes-0jzi. Extend native fuzzing to the new measures.

## Design

In `fuzz_test.go`, reuse the existing byte-decoded builder operations (`genOp`, strictly positive edge weights) used by `FuzzLeidenLouvain`, `FuzzFoldingTwoM`, `FuzzDirected`.

- `FuzzBetweenness`: never panics; every value in `[0, 1]`; the same op stream applied in two insertion orders under `Canonical()` gives maps equal under `math.Float64bits`; equals the brute-force enumerator from mes-i21e's tests (tolerance 1e-12) when `n <= 10`. Undirected and directed.
- `FuzzSubgraph`: pick a member set from the fuzz bytes; induced weights match `g.Weight` for every member pair; `Keys()` ascending; builder invariants hold; unknown or duplicate keys give the documented sentinel errors.

Seed both corpora with the closed-form fixtures (star, path, complete, directed cycle).

## Acceptance Criteria

1) Both targets pass their seed corpus under `make test`. 2) A timed local run (for example `go test -fuzz=FuzzBetweenness -fuzztime=5m`, likewise for `FuzzSubgraph`) finds no failure; the close note records the durations run. `make validate` green in both modules (fmt-check, vet, lint, test).
