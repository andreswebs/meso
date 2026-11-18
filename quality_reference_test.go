package meso

import (
	"math"
	"testing"
)

// literalQuality is the dense row-major double sum each Quality method is
// defined by: every pair (i, j) is visited, and same-community pairs add their
// kernel term. It is the reference the per-community evaluation is held to.
func literalQuality(g *csr, p Partition, term func(i, j int) float64) float64 {
	sum := 0.0
	for i := range g.numNodes() {
		for j := range g.numNodes() {
			if p[i] != p[j] {
				continue
			}
			sum += term(i, j)
		}
	}
	return sum
}

// TestQualityMatchesLiteralDoubleSum holds each Quality method to the dense
// double sum it is defined by. Quality groups the sum by community, which adds
// the same terms in a different order, so the two agree to rounding and not bit
// for bit.
func TestQualityMatchesLiteralDoubleSum(t *testing.T) {
	const n = 60
	for seed := uint64(1); seed <= 8; seed++ {
		for _, directed := range []bool{false, true} {
			g := pricerGraph(t, n, 5*n, seed, directed, true)
			r := newPRNG(seed + 50)
			p := make(Partition, n)
			for i := range p {
				// Sparse, unordered labels, including ones past n.
				p[i] = int(r.next()%7) * 1000
			}
			total := g.twoM()

			if !directed {
				gamma := 1.1
				want := literalQuality(g, p, func(i, j int) float64 {
					return g.weight(i, j) - gamma*g.degree(i)*g.degree(j)/total
				}) / total
				if got := (modularity{gamma: gamma}).Quality(g, p); !closeTo(got, want) {
					t.Errorf("seed %d: modularity Quality = %v, want %v", seed, got, want)
				}

				want = literalQuality(g, p, func(i, j int) float64 {
					return g.weight(i, j) - gamma*g.nodeSize(i)*g.nodeSize(j)
				})
				if got := (cpm{gamma: gamma}).Quality(g, p); !closeTo(got, want) {
					t.Errorf("seed %d: cpm Quality = %v, want %v", seed, got, want)
				}
			}

			gamma := 0.9
			want := literalQuality(g, p, func(i, j int) float64 {
				return g.weight(i, j) - gamma*g.outDegree(i)*g.inDegree(j)/total
			}) / total
			if got := (directedModularity{gamma: gamma}).Quality(g, p); !closeTo(got, want) {
				t.Errorf("seed %d directed=%v: directed Quality = %v, want %v", seed, directed, got, want)
			}
		}
	}
}

// closeTo reports whether got equals want to a relative 1e-12.
func closeTo(got, want float64) bool {
	return math.Abs(got-want) <= 1e-12*math.Max(1, math.Abs(want))
}
