package meso

import "testing"

func TestBuilder_StableIndicesAndRoundTrip(t *testing.T) {
	g, err := NewBuilder().
		AddEdge("a", "b", 1.0).
		AddEdge("b", "c", 2.0).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}

	if got := g.NumNodes(); got != 3 {
		t.Fatalf("NumNodes() = %d, want 3", got)
	}

	// Indices are assigned in first-seen order: a, b, c.
	wantIndex := map[string]int{"a": 0, "b": 1, "c": 2}
	for key, want := range wantIndex {
		got, ok := g.Index(key)
		if !ok || got != want {
			t.Errorf("Index(%q) = (%d, %v), want (%d, true)", key, got, ok, want)
		}
		if k := g.Key(want); k != key {
			t.Errorf("Key(%d) = %q, want %q", want, k, key)
		}
	}

	if _, ok := g.Index("missing"); ok {
		t.Errorf("Index(%q) ok = true, want false", "missing")
	}
}

// edgeWeight returns the off-diagonal weight stored from node i to node j in
// the out adjacency, or 0 if j is not a neighbour of i.
func edgeWeight(g *csr, i, j int) float64 {
	nbrs := g.neighbors(i)
	w := g.neighborWeights(i)
	for k, nb := range nbrs {
		if nb == j {
			return w[k]
		}
	}
	return 0
}

func TestBuilder_ParallelEdgesSum(t *testing.T) {
	// Three insertions on the same unordered pair, one of them reversed, fold
	// into a single edge of the summed weight, symmetric on both endpoints.
	g, err := NewBuilder().
		AddEdge("a", "b", 1.0).
		AddEdge("a", "b", 2.0).
		AddEdge("b", "a", 3.0).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}

	a, _ := g.Index("a")
	b, _ := g.Index("b")

	if got := len(g.model.neighbors(a)); got != 1 {
		t.Fatalf("node a neighbour count = %d, want 1 (folded)", got)
	}
	if got := edgeWeight(g.model, a, b); got != 6 {
		t.Errorf("edge weight a-b = %v, want 6", got)
	}
	if got := edgeWeight(g.model, b, a); got != 6 {
		t.Errorf("edge weight b-a = %v, want 6", got)
	}
}

func TestBuilder_SelfLoopIsNodeInternal(t *testing.T) {
	// A self-loop is node-internal weight, not a neighbour entry, and is visible
	// to twoM: off-diagonal weight 3 counts twice (both endpoints), the self-loop
	// weight 5 counts once, so twoM = 6 + 5 = 11.
	g, err := NewBuilder().
		AddEdge("x", "x", 5.0).
		AddEdge("x", "y", 3.0).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}

	x, _ := g.Index("x")
	if got := len(g.model.neighbors(x)); got != 1 {
		t.Errorf("node x neighbour count = %d, want 1 (self-loop excluded)", got)
	}
	for _, nb := range g.model.neighbors(x) {
		if nb == x {
			t.Errorf("self-loop leaked into neighbour list of x")
		}
	}
	if got := g.model.degree(x); got != 8 {
		t.Errorf("degree(x) = %v, want 8", got)
	}
	if got := g.model.twoM(); got != 11 {
		t.Errorf("twoM() = %v, want 11", got)
	}
}

func TestBuilder_NodeWeights(t *testing.T) {
	// AddNodeWeight sets a size and registers a node that has no edges (isolated),
	// while unset nodes default to 1.0.
	g, err := NewBuilder().
		AddEdge("a", "b", 1.0).
		AddNodeWeight("a", 2.5).
		AddNodeWeight("c", 4.0).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}

	if got := g.NumNodes(); got != 3 {
		t.Fatalf("NumNodes() = %d, want 3 (isolated c registered)", got)
	}

	a, _ := g.Index("a")
	b, _ := g.Index("b")
	c, _ := g.Index("c")

	if got := g.model.nodeSize(a); got != 2.5 {
		t.Errorf("nodeSize(a) = %v, want 2.5", got)
	}
	if got := g.model.nodeSize(b); got != 1.0 {
		t.Errorf("nodeSize(b) = %v, want 1.0 (default)", got)
	}
	if got := g.model.nodeSize(c); got != 4.0 {
		t.Errorf("nodeSize(c) = %v, want 4.0", got)
	}
	if got := g.model.degree(c); got != 0 {
		t.Errorf("degree(c) = %v, want 0 (isolated)", got)
	}
}

func TestBuilder_NegativeWeightsRejected(t *testing.T) {
	if _, err := NewBuilder().AddEdge("a", "b", -1.0).Build(); err == nil {
		t.Errorf("negative edge weight: Build() error = nil, want error")
	}
	if _, err := NewBuilder().AddNodeWeight("a", -1.0).Build(); err == nil {
		t.Errorf("negative node weight: Build() error = nil, want error")
	}
	// The first error is retained even if later calls are valid.
	if _, err := NewBuilder().AddEdge("a", "b", -1.0).AddEdge("b", "c", 1.0).Build(); err == nil {
		t.Errorf("negative then valid: Build() error = nil, want error")
	}
}

func TestBuilder_ZeroWeightEdgeDropped(t *testing.T) {
	// Zero is the accepted boundary of the weight >= 0 guard, so a zero-weight
	// edge is not an error, but it is not an edge either: Build omits it from the
	// adjacency while still registering both endpoints. A zero-size node keeps
	// size 0.
	g, err := NewBuilder().
		AddEdge("a", "b", 0.0).
		AddNodeWeight("c", 0.0).
		Build()
	if err != nil {
		t.Fatalf("zero weights: Build() error = %v, want nil", err)
	}
	if got := g.NumNodes(); got != 3 {
		t.Fatalf("NumNodes() = %d, want 3", got)
	}

	a, _ := g.Index("a")
	b, _ := g.Index("b")
	c, _ := g.Index("c")
	if got := g.model.neighbors(a); len(got) != 0 {
		t.Errorf("neighbors(a) = %v, want none", got)
	}
	if got := g.model.neighbors(b); len(got) != 0 {
		t.Errorf("neighbors(b) = %v, want none", got)
	}
	if got := g.model.nodeSize(c); got != 0 {
		t.Errorf("nodeSize(c) = %v, want 0", got)
	}
}

func TestBuilder_ZeroWeightArcDropped(t *testing.T) {
	// A zero-weight arc leaves neither an out- nor an in-entry; beside it a
	// positive reverse arc survives on its own.
	g, err := NewDirectedBuilder().
		AddEdge("a", "b", 0.0).
		AddEdge("b", "a", 2.0).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	a, _ := g.Index("a")
	b, _ := g.Index("b")
	if got := g.model.neighbors(a); len(got) != 0 {
		t.Errorf("out-neighbors(a) = %v, want none", got)
	}
	if got := g.model.inNeighbors(b); len(got) != 0 {
		t.Errorf("in-neighbors(b) = %v, want none", got)
	}
	if got := g.model.neighbors(b); len(got) != 1 || got[0] != a {
		t.Errorf("out-neighbors(b) = %v, want [%d]", got, a)
	}
	if got := edgeWeight(g.model, b, a); got != 2 {
		t.Errorf("arc b->a weight = %v, want 2", got)
	}
}

func TestBuilder_ZeroParallelFoldsIntoPositive(t *testing.T) {
	g, err := NewBuilder().AddEdge("a", "b", 0).AddEdge("b", "a", 1.5).Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	a, _ := g.Index("a")
	b, _ := g.Index("b")
	if got := g.model.neighbors(a); len(got) != 1 || got[0] != b {
		t.Fatalf("neighbors(a) = %v, want [%d]", got, b)
	}
	if got := edgeWeight(g.model, a, b); got != 1.5 {
		t.Errorf("edge weight = %v, want 1.5", got)
	}
}

func TestBuilder_CanonicalNeighbourOrder(t *testing.T) {
	// Neighbours are sorted by index regardless of insertion order, so the CSR
	// layout is deterministic. Insert d's edges out of index order.
	g, err := NewBuilder().
		AddEdge("d", "c", 1.0).
		AddEdge("d", "a", 1.0).
		AddEdge("d", "b", 1.0).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}
	d, _ := g.Index("d")
	nbrs := g.model.neighbors(d)
	for i := 1; i < len(nbrs); i++ {
		if nbrs[i-1] >= nbrs[i] {
			t.Fatalf("neighbours of d not strictly ascending: %v", nbrs)
		}
	}
}

func TestBuilder_DegenerateShapes(t *testing.T) {
	// Empty graph.
	empty, err := NewBuilder().Build()
	if err != nil {
		t.Fatalf("empty: Build() error = %v", err)
	}
	if empty.NumNodes() != 0 || empty.model.twoM() != 0 {
		t.Errorf("empty: NumNodes=%d twoM=%v, want 0 0", empty.NumNodes(), empty.model.twoM())
	}
	if err := empty.model.checkInvariants(); err != nil {
		t.Errorf("empty: checkInvariants() = %v", err)
	}

	// Single isolated node (registered only via a node weight).
	single, err := NewBuilder().AddNodeWeight("solo", 1.0).Build()
	if err != nil {
		t.Fatalf("single: Build() error = %v", err)
	}
	if single.NumNodes() != 1 || single.model.degree(0) != 0 {
		t.Errorf("single: NumNodes=%d degree=%v, want 1 0", single.NumNodes(), single.model.degree(0))
	}

	// Two disconnected components: a-b and c-d.
	disc, err := NewBuilder().
		AddEdge("a", "b", 1.0).
		AddEdge("c", "d", 1.0).
		Build()
	if err != nil {
		t.Fatalf("disconnected: Build() error = %v", err)
	}
	if disc.NumNodes() != 4 {
		t.Errorf("disconnected: NumNodes = %d, want 4", disc.NumNodes())
	}
	if err := disc.model.checkInvariants(); err != nil {
		t.Errorf("disconnected: checkInvariants() = %v", err)
	}
	if got := disc.model.twoM(); got != 4 {
		t.Errorf("disconnected: twoM() = %v, want 4", got)
	}
}

func TestBuilder_DirectedInOutSeparable(t *testing.T) {
	// Directed: an arc a -> b and a -> c, plus b -> a. Out-arcs and in-arcs are
	// kept separable, and an opposite-direction arc is distinct from its reverse
	// (not folded).
	g, err := NewDirectedBuilder().
		AddEdge("a", "b", 2.0).
		AddEdge("a", "c", 3.0).
		AddEdge("b", "a", 5.0).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}
	if !g.Directed() {
		t.Fatalf("Directed() = false, want true")
	}
	if err := g.model.checkInvariants(); err != nil {
		t.Fatalf("checkInvariants() = %v", err)
	}

	a, _ := g.Index("a")
	b, _ := g.Index("b")

	// a's out-degree: 2 (to b) + 3 (to c) = 5. a's in-degree: 5 (from b).
	if got := g.model.outDegree(a); got != 5 {
		t.Errorf("outDegree(a) = %v, want 5", got)
	}
	if got := g.model.inDegree(a); got != 5 {
		t.Errorf("inDegree(a) = %v, want 5", got)
	}
	// The a<->b arcs are not folded: a->b keeps weight 2, b->a keeps weight 5.
	if got := edgeWeight(g.model, a, b); got != 2 {
		t.Errorf("out arc a->b = %v, want 2", got)
	}
	if got := g.model.outDegree(b); got != 5 {
		t.Errorf("outDegree(b) = %v, want 5 (arc b->a)", got)
	}
	if got := g.model.inDegree(b); got != 2 {
		t.Errorf("inDegree(b) = %v, want 2 (arc a->b)", got)
	}
}

func TestBuilder_ZeroWeightBridgeSeparatesCommunities(t *testing.T) {
	// End-to-end guard: two triangles joined only by a zero-weight edge are two
	// components, so no optimiser may place them in one community. This held
	// before the builder dropped zero-weight edges too (a zero weight adds no
	// quality, and communityConnected already follows only positive weights);
	// it pins that the builder change and the optimisers agree.
	for _, run := range []struct {
		name string
		fn   func(*Graph, ...Option) (*Result, error)
	}{{"Leiden", Leiden}, {"Louvain", Louvain}} {
		g, err := NewBuilder().
			AddEdge("a", "b", 1).AddEdge("b", "c", 1).AddEdge("c", "a", 1).
			AddEdge("x", "y", 1).AddEdge("y", "z", 1).AddEdge("z", "x", 1).
			AddEdge("c", "x", 0).
			Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		res, err := run.fn(g, WithQuality(CPM(0)), WithSeed(1))
		if err != nil {
			t.Fatalf("%s error = %v", run.name, err)
		}
		comm := res.Communities()
		for _, l := range []string{"a", "b", "c"} {
			for _, r := range []string{"x", "y", "z"} {
				if comm[l] == comm[r] {
					t.Errorf("%s: %s and %s share community %d across a zero-weight bridge", run.name, l, r, comm[l])
				}
			}
		}
	}
}
