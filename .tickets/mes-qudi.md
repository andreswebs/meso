---
id: mes-qudi
status: closed
deps: [mes-jbc7, mes-hcvp]
links: []
created: 2026-07-14T03:40:30Z
type: task
priority: 3
assignee: Andre Silva
parent: mes-45a8
tags: [depth, benchmark, step-12]
---

# Benchmark suite + `benchstat` regression gate

A benchmark suite across small-to-large graphs versus Louvain and the reference runtimes, with allocation tracking and a CI regression guard via `benchstat`. Design of record: `docs/meso-design.md` sections 6.3 and 8; step 12 of `docs/specs/001-initial-implementation/plan.md`.

## Design

`testing.B` benchmarks for Leiden and Louvain across graph sizes; track allocations (`b.ReportAllocs`). `benchstat` comparison against a committed baseline gates regressions in CI. Reuse the corpus and generated graphs. Performance is prioritized after correctness/determinism (`docs/meso-design.md` section 4.5), so this gate guards against regressions, not for absolute speed targets.

## Acceptance Criteria

TDD/verify order. 1) Benchmarks run for Leiden and Louvain across small, medium, and large graphs and report ns/op and allocs/op. 2) A committed baseline exists and `benchstat` compares against it. 3) A deliberate slowdown is caught by the regression guard (self-test of the gate). 4) No hot-path allocation regressions on the core loops. `make validate` green.

## Notes

**2026-07-18T01:28:30Z**

Benchmark suite + regression gate implemented (step 12).

- benchmark_test.go: BenchmarkLeiden/BenchmarkLouvain over a small-to-large tier - corpus (karate/dolphins/lesmis) plus deterministic planted-partition graphs planted-300 and planted-800 built from the package PRNG. b.ReportAllocs() gives ns/op and allocs/op. Graph build is outside the timed loop. Sizes calibrated to this O(n^2)-ish implementation (planted-800 ~150ms is the practical ceiling; n>=1000 gets too slow for an on-demand gate).
- Committed baseline: benchmarks/baseline.txt (10x, count=6).
- Gate is a Go test (benchgate_test.go, package meso, test-only) - NOT a module dependency - because the core is deliberately dependency-free (empty go.mod, no go.sum). Parser + median summary + comparator live there. benchstat is still used for the human diff via 'make bench-compare' (go run pkg@version, isolated, never touches core go.mod - verified).
- Thresholds split by metric: ns/op default 100% headroom (cross-machine baseline vs CI), allocs/op default 0 (deterministic, machine-independent) - the real hot-path allocation guard (criterion 4). Override via MESO_BENCH_NS_FRACTION / MESO_BENCH_ALLOCS_FRACTION.
- make validate runs only the deterministic gate self-test (TestDetectRegressions: a slowed-down and a newly-allocating benchmark are flagged) + TestBenchmarkBaselineValid. The slow benchmarks run only under 'make bench'/'make bench-check'. TestBenchmarkRegressionGate skips unless MESO_BENCH_CURRENT is set.
- Make targets: bench, bench-baseline, bench-check, bench-compare (documented in AGENTS.md). bench-check wired into .github/workflows.disabled/ci.yml for when CI is re-enabled. new.txt gitignored + cleaned.
- Verified end-to-end: gate PASSes on unchanged run, FAILs on a doctored 3x-ns / +1-alloc run. make validate green. loadGMLGraph signature widened to testing.TB so benchmarks reuse it (safe for all *testing.T callers).
