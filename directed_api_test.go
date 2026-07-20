package meso_test

import (
	"testing"

	"github.com/andreswebs/meso"
)

// sameGrouping reports whether two caller-key community maps induce the same
// partition (the same equivalence classes), ignoring the specific label values.
func sameGrouping(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	keys := make([]string, 0, len(a))
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
		keys = append(keys, k)
	}
	for _, x := range keys {
		for _, y := range keys {
			if (a[x] == a[y]) != (b[x] == b[y]) {
				return false
			}
		}
	}
	return true
}

// twoDirectedCyclesGraph builds two directed 3-cycles joined by a single weak
// bridge arc, a directed graph with two obvious communities: {a0,a1,a2} and
// {b0,b1,b2}. Each group is a directed cycle (a0->a1->a2->a0), so a node reaches
// its community-mates through one out-arc and one in-arc - recovering the groups
// exercises the directed local move and refinement in both arc directions, not
// just the out-arcs. Internal arcs weigh 3, the bridge a2->b0 weighs 1.
func twoDirectedCyclesGraph(t *testing.T) *meso.Graph {
	t.Helper()
	g, err := meso.NewDirectedBuilder().
		AddEdge("a0", "a1", 3).AddEdge("a1", "a2", 3).AddEdge("a2", "a0", 3).
		AddEdge("b0", "b1", 3).AddEdge("b1", "b2", 3).AddEdge("b2", "b0", 3).
		AddEdge("a2", "b0", 1).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	return g
}

// TestLeiden_DirectedFixtureCommunities is acceptance criterion 1: a directed
// fixture recovers its expected communities. The two directed cycles come out as
// two separate communities, with each cycle intact - the hand-verifiable ground
// truth of this construction.
func TestLeiden_DirectedFixtureCommunities(t *testing.T) {
	g := twoDirectedCyclesGraph(t)

	res, err := meso.Leiden(g, meso.WithQuality(meso.DirectedModularity(1.0)), meso.WithSeed(1))
	if err != nil {
		t.Fatalf("Leiden() error = %v", err)
	}
	comm := res.Communities()
	if len(comm) != 6 {
		t.Fatalf("Communities() has %d keys, want 6", len(comm))
	}
	if n := numCommunitiesOf(comm); n != 2 {
		t.Fatalf("directed Leiden found %d communities, want 2 (the two cycles): %v", n, comm)
	}
	if comm["a0"] != comm["a1"] || comm["a1"] != comm["a2"] {
		t.Errorf("cycle A split across communities: %v", comm)
	}
	if comm["b0"] != comm["b1"] || comm["b1"] != comm["b2"] {
		t.Errorf("cycle B split across communities: %v", comm)
	}
	if comm["a0"] == comm["b0"] {
		t.Errorf("the two cycles collapsed into one community: %v", comm)
	}
	if res.Quality() <= 0 {
		t.Errorf("Quality() = %v, want positive directed modularity for this structure", res.Quality())
	}
}

// TestLeiden_DirectedByteIdentical is acceptance criterion 4: a directed run is
// byte-identical at a fixed seed. Two runs at the same seed produce the identical
// community map and quality.
func TestLeiden_DirectedByteIdentical(t *testing.T) {
	g := twoDirectedCyclesGraph(t)
	opts := []meso.Option{meso.WithQuality(meso.DirectedModularity(1.0)), meso.WithSeed(7)}

	res1, err := meso.Leiden(g, opts...)
	if err != nil {
		t.Fatalf("Leiden() error = %v", err)
	}
	res2, err := meso.Leiden(g, opts...)
	if err != nil {
		t.Fatalf("Leiden() second run error = %v", err)
	}
	if res1.Quality() != res2.Quality() {
		t.Errorf("directed quality differs across runs at fixed seed: %v vs %v", res1.Quality(), res2.Quality())
	}
	for k, c := range res1.Communities() {
		if res2.Communities()[k] != c {
			t.Errorf("community of %q differs across runs at fixed seed", k)
		}
	}
}

// edge is one undirected edge in a test fixture, replayed into an undirected
// builder as itself and into a directed builder as the arc pair x->y and y->x so
// out-degree equals in-degree at every node.
type edge struct {
	x, y string
	w    float64
}

// TestLeiden_SymmetricDirectedReducesToUndirected is acceptance criterion 2: on a
// symmetric directed graph, directed Leiden recovers the same communities (the
// same grouping) as undirected Leiden on the undirected original, end to end and
// across seeds, with identical quality.
func TestLeiden_SymmetricDirectedReducesToUndirected(t *testing.T) {
	graphs := []struct {
		name  string
		edges []edge
	}{
		{"two triangles", []edge{
			{"a0", "a1", 1}, {"a1", "a2", 1}, {"a0", "a2", 1},
			{"b0", "b1", 1}, {"b1", "b2", 1}, {"b0", "b2", 1},
			{"a2", "b0", 1},
		}},
		{"weighted path of triangles", []edge{
			{"a0", "a1", 2}, {"a1", "a2", 2}, {"a0", "a2", 2},
			{"b0", "b1", 5}, {"b1", "b2", 5}, {"b0", "b2", 5},
			{"c0", "c1", 3}, {"c1", "c2", 3}, {"c0", "c2", 3},
			{"a2", "b0", 1}, {"b2", "c0", 1},
		}},
	}

	for _, gr := range graphs {
		t.Run(gr.name, func(t *testing.T) {
			ub := meso.NewBuilder()
			db := meso.NewDirectedBuilder()
			for _, e := range gr.edges {
				ub.AddEdge(e.x, e.y, e.w)
				db.AddEdge(e.x, e.y, e.w).AddEdge(e.y, e.x, e.w)
			}
			ug, err := ub.Build()
			if err != nil {
				t.Fatalf("undirected Build() error = %v", err)
			}
			dg, err := db.Build()
			if err != nil {
				t.Fatalf("directed Build() error = %v", err)
			}

			for _, seed := range []uint64{0, 1, 42} {
				uRes, err := meso.Leiden(ug, meso.WithQuality(meso.Modularity(1.0)), meso.WithSeed(seed))
				if err != nil {
					t.Fatalf("undirected Leiden() error = %v", err)
				}
				dRes, err := meso.Leiden(dg, meso.WithQuality(meso.DirectedModularity(1.0)), meso.WithSeed(seed))
				if err != nil {
					t.Fatalf("directed Leiden() error = %v", err)
				}
				if !sameGrouping(uRes.Communities(), dRes.Communities()) {
					t.Fatalf("seed %d: directed communities %v differ from undirected %v",
						seed, dRes.Communities(), uRes.Communities())
				}
				if dRes.Quality() != uRes.Quality() {
					t.Errorf("seed %d: directed quality %v != undirected quality %v (same grouping)",
						seed, dRes.Quality(), uRes.Quality())
				}
			}
		})
	}
}

// TestLouvain_DirectedFixtureCommunities confirms directed Louvain shares the
// directed pipeline and recovers the two cycles as well.
func TestLouvain_DirectedFixtureCommunities(t *testing.T) {
	g := twoDirectedCyclesGraph(t)
	res, err := meso.Louvain(g, meso.WithQuality(meso.DirectedModularity(1.0)))
	if err != nil {
		t.Fatalf("Louvain() error = %v", err)
	}
	if n := numCommunitiesOf(res.Communities()); n != 2 {
		t.Fatalf("directed Louvain found %d communities, want 2: %v", n, res.Communities())
	}
}
