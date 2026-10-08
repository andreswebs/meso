---
id: mes-bctb
status: open
deps: [mes-0jzi]
links: []
created: 2026-10-07T16:34:16Z
type: feature
priority: 2
assignee: Andre Silva
tags: [gonum, awaits-core-tag, follow-up]
---
# gonum adapter: int64-keyed `Betweenness` and `Members` helpers after the core v0.2.0 tag

Waits on a manual event the tracker does not model: the core `v0.2.0` tag must exist (tag `awaits-core-tag`). The dependency edge on mes-0jzi covers only "the core API is complete". Deferred from step 6 of `docs/specs/002-structural-measures/plan.md` by the plan owner on 2026-10-07; see that plan's "Follow-up after the core tag".

## Why it waits

`gonum/go.mod` requires `github.com/andreswebs/meso v0.1.0` with no `replace`, and the repo has no `go.work`. Helpers calling the new core API cannot compile, and `make validate` fans out to every module, until the adapter can require the published `v0.2.0`.

## Design

In `github.com/andreswebs/meso/gonum` (`gonum/adapter.go`): bump the core `require` to `v0.2.0` and `make tidy`; add

```go
func Betweenness(g *meso.Graph) (map[int64]float64, error)
func Members(r *meso.Result, label int) ([]int64, error)
```

inverting the decimal key scheme exactly as the existing `Communities` does (`strconv.ParseInt(k, 10, 64)`, same error wording on a non-numeric key). `Members` keeps dense index order. The adapter's `Build` already returns `*meso.Graph`, so accessors and `Subgraph` need no mirror. Correct the stale header comment in `gonum/go.mod`, which still describes a local `replace` that no longer exists. Mention the helpers in the package doc.

## Acceptance Criteria

1) Re-verify the tag exists (`git tag -l v0.2.0`) before starting; stop if not. 2) Helpers agree with the core on a gonum-built karate graph. 3) A non-numeric-key graph returns the documented error. 4) `TestCoreDependencyGraphFreeOfGonum` still green. 5) `make validate` green in both modules (fmt-check, vet, lint, test). 6) The owner tags `gonum/v0.2.0` (user action, not done by the implementer).
