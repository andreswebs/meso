---
id: mes-0jzi
status: open
deps: []
links: []
created: 2026-10-07T16:34:15Z
type: epic
priority: 1
assignee: Andre Silva
tags: [milestone, structural-measures, v0.2.0]
---
# v0.2.0: structural measures (spec 002)

Umbrella for meso v0.2.0, the structural-measures release: node betweenness, community cohesion, the canonical induced subgraph, and the `Graph`/`Result` accessors a consumer (graphomania spec 003, shipping in graphomania v0.0.3) needs to reconcile those answers with its own edge set. Plan of record: `docs/specs/002-structural-measures/plan.md`; design of record: `docs/meso-design.md` (gains a "Structural measures" section in step 8). All API changes are additive except the deliberate builder change in step 1.

## Children, in plan order

1. mes-rfgu - step 0: Go 1.27 toolchain, CI golangci-lint bump, bench baseline regenerated.
2. mes-dj0k - step 1: builder drops zero-weight edges.
3. mes-ojdd - step 2: `Graph` accessors.
4. mes-8x09 - step 3: `Result` accessors and `Cohesion`.
5. mes-6mfk - step 4: `Subgraph`.
6. mes-i21e - step 5: `Betweenness` (Brandes).
7. mes-ar6b - step 6: networkx reference CSVs.
8. mes-z1rf, mes-slkl, mes-5e63 - step 7 split three ways: benchmarks, fuzz, mutation.
9. mes-2ppe - step 8: design doc, package docs, correspondence divergence entry.

The dependency edges are the real ordering. `Betweenness` (mes-i21e) depends only on the builder change, so it can run in parallel with the accessor chain (mes-ojdd -> mes-8x09 -> mes-6mfk).

## Phase-wide policies

- Verification tier is empirical, decided with the plan owner on 2026-10-07: brute-force Go oracles on small graphs, closed forms, committed networkx references, determinism tests. No Lean obligation in this phase; mes-2ppe records the gap in the correspondence divergence register. A Lean track for these measures was sketched and deliberately not filed.
- Directed graphs are supported by every new symbol from the start, with the pinned semantics in the plan's "Pinned semantics" section. Tickets restate the rule they need.
- Zero-weight edges are dropped by the builder (owner ruling, 2026-10-07), so every new symbol reads "edge" as "entry in the adjacency".
- Every ticket closes with `make validate` green in both modules.

## Out of scope / handoffs

- Tagging `v0.2.0` is a user action after this umbrella closes; owner ruled no ticket for it (2026-10-07). graphomania's v0.0.3 gate closes when the core tag exists and graphomania pins it.
- The int64-keyed gonum helpers are mes-bctb, outside this umbrella and depending on it: they cannot compile until the core tag exists.
- A `WithParallelism` option for `Betweenness` and a weighted (Dijkstra) variant are not requested; both would be later additive changes.
