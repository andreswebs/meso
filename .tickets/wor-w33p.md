---
id: wor-w33p
status: closed
deps: []
links: [mes-wmzq, mes-y8ru]
created: 2026-07-17T23:59:38Z
type: bug
priority: 1
assignee: Andre Silva
parent: mes-mklx
tags: [leiden, refinement, connectivity, modularity, fuzz-found]
---
# Leiden refinement gate uses node sizes for modularity -> disconnected, suboptimal communities

The native fuzz target FuzzLeidenLouvain (mes-y8ru) found undirected graphs where meso's Leiden under modularity returns a DISCONNECTED community that is also LOWER quality than a trivially-better connected partition, violating the Leiden connectivity guarantee (docs/meso-design.md lines 85, 280) and the quality claim at once. Deterministic across all seeds; triggered by non-unit node weights combined with modularity.

## Design

Root cause: refine.go refineIntents (around line 204) gates every refinement merge on gamma*nodeSize(v)*nodeSize(u) <= w_vu - the CPM node-size density criterion (the Lean GammaMergeStep gate) - applied REGARDLESS of the objective. But modularity's objective is blind to node sizes: modularity.Quality and modularity.moveDelta use only weighted degrees and 2m (node sizes appear only in cpm.moveDelta, move.go:104). So when a caller sets node weights (a general Builder feature, not documented as CPM-only) and runs modularity, the objective ignores them but the refinement gate reads them. With node sizes large relative to edge weights, gamma*s_v*s_u > w_vu rejects every candidate, refineIntents returns all-singletons, refinement becomes a no-op, and Leiden degenerates to Louvain - inheriting Louvain's disconnected-community defect and its lost quality.

Reproduction (found by go test -fuzz):
  data := []byte("%CYc02v72X00\xf7290Y0VC2A11b88011029X720C7x00\x0680\xf901\n1Ab98X27a1A9810")
  seed = 111, gammaByte = 0x19  (any seed reproduces)
Decoded to a 14-node undirected graph, Leiden under Modularity(1.0) returns community {0,1,5,6} which splits into {0,1} (edge 0-1) and {5,6} (edge 5-6) with no edge between the halves. Q(disconnected output) = 0.303694; Q(split {5,6} off) = 0.356844 and connected. The output is single-node local-move stable, so only refinement (a subset/two-node move) can escape it - which is exactly the phase the node-size gate disables.

Scope of impact: modularity with non-unit node weights. Aggregation grows super-node sizes even from unit input, so deeper recursion levels could in principle hit the same gate; extensive random probing (80k graphs) and the existing corpus+800-fuzzed connectivity test did not surface a unit-input modularity failure, and the fuzzer only found node-weight-driven cases, but the aggregate-size path is worth checking during the fix.

Candidate fix directions (needs Lean CORRESPONDENCE review, since the gate mirrors GammaMergeStep): make the refinement well-connectedness gate objective-appropriate - for modularity, base it on the modularity move-delta / degrees rather than the CPM node-size product - or document node weights as CPM-only and reject/ignore them for modularity at the API. Either way the connectivity guarantee must hold for modularity over arbitrary node weights, or the guarantee's precondition must be stated explicitly.

## Acceptance Criteria

1. A regression test pins the reproduction above: meso's Leiden under modularity returns a connected partition on that graph (and no lower-quality-than-a-connected-alternative output). 2. The refinement well-connectedness gate is objective-appropriate so modularity refinement is not spuriously disabled by node sizes, OR node-weight semantics for modularity are explicitly defined and enforced. 3. FuzzLeidenLouvain can assert Leiden connectivity for modularity over node-weighted graphs (the temporary domain split in fuzz_test.go, modularity=unit sizes only, can be removed). 4. Lean CORRESPONDENCE stays consistent with the changed gate. 5. make validate and make test-race green.


## Notes

**2026-07-18T00:14:46Z**

Fixed. Root cause was that refineIntents (refine.go) gated every merge on the CPM node-size density criterion gamma*s_v*s_u > w regardless of objective; under modularity (blind to node sizes) non-unit node weights made the gate reject every merge, so refinement became a no-op and Leiden degenerated to Louvain, inheriting its disconnected/suboptimal communities.

Key insight: at the singleton base refineIntents runs over, the well-connectedness density criterion IS the objective's own move-delta. So gating on obj.moveDelta(base,v,u) > moveImproveEps (already present) is automatically objective-appropriate: for CPM it is 2*(w - gamma*s_v*s_u) > 0 = the strict GammaMergeStep gate; for modularity the degree-based (2/2m)*(w - gamma*k_v*k_u/2m) > 0. The explicit gamma*s_v*s_u gate was thus redundant-and-correct for CPM (gain check subsumes it) but wrong for modularity. Fix = delete that gate line; connectivity is preserved by the unconditional w>0 candidate filter, which is all the objective-generic connectivity guarantee needs (Lean connectedCommunities_of_refineRun proves it from a shared positive-weight edge alone). CPM output is byte-identical; modularity now refines correctly.

Deleting the gate dropped resolution() out of the objective interface (its only reader), so I removed resolution() from the interface and modularity/cpm/directedModularity.

Lean CORRESPONDENCE: confirmed via research that the node-size gate is CPM-only (GammaMergeStep/GammaConnectivity) with no modularity referent, and connectivity is objective-generic. Added a bullet to CORRESPONDENCE.md documenting the objective-appropriate Go gate.

fuzz_test.go: removed the temporary modularity=unit-sizes domain split (dropped the applyNodeWeights param from buildFuzzGraph; both objectives now run over node-weighted graphs); added the reproduction bytes as a FuzzLeidenLouvain seed. A 30s/~900k-exec fuzz run stayed green including deep aggregation levels. make validate + make test-race + make build all green. Learnings appended to docs/specs/learnings.md.
