---
id: mes-ojdd
status: closed
deps: [mes-dj0k]
links: []
created: 2026-10-07T16:34:15Z
type: feature
priority: 1
assignee: Andre Silva
parent: mes-0jzi
tags: [api, accessors, step-2, implementation]
---
# Graph accessors: `Keys`, `NumEdges`, `Degree`, `Neighbors`, `Weight`

Step 2 of `docs/specs/002-structural-measures/plan.md`, parent mes-0jzi. Expose the graph structure graphomania otherwise re-derives from its own edge set and then has to prove equal.

## API

```go
func (g *Graph) Keys() []string
func (g *Graph) NumEdges() int
func (g *Graph) Degree(key string) (int, bool)
func (g *Graph) Neighbors(key string) []string
func (g *Graph) Weight(a, b string) (float64, bool)
```

## Pinned semantics (owner-approved, 2026-10-07)

- `Keys()` returns every key in dense index order, as a copy. Under `Canonical()` that is ascending key order.
- `NumEdges()` counts distinct unordered pairs (undirected) or distinct arcs (directed), self-loops excluded.
- `Degree(key)` counts distinct neighbours, self excluded. On a directed graph it is the size of the union of out- and in-neighbours, so a node reachable both ways counts once, and `len(Neighbors(k)) == Degree(k)` always holds. Absent key: `(0, false)`.
- `Neighbors(key)` returns distinct neighbours in dense index order, self excluded; nil for an absent key. Directed: the sorted union of out- and in-neighbours.
- `Weight(a, b)` returns the folded weight and `true` when the edge exists. Directed: it follows the arc `a -> b`, so `Weight(a, b)` and `Weight(b, a)` may differ. `Weight(a, a)` returns the folded self-loop weight and `true` when it is positive. Absent key: `(0, false)`.
- After mes-dj0k every adjacency entry has positive weight, so existence is adjacency membership.

## Evidence (verified 2026-10-07)

- `Graph` in `builder.go` wraps `model *csr`, `keys []string`, `index map[string]int`; `NumNodes`, `Directed`, `Key`, `Index` already exist.
- `csr` in `graph.go`: `neighbors(i)` and `neighborWeights(i)` view the out-adjacency; `inNeighbors(i)` views the in-adjacency on a directed graph (and falls back to out on undirected); `weight(i, j)` returns `selfLoops[i]` for `i == j` and otherwise linearly scans `neighbors(i)`; `selfLoops []float64` holds self-loops out of the neighbour lists.
- `flattenAdjacency` sorts every list by dense index, so out- and in-lists are sorted and a directed union is a linear merge.
- Undirected `NumEdges` is `len(out.neighbors) / 2`; directed is `len(out.neighbors)`.

## Warnings

- `Weight` must not report `(0, true)` for a missing pair: test membership, not the value `csr.weight` returns (it returns 0 for absent).
- The neighbour lists are sorted, so a binary search in `Weight` is an easy win; it is optional.

## Acceptance Criteria

TDD order. 1) `Keys()` is dense order, equals ascending key order under `Canonical()`, and mutating it does not affect the graph. 2) `NumEdges()` on fixtures: parallel edges fold to one, self-loops excluded, directed reciprocal arcs count two. 3) `len(Neighbors(k)) == Degree(k)` on every corpus graph; directed union counts a two-way neighbour once; absent key gives `(0, false)` and nil. 4) `Weight` round-trips every folded edge of a fixture, is asymmetric on a directed fixture, reports a positive self-loop for `(k, k)`, and is absent for a non-edge and an unknown key. 5) Reconciliation over every corpus GML loaded with `loadGMLGraph` (`golden_test.go`): sum of `Degree` over `Keys()` equals `2 * NumEdges()` on undirected graphs, and `NumEdges()` equals the folded GML edge count. `make validate` green in both modules (fmt-check, vet, lint, test).

## Notes

**2026-10-08T01:50:49Z**

Done. New accessors.go: `Keys()` (clone of the dense key slice), `NumEdges()` (len of out-adjacency, halved when undirected), `Degree(key)`, `Neighbors(key)` (undirected: CSR view mapped to keys; directed: linear merge of the sorted out- and in-lists with dedup, via the unexported `neighborIndices`), `Weight(a, b)` (binary search in the sorted out-list; self-loop present iff positive; membership decides existence, never the value).

Tests in accessors_test.go: table tests per accessor (dense vs canonical order, copy semantics, folding, self-loop exclusion, directed reciprocal arcs, directed union, absent keys, zero self-loop absent) and `TestGraph_AccessorsReconcileWithCorpus` over the five undirected corpus GMLs (NumEdges equals folded GML pairs, every folded pair round-trips through Weight, len(Neighbors) == Degree, degree sum == 2*NumEdges). celegansneural is excluded because `loadGMLGraph` builds undirected.

Refactor for the next tickets: `golden_test.go` now has `parseGML(t, path) (nodes, edges []gmlEdge)`; `loadGMLGraph` is a thin builder over it. mes-ar6b can build a directed loader from `parseGML`. Helper `mustBuild(t, *Builder)` lives in accessors_test.go. Note: directed `Degree` allocates the merged slice; fine for graphomania's sizes, revisit if it shows in a profile. `make validate` green.
