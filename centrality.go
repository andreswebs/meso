package meso

// Betweenness returns the betweenness centrality of every node, keyed by the
// caller's keys, computed by Brandes' algorithm (Brandes, 2001) over unweighted
// shortest paths: every edge counts one hop, and edge weights, node weights and
// self-loops are ignored.
//
// On an undirected graph each unordered pair of endpoints is counted once and
// the value is normalized by (n-1)(n-2)/2; on a directed graph paths follow
// the arcs and the value is normalized by (n-1)(n-2). Values lie in [0, 1]. A
// graph with fewer than three nodes yields zero for every node.
//
// Sources are processed in ascending dense index and neighbours in adjacency
// order, so the floating-point accumulation order is fixed and the result is
// byte-identical across runs and machines for the same graph.
func Betweenness(g *Graph) map[string]float64 {
	m := g.model
	n := m.n
	bc := make([]float64, n)
	if n >= 3 {
		brandes(m, bc)
		// Undirected: halving the doubled pair count and dividing by
		// (n-1)(n-2)/2 is one division by (n-1)(n-2), the directed constant.
		scale := 1 / float64((n-1)*(n-2))
		for i := range bc {
			bc[i] *= scale
		}
	}
	out := make(map[string]float64, n)
	for i, v := range bc {
		out[g.keys[i]] = v
	}
	return out
}

// brandes adds to bc every node's raw dependency summed over all sources: the
// number of ordered (s, t) pairs, weighted by path fraction, whose shortest
// paths pass through it. Scratch arrays are allocated once and reset per
// source; predecessors are recovered from the in-adjacency (a predecessor of w
// is an in-neighbour one hop closer to the source), so the per-source loop
// does not allocate.
func brandes(m *csr, bc []float64) {
	n := m.n
	dist := make([]int, n)
	sigma := make([]float64, n)
	delta := make([]float64, n)
	order := make([]int, 0, n)
	for s := range n {
		for i := range n {
			dist[i] = -1
			sigma[i] = 0
			delta[i] = 0
		}
		dist[s], sigma[s] = 0, 1
		order = append(order[:0], s)
		// order doubles as the BFS queue and, read backwards, the Brandes
		// stack: nodes are appended in non-decreasing distance.
		for head := 0; head < len(order); head++ {
			v := order[head]
			for _, w := range m.neighbors(v) {
				if dist[w] < 0 {
					dist[w] = dist[v] + 1
					order = append(order, w)
				}
				if dist[w] == dist[v]+1 {
					sigma[w] += sigma[v]
				}
			}
		}
		for k := len(order) - 1; k > 0; k-- {
			w := order[k]
			coeff := (1 + delta[w]) / sigma[w]
			// w is never the source, so dist[w]-1 >= 0 and an unreached v
			// (dist -1) cannot match.
			for _, v := range m.inNeighbors(w) {
				if dist[v] == dist[w]-1 {
					delta[v] += sigma[v] * coeff
				}
			}
			bc[w] += delta[w]
		}
	}
}
