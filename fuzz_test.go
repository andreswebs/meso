package meso

import (
	"strconv"
	"testing"
)

// Native fuzz targets (mes-y8ru, step 12): feed a Builder from fuzzed bytes, run
// the public Leiden and Louvain over the built graph, and assert on every output
// the invariants the verified core promises - the run never panics, always
// returns a well-formed partition, never loses quality against the all-singletons
// baseline it starts from, and (for Leiden) leaves every community connected.
// Self-loop and multi-edge fuzzed inputs must fold correctly and never break the
// 2m total-weight bookkeeping. Design of record: docs/meso-design.md section 6.3;
// step 12 of docs/specs/001-initial-implementation/plan.md.
//
// These targets are CI-cheap by construction: `go test` (and thus `make validate`)
// runs each target once per committed seed, exercising the corpus of degenerate
// and adversarial shapes below without a mutation budget. A real mutation search
// is on-demand via `make fuzz` (or `go test -run=^$ -fuzz=Fuzz...`).

// genOp is one decoded builder operation: an edge from -> to of weight w, or,
// when node is set, a node-weight assignment of size w to `from`.
type genOp struct {
	from, to int
	w        float64
	node     bool
}

// decodeGraph turns fuzz bytes into a node count and a list of builder ops. The
// first byte caps the node count in [1, 24] (small enough to keep the O(n^2)
// connectivity and quality checks cheap under a mutation search); the remaining
// bytes are read in 3-byte records (from, to, control). A record whose control
// byte has the high bit set assigns a node weight to `from`; otherwise it adds an
// edge from -> to, a self-loop when from == to. Edge weights are strictly
// positive so connectivity is unambiguous (a zero-weight pair is a non-edge);
// repeated records on the same pair fold and self-loops accumulate. Empty input
// decodes to the empty graph.
func decodeGraph(data []byte) (int, []genOp) {
	if len(data) == 0 {
		return 0, nil
	}
	maxN := 1 + int(data[0])%24
	var ops []genOp
	for i := 1; i+2 < len(data); i += 3 {
		from := int(data[i]) % maxN
		to := int(data[i+1]) % maxN
		ctrl := data[i+2]
		if ctrl&0x80 != 0 {
			ops = append(ops, genOp{from: from, w: float64(ctrl&0x7f) / 8, node: true})
		} else {
			ops = append(ops, genOp{from: from, to: to, w: 1 + float64(ctrl)/8})
		}
	}
	return maxN, ops
}

// buildFuzzGraph builds the decoded graph through the public Builder, registering
// all n nodes up front (so unreferenced indices become isolated nodes) and then
// applying the ops, including node-weight assignments. directed selects
// NewDirectedBuilder. Node weights are always applied: CPM scales its resolution
// penalty by node size, and while modularity's null model is blind to node sizes,
// its refinement gate must stay objective-appropriate over them (bug wor-w33p), so
// both objectives are fuzzed over node-weighted graphs. Build must not fail: the
// decoder only emits finite non-negative weights and sizes.
func buildFuzzGraph(t *testing.T, n int, ops []genOp, directed bool) *Graph {
	t.Helper()
	var b *Builder
	if directed {
		b = NewDirectedBuilder()
	} else {
		b = NewBuilder()
	}
	for i := range n {
		b.AddNodeWeight(strconv.Itoa(i), 1.0)
	}
	for _, op := range ops {
		if op.node {
			b.AddNodeWeight(strconv.Itoa(op.from), op.w)
		} else {
			b.AddEdge(strconv.Itoa(op.from), strconv.Itoa(op.to), op.w)
		}
	}
	g, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed on decoded graph (n=%d, %d ops): %v", n, len(ops), err)
	}
	return g
}

// checkOutputInvariants asserts the shared step-7 invariants on a run output p of
// objective obj over graph g: p is a well-formed partition (every node in exactly
// one community, labels in range), the community count is in [0, n], and p's
// quality is no worse than the all-singletons partition the run starts from
// (monotone improvement, since local moving only accepts non-negative deltas and
// aggregation preserves quality). connectivity is asserted only when checkConn is
// set - a connected community per community is Leiden's guarantee, not Louvain's,
// and not a formal guarantee for the directed objective.
func checkOutputInvariants(t *testing.T, tag string, g *csr, obj objective, p Partition, checkConn bool) {
	t.Helper()
	n := g.numNodes()
	if err := p.wellFormed(n); err != nil {
		t.Fatalf("%s: output not a well-formed partition: %v", tag, err)
	}
	if nc := numCommunities(p); nc < 0 || nc > n {
		t.Fatalf("%s: community count %d out of range [0, %d]", tag, nc, n)
	}
	base := obj.Quality(g, singleton(n))
	got := obj.Quality(g, p)
	if got < base-louvainTol {
		t.Fatalf("%s: output quality %v below singleton baseline %v", tag, got, base)
	}
	if checkConn && !connectedCommunities(g, p) {
		t.Fatalf("%s: output has a disconnected community", tag)
	}
}

// FuzzLeidenLouvain is acceptance criteria 1 and 2: over arbitrary decoded
// undirected graphs, the public Leiden and Louvain never panic and always return
// a well-formed, quality-monotone partition, with Leiden's communities connected.
// Modularity and CPM are both exercised (CPM at a fuzz-driven resolution), and the
// synchronous-round parallel local-move path is checked to be byte-identical
// across worker counts (design section 4.5) while satisfying the same invariants.
func FuzzLeidenLouvain(f *testing.F) {
	seeds := [][]byte{
		{},                             // empty graph
		{0},                            // single isolated node
		{2, 0, 1, 8, 1, 2, 8, 0, 2, 8}, // triangle
		{3, 0, 1, 8, 2, 3, 8},          // two disjoint edges: multiple components
		{1, 0, 0, 8},                   // self-loop
		{1, 0, 1, 8, 0, 1, 40},         // parallel edges on one pair (folding)
		{5, 0, 1, 8, 1, 2, 16, 2, 0, 24, 3, 4, 8, 0, 0x83},
		{9, 0, 1, 8, 1, 2, 8, 2, 3, 8, 3, 4, 8, 4, 5, 8, 5, 6, 8, 6, 7, 8, 7, 8, 8},
	}
	for _, s := range seeds {
		f.Add(s, uint64(1), byte(16))
	}
	// wor-w33p reproduction: a node-weighted graph on which modularity Leiden
	// returned a disconnected community until the refinement gate was made
	// objective-appropriate.
	f.Add([]byte("%CYc02v72X00\xf7290Y0VC2A11b88011029X720C7x00\x0680\xf901\n1Ab98X27a1A9810"),
		uint64(111), byte(0x19))

	f.Fuzz(func(t *testing.T, data []byte, seed uint64, gammaByte byte) {
		n, ops := decodeGraph(data)
		gamma := 0.25 + float64(gammaByte)/32

		// Both objectives run over the fuzzed node weights. CPM's resolution
		// penalty scales with node size; modularity's null model ignores node
		// sizes, but its refinement connectivity guarantee must hold over them all
		// the same (bug wor-w33p), so node-weighted graphs are exactly where that
		// guarantee needs fuzzing.
		g := buildFuzzGraph(t, n, ops, false)
		runAndCheck(t, g, Modularity(1.0), modularity{gamma: 1.0}, seed)
		runAndCheck(t, g, CPM(gamma), cpm{gamma: gamma}, seed)
	})
}

// runAndCheck runs Leiden and Louvain (serial and, for Leiden, the parallel
// synchronous-round path) over g under objective obj and asserts the shared
// invariants on every output: never an error, well-formed and quality-monotone
// partitions, Leiden's communities connected, parallel Leiden byte-identical
// across worker counts, and the across-level quality/termination invariants.
func runAndCheck(t *testing.T, g *Graph, q QualityFunction, obj objective, seed uint64) {
	t.Helper()

	lr, err := Leiden(g, WithQuality(q), WithSeed(seed))
	if err != nil {
		t.Fatalf("Leiden(%T) returned error: %v", obj, err)
	}
	checkOutputInvariants(t, "leiden", g.model, obj, lr.part, true)

	vr, err := Louvain(g, WithQuality(q))
	if err != nil {
		t.Fatalf("Louvain(%T) returned error: %v", obj, err)
	}
	checkOutputInvariants(t, "louvain", g.model, obj, vr.part, false)

	p1, err := Leiden(g, WithQuality(q), WithSeed(seed), WithParallelism(1))
	if err != nil {
		t.Fatalf("parallel Leiden(1) error: %v", err)
	}
	p4, err := Leiden(g, WithQuality(q), WithSeed(seed), WithParallelism(4))
	if err != nil {
		t.Fatalf("parallel Leiden(4) error: %v", err)
	}
	if !partitionsEqual(p1.part, p4.part) {
		t.Fatalf("%T: parallel Leiden not byte-identical across 1 vs 4 workers", obj)
	}
	checkOutputInvariants(t, "leiden-parallel", g.model, obj, p4.part, true)

	checkLevelInvariants(t, g.model, obj, seed)
}

// checkLevelInvariants walks the aggregation levels of a full Leiden run (via the
// leidenTrace helper that mirrors leiden() exactly) and asserts the across-level
// invariants: the lifted base-graph quality is monotone non-decreasing from one
// level to the next, and the recursion terminates - at most n levels, each running
// on strictly fewer working nodes than the last. Together these are the fuzzed-input
// image of TestLeidenRun_QualityMonotoneAcrossLevels and TestLeidenRun_LevelsTerminate.
func checkLevelInvariants(t *testing.T, g *csr, obj objective, seed uint64) {
	t.Helper()
	n := g.numNodes()
	levels := leidenTrace(g, obj, seed)
	if n > 0 && len(levels) > n {
		t.Fatalf("%T: %d levels exceeds node bound %d", obj, len(levels), n)
	}
	prev := obj.Quality(g, levels[0].base)
	for l := 1; l < len(levels); l++ {
		q := obj.Quality(g, levels[l].base)
		if q < prev-louvainTol {
			t.Fatalf("%T: quality dropped at level %d: %v -> %v", obj, l, prev, q)
		}
		prev = q
		if levels[l].working >= levels[l-1].working {
			t.Fatalf("%T: level %d working size %d did not shrink from %d",
				obj, l, levels[l].working, levels[l-1].working)
		}
	}
}

// FuzzFoldingTwoM is acceptance criterion 3: self-loop and multi-edge fuzzed
// inputs fold correctly and never break the 2m bookkeeping. It rebuilds the folded
// expectation independently from the decoded ops and asserts the built CSR matches
// - each node's neighbour list is deduplicated and symmetric, self-loop weights
// accumulate, and twoM equals 2*(off-diagonal weight) + (self-loop weight), a
// finite non-negative number.
func FuzzFoldingTwoM(f *testing.F) {
	seeds := [][]byte{
		{},
		{1, 0, 0, 8},                     // single self-loop
		{1, 0, 0, 8, 0, 0, 40},           // accumulating self-loops
		{1, 0, 1, 8, 0, 1, 16, 1, 0, 24}, // parallel and reversed edges fold to one
		{4, 0, 1, 8, 1, 2, 16, 2, 3, 24, 3, 0, 32, 1, 1, 8},
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		n, ops := decodeGraph(data)
		g := buildFuzzGraph(t, n, ops, false)
		m := g.model

		wantSelf := make([]float64, n)
		wantOff := make(map[[2]int]float64)
		for _, op := range ops {
			if op.node {
				continue
			}
			if op.from == op.to {
				wantSelf[op.from] += op.w
				continue
			}
			a, b := op.from, op.to
			if a > b {
				a, b = b, a
			}
			wantOff[[2]int{a, b}] += op.w
		}

		for i := range n {
			if !floatClose(m.selfLoops[i], wantSelf[i]) {
				t.Fatalf("node %d self-loop = %v, folded want %v", i, m.selfLoops[i], wantSelf[i])
			}
			nb := m.neighbors(i)
			nw := m.neighborWeights(i)
			seen := make(map[int]bool, len(nb))
			for k, v := range nb {
				if seen[v] {
					t.Fatalf("node %d has duplicate neighbour %d (edges not folded)", i, v)
				}
				seen[v] = true
				if v == i {
					t.Fatalf("node %d lists itself as an off-diagonal neighbour", i)
				}
				a, b := i, v
				if a > b {
					a, b = b, a
				}
				if !floatClose(nw[k], wantOff[[2]int{a, b}]) {
					t.Fatalf("edge (%d,%d) weight = %v, folded want %v", i, v, nw[k], wantOff[[2]int{a, b}])
				}
				if rev := m.weight(v, i); !floatClose(rev, nw[k]) {
					t.Fatalf("edge (%d,%d) asymmetric: %v vs %v", i, v, nw[k], rev)
				}
			}
		}

		var wantTwoM float64
		for _, w := range wantOff {
			wantTwoM += 2 * w
		}
		for _, w := range wantSelf {
			wantTwoM += w
		}
		got := m.twoM()
		if got < 0 {
			t.Fatalf("twoM = %v, must be non-negative", got)
		}
		if got != got { // NaN guard
			t.Fatalf("twoM is NaN")
		}
		if !floatClose(got, wantTwoM) {
			t.Fatalf("twoM = %v, independently folded want %v", got, wantTwoM)
		}
	})
}

// FuzzDirected hardens the directed path (M3): over arbitrary decoded directed
// graphs, the public Leiden and Louvain under DirectedModularity never panic and
// always return a well-formed, quality-monotone partition. Connectivity is not
// asserted - it is a formal guarantee only for the undirected objective, and
// directed is reference-tested rather than verified (design section 7).
func FuzzDirected(f *testing.F) {
	seeds := [][]byte{
		{},
		{0},
		{2, 0, 1, 8, 1, 2, 8, 2, 0, 8}, // directed cycle
		{3, 0, 1, 8, 1, 0, 16, 2, 3, 8},
	}
	for _, s := range seeds {
		f.Add(s, uint64(1))
	}

	f.Fuzz(func(t *testing.T, data []byte, seed uint64) {
		n, ops := decodeGraph(data)
		g := buildFuzzGraph(t, n, ops, true)
		obj := directedModularity{gamma: 1.0}

		lr, err := Leiden(g, WithQuality(DirectedModularity(1.0)), WithSeed(seed))
		if err != nil {
			t.Fatalf("directed Leiden error: %v", err)
		}
		checkOutputInvariants(t, "directed-leiden", g.model, obj, lr.part, false)

		vr, err := Louvain(g, WithQuality(DirectedModularity(1.0)))
		if err != nil {
			t.Fatalf("directed Louvain error: %v", err)
		}
		checkOutputInvariants(t, "directed-louvain", g.model, obj, vr.part, false)
	})
}

// TestLeiden_ModularityNodeWeightsConnected is the wor-w33p regression test: the
// native fuzzer found an undirected, node-weighted graph on which Leiden under
// modularity returned a DISCONNECTED community ({0,1,5,6}, splitting into {0,1}
// and {5,6} with no edge between the halves) that was also lower quality than a
// connected alternative - violating the Leiden connectivity guarantee
// (docs/meso-design.md lines 86, 280) and the quality claim at once. The root
// cause was the refinement well-connectedness gate reading node sizes (the CPM
// density criterion) even under modularity, whose objective is blind to node
// sizes; with sizes large relative to edge weights the gate rejected every merge,
// refinement became a no-op, and Leiden degenerated to Louvain. Any seed
// reproduces because the gate is deterministic. The fix makes the gate
// objective-appropriate, so modularity refinement is no longer disabled by node
// sizes and every returned community is connected.
func TestLeiden_ModularityNodeWeightsConnected(t *testing.T) {
	data := []byte("%CYc02v72X00\xf7290Y0VC2A11b88011029X720C7x00\x0680\xf901\n1Ab98X27a1A9810")
	n, ops := decodeGraph(data)
	g := buildFuzzGraph(t, n, ops, false) // undirected, node weights applied

	for _, seed := range []uint64{0, 1, 13, 111} {
		r, err := Leiden(g, WithQuality(Modularity(1.0)), WithSeed(seed))
		if err != nil {
			t.Fatalf("seed %d: Leiden returned error: %v", seed, err)
		}
		if !connectedCommunities(g.model, r.part) {
			t.Errorf("seed %d: Leiden under modularity returned a disconnected community: %v",
				seed, r.part)
		}
	}
}

// partitionsEqual reports whether two partitions are element-wise identical, the
// byte-identical determinism check for the parallel local-move path.
func partitionsEqual(a, b Partition) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// floatClose reports whether x and y agree within the run tolerance, scaled by
// magnitude, for the folded-weight cross-checks.
func floatClose(x, y float64) bool {
	d := x - y
	if d < 0 {
		d = -d
	}
	scale := absf(x)
	if absf(y) > scale {
		scale = absf(y)
	}
	return d <= louvainTol*(1+scale)
}
