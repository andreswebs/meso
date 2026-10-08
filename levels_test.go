package meso

import (
	"maps"
	"math"
	"testing"
)

type runFunc func(*Graph, ...Option) (*Result, error)

var algorithms = []struct {
	name string
	run  runFunc
}{{"Leiden", Leiden}, {"Louvain", Louvain}}

func TestLevels_LastLevelIsTheResult(t *testing.T) {
	for _, path := range undirectedCorpusGML {
		g := loadGMLGraph(t, path)
		for _, alg := range algorithms {
			for _, opts := range [][]Option{{WithSeed(3)}, {WithSeed(3), WithParallelism(4)}} {
				res, err := alg.run(g, opts...)
				if err != nil {
					t.Fatalf("%s %s: %v", alg.name, path, err)
				}
				n := res.NumLevels()
				if n < 1 {
					t.Fatalf("%s %s: NumLevels() = %d, want >= 1", alg.name, path, n)
				}
				if !maps.Equal(res.Level(n-1), res.Communities()) {
					t.Errorf("%s %s: last level differs from Communities()", alg.name, path)
				}
				if res.LevelQuality(n-1) != res.Quality() {
					t.Errorf("%s %s: LevelQuality(last) = %v, Quality() = %v", alg.name, path, res.LevelQuality(n-1), res.Quality())
				}
			}
		}
	}
}

// levelGraphs pairs each corpus graph with an objective it accepts, including
// the directed celegansneural under DirectedModularity.
func levelGraphs(t *testing.T) []struct {
	name string
	g    *Graph
	q    QualityFunction
} {
	t.Helper()
	type lg = struct {
		name string
		g    *Graph
		q    QualityFunction
	}
	var out []lg
	for _, path := range undirectedCorpusGML {
		g := loadGMLGraph(t, path)
		out = append(out, lg{path + "/modularity", g, Modularity(1)}, lg{path + "/cpm", g, CPM(0.05)})
	}
	out = append(out, lg{"celegansneural/directed", loadGMLDirectedGraph(t, "datasets/celegansneural/celegansneural.gml"), DirectedModularity(1)})
	return out
}

func TestLevels_WellFormedAndMonotone(t *testing.T) {
	for _, tc := range levelGraphs(t) {
		for _, alg := range algorithms {
			res, err := alg.run(tc.g, WithQuality(tc.q), WithSeed(5))
			if err != nil {
				t.Fatalf("%s %s: %v", alg.name, tc.name, err)
			}
			for l := range res.NumLevels() {
				labels := res.Level(l)
				if len(labels) != tc.g.NumNodes() {
					t.Fatalf("%s %s level %d: %d labels for %d nodes", alg.name, tc.name, l, len(labels), tc.g.NumNodes())
				}
				seen := map[int]bool{}
				for _, c := range labels {
					seen[c] = true
				}
				for c := range len(seen) {
					if !seen[c] {
						t.Errorf("%s %s level %d: labels not dense, %d missing", alg.name, tc.name, l, c)
					}
				}
				if l > 0 && res.LevelQuality(l) < res.LevelQuality(l-1)-louvainTol {
					t.Errorf("%s %s: quality fell from level %d to %d: %v -> %v",
						alg.name, tc.name, l-1, l, res.LevelQuality(l-1), res.LevelQuality(l))
				}
			}
		}
	}
}

func TestLevels_LouvainNests(t *testing.T) {
	for _, tc := range levelGraphs(t) {
		res, err := Louvain(tc.g, WithQuality(tc.q))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		for l := 1; l < res.NumLevels(); l++ {
			finer, coarser := res.Level(l-1), res.Level(l)
			parent := map[int]int{}
			for k, c := range finer {
				if p, ok := parent[c]; ok && p != coarser[k] {
					t.Fatalf("%s: level %d community %d splits across level %d communities %d and %d", tc.name, l-1, c, l, p, coarser[k])
				}
				parent[c] = coarser[k]
			}
		}
	}
}

func TestLevels_OutOfRange(t *testing.T) {
	res, err := Leiden(mustBuild(t, NewBuilder().AddEdge("a", "b", 1)))
	if err != nil {
		t.Fatalf("Leiden() error = %v", err)
	}
	for _, i := range []int{-1, res.NumLevels()} {
		if res.Level(i) != nil || res.LevelQuality(i) != 0 {
			t.Errorf("level %d: Level = %v, LevelQuality = %v, want nil and 0", i, res.Level(i), res.LevelQuality(i))
		}
	}
}

func TestLevels_SingleLevelWhenNothingMerges(t *testing.T) {
	g := mustBuild(t, NewBuilder().AddNodeWeight("a", 1).AddNodeWeight("b", 1).AddNodeWeight("c", 1))
	for _, alg := range algorithms {
		res, err := alg.run(g)
		if err != nil {
			t.Fatalf("%s: %v", alg.name, err)
		}
		if res.NumLevels() != 1 {
			t.Fatalf("%s: NumLevels() = %d, want 1", alg.name, res.NumLevels())
		}
		if lv := res.Level(0); lv["a"] == lv["b"] || lv["b"] == lv["c"] || lv["a"] == lv["c"] {
			t.Errorf("%s: level 0 = %v, want all singletons", alg.name, lv)
		}
	}
}

func TestLevels_IterationsReportTheFinalPass(t *testing.T) {
	g := loadGMLGraph(t, "datasets/lesmis/lesmis.gml")
	for k := 2; k <= 4; k++ {
		prev, err := Leiden(g, WithSeed(9), WithIterations(k-1))
		if err != nil {
			t.Fatal(err)
		}
		res, err := Leiden(g, WithSeed(9), WithIterations(k))
		if err != nil {
			t.Fatal(err)
		}
		// The final pass starts from the (k-1)-pass result, so its first level
		// is already at least that good; a concatenation of all passes would
		// start from the singleton-seeded first level instead.
		if res.LevelQuality(0) < prev.Quality()-louvainTol {
			t.Errorf("k=%d: LevelQuality(0) = %v below the %d-pass result %v", k, res.LevelQuality(0), k-1, prev.Quality())
		}
		first, err := Leiden(g, WithSeed(9))
		if err != nil {
			t.Fatal(err)
		}
		if res.LevelQuality(0) < first.LevelQuality(first.NumLevels()-1)-louvainTol {
			t.Errorf("k=%d: final pass starts below the first pass's result", k)
		}
		if !maps.Equal(res.Level(res.NumLevels()-1), res.Communities()) {
			t.Errorf("k=%d: last level differs from Communities()", k)
		}
	}
}

func TestLevels_Deterministic(t *testing.T) {
	g := loadGMLGraph(t, "datasets/dolphins/dolphins.gml")
	for _, alg := range algorithms {
		ref, err := alg.run(g, WithSeed(11), WithParallelism(1))
		if err != nil {
			t.Fatal(err)
		}
		for _, workers := range []int{1, 2, 4, 8} {
			for range 2 {
				res, err := alg.run(g, WithSeed(11), WithParallelism(workers))
				if err != nil {
					t.Fatal(err)
				}
				if res.NumLevels() != ref.NumLevels() {
					t.Fatalf("%s workers=%d: %d levels, want %d", alg.name, workers, res.NumLevels(), ref.NumLevels())
				}
				for l := range ref.NumLevels() {
					if !maps.Equal(res.Level(l), ref.Level(l)) || math.Float64bits(res.LevelQuality(l)) != math.Float64bits(ref.LevelQuality(l)) {
						t.Errorf("%s workers=%d: level %d differs", alg.name, workers, l)
					}
				}
			}
		}
		again, err := alg.run(g, WithSeed(11))
		if err != nil {
			t.Fatal(err)
		}
		serial, err := alg.run(g, WithSeed(11))
		if err != nil {
			t.Fatal(err)
		}
		for l := range serial.NumLevels() {
			if !maps.Equal(again.Level(l), serial.Level(l)) {
				t.Errorf("%s serial: level %d differs across runs", alg.name, l)
			}
		}
	}
}
