---
id: mes-z1rf
status: closed
deps: [mes-6mfk, mes-i21e]
links: []
created: 2026-10-07T16:34:16Z
type: task
priority: 3
assignee: Andre Silva
parent: mes-0jzi
tags: [bench, step-7, implementation]
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

## Notes

**2026-10-08T02:23:15Z**

Done. benchmark_test.go: `BenchmarkBetweenness` (karate, lesmis, lfr-S-mu030 = datasets/lfr/S/mu030-r0.txt via a new internal `loadLFREdgeList`, since the full LFR loader is in the external test package) and `BenchmarkSubgraph/karate-largest` (largest Leiden community, seed 1). Both report allocs.

Two gate gaps fixed on the way: the Makefile's `BENCH_RE` only matched Leiden/Louvain, so new benchmarks never ran under `make bench`; and `detectRegressions` iterates baseline names, so a benchmark missing from the baseline is silently uncompared. `BENCH_RE` now includes both new families; `TestBenchmarkBaselineValid` requires the four new names (it went red before the baseline regen).

Baseline regenerated with the documented container procedure (golang:1.27.1-bookworm, --cpus=4, linux/arm64): 14 benchmarks x 6 samples. Container figures: Betweenness karate 12.6us/9 allocs, lesmis 69us/9, lfr-S-mu030 99.6ms/11; Subgraph karate-largest 7.9us/108 allocs. `make bench-check` from the darwin host against it is green. `make validate` green; learnings recorded.
