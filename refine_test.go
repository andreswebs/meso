package meso

import (
	"math/rand"
	"reflect"
	"testing"
)

// TestConnectivity_SingletonConnected is acceptance criterion 1 and the Go image
// of connectedCommunities_singleton (verification/lean/Meso/Connectivity.lean):
// with every node its own community, each community is a single node and a
// one-vertex subgraph is trivially connected, so connectedCommunities holds.
func TestConnectivity_SingletonConnected(t *testing.T) {
	graphs := []struct {
		name string
		g    *csr
	}{
		{"triangle", triangleCSR()},
		{"path", pathCSR()},
		{"two disjoint edges", twoEdgesCSR()},
		{"self-loop", selfLoopCSR()},
	}
	for _, tc := range graphs {
		p := singleton(tc.g.numNodes())
		for c := 0; c < tc.g.numNodes(); c++ {
			if !communityConnected(tc.g, p, c) {
				t.Errorf("%s: singleton community %d reported disconnected", tc.name, c)
			}
		}
		if !connectedCommunities(tc.g, p) {
			t.Errorf("%s: singleton partition reported not all-connected", tc.name)
		}
	}
}

// TestRefine_MergePreservesConnectivity is acceptance criterion 2 and the Go
// image of MergeStep.connectedCommunities (verification/lean/Meso/Refinement.lean):
// merging two communities that share an edge, starting from a partition whose
// communities are connected, yields a partition whose communities are still
// connected. The merged community is the union of two connected pieces joined by
// an edge; every other community is unchanged.
func TestRefine_MergePreservesConnectivity(t *testing.T) {
	// 4-node path 0-1-2-3. Communities {0,1} and {2,3} are each connected and
	// share the edge 1-2. Merging b=1 into a=0 unions them into the connected
	// path {0,1,2,3}.
	g := newCSR(
		[]int{0, 1, 3, 5, 6},
		[]int{1, 0, 2, 1, 3, 2},
		[]float64{1, 1, 1, 1, 1, 1},
		nil, nil,
	)
	p := Partition{0, 0, 1, 1}
	if !connectedCommunities(g, p) {
		t.Fatal("precondition: split partition should have connected communities")
	}
	q := mergeCommunities(p, 0, 1)
	if !connectedCommunities(g, q) {
		t.Errorf("edge-merge across shared edge 1-2 broke connectivity: %v", q)
	}

	// Merging two adjacent singletons also stays connected.
	s := singleton(g.numNodes())
	if r := mergeCommunities(s, 0, 1); !connectedCommunities(g, r) {
		t.Errorf("merging adjacent singletons 0,1 broke connectivity: %v", r)
	}
}

// TestRefine_RunConnected is acceptance criterion 3 and the Go image of
// connectedCommunities_of_mergeRun (verification/lean/Meso/Refinement.lean): any
// sequence of edge-merges starting from the singleton partition keeps every
// community connected. Each step merges across a shared edge, so connectivity is
// the invariant of the whole run.
func TestRefine_RunConnected(t *testing.T) {
	g := triangleCSR() // 0-1-2 all pairwise adjacent
	p := singleton(g.numNodes())
	if !connectedCommunities(g, p) {
		t.Fatal("base case: singletons should be connected")
	}
	// Merge 1 into 0 across edge 0-1.
	p = mergeCommunities(p, 0, 1)
	if !connectedCommunities(g, p) {
		t.Fatalf("after merge {0,1}: not connected: %v", p)
	}
	// Merge 2 into 0 across edge 0-2.
	p = mergeCommunities(p, 0, 2)
	if !connectedCommunities(g, p) {
		t.Fatalf("after merge {0,1,2}: not connected: %v", p)
	}
	if numCommunities(p) != 1 {
		t.Fatalf("expected one community after two merges, got %d", numCommunities(p))
	}
}

// gammaUnionCSR is the fixture for the gamma-density tests: a 4-cycle 0-1-3-2-0
// with unit edges (0-1, 2-3, 0-2, 1-3) and unit node sizes. At gamma = 0.5 the
// two communities {0,1} and {2,3} are each internally gamma-dense (e = 2 = 0.5*2^2)
// and their cut is gamma-dense (e(S,T) = 2 = 0.5*2*2), so the union {0,1,2,3} is
// gamma-dense too (e = 8 = 0.5*4^2). Every inequality holds with equality, which
// exercises the boundary of the <= gate.
func gammaUnionCSR() *csr {
	return newCSR(
		[]int{0, 2, 4, 6, 8},
		[]int{1, 2, 0, 3, 0, 3, 1, 2},
		[]float64{1, 1, 1, 1, 1, 1, 1, 1},
		nil, nil,
	)
}

// TestCPM_GammaDenseUnion is acceptance criterion 5 and the Go image of
// gammaDense_union (verification/lean/Meso/GammaConnectivity.lean): if two
// disjoint communities are each internally gamma-dense and the cut between them is
// gamma-dense, their union is internally gamma-dense. e(S∪T) = e_S + e_T + 2·e(S,T)
// and (S_S+S_T)^2 expands to match, so the three piecewise bounds add up.
func TestCPM_GammaDenseUnion(t *testing.T) {
	g := gammaUnionCSR()
	const gamma = 0.5
	p := Partition{0, 0, 1, 1}

	if !isGammaDense(g, gamma, p, 0) {
		t.Fatal("precondition: community S = {0,1} should be gamma-dense")
	}
	if !isGammaDense(g, gamma, p, 1) {
		t.Fatal("precondition: community T = {2,3} should be gamma-dense")
	}
	if !gammaDenseCut(g, gamma, p, 0, 1) {
		t.Fatal("precondition: the cut S-T should be gamma-dense")
	}

	union := mergeCommunities(p, 0, 1)
	if !isGammaDense(g, gamma, union, 0) {
		t.Errorf("union {0,1,2,3} not gamma-dense: e=%v, gamma*S^2=%v",
			communityInternalWeight(g, union, 0),
			gamma*communitySize(g, union, 0)*communitySize(g, union, 0))
	}
}

// TestRefine_MergePreservesGammaDensity is acceptance criterion 6 and the Go
// image of GammaMergeStep.gammaDenseCommunities
// (verification/lean/Meso/GammaConnectivity.lean): if every community of p is
// gamma-dense and q merges two of them across a gamma-dense cut, every community
// of q is gamma-dense. The merged community is a union over a gamma-dense cut
// (gammaDense_union); all others are unchanged.
func TestRefine_MergePreservesGammaDensity(t *testing.T) {
	g := gammaUnionCSR()
	const gamma = 0.5
	p := Partition{0, 0, 1, 1}

	if !gammaDenseCommunities(g, gamma, p) {
		t.Fatal("precondition: both communities should be gamma-dense")
	}
	if !gammaDenseCut(g, gamma, p, 0, 1) {
		t.Fatal("precondition: the merged cut should be gamma-dense")
	}

	q := mergeCommunities(p, 0, 1)
	if !gammaDenseCommunities(g, gamma, q) {
		t.Errorf("gated merge did not preserve gamma-density of every community: %v", q)
	}
}

// TestRefine_GateRefusesDisconnecting is acceptance criterion 4: the
// well-connectedness gate refuses a merge that would create a disconnected
// community. The fixture is a 3-node path 0-1-2 (edges 0-1 and 1-2, no 0-2), all
// three inside one phase-1 community. A naive merge of 0 and 2 would form the
// community {0,2} with no internal edge - disconnected - so refinement must never
// produce it: 0 and 2 may share a refined sub-community only alongside 1.
func TestRefine_GateRefusesDisconnecting(t *testing.T) {
	g := pathCSR() // 0-1-2 path, no 0-2 edge
	outer := Partition{0, 0, 0}

	// The naive merge that the gate must refuse really does disconnect.
	naive := mergeCommunities(singleton(g.numNodes()), 0, 2)
	if connectedCommunities(g, naive) {
		t.Fatal("fixture broken: merging non-adjacent 0 and 2 should disconnect a community")
	}

	for _, obj := range []objective{modularity{gamma: 1.0}, cpm{gamma: 0.1}} {
		intents := refineIntents(g, obj, outer, 7)
		if intents[0] == 2 {
			t.Errorf("%T: node 0 intended to merge into non-adjacent node 2", obj)
		}
		if intents[2] == 0 {
			t.Errorf("%T: node 2 intended to merge into non-adjacent node 0", obj)
		}

		refined := refine(g, obj, outer, 7)
		if !connectedCommunities(g, refined) {
			t.Errorf("%T: refined partition has a disconnected community: %v", obj, refined)
		}
		// 0 and 2 may only be together if 1 joined them.
		if refined[0] == refined[2] && refined[1] != refined[0] {
			t.Errorf("%T: 0 and 2 merged without 1 - a disconnected community: %v", obj, refined)
		}
	}
}

// TestRefine_SchedulingIndependent is acceptance criterion 7: refinement
// randomness is per-node seeded, so the result is independent of the order nodes
// are processed. Each node's merge intent is a pure function of (globalSeed,
// nodeID) via nodeSeed, and the intents are resolved by an order-independent union
// (min-root tie-break), so applying them in any permutation yields the same
// canonicalised partition. refine itself is therefore byte-identical across runs.
func TestRefine_SchedulingIndependent(t *testing.T) {
	for _, fg := range corpusAndFuzzed(t, 4242) {
		for _, obj := range []objective{modularity{gamma: 1.0}, cpm{gamma: 0.1}} {
			g := fg.g
			n := g.numNodes()

			// Phase-1 partition seeds the outer communities refinement runs within.
			outer := singleton(n)
			localMoveQueue(g, obj, outer)

			intents := refineIntents(g, obj, outer, 99)

			identity := make([]int, n)
			for i := range identity {
				identity[i] = i
			}
			want := unionIntents(n, intents, identity)

			rng := rand.New(rand.NewSource(1))
			for range 8 {
				order := rng.Perm(n)
				got := unionIntents(n, intents, order)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("%s %T: union order %v changed the result\n want %v\n got  %v",
						fg.name, obj, order, want, got)
				}
			}

			// The full operator is deterministic across repeated calls.
			first := refine(g, obj, outer, 99)
			for run := 1; run < 4; run++ {
				if got := refine(g, obj, outer, 99); !reflect.DeepEqual(got, first) {
					t.Fatalf("%s %T: refine run %d differs from run 0", fg.name, obj, run)
				}
			}
			if !reflect.DeepEqual(first, want) {
				t.Fatalf("%s %T: refine disagrees with identity-order union", fg.name, obj)
			}
		}
	}
}
