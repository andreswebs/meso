package meso

import (
	"math"
	"math/rand"
	"sort"
	"testing"
)

// randomDirectedCSR builds a random directed graph for the directed move-delta
// property test: n in [2, 50], each ordered pair (i, j) with i != j wired as an
// arc i -> j with probability 0.3 and an independent weight in [0.5, 5), and
// self-loops on ~20% of nodes. Weights are set per direction, so out- and
// in-degrees differ (the Leicht-Newman null model's whole point), and the run
// naturally yields self-loops, isolated nodes, and disconnected components. The
// in adjacency is the transpose of the out adjacency, both sorted by neighbour.
func randomDirectedCSR(rng *rand.Rand) *csr {
	n := 2 + rng.Intn(49)

	type arc struct {
		to int
		w  float64
	}
	outArcs := make([][]arc, n)
	inArcs := make([][]arc, n)
	selfLoops := make([]float64, n)
	for i := range n {
		for j := range n {
			if i == j {
				continue
			}
			if rng.Float64() < 0.3 {
				w := 0.5 + rng.Float64()*4.5
				outArcs[i] = append(outArcs[i], arc{to: j, w: w})
				inArcs[j] = append(inArcs[j], arc{to: i, w: w})
			}
		}
		if rng.Float64() < 0.2 {
			selfLoops[i] = 0.5 + rng.Float64()*4.5
		}
	}

	build := func(arcs [][]arc) adjacency {
		offsets := make([]int, n+1)
		var neighbors []int
		var weights []float64
		for i := range n {
			sort.Slice(arcs[i], func(a, b int) bool { return arcs[i][a].to < arcs[i][b].to })
			for _, e := range arcs[i] {
				neighbors = append(neighbors, e.to)
				weights = append(weights, e.w)
			}
			offsets[i+1] = len(neighbors)
		}
		return adjacency{offsets: offsets, neighbors: neighbors, weights: weights}
	}

	nodeSizes := make([]float64, n)
	for i := range nodeSizes {
		nodeSizes[i] = 1.0
	}

	return &csr{
		n:         n,
		directed:  true,
		out:       build(outArcs),
		in:        build(inArcs),
		selfLoops: selfLoops,
		nodeSizes: nodeSizes,
	}
}

// TestDirectedMove_SelfIsNoOp: a directed move to the community the node already
// occupies is a no-op, so its delta is exactly 0 (criterion 1).
func TestDirectedMove_SelfIsNoOp(t *testing.T) {
	cases := []struct {
		name string
		g    *csr
		p    Partition
	}{
		{"asymmetric one community", asymmetricDirectedCSR(), Partition{0, 0, 0}},
		{"asymmetric singletons", asymmetricDirectedCSR(), Partition{0, 1, 2}},
		{"asymmetric mixed", asymmetricDirectedCSR(), Partition{0, 0, 1}},
	}
	m := directedModularity{gamma: 1.0}
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

// selfLoopDirectedCSR is a 3-node directed graph with a self-loop on the node
// that gets moved, to exercise the dropped-diagonal handling.
//
// Arcs: 0->1 (2), 2->0 (3), self-loop on 0 (4).
func selfLoopDirectedCSR() *csr {
	return &csr{
		n:        3,
		directed: true,
		out: adjacency{
			offsets:   []int{0, 1, 1, 2},
			neighbors: []int{1, 0},
			weights:   []float64{2, 3},
		},
		in: adjacency{
			offsets:   []int{0, 1, 2, 2},
			neighbors: []int{2, 0},
			weights:   []float64{3, 2},
		},
		selfLoops: []float64{4, 0, 0},
		nodeSizes: []float64{1, 1, 1},
	}
}

// TestDirectedMove_Regressions pins the explicit cases the verification doc
// requires for the directed delta: singleton join/leave, a moved node carrying a
// self-loop, and an empty-target move. Each delta is checked against the directed
// from-scratch oracle, and the singleton-join case against its hand value
// (criterion 3).
func TestDirectedMove_Regressions(t *testing.T) {
	cases := []struct {
		name   string
		g      *csr
		p      Partition
		u      int
		target int
		gamma  float64
		want   float64 // exact hand value; NaN means "oracle only"
	}{
		// Node 1 joins node 0's community on the asymmetric fixture. By hand at
		// gamma=1: Q_before(singletons) = -0.3125, Q_after({0,1},{2}) = 0, so
		// delta = 0.3125 (both in- and out-degree contributions of the move count).
		{"singleton join", asymmetricDirectedCSR(), Partition{0, 1, 2}, 1, 0, 1.0, 0.3125},
		// The reverse: node 1 leaves {0,1} back to its own singleton.
		{"singleton leave", asymmetricDirectedCSR(), Partition{0, 0, 1}, 1, 1, 1.0, math.NaN()},
		// The moved node carries a self-loop (node 0, self-loop weight 4).
		{"self-loop node joins", selfLoopDirectedCSR(), Partition{0, 1, 2}, 0, 1, 1.0, math.NaN()},
		{"self-loop node leaves", selfLoopDirectedCSR(), Partition{0, 0, 1}, 0, 1, 0.7, math.NaN()},
		// Empty-target move: node 2 leaves to a currently-unused community label.
		{"empty target", asymmetricDirectedCSR(), Partition{0, 0, 0}, 2, 1, 1.3, math.NaN()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := directedModularity{gamma: tc.gamma}
			delta := m.moveDelta(tc.g, tc.p, tc.u, tc.target)
			q := DirectedModularity(tc.gamma)
			oracle := q.Quality(tc.g, move(tc.p, tc.u, tc.target)) - q.Quality(tc.g, tc.p)
			if math.Abs(delta-oracle) > moveTol {
				t.Errorf("delta %v != Qdir_after-Qdir_before %v (diff %g)", delta, oracle, math.Abs(delta-oracle))
			}
			if !math.IsNaN(tc.want) && math.Abs(delta-tc.want) > moveTol {
				t.Errorf("delta %v != hand value %v", delta, tc.want)
			}
		})
	}
}

// TestDirectedMove_MatchesOracle is the property test: over thousands of random
// asymmetric graphs, partitions, legal moves, and resolutions, the incremental
// directed delta must equal the directed from-scratch Qdir(after) - Qdir(before).
// The seed is logged on failure so any counterexample is reproducible
// (criterion 2). The undirected oracle is deliberately not used.
func TestDirectedMove_MatchesOracle(t *testing.T) {
	const trials = 5000
	base := rand.New(rand.NewSource(1))
	for range trials {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))

		g := randomDirectedCSR(rng)
		n := g.numNodes()
		p := randomPartition(rng, n)
		gamma := 0.3 + rng.Float64()*1.5
		u := rng.Intn(n)
		target := rng.Intn(n)

		m := directedModularity{gamma: gamma}
		delta := m.moveDelta(g, p, u, target)
		q := DirectedModularity(gamma)
		oracle := q.Quality(g, move(p, u, target)) - q.Quality(g, p)
		if math.Abs(delta-oracle) > moveTol {
			t.Fatalf("seed %d: delta %v != Qdir_after-Qdir_before %v (diff %g); n=%d u=%d target=%d gamma=%v",
				seed, delta, oracle, math.Abs(delta-oracle), n, u, target, gamma)
		}
	}
}

// TestDirectedMove_SymmetricMatchesUndirected checks that on a symmetric directed
// graph the directed delta equals the undirected delta, for the same move
// (criterion 4). On a symmetric graph in- and out-adjacency coincide, so the two
// formulas are algebraically equal; they sum the null term in a different order,
// so the agreement is to float tolerance rather than bit-identical.
func TestDirectedMove_SymmetricMatchesUndirected(t *testing.T) {
	graphs := []struct {
		name string
		u    *csr
		ps   []Partition
	}{
		{"triangle", triangleCSR(), []Partition{{0, 0, 0}, {0, 1, 2}, {0, 0, 1}}},
		{"path", pathCSR(), []Partition{{0, 0, 0}, {0, 1, 2}, {0, 0, 1}}},
		{"two-edges", twoEdgesCSR(), []Partition{{0, 0, 1, 1}, {0, 1, 2, 3}, {0, 0, 0, 0}}},
	}
	gammas := []float64{0.5, 1.0, 2.0}

	for _, gr := range graphs {
		dir := symmetricDirected(gr.u)
		n := gr.u.numNodes()
		for _, gamma := range gammas {
			und := modularity{gamma: gamma}
			d := directedModularity{gamma: gamma}
			for _, p := range gr.ps {
				for u := range n {
					for target := range n {
						want := und.moveDelta(gr.u, p, u, target)
						got := d.moveDelta(dir, p, u, target)
						if math.Abs(got-want) > moveTol {
							t.Errorf("%s gamma=%v p=%v u=%d target=%d: directed delta = %v, want undirected delta = %v (diff %g)",
								gr.name, gamma, p, u, target, got, want, math.Abs(got-want))
						}
					}
				}
			}
		}
	}
}
