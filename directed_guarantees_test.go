package meso_test

import (
	"math"
	"testing"

	"github.com/andreswebs/meso"
)

// TestLeiden_DirectedSubsetOptimalityLimitation is a characterization test: it
// pins meso's current behaviour on the n=4 directed fixture from the Phase 3
// triage (docs/research/directed-modularity-triage.md, section 6), where the
// converged output is NOT subset-optimal.
//
// The fixture (arcs i -> j : w):
//
//	0 -> 1 : 1    1 -> 0 : 3    1 -> 2 : 1    2 -> 0 : 2
//	1 -> 3 : 5    0 -> 3 : 1    3 -> 2 : 5
//
// has m = 18. At gamma = 1 meso converges to the all-in-one partition
// [0,0,0,0] (quality 0), a genuine local optimum of the move-and-merge
// dynamics: no single-node move (isolation included) and no community merge
// improves directed modularity. Yet the two-community split {0,1},{2,3} scores
// Q = 1/27 ~ 0.037 > 0 (hand-verified in the triage; the split gain 1/27 is also
// reproduced by the out-of-band scout at verification/reference/directed-scout).
// Taking S = {0,1}, T = {2,3}: e(S,T)+e(T,S) = 9 while
// (gamma/m)(Kout_S*Kin_T + Kout_T*Kin_S) = 174/18 ~ 9.667, so the directed
// subset bound is violated by meso's output.
//
// This is a documented limitation, tracked by ticket mes-niic, not a
// regression. It is inherited from modularity (the symmetric regime shows the
// same behaviour by the reduction), and it is consistent with the formal model:
// the Lean development proves the directed subset bound only as a CONDITIONAL on
// directed subset stability (Meso.directedSubsetGammaDense_of_subsetStable), a
// hypothesis meso's refinement does not always attain. See CORRESPONDENCE.md.
//
// The all-in-one outcome is seed-independent here: meso's phase-1 local move is
// seed-independent, so multi-seed restarts do not escape this basin (see
// docs/specs/learnings.md). The node registration order is pinned by the AddEdge
// sequence below (keys interned in first-seen order 0,1,2,3); a different order
// permutes the internal indices and can let meso escape to the better split, so
// the sequence is load-bearing, not incidental.
//
// A future refinement improvement (mes-niic) that attains subset stability will
// intentionally flip this test: meso would then return the {0,1},{2,3} split and
// the assertions below would need updating to the improved behaviour.
func TestLeiden_DirectedSubsetOptimalityLimitation(t *testing.T) {
	// AddEdge order pins the first-seen interning to 0,1,2,3 (node "2" is seen
	// before node "3"), reproducing the converged all-in-one basin.
	g, err := meso.NewDirectedBuilder().
		AddEdge("0", "1", 1).
		AddEdge("1", "0", 3).
		AddEdge("1", "2", 1).
		AddEdge("2", "0", 2).
		AddEdge("1", "3", 5).
		AddEdge("0", "3", 1).
		AddEdge("3", "2", 5).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	res, err := meso.Leiden(g, meso.WithQuality(meso.DirectedModularity(1.0)), meso.WithSeed(1))
	if err != nil {
		t.Fatalf("Leiden() error = %v", err)
	}
	comm := res.Communities()

	// Characterization: meso converges to the all-in-one partition. This is the
	// subset-suboptimal local optimum described above; the better split exists but
	// meso does not reach it.
	if n := numCommunitiesOf(comm); n != 1 {
		t.Fatalf("directed Leiden found %d communities, want 1 (all-in-one); "+
			"the subset-optimality limitation (mes-niic) may have been fixed - "+
			"if so, update this characterization test: %v", n, comm)
	}
	for _, k := range []string{"0", "1", "2", "3"} {
		if comm[k] != comm["0"] {
			t.Fatalf("node %q not in the all-in-one community: %v", k, comm)
		}
	}

	// The all-in-one partition scores exactly 0 at gamma = 1
	// (directedModularity_const_one in the Lean model). The better {0,1},{2,3}
	// split scores 1/27 > 0, so meso's converged output is subset-suboptimal.
	if q := res.Quality(); math.Abs(q) > 1e-9 {
		t.Fatalf("all-in-one quality = %v, want ~0 (the subset-suboptimal fixed point)", q)
	}
}
