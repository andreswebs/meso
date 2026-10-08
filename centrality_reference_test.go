package meso

import (
	"encoding/csv"
	"math"
	"os"
	"strconv"
	"testing"
)

// loadGMLDirectedGraph is loadGMLGraph for a GML declaring `directed 1`: each
// edge block is an arc source -> target, parallel arcs fold.
func loadGMLDirectedGraph(t testing.TB, path string) *Graph {
	t.Helper()
	nodes, edges := parseGML(t, path)
	b := NewDirectedBuilder()
	for _, id := range nodes {
		b.AddNodeWeight(id, 1)
	}
	for _, e := range edges {
		b.AddEdge(e.src, e.tgt, e.w)
	}
	g, err := b.Build()
	if err != nil {
		t.Fatalf("build %s: %v", path, err)
	}
	return g
}

// readBetweennessCSV reads a committed networkx reference written by
// datasets/betweenness.py: a key,betweenness header then one row per node.
func readBetweennessCSV(t *testing.T, path string) map[string]float64 {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			t.Errorf("close %s: %v", path, cerr)
		}
	}()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(rows) == 0 || rows[0][0] != "key" || rows[0][1] != "betweenness" {
		t.Fatalf("%s: missing key,betweenness header", path)
	}
	out := make(map[string]float64, len(rows)-1)
	for _, row := range rows[1:] {
		v, err := strconv.ParseFloat(row[1], 64)
		if err != nil {
			t.Fatalf("%s: row %v: %v", path, row, err)
		}
		if _, dup := out[row[0]]; dup {
			t.Fatalf("%s: duplicate key %q", path, row[0])
		}
		out[row[0]] = v
	}
	return out
}

func TestBetweenness_MatchesNetworkx(t *testing.T) {
	for _, tc := range []struct {
		name     string
		directed bool
	}{
		{"karate", false},
		{"dolphins", false},
		{"lesmis", false},
		{"celegansneural", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gml := "datasets/" + tc.name + "/" + tc.name + ".gml"
			g := loadGMLGraph(t, gml)
			if tc.directed {
				g = loadGMLDirectedGraph(t, gml)
			}
			want := readBetweennessCSV(t, "datasets/"+tc.name+"/betweenness.csv")
			got := Betweenness(g)
			if len(want) != len(got) {
				t.Fatalf("reference has %d keys, graph has %d", len(want), len(got))
			}
			for k, w := range want {
				v, ok := got[k]
				if !ok {
					t.Fatalf("reference key %q is not a graph key", k)
				}
				if math.Abs(v-w) > 1e-12 {
					t.Errorf("node %s = %.17g, networkx %.17g", k, v, w)
				}
			}
		})
	}
}

func TestBetweenness_KaratePublishedValues(t *testing.T) {
	// The commonly quoted values are truncated to four decimals (node 34 is
	// 0.30407...), so they are checked to within one unit in the fourth place.
	bc := Betweenness(loadGMLGraph(t, "datasets/karate/karate.gml"))
	for key, want := range map[string]float64{"1": 0.4376, "34": 0.3040, "33": 0.1452} {
		if got := bc[key]; math.Abs(got-want) > 1e-4 {
			t.Errorf("node %s = %.6f, published %.4f", key, got, want)
		}
	}
}
