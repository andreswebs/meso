package meso

import (
	"math"
	"strconv"
	"testing"
)

func starBuilder(leaves int) *Builder {
	b := NewBuilder()
	for i := range leaves {
		b.AddEdge("hub", "leaf"+strconv.Itoa(i), 1)
	}
	return b
}

func TestBetweenness_Star(t *testing.T) {
	for _, leaves := range []int{2, 3, 7} {
		bc := Betweenness(mustBuild(t, starBuilder(leaves)))
		if bc["hub"] != 1 {
			t.Errorf("star(%d): hub = %v, want 1", leaves, bc["hub"])
		}
		for i := range leaves {
			if v := bc["leaf"+strconv.Itoa(i)]; v != 0 {
				t.Errorf("star(%d): leaf%d = %v, want 0", leaves, i, v)
			}
		}
	}
}

func approxEqual(a, b float64) bool { return math.Abs(a-b) <= 1e-12 }

func nodeKey(i int) string { return strconv.Itoa(i) }

func TestBetweenness_Path(t *testing.T) {
	for _, n := range []int{3, 4, 7, 12} {
		b := NewBuilder()
		for i := range n - 1 {
			b.AddEdge(nodeKey(i), nodeKey(i+1), 1)
		}
		bc := Betweenness(mustBuild(t, b))
		for i := range n {
			want := 2 * float64(i*(n-1-i)) / float64((n-1)*(n-2))
			if got := bc[nodeKey(i)]; !approxEqual(got, want) {
				t.Errorf("path(%d): node %d = %v, want %v", n, i, got, want)
			}
		}
	}
}

func TestBetweenness_Complete(t *testing.T) {
	for _, directed := range []bool{false, true} {
		b := NewBuilder()
		if directed {
			b = NewDirectedBuilder()
		}
		for i := range 6 {
			for j := range 6 {
				if i != j {
					b.AddEdge(nodeKey(i), nodeKey(j), 1)
				}
			}
		}
		for k, v := range Betweenness(mustBuild(t, b)) {
			if v != 0 {
				t.Errorf("complete directed=%v: node %s = %v, want 0", directed, k, v)
			}
		}
	}
}

func TestBetweenness_DirectedCycle(t *testing.T) {
	for _, n := range []int{3, 5, 9} {
		b := NewDirectedBuilder()
		for i := range n {
			b.AddEdge(nodeKey(i), nodeKey((i+1)%n), 1)
		}
		for k, v := range Betweenness(mustBuild(t, b)) {
			if !approxEqual(v, 0.5) {
				t.Errorf("directed cycle(%d): node %s = %v, want 0.5", n, k, v)
			}
		}
	}
}

// bruteBetweenness computes normalized betweenness from its definition: for
// every pair (s, t) and every other v, add sigma(s,v)*sigma(v,t)/sigma(s,t)
// when v lies on a shortest s-t path. Distances and shortest-path counts come
// from a BFS per source over the graph's hop relation (Weight presence, self
// excluded). Undirected graphs sum over unordered pairs and divide by
// (n-1)(n-2)/2; directed graphs sum over ordered pairs and divide by (n-1)(n-2).
func bruteBetweenness(g *Graph) map[string]float64 {
	keys := g.Keys()
	n := len(keys)
	adj := make([][]int, n)
	for i, a := range keys {
		for j, b := range keys {
			if _, ok := g.Weight(a, b); ok && i != j {
				adj[i] = append(adj[i], j)
			}
		}
	}
	dist := make([][]int, n)
	sigma := make([][]float64, n)
	for s := range n {
		dist[s] = make([]int, n)
		sigma[s] = make([]float64, n)
		for i := range dist[s] {
			dist[s][i] = -1
		}
		dist[s][s], sigma[s][s] = 0, 1
		queue := []int{s}
		for len(queue) > 0 {
			v := queue[0]
			queue = queue[1:]
			for _, w := range adj[v] {
				if dist[s][w] < 0 {
					dist[s][w] = dist[s][v] + 1
					queue = append(queue, w)
				}
				if dist[s][w] == dist[s][v]+1 {
					sigma[s][w] += sigma[s][v]
				}
			}
		}
	}
	out := make(map[string]float64, n)
	for _, k := range keys {
		out[k] = 0
	}
	if n < 3 {
		return out
	}
	norm := float64((n - 1) * (n - 2))
	if !g.Directed() {
		norm /= 2
	}
	for s := range n {
		for t := range n {
			if s == t || dist[s][t] < 0 || (!g.Directed() && t < s) {
				continue
			}
			for v := range n {
				if v == s || v == t || dist[s][v] < 0 || dist[v][t] < 0 {
					continue
				}
				if dist[s][v]+dist[v][t] == dist[s][t] {
					out[keys[v]] += sigma[s][v] * sigma[v][t] / sigma[s][t] / norm
				}
			}
		}
	}
	return out
}

// randomHopGraph builds a random graph on n nodes, deterministic in seed, with
// edge probability about 1/density, random positive weights, and occasional
// self-loops, so the hop relation, not the weights, must decide the result.
func randomHopGraph(t testing.TB, seed uint64, n, density int, directed bool) *Graph {
	r := newPRNG(seed)
	b := NewBuilder()
	if directed {
		b = NewDirectedBuilder()
	}
	for i := range n {
		b.AddNodeWeight(nodeKey(i), 1)
		if r.next()%4 == 0 {
			b.AddEdge(nodeKey(i), nodeKey(i), 1+r.float64())
		}
		for j := range n {
			if i == j || (!directed && j < i) {
				continue
			}
			if r.next()%uint64(density) == 0 {
				b.AddEdge(nodeKey(i), nodeKey(j), 0.1+5*r.float64())
			}
		}
	}
	g, err := b.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	return g
}

func TestBetweenness_MatchesBruteForce(t *testing.T) {
	for seed := range uint64(400) {
		directed := seed%2 == 1
		n := 1 + int(seed%12)
		density := 2 + int(seed/2%4)
		g := randomHopGraph(t, seed, n, density, directed)
		got, want := Betweenness(g), bruteBetweenness(g)
		if len(got) != len(want) {
			t.Fatalf("seed %d: %d values, want %d", seed, len(got), len(want))
		}
		for k, w := range want {
			if !approxEqual(got[k], w) {
				t.Fatalf("seed %d (n=%d directed=%v): node %s = %v, brute force %v", seed, n, directed, k, got[k], w)
			}
			if got[k] < 0 || got[k] > 1 {
				t.Fatalf("seed %d: node %s = %v outside [0, 1]", seed, k, got[k])
			}
		}
	}
}

func TestBetweenness_Degenerate(t *testing.T) {
	empty := Betweenness(mustBuild(t, NewBuilder()))
	if empty == nil || len(empty) != 0 {
		t.Errorf("empty graph: %#v, want empty non-nil map", empty)
	}
	for _, b := range []*Builder{
		NewBuilder().AddNodeWeight("a", 1),
		NewBuilder().AddEdge("a", "b", 1),
		NewDirectedBuilder().AddEdge("a", "b", 1).AddEdge("b", "a", 1),
	} {
		g := mustBuild(t, b)
		bc := Betweenness(g)
		if len(bc) != g.NumNodes() {
			t.Errorf("%d-node graph: %d values", g.NumNodes(), len(bc))
		}
		for k, v := range bc {
			if v != 0 {
				t.Errorf("%d-node graph: node %s = %v, want 0", g.NumNodes(), k, v)
			}
		}
	}
}

func TestBetweenness_IgnoresWeightsSelfLoopsAndNodeWeights(t *testing.T) {
	plain := NewBuilder().AddEdge("a", "b", 1).AddEdge("b", "c", 1).AddEdge("c", "d", 1).AddEdge("a", "c", 1)
	// The heavy a-c edge would reroute a weighted shortest path through b.
	decorated := NewBuilder().AddEdge("a", "b", 0.1).AddEdge("b", "c", 0.1).AddEdge("c", "d", 9).AddEdge("a", "c", 50).
		AddEdge("b", "b", 3).AddEdge("d", "d", 1).AddNodeWeight("a", 40).AddNodeWeight("c", 0)
	want := Betweenness(mustBuild(t, plain))
	got := Betweenness(mustBuild(t, decorated))
	for k, w := range want {
		if got[k] != w {
			t.Errorf("node %s = %v, want %v as on the plain graph", k, got[k], w)
		}
	}
}

func TestBetweenness_DeterministicAcrossInsertionOrder(t *testing.T) {
	for _, directed := range []bool{false, true} {
		g := randomHopGraph(t, 99, 40, 6, directed)
		type arc struct {
			a, b string
			w    float64
		}
		var arcs []arc
		for _, a := range g.Keys() {
			for _, b := range g.Keys() {
				if w, ok := g.Weight(a, b); ok && (directed || a <= b) {
					arcs = append(arcs, arc{a, b, w})
				}
			}
		}
		build := func(reverse bool) *Graph {
			b := NewBuilder()
			if directed {
				b = NewDirectedBuilder()
			}
			b.Canonical()
			for i := range arcs {
				e := arcs[i]
				if reverse {
					e = arcs[len(arcs)-1-i]
				}
				b.AddEdge(e.a, e.b, e.w)
			}
			for _, k := range g.Keys() {
				b.AddNodeWeight(k, 1)
			}
			return mustBuild(t, b)
		}
		first, second := Betweenness(build(false)), Betweenness(build(true))
		again := Betweenness(build(false))
		for k, v := range first {
			if math.Float64bits(v) != math.Float64bits(second[k]) {
				t.Errorf("directed=%v: node %s differs across insertion order: %v vs %v", directed, k, v, second[k])
			}
			if math.Float64bits(v) != math.Float64bits(again[k]) {
				t.Errorf("directed=%v: node %s differs across repeated calls", directed, k)
			}
		}
	}
}

func TestBetweenness_AllocationsIndependentOfEdgeCount(t *testing.T) {
	const n = 60
	path := NewBuilder()
	complete := NewBuilder()
	for i := range n {
		for j := i + 1; j < n; j++ {
			complete.AddEdge(nodeKey(i), nodeKey(j), 1)
		}
		if i+1 < n {
			path.AddEdge(nodeKey(i), nodeKey(i+1), 1)
		}
	}
	sparse, dense := mustBuild(t, path), mustBuild(t, complete)
	a := testing.AllocsPerRun(5, func() { Betweenness(sparse) })
	b := testing.AllocsPerRun(5, func() { Betweenness(dense) })
	if a != b {
		t.Errorf("allocations depend on edge count: path %v, complete %v", a, b)
	}
	if a > 20 {
		t.Errorf("allocations = %v, want a small constant plus the result map", a)
	}
}
