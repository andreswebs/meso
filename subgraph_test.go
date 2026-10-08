package meso

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
)

// assertInduced checks that sub is the subgraph of g induced by keys: same
// directedness, exactly those keys in ascending order, every member pair's edge
// and every member's self-loop and node size carried over, nothing else.
func assertInduced(t *testing.T, g, sub *Graph, keys []string) {
	t.Helper()
	want := slices.Sorted(slices.Values(keys))
	if got := sub.Keys(); !slices.Equal(got, want) {
		t.Fatalf("Keys() = %v, want %v", got, want)
	}
	if sub.Directed() != g.Directed() {
		t.Errorf("Directed() = %v, want %v", sub.Directed(), g.Directed())
	}
	edges := 0
	for _, a := range keys {
		for _, b := range keys {
			gw, gok := g.Weight(a, b)
			sw, sok := sub.Weight(a, b)
			if gw != sw || gok != sok {
				t.Errorf("Weight(%q, %q): subgraph (%v, %v), graph (%v, %v)", a, b, sw, sok, gw, gok)
			}
			if a != b && gok && (g.Directed() || a < b) {
				edges++
			}
		}
		gi, _ := g.Index(a)
		si, _ := sub.Index(a)
		if gs, ss := g.model.nodeSize(gi), sub.model.nodeSize(si); gs != ss {
			t.Errorf("node size of %q: subgraph %v, graph %v", a, ss, gs)
		}
	}
	if got := sub.NumEdges(); got != edges {
		t.Errorf("NumEdges() = %d, want %d induced edges", got, edges)
	}
}

func TestSubgraph_Induced(t *testing.T) {
	g := mustBuild(t, NewBuilder().
		AddEdge("a", "b", 2).AddEdge("b", "c", 3).AddEdge("c", "d", 4).AddEdge("a", "d", 5).
		AddEdge("b", "b", 6).AddNodeWeight("c", 7).AddNodeWeight("e", 0.5))
	keys := []string{"c", "a", "b", "e"}
	sub, err := Subgraph(g, keys)
	if err != nil {
		t.Fatalf("Subgraph() error = %v", err)
	}
	assertInduced(t, g, sub, keys)
	if _, ok := sub.Index("d"); ok {
		t.Errorf("non-member d present in subgraph")
	}
}

func TestSubgraph_Errors(t *testing.T) {
	g := mustBuild(t, NewBuilder().AddEdge("a", "b", 1))
	tests := []struct {
		name string
		keys []string
		want error
		key  string
	}{
		{"unknown key", []string{"a", "zz"}, ErrUnknownKey, "zz"},
		{"duplicate key", []string{"a", "b", "a"}, ErrDuplicateKey, "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub, err := Subgraph(g, tt.keys)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Subgraph() error = %v, want %v", err, tt.want)
			}
			if !strings.Contains(err.Error(), `"`+tt.key+`"`) {
				t.Errorf("error %q does not name key %q", err, tt.key)
			}
			if sub != nil {
				t.Errorf("Subgraph() graph = %v, want nil on error", sub)
			}
		})
	}
}

func TestSubgraph_Empty(t *testing.T) {
	for _, b := range []*Builder{NewBuilder().AddEdge("a", "b", 1), NewDirectedBuilder().AddEdge("a", "b", 1)} {
		g := mustBuild(t, b)
		sub, err := Subgraph(g, nil)
		if err != nil {
			t.Fatalf("Subgraph(nil) error = %v", err)
		}
		if sub.NumNodes() != 0 || sub.NumEdges() != 0 || sub.Directed() != g.Directed() {
			t.Errorf("Subgraph(nil) = %d nodes, %d edges, directed %v", sub.NumNodes(), sub.NumEdges(), sub.Directed())
		}
	}
}

func TestSubgraph_DirectedKeepsArcDirection(t *testing.T) {
	g := mustBuild(t, NewDirectedBuilder().
		AddEdge("a", "b", 2).AddEdge("b", "a", 3).AddEdge("b", "c", 4).AddEdge("c", "d", 5).AddEdge("c", "c", 1))
	keys := []string{"b", "c", "a"}
	sub, err := Subgraph(g, keys)
	if err != nil {
		t.Fatalf("Subgraph() error = %v", err)
	}
	assertInduced(t, g, sub, keys)
}

func TestSubgraph_CanonicalWhateverTheSource(t *testing.T) {
	_, edges := parseGML(t, "datasets/karate/karate.gml")
	forward, backward := NewBuilder(), NewBuilder()
	for _, e := range edges {
		forward.AddEdge(e.src, e.tgt, e.w)
	}
	for _, e := range slices.Backward(edges) {
		backward.AddEdge(e.tgt, e.src, e.w)
	}
	g1, g2 := mustBuild(t, forward), mustBuild(t, backward)
	if slices.Equal(g1.Keys(), g2.Keys()) {
		t.Fatalf("fixture: the two non-canonical builds should index differently")
	}

	members := []string{"34", "3", "9", "10", "14", "15", "16", "19", "20", "21", "23", "24", "27", "28", "29", "30", "31", "32", "33"}
	reversed := slices.Clone(members)
	slices.Reverse(reversed)
	s1, err := Subgraph(g1, members)
	if err != nil {
		t.Fatalf("Subgraph(g1) error = %v", err)
	}
	s2, err := Subgraph(g2, reversed)
	if err != nil {
		t.Fatalf("Subgraph(g2) error = %v", err)
	}
	assertInduced(t, g1, s1, members)
	if !slices.Equal(s1.Keys(), s2.Keys()) {
		t.Fatalf("Keys differ: %v vs %v", s1.Keys(), s2.Keys())
	}
	r1, err := Leiden(s1, WithSeed(3))
	if err != nil {
		t.Fatalf("Leiden(s1) error = %v", err)
	}
	r2, err := Leiden(s2, WithSeed(3))
	if err != nil {
		t.Fatalf("Leiden(s2) error = %v", err)
	}
	if !maps.Equal(r1.Communities(), r2.Communities()) || r1.Quality() != r2.Quality() {
		t.Errorf("Leiden on the two subgraphs differs")
	}
}

func TestSubgraph_IdentityOnCanonicalGraph(t *testing.T) {
	_, edges := parseGML(t, "datasets/lesmis/lesmis.gml")
	b := NewBuilder().Canonical()
	for _, e := range edges {
		b.AddEdge(e.src, e.tgt, e.w)
	}
	g := mustBuild(t, b)
	sub, err := Subgraph(g, g.Keys())
	if err != nil {
		t.Fatalf("Subgraph() error = %v", err)
	}
	assertInduced(t, g, sub, g.Keys())
	r1, err := Leiden(g, WithSeed(5))
	if err != nil {
		t.Fatalf("Leiden(g) error = %v", err)
	}
	r2, err := Leiden(sub, WithSeed(5))
	if err != nil {
		t.Fatalf("Leiden(sub) error = %v", err)
	}
	if !maps.Equal(r1.Communities(), r2.Communities()) || r1.Quality() != r2.Quality() {
		t.Errorf("Leiden on Subgraph(g, g.Keys()) differs from Leiden on g")
	}
}

func TestSubgraph_ResplitLargestCommunity(t *testing.T) {
	g := loadGMLGraph(t, "datasets/karate/karate.gml")
	res, err := Leiden(g, WithSeed(1))
	if err != nil {
		t.Fatalf("Leiden() error = %v", err)
	}
	largest := 0
	for l := range res.NumCommunities() {
		if len(res.Members(l)) > len(res.Members(largest)) {
			largest = l
		}
	}
	sub, err := Subgraph(g, res.Members(largest))
	if err != nil {
		t.Fatalf("Subgraph() error = %v", err)
	}
	inner, err := Leiden(sub, WithSeed(1))
	if err != nil {
		t.Fatalf("inner Leiden() error = %v", err)
	}
	seen := map[int]bool{}
	for _, c := range inner.Communities() {
		seen[c] = true
	}
	for l := range inner.NumCommunities() {
		if !seen[l] {
			t.Errorf("inner labels not dense: %d missing", l)
		}
	}
	if !connectedCommunities(sub.model, inner.part) {
		t.Errorf("an inner community is disconnected")
	}
}
