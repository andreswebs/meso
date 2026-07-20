---
id: mes-b0id
status: closed
deps: [mes-jbc7]
links: []
created: 2026-07-14T03:40:30Z
type: task
priority: 3
assignee: Andre Silva
parent: mes-45a8
tags: [depth, gonum, step-10]
---

# gonum adapter (nested module)

The optional `github.com/andreswebs/meso/gonum` adapter that converts gonum graphs to a meso build and back, kept in its own module so the core stays dependency-free. Slots in once the public `Builder`/`Partition` API is stable. Design of record: `docs/meso-design.md` sections 3 and 9; step 10 of `docs/specs/001-initial-implementation/plan.md`.

## Design

Implement in the existing `gonum/` module (`gonum/doc.go` stub today). Convert a gonum graph (`gonum.org/v1/gonum/graph`) into a meso `Builder` and map a meso `Partition` back onto gonum node IDs. Add the gonum dependency to `gonum/go.mod` only; assert the core module dependency graph stays free of gonum. Retire the gonum smoke test from step 0. Go conventions: adapter package, small surface, doc comments.

## Acceptance Criteria

TDD order. 1) A gonum graph converts to a meso build and back with edges and weights preserved. 2) The adapter produces the same partition as the core API on shared fixtures. 3) The core module (`.`) dependency graph is asserted free of gonum (`go list` / `go mod graph` check). 4) The gonum module builds and tests green under `make validate`. `make validate` green.

## Notes

**2026-07-18T00:53:28Z**

Implemented the gonum adapter in gonum/adapter.go: Build(graph.Graph) (*meso.Graph, error) converts a gonum graph to a Canonical() meso build (directed when the graph implements graph.Directed, undirected otherwise; weights from graph.Weighted or graph.WeightedEdge, default 1; isolated nodes preserved via AddNodeWeight); Communities(*meso.Result) (map[int64]int, error) inverts the key scheme (meso key = decimal string of the int64 node ID). Undirected edges are de-duplicated by only adding the from<=to direction, since From reports a neighbour from both endpoints and undirected AddEdge folds weights. gonum/go.mod now requires gonum v0.17.0 + a local replace github.com/andreswebs/meso => ../ (core is unpublished). Core stays dependency-free: TestCoreDependencyGraphFreeOfGonum (dependency_test.go) runs 'go list -deps' over ./... and asserts no gonum.org/ package. Retired the step-0 smoke test (removed gonum/doc.go and gonum/gonum_test.go; package doc now lives on adapter.go). make validate green in both modules; race clean.
