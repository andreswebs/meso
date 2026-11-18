package meso

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

// benchGraph is a named graph fixture for the benchmark suite.
type benchGraph struct {
	name  string
	graph *Graph
}

// benchGraphs returns the small-to-large fixtures the benchmark suite runs
// Leiden and Louvain over (design section 8, step 12): the three canonical
// corpus graphs for the small tier, deterministic planted-partition graphs for
// the medium tier, and for the large tier a planted graph of four communities
// of 4000 nodes (large communities) and sparse hub-and-components graphs (many
// small ones). The two sparse sizes differ by a factor of two so the pair shows
// how the run time grows with the graph. Graph construction is outside the timed loop, so only
// the detection call is measured.
func benchGraphs(tb testing.TB) []benchGraph {
	tb.Helper()
	return []benchGraph{
		{"karate", loadGMLGraph(tb, "datasets/karate/karate.gml")},
		{"dolphins", loadGMLGraph(tb, "datasets/dolphins/dolphins.gml")},
		{"lesmis", loadGMLGraph(tb, "datasets/lesmis/lesmis.gml")},
		{"planted-300", plantedGraph(300, 8, 6, 1, 1)},
		{"planted-800", plantedGraph(800, 20, 6, 1, 2)},
		{"planted-16k-4", plantedGraph(16000, 4, 6, 1, 1)},
		{"sparse-10k", sparseGraph(10000, 3, false, false)},
		{"sparse-20k", sparseGraph(20000, 3, false, false)},
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

// BenchmarkParallel measures Leiden and Louvain with the synchronous-round
// parallel local-move phase on the larger sparse graph, where nearly every node
// proposes a move that conflicts with a neighbour's in the first rounds.
func BenchmarkParallel(b *testing.B) {
	g := sparseGraph(20000, 3, false, false)
	for _, alg := range []struct {
		name string
		run  func(*Graph, ...Option) (*Result, error)
	}{
		{"leiden", Leiden},
		{"louvain", Louvain},
	} {
		b.Run(alg.name+"/sparse-20k", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := alg.run(g, WithParallelism(4)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// loadLFREdgeList builds the undirected graph of a committed LFR edge list
// (datasets/lfr: "u v" lines, "#" comments), canonically indexed. The richer
// LFR loader lives in the external test package and is not reachable here.
func loadLFREdgeList(tb testing.TB, path string) *Graph {
	tb.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("read %s: %v", path, err)
	}
	b := NewBuilder().Canonical()
	for line := range strings.SplitSeq(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) == 0 || strings.HasPrefix(f[0], "#") {
			continue
		}
		if len(f) != 2 {
			tb.Fatalf("%s: malformed line %q", path, line)
		}
		b.AddEdge(f[0], f[1], 1)
	}
	g, err := b.Build()
	if err != nil {
		tb.Fatalf("build %s: %v", path, err)
	}
	return g
}

// BenchmarkBetweenness measures Betweenness from a corpus graph up to a
// 1000-node LFR graph, where its O(nm) cost dominates.
func BenchmarkBetweenness(b *testing.B) {
	for _, g := range []benchGraph{
		{"karate", loadGMLGraph(b, "datasets/karate/karate.gml")},
		{"lesmis", loadGMLGraph(b, "datasets/lesmis/lesmis.gml")},
		{"lfr-S-mu030", loadLFREdgeList(b, "datasets/lfr/S/mu030-r0.txt")},
	} {
		b.Run(g.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				Betweenness(g.graph)
			}
		})
	}
}

// BenchmarkSubgraph measures extracting the largest Leiden community of karate,
// the re-split step a consumer runs before a second Leiden pass.
func BenchmarkSubgraph(b *testing.B) {
	g := loadGMLGraph(b, "datasets/karate/karate.gml")
	res, err := Leiden(g, WithSeed(1))
	if err != nil {
		b.Fatal(err)
	}
	largest := 0
	for l := range res.NumCommunities() {
		if len(res.Members(l)) > len(res.Members(largest)) {
			largest = l
		}
	}
	members := res.Members(largest)
	b.Run("karate-largest", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := Subgraph(g, members); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// sparseGraph builds a deterministic sparse graph of n nodes shaped like a
// dependency estate: many small tree-like components (about one edge per node,
// plus a few extra intra-component edges) and a handful of hubs wired to random
// nodes across the graph, for roughly 1.1 edges per node overall. Weights are 1
// when fractional is false and non-integer otherwise, so the same shape
// exercises both exact and rounded community totals. The structure is a pure
// function of the arguments via the package PRNG.
func sparseGraph(n int, seed uint64, directed, fractional bool) *Graph {
	r := newPRNG(seed)
	b := NewBuilder()
	if directed {
		b = NewDirectedBuilder()
	}
	b.Canonical()
	key := func(u int) string { return fmt.Sprintf("n%07d", u) }
	for u := range n {
		b.AddNodeWeight(key(u), 1.0)
	}
	weight := func() float64 {
		if fractional {
			return 0.25 + r.float64()
		}
		return 1.0
	}
	pick := func(lo, hi int) int { return lo + int(r.next()%uint64(hi-lo)) }

	for lo := 0; lo < n; {
		hi := min(lo+pick(5, 40), n)
		for u := lo + 1; u < hi; u++ {
			b.AddEdge(key(u), key(pick(lo, u)), weight())
			if r.next()%20 == 0 && u-lo > 2 {
				if v := pick(lo, u); v != u {
					b.AddEdge(key(v), key(u), weight())
				}
			}
		}
		lo = hi
	}

	hubs := 5
	for h := range hubs {
		hub := (h*n)/hubs + n/(2*hubs)
		for range n / 100 {
			if v := pick(0, n); v != hub {
				b.AddEdge(key(hub), key(v), weight())
			}
		}
	}

	g, err := b.Build()
	if err != nil {
		panic("meso: sparseGraph build failed: " + err.Error())
	}
	return g
}
