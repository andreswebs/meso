package meso

import (
	"math"
	"slices"
	"sync"
	"testing"
)

func TestResult_MembersPartitionKeys(t *testing.T) {
	for _, path := range undirectedCorpusGML {
		t.Run(path, func(t *testing.T) {
			g := loadGMLGraph(t, path)
			res, err := Leiden(g, WithSeed(7))
			if err != nil {
				t.Fatalf("Leiden() error = %v", err)
			}
			distinct := map[int]bool{}
			for _, c := range res.Communities() {
				distinct[c] = true
			}
			if got := res.NumCommunities(); got != len(distinct) {
				t.Fatalf("NumCommunities() = %d, want %d distinct labels", got, len(distinct))
			}

			comm := res.Communities()
			var all []string
			for l := range res.NumCommunities() {
				members := res.Members(l)
				if len(members) == 0 {
					t.Errorf("Members(%d) is empty for a dense label", l)
				}
				prev := -1
				for _, k := range members {
					if comm[k] != l {
						t.Errorf("Members(%d) contains %q labelled %d", l, k, comm[k])
					}
					i, _ := g.Index(k)
					if i <= prev {
						t.Errorf("Members(%d) not in dense index order at %q", l, k)
					}
					prev = i
				}
				all = append(all, members...)
			}
			slices.Sort(all)
			want := g.Keys()
			slices.Sort(want)
			if !slices.Equal(all, want) {
				t.Errorf("Members over all labels does not partition Keys()")
			}
		})
	}
}

func TestResult_MembersOutOfRange(t *testing.T) {
	res, err := Leiden(mustBuild(t, NewBuilder().AddEdge("a", "b", 1)))
	if err != nil {
		t.Fatalf("Leiden() error = %v", err)
	}
	for _, l := range []int{-1, res.NumCommunities()} {
		if got := res.Members(l); got != nil {
			t.Errorf("Members(%d) = %v, want nil", l, got)
		}
	}
}

func TestResult_MembersIsACopy(t *testing.T) {
	res, err := Leiden(mustBuild(t, NewBuilder().AddEdge("a", "b", 1)))
	if err != nil {
		t.Fatalf("Leiden() error = %v", err)
	}
	m := res.Members(0)
	m[0] = "mutated"
	if res.Members(0)[0] == "mutated" {
		t.Errorf("mutating Members() changed the result")
	}
}

// resultFor builds a Result for g with labels given by key, for fixtures that
// need a hand-chosen partition. Labels must be dense.
func resultFor(t *testing.T, g *Graph, labels map[string]int) *Result {
	t.Helper()
	p := make(Partition, g.NumNodes())
	for k, c := range labels {
		i, ok := g.Index(k)
		if !ok {
			t.Fatalf("resultFor: unknown key %q", k)
		}
		p[i] = c
	}
	return newResult(g, p, 0)
}

func TestResult_CohesionClosedForms(t *testing.T) {
	clique := NewBuilder().AddEdge("a", "b", 1).AddEdge("a", "c", 1).AddEdge("b", "c", 1).AddEdge("a", "d", 1).AddEdge("b", "d", 1).AddEdge("c", "d", 1)
	path4 := NewBuilder().AddEdge("a", "b", 1).AddEdge("b", "c", 1).AddEdge("c", "d", 1)
	symmetric := NewDirectedBuilder().AddEdge("a", "b", 1).AddEdge("b", "a", 1).AddEdge("b", "c", 1).AddEdge("c", "b", 1)
	oneArc := NewDirectedBuilder().AddEdge("a", "b", 1)
	tests := []struct {
		name   string
		b      *Builder
		labels map[string]int
		label  int
		want   float64
	}{
		{"clique", clique, map[string]int{"a": 0, "b": 0, "c": 0, "d": 0}, 0, 1},
		{"four members, three internal edges", path4, map[string]int{"a": 0, "b": 0, "c": 0, "d": 0}, 0, 0.5},
		{"no internal edges", path4, map[string]int{"a": 0, "b": 1, "c": 0, "d": 1}, 0, 0},
		{"singleton", path4, map[string]int{"a": 0, "b": 1, "c": 1, "d": 1}, 0, 0},
		{"edges leaving the community do not count", path4, map[string]int{"a": 0, "b": 0, "c": 1, "d": 1}, 0, 1},
		{"directed symmetric equals undirected", symmetric, map[string]int{"a": 0, "b": 0, "c": 0}, 0, 2.0 / 3},
		{"lone arc in a two-member directed community", oneArc, map[string]int{"a": 0, "b": 0}, 0, 0.5},
		{"unknown label", path4, map[string]int{"a": 0, "b": 0, "c": 0, "d": 0}, 1, 0},
		{"negative label", path4, map[string]int{"a": 0, "b": 0, "c": 0, "d": 0}, -1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := resultFor(t, mustBuild(t, tt.b), tt.labels)
			if got := r.Cohesion(tt.label); got != tt.want {
				t.Errorf("Cohesion(%d) = %v, want %v", tt.label, got, tt.want)
			}
		})
	}
}

// randomCohesionFixture builds a random graph on n nodes keyed "0".."n-1" with
// a random dense partition into at most k labels, deterministic in seed.
// weight maps a uniform draw in [0, 1) to each edge's weight; the draw is
// always taken so fixtures differing only in weight share their topology.
// selfLoops adds a self-loop to every node.
func randomCohesionFixture(t *testing.T, seed uint64, n, k int, directed, selfLoops bool, weight func(float64) float64) (*Graph, map[string]int) {
	t.Helper()
	r := newPRNG(seed)
	b := NewBuilder()
	if directed {
		b = NewDirectedBuilder()
	}
	b.Canonical()
	key := func(i int) string { return string(rune('A' + i)) }
	for i := range n {
		b.AddNodeWeight(key(i), 1)
		if selfLoops {
			b.AddEdge(key(i), key(i), 1)
		}
		for j := range n {
			if i == j || (!directed && j < i) {
				continue
			}
			edge, u := r.next()%3 == 0, r.float64()
			if edge {
				b.AddEdge(key(i), key(j), weight(u))
			}
		}
	}
	raw := make([]int, n)
	for i := range raw {
		raw[i] = int(r.next() % uint64(k))
	}
	canon := canonicalize(raw)
	labels := make(map[string]int, n)
	for i, c := range canon {
		labels[key(i)] = c
	}
	return mustBuild(t, b), labels
}

func bruteCohesion(g *Graph, members []string) float64 {
	k := len(members)
	if k < 2 {
		return 0
	}
	count := 0
	for _, a := range members {
		for _, b := range members {
			if a == b || (!g.Directed() && a > b) {
				continue
			}
			if _, ok := g.Weight(a, b); ok {
				count++
			}
		}
	}
	pairs := k * (k - 1)
	if !g.Directed() {
		pairs /= 2
	}
	return float64(count) / float64(pairs)
}

func TestResult_CohesionMatchesBruteForce(t *testing.T) {
	unit := func(float64) float64 { return 1 }
	for seed := range uint64(200) {
		directed := seed%2 == 1
		g, labels := randomCohesionFixture(t, seed, 2+int(seed%9), 1+int(seed%4), directed, false, unit)
		r := resultFor(t, g, labels)
		for l := range r.NumCommunities() {
			want := bruteCohesion(g, r.Members(l))
			if got := r.Cohesion(l); math.Abs(got-want) > 1e-15 {
				t.Fatalf("seed %d directed=%v: Cohesion(%d) = %v, brute force %v", seed, directed, l, got, want)
			}
			if got := r.Cohesion(l); got < 0 || got > 1 {
				t.Fatalf("seed %d: Cohesion(%d) = %v outside [0, 1]", seed, l, got)
			}
		}
	}
}

func TestResult_CohesionIgnoresWeightsAndSelfLoops(t *testing.T) {
	unit := func(float64) float64 { return 1 }
	random := func(u float64) float64 { return 0.01 + 10*u }
	for seed := range uint64(50) {
		directed := seed%2 == 1
		base, labels := randomCohesionFixture(t, seed, 8, 3, directed, false, unit)
		weighted, _ := randomCohesionFixture(t, seed, 8, 3, directed, false, random)
		looped, _ := randomCohesionFixture(t, seed, 8, 3, directed, true, unit)
		rb, rw, rl := resultFor(t, base, labels), resultFor(t, weighted, labels), resultFor(t, looped, labels)
		for l := range rb.NumCommunities() {
			if rb.Cohesion(l) != rw.Cohesion(l) {
				t.Errorf("seed %d: weights changed Cohesion(%d): %v vs %v", seed, l, rb.Cohesion(l), rw.Cohesion(l))
			}
			if rb.Cohesion(l) != rl.Cohesion(l) {
				t.Errorf("seed %d: self-loops changed Cohesion(%d): %v vs %v", seed, l, rb.Cohesion(l), rl.Cohesion(l))
			}
		}
	}
}

func TestResult_ConcurrentReaders(t *testing.T) {
	g := loadGMLGraph(t, "datasets/karate/karate.gml")
	res, err := Leiden(g, WithSeed(1))
	if err != nil {
		t.Fatalf("Leiden() error = %v", err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for l := range res.NumCommunities() {
				_ = res.Members(l)
				_ = res.Cohesion(l)
			}
			_ = res.Communities()
		})
	}
	wg.Wait()
}
