package meso

import "testing"

// triangleCSR is a fixed 3-node graph with unit-weight edges between every pair
// (a triangle) and no self-loops, used across the CSR model tests.
//
//	offsets:   [0 2 4 6]
//	neighbors: [1 2 | 0 2 | 0 1]
//	weights:   [1 1 | 1 1 | 1 1]
func triangleCSR() *csr {
	return newCSR(
		[]int{0, 2, 4, 6},
		[]int{1, 2, 0, 2, 0, 1},
		[]float64{1, 1, 1, 1, 1, 1},
		nil,
		nil,
	)
}

// selfLoopCSR is a 2-node graph with a single off-diagonal edge 0-1 of weight 3
// and a self-loop of weight 5 on node 0.
func selfLoopCSR() *csr {
	return newCSR(
		[]int{0, 1, 2},
		[]int{1, 0},
		[]float64{3, 3},
		[]float64{5, 0},
		nil,
	)
}

func TestCSR_TwoM(t *testing.T) {
	if got := triangleCSR().twoM(); got != 6 {
		t.Errorf("triangle twoM() = %v, want 6", got)
	}

	// Off-diagonal weight 3 is counted from both endpoints (2*3); the self-loop
	// weight 5 is counted exactly once: 6 + 5 = 11.
	g := selfLoopCSR()
	if got := g.degree(0); got != 8 {
		t.Errorf("degree(0) = %v, want 8", got)
	}
	if got := g.twoM(); got != 11 {
		t.Errorf("self-loop twoM() = %v, want 11", got)
	}
}

func TestCSR_DegenerateShapes(t *testing.T) {
	// Empty graph: n = 0, offsets = [0].
	empty := newCSR([]int{0}, nil, nil, nil, nil)
	if err := empty.checkInvariants(); err != nil {
		t.Errorf("empty: checkInvariants() = %v, want nil", err)
	}
	if empty.numNodes() != 0 || empty.twoM() != 0 {
		t.Errorf("empty: numNodes=%d twoM=%v, want 0 0", empty.numNodes(), empty.twoM())
	}

	// Single isolated node: degree 0, default size, no neighbours.
	single := newCSR([]int{0, 0}, nil, nil, nil, nil)
	if err := single.checkInvariants(); err != nil {
		t.Errorf("single: checkInvariants() = %v, want nil", err)
	}
	if single.degree(0) != 0 || len(single.neighbors(0)) != 0 || single.nodeSize(0) != 1.0 {
		t.Errorf("single: degree=%v neighbours=%d size=%v", single.degree(0), len(single.neighbors(0)), single.nodeSize(0))
	}

	// A graph with an isolated node (node 2) alongside an edge 0-1.
	iso := newCSR([]int{0, 1, 2, 2}, []int{1, 0}, []float64{4, 4}, nil, nil)
	if err := iso.checkInvariants(); err != nil {
		t.Errorf("isolated: checkInvariants() = %v, want nil", err)
	}
	if got := iso.degree(2); got != 0 {
		t.Errorf("isolated: degree(2) = %v, want 0", got)
	}
	if got := len(iso.neighbors(2)); got != 0 {
		t.Errorf("isolated: neighbors(2) len = %d, want 0", got)
	}
	if got := iso.twoM(); got != 8 {
		t.Errorf("isolated: twoM() = %v, want 8", got)
	}
}

func TestCSR_NodeSize(t *testing.T) {
	// nil nodeSizes defaults every node to 1.0.
	def := triangleCSR()
	for i := 0; i < def.numNodes(); i++ {
		if got := def.nodeSize(i); got != 1.0 {
			t.Errorf("default nodeSize(%d) = %v, want 1.0", i, got)
		}
	}

	// Explicit sizes are returned as stored.
	sized := newCSR([]int{0, 1, 2}, []int{1, 0}, []float64{1, 1}, nil, []float64{2.5, 4})
	if got := sized.nodeSize(0); got != 2.5 {
		t.Errorf("nodeSize(0) = %v, want 2.5", got)
	}
	if got := sized.nodeSize(1); got != 4 {
		t.Errorf("nodeSize(1) = %v, want 4", got)
	}
}

func TestCSR_InvariantsHold(t *testing.T) {
	for name, g := range map[string]*csr{"triangle": triangleCSR(), "selfLoop": selfLoopCSR()} {
		if err := g.checkInvariants(); err != nil {
			t.Errorf("%s: checkInvariants() = %v, want nil", name, err)
		}
	}
}

func TestCSR_InvariantsRejectMalformed(t *testing.T) {
	cases := map[string]*csr{
		"offsets not monotonic": {
			n:         3,
			out:       adjacency{offsets: []int{0, 2, 1, 6}, neighbors: []int{1, 2, 0, 2, 0, 1}, weights: []float64{1, 1, 1, 1, 1, 1}},
			selfLoops: make([]float64, 3),
			nodeSizes: []float64{1, 1, 1},
		},
		"neighbors/weights length mismatch": {
			n:         2,
			out:       adjacency{offsets: []int{0, 1, 2}, neighbors: []int{1, 0}, weights: []float64{1}},
			selfLoops: make([]float64, 2),
			nodeSizes: []float64{1, 1},
		},
		"offsets tail past neighbors": {
			n:         2,
			out:       adjacency{offsets: []int{0, 1, 3}, neighbors: []int{1, 0}, weights: []float64{1, 1}},
			selfLoops: make([]float64, 2),
			nodeSizes: []float64{1, 1},
		},
		"neighbor index out of range": {
			n:         2,
			out:       adjacency{offsets: []int{0, 1, 2}, neighbors: []int{9, 0}, weights: []float64{1, 1}},
			selfLoops: make([]float64, 2),
			nodeSizes: []float64{1, 1},
		},
		"selfLoops wrong length": {
			n:         2,
			out:       adjacency{offsets: []int{0, 1, 2}, neighbors: []int{1, 0}, weights: []float64{1, 1}},
			selfLoops: make([]float64, 1),
			nodeSizes: []float64{1, 1},
		},
	}
	for name, g := range cases {
		if err := g.checkInvariants(); err == nil {
			t.Errorf("%s: checkInvariants() = nil, want error", name)
		}
	}
}

func TestCSR_DegreeAndNeighbors(t *testing.T) {
	g := triangleCSR()

	if got := g.numNodes(); got != 3 {
		t.Fatalf("numNodes() = %d, want 3", got)
	}

	for i := range 3 {
		if got := g.degree(i); got != 2 {
			t.Errorf("degree(%d) = %v, want 2", i, got)
		}
	}

	nbrs := g.neighbors(1)
	if len(nbrs) != 2 || nbrs[0] != 0 || nbrs[1] != 2 {
		t.Errorf("neighbors(1) = %v, want [0 2]", nbrs)
	}
	w := g.neighborWeights(1)
	if len(w) != 2 || w[0] != 1 || w[1] != 1 {
		t.Errorf("neighborWeights(1) = %v, want [1 1]", w)
	}
}
