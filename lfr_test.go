package meso_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/andreswebs/meso"
)

// The LFR accuracy suite grades meso's recovery of planted communities on the
// committed Lancichinetti-Fortunato-Radicchi benchmark graphs (design section
// 6.1, plan step 11). The graphs and their ground truth are frozen vectors
// produced out of band by datasets/lfr/generate.py (networkx's
// LFR_benchmark_graph, pinned by version and seed); nothing generates them in
// CI. This file is the Go consumer: it loads a graph and its planted partition,
// runs Leiden, and scores recovery with the NMI/ARI metrics against the
// planting.

// lfrDir is the committed LFR suite root, relative to the package directory
// (the module root, where tests run).
const lfrDir = "datasets/lfr"

// lfrManifest is the parameter record datasets/lfr/generate.py writes alongside
// the graphs. Only the fields the accuracy sweep reads are modelled.
type lfrManifest struct {
	Grid struct {
		N            int       `json:"n"`
		Mu           []float64 `json:"mu"`
		Realizations int       `json:"realizations"`
	} `json:"grid"`
	Graphs []lfrEntry `json:"graphs"`
}

// lfrEntry describes one committed graph: where its edges and ground truth live
// and the planting parameters it was generated under.
type lfrEntry struct {
	EdgesFile       string  `json:"edges_file"`
	GroundTruthFile string  `json:"ground_truth_file"`
	Regime          string  `json:"regime"`
	N               int     `json:"n"`
	NumCommunities  int     `json:"num_communities"`
	NominalMu       float64 `json:"nominal_mu"`
	Realization     int     `json:"realization"`
}

// lfrGraph is a loaded benchmark graph paired with its planted ground truth.
// planted[i] is the ground-truth community of dense node i, aligned to g's
// index so it can be scored directly against a detected partition.
type lfrGraph struct {
	g       *meso.Graph
	planted []int
	entry   lfrEntry
}

// loadLFRManifest reads and parses the committed suite manifest.
func loadLFRManifest(t *testing.T) lfrManifest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(lfrDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read LFR manifest: %v", err)
	}
	var m lfrManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse LFR manifest: %v", err)
	}
	if len(m.Graphs) == 0 {
		t.Fatalf("LFR manifest lists no graphs")
	}
	return m
}

// loadLFRGraph builds the meso.Graph and planted partition for one manifest
// entry. Ground-truth nodes are registered first so every planted node exists
// in the graph even if a future grid produced an isolated one; the edge list
// then folds into an undirected, unit-weighted graph. Node ids are 1-based in
// the files; they become string keys, and the planting is aligned back through
// the graph's index.
func loadLFRGraph(t *testing.T, entry lfrEntry) lfrGraph {
	t.Helper()

	truth := readGroundTruth(t, filepath.Join(lfrDir, entry.GroundTruthFile))

	b := meso.NewBuilder().Canonical()
	for _, nc := range truth {
		b.AddNodeWeight(strconv.Itoa(nc.node), 1.0)
	}
	for _, e := range readEdges(t, filepath.Join(lfrDir, entry.EdgesFile)) {
		b.AddEdge(strconv.Itoa(e[0]), strconv.Itoa(e[1]), 1.0)
	}
	g, err := b.Build()
	if err != nil {
		t.Fatalf("build LFR graph %s: %v", entry.EdgesFile, err)
	}

	planted := make([]int, g.NumNodes())
	for i := range planted {
		planted[i] = -1
	}
	for _, nc := range truth {
		idx, ok := g.Index(strconv.Itoa(nc.node))
		if !ok {
			t.Fatalf("ground-truth node %d absent from graph %s", nc.node, entry.EdgesFile)
		}
		planted[idx] = nc.community
	}
	for i, c := range planted {
		if c < 0 {
			t.Fatalf("graph %s: node index %d has no planted community", entry.EdgesFile, i)
		}
	}

	return lfrGraph{g: g, planted: planted, entry: entry}
}

// nodeCommunity is one planted assignment parsed from a ground-truth CSV.
type nodeCommunity struct {
	node, community int
}

// readGroundTruth parses a ground-truth CSV: a "node,community" header, optional
// "#" comment lines, then one "node,community" pair per line, all 1-based.
func readGroundTruth(t *testing.T, path string) []nodeCommunity {
	t.Helper()
	var out []nodeCommunity
	for _, line := range readDataLines(t, path) {
		if line == "node,community" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) != 2 {
			t.Fatalf("ground truth %s: malformed line %q", path, line)
		}
		node, err1 := strconv.Atoi(strings.TrimSpace(fields[0]))
		comm, err2 := strconv.Atoi(strings.TrimSpace(fields[1]))
		if err1 != nil || err2 != nil {
			t.Fatalf("ground truth %s: non-integer line %q", path, line)
		}
		out = append(out, nodeCommunity{node: node, community: comm})
	}
	return out
}

// readEdges parses an edge list: "#" comment lines then one "u v" pair per line,
// whitespace-separated, 1-based, each undirected edge listed once.
func readEdges(t *testing.T, path string) [][2]int {
	t.Helper()
	var out [][2]int
	for _, line := range readDataLines(t, path) {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatalf("edges %s: malformed line %q", path, line)
		}
		u, err1 := strconv.Atoi(fields[0])
		v, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil {
			t.Fatalf("edges %s: non-integer line %q", path, line)
		}
		out = append(out, [2]int{u, v})
	}
	return out
}

// readDataLines returns the non-empty, non-comment lines of a committed data
// file, trimmed. Reading the whole file keeps the parsers free of an open
// handle to close.
func readDataLines(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out []string
	for line := range strings.SplitSeq(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// TestLFRLoad is acceptance criterion 1: loading a committed graph yields the
// requested node count and a valid planted partition (every node assigned, and
// the community count matching what the generator recorded).
func TestLFRLoad(t *testing.T) {
	m := loadLFRManifest(t)
	lg := loadLFRGraph(t, m.Graphs[0])

	if lg.g.NumNodes() != lg.entry.N {
		t.Errorf("NumNodes() = %d, want %d", lg.g.NumNodes(), lg.entry.N)
	}
	if lg.g.NumNodes() != m.Grid.N {
		t.Errorf("NumNodes() = %d, want grid N %d", lg.g.NumNodes(), m.Grid.N)
	}

	seen := make(map[int]struct{})
	for _, c := range lg.planted {
		seen[c] = struct{}{}
	}
	if len(seen) != lg.entry.NumCommunities {
		t.Errorf("planted communities = %d, want %d", len(seen), lg.entry.NumCommunities)
	}
}

// TestLFRLoadDeterministic is acceptance criterion 2: the committed suite is
// reproducible. datasets/lfr/generate.py seeds networkx deterministically, so
// the frozen files are fixed; loading the same entry twice must yield a
// byte-identical graph structure and planting.
func TestLFRLoadDeterministic(t *testing.T) {
	m := loadLFRManifest(t)
	a := loadLFRGraph(t, m.Graphs[0])
	b := loadLFRGraph(t, m.Graphs[0])

	if a.g.NumNodes() != b.g.NumNodes() {
		t.Fatalf("node counts differ: %d vs %d", a.g.NumNodes(), b.g.NumNodes())
	}
	for i := range a.planted {
		if a.planted[i] != b.planted[i] {
			t.Fatalf("planting differs at node %d: %d vs %d", i, a.planted[i], b.planted[i])
		}
	}
	// Identical structure implies identical detected partitions at a fixed seed.
	ra, err := meso.Leiden(a.g, meso.WithSeed(1))
	if err != nil {
		t.Fatalf("Leiden(a): %v", err)
	}
	rb, err := meso.Leiden(b.g, meso.WithSeed(1))
	if err != nil {
		t.Fatalf("Leiden(b): %v", err)
	}
	if ra.Quality() != rb.Quality() {
		t.Fatalf("quality differs across identical loads: %v vs %v", ra.Quality(), rb.Quality())
	}
	ca, cb := ra.Communities(), rb.Communities()
	for k, v := range ca {
		if cb[k] != v {
			t.Fatalf("community label differs for node %q: %d vs %d", k, v, cb[k])
		}
	}
}
