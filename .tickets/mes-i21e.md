---
id: mes-i21e
status: closed
deps: [mes-dj0k]
links: []
created: 2026-10-07T16:34:15Z
type: feature
priority: 1
assignee: Andre Silva
parent: mes-0jzi
tags: [centrality, step-5, implementation]
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

## Notes

**2026-10-08T01:57:09Z**

Done. New centrality.go: `Betweenness(g) map[string]float64` and unexported `brandes(m, bc)`. Design notes for the next person:
- One scale for both directednesses: the raw Brandes sum counts every ordered pair, so undirected "halve then divide by (n-1)(n-2)/2" equals "divide by (n-1)(n-2)", the directed constant. n < 3 skips the computation (all zeros); n == 0 returns an empty non-nil map.
- No predecessor lists: in the accumulation, the predecessors of w are its in-neighbours v with dist[v] == dist[w]-1 (`csr.inNeighbors` falls back to out-neighbours on undirected graphs). The BFS queue slice doubles as the Brandes stack (read backwards). Scratch arrays are allocated once per call; the per-source loop does not allocate.
- Accumulation uses networkx's form `delta[v] += sigma[v] * (1+delta[w]) / sigma[w]` to keep float agreement with the mes-ar6b reference values tight.

Tests in centrality_test.go: closed forms (star 1/0, path 2i(n-1-i)/((n-1)(n-2)), complete 0 both directednesses, directed cycle 1/2: derivation each node's interior counts sum to (n-1)(n-2)/2, checked by hand); `bruteBetweenness` oracle from the definitional pair formula over distance and path-count matrices (independent of Brandes accumulation) on 400 random graphs n <= 12, undirected and directed, with self-loops, random weights and disconnected components, tolerance 1e-12; an injected bug (out- instead of in-neighbours) was confirmed caught. Degenerate sizes; weights, self-loops and node weights ignored; bit-identical across insertion orders under Canonical and across repeated calls; AllocsPerRun equal for a 60-node path and K60 (also under -race and -cover). Reusable helpers: `bruteBetweenness`, `randomHopGraph`, `nodeKey`, `approxEqual` (for mes-slkl). `make validate` green.
