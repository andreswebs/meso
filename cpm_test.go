package meso

import (
	"math"
	"math/rand"
	"testing"
)

// cpmTol is the absolute tolerance for CPM value checks. CPM has the units of
// edge weight (no 2m normalisation), so the from-scratch double sum and the
// per-community regrouping agree far tighter than this at test scale.
const cpmTol = 1e-12

// Compile-time assertion that the cpm type satisfies QualityFunction.
var _ QualityFunction = cpm{}

// TestCPM_HandComputed checks CPM against by-hand values on the tiny fixtures at
// several gamma. With unit node sizes the penalty on a community of size S is
// gamma*S^2, so CPM of a partition is sum_c (e_c - gamma*S_c^2) with e_c the
// community's internal (ordered-pair) weight.
func TestCPM_HandComputed(t *testing.T) {
	tests := []struct {
		name  string
		g     *csr
		gamma float64
		p     Partition
		want  float64
	}{
		// Triangle, one community, gamma=1: e_c = 6 (six ordered unit edges),
		// S_c = 3. CPM = 6 - 9 = -3.
		{"triangle one-community gamma=1", triangleCSR(), 1.0, Partition{0, 0, 0}, -3.0},
		// Triangle, singletons, gamma=1: each e_c = 0, S_c = 1. CPM = 3*(0-1) = -3.
		{"triangle singletons gamma=1", triangleCSR(), 1.0, Partition{0, 1, 2}, -3.0},
		// Triangle, one community, gamma=0.5: 6 - 0.5*9 = 1.5.
		{"triangle one-community gamma=0.5", triangleCSR(), 0.5, Partition{0, 0, 0}, 1.5},
		// Path 0-1-2, one community, gamma=1: e_c = 4, S_c = 3. CPM = 4 - 9 = -5.
		{"path one-community gamma=1", pathCSR(), 1.0, Partition{0, 0, 0}, -5.0},
		// Two disjoint edges, natural communities, gamma=1: each e_c = 2, S_c = 2.
		// CPM = 2*(2 - 4) = -4.
		{"two-edges natural gamma=1", twoEdgesCSR(), 1.0, Partition{0, 0, 1, 1}, -4.0},
		// Same, gamma=0.5: each 2 - 0.5*4 = 0. CPM = 0 (the gamma-dense boundary).
		{"two-edges natural gamma=0.5", twoEdgesCSR(), 0.5, Partition{0, 0, 1, 1}, 0.0},
		// Same, gamma=0.25: each 2 - 0.25*4 = 1. CPM = 2.
		{"two-edges natural gamma=0.25", twoEdgesCSR(), 0.25, Partition{0, 0, 1, 1}, 2.0},
		// Two disjoint edges, all singletons, gamma=1: each e_c = 0, S_c = 1.
		// CPM = 4*(0 - 1) = -4.
		{"two-edges singletons gamma=1", twoEdgesCSR(), 1.0, Partition{0, 1, 2, 3}, -4.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CPM(tt.gamma).Quality(tt.g, tt.p)
			if math.Abs(got-tt.want) > cpmTol {
				t.Errorf("Quality() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCPM_SelfLoop checks the diagonal term survives into the internal weight: a
// self-loop is the i == j weight and contributes to e_c but not to the penalty.
func TestCPM_SelfLoop(t *testing.T) {
	// selfLoopCSR: edge 0-1 weight 3, self-loop 5 on node 0; unit node sizes.
	g := selfLoopCSR()

	// One community, gamma=1: e_c = w00 + w01 + w10 + w11 = 5 + 3 + 3 + 0 = 11,
	// S_c = 2. CPM = 11 - 1*4 = 7.
	if got := CPM(1.0).Quality(g, Partition{0, 0}); math.Abs(got-7.0) > cpmTol {
		t.Errorf("self-loop one-community: Quality() = %v, want 7", got)
	}

	// Singletons, gamma=1: node 0 contributes its self-loop (e=5, S=1) -> 4;
	// node 1 contributes 0 - 1 -> -1. CPM = 4 + (-1) = 3.
	if got := CPM(1.0).Quality(g, Partition{0, 1}); math.Abs(got-3.0) > cpmTol {
		t.Errorf("self-loop singletons: Quality() = %v, want 3", got)
	}
}

// TestCPM_NodeSizesAffectPenalty pins acceptance criterion 5: node sizes enter
// only the penalty term, so scaling a node's size changes CPM by exactly the
// predicted amount while the internal weight is untouched.
func TestCPM_NodeSizesAffectPenalty(t *testing.T) {
	// Two disjoint edges 0-1 and 2-3, unit weights, but node 0 has size 2.
	g := newCSR(
		[]int{0, 1, 2, 3, 4},
		[]int{1, 0, 3, 2},
		[]float64{1, 1, 1, 1},
		nil,
		[]float64{2, 1, 1, 1},
	)
	p := Partition{0, 0, 1, 1}

	// Community {0,1}: e_c = 2, S_c = 2 + 1 = 3. gamma=1: 2 - 9 = -7.
	// Community {2,3}: e_c = 2, S_c = 2.            gamma=1: 2 - 4 = -2.
	// CPM = -7 + -2 = -9 (versus -4 with all-unit sizes).
	if got := CPM(1.0).Quality(g, p); math.Abs(got-(-9.0)) > cpmTol {
		t.Errorf("weighted-node CPM = %v, want -9", got)
	}
}

// TestCPM_Deterministic asserts repeated evaluation is bit-identical, the
// property canonical row-major summation guarantees.
func TestCPM_Deterministic(t *testing.T) {
	g := triangleCSR()
	q := CPM(0.7)
	p := Partition{0, 0, 1}

	first := q.Quality(g, p)
	for i := range 100 {
		if got := q.Quality(g, p); got != first {
			t.Fatalf("evaluation %d = %v, want bit-identical %v", i, got, first)
		}
	}
}

// TestCPM_EmptyGraph checks the degenerate zero-node graph evaluates to 0 rather
// than panicking.
func TestCPM_EmptyGraph(t *testing.T) {
	empty := newCSR([]int{0}, nil, nil, nil, nil)
	if got := CPM(1.0).Quality(empty, Partition{}); got != 0 {
		t.Errorf("empty graph: Quality() = %v, want 0", got)
	}
}

// randomCSRSized wraps randomCSR and gives every node a random size in [0.5, 3),
// so the CPM property tests exercise the node-size penalty rather than the
// all-ones default.
func randomCSRSized(rng *rand.Rand) *csr {
	g := randomCSR(rng)
	for i := range g.nodeSizes {
		g.nodeSizes[i] = 0.5 + rng.Float64()*2.5
	}
	return g
}

// TestCPM_CommunitySum is the Go image of Lean cpm_eq_communitySum: over random
// graphs (with non-unit node sizes), partitions, and gamma, the from-scratch CPM
// equals the sum over distinct communities of e_c - gamma*S_c^2.
func TestCPM_CommunitySum(t *testing.T) {
	const trials = 3000
	base := rand.New(rand.NewSource(2))
	for range trials {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))

		g := randomCSRSized(rng)
		n := g.numNodes()
		p := randomPartition(rng, n)
		gamma := 0.3 + rng.Float64()*1.5

		want := CPM(gamma).Quality(g, p)

		var got float64
		for _, c := range distinctCommunities(p) {
			s := communitySize(g, p, c)
			got += communityInternalWeight(g, p, c) - gamma*s*s
		}

		if math.Abs(got-want) > cpmTol*(1+math.Abs(want)) {
			t.Fatalf("seed %d: community sum %v != CPM %v (n=%d gamma=%v)", seed, got, want, n, gamma)
		}
	}
}

// TestCPM_GammaDenseContribution is the Go image of Lean isGammaDense_iff: for
// every community of every random partition, isGammaDense holds exactly when the
// CPM contribution e_c - gamma*S_c^2 is nonnegative.
func TestCPM_GammaDenseContribution(t *testing.T) {
	const trials = 3000
	base := rand.New(rand.NewSource(3))
	for range trials {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))

		g := randomCSRSized(rng)
		n := g.numNodes()
		p := randomPartition(rng, n)
		gamma := 0.3 + rng.Float64()*1.5

		for _, c := range distinctCommunities(p) {
			s := communitySize(g, p, c)
			contribution := communityInternalWeight(g, p, c) - gamma*s*s
			dense := isGammaDense(g, gamma, p, c)
			if dense != (contribution >= 0) {
				t.Fatalf("seed %d: community %d isGammaDense=%v but contribution=%v", seed, c, dense, contribution)
			}
		}
	}
}

// cpmMoveTol is the absolute tolerance for the CPM move-delta identity: the
// incremental delta must reproduce the from-scratch CPM(after) - CPM(before)
// within this bound (docs/research/move-delta-verification.md). CPM carries the
// units of edge weight, so at n <= 50 with bounded weights and node sizes the
// two evaluations agree far tighter than 1e-9.
const cpmMoveTol = 1e-9

// TestCPM_MoveSelfIsNoOp is the CPM image of Lean move_self: reassigning a node
// to the community it already occupies is a no-op, so its CPM delta is exactly 0.
func TestCPM_MoveSelfIsNoOp(t *testing.T) {
	cases := []struct {
		name string
		g    *csr
		p    Partition
	}{
		{"triangle one community", triangleCSR(), Partition{0, 0, 0}},
		{"triangle singletons", triangleCSR(), Partition{0, 1, 2}},
		{"path mixed", pathCSR(), Partition{2, 2, 0}},
		{"self-loop graph", selfLoopCSR(), Partition{1, 1}},
	}
	m := cpm{gamma: 1.0}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for u := range tc.p {
				if got := m.moveDelta(tc.g, tc.p, u, tc.p[u]); got != 0 {
					t.Errorf("moveDelta(node %d -> own community %d) = %v, want exactly 0", u, tc.p[u], got)
				}
			}
		})
	}
}

// TestCPM_MoveRegressions pins the explicit cases from the verification doc for
// the CPM objective: singleton in/out, a moved node carrying a self-loop, and an
// empty-target move. Each delta is checked against the from-scratch oracle, and
// the singleton-join case against its hand-computed value.
func TestCPM_MoveRegressions(t *testing.T) {
	cases := []struct {
		name   string
		g      *csr
		p      Partition
		u      int
		target int
		gamma  float64
		want   float64 // exact hand value; NaN means "oracle only"
	}{
		// Two disjoint edges, all singletons; node 1 joins node 0's community.
		// Merge of two unit-size singletons with edge weight 1: delta =
		// 2*(w_01 - gamma*s_0*s_1) = 2*(1 - 1) = 0 at gamma=1.
		{"singleton join gamma=1", twoEdgesCSR(), Partition{0, 1, 2, 3}, 1, 0, 1.0, 0.0},
		// Same join at gamma=0.5: delta = 2*(1 - 0.5) = 1.
		{"singleton join gamma=0.5", twoEdgesCSR(), Partition{0, 1, 2, 3}, 1, 0, 0.5, 1.0},
		// The reverse: node 1 leaves the {0,1} community to form its own singleton.
		{"singleton leave", twoEdgesCSR(), Partition{0, 0, 2, 3}, 1, 1, 1.0, math.NaN()},
		// Moved node carries a self-loop (node 0 has self-loop weight 5): the
		// diagonal term is constant across the move and must not enter the delta.
		{"self-loop node joins", selfLoopCSR(), Partition{0, 1}, 0, 1, 1.0, math.NaN()},
		{"self-loop node leaves", selfLoopCSR(), Partition{0, 0}, 0, 1, 0.7, math.NaN()},
		// Empty-target move: node 2 leaves to a currently-unused community label.
		{"empty target", triangleCSR(), Partition{0, 0, 0}, 2, 1, 1.3, math.NaN()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := cpm{gamma: tc.gamma}
			delta := m.moveDelta(tc.g, tc.p, tc.u, tc.target)
			oracle := m.Quality(tc.g, move(tc.p, tc.u, tc.target)) - m.Quality(tc.g, tc.p)
			if math.Abs(delta-oracle) > cpmMoveTol {
				t.Errorf("delta %v != CPM_after-CPM_before %v (diff %g)", delta, oracle, math.Abs(delta-oracle))
			}
			if !math.IsNaN(tc.want) && math.Abs(delta-tc.want) > cpmMoveTol {
				t.Errorf("delta %v != hand value %v", delta, tc.want)
			}
		})
	}
}

// TestCPM_MoveMatchesOracle is the CPM property test at the heart of this step:
// over thousands of random graphs (with non-unit node sizes so the penalty term
// is exercised), partitions, legal moves, and resolutions, the incremental delta
// must equal the from-scratch CPM(after) - CPM(before). The seed is logged on
// failure so any counterexample is reproducible.
func TestCPM_MoveMatchesOracle(t *testing.T) {
	const trials = 5000
	base := rand.New(rand.NewSource(4))
	for range trials {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))

		g := randomCSRSized(rng)
		n := g.numNodes()
		p := randomPartition(rng, n)
		gamma := 0.3 + rng.Float64()*1.5
		u := rng.Intn(n)
		target := rng.Intn(n)

		m := cpm{gamma: gamma}
		delta := m.moveDelta(g, p, u, target)
		oracle := m.Quality(g, move(p, u, target)) - m.Quality(g, p)
		if math.Abs(delta-oracle) > cpmMoveTol {
			t.Fatalf("seed %d: delta %v != CPM_after-CPM_before %v (diff %g); n=%d u=%d target=%d gamma=%v",
				seed, delta, oracle, math.Abs(delta-oracle), n, u, target, gamma)
		}
	}
}

// TestCPM_MergeGain is the Go image of Lean cpm_merge_two_singletons: starting
// from the singleton partition, reassigning node A to B's community raises CPM by
// exactly 2*(w_AB - gamma*s_A*s_B). It checks the cpmMergeGain formula against
// both the from-scratch oracle and the general single-node moveDelta on fixtures
// with non-unit node sizes and an inter-node edge.
func TestCPM_MergeGain(t *testing.T) {
	// Edge 0-1 weight 3, node sizes 2 and 1.5; nodes 2,3 padding singletons.
	g := newCSR(
		[]int{0, 1, 2, 2, 2},
		[]int{1, 0},
		[]float64{3, 3},
		nil,
		[]float64{2, 1.5, 1, 1},
	)
	cases := []struct {
		name  string
		gamma float64
		a, b  int
	}{
		{"adjacent nodes gamma=1", 1.0, 0, 1},
		{"adjacent nodes gamma=0.4", 0.4, 0, 1},
		{"non-adjacent nodes", 1.0, 0, 2},
		{"reversed order", 0.6, 1, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gain := cpmMergeGain(g, tc.gamma, tc.a, tc.b)

			want := 2 * (g.weight(tc.a, tc.b) - tc.gamma*g.nodeSize(tc.a)*g.nodeSize(tc.b))
			if math.Abs(gain-want) > cpmMoveTol {
				t.Errorf("cpmMergeGain = %v, want 2*(w_AB - gamma*s_A*s_B) = %v", gain, want)
			}

			m := cpm{gamma: tc.gamma}
			singletons := make(Partition, g.numNodes())
			for i := range singletons {
				singletons[i] = i
			}
			oracle := m.Quality(g, move(singletons, tc.a, tc.b)) - m.Quality(g, singletons)
			if math.Abs(gain-oracle) > cpmMoveTol {
				t.Errorf("cpmMergeGain %v != CPM_after-CPM_before %v (diff %g)", gain, oracle, math.Abs(gain-oracle))
			}

			delta := m.moveDelta(g, singletons, tc.a, tc.b)
			if math.Abs(gain-delta) > cpmMoveTol {
				t.Errorf("cpmMergeGain %v != moveDelta %v", gain, delta)
			}
		})
	}
}

// distinctCommunities returns the sorted set of community labels used by p. Test
// helper for iterating the per-community decomposition.
func distinctCommunities(p Partition) []int {
	seen := make(map[int]bool, len(p))
	var cs []int
	for _, c := range p {
		if !seen[c] {
			seen[c] = true
			cs = append(cs, c)
		}
	}
	return cs
}
