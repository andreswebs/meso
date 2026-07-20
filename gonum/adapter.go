// Package gonum adapts meso's graph builder and partition types to and from
// gonum's graph interfaces (gonum.org/v1/gonum/graph).
//
// It lives in its own module so that gonum is an opt-in dependency: the meso
// core (github.com/andreswebs/meso) never imports it. Convert a gonum graph
// with [Build], run [meso.Leiden] or [meso.Louvain] on the result, then map the
// detected communities back onto the original gonum node IDs with [Communities].
//
// See docs/meso-design.md sections 3 and 9 for the design of record.
package gonum

import (
	"fmt"
	"strconv"

	"github.com/andreswebs/meso"
	"gonum.org/v1/gonum/graph"
)

// key is the meso builder key for a gonum node ID: its base-10 decimal string.
// Communities parses it back with [strconv.ParseInt] to recover the ID.
func key(id int64) string { return strconv.FormatInt(id, 10) }

// Communities maps the community labels of a [meso.Result] back onto the gonum
// node IDs of the graph it was built from, inverting the key scheme [Build]
// uses. Two node IDs share a label exactly when their nodes are in the same
// community. It returns an error if r was produced from a graph whose keys are
// not [Build]-issued node IDs (for example a graph built directly through the
// core with non-numeric keys).
func Communities(r *meso.Result) (map[int64]int, error) {
	labels := r.Communities()
	comm := make(map[int64]int, len(labels))
	for k, c := range labels {
		id, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("gonum: result key %q is not a gonum node ID: %w", k, err)
		}
		comm[id] = c
	}
	return comm, nil
}

// Build converts a gonum graph into an immutable meso [meso.Graph], preserving
// nodes, edges, and weights. A graph implementing [graph.Directed] builds a
// directed meso graph (each out-arc from -> to is recorded once); any other
// graph is treated as undirected (each edge recorded once). Edge weights come
// from a [graph.Weighted] graph or from [graph.WeightedEdge] edges, defaulting
// to 1 for an unweighted graph. Isolated nodes are preserved.
//
// The builder is [meso.Builder.Canonical], so the result is a pure function of
// the graph's node IDs and weights, independent of gonum's node iteration order.
// Build returns the error from [meso.Builder.Build] if any weight is negative or
// NaN.
func Build(g graph.Graph) (*meso.Graph, error) {
	_, directed := g.(graph.Directed)

	var b *meso.Builder
	if directed {
		b = meso.NewDirectedBuilder()
	} else {
		b = meso.NewBuilder()
	}
	b.Canonical()

	// Register every node first so isolated nodes survive the conversion, then
	// walk each node's out-neighbours. For an undirected graph From reports a
	// neighbour from both endpoints, so only the from <= to direction is added
	// to avoid folding an edge's weight in twice.
	nodes := g.Nodes()
	for nodes.Next() {
		b.AddNodeWeight(key(nodes.Node().ID()), 1)
	}
	nodes.Reset()
	for nodes.Next() {
		from := nodes.Node().ID()
		to := g.From(from)
		for to.Next() {
			t := to.Node().ID()
			if !directed && from > t {
				continue
			}
			b.AddEdge(key(from), key(t), weightOf(g, from, t))
		}
	}

	return b.Build()
}

// weightOf returns the weight of the edge from uid to vid, preferring a
// [graph.Weighted] graph's Weight, then a [graph.WeightedEdge], and finally 1
// for an unweighted graph.
func weightOf(g graph.Graph, uid, vid int64) float64 {
	if wg, ok := g.(graph.Weighted); ok {
		if w, ok := wg.Weight(uid, vid); ok {
			return w
		}
	}
	if e := g.Edge(uid, vid); e != nil {
		if we, ok := e.(graph.WeightedEdge); ok {
			return we.Weight()
		}
	}
	return 1
}
