---
id: mes-dj0k
status: closed
deps: [mes-rfgu]
links: []
created: 2026-10-07T16:34:15Z
type: task
priority: 1
assignee: Andre Silva
parent: mes-0jzi
tags: [builder, step-1, implementation]
---
# Builder drops zero-weight edges (v0.1.0 behaviour change)

Step 1 of `docs/specs/002-structural-measures/plan.md`, parent mes-0jzi. Make `Build` omit edges and arcs whose folded weight is zero, so every structural measure in this phase can equate "edge" with "entry in the adjacency". Decided with the plan owner on 2026-10-07 as a deliberate v0.1.0 behaviour change.

## Evidence (verified 2026-10-07)

- `AddEdge` in `builder.go` rejects only `!(weight >= 0)`; a zero weight is accepted and folded into `b.edges` (or `b.selfLoops` when `from == to`).
- `Build` copies every `b.edges` entry into the adjacency lists with no weight test, so a zero-weight edge becomes a neighbour entry.
- `TestBuilder_ZeroWeightsAccepted` in `builder_test.go` pins today's behaviour: "a zero-weight edge folds in as a zero-weight neighbour entry".
- The fuzz generator in `fuzz_test.go` already treats "a zero-weight pair is a non-edge" and emits strictly positive edge weights, so fuzz corpora are unaffected.
- No corpus GML has a zero edge weight (the `value 0` lines in `datasets/football/football.gml` are node conference labels).
- Self-loops live in a `[]float64`, so a zero-weight self-loop is already indistinguishable from none.

## Design

In `Build`, skip a `b.edges` entry when its folded weight is `0`. Weights are non-negative, so a folded total is zero only when every contribution was zero. Leave `AddEdge`'s endpoint registration untouched: both endpoints stay nodes, isolated if nothing else connects them. Update the godoc of `AddEdge` (and `Builder` if it describes folding) to state the rule. Negative and NaN weights stay errors.

## Warnings

- Zero node sizes (`AddNodeWeight(k, 0)`) are a different thing and must keep working exactly as today.
- Leiden's connectivity guarantee reads the adjacency, so a zero-weight bridge now separates components. That is the intended alignment with the fuzz generator's convention; record it in the close note.

## Acceptance Criteria

TDD order. 1) Rewrite `TestBuilder_ZeroWeightsAccepted`: a zero-weight edge builds without error, registers both endpoints, and leaves no neighbour entry; a zero-size node keeps size 0. 2) Directed: a zero-weight arc leaves neither an out- nor an in-entry; beside a positive reverse arc only the positive arc remains. 3) Parallel edges `0` and `w > 0` fold to one entry of weight `w`. 4) Golden corpus, oracle, determinism and benchmark suites unchanged. 5) Leiden and Louvain on two triangles joined by a zero-weight edge never put both triangles in one community. `make validate` green in both modules (fmt-check, vet, lint, test).

## Notes

**2026-10-08T01:48:28Z**

Done. `Build` (builder.go) skips any folded edge or arc whose total weight is zero; `AddEdge` still registers both endpoints, and its godoc states the rule. Negative/NaN still error; zero node sizes unchanged.

Tests (builder_test.go): `TestBuilder_ZeroWeightsAccepted` replaced by `TestBuilder_ZeroWeightEdgeDropped` (no neighbour entry, endpoints registered, zero-size node kept), `TestBuilder_ZeroWeightArcDropped` (no out- or in-entry; positive reverse arc survives), `TestBuilder_ZeroParallelFoldsIntoPositive`, and `TestBuilder_ZeroWeightBridgeSeparatesCommunities` (Leiden and Louvain).

Finding: the change does not affect the optimisers. Zero weights add no quality and `communityConnected` in refine.go already follows only `w > 0` edges. So the bridge test passes before and after (a guard, not a red-green proof); the adjacency tests are the discriminating ones. Golden, oracle, determinism and fuzz suites unchanged. `make validate` green. Learnings recorded in docs/specs/learnings.md.
