package meso

import (
	"math/rand"
	"testing"
)

// Empirical invariants and formal-guarantee checks (mes-wmzq, step 7): the Go
// mirror of the Lean paper-theorems tier. These tests assert the structural
// invariants and the three Traag-Waltman-van Eck (2019) guarantees on the corpus
// and on fuzzed inputs, and cross-check meso's predicate deciders against the
// committed, proved oracle flags (verification/oracle/golden/*.json). Design of
// record: docs/meso-design.md sections 6.3 and 7; CORRESPONDENCE.md section 2.
//
// The guarantee predicates carry a proved oracle: each committed golden case has
// a predicates object whose booleans are the value of a Lean Bool mirror proved
// equal to the paper predicate (Meso/Predicates.lean). So the vector tests below
// assert a Go decider against a proved boolean, not merely against an independent
// recomputation.

// oracleFixture.gammaFloat is the case's resolution as a float64, the value the
// Go deciders take.
func (c oracleCase) gammaFloat() float64 {
	f, _ := c.Gamma.Float64()
	return f
}

// TestOracle_ConnectivityVector is the Go image of connectedCommunitiesFast_iff
// (with mem_reachableFinset_iff folded in): the committed connected flag decides
// ConnectedCommunities, so meso's connectedCommunities decider must agree with
// the proved flag on every case of every fixture. The tiny fixtures carry a
// deliberate false branch (path3 [0,1,0] is disconnected), so the check has teeth
// on both outcomes.
func TestOracle_ConnectivityVector(t *testing.T) {
	checked, sawFalse := 0, false
	for _, name := range oracleFixtureNames {
		f := loadOracleFixture(t, name)
		for i, c := range f.Cases {
			got := connectedCommunities(f.Graph.model, c.Partition)
			if got != c.Predicates.Connected {
				t.Errorf("%s case %d: connectedCommunities = %v, oracle connected = %v",
					name, i, got, c.Predicates.Connected)
			}
			if !c.Predicates.Connected {
				sawFalse = true
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no connectivity flags checked")
	}
	if !sawFalse {
		t.Error("no disconnected case exercised; the connectivity check has no negative teeth")
	}
}

// TestOracle_GammaDenseVector is the Go image of gammaDenseCommunitiesQ_iff: the
// committed gammaDense flag decides GammaDenseCommunities, so meso's
// gammaDenseCommunities decider must agree with the proved flag everywhere. Both
// branches occur (triangle CPM at gamma=1/2 all-in-one is dense; most corpus
// cases are not).
func TestOracle_GammaDenseVector(t *testing.T) {
	checked, sawTrue, sawFalse := 0, false, false
	for _, name := range oracleFixtureNames {
		f := loadOracleFixture(t, name)
		for i, c := range f.Cases {
			// Every undirected golden case carries the flag; nil would mean a
			// directed fixture leaked into oracleFixtureNames.
			if c.Predicates.GammaDense == nil {
				t.Fatalf("%s case %d: missing gammaDense flag", name, i)
			}
			got := gammaDenseCommunities(f.Graph.model, c.gammaFloat(), c.Partition)
			if got != *c.Predicates.GammaDense {
				t.Errorf("%s case %d: gammaDenseCommunities = %v, oracle gammaDense = %v",
					name, i, got, *c.Predicates.GammaDense)
			}
			if *c.Predicates.GammaDense {
				sawTrue = true
			} else {
				sawFalse = true
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no gamma-density flags checked")
	}
	if !sawTrue || !sawFalse {
		t.Errorf("gamma-density check lacks two-sided teeth (sawTrue=%v sawFalse=%v)", sawTrue, sawFalse)
	}
}

// TestOracle_GammaSeparatedVector is the Go image of gammaSeparatedCommunitiesQ_iff:
// the committed gammaSeparated flag decides gamma-separation of communities, so
// meso's gammaSeparatedCommunities decider must agree with the proved flag on
// every case. The tiny fixtures carry deliberate false branches (triangle
// [0,1,2] and [0,0,1] at gamma=1/2 are not gamma-separated).
func TestOracle_GammaSeparatedVector(t *testing.T) {
	checked, sawTrue, sawFalse := 0, false, false
	for _, name := range oracleFixtureNames {
		f := loadOracleFixture(t, name)
		for i, c := range f.Cases {
			got := gammaSeparatedCommunities(f.Graph.model, c.gammaFloat(), c.Partition)
			if got != c.Predicates.GammaSeparated {
				t.Errorf("%s case %d: gammaSeparatedCommunities = %v, oracle gammaSeparated = %v",
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
		t.Fatal("no gamma-separation flags checked")
	}
	if !sawTrue || !sawFalse {
		t.Errorf("gamma-separation check lacks two-sided teeth (sawTrue=%v sawFalse=%v)", sawTrue, sawFalse)
	}
}

// TestOracle_SubsetOptimalVector and TestCPM_SubsetOptimal are the Go image of
// subsetOptimalQ_iff: the committed subsetOptimal flag decides IsSubsetOptimal,
// so meso's subset-optimality decider must agree with the proved flag on every
// case that carries one. The flag is null above the emitter's node bound (all
// corpus cases, whose subset decision would enumerate exponentially many
// subsets), so those cases are skipped; the tiny fixtures carry deliberate false
// branches (path3 [0,1,0], square all-in-one at gamma=1).
func TestOracle_SubsetOptimalVector(t *testing.T) {
	checked, sawTrue, sawFalse := 0, false, false
	for _, name := range oracleFixtureNames {
		f := loadOracleFixture(t, name)
		for i, c := range f.Cases {
			if c.Predicates.SubsetOptimal == nil {
				continue
			}
			got := subsetOptimalOracle(f.Graph.model, c.gammaFloat(), c.Partition)
			if got != *c.Predicates.SubsetOptimal {
				t.Errorf("%s case %d: subsetOptimalOracle = %v, oracle subsetOptimal = %v",
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
		t.Fatal("no subsetOptimal flags checked; every fixture skipped")
	}
	if !sawTrue || !sawFalse {
		t.Errorf("subset-optimality check lacks two-sided teeth (sawTrue=%v sawFalse=%v)", sawTrue, sawFalse)
	}
}

// TestCPM_SubsetOptimal is acceptance criterion 8's no-sparse-cut check
// (isSubsetOptimal_of_stable): the subset-optimality decider computes the paper's
// no-sparse-cut bound e(S, C\S) >= gamma ||S|| ||C\S|| for every subset S of
// every community C, and it must agree with the proved subsetOptimal oracle flag
// on the tiny fixtures. It shares the vector check above but is named for the
// theorem it realizes.
func TestCPM_SubsetOptimal(t *testing.T) {
	for _, name := range []string{"triangle", "path3", "square"} {
		f := loadOracleFixture(t, name)
		for i, c := range f.Cases {
			if c.Predicates.SubsetOptimal == nil {
				continue
			}
			got := subsetOptimalOracle(f.Graph.model, c.gammaFloat(), c.Partition)
			if got != *c.Predicates.SubsetOptimal {
				t.Errorf("%s case %d: subset-optimal = %v, oracle = %v", name, i, got, *c.Predicates.SubsetOptimal)
			}
		}
	}
}

// subsetOptimalOracle decides IsSubsetOptimal (verification/lean/Meso/SubsetOptimality.lean):
// every subset S of every occupied community C satisfies the no-sparse-cut bound
// gamma * ||S|| * ||C\S|| <= e(S, C\S), where ||.|| sums node sizes and
// e(S, C\S) sums the weight over ordered pairs from S into C\S. It enumerates all
// 2^|C| subsets per community, so it is a test-only oracle for the tiny fixtures
// whose node count sits under the golden emitter's bound; corpus cases carry a
// null flag and are never passed here. The complement is exactly C\S under p, so
// the split-off gain 2*(gamma*||S||*||C\S|| - e(S, C\S)) being nonpositive is the
// bound checked.
func subsetOptimalOracle(g *csr, gamma float64, p Partition) bool {
	byComm := map[int][]int{}
	for i, c := range p {
		byComm[c] = append(byComm[c], i)
	}
	for _, mem := range byComm {
		m := len(mem)
		for mask := 1; mask < (1 << m); mask++ {
			var sizeS, sizeComp, cross float64
			var inS [64]bool
			for bit := range m {
				if mask&(1<<bit) != 0 {
					inS[bit] = true
					sizeS += g.nodeSize(mem[bit])
				} else {
					sizeComp += g.nodeSize(mem[bit])
				}
			}
			for a := range m {
				if !inS[a] {
					continue
				}
				for b := range m {
					if !inS[b] {
						cross += g.weight(mem[a], mem[b])
					}
				}
			}
			if gamma*sizeS*sizeComp > cross {
				return false
			}
		}
	}
	return true
}

// leidenLevel is one level of a full Leiden run recorded for the invariant
// checks: the non-refined phase-1 partition lifted to the base graph (the
// communities read off at this level) and the working-graph node count the level
// ran on.
type leidenLevel struct {
	base    Partition
	working int
}

// leidenTrace re-runs the serial Leiden loop of leiden() and records one
// leidenLevel per level taken, so the invariant tests can walk the aggregation
// levels the production run passes through. It mirrors leiden() exactly - same
// phases, seeds, and termination guards - and its last level's base partition is
// therefore the value leiden() returns (asserted by TestLeiden_TraceMatchesRun).
// It lives in the test layer: mes-wmzq adds no production trace, only the checks
// that hold the run to the model.
func leidenTrace(g *csr, obj objective, seed uint64) []leidenLevel {
	n := g.numNodes()
	baseOf := make([]int, n)
	for i := range baseOf {
		baseOf[i] = i
	}
	h := g
	initP := singleton(h.numNodes())
	var levels []leidenLevel
	for {
		p := make(Partition, len(initP))
		copy(p, initP)
		localMoveQueue(h, obj, p)
		p = canonicalize(p)

		base := make(Partition, n)
		for i := range base {
			base[i] = p[baseOf[i]]
		}
		levels = append(levels, leidenLevel{base: canonicalize(base), working: h.numNodes()})

		if numCommunities(p) == h.numNodes() {
			break
		}
		refined := refine(h, obj, p, seed)
		aggG, superOf := aggregate(h, refined)
		if aggG.numNodes() >= h.numNodes() {
			break
		}
		initP = leidenLevelSeed(superOf, p, aggG.numNodes())
		for i := range baseOf {
			baseOf[i] = superOf[baseOf[i]]
		}
		h = aggG
	}
	return levels
}

// TestLeiden_TraceMatchesRun pins leidenTrace to the production leiden(): the
// last level's base partition must equal leiden's returned partition, so the
// invariant checks that walk the trace are checking the real run's levels, not a
// drifted copy.
func TestLeiden_TraceMatchesRun(t *testing.T) {
	for _, tc := range corpusAndFuzzed(t, 1234) {
		for _, obj := range []objective{modularity{gamma: 1.0}, cpm{gamma: 0.3}} {
			levels := leidenTrace(tc.g, obj, 88)
			last := levels[len(levels)-1].base
			run := leiden(tc.g, obj, 88)
			if len(last) != len(run) {
				t.Fatalf("%s %T: trace length %d != run length %d", tc.name, obj, len(last), len(run))
			}
			for i := range run {
				if last[i] != run[i] {
					t.Fatalf("%s %T: trace last level differs from leiden() at node %d (%d vs %d)",
						tc.name, obj, i, last[i], run[i])
				}
			}
		}
	}
}

// TestLeiden_OutputWellFormed is acceptance criterion 1: every Leiden and Louvain
// output is a well-formed partition - length n, every label in [0, n), so each
// node lies in exactly one community. Checked over the corpus and fuzzed inputs
// (with non-unit node sizes) under both objectives.
func TestLeiden_OutputWellFormed(t *testing.T) {
	for _, tc := range corpusAndFuzzed(t, 202) {
		n := tc.g.numNodes()
		for _, obj := range []objective{modularity{gamma: 1.0}, cpm{gamma: 0.3}} {
			if err := leiden(tc.g, obj, 7).wellFormed(n); err != nil {
				t.Errorf("%s %T: leiden output not well-formed: %v", tc.name, obj, err)
			}
			if err := louvain(tc.g, obj).wellFormed(n); err != nil {
				t.Errorf("%s %T: louvain output not well-formed: %v", tc.name, obj, err)
			}
		}
	}
}

// TestLeiden_OutputCommunitiesConnected is acceptance criterion 2 for the whole
// returned partition (the formal-guarantee concern deferred from mes-jbc7): every
// community of the final Leiden output induces a connected subgraph. Where
// TestLeiden_CommunitiesConnected checks the refined partition at each level, this
// checks the base-graph partition the run actually returns. Louvain output is
// checked too. Held over the corpus and fuzzed inputs under both objectives.
func TestLeiden_OutputCommunitiesConnected(t *testing.T) {
	for _, tc := range corpusAndFuzzed(t, 203) {
		for _, obj := range []objective{modularity{gamma: 1.0}, cpm{gamma: 0.3}} {
			if !connectedCommunities(tc.g, leiden(tc.g, obj, 13)) {
				t.Errorf("%s %T: leiden output has a disconnected community", tc.name, obj)
			}
			if !connectedCommunities(tc.g, louvain(tc.g, obj)) {
				t.Errorf("%s %T: louvain output has a disconnected community", tc.name, obj)
			}
		}
	}
}

// TestLeidenRun_QualityMonotoneAcrossLevels is acceptance criterion 3 and the Go
// image of quality_monotone_of_stepwise / modularity_le_of_algorithmRun (and its
// CPM twin cpm_le_of_algorithmRun): the quality of the base-graph partition is
// non-decreasing across the aggregation levels of a full Leiden run. Each level's
// local move raises quality on the working graph and aggregation preserves it, so
// the lifted base partition never loses quality from one level to the next.
// Checked over the corpus and fuzzed inputs under both objectives.
func TestLeidenRun_QualityMonotoneAcrossLevels(t *testing.T) {
	for _, tc := range corpusAndFuzzed(t, 204) {
		for _, obj := range []objective{modularity{gamma: 1.0}, cpm{gamma: 0.3}} {
			levels := leidenTrace(tc.g, obj, 41)
			prev := obj.Quality(tc.g, levels[0].base)
			for l := 1; l < len(levels); l++ {
				q := obj.Quality(tc.g, levels[l].base)
				if q < prev-louvainTol {
					t.Fatalf("%s %T: quality dropped at level %d: %v -> %v", tc.name, obj, l, prev, q)
				}
				prev = q
			}
		}
	}
}

// TestLeidenRun_LevelsTerminate is acceptance criterion 4: the multilevel recursion
// halts within its iteration bound. Each continued level strictly shrinks the
// working graph (RunningLevelStep.size_lt / no_infinite_descending_levels), so a
// run on n nodes takes at most n levels, and every level runs on strictly fewer
// nodes than the last. Checked over the corpus and fuzzed inputs under both
// objectives; that leidenTrace returns at all is itself the no-infinite-run
// witness.
func TestLeidenRun_LevelsTerminate(t *testing.T) {
	for _, tc := range corpusAndFuzzed(t, 205) {
		n := tc.g.numNodes()
		for _, obj := range []objective{modularity{gamma: 1.0}, cpm{gamma: 0.3}} {
			levels := leidenTrace(tc.g, obj, 9)
			if len(levels) > n {
				t.Fatalf("%s %T: %d levels exceeds node bound %d", tc.name, obj, len(levels), n)
			}
			for l := 1; l < len(levels); l++ {
				if levels[l].working >= levels[l-1].working {
					t.Fatalf("%s %T: level %d working size %d did not shrink from %d",
						tc.name, obj, l, levels[l].working, levels[l-1].working)
				}
			}
		}
	}
}

// isLocalMoveStable reports whether p is local-move-stable under obj: no
// single-node reassignment - to any occupied community or to a fresh isolation
// label - strictly improves the objective beyond louvainTol. It is the Go image
// of IsLocalMoveStable (verification/lean/Meso/Convergence.lean), the first half
// of IsConverged, evaluated exhaustively rather than through the move loop's
// stopping flag.
func isLocalMoveStable(g *csr, obj objective, p Partition) bool {
	fresh := g.numNodes()
	for _, c := range p {
		if c >= fresh {
			fresh = c + 1
		}
	}
	for u := 0; u < g.numNodes(); u++ {
		if _, delta := bestMove(g, obj, p, u, fresh); delta > louvainTol {
			return false
		}
	}
	return true
}

// aggregateLevelStable reports whether p is level-stable under obj: the singleton
// partition of the aggregate graph is local-move-stable, i.e. no merge of two
// whole communities strictly improves the objective. It is the Go image of the
// second half of IsConverged (verification/lean/Meso/Convergence.lean,
// IsConverged.levelStable), the hypothesis gammaSeparated_of_converged rests on.
// A multilevel run leaves its output level-stable by construction: it stops only
// when the top aggregate's local move merges nothing.
func aggregateLevelStable(g *csr, obj objective, p Partition) bool {
	agg, _ := aggregate(g, canonicalize(p))
	return isLocalMoveStable(agg, obj, singleton(agg.numNodes()))
}

// noStrictlyBetterCommunity reports whether, under obj, no node has a strictly
// better community than its own in p: reassigning any node to any occupied
// community or to a fresh isolation label does not raise the objective beyond
// louvainTol. It is the exhaustive, all-targets reading of
// cpm_noStrictlyBetterCommunity (verification/lean/Meso/Separation.lean), checked
// against every distinct label rather than only neighbour communities.
func noStrictlyBetterCommunity(g *csr, obj objective, p Partition) bool {
	seen := make(map[int]bool, len(p))
	labels := make([]int, 0, len(p))
	fresh := g.numNodes()
	for _, c := range p {
		if !seen[c] {
			seen[c] = true
			labels = append(labels, c)
		}
		if c >= fresh {
			fresh = c + 1
		}
	}
	labels = append(labels, fresh)
	for u := 0; u < g.numNodes(); u++ {
		for _, c := range labels {
			if obj.moveDelta(g, p, u, c) > louvainTol {
				return false
			}
		}
	}
	return true
}

// TestConverge_NoBetterCommunity is acceptance criterion 5 and the Go image of
// cpm_noStrictlyBetterCommunity (verification/lean/Meso/Separation.lean): at a
// converged partition no node has a strictly better community. A CPM local-move
// run leaves a partition that is local-move-stable (asserted), and at that
// partition reassigning any node to any community - not merely a neighbour's -
// never raises CPM. Checked over the corpus and fuzzed inputs across a spread of
// gammas; the local-move loop, not louvain's aggregation, is the fixed point
// under test, so the base-level stable partition is used directly.
func TestConverge_NoBetterCommunity(t *testing.T) {
	for _, tc := range corpusAndFuzzed(t, 206) {
		for _, gamma := range []float64{0.1, 0.5, 1.0} {
			obj := objective(cpm{gamma: gamma})
			p := singleton(tc.g.numNodes())
			localMoveQueue(tc.g, obj, p)
			if !isLocalMoveStable(tc.g, obj, p) {
				t.Fatalf("%s gamma=%v: local-move run did not reach a stable partition", tc.name, gamma)
			}
			if !noStrictlyBetterCommunity(tc.g, obj, p) {
				t.Errorf("%s gamma=%v: a node has a strictly better community at convergence", tc.name, gamma)
			}
		}
	}
}

// TestCPM_GammaSeparated is acceptance criterion 6 and the Go image of
// gammaSeparated_of_converged (verification/lean/Meso/Separation.lean): at a
// CPM-converged partition, distinct communities are gamma-separated,
// e(C,D) <= gamma S_C S_D. A full multilevel run (Louvain or Leiden) returns a
// level-stable partition - no community merge improves CPM - which is exactly the
// theorem's hypothesis (asserted here), and from it gamma-separation follows.
// Checked over the corpus and fuzzed inputs across a spread of gammas, for both
// the Louvain and Leiden outputs.
func TestCPM_GammaSeparated(t *testing.T) {
	for _, tc := range corpusAndFuzzed(t, 207) {
		for _, gamma := range []float64{0.1, 0.5, 1.0} {
			obj := objective(cpm{gamma: gamma})
			for _, run := range []struct {
				name string
				p    Partition
			}{
				{"louvain", louvain(tc.g, obj)},
				{"leiden", leiden(tc.g, obj, 55)},
			} {
				if !aggregateLevelStable(tc.g, obj, run.p) {
					t.Fatalf("%s %s gamma=%v: output is not level-stable (theorem hypothesis fails)", tc.name, run.name, gamma)
				}
				if !gammaSeparatedCommunities(tc.g, gamma, run.p) {
					t.Errorf("%s %s gamma=%v: converged communities are not gamma-separated", tc.name, run.name, gamma)
				}
			}
		}
	}
}

// completeCSR builds the complete graph K_n with unit edge weights, no
// self-loops, and unit node sizes: a dense fixture where every community is
// connected and, at a small enough gamma, gamma-dense with gamma-dense cuts, so a
// gated merge run can proceed without ever violating the invariant.
func completeCSR(n int) *csr {
	offsets := make([]int, n+1)
	var neighbors []int
	var weights []float64
	for i := range n {
		for j := range n {
			if j == i {
				continue
			}
			neighbors = append(neighbors, j)
			weights = append(weights, 1)
		}
		offsets[i+1] = len(neighbors)
	}
	return newCSR(offsets, neighbors, weights, nil, nil)
}

// gammaMergeStep applies one gated refinement merge to p: if communities a and b
// share a positive-weight edge (the connectivity half of the gate) and their cut
// is gamma-dense (gammaDenseCut, the density half), it merges b into a and
// reports ok; otherwise it returns p unchanged and false. It is the Go image of
// GammaMergeStep (verification/lean/Meso/GammaConnectivity.lean), the one-step
// relation the gamma-connectivity run theorem is stated over.
func gammaMergeStep(g *csr, gamma float64, p Partition, a, b int) (Partition, bool) {
	if a == b {
		return p, false
	}
	shared := false
	for i := 0; i < g.numNodes() && !shared; i++ {
		if p[i] != a {
			continue
		}
		nb := g.neighbors(i)
		nw := g.neighborWeights(i)
		for k, j := range nb {
			if nw[k] > 0 && p[j] == b {
				shared = true
				break
			}
		}
	}
	if !shared || !gammaDenseCut(g, gamma, p, a, b) {
		return p, false
	}
	return mergeCommunities(p, a, b), true
}

// TestLeiden_CommunitiesGammaConnected is acceptance criterion 7 and the Go image
// of gammaWellConnectedCommunities_of_gammaMergeRun
// (verification/lean/Meso/GammaConnectivity.lean): a gated refinement run from a
// gamma-well-connected base keeps every community connected and internally
// gamma-dense. Starting from three gamma-dense pairs of K6 (a gamma-well-connected
// base, asserted), gated merges are applied greedily to a fixed point; the
// invariant is re-checked after every step, so a run of two merges collapsing the
// pairs into one community exercises the multi-step ReflTransGen closure, not just
// a single GammaMergeStep. The gate is shown to have teeth: at the base gamma
// every candidate cut is dense, and raising gamma past the cut density blocks the
// first merge outright.
func TestLeiden_CommunitiesGammaConnected(t *testing.T) {
	g := completeCSR(6)
	const gamma = 0.1
	base := Partition{0, 0, 1, 1, 2, 2}

	if !gammaWellConnectedCommunities(g, gamma, base) {
		t.Fatal("precondition: the three-pair base should be gamma-well-connected")
	}

	p := base
	steps := 0
	for {
		merged := false
		labels := distinctCommunities(p)
		for _, a := range labels {
			for _, b := range labels {
				if a >= b {
					continue
				}
				if q, ok := gammaMergeStep(g, gamma, p, a, b); ok {
					p = canonicalize(q)
					steps++
					if !gammaWellConnectedCommunities(g, gamma, p) {
						t.Fatalf("gated merge of %d,%d broke gamma-well-connectedness: %v", a, b, p)
					}
					merged = true
					break
				}
			}
			if merged {
				break
			}
		}
		if !merged {
			break
		}
	}
	if steps < 2 {
		t.Fatalf("expected a multi-step merge run, only %d gated merges applied", steps)
	}
	if numCommunities(p) != 1 {
		t.Fatalf("gated merge run left %d communities, want a single well-connected community", numCommunities(p))
	}

	// Teeth: at a gamma above the cut density the very first merge is refused, so
	// the run is not vacuously always-mergeable.
	if _, ok := gammaMergeStep(g, 4.0, base, 0, 1); ok {
		t.Error("cut passed the gamma-dense gate at gamma=4.0; the gate has no density teeth")
	}
}

// moveSubset returns a copy of p with every node in S reassigned to community c,
// leaving all other nodes fixed. It is the Go image of the Lean moveSubset
// (verification/lean/Meso/SubsetOptimality.lean), the subset generalisation of
// move; moveSubset(p, {v}, c) equals move(p, v, c).
func moveSubset(p Partition, s []int, c int) Partition {
	q := make(Partition, len(p))
	copy(q, p)
	for _, v := range s {
		q[v] = c
	}
	return q
}

// subsetCrossWeight returns e(S, T): the total weight over ordered pairs with the
// first node in S and the second in T, the cross term of the subset-split gain.
func subsetCrossWeight(g *csr, s, tt []int) float64 {
	sum := 0.0
	for _, i := range s {
		for _, j := range tt {
			sum += g.weight(i, j)
		}
	}
	return sum
}

// subsetSizeSum returns ||S||: the summed node size of the members of S.
func subsetSizeSum(g *csr, s []int) float64 {
	sum := 0.0
	for _, i := range s {
		sum += g.nodeSize(i)
	}
	return sum
}

// TestCPM_SubsetSplitGain is acceptance criterion 8 and the Go image of
// cpm_moveSubset_split (verification/lean/Meso/SubsetOptimality.lean): splitting a
// subset S of a community C off to a fresh label changes CPM by exactly
// -2*(e(S, C\S) - gamma*||S||*||C\S||). The from-scratch full-evaluation
// difference Quality(moveSubset(p, S, f)) - Quality(p) must equal the closed form
// over random graphs (with non-unit node sizes), partitions, gammas, and random
// nonempty proper subsets of a chosen community - the subset-level cousin of the
// single-node move-delta property test.
func TestCPM_SubsetSplitGain(t *testing.T) {
	base := rand.New(rand.NewSource(208))
	checked := 0
	for range 4000 {
		rng := rand.New(rand.NewSource(base.Int63()))
		g := randomCSRSized(rng)
		n := g.numNodes()
		gamma := 0.3 + rng.Float64()*1.5
		p := canonicalize(randomPartition(rng, n))

		labels := distinctCommunities(p)
		c := labels[rng.Intn(len(labels))]
		var mem []int
		for i, l := range p {
			if l == c {
				mem = append(mem, i)
			}
		}
		if len(mem) < 2 {
			continue
		}

		var s, comp []int
		for _, v := range mem {
			if rng.Intn(2) == 0 {
				s = append(s, v)
			} else {
				comp = append(comp, v)
			}
		}
		if len(s) == 0 || len(comp) == 0 {
			continue
		}

		fresh := len(labels) // p is canonical, so len(labels) is an unused label
		obj := cpm{gamma: gamma}
		got := obj.Quality(g, moveSubset(p, s, fresh)) - obj.Quality(g, p)
		want := -2 * (subsetCrossWeight(g, s, comp) - gamma*subsetSizeSum(g, s)*subsetSizeSum(g, comp))

		if diff := got - want; diff < -1e-7*(1+absf(want)) || diff > 1e-7*(1+absf(want)) {
			t.Fatalf("subset split gain: got %v, want %v (n=%d gamma=%v |S|=%d |C\\S|=%d)",
				got, want, n, gamma, len(s), len(comp))
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no subset split gains checked")
	}
}

// enumeratePartitions returns every set partition of n nodes in canonical
// (restricted-growth) labelling, for brute-force checks on tiny graphs.
func enumeratePartitions(n int) []Partition {
	var out []Partition
	cur := make(Partition, n)
	var rec func(i, next int)
	rec = func(i, next int) {
		if i == n {
			p := make(Partition, n)
			copy(p, cur)
			out = append(out, p)
			return
		}
		for c := 0; c <= next; c++ {
			cur[i] = c
			nn := next
			if c == next {
				nn = next + 1
			}
			rec(i+1, nn)
		}
	}
	rec(0, 0)
	return out
}

// subsetStableOracle reports whether p is subset-stable under CPM at gamma: no
// reassignment of any node subset to any label strictly raises CPM. It is the Go
// image of IsSubsetStable (verification/lean/Meso/SubsetOptimality.lean),
// enumerating all 2^n subsets against all n+1 target labels, so it is a
// brute-force oracle for tiny graphs only.
func subsetStableOracle(g *csr, gamma float64, p Partition) bool {
	n := g.numNodes()
	obj := cpm{gamma: gamma}
	baseQ := obj.Quality(g, p)
	for mask := 1; mask < (1 << n); mask++ {
		var s []int
		for bit := range n {
			if mask&(1<<bit) != 0 {
				s = append(s, bit)
			}
		}
		for c := 0; c <= n; c++ {
			if obj.Quality(g, moveSubset(p, s, c)) > baseQ+louvainTol {
				return false
			}
		}
	}
	return true
}

// TestConverge_SubsetStableIsMoveStable is acceptance criterion 8 and the Go
// image of IsSubsetStable.isLocalMoveStable
// (verification/lean/Meso/SubsetOptimality.lean): subset stability is strictly
// stronger than single-node local-move stability, so every subset-stable
// partition is move-stable. On tiny graphs every set partition is enumerated;
// each one the brute-force subset oracle calls stable must also be local-move
// stable. The check has teeth on both sides: at least one subset-stable partition
// exists (else the implication is vacuous) and at least one partition is not
// subset-stable (else the hypothesis is trivial).
func TestConverge_SubsetStableIsMoveStable(t *testing.T) {
	fixtures := []struct {
		name string
		g    *csr
	}{
		{"triangle", triangleCSR()},
		{"path", pathCSR()},
		{"gammaUnion", gammaUnionCSR()},
	}
	stableSeen, unstableSeen := false, false
	for _, fx := range fixtures {
		for _, gamma := range []float64{0.3, 0.5, 1.0} {
			obj := objective(cpm{gamma: gamma})
			for _, p := range enumeratePartitions(fx.g.numNodes()) {
				if subsetStableOracle(fx.g, gamma, p) {
					stableSeen = true
					if !isLocalMoveStable(fx.g, obj, p) {
						t.Errorf("%s gamma=%v: subset-stable partition %v is not move-stable", fx.name, gamma, p)
					}
				} else {
					unstableSeen = true
				}
			}
		}
	}
	if !stableSeen {
		t.Error("no subset-stable partition found; the implication is vacuous")
	}
	if !unstableSeen {
		t.Error("no subset-unstable partition found; subset stability is trivially true here")
	}
}

// leidenGuaranteesDecider reports whether p satisfies all three paper guarantees
// of LeidenGuarantees (verification/lean/Meso/Guarantees.lean) at once:
// gamma-separation of distinct communities, gamma-well-connectedness (connected
// and internally gamma-dense) of every community, and subset-optimality (no
// sparse cut at any scale). It conjoins the individual deciders; the exponential
// subset-optimality decider restricts it to tiny graphs.
func leidenGuaranteesDecider(g *csr, gamma float64, p Partition) bool {
	return gammaSeparatedCommunities(g, gamma, p) &&
		gammaWellConnectedCommunities(g, gamma, p) &&
		subsetOptimalOracle(g, gamma, p)
}

// TestLeiden_Guarantees is acceptance criterion 9 and the Go image of
// leidenGuarantees_of_stable (verification/lean/Meso/Guarantees.lean): a converged
// Leiden output satisfies all three paper guarantees together. On the tiny golden
// fixtures (whose partitions carry the proved predicate flags) the conjoined Go
// decider must equal the conjunction of the four proved flags - gamma-separated,
// connected, gamma-dense, and subset-optimal - so the bundled guarantee is checked
// against the oracle, not merely recomputed. The check has two-sided teeth: at
// least one committed case satisfies all three guarantees at once (triangle CPM
// gamma=1/2 all-in-one, square CPM gamma=1/3 all-in-one) and at least one fails.
func TestLeiden_Guarantees(t *testing.T) {
	checked, sawAll, sawSome := 0, false, false
	for _, name := range []string{"triangle", "path3", "square"} {
		f := loadOracleFixture(t, name)
		for i, c := range f.Cases {
			if c.Predicates.SubsetOptimal == nil || c.Predicates.GammaDense == nil {
				continue
			}
			want := c.Predicates.GammaSeparated &&
				c.Predicates.Connected &&
				*c.Predicates.GammaDense &&
				*c.Predicates.SubsetOptimal
			got := leidenGuaranteesDecider(f.Graph.model, c.gammaFloat(), c.Partition)
			if got != want {
				t.Errorf("%s case %d: leidenGuarantees = %v, oracle conjunction = %v", name, i, got, want)
			}
			if want {
				sawAll = true
			} else {
				sawSome = true
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no guarantee bundles checked")
	}
	if !sawAll {
		t.Error("no committed case satisfies all three guarantees at once; the bundle check lacks positive teeth")
	}
	if !sawSome {
		t.Error("every committed case satisfies all three guarantees; the bundle check lacks negative teeth")
	}
}

// absf is the float64 absolute value, kept local to the guarantee tests.
func absf(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
