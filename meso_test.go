package meso_test

import (
	"testing"

	"github.com/andreswebs/meso"
)

// TestBuilder_PublicAPI exercises the public Builder surface from an external
// package, confirming a consumer can build a graph, round-trip community labels
// back to caller keys, and observe validation errors, using only exported
// symbols.
func TestBuilder_PublicAPI(t *testing.T) {
	g, err := meso.NewBuilder().
		AddEdge("alice", "bob", 1.0).
		AddEdge("bob", "carol", 2.0).
		AddNodeWeight("dave", 3.0).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}

	if got := g.NumNodes(); got != 4 {
		t.Fatalf("NumNodes() = %d, want 4", got)
	}

	// Round-trip a dense index back to its caller key.
	i, ok := g.Index("carol")
	if !ok {
		t.Fatalf("Index(%q) not found", "carol")
	}
	if got := g.Key(i); got != "carol" {
		t.Errorf("Key(%d) = %q, want %q", i, got, "carol")
	}

	if _, err := meso.NewBuilder().AddEdge("a", "b", -1.0).Build(); err == nil {
		t.Errorf("negative edge weight: Build() error = nil, want error")
	}
}

// twoTrianglesGraph builds two triangles joined by a single bridge edge, a graph
// with two obvious communities: {a0,a1,a2} and {b0,b1,b2}, bridged a2-b0.
func twoTrianglesGraph(t *testing.T) *meso.Graph {
	t.Helper()
	g, err := meso.NewBuilder().
		AddEdge("a0", "a1", 1).AddEdge("a1", "a2", 1).AddEdge("a0", "a2", 1).
		AddEdge("b0", "b1", 1).AddEdge("b1", "b2", 1).AddEdge("b0", "b2", 1).
		AddEdge("a2", "b0", 1).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	return g
}

// numCommunitiesOf counts the distinct community labels in a caller-key mapping.
func numCommunitiesOf(comm map[string]int) int {
	seen := make(map[int]struct{}, len(comm))
	for _, c := range comm {
		seen[c] = struct{}{}
	}
	return len(seen)
}

// TestLeiden_PublicAPI is acceptance criterion 5: the public Leiden entrypoint
// returns communities keyed by the caller's keys and the achieved quality, and is
// byte-identical at a fixed seed. It recovers the two triangles as separate
// communities and, run twice at the same seed, produces the identical community
// map and quality.
func TestLeiden_PublicAPI(t *testing.T) {
	g := twoTrianglesGraph(t)

	res, err := meso.Leiden(g, meso.WithQuality(meso.Modularity(1.0)), meso.WithSeed(42))
	if err != nil {
		t.Fatalf("Leiden() error = %v", err)
	}

	comm := res.Communities()
	if len(comm) != 6 {
		t.Fatalf("Communities() has %d keys, want 6", len(comm))
	}
	for _, k := range []string{"a0", "a1", "a2", "b0", "b1", "b2"} {
		if _, ok := comm[k]; !ok {
			t.Errorf("Communities() missing caller key %q", k)
		}
	}
	if numCommunitiesOf(comm) != 2 {
		t.Fatalf("Communities() has %d communities, want 2 (the two triangles)", numCommunitiesOf(comm))
	}
	if comm["a0"] != comm["a1"] || comm["a1"] != comm["a2"] {
		t.Errorf("triangle A split across communities: %v", comm)
	}
	if comm["b0"] != comm["b1"] || comm["b1"] != comm["b2"] {
		t.Errorf("triangle B split across communities: %v", comm)
	}
	if comm["a0"] == comm["b0"] {
		t.Errorf("the two triangles collapsed into one community: %v", comm)
	}

	if res.Quality() <= 0 {
		t.Errorf("Quality() = %v, want a positive modularity for this structure", res.Quality())
	}

	// Byte-identical at a fixed seed.
	res2, err := meso.Leiden(g, meso.WithQuality(meso.Modularity(1.0)), meso.WithSeed(42))
	if err != nil {
		t.Fatalf("Leiden() second run error = %v", err)
	}
	if res2.Quality() != res.Quality() {
		t.Errorf("quality differs across runs at fixed seed: %v vs %v", res.Quality(), res2.Quality())
	}
	for k, c := range comm {
		if res2.Communities()[k] != c {
			t.Errorf("community of %q differs across runs at fixed seed", k)
		}
	}
}

// TestLeiden_WithResolution confirms WithResolution overrides the resolution
// parameter of the selected quality function: at a tiny gamma the null-model
// penalty vanishes and every node collapses into a single community.
func TestLeiden_WithResolution(t *testing.T) {
	g := twoTrianglesGraph(t)

	res, err := meso.Leiden(g, meso.WithQuality(meso.Modularity(1.0)), meso.WithResolution(1e-6))
	if err != nil {
		t.Fatalf("Leiden() error = %v", err)
	}
	if n := numCommunitiesOf(res.Communities()); n != 1 {
		t.Fatalf("at gamma ~ 0 Leiden found %d communities, want 1 (all merged)", n)
	}
}

// TestLeiden_Defaults confirms Leiden runs with no options: the default quality
// is modularity at gamma 1 and the default seed is fixed, so the result is valid
// and reproducible.
func TestLeiden_Defaults(t *testing.T) {
	g := twoTrianglesGraph(t)
	res, err := meso.Leiden(g)
	if err != nil {
		t.Fatalf("Leiden() error = %v", err)
	}
	if numCommunitiesOf(res.Communities()) != 2 {
		t.Errorf("default Leiden found %d communities, want 2", numCommunitiesOf(res.Communities()))
	}
}

// TestLeiden_DirectedRequiresDirectedModularity confirms a directed graph must be
// run with DirectedModularity: the default (undirected) modularity and CPM are
// rejected because their symmetric null models mis-score directed arcs, while
// DirectedModularity is accepted.
func TestLeiden_DirectedRequiresDirectedModularity(t *testing.T) {
	g, err := meso.NewDirectedBuilder().AddEdge("a", "b", 1).AddEdge("b", "a", 1).Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if _, err := meso.Leiden(g); err == nil {
		t.Error("Leiden() on a directed graph with default modularity: error = nil, want error")
	}
	if _, err := meso.Leiden(g, meso.WithQuality(meso.CPM(1.0))); err == nil {
		t.Error("Leiden() on a directed graph with CPM: error = nil, want error")
	}
	if _, err := meso.Leiden(g, meso.WithQuality(meso.DirectedModularity(1.0))); err != nil {
		t.Errorf("Leiden() on a directed graph with DirectedModularity: error = %v, want nil", err)
	}
}

// TestLouvain_PublicAPI confirms the public Louvain entrypoint shares the Result
// and option surface and recovers the two triangles.
func TestLouvain_PublicAPI(t *testing.T) {
	g := twoTrianglesGraph(t)
	res, err := meso.Louvain(g, meso.WithQuality(meso.Modularity(1.0)))
	if err != nil {
		t.Fatalf("Louvain() error = %v", err)
	}
	if numCommunitiesOf(res.Communities()) != 2 {
		t.Errorf("Louvain found %d communities, want 2", numCommunitiesOf(res.Communities()))
	}
}
