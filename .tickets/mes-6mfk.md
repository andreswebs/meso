---
id: mes-6mfk
status: open
deps: [mes-ojdd, mes-8x09]
links: []
created: 2026-10-07T16:34:15Z
type: feature
priority: 1
assignee: Andre Silva
parent: mes-0jzi
tags: [api, subgraph, step-4, implementation]
---
# `Subgraph`: canonical induced subgraph with `ErrUnknownKey` and `ErrDuplicateKey`

Step 4 of `docs/specs/002-structural-measures/plan.md`, parent mes-0jzi. The induced subgraph graphomania feeds to a second Leiden pass on an oversize or low-cohesion community; it must be canonical so that pass is a pure function of the member set.

## API

```go
var ErrUnknownKey = errors.New(...)
var ErrDuplicateKey = errors.New(...)
func Subgraph(g *Graph, keys []string) (*Graph, error)
```

## Pinned semantics (owner-approved, 2026-10-07)

- Result: those nodes, every edge or arc between them with its folded weight, their self-loops, their node weights, same directedness as `g`.
- Always canonically indexed, whatever `g`'s indexing.
- Unknown key: error wrapping `ErrUnknownKey`, message names the key. Key listed twice: error wrapping `ErrDuplicateKey`, message names the key. Empty list: an empty graph, not an error.

## Design

Feed a `Canonical()` builder of the same directedness (`NewBuilder` or `NewDirectedBuilder`): register every member with its node size via `AddNodeWeight`, then for each member in dense order add every edge or arc to another member (undirected: only `j > i`, so each pair is added once, because `AddEdge` folds both directions into one key), and its self-loop via `AddEdge(k, k, w)` when positive. Going through the public builder gives every builder invariant and validation for free.

## Evidence (verified 2026-10-07)

- `AddNodeWeight` sets `b.sizes[...]`; nodes not given a size default to 1.0 in `Build`, so always copy the size explicitly.
- Undirected `AddEdge` canonicalises the pair through `edgeKey` and sums weights, so adding both directions would double the weight.
- Accessors from mes-ojdd (`Keys`, `Weight`, `NumEdges`) and `Members` from mes-8x09 are used by the tests.

## Acceptance Criteria

TDD order. 1) For every member pair `Weight` agrees between `g` and the subgraph; no non-member key is present; `NumEdges` equals the count of `g`'s edges inside the set. 2) Self-loops, node weights, directedness and arc direction survive. 3) A non-canonical `g` built in two insertion orders gives subgraphs with identical `Keys()` and identical Leiden output under a fixed seed. 4) Unknown key wraps `ErrUnknownKey`; duplicate wraps `ErrDuplicateKey`; empty list builds an empty graph. 5) `Subgraph(g, g.Keys())` on a canonical `g` matches `g` in keys, edges, weights, and Leiden output. 6) On karate, the largest Leiden community's subgraph re-runs through Leiden with dense labels and every inner community connected. `make validate` green in both modules (fmt-check, vet, lint, test).
