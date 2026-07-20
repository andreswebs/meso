package meso

import (
	"strconv"
	"testing"
)

// benchGraph is a named graph fixture for the benchmark suite.
type benchGraph struct {
	name  string
	graph *Graph
}

// benchGraphs returns the small-to-large fixtures the benchmark suite runs
// Leiden and Louvain over (design section 8, step 12): the three canonical
// corpus graphs for the small tier, and deterministic planted-partition graphs
// for the medium and large tiers. Graph construction is outside the timed loop,
// so only the detection call is measured.
func benchGraphs(tb testing.TB) []benchGraph {
	tb.Helper()
	return []benchGraph{
		{"karate", loadGMLGraph(tb, "datasets/karate/karate.gml")},
		{"dolphins", loadGMLGraph(tb, "datasets/dolphins/dolphins.gml")},
		{"lesmis", loadGMLGraph(tb, "datasets/lesmis/lesmis.gml")},
		{"planted-300", plantedGraph(300, 8, 6, 1, 1)},
		{"planted-800", plantedGraph(800, 20, 6, 1, 2)},
	}
}

// plantedGraph builds a deterministic undirected planted-partition graph: n nodes
// split into k equal communities, each node wired to intra random neighbours in
// its own community and inter random neighbours in others. The structure is a
// pure function of seed via the package PRNG, so a given size yields the same
// graph on every run and the benchmark's ns/op stays comparable across runs. The
// Builder folds the duplicate edges the random wiring produces.
func plantedGraph(n, k, intra, inter int, seed uint64) *Graph {
	size := n / k
	r := newPRNG(seed)
	b := NewBuilder().Canonical()
	for u := range n {
		b.AddNodeWeight(strconv.Itoa(u), 1.0)
	}

	community := func(u int) int {
		c := u / size
		if c >= k {
			return k - 1
		}
		return c
	}
	pick := func(lo, hi int) int { return lo + int(r.next()%uint64(hi-lo)) }

	for u := range n {
		c := community(u)
		lo, hi := c*size, (c+1)*size
		if c == k-1 {
			hi = n
		}
		for range intra {
			v := pick(lo, hi)
			if v != u {
				b.AddEdge(strconv.Itoa(u), strconv.Itoa(v), 1.0)
			}
		}
		for range inter {
			rc := pick(0, k)
			if rc == c {
				rc = (rc + 1) % k
			}
			vlo, vhi := rc*size, (rc+1)*size
			if rc == k-1 {
				vhi = n
			}
			b.AddEdge(strconv.Itoa(u), strconv.Itoa(pick(vlo, vhi)), 1.0)
		}
	}

	g, err := b.Build()
	if err != nil {
		panic("meso: plantedGraph build failed: " + err.Error())
	}
	return g
}

// BenchmarkLeiden measures Leiden across the small-to-large tier, reporting
// ns/op and allocs/op per graph so the regression gate can compare them.
func BenchmarkLeiden(b *testing.B) {
	for _, g := range benchGraphs(b) {
		b.Run(g.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := Leiden(g.graph); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkLouvain measures Louvain across the same tier as [BenchmarkLeiden];
// Louvain is the deterministic baseline Leiden is compared against.
func BenchmarkLouvain(b *testing.B) {
	for _, g := range benchGraphs(b) {
		b.Run(g.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := Louvain(g.graph); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
