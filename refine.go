package meso

import "sort"

// mergeCommunities merges community b into community a: every node labelled b is
// relabelled a, every other node is left untouched, and a fresh Partition is
// returned (p is not mutated). It is the Go image of the Lean mergeCommunities
// (verification/lean/Meso/Refinement.lean), the one-step operator the refinement
// connectivity and gamma-density theorems are stated over.
func mergeCommunities(p Partition, a, b int) Partition {
	q := make(Partition, len(p))
	for i, c := range p {
		if c == b {
			q[i] = a
		} else {
			q[i] = c
		}
	}
	return q
}

// communityConnected reports whether community c induces a connected subgraph of
// g: any two nodes labelled c are joined by a walk that stays within c, over the
// underlying simple graph (an edge exactly where the off-diagonal weight is
// positive; self-loops carry internal weight and are not connectivity edges). It
// is the Go image of the Lean CommunityConnected
// (verification/lean/Meso/Connectivity.lean).
//
// A community with at most one member is connected (Preconnected.of_subsingleton):
// the empty and singleton cases return true. Otherwise a depth-first traversal
// from one member, following only positive-weight edges to other members, must
// reach every member.
func communityConnected(g *csr, p Partition, c int) bool {
	var members []int
	for i := 0; i < g.numNodes(); i++ {
		if p[i] == c {
			members = append(members, i)
		}
	}
	if len(members) <= 1 {
		return true
	}

	visited := make(map[int]bool, len(members))
	stack := []int{members[0]}
	visited[members[0]] = true
	reached := 1
	for len(stack) > 0 {
		u := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		nb := g.neighbors(u)
		nw := g.neighborWeights(u)
		for k, v := range nb {
			if nw[k] > 0 && p[v] == c && !visited[v] {
				visited[v] = true
				reached++
				stack = append(stack, v)
			}
		}
	}
	return reached == len(members)
}

// connectedCommunities reports whether every occupied community of p induces a
// connected subgraph of g. It is the Go image of the Lean ConnectedCommunities
// (verification/lean/Meso/Connectivity.lean) and the connectivity guarantee
// Leiden's refinement establishes while Louvain does not.
func connectedCommunities(g *csr, p Partition) bool {
	seen := make(map[int]bool, len(p))
	for _, c := range p {
		if seen[c] {
			continue
		}
		seen[c] = true
		if !communityConnected(g, p, c) {
			return false
		}
	}
	return true
}

// gammaDenseCut reports whether the cut between communities a and b is gamma-dense
// under p: the cross weight covers gamma*S_a*S_b (gamma*communitySize a *
// communitySize b <= blockWeight a b). It is the Go image of the density half of
// the Lean GammaMergeStep gate (verification/lean/Meso/GammaConnectivity.lean) -
// the well-connectedness condition on a merge - and the arithmetic hypothesis of
// gammaDense_union. The connectivity half is a shared positive-weight edge.
func gammaDenseCut(g *csr, gamma float64, p Partition, a, b int) bool {
	return gamma*communitySize(g, p, a)*communitySize(g, p, b) <= blockWeight(g, p, a, b)
}

// gammaDenseCommunities reports whether every occupied community of p is
// internally gamma-dense. It is the Go image of the Lean GammaDenseCommunities
// (verification/lean/Meso/GammaConnectivity.lean), the density analog of
// connectedCommunities and the invariant a gamma-gated merge preserves.
func gammaDenseCommunities(g *csr, gamma float64, p Partition) bool {
	seen := make(map[int]bool, len(p))
	for _, c := range p {
		if seen[c] {
			continue
		}
		seen[c] = true
		if !isGammaDense(g, gamma, p, c) {
			return false
		}
	}
	return true
}

// gammaSeparatedCommunities reports whether every pair of distinct occupied
// communities of p is gamma-separated: the between-community weight covers no
// more than gamma times the product of their sizes
// (blockWeight a b <= gamma*S_a*S_b for all a != b). It is the Go image of the
// paper's first guarantee predicate (verification/lean/Meso/Separation.lean,
// gammaSeparated_of_converged): at a CPM-converged partition, merging any two
// communities cannot raise quality, which is exactly this bound on the aggregate
// edge weight. The density companion of gammaDenseCommunities and the separation
// analog of connectedCommunities.
func gammaSeparatedCommunities(g *csr, gamma float64, p Partition) bool {
	seen := make(map[int]bool, len(p))
	labels := make([]int, 0, len(p))
	for _, c := range p {
		if !seen[c] {
			seen[c] = true
			labels = append(labels, c)
		}
	}
	for _, a := range labels {
		sa := communitySize(g, p, a)
		for _, b := range labels {
			if a == b {
				continue
			}
			if blockWeight(g, p, a, b) > gamma*sa*communitySize(g, p, b) {
				return false
			}
		}
	}
	return true
}

// gammaWellConnectedCommunities reports whether every occupied community of p is
// both connected and internally gamma-dense. It is the Go image of the Lean
// GammaWellConnectedCommunities (verification/lean/Meso/GammaConnectivity.lean),
// the conjunction (connectedCommunities and gammaDenseCommunities) that a gated
// refinement run preserves (gammaWellConnectedCommunities_of_gammaMergeRun) and
// the second of the three paper guarantees.
func gammaWellConnectedCommunities(g *csr, gamma float64, p Partition) bool {
	return connectedCommunities(g, p) && gammaDenseCommunities(g, gamma, p)
}

// refineCand is one candidate merge target for a node during refinement: the
// neighbour community's node u and the quality gain of merging into it.
type refineCand struct {
	u    int
	gain float64
}

// refineIntents computes, for each node, its refinement merge intent: the
// neighbouring singleton it chooses to merge with, or -1 for none. It is the
// scheduling-independent heart of Leiden's refinement phase (design section 4.1
// phase 2, 4.4): every node restarts as its own singleton and, using only its own
// seed, picks one gated, quality-improving neighbour to merge toward. Because each
// intent is a pure function of (globalSeed, nodeID) and the fixed singleton
// gains - never of the evolving partition or the visiting order - the intents can
// be resolved in any order (unionIntents) and give the same result.
//
// A neighbour u is a candidate for node v only when the well-connectedness gate
// passes: u shares a positive-weight edge with v (connectivity, so no merge can
// disconnect a community), u lies in the same phase-1 outer community
// (outer[u] == outer[v]), and merging v into u strictly improves the objective
// (obj.moveDelta above moveImproveEps). Among the candidates, examined in
// ascending node order so the choice is order-independent, one is drawn with
// probability proportional to its gain from node v's private stream
// (newPRNG(nodeSeed(seed, v))).
//
// The gate splits into an objective-generic connectivity half and an
// objective-appropriate density half. The connectivity half - a shared
// positive-weight edge (w > 0) - is all the Leiden connectivity guarantee needs:
// every refined community stays a connected subgraph regardless of the objective
// (the Lean MergeStep.connectedCommunities, verification/lean/Meso/Refinement.lean,
// is proved from the shared edge alone, with no node-size or density hypothesis).
// The density half is the objective's own move-delta: at the singleton level base
// (each node alone), obj.moveDelta(base, v, u) > 0 is exactly the well-connectedness
// density criterion in the objective's own currency - for CPM it is
// gamma*s_v*s_u < w_vu (the Lean GammaMergeStep density gate, which reads node
// sizes because CPM's objective does), and for modularity it is the
// degree-based gamma*k_v*k_u/2m < w_vu (modularity's objective is blind to node
// sizes). Requiring obj.moveDelta above moveImproveEps therefore applies the
// objective-appropriate gate for whichever objective is running; a fixed
// node-size product would wrongly gate a modularity run on sizes its objective
// ignores (bug wor-w33p: with non-unit node weights it rejected every modularity
// merge, disabling refinement and degrading Leiden to Louvain).
//
// On a directed graph the candidates are gathered from both v's out-neighbours
// and its in-neighbours: v and u are connected if a positive-weight arc runs in
// either direction, so the connectivity half reads the arc weight in the
// direction u was discovered (w_vu for an out-arc, w_uv for an in-arc), while the
// gain is the full directed move-delta regardless. A node reachable both ways is
// considered once. On an undirected graph the in-adjacency mirrors the
// out-adjacency and w_vu == w_uv, so the in pass adds nothing and the behaviour
// is unchanged.
func refineIntents(g *csr, obj objective, outer Partition, seed uint64) []int {
	n := g.numNodes()
	base := singleton(n)

	intents := make([]int, n)
	for v := range n {
		intents[v] = -1

		var cands []refineCand
		seen := make(map[int]struct{})
		consider := func(u int, w float64) {
			if _, ok := seen[u]; ok {
				return
			}
			if w <= 0 || outer[u] != outer[v] {
				return
			}
			gain := obj.moveDelta(g, base, v, u)
			if gain <= moveImproveEps {
				return
			}
			seen[u] = struct{}{}
			cands = append(cands, refineCand{u: u, gain: gain})
		}

		nb := g.neighbors(v)
		nw := g.neighborWeights(v)
		for k, u := range nb {
			consider(u, nw[k])
		}
		if g.directed {
			inNb := g.inNeighbors(v)
			inW := g.inNeighborWeights(v)
			for k, u := range inNb {
				consider(u, inW[k])
			}
		}

		if len(cands) == 0 {
			continue
		}
		sort.Slice(cands, func(i, j int) bool { return cands[i].u < cands[j].u })
		intents[v] = weightedChoice(cands, newPRNG(nodeSeed(seed, v)))
	}
	return intents
}

// weightedChoice draws one candidate's node from cands with probability
// proportional to its gain, using rng, and returns it. cands must be non-empty
// with positive gains and in a canonical (ascending-node) order, so the choice is
// a deterministic function of the stream and independent of discovery order.
func weightedChoice(cands []refineCand, rng *prng) int {
	total := 0.0
	for _, c := range cands {
		total += c.gain
	}
	r := rng.float64() * total
	acc := 0.0
	for _, c := range cands {
		acc += c.gain
		if r < acc {
			return c.u
		}
	}
	return cands[len(cands)-1].u
}

// unionIntents resolves refinement intents into a partition: it unions each node
// with its intended merge target (skipping -1) and labels each node by its
// union-find root, canonicalised. order is the sequence in which intents are
// applied; because a union is symmetric and ties break to the smaller root, the
// resulting components - and the canonical labels - are identical for every
// permutation, which is what makes refinement scheduling-independent (design
// section 4.4). It is separated from refineIntents so tests can apply the same
// intents in different orders and confirm the invariance.
func unionIntents(n int, intents []int, order []int) Partition {
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	find := func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra == rb {
			return
		}
		if ra < rb {
			parent[rb] = ra
		} else {
			parent[ra] = rb
		}
	}

	for _, v := range order {
		if t := intents[v]; t >= 0 {
			union(v, t)
		}
	}

	refined := make(Partition, n)
	for i := range n {
		refined[i] = find(i)
	}
	return canonicalize(refined)
}

// refine runs Leiden's refinement phase over graph g under objective obj: starting
// from singletons within each phase-1 outer community, it merges nodes by a
// randomized, gain-weighted, well-connectedness-gated choice and returns the
// refined partition, a well-formed canonically-labelled Partition. Every refined
// community is connected (the gate merges only across shared edges) and the result
// is a pure function of (g, obj, outer, seed): per-node seeding makes it
// independent of node processing order (design section 4.1 phase 2, 4.4).
//
// refine produces the sub-communities that seed the next aggregation level; the
// aggregate's initial partition is still taken from the non-refined outer
// partition, which the Leiden assembly (a later step) wires up. This function is
// the refinement operator in isolation.
func refine(g *csr, obj objective, outer Partition, seed uint64) Partition {
	n := g.numNodes()
	intents := refineIntents(g, obj, outer, seed)
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	return unionIntents(n, intents, order)
}
