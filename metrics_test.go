package meso

import (
	"math"
	"testing"
)

// closeEnough reports whether got is within tol of want.
func closeEnough(got, want, tol float64) bool {
	return math.Abs(got-want) <= tol
}

// randomLabels builds a partition of n nodes with labels drawn uniformly from
// [0, k) using meso's own PRNG, so the draw is reproducible.
func randomLabels(seed uint64, n, k int) Partition {
	r := newPRNG(seed)
	p := make(Partition, n)
	for i := range p {
		p[i] = int(r.next() % uint64(k))
	}
	return p
}

func TestMetrics_IndependentScoreNearZero(t *testing.T) {
	// Two independently drawn labelings share no real structure, so the
	// chance-adjusted metrics must sit near zero. The threshold is far above the
	// empirical spread at this size (max |score| ~ 0.03 across many seeds).
	const bound = 0.1
	const n, k = 200, 5
	for s := range uint64(20) {
		a := randomLabels(2*s+1, n, k)
		b := randomLabels(2*s+2, n, k)
		if got := AMI(a, b); math.Abs(got) > bound {
			t.Errorf("AMI(random, random) seed %d = %v, want within %v of 0", s, got, bound)
		}
		if got := ARI(a, b); math.Abs(got) > bound {
			t.Errorf("ARI(random, random) seed %d = %v, want within %v of 0", s, got, bound)
		}
	}
}

func TestMetrics_Symmetric(t *testing.T) {
	const tol = 1e-12
	pairs := []struct {
		name string
		a, b Partition
	}{
		{"partial overlap", Partition{0, 0, 1, 1, 2, 2}, Partition{0, 0, 1, 2, 2, 2}},
		{"nested", Partition{0, 1, 2, 3}, Partition{0, 0, 1, 1}},
		{"uneven", Partition{0, 0, 0, 1, 1, 1, 2, 2, 2, 2}, Partition{0, 1, 2, 0, 1, 2, 0, 1, 2, 0}},
		{"random", randomLabels(11, 120, 4), randomLabels(12, 120, 7)},
	}
	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			for _, m := range []struct {
				name string
				fn   func(a, b Partition) float64
			}{{"NMI", NMI}, {"AMI", AMI}, {"ARI", ARI}} {
				ab := m.fn(p.a, p.b)
				ba := m.fn(p.b, p.a)
				if !closeEnough(ab, ba, tol) {
					t.Errorf("%s not symmetric: (a,b)=%v, (b,a)=%v", m.name, ab, ba)
				}
			}
		})
	}
}

func TestMetrics_Degenerate(t *testing.T) {
	const tol = 1e-12
	cases := []struct {
		name string
		a, b Partition
	}{
		{"empty", Partition{}, Partition{}},
		{"single node", Partition{0}, Partition{0}},
		{"all one community", Partition{0, 0, 0, 0}, Partition{0, 0, 0, 0}},
		{"all singletons", Partition{0, 1, 2, 3}, Partition{0, 1, 2, 3}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, m := range []struct {
				name string
				fn   func(a, b Partition) float64
			}{{"NMI", NMI}, {"AMI", AMI}, {"ARI", ARI}} {
				if got := m.fn(tc.a, tc.b); !closeEnough(got, 1.0, tol) {
					t.Errorf("%s(%v, %v) = %v, want 1", m.name, tc.a, tc.b, got)
				}
			}
		})
	}
}

func TestMetrics_LengthMismatchPanics(t *testing.T) {
	for _, m := range []struct {
		name string
		fn   func(a, b Partition) float64
	}{{"NMI", NMI}, {"AMI", AMI}, {"ARI", ARI}} {
		t.Run(m.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("%s(len 2, len 3) did not panic", m.name)
				}
			}()
			m.fn(Partition{0, 1}, Partition{0, 1, 2})
		})
	}
}

func TestAMI(t *testing.T) {
	const tol = 1e-12
	tests := []struct {
		name string
		a, b Partition
		want float64
	}{
		// Identical partitions: MI equals the normalizer, so after subtracting
		// the expected MI numerator and denominator coincide and AMI = 1.
		{"identical", Partition{0, 0, 1, 1, 2, 2}, Partition{0, 0, 1, 1, 2, 2}, 1.0},
		// Hand-verified small table (n=3): rows [2,1], cols [1,2]. The chance
		// adjustment drives this below zero to exactly -0.5.
		{"below chance n=3", Partition{0, 0, 1}, Partition{0, 1, 1}, -0.5},
		// Maximally disagreeing 2x2 grid: exactly -0.5 after adjustment.
		{"below chance grid", Partition{0, 0, 1, 1}, Partition{0, 1, 0, 1}, -0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AMI(tt.a, tt.b); !closeEnough(got, tt.want, tol) {
				t.Errorf("AMI(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestNMI(t *testing.T) {
	const tol = 1e-12
	tests := []struct {
		name string
		a, b Partition
		want float64
	}{
		// Identical partitions carry all of each other's information: NMI = 1.
		{"identical", Partition{0, 0, 1, 1, 2, 2}, Partition{0, 0, 1, 1, 2, 2}, 1.0},
		// a is a strict refinement of b, so MI = H(b) and, with the
		// arithmetic-mean normalizer, NMI = H(b)/((H(a)+H(b))/2) = ln2/(1.5 ln2) = 2/3.
		{"nested refinement", Partition{0, 1, 2, 3}, Partition{0, 0, 1, 1}, 2.0 / 3.0},
		// A 2x2 grid with independent labelings shares no information: MI = 0.
		{"independent grid", Partition{0, 0, 1, 1}, Partition{0, 1, 0, 1}, 0.0},
		// Both partitions collapse to a single community: defined as identical.
		{"both single community", Partition{0, 0, 0}, Partition{0, 0, 0}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NMI(tt.a, tt.b); !closeEnough(got, tt.want, tol) {
				t.Errorf("NMI(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestARI(t *testing.T) {
	const tol = 1e-12
	tests := []struct {
		name string
		a, b Partition
		want float64
	}{
		// Identical partitions score exactly 1.
		{"identical", Partition{0, 0, 1, 1, 2, 2}, Partition{0, 0, 1, 1, 2, 2}, 1.0},
		// Hand-computed Hubert-Arabie value. Contingency rows [2],[1,1],[2];
		// sum_comb=2, sum_a=3, sum_b=4, C(6,2)=15: (2-0.8)/(3.5-0.8)=4/9.
		{"partial overlap", Partition{0, 0, 1, 1, 2, 2}, Partition{0, 0, 1, 2, 2, 2}, 4.0 / 9.0},
		// Maximally disagreeing 2x2 grid scores below chance: exactly -0.5.
		{"below chance grid", Partition{0, 0, 1, 1}, Partition{0, 1, 0, 1}, -0.5},
		// A relabeling of the same partition is still identical.
		{"relabeled", Partition{0, 0, 1, 1}, Partition{7, 7, 3, 3}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ARI(tt.a, tt.b); !closeEnough(got, tt.want, tol) {
				t.Errorf("ARI(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
