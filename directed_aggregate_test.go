package meso

import (
	"math"
	"math/rand"
	"testing"
)

// TestAggregate_DirectedSeparability pins acceptance criterion 3: directed
// aggregation preserves in/out degree separability across a level. Each
// super-node's aggregate out-degree (in-degree) equals the sum of its members'
// out-degrees (in-degrees), the aggregate is itself directed, and the asymmetry
// survives - at least one super-node has out-degree != in-degree, so the level
// has not silently collapsed to an undirected graph.
func TestAggregate_DirectedSeparability(t *testing.T) {
	const trials = 2000
	base := rand.New(rand.NewSource(29))
	sawAsymmetric := false
	for range trials {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))
		g := randomDirectedCSR(rng)
		p := randomPartition(rng, g.numNodes())

		agg, superOf := aggregate(g, p)
		if !agg.directed {
			t.Fatalf("seed %d: aggregate of a directed graph is not directed", seed)
		}
		if err := agg.checkInvariants(); err != nil {
			t.Fatalf("seed %d: aggregate invariants: %v", seed, err)
		}

		wantOut := make([]float64, agg.numNodes())
		wantIn := make([]float64, agg.numNodes())
		for i := 0; i < g.numNodes(); i++ {
			a := superOf[i]
			wantOut[a] += g.outDegree(i)
			wantIn[a] += g.inDegree(i)
		}
		for a := 0; a < agg.numNodes(); a++ {
			if math.Abs(agg.outDegree(a)-wantOut[a]) > aggTol {
				t.Fatalf("seed %d: super-node %d out-degree %v, want %v", seed, a, agg.outDegree(a), wantOut[a])
			}
			if math.Abs(agg.inDegree(a)-wantIn[a]) > aggTol {
				t.Fatalf("seed %d: super-node %d in-degree %v, want %v", seed, a, agg.inDegree(a), wantIn[a])
			}
			if math.Abs(agg.outDegree(a)-agg.inDegree(a)) > aggTol {
				sawAsymmetric = true
			}
		}
	}
	if !sawAsymmetric {
		t.Fatal("no aggregate super-node kept out-degree != in-degree; separability may have collapsed")
	}
}

// TestAggregate_DirectedRoundTrip checks the level-composition property for
// directed modularity: scoring any aggregate partition equals scoring its base
// expansion, so a directed run can carry a partition found on the aggregate back
// down without re-scoring. This is the directed analogue of
// TestAggregate_RoundTrip.
func TestAggregate_DirectedRoundTrip(t *testing.T) {
	const trials = 3000
	base := rand.New(rand.NewSource(31))
	for range trials {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))
		g := randomDirectedCSR(rng)
		p := randomPartition(rng, g.numNodes())
		gamma := 0.3 + rng.Float64()*1.5

		agg, superOf := aggregate(g, p)
		aggP := randomPartition(rng, agg.numNodes())
		lifted := expand(superOf, aggP)

		qf := DirectedModularity(gamma)
		got := qf.Quality(agg, aggP)
		want := qf.Quality(g, lifted)
		if math.Abs(got-want) > aggTol {
			t.Fatalf("seed %d: DirectedModularity Quality(agg, aggP) %v != Quality(g, expand) %v (diff %g)",
				seed, got, want, math.Abs(got-want))
		}
	}
}

// TestAggregate_SymmetricDirectedIsSymmetric checks that aggregating a symmetric
// directed graph yields a symmetric aggregate (out-degree equals in-degree at
// every super-node), and that its self-loops and off-diagonal weights match the
// undirected aggregate of the same base. This is what makes directed Leiden on a
// symmetric graph track undirected Leiden across levels (acceptance criterion 2,
// the aggregation link of the end-to-end reduction).
func TestAggregate_SymmetricDirectedIsSymmetric(t *testing.T) {
	const trials = 1500
	base := rand.New(rand.NewSource(37))
	for range trials {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))
		u := randomCSR(rng)
		p := randomPartition(rng, u.numNodes())

		uAgg, uSuper := aggregate(u, p)
		dAgg, dSuper := aggregate(symmetricDirected(u), p)

		if dAgg.numNodes() != uAgg.numNodes() {
			t.Fatalf("seed %d: aggregate node counts differ: directed %d, undirected %d",
				seed, dAgg.numNodes(), uAgg.numNodes())
		}
		for i := range uSuper {
			if uSuper[i] != dSuper[i] {
				t.Fatalf("seed %d: superOf differs at node %d: directed %d, undirected %d",
					seed, i, dSuper[i], uSuper[i])
			}
		}
		for a := 0; a < dAgg.numNodes(); a++ {
			if math.Abs(dAgg.outDegree(a)-dAgg.inDegree(a)) > aggTol {
				t.Fatalf("seed %d: symmetric-directed aggregate super-node %d asymmetric: out %v, in %v",
					seed, a, dAgg.outDegree(a), dAgg.inDegree(a))
			}
			if math.Abs(dAgg.selfLoops[a]-uAgg.selfLoops[a]) > aggTol {
				t.Fatalf("seed %d: super-node %d self-loop directed %v != undirected %v",
					seed, a, dAgg.selfLoops[a], uAgg.selfLoops[a])
			}
			if math.Abs(dAgg.outDegree(a)-uAgg.degree(a)) > aggTol {
				t.Fatalf("seed %d: super-node %d out-degree directed %v != undirected degree %v",
					seed, a, dAgg.outDegree(a), uAgg.degree(a))
			}
		}
	}
}
