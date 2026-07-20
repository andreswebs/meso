package meso

import (
	"fmt"
	"sort"
)

// Builder is the public entry point for constructing a Graph from caller data.
// It maps arbitrary string keys to dense node indices in first-seen order,
// accumulates weighted edges (folding parallel edges and self-loops), and
// records optional node weights. Build produces the immutable Graph.
//
// A Builder is not safe for concurrent use. Its methods return the Builder so
// calls can be chained; input validation is deferred to Build, which returns
// the first error encountered.
type Builder struct {
	directed  bool
	canonical bool
	index     map[string]int
	keys      []string
	edges     map[[2]int]float64
	selfLoops map[int]float64
	sizes     map[int]float64
	err       error
}

// NewBuilder returns a Builder for an undirected graph.
func NewBuilder() *Builder {
	return newBuilder(false)
}

// NewDirectedBuilder returns a Builder for a directed graph. The resulting
// Graph keeps in-arcs and out-arcs separable (the foundation for directed
// modularity in M3); AddEdge(from, to, w) records an arc from -> to.
func NewDirectedBuilder() *Builder {
	return newBuilder(true)
}

func newBuilder(directed bool) *Builder {
	return &Builder{
		directed:  directed,
		index:     make(map[string]int),
		edges:     make(map[[2]int]float64),
		selfLoops: make(map[int]float64),
		sizes:     make(map[int]float64),
	}
}

// intern returns the dense index for key, assigning the next index in
// first-seen order when the key is new.
func (b *Builder) intern(key string) int {
	if i, ok := b.index[key]; ok {
		return i
	}
	i := len(b.keys)
	b.index[key] = i
	b.keys = append(b.keys, key)
	return i
}

// Canonical makes Build assign dense node indices by ascending caller key
// rather than first-seen order, so the resulting Graph is a pure function of the
// key set and edge weights - independent of the order edges were inserted. This
// is the order-independence half of the determinism model (design section 4.4):
// two callers who insert the same edges in different orders get byte-identical
// models and therefore identical partitions. It returns the Builder for
// chaining and must be set before Build.
func (b *Builder) Canonical() *Builder {
	b.canonical = true
	return b
}

// AddEdge adds a weighted edge between from and to. Repeated calls on the same
// endpoint pair sum their weights into a single edge; for an undirected builder
// the pair is unordered, for a directed builder the arc from -> to is distinct
// from to -> from. A self-loop (from == to) is recorded as node-internal weight
// rather than a neighbour entry. Both endpoints are registered as nodes.
//
// A negative or NaN weight is rejected: Build will return an error.
func (b *Builder) AddEdge(from, to string, weight float64) *Builder {
	if b.err != nil {
		return b
	}
	if !(weight >= 0) {
		b.err = fmt.Errorf("meso: edge (%q, %q) weight %v must be non-negative", from, to, weight)
		return b
	}
	i := b.intern(from)
	j := b.intern(to)
	if i == j {
		b.selfLoops[i] += weight
		return b
	}
	b.edges[b.edgeKey(i, j)] += weight
	return b
}

// edgeKey canonicalises an endpoint pair: unordered (min, max) for an
// undirected builder, ordered (from, to) for a directed one.
func (b *Builder) edgeKey(i, j int) [2]int {
	if !b.directed && i > j {
		i, j = j, i
	}
	return [2]int{i, j}
}

// AddNodeWeight sets the node weight (size) of key, registering it as a node if
// unseen. Nodes without an explicit weight default to size 1.0. A negative or
// NaN size is rejected: Build will return an error.
func (b *Builder) AddNodeWeight(key string, size float64) *Builder {
	if b.err != nil {
		return b
	}
	if !(size >= 0) {
		b.err = fmt.Errorf("meso: node %q weight %v must be non-negative", key, size)
		return b
	}
	b.sizes[b.intern(key)] = size
	return b
}

// arc is one accumulated adjacency entry: an edge to dst with weight w.
type arc struct {
	dst int
	w   float64
}

// Build validates the accumulated input and produces the immutable Graph. It
// returns an error if any edge or node weight was negative or NaN. Degenerate
// inputs (empty, single node, isolated nodes, disconnected components) build a
// valid graph.
func (b *Builder) Build() (*Graph, error) {
	if b.err != nil {
		return nil, b.err
	}

	n := len(b.keys)

	// pos maps each first-seen index to its final dense index: the identity by
	// default, and the ascending-key permutation under Canonical() so the layout
	// no longer depends on insertion order. Everything below is written through
	// pos, so the default path is unchanged.
	pos := b.densePositions()

	nodeSizes := make([]float64, n)
	for i := range nodeSizes {
		nodeSizes[i] = 1.0
	}
	for i, s := range b.sizes {
		nodeSizes[pos[i]] = s
	}

	selfLoops := make([]float64, n)
	for i, w := range b.selfLoops {
		selfLoops[pos[i]] = w
	}

	outLists := make([][]arc, n)
	var inLists [][]arc
	if b.directed {
		inLists = make([][]arc, n)
	}
	for pair, w := range b.edges {
		i, j := pos[pair[0]], pos[pair[1]]
		outLists[i] = append(outLists[i], arc{dst: j, w: w})
		if b.directed {
			inLists[j] = append(inLists[j], arc{dst: i, w: w})
		} else {
			outLists[j] = append(outLists[j], arc{dst: i, w: w})
		}
	}

	g := &csr{
		n:         n,
		directed:  b.directed,
		out:       flattenAdjacency(outLists, n),
		selfLoops: selfLoops,
		nodeSizes: nodeSizes,
	}
	if b.directed {
		g.in = flattenAdjacency(inLists, n)
	}

	if err := g.checkInvariants(); err != nil {
		return nil, err
	}

	index := make(map[string]int, n)
	for k, i := range b.index {
		index[k] = pos[i]
	}
	keys := make([]string, n)
	for i, k := range b.keys {
		keys[pos[i]] = k
	}

	return &Graph{model: g, keys: keys, index: index}, nil
}

// densePositions returns the mapping from first-seen index to final dense
// index. Without Canonical() it is the identity (first-seen order preserved);
// with it, the permutation that ranks nodes by ascending key, so the dense
// labelling is independent of insertion order.
func (b *Builder) densePositions() []int {
	n := len(b.keys)
	pos := make([]int, n)
	for i := range pos {
		pos[i] = i
	}
	if !b.canonical {
		return pos
	}
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(x, y int) bool { return b.keys[order[x]] < b.keys[order[y]] })
	for rank, first := range order {
		pos[first] = rank
	}
	return pos
}

// flattenAdjacency turns per-node arc lists into a CSR adjacency, sorting each
// node's neighbours by index so the layout is canonical (order-independent of
// map iteration and insertion order).
func flattenAdjacency(lists [][]arc, n int) adjacency {
	offsets := make([]int, n+1)
	for i := range n {
		sort.Slice(lists[i], func(x, y int) bool { return lists[i][x].dst < lists[i][y].dst })
		offsets[i+1] = offsets[i] + len(lists[i])
	}
	m := offsets[n]
	neighbors := make([]int, m)
	weights := make([]float64, m)
	k := 0
	for i := range n {
		for _, a := range lists[i] {
			neighbors[k] = a.dst
			weights[k] = a.w
			k++
		}
	}
	return adjacency{offsets: offsets, neighbors: neighbors, weights: weights}
}

// Graph is the immutable weighted graph produced by a Builder. It wraps the
// internal CSR model over dense indices [0, n) together with the reverse
// index-to-key mapping needed to round-trip community labels back to the
// caller's keys.
type Graph struct {
	model *csr
	keys  []string
	index map[string]int
}

// NumNodes reports the number of nodes in the graph.
func (g *Graph) NumNodes() int { return g.model.n }

// Directed reports whether the graph was built as directed.
func (g *Graph) Directed() bool { return g.model.directed }

// Key returns the caller key mapped to dense index i. It panics if i is out of
// range [0, NumNodes).
func (g *Graph) Key(i int) string { return g.keys[i] }

// Index returns the dense index assigned to key and whether key is present.
func (g *Graph) Index(key string) (int, bool) {
	i, ok := g.index[key]
	return i, ok
}
