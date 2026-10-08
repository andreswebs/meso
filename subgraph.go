package meso

import (
	"errors"
	"fmt"
)

// ErrUnknownKey reports a key that is not a node of the graph.
var ErrUnknownKey = errors.New("meso: unknown key")

// ErrDuplicateKey reports a key listed more than once where a set is expected.
var ErrDuplicateKey = errors.New("meso: duplicate key")

// Subgraph returns the graph induced by keys: those nodes with their node
// weights and self-loops, and every edge (or arc) of g between two of them
// with its folded weight. The result has g's directedness and is always
// canonically indexed (ascending key order, see [Builder.Canonical]) whatever
// g's indexing, so anything computed on it is a pure function of the key set.
//
// A key that is not in g yields an error wrapping [ErrUnknownKey]; a key listed
// twice yields an error wrapping [ErrDuplicateKey]. An empty keys builds the
// empty graph.
func Subgraph(g *Graph, keys []string) (*Graph, error) {
	member := make(map[int]bool, len(keys))
	for _, k := range keys {
		i, ok := g.index[k]
		if !ok {
			return nil, fmt.Errorf("%w %q", ErrUnknownKey, k)
		}
		if member[i] {
			return nil, fmt.Errorf("%w %q", ErrDuplicateKey, k)
		}
		member[i] = true
	}

	m := g.model
	b := newBuilder(m.directed).Canonical()
	for _, k := range keys {
		i := g.index[k]
		b.AddNodeWeight(k, m.nodeSize(i))
		if w := m.selfLoops[i]; w > 0 {
			b.AddEdge(k, k, w)
		}
		for n, j := range m.neighbors(i) {
			// An undirected edge appears in both endpoints' lists and AddEdge
			// would fold it twice; take it once, from its lower endpoint.
			if !member[j] || (!m.directed && j < i) {
				continue
			}
			b.AddEdge(k, g.keys[j], m.neighborWeights(i)[n])
		}
	}
	return b.Build()
}
