package meso

import (
	"math"
	"math/rand"
	"testing"
)

// bruteBestDelta is the reference local-move oracle: the maximum move-delta over
// every occupied community plus a fresh isolation label (bestMove's acceptance
// rule keeps only strictly positive gains, so a non-positive maximum means "stay
// put", delta 0). It exhaustively scores every community, so on a directed graph
// it exposes any target - in particular one reached only through an in-arc - that
// a one-directional local move would miss. Targets are not returned: several
// communities can tie for the optimum, and bestMove is free to pick any of them.
func bruteBestDelta(g *csr, obj objective, p Partition, u, isolateLabel int) float64 {
	src := p[u]
	labelSet := map[int]struct{}{isolateLabel: {}}
	for _, c := range p {
		labelSet[c] = struct{}{}
	}
	best := 0.0
	for c := range labelSet {
		if c == src {
			continue
		}
		if d := obj.moveDelta(g, p, u, c); d > best {
			best = d
		}
	}
	return best
}

// TestBestMove_DirectedMatchesBruteForce checks that on directed graphs bestMove
// attains the exhaustive optimum move-delta, including gains available only
// through an in-arc. bestMove searches neighbour communities in both arc
// directions plus isolation; isolation provably dominates any community u shares
// no arc with (its edge term is zero and its null penalty is smallest), so that
// search space attains the global optimum. Iterating out-neighbours alone would
// miss in-arc gains, so this fails until the local move considers both directions.
// The achieved delta is compared, not the target: distinct communities can tie
// for the optimum and bestMove may pick any of them.
func TestBestMove_DirectedMatchesBruteForce(t *testing.T) {
	const trials = 4000
	obj := directedModularity{gamma: 1.0}
	base := rand.New(rand.NewSource(41))
	for range trials {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))
		g := randomDirectedCSR(rng)
		p := randomPartition(rng, g.numNodes())
		isolate := g.numNodes()
		for _, c := range p {
			if c >= isolate {
				isolate = c + 1
			}
		}
		for u := 0; u < g.numNodes(); u++ {
			wantD := bruteBestDelta(g, obj, p, u, isolate)
			gotT, gotD := bestMove(g, obj, p, u, isolate)
			if math.Abs(gotD-wantD) > moveTol {
				t.Fatalf("seed %d node %d: bestMove delta %v, exhaustive optimum %v",
					seed, u, gotD, wantD)
			}
			// The returned target must actually deliver the returned delta.
			if gotT != p[u] {
				if d := obj.moveDelta(g, p, u, gotT); math.Abs(d-gotD) > moveTol {
					t.Fatalf("seed %d node %d: target %d delta %v != reported %v",
						seed, u, gotT, d, gotD)
				}
			}
		}
	}
}

// TestBestMove_SymmetricDirectedMatchesUndirected confirms the directed local
// move does not perturb the symmetric case: on a symmetric directed graph
// bestMove returns the same target (the decision) as on the undirected original
// for every node, with a delta agreeing to within moveTol. The delta is not
// bit-identical because the directed formula sums its null term in a different
// order (koutU*.. + kinU*.. rather than 2*ku*..), landing ~1 ULP away, exactly
// as the directed move-delta note in the learnings records. This is the
// local-move half of the end-to-end reduction.
func TestBestMove_SymmetricDirectedMatchesUndirected(t *testing.T) {
	const trials = 1500
	base := rand.New(rand.NewSource(43))
	for range trials {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))
		u := randomCSR(rng)
		p := randomPartition(rng, u.numNodes())
		dir := symmetricDirected(u)
		isolate := u.numNodes()
		for _, c := range p {
			if c >= isolate {
				isolate = c + 1
			}
		}
		uObj := modularity{gamma: 1.0}
		dObj := directedModularity{gamma: 1.0}
		for v := 0; v < u.numNodes(); v++ {
			wantT, wantD := bestMove(u, uObj, p, v, isolate)
			gotT, gotD := bestMove(dir, dObj, p, v, isolate)
			if gotT != wantT || math.Abs(gotD-wantD) > moveTol {
				t.Fatalf("seed %d node %d: directed bestMove = (%d, %v), undirected = (%d, %v)",
					seed, v, gotT, gotD, wantT, wantD)
			}
		}
	}
}

// TestRefine_SymmetricDirectedReducesToUndirected checks that refinement on a
// symmetric directed graph gives byte-identical sub-communities to refinement on
// the undirected original, across seeds and outer partitions. This is the
// refinement half of the end-to-end reduction (acceptance criterion 2).
func TestRefine_SymmetricDirectedReducesToUndirected(t *testing.T) {
	const trials = 1200
	base := rand.New(rand.NewSource(47))
	for range trials {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))
		u := randomCSR(rng)
		outer := louvain(u, modularity{gamma: 1.0})
		dir := symmetricDirected(u)
		for _, s := range []uint64{0, 1, 7, 42} {
			want := refine(u, modularity{gamma: 1.0}, outer, s)
			got := refine(dir, directedModularity{gamma: 1.0}, outer, s)
			if len(got) != len(want) {
				t.Fatalf("seed %d s=%d: length %d != %d", seed, s, len(got), len(want))
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("seed %d s=%d: refine differs at node %d: directed %d, undirected %d",
						seed, s, i, got[i], want[i])
				}
			}
		}
	}
}
