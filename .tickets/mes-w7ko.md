---
id: mes-w7ko
status: closed
deps: [mes-1lg6, mes-z0pd]
links: []
created: 2026-07-14T03:40:30Z
type: task
priority: 1
assignee: Andre Silva
parent: mes-mklx
tags: [leiden, refinement, connectivity, step-6, verified]
---

# Leiden refinement: singletons, randomized gain-weighted merges, well-connectedness gate

The refinement phase: within each phase-1 community, restart every node as a singleton and merge by a randomized, quality-gain-weighted choice, merging only singletons and only into sub-communities well-connected to the rest of their community. This is what guarantees connected communities and lifts modularity above Louvain. Design of record: `docs/meso-design.md` section 4.1 phase 2; step 6 of `docs/specs/001-initial-implementation/plan.md`.

## Design

Per-node refinement randomness from `hash(globalSeed, nodeID)` (`DET`) so the outcome is scheduling-independent. The well-connectedness gate maps to the Lean gated merge: `verification/lean/Meso/Refinement.lean` (`RefineStep`, `refineRun_isMergeRun`), `Meso/Connectivity.lean` (`CommunityConnected`), `Meso/GammaConnectivity.lean` (`GammaMergeStep` = shared edge + gamma-dense cut, `gammaDense_union`). Only merge across a shared positive-weight edge (connectivity) into a gamma-dense sub-community (density). CORRESPONDENCE rows: `connectedCommunities_singleton` -> `TestConnectivity_SingletonConnected`; `MergeStep.connectedCommunities` -> `TestRefine_MergePreservesConnectivity`; `connectedCommunities_of_mergeRun` -> `TestRefine_RunConnected`; `gammaDense_union` -> `TestCPM_GammaDenseUnion`; `GammaMergeStep.gammaDenseCommunities` -> `TestRefine_MergePreservesGammaDensity`.

## Acceptance Criteria

TDD order. 1) `TestConnectivity_SingletonConnected`: singleton-partition communities (single nodes) are connected. 2) `TestRefine_MergePreservesConnectivity`: an edge-merge of two communities sharing an edge preserves connected communities (`MergeStep.connectedCommunities`). 3) `TestRefine_RunConnected`: any edge-merge run from singletons yields connected communities. 4) Well-connectedness gate fixture: a case where a naive merge would create a disconnected community - assert refinement refuses it. 5) `TestCPM_GammaDenseUnion`: a gamma-dense cut joins two gamma-dense sets into a gamma-dense union (`gammaDense_union`). 6) `TestRefine_MergePreservesGammaDensity`: a gated merge preserves internal gamma-density (`GammaMergeStep.gammaDenseCommunities`). 7) Refinement randomness is per-node seeded and scheduling-independent (same result regardless of node processing order). `make validate` and `make test-race` green.

## Notes

**2026-07-17T21:50:13Z**

Leiden refinement phase landed in refine.go. Adds mergeCommunities, communityConnected/connectedCommunities (Go image of Lean Connectivity.lean), gammaDenseCut/gammaDenseCommunities, blockWeight (in cpm.go), and the refine operator. Design: each node computes one merge intent against the fixed singleton partition (refineIntents), gated by shared positive-weight edge (connectivity) + same outer community + gamma-dense cut + strictly improving moveDelta, chosen gain-weighted from its own stream newPRNG(nodeSeed(seed,v)); intents are resolved by an order-independent union-find (unionIntents, smaller-root tie-break). This makes refine a pure function of (g,obj,outer,seed) and byte-identical across node processing orders (AC7), diverging from the sequential reference on purpose per design 4.4/4.5. objective grew resolution() so the gate can read gamma; prng grew float64(). All 7 ACs green (TestConnectivity_SingletonConnected, TestRefine_MergePreservesConnectivity/RunConnected/GateRefusesDisconnecting/MergePreservesGammaDensity/SchedulingIndependent, TestCPM_GammaDenseUnion). make validate + make test-race + make build green. CORRESPONDENCE.md: 5 step-level rows flipped to landed; full-operator run rows (connectedCommunities_of_refineRun, gammaWellConnectedCommunities_of_gammaMergeRun) stay planned for the assembly ticket mes-jbc7. Unblocks mes-jbc7.
