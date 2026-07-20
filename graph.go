package meso

import "fmt"

// adjacency is a CSR (compressed sparse row) view of a set of weighted arcs.
// For node i the arcs are neighbors[offsets[i]:offsets[i+1]] with the aligned
// weights weights[offsets[i]:offsets[i+1]]. offsets has length n+1 and is
// monotonic non-decreasing.
//
// For an undirected graph a single adjacency holds each edge from both of its
// endpoints. Directed graphs (M3) carry a second adjacency for in-arcs, which
// is why the neighbour lists are factored out here rather than inlined into
// csr: the layout is reused unchanged.
type adjacency struct {
	offsets   []int
	neighbors []int
	weights   []float64
}

// csr is the internal compact weighted-graph model over dense node indices
// [0, n). It mirrors the Lean WeightedGraph over Fin n (see
// verification/lean/Meso/Graph.lean): symmetric nonnegative edge weights plus
// per-node sizes.
//
// Self-loops are kept out of the neighbour lists and stored separately in
// selfLoops so the resolution term can use each node's internal edge weight
// directly and aggregation can fold a community's internal weight into a single
// self-loop. A self-loop contributes once to degree and to twoM, an off-diagonal
// edge contributes to both of its endpoints' adjacency entries.
//
// For an undirected graph (directed == false) out holds every incident arc and
// in is unused. For a directed graph out holds each node's out-arcs and in holds
// its in-arcs, so out-degree and in-degree stay separable (the foundation for
// directed modularity in M3).
type csr struct {
	n         int
	directed  bool
	out       adjacency
	in        adjacency
	selfLoops []float64
	nodeSizes []float64
}

// newCSR builds a csr from raw CSR arrays. It is the low-level constructor used
// by tests and, later, by the public Builder (which computes these arrays).
//
// offsets must have length n+1; neighbors and weights are aligned and have
// length offsets[n]. A nil selfLoops is treated as all-zero; a nil nodeSizes is
// treated as the default size 1.0 for every node.
func newCSR(offsets, neighbors []int, weights, selfLoops, nodeSizes []float64) *csr {
	n := len(offsets) - 1

	if selfLoops == nil {
		selfLoops = make([]float64, n)
	}
	if nodeSizes == nil {
		nodeSizes = make([]float64, n)
		for i := range nodeSizes {
			nodeSizes[i] = 1.0
		}
	}

	return &csr{
		n:         n,
		out:       adjacency{offsets: offsets, neighbors: neighbors, weights: weights},
		selfLoops: selfLoops,
		nodeSizes: nodeSizes,
	}
}

// checkInvariants verifies the structural invariants of the CSR layout and
// returns a descriptive error on the first violation, nil when all hold: the
// out adjacency (and, for a directed graph, the in adjacency) is well-formed
// (offsets has length n+1, starts at 0, is monotonic non-decreasing, offsets[n]
// equals the aligned neighbour and weight lengths, every neighbour index lies
// in [0, n)); selfLoops and nodeSizes have length n.
func (g *csr) checkInvariants() error {
	if err := g.out.check(g.n, "out"); err != nil {
		return err
	}
	if g.directed {
		if err := g.in.check(g.n, "in"); err != nil {
			return err
		}
	}
	if len(g.selfLoops) != g.n {
		return fmt.Errorf("meso: selfLoops length %d, want n = %d", len(g.selfLoops), g.n)
	}
	if len(g.nodeSizes) != g.n {
		return fmt.Errorf("meso: nodeSizes length %d, want n = %d", len(g.nodeSizes), g.n)
	}
	return nil
}

// check verifies the structural invariants of a single CSR adjacency over n
// nodes, returning a descriptive error (tagged with name) on the first
// violation and nil when all hold.
func (a adjacency) check(n int, name string) error {
	if len(a.offsets) != n+1 {
		return fmt.Errorf("meso: %s offsets length %d, want n+1 = %d", name, len(a.offsets), n+1)
	}
	if a.offsets[0] != 0 {
		return fmt.Errorf("meso: %s offsets[0] = %d, want 0", name, a.offsets[0])
	}
	for i := range n {
		if a.offsets[i+1] < a.offsets[i] {
			return fmt.Errorf("meso: %s offsets not monotonic at %d: %d < %d", name, i, a.offsets[i+1], a.offsets[i])
		}
	}
	if got := a.offsets[n]; got != len(a.neighbors) {
		return fmt.Errorf("meso: %s offsets[n] = %d, want len(neighbors) = %d", name, got, len(a.neighbors))
	}
	if len(a.neighbors) != len(a.weights) {
		return fmt.Errorf("meso: %s neighbors length %d != weights length %d", name, len(a.neighbors), len(a.weights))
	}
	for _, v := range a.neighbors {
		if v < 0 || v >= n {
			return fmt.Errorf("meso: %s neighbour index %d out of range [0, %d)", name, v, n)
		}
	}
	return nil
}

// numNodes reports the node count n.
func (g *csr) numNodes() int { return g.n }

// neighbors returns node i's off-diagonal neighbour indices as a view into the
// underlying storage. The aligned weights are neighborWeights(i).
func (g *csr) neighbors(i int) []int {
	return g.out.neighbors[g.out.offsets[i]:g.out.offsets[i+1]]
}

// neighborWeights returns the edge weights aligned with neighbors(i).
func (g *csr) neighborWeights(i int) []float64 {
	return g.out.weights[g.out.offsets[i]:g.out.offsets[i+1]]
}

// weight returns the edge weight w_ij between nodes i and j, mirroring the Lean
// model's G.weight i j: the self-loop weight when i == j, otherwise the
// off-diagonal edge weight (0 when i and j are not adjacent). It scans node i's
// sorted neighbour list, so it is O(deg i) and intended for the from-scratch
// quality evaluators, not the hot path.
func (g *csr) weight(i, j int) float64 {
	if i == j {
		return g.selfLoops[i]
	}
	nb := g.neighbors(i)
	for k, v := range nb {
		if v == j {
			return g.neighborWeights(i)[k]
		}
	}
	return 0
}

// inNeighbors returns node i's in-arc source indices as a view into the
// underlying storage; the aligned weights are inNeighborWeights(i). For an
// undirected graph the in adjacency is unused, so this returns node i's
// out-neighbours instead, mirroring inDegree's fallback.
func (g *csr) inNeighbors(i int) []int {
	if !g.directed {
		return g.neighbors(i)
	}
	return g.in.neighbors[g.in.offsets[i]:g.in.offsets[i+1]]
}

// inNeighborWeights returns the arc weights aligned with inNeighbors(i).
func (g *csr) inNeighborWeights(i int) []float64 {
	if !g.directed {
		return g.neighborWeights(i)
	}
	return g.in.weights[g.in.offsets[i]:g.in.offsets[i+1]]
}

// nodeSize returns node i's size (its node weight), used by the resolution
// term and preserved through aggregation. Nodes built without an explicit size
// default to 1.0 (see newCSR).
func (g *csr) nodeSize(i int) float64 { return g.nodeSizes[i] }

// twoM returns 2m: twice the total edge weight, sum_i degree(i) =
// sum_{i,j} w_ij. Off-diagonal edges contribute to both endpoints (and so are
// counted twice); self-loop weights are counted once.
func (g *csr) twoM() float64 {
	sum := 0.0
	for i := 0; i < g.n; i++ {
		sum += g.degree(i)
	}
	return sum
}

// degree returns the weighted degree k_i = sum_j w_ij, including the node's
// self-loop weight (the j == i term), counted once. For an undirected graph
// this is the node's total incident weight; for a directed graph it is the
// out-degree (out holds the out-arcs), so prefer outDegree/inDegree there.
func (g *csr) degree(i int) float64 {
	sum := g.selfLoops[i]
	for _, w := range g.neighborWeights(i) {
		sum += w
	}
	return sum
}

// outDegree returns node i's weighted out-degree: the sum of its out-arc
// weights plus its self-loop weight. For an undirected graph it equals degree.
func (g *csr) outDegree(i int) float64 { return g.degree(i) }

// inDegree returns node i's weighted in-degree: the sum of its in-arc weights
// plus its self-loop weight. For an undirected graph, where in-degree equals
// out-degree, it equals degree.
func (g *csr) inDegree(i int) float64 {
	if !g.directed {
		return g.degree(i)
	}
	sum := g.selfLoops[i]
	for _, w := range g.in.weights[g.in.offsets[i]:g.in.offsets[i+1]] {
		sum += w
	}
	return sum
}
