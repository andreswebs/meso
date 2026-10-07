---
id: mes-z1rf
status: open
deps: [mes-6mfk, mes-i21e]
links: []
created: 2026-10-07T16:34:16Z
type: task
priority: 3
assignee: Andre Silva
parent: mes-0jzi
tags: [bench, step-7]
---
# Benchmarks for `Betweenness` and `Subgraph`, baseline regenerated

Step 7 (benchmarks part) of `docs/specs/002-structural-measures/plan.md`, parent mes-0jzi. Put the new hot paths under the existing `benchstat` regression gate.

## Design

Add `BenchmarkBetweenness` sub-benchmarks on karate, lesmis and one LFR corpus graph (from `datasets/lfr`), and `BenchmarkSubgraph` on the largest karate Leiden community, in `benchmark_test.go` beside the existing corpus benchmarks. Report allocations. Then `make bench-baseline` so `benchmarks/baseline.txt` carries the new names; the baseline regenerated in mes-rfgu predates them.

## Warnings

- Regenerate the baseline on the same machine and toolchain as mes-rfgu's run, otherwise every existing entry shifts too. If the machine changed, say so in the close note.
- Check how `benchgate_test.go` treats a benchmark present in the run but absent from the baseline before relying on the gate.

## Acceptance Criteria

1) New benchmarks run under `make bench`. 2) `benchmarks/baseline.txt` contains them. 3) `make bench-check` green. 4) Close note records ns/op and allocs/op for each new entry. `make validate` green in both modules (fmt-check, vet, lint, test).
