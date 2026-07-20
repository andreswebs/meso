package meso

import (
	"math"
	"testing"
)

// asymmetricDirectedCSR is a 3-node directed graph with distinct in- and
// out-degrees, exercising the Leicht-Newman null model's separate degrees.
//
// Arcs: 0->1 (2), 0->2 (1), 1->0 (1).
//
//	out-degrees: [3, 1, 0]   in-degrees: [1, 2, 1]   T = 4
func asymmetricDirectedCSR() *csr {
	return &csr{
		n:        3,
		directed: true,
		out: adjacency{
			offsets:   []int{0, 2, 3, 3},
			neighbors: []int{1, 2, 0},
			weights:   []float64{2, 1, 1},
		},
		in: adjacency{
			offsets:   []int{0, 1, 2, 3},
			neighbors: []int{1, 0, 0},
			weights:   []float64{1, 2, 1},
		},
		selfLoops: []float64{0, 0, 0},
		nodeSizes: []float64{1, 1, 1},
	}
}

// TestDirectedModularity_HandComputed checks directed Q against by-hand values
// on the asymmetric fixture, where separate in/out degrees matter.
func TestDirectedModularity_HandComputed(t *testing.T) {
	g := asymmetricDirectedCSR()
	tests := []struct {
		name  string
		gamma float64
		p     Partition
		want  float64
	}{
		// All-in-one: sum_ij w = 4, sum_ij kout_i kin_j / T = 16/4 = 4;
		// Q = (4 - 4) / 4 = 0. Modularity of a single community is always 0.
		{"one-community gamma=1", 1.0, Partition{0, 0, 0}, 0.0},
		// Singletons, gamma = 1: only diagonal survives, all self-loops 0, so
		// numerator sum = -sum_i kout_i kin_i / T = -(3*1 + 1*2 + 0*1)/4 = -1.25;
		// Q = -1.25 / 4 = -0.3125.
		{"singletons gamma=1", 1.0, Partition{0, 1, 2}, -0.3125},
		// Singletons, gamma = 2: numerator = -2*5/4 = -2.5; Q = -2.5/4 = -0.625.
		{"singletons gamma=2", 2.0, Partition{0, 1, 2}, -0.625},
		// Communities {0,1} and {2}: within-pairs (0,0),(0,1),(1,0),(1,1),(2,2).
		// weights: 0+2+1+0+0 = 3; null: (3*1+3*2+1*1+1*2+0*1)/4 = 12/4 = 3;
		// Q = (3 - 3) / 4 = 0.
		{"two-communities gamma=1", 1.0, Partition{0, 0, 1}, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DirectedModularity(tt.gamma).Quality(g, tt.p)
			if math.Abs(got-tt.want) > qualityTol {
				t.Errorf("Quality() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Compile-time assertion that directedModularity satisfies QualityFunction.
var _ QualityFunction = directedModularity{}

// TestDirectedModularity_Deterministic checks that repeated evaluation of the
// same partition is bit-identical, as the canonical row-major summation order
// requires.
func TestDirectedModularity_Deterministic(t *testing.T) {
	g := asymmetricDirectedCSR()
	q := DirectedModularity(1.0)
	p := Partition{0, 1, 2}

	first := q.Quality(g, p)
	for i := range 16 {
		if got := q.Quality(g, p); got != first {
			t.Fatalf("Quality() not bit-identical: run %d = %v, first = %v", i, got, first)
		}
	}
}

// symmetricDirected returns a directed csr representing the same graph as the
// undirected u: every incident arc is kept in both the out- and in-adjacency,
// so out-degree equals in-degree at every node. Directed modularity on it must
// match undirected modularity on u.
func symmetricDirected(u *csr) *csr {
	return &csr{
		n:         u.n,
		directed:  true,
		out:       u.out,
		in:        u.out,
		selfLoops: u.selfLoops,
		nodeSizes: u.nodeSizes,
	}
}

// TestDirectedModularity_SymmetricReducesToUndirected checks that on a symmetric
// directed graph directed modularity equals undirected modularity, across a
// range of gamma and partitions.
func TestDirectedModularity_SymmetricReducesToUndirected(t *testing.T) {
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
		for _, gamma := range gammas {
			for _, p := range gr.ps {
				want := Modularity(gamma).Quality(gr.u, p)
				got := DirectedModularity(gamma).Quality(dir, p)
				if got != want {
					t.Errorf("%s gamma=%v p=%v: directed Q = %v, want undirected Q = %v",
						gr.name, gamma, p, got, want)
				}
			}
		}
	}
}
