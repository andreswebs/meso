---
id: mes-1ekt
status: closed
deps: [mes-0isk]
links: []
created: 2026-07-14T03:40:29Z
type: task
priority: 1
assignee: Andre Silva
parent: mes-t76u
tags: [core, builder, step-1]
---

# Public Builder: keys to dense indices, edge and node-weight ingestion, folding

The public Builder that maps arbitrary caller keys to stable dense indices, accepts weighted edges and optional node weights, and folds self-loops and parallel edges into edge weights, producing a CSR graph. This is the public entry to the whole library (`docs/meso-design.md` section 3; step 1 of `docs/specs/001-initial-implementation/plan.md`).

## Design

`NewBuilder().AddEdge(keyA, keyB, w).AddNodeWeight(key, s).Build()` -> graph. Maintain a key->index map assigning dense indices in first-seen order; keep the reverse index->key slice for round-tripping community labels back to caller keys. Parallel edges (same unordered pair) sum their weights; a self-loop `AddEdge(k,k,w)` records w as node-internal weight (feeds `twoM` and the resolution term, not the neighbor list). Undirected by default; keep a directed build path that leaves in/out separable (foundation for M3 step 8). Validate at the boundary what the Lean model assumes structurally: `weight_nonneg`, `nodeSize_nonneg` (CORRESPONDENCE divergence register: Go enforces these with an error where Lean assumes them). Go conventions: functional-options-free simple builder, exported-symbol doc comments, return `(Graph, error)` on invalid input, no panics on caller data.

## Acceptance Criteria

TDD order. 1) `AddEdge` on new keys assigns stable dense indices; `Build` round-trips indices back to the original keys. 2) Two `AddEdge` calls on the same unordered pair sum into one edge weight. 3) A self-loop is recorded as node-internal weight, not as a neighbor entry, and is visible to `twoM`. 4) `AddNodeWeight` sets `nodeSize`; unset nodes default to `1.0`. 5) Negative edge weight and negative node weight each return an error from `Build` (boundary enforcement of `weight_nonneg` / `nodeSize_nonneg`). 6) Degenerate inputs build valid CSR: empty graph, single node, isolated nodes, a disconnected graph across two components. 7) A directed build keeps in-degree and out-degree separable. `make validate` green.

## Notes

**2026-07-17T20:16:23Z**

Implemented the public Builder in builder.go, producing an immutable Graph. NewBuilder() (undirected) / NewDirectedBuilder() (directed); fluent AddEdge/AddNodeWeight returning *Builder; Build() (*Graph, error). Keys are string (matches design sketch; generics rejected since NewBuilder() can't infer a type param). Graph exposes NumNodes/Directed/Key(i)/Index(key) for round-tripping community labels to caller keys.

Behaviours (all TDD, one test per AC in builder_test.go): stable first-seen dense indices + round-trip; parallel edges fold by canonical endpoint pair (unordered min,max undirected / ordered from,to directed) summing weights; self-loops -> selfLoops[i] (not neighbour list), visible to twoM; AddNodeWeight sets nodeSize and registers isolated nodes, default 1.0; negative/NaN edge or node weight rejected via deferred b.err surfaced at Build() using faithful !(w>=0) (negation of Lean weight_nonneg 0<=w, also catches NaN); degenerate shapes (empty/single/isolated/disconnected) build valid CSR; directed keeps in/out separable.

csr extended: added directed bool + in adjacency; checkInvariants validates in only when directed via shared adjacency.check helper (undirected out-only tests unaffected); added outDegree/inDegree (degree/twoM remain the undirected quantities). flattenAdjacency sorts neighbours by index => deterministic layout independent of map-iteration and insertion order. Retired the meso_test.go blank-import smoke test, replaced with an external public-API test. Updated doc.go with the Builder entry point.

Flipped section-1 representation rows in verification/lean/CORRESPONDENCE.md from planned to a new 'landed' status (Go symbol written + tested), per the mes-0isk note; algorithm/quality rows stay planned. make validate + make build + make test-race all green across both modules. Learnings updated. Unblocks mes-orqz (QualityFunction + modularity), mes-pgah (directed modularity).
