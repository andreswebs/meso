---
id: mes-8x09
status: closed
deps: [mes-ojdd]
links: []
created: 2026-10-07T16:34:15Z
type: feature
priority: 1
assignee: Andre Silva
parent: mes-0jzi
tags: [api, cohesion, step-3, implementation]
---
# Result accessors and `Cohesion`: `NumCommunities`, `Members`, `Cohesion`

Step 3 of `docs/specs/002-structural-measures/plan.md`, parent mes-0jzi. Community accessors and the cohesion measure graphomania uses to decide re-splits (it re-splits communities of 50 or more members whose cohesion is below 0.05).

## API

```go
func (r *Result) NumCommunities() int
func (r *Result) Members(label int) []string
func (r *Result) Cohesion(label int) float64
```

## Pinned semantics (owner-approved, 2026-10-07)

- Labels are dense in `[0, NumCommunities)`.
- `Members(label)` returns member keys in dense index order; nil for a label out of range.
- `Cohesion(label)`: distinct edges with both endpoints in the community, self-loops excluded, divided by `k(k-1)/2` for `k` members (undirected), or distinct arcs divided by `k(k-1)` (directed; equals the undirected value on a symmetric graph). Fewer than two members or an unknown label gives 0. Counts edges, never weights.

## Evidence (verified 2026-10-07)

- `Result` in `api.go` holds `g *Graph`, `part Partition`, `quality float64`; `Communities()` documents labels as dense in `[0, number of communities)`, and `canonicalize` in `louvain.go` relabels to dense indices.
- `Keys()` arrives in mes-ojdd; this ticket's tests use it.

## Warnings

- A `*Result` may be shared across goroutines. If `Members` caches per-label lists, guard the cache with `sync.Once` (or build it eagerly in the constructor) so `make test-race` stays clean; a bare lazy field is a data race.
- Return a copy from `Members` so callers cannot corrupt the cache.

## Acceptance Criteria

TDD order. 1) `NumCommunities()` equals the distinct-label count of `Communities()` on every corpus result. 2) `Members` over all labels partitions `Keys()` exactly, each list in dense order; out-of-range gives nil. 3) `Cohesion` closed forms: clique 1, no internal edges 0, singleton 0, 4 members with 3 internal edges 0.5, directed form on a symmetric graph equals undirected, a lone arc in a 2-member directed community gives 0.5. 4) Weight-invariant: unit and random weights give identical cohesion. 5) Self-loop-invariant. 6) Brute force on small random graphs and partitions matches a direct pair count. 7) A concurrent-readers test under `make test-race`. `make validate` green in both modules (fmt-check, vet, lint, test).

## Notes

**2026-10-08T01:53:00Z**

Done. `Result` (api.go) gains a `members [][]int` field built eagerly by the new `newResult(g, p, quality)` constructor, which `Leiden` and `Louvain` now use. Eager construction (one O(n) pass, negligible next to the run) was chosen over a lazy cache so a shared `*Result` needs no `sync.Once` and stays race-free.

New result_accessors.go: `NumCommunities()` (len of members), `Members(label)` (keys in dense order, copied; nil out of range), `Cohesion(label)` (count out-adjacency entries of members whose target shares the label, divide by k(k-1); undirected edges appear in both lists so the doubled count matches k(k-1) = 2 * k(k-1)/2; directed arcs appear once against k(k-1) ordered pairs; 0 for k < 2 or unknown label).

Tests in result_accessors_test.go: Members partition Keys() in dense order on the five undirected corpus graphs; out-of-range and copy semantics; Cohesion closed forms (clique 1, path of 4 = 0.5, 0, singleton, directed symmetric = 2/3, lone arc 0.5, unknown labels); brute force over 200 random undirected/directed graphs and partitions; weight and self-loop invariance; 8 concurrent readers under -race. Fixture helpers `resultFor(t, g, labels)` and `randomCohesionFixture` are reusable by mes-6mfk. Gotcha hit: a fixture whose weight function consumes PRNG draws changes the topology between variants; the fixture now always takes the draw. `make validate` green.
