package meso

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// updateGolden regenerates the committed corpus golden files instead of
// asserting against them: `go test -run TestGoldenCorpus -update`. It is the
// standard Go golden idiom, so a deliberate, reviewed change to meso's
// deterministic output is a one-command refresh rather than a hand edit.
var updateGolden = flag.Bool("update", false, "regenerate golden corpus files in testdata/golden")

// corpusGraph names one canonical benchmark graph and the published-literature
// modularity envelope its Louvain partition must land inside. The envelopes are
// deliberately loose sanity bands around the published optima (they bracket the
// literature value, not meso's exact number): karate's modularity optimum is
// about 0.4198, dolphins about 0.52, and Les Miserables about 0.56. The exact
// pinned partition and quality live in the golden file; this band only guards
// against a grossly broken optimiser. The exact-rational value-oracle check on
// meso's own converged partition lives in the oracle harness (the committed
// converged case in verification/oracle, pinned by TestOracleConvergedPartitionPinned).
type corpusGraph struct {
	name    string
	gml     string
	loQ     float64
	hiQ     float64
	minComm int // a correct partition merges nodes: fewer communities than nodes
}

var corpus = []corpusGraph{
	{name: "karate", gml: "datasets/karate/karate.gml", loQ: 0.40, hiQ: 0.43, minComm: 2},
	{name: "dolphins", gml: "datasets/dolphins/dolphins.gml", loQ: 0.49, hiQ: 0.53, minComm: 2},
	{name: "lesmis", gml: "datasets/lesmis/lesmis.gml", loQ: 0.53, hiQ: 0.57, minComm: 2},
}

// goldenCorpus is the committed, byte-identical record of meso's deterministic
// Louvain output on one corpus graph: the canonical partition (every node keyed
// by its GML id, communities relabelled in ascending-node order) and the
// achieved modularity as its shortest round-trippable decimal. Because meso is
// deterministic, this file is a pure function of (graph, options, seed) and any
// drift in a future change is caught as a golden diff.
type goldenCorpus struct {
	Graph       string         `json:"graph"`
	Algorithm   string         `json:"algorithm"`
	Quality     string         `json:"quality"`
	Gamma       float64        `json:"gamma"`
	Seed        uint64         `json:"seed"`
	Communities int            `json:"communities"`
	Modularity  string         `json:"modularity"`
	Partition   []goldenAssign `json:"partition"`
}

// goldenAssign is one node-to-community assignment in a golden partition.
type goldenAssign struct {
	Node      string `json:"node"`
	Community int    `json:"community"`
}

// TestGoldenCorpus pins meso's deterministic Louvain partition and quality on
// the canonical corpus (karate, dolphins, Les Miserables) as committed golden
// files, cross-checks the achieved modularity against the published-literature
// envelope, and confirms a second run is byte-identical. Regenerate with
// -update after a reviewed change to the deterministic output.
func TestGoldenCorpus(t *testing.T) {
	for _, cg := range corpus {
		t.Run(cg.name, func(t *testing.T) {
			g := loadGMLGraph(t, cg.gml)

			got := runGoldenCorpus(t, g, cg.name)

			path := filepath.Join("testdata", "golden", cg.name+".json")
			if *updateGolden {
				writeGolden(t, path, got)
			}

			want := readGolden(t, path)
			assertGoldenEqual(t, cg.name, got, want)

			// Published-literature sanity band: a grossly broken optimiser is
			// caught even if the golden file were regenerated against it.
			if got.mod < cg.loQ || got.mod > cg.hiQ {
				t.Errorf("%s modularity = %v, want within literature band [%v, %v]",
					cg.name, got.mod, cg.loQ, cg.hiQ)
			}
			if got.corpus.Communities < cg.minComm || got.corpus.Communities >= g.NumNodes() {
				t.Errorf("%s communities = %d, want in [%d, %d)",
					cg.name, got.corpus.Communities, cg.minComm, g.NumNodes())
			}

			// Determinism: a second independent run is byte-identical.
			again := runGoldenCorpus(t, g, cg.name)
			assertGoldenEqual(t, cg.name+" (rerun)", again, want)
		})
	}
}

// goldenResult carries the marshalled golden record plus the raw modularity
// float, so the envelope check reads the exact value while the golden compare
// works on the committed bytes.
type goldenResult struct {
	corpus goldenCorpus
	mod    float64
}

// runGoldenCorpus runs Louvain at the default configuration (modularity gamma=1,
// seed 0) and canonicalises the result into a committable golden record.
func runGoldenCorpus(t *testing.T, g *Graph, name string) goldenResult {
	t.Helper()
	res, err := Louvain(g)
	if err != nil {
		t.Fatalf("Louvain(%s): %v", name, err)
	}
	comm := res.Communities()
	labels, count := canonicalLabels(comm)

	keys := make([]string, 0, len(comm))
	for k := range comm {
		keys = append(keys, k)
	}
	sort.Sort(byNumericKey(keys))

	part := make([]goldenAssign, len(keys))
	for i, k := range keys {
		part[i] = goldenAssign{Node: k, Community: labels[comm[k]]}
	}

	return goldenResult{
		corpus: goldenCorpus{
			Graph:       name,
			Algorithm:   "louvain",
			Quality:     "modularity",
			Gamma:       1,
			Seed:        0,
			Communities: count,
			Modularity:  strconv.FormatFloat(res.Quality(), 'g', -1, 64),
			Partition:   part,
		},
		mod: res.Quality(),
	}
}

// canonicalLabels relabels community ids so they are numbered in ascending order
// of their smallest member's numeric node key, making the golden partition
// independent of the internal label meso happened to assign. It returns the
// old-to-new label map and the community count.
func canonicalLabels(comm map[string]int) (map[int]int, int) {
	keys := make([]string, 0, len(comm))
	for k := range comm {
		keys = append(keys, k)
	}
	sort.Sort(byNumericKey(keys))

	labels := make(map[int]int)
	next := 0
	for _, k := range keys {
		if _, seen := labels[comm[k]]; !seen {
			labels[comm[k]] = next
			next++
		}
	}
	return labels, next
}

// byNumericKey sorts node keys by their integer value when both parse as
// integers (the GML corpus uses numeric ids), falling back to lexicographic
// order otherwise, so the golden partition lists nodes 0,1,2,...,10 rather than
// 0,1,10,2.
type byNumericKey []string

func (s byNumericKey) Len() int      { return len(s) }
func (s byNumericKey) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s byNumericKey) Less(i, j int) bool {
	ai, aerr := strconv.Atoi(s[i])
	bi, berr := strconv.Atoi(s[j])
	if aerr == nil && berr == nil {
		return ai < bi
	}
	return s[i] < s[j]
}

// gmlEdge is one edge block of a GML file: source and target ids and the
// weight from its value or weight attribute (1 when absent).
type gmlEdge struct {
	src, tgt string
	w        float64
}

// parseGML reads the node ids and edge blocks of a GML file, without folding.
// Node ids come back in ascending numeric order.
func parseGML(t testing.TB, path string) (nodes []string, edges []gmlEdge) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var block string // "node" or "edge" or ""
	var src, tgt string
	var w float64
	var haveSrc, haveTgt bool
	for line := range strings.SplitSeq(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		switch f[0] {
		case "node":
			block = "node"
		case "edge":
			block, haveSrc, haveTgt, w = "edge", false, false, 1.0
		case "id":
			if block == "node" && len(f) >= 2 {
				nodes = append(nodes, f[1])
			}
		case "source":
			if block == "edge" && len(f) >= 2 {
				src, haveSrc = f[1], true
			}
		case "target":
			if block == "edge" && len(f) >= 2 {
				tgt, haveTgt = f[1], true
			}
		case "value", "weight":
			if block == "edge" && len(f) >= 2 {
				if v, perr := strconv.ParseFloat(f[1], 64); perr == nil {
					w = v
				}
			}
		case "]":
			if block == "edge" && haveSrc && haveTgt {
				edges = append(edges, gmlEdge{src, tgt, w})
			}
			block = ""
		}
	}
	sort.Sort(byNumericKey(nodes))
	return nodes, edges
}

// loadGMLGraph reads a Newman-format GML graph into a public *Graph via the
// Builder. Nodes are registered in ascending numeric id order first (so isolated
// nodes survive and the labelling is order-independent), then edges are added;
// an edge weight comes from the GML value/weight attribute, defaulting to 1.
func loadGMLGraph(t testing.TB, path string) *Graph {
	t.Helper()
	nodes, edges := parseGML(t, path)
	b := NewBuilder()
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

// writeGolden marshals a golden record to path with a trailing newline, creating
// the directory if needed. Only reached under -update.
func writeGolden(t *testing.T, path string, r goldenResult) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	data, err := json.MarshalIndent(r.corpus, "", "  ")
	if err != nil {
		t.Fatalf("marshal golden %s: %v", path, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// readGolden loads a committed golden record, failing with an actionable hint to
// run -update when the file is missing.
func readGolden(t *testing.T, path string) goldenCorpus {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run: go test -run TestGoldenCorpus -update)", path, err)
	}
	var g goldenCorpus
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatalf("parse golden %s: %v", path, err)
	}
	return g
}

// assertGoldenEqual compares a fresh run against the committed golden record,
// re-marshalling both so the check is on canonical bytes and a mismatch prints
// the differing field.
func assertGoldenEqual(t *testing.T, label string, got goldenResult, want goldenCorpus) {
	t.Helper()
	gotBytes, err := json.MarshalIndent(got.corpus, "", "  ")
	if err != nil {
		t.Fatalf("%s: marshal got: %v", label, err)
	}
	wantBytes, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		t.Fatalf("%s: marshal want: %v", label, err)
	}
	if string(gotBytes) != string(wantBytes) {
		t.Errorf("%s: golden mismatch (run -update if intended)\n got:  %s\n want: %s",
			label, gotBytes, wantBytes)
	}
}
