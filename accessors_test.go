package meso

import (
	"slices"
	"testing"
)

func mustBuild(t testing.TB, b *Builder) *Graph {
	t.Helper()
	g, err := b.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	return g
}

func TestGraph_KeysDenseOrder(t *testing.T) {
	g := mustBuild(t, NewBuilder().AddEdge("c", "a", 1).AddEdge("b", "c", 1))
	if got, want := g.Keys(), []string{"c", "a", "b"}; !slices.Equal(got, want) {
		t.Errorf("Keys() = %v, want first-seen order %v", got, want)
	}
	cg := mustBuild(t, NewBuilder().Canonical().AddEdge("c", "a", 1).AddEdge("b", "c", 1))
	if got, want := cg.Keys(), []string{"a", "b", "c"}; !slices.Equal(got, want) {
		t.Errorf("Canonical Keys() = %v, want ascending %v", got, want)
	}
	for i, k := range cg.Keys() {
		if cg.Key(i) != k {
			t.Errorf("Keys()[%d] = %q, Key(%d) = %q", i, k, i, cg.Key(i))
		}
	}
}

func TestGraph_KeysIsACopy(t *testing.T) {
	g := mustBuild(t, NewBuilder().AddEdge("a", "b", 1))
	keys := g.Keys()
	keys[0] = "mutated"
	if g.Key(0) != "a" {
		t.Errorf("mutating Keys() changed the graph: Key(0) = %q", g.Key(0))
	}
}

func TestGraph_NumEdges(t *testing.T) {
	tests := []struct {
		name string
		b    *Builder
		want int
	}{
		{"empty", NewBuilder(), 0},
		{"parallel edges fold", NewBuilder().AddEdge("a", "b", 1).AddEdge("b", "a", 2), 1},
		{"self-loops excluded", NewBuilder().AddEdge("a", "a", 1).AddEdge("a", "b", 1).AddEdge("b", "c", 1), 2},
		{"directed reciprocal arcs count two", NewDirectedBuilder().AddEdge("a", "b", 1).AddEdge("b", "a", 1), 2},
		{"directed parallel arcs fold", NewDirectedBuilder().AddEdge("a", "b", 1).AddEdge("a", "b", 1).AddEdge("a", "a", 3), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mustBuild(t, tt.b).NumEdges(); got != tt.want {
				t.Errorf("NumEdges() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGraph_NeighborsAndDegree(t *testing.T) {
	undirected := mustBuild(t, NewBuilder().Canonical().
		AddEdge("a", "c", 1).AddEdge("a", "b", 1).AddEdge("b", "a", 1).AddEdge("a", "a", 5).AddNodeWeight("z", 1))
	// b -> a and a -> b make a two-way neighbour; c -> a is in-only; a -> d is out-only.
	directed := mustBuild(t, NewDirectedBuilder().Canonical().
		AddEdge("a", "b", 1).AddEdge("b", "a", 1).AddEdge("c", "a", 1).AddEdge("a", "d", 1).AddEdge("a", "a", 1))
	tests := []struct {
		name string
		g    *Graph
		key  string
		want []string
	}{
		{"undirected sorted, self excluded", undirected, "a", []string{"b", "c"}},
		{"undirected leaf", undirected, "c", []string{"a"}},
		{"isolated node", undirected, "z", []string{}},
		{"directed union, two-way counted once", directed, "a", []string{"b", "c", "d"}},
		{"directed in-only target", directed, "d", []string{"a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.g.Neighbors(tt.key)
			if !slices.Equal(got, tt.want) || got == nil {
				t.Errorf("Neighbors(%q) = %#v, want %#v", tt.key, got, tt.want)
			}
			deg, ok := tt.g.Degree(tt.key)
			if !ok || deg != len(tt.want) {
				t.Errorf("Degree(%q) = (%d, %v), want (%d, true)", tt.key, deg, ok, len(tt.want))
			}
		})
	}
}

func TestGraph_NeighborsAndDegreeAbsentKey(t *testing.T) {
	g := mustBuild(t, NewBuilder().AddEdge("a", "b", 1))
	if got := g.Neighbors("nope"); got != nil {
		t.Errorf("Neighbors(absent) = %#v, want nil", got)
	}
	if deg, ok := g.Degree("nope"); deg != 0 || ok {
		t.Errorf("Degree(absent) = (%d, %v), want (0, false)", deg, ok)
	}
}

func TestGraph_Weight(t *testing.T) {
	undirected := mustBuild(t, NewBuilder().
		AddEdge("a", "b", 1).AddEdge("b", "a", 2.5).AddEdge("b", "c", 4).AddEdge("c", "c", 7).AddEdge("a", "a", 0))
	directed := mustBuild(t, NewDirectedBuilder().AddEdge("a", "b", 3).AddEdge("b", "a", 1).AddEdge("b", "c", 2))
	tests := []struct {
		name   string
		g      *Graph
		a, b   string
		want   float64
		wantOK bool
	}{
		{"folded parallel edge", undirected, "a", "b", 3.5, true},
		{"symmetric", undirected, "b", "a", 3.5, true},
		{"other edge", undirected, "c", "b", 4, true},
		{"non-edge", undirected, "a", "c", 0, false},
		{"positive self-loop", undirected, "c", "c", 7, true},
		{"zero self-loop is absent", undirected, "a", "a", 0, false},
		{"no self-loop", undirected, "b", "b", 0, false},
		{"unknown first key", undirected, "x", "a", 0, false},
		{"unknown second key", undirected, "a", "x", 0, false},
		{"arc a->b", directed, "a", "b", 3, true},
		{"arc b->a differs", directed, "b", "a", 1, true},
		{"arc follows direction", directed, "c", "b", 0, false},
		{"arc b->c", directed, "b", "c", 2, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.g.Weight(tt.a, tt.b)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("Weight(%q, %q) = (%v, %v), want (%v, %v)", tt.a, tt.b, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// undirectedCorpusGML lists every undirected GML in datasets/; celegansneural
// is directed and loadGMLGraph would fold its arcs.
var undirectedCorpusGML = []string{
	"datasets/karate/karate.gml",
	"datasets/dolphins/dolphins.gml",
	"datasets/lesmis/lesmis.gml",
	"datasets/football/football.gml",
	"datasets/polbooks/polbooks.gml",
}

func TestGraph_AccessorsReconcileWithCorpus(t *testing.T) {
	for _, path := range undirectedCorpusGML {
		t.Run(path, func(t *testing.T) {
			g := loadGMLGraph(t, path)
			_, edges := parseGML(t, path)

			folded := map[[2]string]float64{}
			for _, e := range edges {
				if e.src == e.tgt {
					continue
				}
				pair := [2]string{min(e.src, e.tgt), max(e.src, e.tgt)}
				folded[pair] += e.w
			}
			for pair, w := range folded {
				if w == 0 {
					delete(folded, pair)
				}
			}
			if got := g.NumEdges(); got != len(folded) {
				t.Errorf("NumEdges() = %d, want %d folded GML pairs", got, len(folded))
			}
			for pair, w := range folded {
				if got, ok := g.Weight(pair[0], pair[1]); !ok || got != w {
					t.Errorf("Weight(%s, %s) = (%v, %v), want (%v, true)", pair[0], pair[1], got, ok, w)
				}
			}

			degreeSum := 0
			for _, k := range g.Keys() {
				deg, ok := g.Degree(k)
				if !ok {
					t.Fatalf("Degree(%q) reports absent for a listed key", k)
				}
				if n := len(g.Neighbors(k)); n != deg {
					t.Errorf("len(Neighbors(%q)) = %d, Degree = %d", k, n, deg)
				}
				degreeSum += deg
			}
			if degreeSum != 2*g.NumEdges() {
				t.Errorf("sum of Degree = %d, want 2*NumEdges = %d", degreeSum, 2*g.NumEdges())
			}
		})
	}
}
