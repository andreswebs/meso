---
id: mes-w60v
status: closed
deps: [mes-orqz, mes-qvav]
links: []
created: 2026-07-14T03:40:30Z
type: task
priority: 1
assignee: Andre Silva
parent: mes-t76u
tags: [core, aggregation, step-4, verified]
---

# Aggregation machinery: super-nodes, preserved self-loops and node sizes

Build the aggregate graph whose nodes are communities: fold each community into a super-node, carry internal edge weight into the super-node self-loop and summed node sizes into its node size, relabel communities to dense indices. Shared by Louvain and Leiden. Design of record: `docs/meso-design.md` section 4.1; step 4 of `docs/specs/001-initial-implementation/plan.md`.

## Design

Model `verification/lean/Meso/Aggregate.lean`: `aggregate G p` over `Fin (numComm p)`, with `aggregate_degree` (super-node degree = community degree), `aggregate_twoM` (total weight preserved), and the graph-to-graph invariance `modularity_aggregate_eq` / `cpm_aggregate_eq` (scoring the aggregate singleton partition equals scoring `p` on `G`). The `commLabel` bijection is the community-to-dense-index relabel. Crucially, internal community weight must land on the super-node self-loop and node sizes must sum, or the resolution term breaks silently (`docs/meso-design.md` section 4.1 correctness-critical subtleties). CORRESPONDENCE rows: `modularity_aggregate_eq` -> `TestAggregate_QualityPreserved`; `cpm_aggregate_eq` -> `TestAggregate_CPMPreserved`; `modularity_eq_communitySum` -> `TestAggregate_ModularityCommunitySum`.

## Acceptance Criteria

TDD order. 1) `TestAggregate_ModularityCommunitySum`: modularity equals the sum of within-community block contributions (Go image of `modularity_eq_communitySum`). 2) Aggregation preserves total edge weight (`aggregate twoM == original twoM`) and community count relabels to dense `[0,k)`. 3) `TestAggregate_QualityPreserved`: modularity of the aggregate singleton partition equals modularity of `p` on `G` (`modularity_aggregate_eq`). 4) `TestAggregate_CPMPreserved`: same for CPM (`cpm_aggregate_eq`). 5) Self-loop internal weight and summed node sizes survive into the super-node (a fixture with non-unit sizes and internal weight keeps the resolution term correct). 6) Round-trip: expanding an aggregate partition to the base graph agrees with the community assignment. `make validate` green.

## Notes

**2026-07-17T20:43:13Z**

Implemented undirected aggregation in aggregate.go: aggregate(g, p) -> (*csr, superOf) folds each community into a super-node (internal weight on the self-loop via a single incident-arc pass that visits internal edges twice = blockWeight A A; members' self-loops folded once; node sizes summed), relabels communities canonically by ascending label to dense [0,k). expand(superOf, aggP) lifts an aggregate partition back to the base graph. Tests (aggregate_test.go): ModularityCommunitySum (modularity_eq_communitySum), TwoMAndDenseLabels (aggregate_twoM + dense onto [0,k)), SelfLoopAndSizesPreserved (hand fixture with non-unit sizes + self-loop + internal weight, exact values), QualityPreserved (modularity_aggregate_eq) and CPMPreserved (cpm_aggregate_eq) on fixture + 3000 random trials each, RoundTrip (general level-composition Quality(agg,aggP)==Quality(g,expand) for both objectives, plus singleton recovers p). make validate + test-race green. Directed aggregation deferred to M3 (mes-28rk). Unblocks Louvain (mes-hcvp) and Leiden assembly (mes-jbc7).
