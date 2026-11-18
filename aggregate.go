package meso

import "sort"

// aggregate collapses each community of p into a single super-node, producing
// the aggregate graph over dense indices [0, k) where k is the number of
// distinct communities in p. It is the Go image of the Lean aggregate
// (verification/lean/Meso/Aggregate.lean) and the machinery shared by Louvain
// and Leiden to recurse a level.
//
// Each super-node A carries: its members' summed node sizes (nodeSize A), and a
// self-loop equal to community A's internal edge weight - the block sum over
// node pairs both in A, folding the members' own self-loops and twice each
// internal off-diagonal edge (Lean blockWeight A A). An off-diagonal aggregate
// edge between A and B carries the total weight crossing between the two
// communities (blockWeight A B). Preserving internal weight on the self-loop and
// summing node sizes is correctness-critical: drop either and the resolution
// term breaks silently (docs/meso-design.md section 4.1).
//
// It returns the aggregate csr and superOf, the length-n map from each base node
// to its super-node index. superOf is the community-to-dense-index relabel (the
// Go image of the Lean commLabel bijection): superOf[i] == superOf[j] exactly
// when p[i] == p[j], and communities are numbered canonically, by ascending
// label, so the result is independent of node order.
//
// A directed graph aggregates to a directed graph: the fold runs once over every
// out-arc (a directed arc is stored from its source only), so an internal arc
// (a == b) lands on the self-loop once and a cross arc accumulates into the
// aggregate out-arc a -> b and the aligned in-arc b <- a, keeping out- and
// in-degree separable across the level (design section 4.1). On a symmetric
// directed graph each undirected edge appears as both i -> j and j -> i, so an
// internal edge again lands 2*weight on the self-loop and the aggregate comes out
// symmetric, matching the undirected aggregate.
func aggregate(g *csr, p Partition) (*csr, []int) {
	superOf, k := denseLabels(p)
	if g.directed {
		return aggregateDirected(g, superOf, k), superOf
	}
	return aggregateUndirected(g, superOf, k), superOf
}

// aggregateUndirected folds an undirected graph into its k-node aggregate. An
// off-diagonal edge i-j is visited from both endpoints, so an internal edge
// (a == b) lands on the self-loop twice (giving 2*weight, matching blockWeight
// A A) and a cross edge accumulates blockWeight from each side into the symmetric
// pair (a, b) and (b, a).
func aggregateUndirected(g *csr, superOf []int, k int) *csr {
	out, selfLoops, nodeSizes := foldOutArcs(g, superOf, k)
	return newCSR(out.offsets, out.neighbors, out.weights, selfLoops, nodeSizes)
}

// aggregateDirected folds a directed graph into its k-node aggregate, keeping
// out- and in-arcs separable. The fold runs over each node's out-arcs (each
// directed arc is stored once, from its source): an internal arc (a == b)
// lands once on the self-loop, and a cross arc a -> b accumulates into the
// aggregate out-arc (a, b). The aggregate in-arc (b, a) carries the same sum of
// the same arcs, so the in-adjacency is the transpose of the out-adjacency.
// Node sizes and members' own self-loops fold in as in the undirected case.
func aggregateDirected(g *csr, superOf []int, k int) *csr {
	out, selfLoops, nodeSizes := foldOutArcs(g, superOf, k)
	return &csr{
		n:         k,
		directed:  true,
		out:       out,
		in:        transpose(out, k),
		selfLoops: selfLoops,
		nodeSizes: nodeSizes,
	}
}

// foldOutArcs sums g's out-arcs by super-node into the k-node aggregate's out
// adjacency, self-loops and node sizes. Each super-node's targets are listed in
// ascending index order, so the layout is canonical.
//
// The super-nodes are folded one at a time, each over its members in ascending
// node order, so every sum receives its terms in the order a single ascending
// pass over all nodes would deliver them. One dense row of cross weights is
// reused across super-nodes and cleared through the list of targets it touched.
func foldOutArcs(g *csr, superOf []int, k int) (out adjacency, selfLoops, nodeSizes []float64) {
	n := g.numNodes()
	nodeSizes = make([]float64, k)
	selfLoops = make([]float64, k)

	// Members of each super-node, ascending: members[start[a]:start[a+1]].
	start := make([]int, k+1)
	for _, a := range superOf {
		start[a+1]++
	}
	for a := range k {
		start[a+1] += start[a]
	}
	members := make([]int, n)
	next := make([]int, k)
	copy(next, start[:k])
	for i, a := range superOf {
		members[next[a]] = i
		next[a]++
	}

	row := make([]float64, k)
	inRow := make([]bool, k)
	var touched []int

	out.offsets = make([]int, k+1)
	for a := range k {
		touched = touched[:0]
		for _, i := range members[start[a]:start[a+1]] {
			nodeSizes[a] += g.nodeSize(i)
			selfLoops[a] += g.selfLoops[i]
			nw := g.neighborWeights(i)
			for x, j := range g.neighbors(i) {
				b := superOf[j]
				if a == b {
					selfLoops[a] += nw[x]
					continue
				}
				if !inRow[b] {
					inRow[b] = true
					touched = append(touched, b)
				}
				row[b] += nw[x]
			}
		}
		sort.Ints(touched)
		for _, b := range touched {
			out.neighbors = append(out.neighbors, b)
			out.weights = append(out.weights, row[b])
			row[b] = 0
			inRow[b] = false
		}
		out.offsets[a+1] = len(out.neighbors)
	}
	if out.neighbors == nil {
		out.neighbors = []int{}
		out.weights = []float64{}
	}
	return out, selfLoops, nodeSizes
}

// transpose returns the adjacency holding arc (b, a, w) for every arc (a, b, w)
// of adj over k nodes. Sources are visited in ascending order, so each node's
// list comes out sorted by neighbour.
func transpose(adj adjacency, k int) adjacency {
	offsets := make([]int, k+1)
	for _, b := range adj.neighbors {
		offsets[b+1]++
	}
	for b := range k {
		offsets[b+1] += offsets[b]
	}
	neighbors := make([]int, len(adj.neighbors))
	weights := make([]float64, len(adj.weights))
	next := make([]int, k)
	copy(next, offsets[:k])
	for a := range k {
		for x := adj.offsets[a]; x < adj.offsets[a+1]; x++ {
			b := adj.neighbors[x]
			neighbors[next[b]] = a
			weights[next[b]] = adj.weights[x]
			next[b]++
		}
	}
	return adjacency{offsets: offsets, neighbors: neighbors, weights: weights}
}

// expand lifts an aggregate partition back to the base graph: base node i takes
// the aggregate label of its super-node, aggP[superOf[i]]. It is the round-trip
// counterpart of aggregate's relabel, used by the optimizers to carry a partition
// found on the aggregate back down to the original nodes.
func expand(superOf []int, aggP Partition) Partition {
	base := make(Partition, len(superOf))
	for i, s := range superOf {
		base[i] = aggP[s]
	}
	return base
}
