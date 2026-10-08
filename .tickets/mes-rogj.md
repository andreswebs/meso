---
id: mes-rogj
status: open
deps: []
links: []
created: 2026-10-08T03:49:46Z
type: feature
priority: 1
assignee: Andre Silva
parent: mes-0jzi
tags: [api, levels, step-9, implementation]
---
# Level hierarchy on Result: `NumLevels`, `Level`, `LevelQuality`

Step 9 of `docs/specs/002-structural-measures/plan.md`, parent `mes-0jzi`. Expose the multilevel hierarchy each run already computes. Decided with the plan owner on 2026-10-08: added before the `v0.2.0` tag, one ticket covering code and docs.

## API

```go
func (r *Result) NumLevels() int
func (r *Result) Level(i int) map[string]int
func (r *Result) LevelQuality(i int) float64
```

## Pinned semantics (owner rulings, 2026-10-08)

- `Level(i)` returns level `i`'s community label per caller key, labels dense; nil for `i` outside `[0, NumLevels)`. `LevelQuality(i)` is that level's quality under the run's objective; 0 out of range (documented).
- The last level equals `Communities()` and `LevelQuality(NumLevels()-1) == Quality()`.
- A level is the base-graph partition reported after that level's local moving: each `louvainLevel.base` for Louvain; for Leiden, the non-refined phase-1 partition lifted to the base graph (the `result` variable assigned once per loop iteration in `leidenWith`).
- Louvain levels nest (every level `i+1` community is a union of level `i` communities). Leiden levels are NOT promised to nest: after the first level, Leiden aggregates the refined sub-communities, so a coarser level can split a finer level's community. The godoc says so plainly.
- Under `WithIterations(k)`, report the final pass's levels only.
- Store levels internally as dense partitions plus qualities, built through `newResult`, so a later `LevelResult(i) *Result` can wrap one without an API break. `LevelResult` is NOT part of this ticket.
- Deterministic: byte-identical across repeated runs for a fixed seed and across `WithParallelism` worker counts.

## Evidence (verified 2026-10-08)

- `leidenWith` in `leiden.go` assigns `result = canonicalize(base)` once per loop iteration, before either `break`, so every iteration yields exactly one level and the last is the returned partition. `leidenIterated` calls `leidenWith` once per pass; the public `Leiden` (`api.go`) calls `leidenIterated` with the serial or parallel mover.
- Louvain: `louvainTraceWith(g, obj, mv)` in `louvain.go` returns `[]louvainLevel` whose `.base` is each level's base partition; `louvainLevels` is the test-facing projection; the last `.base` is the result.
- `newResult(g, p, quality)` in `api.go` builds `Result` and its eager `members`; both `Leiden` and `Louvain` call it.
- Tests keep their own copy of the Leiden loop: `leidenTrace` in `guarantees_test.go`, pinned to production by `TestLeiden_TraceMatchesRun` and used by the level guarantee tests (monotonicity, termination, which also need its per-level working-graph size).
- Undirected level monotonicity is already tested (`TestLeidenRun_QualityMonotoneAcrossLevels`, `TestLeiden_QualityMonotoneAcrossLevels`). No test asserts it for `DirectedModularity`.

## Warnings

- Do not change `leidenTrace` (owner ruling, 2026-10-08). Extend `TestLeiden_TraceMatchesRun` so the trace's level bases also equal the production levels.
- `WithIterations` default `k = 1` must stay byte-identical: golden, oracle, determinism and benchmark suites must not move.
- Directed monotonicity is asserted here for the first time. If it fails on a directed graph, stop and record the finding (a discussion note under `.local/tmp`); do not weaken the claim silently.
- Keep allocation modest: one stored partition per level is expected; do not add per-level maps until `Level(i)` is called.

## Steps

1. Re-read `leidenWith`, `leidenIterated`, `louvainTraceWith` and the public `Leiden`/`Louvain`; stop and report if the loop shape changed since 2026-10-08.
2. Test first (TDD, one behaviour at a time), then thread level recording through the serial and parallel paths of both algorithms, final pass only for Leiden.
3. Docs: `docs/meso-design.md` section 2 keeps hierarchical output in scope; section 3's sketch and API bullet gain the level accessors and drop the sentence saying the hierarchy is not yet public; section 4.6 gains the level semantics including the Leiden nesting caveat. Update `doc.go` and `README.md` where they list Result accessors; add an `Example` for the levels.

## Acceptance Criteria

TDD order. 1) Last level equals `Communities()` and `LevelQuality(last) == Quality()` for Leiden and Louvain, serial and parallel, on the corpus. 2) Every level is a well-formed dense partition over all keys; quality is non-decreasing along the levels for modularity, CPM and directed modularity (celegansneural via `loadGMLDirectedGraph`). 3) Louvain levels nest; Leiden levels are not asserted to nest. 4) Out-of-range `Level`/`LevelQuality` give nil/0. 5) `WithIterations(k > 1)`: levels are the final pass's (last level equals the result). 6) Levels byte-identical across repeated runs and across worker counts. 7) A graph whose first local move merges nothing reports one level equal to the all-singletons result. 8) `TestLeiden_TraceMatchesRun` also checks trace bases against production levels. 9) Design doc, package doc, README and an `Example` updated; `markdownlint-cli2` clean. `make validate` green; record learnings in `docs/specs/learnings.md` if any.
