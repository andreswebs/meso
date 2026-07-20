package meso

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// The Lean value-oracle harness. It loads the committed golden vectors
// (verification/oracle/golden/*.json) and their matching inputs
// (verification/oracle/inputs/*.json), builds each fixture graph via the public
// Builder, and exposes the exact rational expected values (as big.Rat) plus the
// per-case predicate flags so the quality, move-delta, and guarantee tests can
// assert Go's float64 results against a proved oracle value. The committed
// vectors are the single source of truth; this harness only consumes them.
// Producing or refreshing the vectors is the Lean track (make oracle-lean).
// Design of record: docs/specs/001-initial-implementation/meso-oracle.md.

// oracleFixtureNames are the committed input sets: the tiny hand-checkable
// fixtures plus the canonical corpus graphs.
var oracleFixtureNames = []string{"triangle", "path3", "square", "karate", "dolphins", "lesmis"}

const oracleDir = "verification/oracle"

// rat parses a JSON scalar that is either a number (1, 8) or an exact rational
// string ("1/2", "p/q") into a big.Rat. Weights, sizes, and gamma are never
// floats in the committed inputs, so no float rounding enters the graph the
// oracle values were computed over.
type rat struct{ *big.Rat }

// UnmarshalJSON accepts either a JSON number or a quoted "p/q"/"n" string.
func (r *rat) UnmarshalJSON(b []byte) error {
	s := string(b)
	if len(s) >= 2 && s[0] == '"' {
		s = s[1 : len(s)-1]
	}
	v := new(big.Rat)
	if _, ok := v.SetString(s); !ok {
		return fmt.Errorf("meso oracle: %q is not an exact rational", s)
	}
	r.Rat = v
	return nil
}

// inputEdge is one undirected edge [i, j, w] from an input file; w is an int or
// "p/q" string, and a self-loop is i == j.
type inputEdge struct {
	I, J int
	W    *big.Rat
}

// UnmarshalJSON reads the fixed 3-element [i, j, w] array form.
func (e *inputEdge) UnmarshalJSON(b []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if len(raw) != 3 {
		return fmt.Errorf("meso oracle: edge has %d fields, want 3", len(raw))
	}
	if err := json.Unmarshal(raw[0], &e.I); err != nil {
		return fmt.Errorf("meso oracle: edge source: %w", err)
	}
	if err := json.Unmarshal(raw[1], &e.J); err != nil {
		return fmt.Errorf("meso oracle: edge target: %w", err)
	}
	var w rat
	if err := w.UnmarshalJSON(raw[2]); err != nil {
		return err
	}
	e.W = w.Rat
	return nil
}

// inputFile mirrors verification/oracle/inputs/*.json: the graph plus the cases
// (quality, gamma, partition, and optional requested move-deltas). Directed
// marks a directed input: each edge is a single arc from -> to, never
// symmetrised, and every case's quality is "directedModularity".
type inputFile struct {
	N         int         `json:"n"`
	Directed  bool        `json:"directed"`
	Edges     []inputEdge `json:"edges"`
	NodeSizes []rat       `json:"nodeSizes"`
	Cases     []struct {
		Quality   string `json:"quality"`
		Gamma     rat    `json:"gamma"`
		Partition []int  `json:"partition"`
		Deltas    []struct {
			Node   int `json:"node"`
			Target int `json:"target"`
		} `json:"deltas"`
	} `json:"cases"`
}

// goldenValue is the exact rational the oracle emits as num/den strings, so no
// JSON-number precision is involved. The file also carries a float "approx" for
// human review, which this harness intentionally ignores in favour of the exact
// num/den.
type goldenValue struct {
	Num string `json:"num"`
	Den string `json:"den"`
}

// rat converts the num/den strings to an exact big.Rat.
func (v goldenValue) rat() (*big.Rat, error) {
	num, ok := new(big.Int).SetString(v.Num, 10)
	if !ok {
		return nil, fmt.Errorf("meso oracle: bad numerator %q", v.Num)
	}
	den, ok := new(big.Int).SetString(v.Den, 10)
	if !ok {
		return nil, fmt.Errorf("meso oracle: bad denominator %q", v.Den)
	}
	return new(big.Rat).SetFrac(num, den), nil
}

// goldenFile mirrors verification/oracle/golden/*.json: per case the exact
// value, the requested deltas' exact values, and the predicate flags.
type goldenFile struct {
	N     int `json:"n"`
	Cases []struct {
		Quality string      `json:"quality"`
		Gamma   string      `json:"gamma"`
		Value   goldenValue `json:"value"`
		Deltas  []struct {
			Node   int         `json:"node"`
			Target int         `json:"target"`
			Value  goldenValue `json:"value"`
		} `json:"deltas"`
		Predicates struct {
			Connected      bool  `json:"connected"`
			GammaDense     *bool `json:"gammaDense"`
			GammaSeparated bool  `json:"gammaSeparated"`
			SubsetOptimal  *bool `json:"subsetOptimal"`
		} `json:"predicates"`
	} `json:"cases"`
}

// oracleDelta is one requested move-delta: reassigning Node to community Target
// under the case partition changes quality by the exact rational Want.
type oracleDelta struct {
	Node   int
	Target int
	Want   *big.Rat
}

// oraclePredicates carries the proved guarantee flags for a case partition.
// SubsetOptimal is nil when the emitter skipped it (above its node bound), and
// GammaDense is nil on a directed case (the directed predicate object is
// {connected, gammaSeparated, subsetOptimal}; gamma-density is CPM-only and
// directed CPM is unmodelled). A nil flag is a skip, not a failure.
type oraclePredicates struct {
	Connected      bool
	GammaDense     *bool
	GammaSeparated bool
	SubsetOptimal  *bool
}

// oracleCase is one (quality, gamma, partition) case with its exact expected
// value, requested deltas, and predicate flags.
type oracleCase struct {
	Quality    string // "modularity" or "cpm"
	Gamma      *big.Rat
	Partition  Partition
	Want       *big.Rat
	Deltas     []oracleDelta
	Predicates oraclePredicates
}

// oracleFixture is a loaded input set: the built graph and its cases, joined
// from the input and golden files. Directed reports that the graph was built
// through NewDirectedBuilder from single arcs.
type oracleFixture struct {
	Name     string
	Directed bool
	Graph    *Graph
	Cases    []oracleCase
}

// loadOracleFixture reads the input and golden files for name, joins their
// per-index cases, and builds the fixture graph via the public Builder. It fails
// the test on any parse, join, or build error.
func loadOracleFixture(t *testing.T, name string) *oracleFixture {
	t.Helper()

	var in inputFile
	readOracleJSON(t, filepath.Join(oracleDir, "inputs", name+".json"), &in)
	var gold goldenFile
	readOracleJSON(t, filepath.Join(oracleDir, "golden", name+".json"), &gold)

	if in.N != gold.N {
		t.Fatalf("%s: input n=%d, golden n=%d", name, in.N, gold.N)
	}
	if len(in.Cases) != len(gold.Cases) {
		t.Fatalf("%s: input has %d cases, golden has %d", name, len(in.Cases), len(gold.Cases))
	}

	f := &oracleFixture{Name: name, Directed: in.Directed, Graph: buildOracleGraph(t, name, &in)}
	for i := range in.Cases {
		ic, gc := in.Cases[i], gold.Cases[i]

		// The input and golden case streams are joined by index; confirm they
		// agree on quality and gamma so a misaligned file is caught, not read as
		// the wrong expected value.
		if ic.Quality != gc.Quality {
			t.Fatalf("%s case %d: input quality %q, golden %q", name, i, ic.Quality, gc.Quality)
		}
		goldGamma := new(big.Rat)
		if _, ok := goldGamma.SetString(gc.Gamma); !ok {
			t.Fatalf("%s case %d: bad golden gamma %q", name, i, gc.Gamma)
		}
		if ic.Gamma.Cmp(goldGamma) != 0 {
			t.Fatalf("%s case %d: input gamma %s, golden %s", name, i, ic.Gamma.RatString(), goldGamma.RatString())
		}

		want, err := gc.Value.rat()
		if err != nil {
			t.Fatalf("%s case %d: %v", name, i, err)
		}

		deltas := make([]oracleDelta, len(gc.Deltas))
		for j, gd := range gc.Deltas {
			dw, err := gd.Value.rat()
			if err != nil {
				t.Fatalf("%s case %d delta %d: %v", name, i, j, err)
			}
			deltas[j] = oracleDelta{Node: gd.Node, Target: gd.Target, Want: dw}
		}

		f.Cases = append(f.Cases, oracleCase{
			Quality:   ic.Quality,
			Gamma:     ic.Gamma.Rat,
			Partition: Partition(ic.Partition),
			Want:      want,
			Deltas:    deltas,
			Predicates: oraclePredicates{
				Connected:      gc.Predicates.Connected,
				GammaDense:     gc.Predicates.GammaDense,
				GammaSeparated: gc.Predicates.GammaSeparated,
				SubsetOptimal:  gc.Predicates.SubsetOptimal,
			},
		})
	}
	return f
}

// Go-vs-Lean comparison tolerance (docs/specs/001-initial-implementation/meso-oracle.md,
// "Go-vs-Lean comparison tolerance"). The reference value is the correctly-rounded
// rational, not an arbitrary float, so the check measures how far Go is from optimal
// rounding rather than comparing two unrelated float pipelines. A case passes iff the
// ULP distance is within a per-quality budget OR the absolute error is within atol.
const (
	// qualityBudgetULP bounds the modularity and CPM value checks. The metric is
	// relative (ULP distance), so CPM's large magnitude needs no larger budget.
	qualityBudgetULP = 16
	// deltaBudgetULP is the tighter budget for the move-delta: a difference of
	// quantities with the most cancellation risk and the highest-value bug catch.
	deltaBudgetULP = 4
	// atol is the absolute floor. ULP distance explodes near zero (adjacent floats
	// there are denormals), so every exact-zero case (square's CPM = 0, karate's
	// all-in-one modularity = 0, the no-op move-delta) would fail a pure ULP check
	// without it. 1e-12 sits well above float64 noise (~1e-16) and well below any
	// partition-quality difference that could matter.
	atol = 1e-12
)

// wantFloat rounds an exact rational to the nearest float64 (round-to-nearest-even):
// the best any float64 pipeline can produce, and the reference the tolerance is
// measured against.
func wantFloat(r *big.Rat) float64 {
	f, _ := r.Float64()
	return f
}

// withinTolerance reports whether got matches the exact rational want within the
// combined policy: ULP distance <= budget OR absolute error <= atol.
func withinTolerance(got float64, want *big.Rat, budgetULP uint64) bool {
	w := wantFloat(want)
	if math.Abs(got-w) <= atol {
		return true
	}
	return ulpDiff(got, w) <= budgetULP
}

// ulpDiff returns the distance in units in the last place between two float64s,
// treating +-0 as adjacent and any NaN/Inf as infinitely far. It maps each float
// to a sign-magnitude-ordered uint64 so that adjacent floats differ by 1.
func ulpDiff(a, b float64) uint64 {
	if math.IsNaN(a) || math.IsNaN(b) || math.IsInf(a, 0) || math.IsInf(b, 0) {
		return math.MaxUint64
	}
	ua, ub := monotonicBits(a), monotonicBits(b)
	if ua > ub {
		return ua - ub
	}
	return ub - ua
}

// monotonicBits reinterprets a float64 as a uint64 whose unsigned order matches
// the float order, so subtraction yields ULP distance across the sign boundary.
func monotonicBits(f float64) uint64 {
	b := math.Float64bits(f)
	if b>>63 == 1 {
		return ^b // negative: flip all bits
	}
	return b | (1 << 63) // non-negative: flip the sign bit
}

// goQuality evaluates the Go quality function for a case: modularity as is (it
// matches igraph directly) and CPM in the canonical (leidenalg) convention the
// oracle emits.
func (f *oracleFixture) goQuality(t *testing.T, c oracleCase) float64 {
	t.Helper()
	gamma, _ := c.Gamma.Float64()
	g := f.Graph.model
	switch c.Quality {
	case "modularity":
		return modularity{gamma: gamma}.Quality(g, c.Partition)
	case "cpm":
		return canonicalCPM(g, gamma, c.Partition)
	case "directedModularity":
		return directedModularity{gamma: gamma}.Quality(g, c.Partition)
	default:
		t.Fatalf("meso oracle: unknown quality %q", c.Quality)
		return 0
	}
}

// goDelta evaluates the Go incremental move-delta for a requested move under a
// case's quality function. The canonical CPM correction is partition-independent,
// so the CPM delta needs no adjustment (Lean cpmCanonicalQ_eq).
func (f *oracleFixture) goDelta(t *testing.T, c oracleCase, d oracleDelta) float64 {
	t.Helper()
	gamma, _ := c.Gamma.Float64()
	g := f.Graph.model
	switch c.Quality {
	case "modularity":
		return modularity{gamma: gamma}.moveDelta(g, c.Partition, d.Node, d.Target)
	case "cpm":
		return cpm{gamma: gamma}.moveDelta(g, c.Partition, d.Node, d.Target)
	case "directedModularity":
		return directedModularity{gamma: gamma}.moveDelta(g, c.Partition, d.Node, d.Target)
	default:
		t.Fatalf("meso oracle: unknown quality %q", c.Quality)
		return 0
	}
}

// canonicalCPM evaluates CPM in the canonical (leidenalg) convention the oracle
// emits, so meso's CPM numbers match the published literature. The Go cpm
// evaluator computes the proof-side objective (diagonal i == j terms included);
// the canonical convention drops the partition-independent diagonal
// D = sum_i (w_ii - gamma*s_i^2), so canonical = cpmQ - D. Subtracting a
// partition-independent constant leaves the optimum and the move-deltas
// unchanged (Lean cpmCanonicalQ_eq), which is why the incremental move-delta
// needs no correction.
func canonicalCPM(g *csr, gamma float64, p Partition) float64 {
	q := cpm{gamma: gamma}.Quality(g, p)
	var diag float64
	for i := 0; i < g.numNodes(); i++ {
		si := g.nodeSize(i)
		diag += g.weight(i, i) - gamma*si*si
	}
	return q - diag
}

// readOracleJSON reads and strictly decodes a committed oracle JSON file into v.
func readOracleJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
}

// buildOracleGraph builds the fixture graph through the public Builder. Nodes
// 0..n-1 are registered in index order first (so the dense index equals the
// input's node index and the case partitions line up), then the edges are
// added; node sizes default to 1 when the input omits them. A directed input
// builds through NewDirectedBuilder, each edge added once as the arc from -> to.
func buildOracleGraph(t *testing.T, name string, in *inputFile) *Graph {
	t.Helper()
	b := NewBuilder()
	if in.Directed {
		b = NewDirectedBuilder()
	}
	for i := 0; i < in.N; i++ {
		size := 1.0
		if i < len(in.NodeSizes) {
			size, _ = in.NodeSizes[i].Float64()
		}
		b.AddNodeWeight(strconv.Itoa(i), size)
	}
	for _, e := range in.Edges {
		w, _ := e.W.Float64()
		b.AddEdge(strconv.Itoa(e.I), strconv.Itoa(e.J), w)
	}
	g, err := b.Build()
	if err != nil {
		t.Fatalf("%s: build graph: %v", name, err)
	}
	if g.NumNodes() != in.N {
		t.Fatalf("%s: built %d nodes, want %d", name, g.NumNodes(), in.N)
	}
	return g
}
