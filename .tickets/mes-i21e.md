---
id: mes-i21e
status: open
deps: [mes-dj0k]
links: []
created: 2026-10-07T16:34:15Z
type: feature
priority: 1
assignee: Andre Silva
parent: mes-0jzi
tags: [centrality, step-5]
---
# `Betweenness`: Brandes over unweighted shortest paths, undirected and directed

Step 5 of `docs/specs/002-structural-measures/plan.md`, parent mes-0jzi. Node betweenness centrality, graphomania's top-priority request.

## API

```go
func Betweenness(g *Graph) map[string]float64
```

No options in v0.2.0 (owner ruling, 2026-10-07); a `WithParallelism` variant would be a later additive change.

## Pinned semantics (owner-approved, 2026-10-07)

- Brandes (2001) over unweighted shortest paths: each edge is one hop; edge weights, node weights and self-loops are ignored. Path counts are `float64`, as in networkx, so large graphs cannot overflow.
- Undirected: each unordered pair counts once; halve the raw dependency sum, then divide by `(n-1)(n-2)/2`. Directed: paths follow out-arcs; divide the raw sum by `(n-1)(n-2)`. Values lie in `[0, 1]`.
- `n < 3`: zero for every node. `n == 0`: an empty, non-nil map.
- Determinism: sources in ascending dense index; neighbours in adjacency order (already sorted by dense index via `flattenAdjacency`); dependency accumulation over the Brandes stack in reverse discovery order; no map iteration on the hot path. Byte-identical across runs and machines.

## Design

New file `centrality.go`. Allocate scratch arrays (distance, sigma, delta, predecessor lists, BFS queue, stack) once per call and reset per source. Use `csr.neighbors(i)` (out-arcs on a directed graph). Zero-weight edges are already gone after mes-dj0k, so every adjacency entry is a hop. A brute-force all-shortest-paths enumerator lives in the test file and is reused by mes-slkl.

## Acceptance Criteria

TDD order. 1) Brute-force oracle on random undirected and directed graphs up to 12 nodes (including disconnected graphs and self-loops), tolerance 1e-12. 2) Closed forms: star centre 1.0 and leaves 0; path of `n` nodes gives node `i` (0-based) `2 i (n-1-i) / ((n-1)(n-2))`; complete graph 0 everywhere; directed cycle of `n >= 3` nodes gives every node exactly 1/2 (derived in planning, not published: check with the `math` skill before committing the test). 3) Degenerate: 0, 1, 2 nodes; reweighting, self-loops and node weights change nothing. 4) Determinism: two insertion orders under `Canonical()` give maps equal under `math.Float64bits`; repeated calls bit-identical. 5) Allocations scale with `n`, not `n * m` (benchmark-backed check). `make validate` green in both modules (fmt-check, vet, lint, test).
