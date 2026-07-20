package gonum_test

import (
	"strconv"
	"testing"

	"github.com/andreswebs/meso"
	mesogonum "github.com/andreswebs/meso/gonum"
	"gonum.org/v1/gonum/graph/simple"
)

// twoTriangleEdges describes two triangles joined by a single bridge, the same
// two-community fixture the core's public tests use, expressed over int64 gonum
// node IDs so the same structure can be built on both sides of the adapter.
var twoTriangleEdges = []struct {
	from, to int64
	weight   float64
}{
	{0, 1, 1}, {1, 2, 1}, {0, 2, 1},
	{3, 4, 1}, {4, 5, 1}, {3, 5, 1},
	{2, 3, 1},
}

// buildGonumUndirected builds a weighted undirected gonum graph from edges.
func buildGonumUndirected(edges []struct {
	from, to int64
	weight   float64
}) *simple.WeightedUndirectedGraph {
	g := simple.NewWeightedUndirectedGraph(0, 0)
	for _, e := range edges {
		g.SetWeightedEdge(g.NewWeightedEdge(simple.Node(e.from), simple.Node(e.to), e.weight))
	}
	return g
}

// buildMesoUndirected builds the same graph directly through the meso Builder,
// keying nodes by the decimal string of the gonum ID, matching what the adapter
// does. Canonical() makes both a pure function of the key set and weights.
func buildMesoUndirected(t *testing.T, edges []struct {
	from, to int64
	weight   float64
}) *meso.Graph {
	t.Helper()
	b := meso.NewBuilder().Canonical()
	for _, e := range edges {
		b.AddEdge(strconv.FormatInt(e.from, 10), strconv.FormatInt(e.to, 10), e.weight)
	}
	g, err := b.Build()
	if err != nil {
		t.Fatalf("buildMesoUndirected: Build() error = %v", err)
	}
	return g
}

// TestBuild_UndirectedMatchesCore is the tracer bullet: a gonum graph converts to
// a meso build whose deterministic Louvain result is identical to the same graph
// built directly through the core Builder. Identical partitions and quality prove
// edges and weights survive the conversion (acceptance criteria 1 and 2).
func TestBuild_UndirectedMatchesCore(t *testing.T) {
	adapted, err := mesogonum.Build(buildGonumUndirected(twoTriangleEdges))
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	direct := buildMesoUndirected(t, twoTriangleEdges)

	if adapted.NumNodes() != direct.NumNodes() {
		t.Fatalf("NumNodes: adapter = %d, core = %d", adapted.NumNodes(), direct.NumNodes())
	}
	if adapted.Directed() {
		t.Errorf("Directed() = true, want false for an undirected gonum graph")
	}

	adaptedRes, err := meso.Louvain(adapted)
	if err != nil {
		t.Fatalf("Louvain(adapted) error = %v", err)
	}
	directRes, err := meso.Louvain(direct)
	if err != nil {
		t.Fatalf("Louvain(direct) error = %v", err)
	}
	if adaptedRes.Quality() != directRes.Quality() {
		t.Errorf("quality: adapter = %v, core = %v", adaptedRes.Quality(), directRes.Quality())
	}
	for k, c := range directRes.Communities() {
		if adaptedRes.Communities()[k] != c {
			t.Errorf("community of key %q: adapter = %d, core = %d", k, adaptedRes.Communities()[k], c)
		}
	}
}

// TestCommunities_MapsBackToNodeIDs covers the "and back" half of the round-trip:
// a partition detected on the converted graph maps back onto the original gonum
// int64 node IDs, recovering the two planted triangles (acceptance criterion 1).
func TestCommunities_MapsBackToNodeIDs(t *testing.T) {
	g, err := mesogonum.Build(buildGonumUndirected(twoTriangleEdges))
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	res, err := meso.Leiden(g, meso.WithSeed(42))
	if err != nil {
		t.Fatalf("Leiden() error = %v", err)
	}

	comm, err := mesogonum.Communities(res)
	if err != nil {
		t.Fatalf("Communities() error = %v", err)
	}
	if len(comm) != 6 {
		t.Fatalf("Communities() has %d node IDs, want 6", len(comm))
	}
	for id := range int64(6) {
		if _, ok := comm[id]; !ok {
			t.Errorf("Communities() missing gonum node ID %d", id)
		}
	}
	if comm[0] != comm[1] || comm[1] != comm[2] {
		t.Errorf("triangle {0,1,2} split across communities: %v", comm)
	}
	if comm[3] != comm[4] || comm[4] != comm[5] {
		t.Errorf("triangle {3,4,5} split across communities: %v", comm)
	}
	if comm[0] == comm[3] {
		t.Errorf("the two triangles collapsed into one community: %v", comm)
	}
}

// directedTwoTriangleEdges is two triangles with reciprocal intra-triangle arcs
// and a single directed bridge 2 -> 3: a directed two-community fixture.
var directedTwoTriangleEdges = []struct {
	from, to int64
	weight   float64
}{
	{0, 1, 1}, {1, 0, 1}, {1, 2, 1}, {2, 1, 1}, {0, 2, 1}, {2, 0, 1},
	{3, 4, 1}, {4, 3, 1}, {4, 5, 1}, {5, 4, 1}, {3, 5, 1}, {5, 3, 1},
	{2, 3, 1},
}

func buildGonumDirected(edges []struct {
	from, to int64
	weight   float64
}) *simple.WeightedDirectedGraph {
	g := simple.NewWeightedDirectedGraph(0, 0)
	for _, e := range edges {
		g.SetWeightedEdge(g.NewWeightedEdge(simple.Node(e.from), simple.Node(e.to), e.weight))
	}
	return g
}

// TestBuild_Directed confirms a graph implementing graph.Directed builds a
// directed meso graph and its communities map back onto the gonum node IDs,
// recovering the two triangles under directed modularity.
func TestBuild_Directed(t *testing.T) {
	g, err := mesogonum.Build(buildGonumDirected(directedTwoTriangleEdges))
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if !g.Directed() {
		t.Fatalf("Directed() = false, want true for a directed gonum graph")
	}
	if g.NumNodes() != 6 {
		t.Fatalf("NumNodes() = %d, want 6", g.NumNodes())
	}

	res, err := meso.Leiden(g, meso.WithQuality(meso.DirectedModularity(1.0)), meso.WithSeed(1))
	if err != nil {
		t.Fatalf("Leiden(directed) error = %v", err)
	}
	comm, err := mesogonum.Communities(res)
	if err != nil {
		t.Fatalf("Communities() error = %v", err)
	}
	if comm[0] != comm[1] || comm[1] != comm[2] {
		t.Errorf("directed triangle {0,1,2} split: %v", comm)
	}
	if comm[3] != comm[4] || comm[4] != comm[5] {
		t.Errorf("directed triangle {3,4,5} split: %v", comm)
	}
	if comm[0] == comm[3] {
		t.Errorf("directed triangles collapsed into one community: %v", comm)
	}
}

// TestBuild_WeightsAreUsed proves the adapter propagates real edge weights rather
// than treating every edge as unit weight: the same topology at weight 5 scores a
// different modularity than at weight 1.
func TestBuild_WeightsAreUsed(t *testing.T) {
	heavy := make([]struct {
		from, to int64
		weight   float64
	}, len(twoTriangleEdges))
	copy(heavy, twoTriangleEdges)
	for i := range heavy {
		heavy[i].weight = 5
	}

	weighted, err := mesogonum.Build(buildGonumUndirected(heavy))
	if err != nil {
		t.Fatalf("Build(weighted) error = %v", err)
	}
	weightedRes, err := meso.Louvain(weighted)
	if err != nil {
		t.Fatalf("Louvain(weighted) error = %v", err)
	}
	unitRes, err := meso.Louvain(buildMesoUndirected(t, twoTriangleEdges))
	if err != nil {
		t.Fatalf("Louvain(unit) error = %v", err)
	}
	if weightedRes.Quality() == unitRes.Quality() {
		t.Errorf("weighted and unit-weight quality both %v: weights were dropped", weightedRes.Quality())
	}
}

// TestBuild_RejectsNegativeWeight confirms the adapter surfaces the core
// Builder's validation: a negative gonum edge weight fails the build.
func TestBuild_RejectsNegativeWeight(t *testing.T) {
	g := simple.NewWeightedUndirectedGraph(0, 0)
	g.SetWeightedEdge(g.NewWeightedEdge(simple.Node(0), simple.Node(1), -1))
	if _, err := mesogonum.Build(g); err == nil {
		t.Error("Build() with a negative edge weight: error = nil, want error")
	}
}

// TestBuild_PreservesIsolatedNode confirms a node with no incident edges survives
// the conversion and appears in the mapped-back communities.
func TestBuild_PreservesIsolatedNode(t *testing.T) {
	g := simple.NewWeightedUndirectedGraph(0, 0)
	g.SetWeightedEdge(g.NewWeightedEdge(simple.Node(0), simple.Node(1), 1))
	g.AddNode(simple.Node(2))

	mg, err := mesogonum.Build(g)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if mg.NumNodes() != 3 {
		t.Fatalf("NumNodes() = %d, want 3 (isolated node dropped)", mg.NumNodes())
	}
	res, err := meso.Louvain(mg)
	if err != nil {
		t.Fatalf("Louvain() error = %v", err)
	}
	comm, err := mesogonum.Communities(res)
	if err != nil {
		t.Fatalf("Communities() error = %v", err)
	}
	if _, ok := comm[2]; !ok {
		t.Errorf("isolated node ID 2 missing from communities: %v", comm)
	}
}

// TestCommunities_RejectsForeignResult confirms Communities reports an error
// rather than silently mismapping a Result whose keys are not gonum node IDs.
func TestCommunities_RejectsForeignResult(t *testing.T) {
	g, err := meso.NewBuilder().AddEdge("alice", "bob", 1).Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	res, err := meso.Louvain(g)
	if err != nil {
		t.Fatalf("Louvain() error = %v", err)
	}
	if _, err := mesogonum.Communities(res); err == nil {
		t.Error("Communities() on non-numeric keys: error = nil, want error")
	}
}

// TestBuild_EmptyGraph confirms a degenerate empty gonum graph converts to a
// valid empty meso graph rather than panicking.
func TestBuild_EmptyGraph(t *testing.T) {
	mg, err := mesogonum.Build(simple.NewWeightedUndirectedGraph(0, 0))
	if err != nil {
		t.Fatalf("Build(empty) error = %v", err)
	}
	if mg.NumNodes() != 0 {
		t.Errorf("NumNodes() = %d, want 0 for an empty graph", mg.NumNodes())
	}
}
