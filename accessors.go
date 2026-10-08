package meso

import "slices"

// Keys returns every node key in dense index order: Keys()[i] == Key(i). Under
// [Builder.Canonical] that is ascending key order. The slice is a copy the
// caller may modify.
func (g *Graph) Keys() []string {
	return slices.Clone(g.keys)
}

// NumEdges returns the number of distinct edges: unordered pairs on an
// undirected graph, arcs on a directed one. Parallel edges were folded by the
// Builder and count once; self-loops are not counted.
func (g *Graph) NumEdges() int {
	n := len(g.model.out.neighbors)
	if g.model.directed {
		return n
	}
	return n / 2
}

// Degree returns the number of distinct neighbours of key, self excluded, and
// whether key is present. On a directed graph a neighbour is any node joined to
// key by an arc in either direction, counted once even when both arcs exist, so
// Degree(key) == len(Neighbors(key)) on every graph.
func (g *Graph) Degree(key string) (int, bool) {
	i, ok := g.index[key]
	if !ok {
		return 0, false
	}
	if !g.model.directed {
		return len(g.model.neighbors(i)), true
	}
	return len(g.neighborIndices(i)), true
}

// Neighbors returns the distinct neighbours of key in dense index order, self
// excluded, or nil when key is absent. On a directed graph it is the union of
// key's out- and in-neighbours.
func (g *Graph) Neighbors(key string) []string {
	i, ok := g.index[key]
	if !ok {
		return nil
	}
	idx := g.neighborIndices(i)
	out := make([]string, len(idx))
	for k, j := range idx {
		out[k] = g.keys[j]
	}
	return out
}

// neighborIndices returns node i's distinct neighbour indices in ascending
// order. The undirected case is a view into the CSR; the directed case merges
// the sorted out- and in-lists into a new slice.
func (g *Graph) neighborIndices(i int) []int {
	out := g.model.neighbors(i)
	if !g.model.directed {
		return out
	}
	in := g.model.inNeighbors(i)
	merged := make([]int, 0, len(out)+len(in))
	a, b := 0, 0
	for a < len(out) || b < len(in) {
		switch {
		case b == len(in) || (a < len(out) && out[a] < in[b]):
			merged = append(merged, out[a])
			a++
		case a == len(out) || in[b] < out[a]:
			merged = append(merged, in[b])
			b++
		default:
			merged = append(merged, out[a])
			a++
			b++
		}
	}
	return merged
}

// Weight returns the folded weight of the edge between a and b and whether
// such an edge exists. On a directed graph it is the arc a -> b, so Weight(a,
// b) and Weight(b, a) may differ. Weight(a, a) reports a's folded self-loop
// weight, present when positive. An unknown key yields (0, false).
func (g *Graph) Weight(a, b string) (float64, bool) {
	i, ok := g.index[a]
	if !ok {
		return 0, false
	}
	j, ok := g.index[b]
	if !ok {
		return 0, false
	}
	if i == j {
		w := g.model.selfLoops[i]
		return w, w > 0
	}
	nb := g.model.neighbors(i)
	k, found := slices.BinarySearch(nb, j)
	if !found {
		return 0, false
	}
	return g.model.neighborWeights(i)[k], true
}
