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
	n := g.numNodes()

	// Canonical relabel: distinct community labels sorted ascending -> [0, k).
	present := make(map[int]struct{}, n)
	for _, c := range p {
		present[c] = struct{}{}
	}
	labels := make([]int, 0, len(present))
	for c := range present {
		labels = append(labels, c)
	}
	sort.Ints(labels)
	dense := make(map[int]int, len(labels))
	for d, c := range labels {
		dense[c] = d
	}
	k := len(labels)

	superOf := make([]int, n)
	for i, c := range p {
		superOf[i] = dense[c]
	}

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
	n := g.numNodes()
	nodeSizes := make([]float64, k)
	selfLoops := make([]float64, k)
	block := make([]map[int]float64, k)
	for a := range block {
		block[a] = make(map[int]float64)
	}

	for i := range n {
		a := superOf[i]
		nodeSizes[a] += g.nodeSize(i)
		selfLoops[a] += g.selfLoops[i]
		nb := g.neighbors(i)
		nw := g.neighborWeights(i)
		for x, j := range nb {
			b := superOf[j]
			if a == b {
				selfLoops[a] += nw[x]
			} else {
				block[a][b] += nw[x]
			}
		}
	}

	offsets, neighbors, weights := flattenBlock(block, k)
	return newCSR(offsets, neighbors, weights, selfLoops, nodeSizes)
}

// aggregateDirected folds a directed graph into its k-node aggregate, keeping
// out- and in-arcs separable. The single pass runs over each node's out-arcs
// (each directed arc is stored once, from its source): an internal arc (a == b)
// lands once on the self-loop, and a cross arc a -> b accumulates into both the
// aggregate out-block (a, b) and the in-block (b, a). Node sizes and members'
// own self-loops fold in as in the undirected case.
func aggregateDirected(g *csr, superOf []int, k int) *csr {
	n := g.numNodes()
	nodeSizes := make([]float64, k)
	selfLoops := make([]float64, k)
	outBlock := make([]map[int]float64, k)
	inBlock := make([]map[int]float64, k)
	for a := range k {
		outBlock[a] = make(map[int]float64)
		inBlock[a] = make(map[int]float64)
	}

	for i := range n {
		a := superOf[i]
		nodeSizes[a] += g.nodeSize(i)
		selfLoops[a] += g.selfLoops[i]
		nb := g.neighbors(i)
		nw := g.neighborWeights(i)
		for x, j := range nb {
			b := superOf[j]
			if a == b {
				selfLoops[a] += nw[x]
			} else {
				outBlock[a][b] += nw[x]
				inBlock[b][a] += nw[x]
			}
		}
	}

	outOff, outNb, outW := flattenBlock(outBlock, k)
	inOff, inNb, inW := flattenBlock(inBlock, k)
	return &csr{
		n:         k,
		directed:  true,
		out:       adjacency{offsets: outOff, neighbors: outNb, weights: outW},
		in:        adjacency{offsets: inOff, neighbors: inNb, weights: inW},
		selfLoops: selfLoops,
		nodeSizes: nodeSizes,
	}
}

// flattenBlock turns per-super-node accumulated cross weights into CSR arrays,
// listing each super-node's targets in ascending index order so the layout is
// canonical (independent of map iteration order).
func flattenBlock(block []map[int]float64, k int) (offsets, neighbors []int, weights []float64) {
	offsets = make([]int, k+1)
	for a := range k {
		offsets[a+1] = offsets[a] + len(block[a])
	}
	neighbors = make([]int, offsets[k])
	weights = make([]float64, offsets[k])
	idx := 0
	for a := range k {
		dsts := make([]int, 0, len(block[a]))
		for b := range block[a] {
			dsts = append(dsts, b)
		}
		sort.Ints(dsts)
		for _, b := range dsts {
			neighbors[idx] = b
			weights[idx] = block[a][b]
			idx++
		}
	}
	return offsets, neighbors, weights
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
