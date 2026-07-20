package meso

import (
	"math"
	"math/rand"
	"sort"
	"testing"
)

// moveTol is the absolute tolerance for the move-delta identity: the incremental
// delta must reproduce the from-scratch Q(after) - Q(before) within this bound
// (docs/research/move-delta-verification.md). At n <= 50 with bounded weights the
// two evaluations agree far tighter than this.
const moveTol = 1e-9

// randomCSR builds a random undirected graph for the move-delta property tests:
// n in [2, 50], off-diagonal edges added with probability 0.3 and weights in
// [0.5, 5), and self-loops on ~20% of nodes. This naturally yields weighted
// edges, self-loops, isolated nodes, and disconnected components across trials,
// as the verification doc requires.
func randomCSR(rng *rand.Rand) *csr {
	n := 2 + rng.Intn(49)

	adj := make(map[int][]int, n)
	w := make(map[[2]int]float64)
	selfLoops := make([]float64, n)
	for i := range n {
		for j := i + 1; j < n; j++ {
			if rng.Float64() < 0.3 {
				weight := 0.5 + rng.Float64()*4.5
				w[[2]int{i, j}] = weight
				w[[2]int{j, i}] = weight
				adj[i] = append(adj[i], j)
				adj[j] = append(adj[j], i)
			}
		}
		if rng.Float64() < 0.2 {
			selfLoops[i] = 0.5 + rng.Float64()*4.5
		}
	}

	offsets := make([]int, n+1)
	var neighbors []int
	var weights []float64
	for i := range n {
		sort.Ints(adj[i])
		for _, j := range adj[i] {
			neighbors = append(neighbors, j)
			weights = append(weights, w[[2]int{i, j}])
		}
		offsets[i+1] = len(neighbors)
	}
	return newCSR(offsets, neighbors, weights, selfLoops, nil)
}

// randomPartition assigns each of n nodes a random label in [0, n), so the
// partition is always well-formed regardless of how many communities it uses.
func randomPartition(rng *rand.Rand, n int) Partition {
	p := make(Partition, n)
	for i := range p {
		p[i] = rng.Intn(n)
	}
	return p
}

// TestLocalMove_SelfIsNoOp is the Go image of Lean move_self: a move to the
// community the node already occupies is a no-op, so its delta is exactly 0.
func TestLocalMove_SelfIsNoOp(t *testing.T) {
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
	m := modularity{gamma: 1.0}
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

// TestLocalMove_Regressions pins the explicit cases from the verification doc:
// singleton in/out, a moved node carrying a self-loop, and an empty-target move.
// Each delta is checked against the from-scratch oracle, and the singleton-join
// case against its hand-computed value.
func TestLocalMove_Regressions(t *testing.T) {
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
		// By hand at gamma=1: Q_before=-0.25, Q_after=0.125, delta=0.375.
		{"singleton join", twoEdgesCSR(), Partition{0, 1, 2, 3}, 1, 0, 1.0, 0.375},
		// The reverse: node 1 leaves the {0,1} community to form its own singleton.
		{"singleton leave", twoEdgesCSR(), Partition{0, 0, 2, 3}, 1, 1, 1.0, math.NaN()},
		// Moved node carries a self-loop (node 0 has self-loop weight 5).
		{"self-loop node joins", selfLoopCSR(), Partition{0, 1}, 0, 1, 1.0, math.NaN()},
		{"self-loop node leaves", selfLoopCSR(), Partition{0, 0}, 0, 1, 0.7, math.NaN()},
		// Empty-target move: node 2 leaves to a currently-unused community label.
		{"empty target", triangleCSR(), Partition{0, 0, 0}, 2, 1, 1.3, math.NaN()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := modularity{gamma: tc.gamma}
			delta := m.moveDelta(tc.g, tc.p, tc.u, tc.target)
			oracle := m.Quality(tc.g, move(tc.p, tc.u, tc.target)) - m.Quality(tc.g, tc.p)
			if math.Abs(delta-oracle) > moveTol {
				t.Errorf("delta %v != Q_after-Q_before %v (diff %g)", delta, oracle, math.Abs(delta-oracle))
			}
			if !math.IsNaN(tc.want) && math.Abs(delta-tc.want) > moveTol {
				t.Errorf("delta %v != hand value %v", delta, tc.want)
			}
		})
	}
}

// TestLocalMove_MatchesOracle is the property test at the heart of this step:
// over thousands of random graphs, partitions, legal moves, and resolutions, the
// incremental delta must equal the from-scratch Q(after) - Q(before). The seed is
// logged on failure so any counterexample is reproducible.
func TestLocalMove_MatchesOracle(t *testing.T) {
	const trials = 5000
	base := rand.New(rand.NewSource(1))
	for range trials {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))

		g := randomCSR(rng)
		n := g.numNodes()
		p := randomPartition(rng, n)
		gamma := 0.3 + rng.Float64()*1.5
		u := rng.Intn(n)
		target := rng.Intn(n)

		m := modularity{gamma: gamma}
		delta := m.moveDelta(g, p, u, target)
		oracle := m.Quality(g, move(p, u, target)) - m.Quality(g, p)
		if math.Abs(delta-oracle) > moveTol {
			t.Fatalf("seed %d: delta %v != Q_after-Q_before %v (diff %g); n=%d u=%d target=%d gamma=%v",
				seed, delta, oracle, math.Abs(delta-oracle), n, u, target, gamma)
		}
	}
}

// TestLocalMove_BestMoveNonDecreasing is the Go image of Lean
// modularity_bestMove_ge: over a candidate set that always includes staying put,
// the best single-node move never lowers modularity. The chosen move's actual
// effect on Q (from the oracle) is checked to be non-negative and to agree with
// its incremental delta.
func TestLocalMove_BestMoveNonDecreasing(t *testing.T) {
	const trials = 2000
	base := rand.New(rand.NewSource(42))
	for range trials {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))

		g := randomCSR(rng)
		n := g.numNodes()
		p := randomPartition(rng, n)
		gamma := 0.3 + rng.Float64()*1.5
		u := rng.Intn(n)
		m := modularity{gamma: gamma}

		// Candidate targets: the communities of u's neighbours plus staying put.
		candSet := map[int]bool{p[u]: true}
		for _, v := range g.neighbors(u) {
			candSet[p[v]] = true
		}
		cands := make([]int, 0, len(candSet))
		for c := range candSet {
			cands = append(cands, c)
		}
		sort.Ints(cands)

		bestTarget, bestDelta := p[u], 0.0
		for _, c := range cands {
			if d := m.moveDelta(g, p, u, c); d > bestDelta {
				bestDelta, bestTarget = d, c
			}
		}

		// Staying is always a candidate with delta 0, so the best cannot be
		// negative (move_best_ge).
		if bestDelta < -moveTol {
			t.Fatalf("seed %d: best delta %v < 0", seed, bestDelta)
		}
		oracle := m.Quality(g, move(p, u, bestTarget)) - m.Quality(g, p)
		if oracle < -moveTol {
			t.Fatalf("seed %d: best move lowered Q by %v (target %d)", seed, oracle, bestTarget)
		}
		if math.Abs(bestDelta-oracle) > moveTol {
			t.Fatalf("seed %d: best delta %v != oracle %v", seed, bestDelta, oracle)
		}
	}
}
