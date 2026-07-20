package meso

import (
	"math/rand"
	"reflect"
	"testing"
)

// TestLouvain_ByteIdenticalRepeated asserts AC#1: at a fixed configuration the
// serial Louvain result is byte-identical across repeated runs. Louvain carries
// no PRNG - its determinism comes from canonical node order, smallest-label
// tie-breaks, and the ascending-label aggregate - so this test locks that
// property in as a regression guard for when refinement layers seeded
// randomness on top (M2).
func TestLouvain_ByteIdenticalRepeated(t *testing.T) {
	graphs := corpusAndFuzzed(t, 20260717)
	objs := []objective{modularity{gamma: 1.0}, cpm{gamma: 0.1}}

	for _, fg := range graphs {
		for _, obj := range objs {
			first := louvain(fg.g, obj)
			for run := 1; run < 5; run++ {
				got := louvain(fg.g, obj)
				if !reflect.DeepEqual(first, got) {
					t.Fatalf("%s: run %d partition differs from run 0\n run0 = %v\n run%d = %v",
						fg.name, run, first, run, got)
				}
			}
		}
	}
}

// TestBuilder_CanonicalOrderIndependent asserts AC#2: with Canonical(), building
// the same graph from differently-ordered (shuffled) edge insertions yields
// byte-identical output - the same key-to-index mapping, the same internal
// model, and the same community partition. The default first-seen indexing makes
// the dense labels depend on insertion order; Canonical() sorts by key so the
// labelling is a pure function of the key set.
func TestBuilder_CanonicalOrderIndependent(t *testing.T) {
	type edge struct {
		a, b string
		w    float64
	}
	edges := []edge{
		{"delta", "alpha", 1.0}, {"alpha", "charlie", 2.0}, {"bravo", "delta", 1.5},
		{"charlie", "bravo", 3.0}, {"echo", "alpha", 0.5}, {"delta", "echo", 2.0},
		{"charlie", "echo", 1.0}, {"bravo", "alpha", 1.0},
	}

	build := func(order []int) *Graph {
		b := NewBuilder().Canonical()
		for _, i := range order {
			e := edges[i]
			b.AddEdge(e.a, e.b, e.w)
		}
		g, err := b.Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		return g
	}

	identity := make([]int, len(edges))
	for i := range identity {
		identity[i] = i
	}
	shuffled := rand.New(rand.NewSource(99)).Perm(len(edges))

	g1 := build(identity)
	g2 := build(shuffled)

	// Same key-to-index mapping.
	if !reflect.DeepEqual(g1.index, g2.index) {
		t.Fatalf("index mappings differ:\n %v\n %v", g1.index, g2.index)
	}
	if !reflect.DeepEqual(g1.keys, g2.keys) {
		t.Fatalf("key orders differ:\n %v\n %v", g1.keys, g2.keys)
	}

	// Byte-identical internal model.
	if !reflect.DeepEqual(g1.model, g2.model) {
		t.Fatalf("internal models differ:\n %+v\n %+v", g1.model, g2.model)
	}

	// And therefore identical community partitions.
	obj := modularity{gamma: 1.0}
	if p1, p2 := louvain(g1.model, obj), louvain(g2.model, obj); !reflect.DeepEqual(p1, p2) {
		t.Fatalf("partitions differ:\n %v\n %v", p1, p2)
	}

	// Keys are in sorted order under Canonical().
	for i := 1; i < len(g1.keys); i++ {
		if g1.keys[i-1] > g1.keys[i] {
			t.Fatalf("keys not sorted: %q before %q", g1.keys[i-1], g1.keys[i])
		}
	}
}
