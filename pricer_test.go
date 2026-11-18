package meso

import (
	"math"
	"strconv"
	"testing"
)

// pricerGraph builds a small random graph with self-loops and explicit node
// sizes, with integer or non-integer weights and sizes.
func pricerGraph(t *testing.T, n, edges int, seed uint64, directed, fractional bool) *csr {
	t.Helper()
	r := newPRNG(seed)
	b := NewBuilder()
	if directed {
		b = NewDirectedBuilder()
	}
	b.Canonical()
	value := func() float64 {
		if fractional {
			return 0.1 + 3*r.float64()
		}
		return float64(1 + r.next()%5)
	}
	for u := range n {
		b.AddNodeWeight(strconv.Itoa(u), value())
	}
	for range edges {
		u, v := int(r.next()%uint64(n)), int(r.next()%uint64(n))
		b.AddEdge(strconv.Itoa(u), strconv.Itoa(v), value())
	}
	g, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	return g.model
}

// TestPricerTracksAppliedMoves drives a pricer through a long random sequence
// of applied moves and holds every delta it reports to the from-scratch
// moveDelta of the same partition: exactly on integer weights, where the kept
// community totals cannot round, and to within rounding otherwise.
func TestPricerTracksAppliedMoves(t *testing.T) {
	const (
		n      = 40
		labels = 9
		steps  = 400
		tol    = 1e-9
	)
	cases := []struct {
		name     string
		obj      objective
		directed bool
	}{
		{"modularity", modularity{gamma: 1.3}, false},
		{"cpm", cpm{gamma: 0.2}, false},
		{"directed", directedModularity{gamma: 0.8}, true},
		{"directed-on-undirected", directedModularity{gamma: 1}, false},
	}
	for _, tc := range cases {
		for _, fractional := range []bool{false, true} {
			name := tc.name + "/integer"
			if fractional {
				name = tc.name + "/fractional"
			}
			t.Run(name, func(t *testing.T) {
				for seed := uint64(1); seed <= 5; seed++ {
					g := pricerGraph(t, n, 4*n, seed, tc.directed, fractional)
					r := newPRNG(seed + 100)
					p := make(Partition, n)
					for i := range p {
						p[i] = int(r.next() % labels)
					}
					pr := tc.obj.newPricer(g, p)

					for step := range steps {
						u := int(r.next() % uint64(n))
						// Past the label range: an empty community, as isolation uses.
						target := int(r.next() % (labels + 3))
						if step%7 == 0 {
							target = n + step
						}

						got := pr.delta(p, u, target)
						want := tc.obj.moveDelta(g, p, u, target)
						if fractional {
							if math.Abs(got-want) > tol {
								t.Fatalf("seed %d step %d: delta(%d -> %d) = %v, want %v", seed, step, u, target, got, want)
							}
						} else if got != want {
							t.Fatalf("seed %d step %d: delta(%d -> %d) = %v, want exactly %v", seed, step, u, target, got, want)
						}

						src := p[u]
						if target != src {
							p[u] = target
							pr.moved(u, src, target)
						}
					}
				}
			})
		}
	}
}
