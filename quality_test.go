package meso

import (
	"math"
	"testing"
)

// pathCSR is a 3-node path 0-1-2 with unit-weight edges and no self-loops.
//
//	offsets:   [0 1 3 4]
//	neighbors: [1 | 0 2 | 1]
//	weights:   [1 | 1 1 | 1]
func pathCSR() *csr {
	return newCSR(
		[]int{0, 1, 3, 4},
		[]int{1, 0, 2, 1},
		[]float64{1, 1, 1, 1},
		nil,
		nil,
	)
}

// twoEdgesCSR is a 4-node graph of two disjoint unit-weight edges: 0-1 and 2-3.
//
//	offsets:   [0 1 2 3 4]
//	neighbors: [1 | 0 | 3 | 2]
//	weights:   [1 | 1 | 1 | 1]
func twoEdgesCSR() *csr {
	return newCSR(
		[]int{0, 1, 2, 3, 4},
		[]int{1, 0, 3, 2},
		[]float64{1, 1, 1, 1},
		nil,
		nil,
	)
}

const qualityTol = 1e-12

// Compile-time assertion that the modularity type satisfies QualityFunction.
var _ QualityFunction = modularity{}

// TestModularity_HandComputed checks Q against by-hand values on tiny fixtures.
func TestModularity_HandComputed(t *testing.T) {
	tests := []struct {
		name  string
		g     *csr
		gamma float64
		p     Partition
		want  float64
	}{
		// Triangle, one community, gamma = 1: Q = 1 - gamma = 0.
		{"triangle one-community gamma=1", triangleCSR(), 1.0, Partition{0, 0, 0}, 0.0},
		// Path 0-1-2, one community, gamma = 1: Q = 1 - gamma = 0.
		{"path one-community gamma=1", pathCSR(), 1.0, Partition{0, 0, 0}, 0.0},
		// Path, all singletons, gamma = 1: Q = -sum k_i^2 / (2m)^2.
		// k = [1,2,1], 2m = 4: -(1+4+1)/16 = -0.375.
		{"path singletons gamma=1", pathCSR(), 1.0, Partition{0, 1, 2}, -0.375},
		// Two disjoint edges, natural 2 communities, gamma = 1:
		// each block contributes (l/m - (d/2m)^2); m=2, per block l=1, d=2:
		// 2 * (1/2 - (2/4)^2) = 2 * 0.25 = 0.5.
		{"two-edges natural gamma=1", twoEdgesCSR(), 1.0, Partition{0, 0, 1, 1}, 0.5},
		// Two disjoint edges, natural 2 communities, gamma = 2:
		// sum_ij w delta = 4; sum_ij k_i k_j delta / 2m = 2; Q = (4 - 2*2)/4 = 0.
		{"two-edges natural gamma=2", twoEdgesCSR(), 2.0, Partition{0, 0, 1, 1}, 0.0},
		// Two disjoint edges, natural 2 communities, gamma = 0.5:
		// Q = (4 - 0.5*2)/4 = 0.75.
		{"two-edges natural gamma=0.5", twoEdgesCSR(), 0.5, Partition{0, 0, 1, 1}, 0.75},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Modularity(tt.gamma).Quality(tt.g, tt.p)
			if math.Abs(got-tt.want) > qualityTol {
				t.Errorf("Quality() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestModularity_ClosedForms checks the all-in-one and all-singletons partitions
// against their closed forms across a range of gamma.
func TestModularity_ClosedForms(t *testing.T) {
	graphs := []struct {
		name string
		g    *csr
	}{
		{"triangle", triangleCSR()},
		{"path", pathCSR()},
		{"two-edges", twoEdgesCSR()},
	}
	gammas := []float64{0.5, 1.0, 2.0}

	for _, gr := range graphs {
		n := gr.g.numNodes()
		twoM := gr.g.twoM()

		// sum of squared degrees, for the singletons closed form.
		var sumSq float64
		for i := range n {
			sumSq += gr.g.degree(i) * gr.g.degree(i)
		}
		// sum of self-loops: the diagonal weight retained by singletons.
		var sumSelf float64
		for i := range n {
			sumSelf += gr.g.weight(i, i)
		}

		allOne := make(Partition, n)
		singletons := make(Partition, n)
		for i := range n {
			singletons[i] = i
		}

		for _, gamma := range gammas {
			q := Modularity(gamma)

			// All-in-one: Q = (2m - gamma*(2m)^2/2m) / 2m = 1 - gamma
			// only when there are no self-loops (sum_ij w_ij = 2m). All three
			// fixtures are self-loop-free, so the clean form applies.
			wantAllOne := 1 - gamma
			if got := q.Quality(gr.g, allOne); math.Abs(got-wantAllOne) > qualityTol {
				t.Errorf("%s all-in-one gamma=%v: Quality() = %v, want %v", gr.name, gamma, got, wantAllOne)
			}

			// All-singletons: Q = (sum_i w_ii - gamma*sum_i k_i^2/2m) / 2m.
			wantSingles := (sumSelf - gamma*sumSq/twoM) / twoM
			if got := q.Quality(gr.g, singletons); math.Abs(got-wantSingles) > qualityTol {
				t.Errorf("%s singletons gamma=%v: Quality() = %v, want %v", gr.name, gamma, got, wantSingles)
			}
		}
	}
}

// TestModularity_SelfLoop exercises the diagonal term: a self-loop is the
// i == j weight and must enter both the edge sum and the degree.
func TestModularity_SelfLoop(t *testing.T) {
	// selfLoopCSR: edge 0-1 weight 3, self-loop 5 on node 0.
	// degrees: k0 = 3 + 5 = 8, k1 = 3; twoM = 8 + 3 = 11.
	g := selfLoopCSR()
	q := Modularity(1.0)

	// One community: sum_ij w_ij delta = w00 + w01 + w10 + w11 = 5 + 3 + 3 + 0 = 11.
	// sum_ij k_i k_j / 2m = (k0+k1)^2 / 2m = 121/11 = 11. Q = (11 - 11)/11 = 0.
	if got := q.Quality(g, Partition{0, 0}); math.Abs(got-0.0) > qualityTol {
		t.Errorf("self-loop one-community: Quality() = %v, want 0", got)
	}

	// Singletons: sum_i w_ii = 5; sum_i k_i^2 = 64 + 9 = 73.
	// Q = (5 - 73/11) / 11 = (5 - 6.6363...) / 11.
	want := (5.0 - 73.0/11.0) / 11.0
	if got := q.Quality(g, Partition{0, 1}); math.Abs(got-want) > qualityTol {
		t.Errorf("self-loop singletons: Quality() = %v, want %v", got, want)
	}
}

// TestModularity_Deterministic asserts repeated evaluation is bit-identical, the
// property the canonical (sorted) summation order guarantees.
func TestModularity_Deterministic(t *testing.T) {
	g := triangleCSR()
	q := Modularity(0.7)
	p := Partition{0, 0, 1}

	first := q.Quality(g, p)
	for i := range 100 {
		if got := q.Quality(g, p); got != first {
			t.Fatalf("evaluation %d = %v, want bit-identical %v", i, got, first)
		}
	}
}

// TestModularity_EmptyGraph checks the degenerate 2m == 0 case does not divide
// by zero.
func TestModularity_EmptyGraph(t *testing.T) {
	empty := newCSR([]int{0}, nil, nil, nil, nil)
	if got := Modularity(1.0).Quality(empty, Partition{}); got != 0 {
		t.Errorf("empty graph: Quality() = %v, want 0", got)
	}
}
