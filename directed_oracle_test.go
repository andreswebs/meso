package meso

import "testing"

// The directed value-oracle harness (directed verification phase 5, epic
// mes-crz3): the directed twins of the oracle_harness_test.go acceptance tests.
// Each committed directed fixture carries exact directed-modularity values and
// move-deltas emitted by the Lean mesoOracle from mirrors proved equal to the
// real directed model (Meso/DirectedCompute.lean: directedModularityQ_eq,
// moveDeltaDirectedModularityQ_eq), so the assertions below hold Go's float64
// directed pipeline to proved rational values, exactly as the undirected core.

// directedOracleFixtureNames are the committed directed input sets. They are
// deliberately separate from oracleFixtureNames: the undirected TestOracle*
// tests build undirected graphs and switch on modularity/cpm, so a directed
// fixture there would build the wrong graph or hit an unknown-quality fatal.
var directedOracleFixtureNames = []string{
	"directed_asym3", "directed_subset4", "directed_cycles6", "celegansneural",
}

// TestOracleLoadsDirected is the directed tracer: the loader reads a committed
// directed fixture, builds the graph through NewDirectedBuilder (asymmetric
// arcs preserved), and round-trips the exact values.
func TestOracleLoadsDirected(t *testing.T) {
	f := loadOracleFixture(t, "directed_asym3")

	if !f.Directed {
		t.Fatal("directed_asym3 not marked directed")
	}
	if !f.Graph.model.directed {
		t.Fatal("directed_asym3 graph model is not directed")
	}
	if f.Graph.NumNodes() != 3 {
		t.Fatalf("directed_asym3 NumNodes = %d, want 3", f.Graph.NumNodes())
	}
	// The arc 0 -> 1 has weight 2 and its reverse does not exist: the builder
	// must not have symmetrised.
	if w := f.Graph.model.weight(0, 1); w != 2 {
		t.Errorf("weight(0, 1) = %v, want 2", w)
	}
	if w := f.Graph.model.weight(2, 0); w != 0 {
		t.Errorf("weight(2, 0) = %v, want 0 (no reverse arc)", w)
	}
	for i, c := range f.Cases {
		if c.Quality != "directedModularity" {
			t.Errorf("case %d quality = %q, want directedModularity", i, c.Quality)
		}
		if c.Want == nil {
			t.Errorf("case %d: nil value", i)
		}
	}
}

// TestOracle_DirectedQualityValues checks that Go's directed modularity of each
// case partition lands within tolerance of the exact rational oracle value,
// across every directed fixture.
func TestOracle_DirectedQualityValues(t *testing.T) {
	checked := 0
	for _, name := range directedOracleFixtureNames {
		t.Run(name, func(t *testing.T) {
			f := loadOracleFixture(t, name)
			for i, c := range f.Cases {
				got := f.goQuality(t, c)
				if !withinTolerance(got, c.Want, qualityBudgetULP) {
					t.Errorf("case %d: got %v, want %v (%s)", i, got, wantFloat(c.Want), c.Want)
				}
				checked++
			}
		})
	}
	if checked == 0 {
		t.Fatal("no directed quality values checked")
	}
}

// TestOracle_DirectedDeltas checks that Go's incremental directed move-delta
// matches the exact rational delta the oracle emits for each requested
// (node, target) move.
func TestOracle_DirectedDeltas(t *testing.T) {
	checked := 0
	for _, name := range directedOracleFixtureNames {
		t.Run(name, func(t *testing.T) {
			f := loadOracleFixture(t, name)
			for i, c := range f.Cases {
				for _, d := range c.Deltas {
					got := f.goDelta(t, c, d)
					if !withinTolerance(got, d.Want, deltaBudgetULP) {
						t.Errorf("case %d delta node=%d target=%d: got %v, want %v (%s)",
							i, d.Node, d.Target, got, wantFloat(d.Want), d.Want)
					}
					checked++
				}
			}
		})
	}
	if checked == 0 {
		t.Fatal("no directed deltas checked")
	}
}

// The directed predicate deciders below are the Go images of the proved directed
// Bool mirrors (verification/lean/Meso/DirectedPredicates.lean). They live in the
// test layer, like subsetOptimalOracle: nothing in the production directed
// pipeline reads them, they exist to be asserted against the proved golden flags.

// directedCommunityConnected reports whether community c induces a weakly
// connected subgraph: connected in the underlying undirected graph, with an edge
// wherever a positive-weight arc runs in either direction. It is the Go image of
// directedCommunityConnectedFast. Unlike the undirected communityConnected it
// must traverse both the out- and the in-adjacency: an arc u -> v joins the two
// nodes whichever endpoint the traversal reaches first, and following out-arcs
// only would wrongly report some weakly connected communities as disconnected.
func directedCommunityConnected(g *csr, p Partition, c int) bool {
	var members []int
	for i := 0; i < g.numNodes(); i++ {
		if p[i] == c {
			members = append(members, i)
		}
	}
	if len(members) <= 1 {
		return true
	}

	visited := make(map[int]bool, len(members))
	stack := []int{members[0]}
	visited[members[0]] = true
	reached := 1
	visit := func(nb []int, nw []float64) {
		for k, v := range nb {
			if nw[k] > 0 && p[v] == c && !visited[v] {
				visited[v] = true
				reached++
				stack = append(stack, v)
			}
		}
	}
	for len(stack) > 0 {
		u := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		visit(g.neighbors(u), g.neighborWeights(u))
		visit(g.inNeighbors(u), g.inNeighborWeights(u))
	}
	return reached == len(members)
}

// directedConnectedCommunities reports whether every occupied community of p is
// weakly connected. It is the Go image of DirectedConnectedCommunitiesFast, the
// directed weak-connectivity guarantee
// (directedConnectedCommunities_of_refineRun).
func directedConnectedCommunities(g *csr, p Partition) bool {
	seen := make(map[int]bool, len(p))
	for _, c := range p {
		if seen[c] {
			continue
		}
		seen[c] = true
		if !directedCommunityConnected(g, p, c) {
			return false
		}
	}
	return true
}

// directedGammaSeparatedCommunities reports whether every pair of distinct
// occupied communities satisfies the directed separation bound
// e(C,D) + e(D,C) <= gamma*(KoutC*KinD + KoutD*KinC)/m: the bidirectional cut on
// the left, both degree-product orientations on the right. It is the Go image of
// DirectedGammaSeparatedCommunitiesQ, the bound directed level stability
// establishes (directedGammaSeparated_blockWeight_of_levelStable). The bound is
// symmetric under swapping C and D, so each unordered pair is checked once. An
// arcless graph (m == 0) is trivially separated, matching the rational
// convention 1/0 = 0.
func directedGammaSeparatedCommunities(g *csr, gamma float64, p Partition) bool {
	m := g.twoM()
	if m == 0 {
		return true
	}
	kout := map[int]float64{}
	kin := map[int]float64{}
	for i, c := range p {
		kout[c] += g.outDegree(i)
		kin[c] += g.inDegree(i)
	}
	labels := distinctCommunities(p)
	for x, a := range labels {
		for _, b := range labels[x+1:] {
			cross := blockWeight(g, p, a, b) + blockWeight(g, p, b, a)
			if cross > gamma*(kout[a]*kin[b]+kout[b]*kin[a])/m {
				return false
			}
		}
	}
	return true
}

// directedSubsetOptimalOracle decides the directed subset bound
// (DirectedSubsetOptimalQ): for every subset S of every occupied community C,
// with T = C\S, gamma*(KoutS*KinT + KoutT*KinS)/m <= e(S,T) + e(T,S). It
// enumerates all 2^|C| subsets per community, so, like subsetOptimalOracle, it
// only ever runs on the tiny fixtures; corpus cases carry a null flag and are
// skipped. Per the triage this is a characterization, not an output guarantee:
// the committed directed_subset4 all-in-one case genuinely violates the bound.
func directedSubsetOptimalOracle(g *csr, gamma float64, p Partition) bool {
	m := g.twoM()
	if m == 0 {
		return true
	}
	byComm := map[int][]int{}
	for i, c := range p {
		byComm[c] = append(byComm[c], i)
	}
	for _, mem := range byComm {
		k := len(mem)
		for mask := 1; mask < (1 << k); mask++ {
			var koutS, kinS, koutT, kinT float64
			var inS [64]bool
			for bit := range k {
				if mask&(1<<bit) != 0 {
					inS[bit] = true
					koutS += g.outDegree(mem[bit])
					kinS += g.inDegree(mem[bit])
				} else {
					koutT += g.outDegree(mem[bit])
					kinT += g.inDegree(mem[bit])
				}
			}
			var cross float64
			for a := range k {
				for b := range k {
					if inS[a] != inS[b] {
						cross += g.weight(mem[a], mem[b])
					}
				}
			}
			if gamma*(koutS*kinT+koutT*kinS)/m > cross {
				return false
			}
		}
	}
	return true
}

// TestOracle_DirectedConnectivityVector is the Go image of
// directedConnectedCommunitiesFast_iff: the committed connected flag decides
// DirectedConnectedCommunities, so the weak-connectivity decider must agree with
// the proved flag on every directed case. asym3 carries both a
// weakly-but-not-strongly connected community ([0,1,0]: {0,2} joined only by the
// arc 0->2, flag true) and a disconnected one ([0,1,1]: {1,2} with no arc either
// way, flag false), so the check has teeth on both outcomes and on the
// weak-vs-strong distinction.
func TestOracle_DirectedConnectivityVector(t *testing.T) {
	checked, sawTrue, sawFalse := 0, false, false
	for _, name := range directedOracleFixtureNames {
		f := loadOracleFixture(t, name)
		for i, c := range f.Cases {
			got := directedConnectedCommunities(f.Graph.model, c.Partition)
			if got != c.Predicates.Connected {
				t.Errorf("%s case %d: directedConnectedCommunities = %v, oracle connected = %v",
					name, i, got, c.Predicates.Connected)
			}
			if c.Predicates.Connected {
				sawTrue = true
			} else {
				sawFalse = true
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no directed connectivity flags checked")
	}
	if !sawTrue || !sawFalse {
		t.Errorf("directed connectivity check lacks two-sided teeth (sawTrue=%v sawFalse=%v)", sawTrue, sawFalse)
	}
}

// TestOracle_DirectedGammaSeparatedVector is the Go image of
// directedGammaSeparatedCommunitiesQ_iff: the committed gammaSeparated flag
// decides the directed separation bound, so the Go decider must agree with the
// proved flag on every directed case. Both branches occur (cycles6 grouped
// across the bridge and asym3 [0,1,2] are not separated; the natural
// two-community partitions are).
func TestOracle_DirectedGammaSeparatedVector(t *testing.T) {
	checked, sawTrue, sawFalse := 0, false, false
	for _, name := range directedOracleFixtureNames {
		f := loadOracleFixture(t, name)
		for i, c := range f.Cases {
			got := directedGammaSeparatedCommunities(f.Graph.model, c.gammaFloat(), c.Partition)
			if got != c.Predicates.GammaSeparated {
				t.Errorf("%s case %d: directedGammaSeparatedCommunities = %v, oracle gammaSeparated = %v",
					name, i, got, c.Predicates.GammaSeparated)
			}
			if c.Predicates.GammaSeparated {
				sawTrue = true
			} else {
				sawFalse = true
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no directed gamma-separation flags checked")
	}
	if !sawTrue || !sawFalse {
		t.Errorf("directed gamma-separation check lacks two-sided teeth (sawTrue=%v sawFalse=%v)", sawTrue, sawFalse)
	}
}

// TestOracle_DirectedSubsetVector is the Go image of directedSubsetOptimalQ_iff:
// the committed subsetOptimal flag decides the directed subset bound, so the
// enumerating Go decider must agree with the proved flag on every directed case
// that carries one (null above the emitter's node bound is a skip). The false
// branch is the point: directed_subset4's all-in-one is meso's converged output
// on that graph yet violates the bound (the Phase 3 refutation fixture,
// mes-niic), so the flag is a characterization the harness keeps honest.
func TestOracle_DirectedSubsetVector(t *testing.T) {
	checked, sawTrue, sawFalse := 0, false, false
	for _, name := range directedOracleFixtureNames {
		f := loadOracleFixture(t, name)
		for i, c := range f.Cases {
			if c.Predicates.SubsetOptimal == nil {
				continue
			}
			got := directedSubsetOptimalOracle(f.Graph.model, c.gammaFloat(), c.Partition)
			if got != *c.Predicates.SubsetOptimal {
				t.Errorf("%s case %d: directedSubsetOptimalOracle = %v, oracle subsetOptimal = %v",
					name, i, got, *c.Predicates.SubsetOptimal)
			}
			if *c.Predicates.SubsetOptimal {
				sawTrue = true
			} else {
				sawFalse = true
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no directed subsetOptimal flags checked; every fixture skipped")
	}
	if !sawTrue || !sawFalse {
		t.Errorf("directed subset check lacks two-sided teeth (sawTrue=%v sawFalse=%v)", sawTrue, sawFalse)
	}
}

// TestOracle_DirectedGammaDenseAbsent pins the directed predicate object shape:
// a directed golden case has no gammaDense key (gamma-density is a CPM
// guarantee, and directed CPM is unmodelled by design), so the loader must
// surface it as nil, never as false.
func TestOracle_DirectedGammaDenseAbsent(t *testing.T) {
	for _, name := range directedOracleFixtureNames {
		f := loadOracleFixture(t, name)
		for i, c := range f.Cases {
			if c.Predicates.GammaDense != nil {
				t.Errorf("%s case %d: gammaDense = %v, want nil (absent on directed cases)",
					name, i, *c.Predicates.GammaDense)
			}
		}
	}
}
