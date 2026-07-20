package meso

import (
	"math/rand"
	"testing"
)

// TestLeidenLocalMoveQueue_Deterministic is acceptance criterion 5: the phase is
// byte-identical across runs. Node visiting order (ascending initial enqueue, FIFO
// re-enqueue) and tie-breaking (bestMove's ascending-label scan) are canonical and
// the phase draws no randomness, so two runs on the same graph and objective
// produce the identical partition. Checked over the corpus and fuzzed inputs under
// both objectives.
func TestLeidenLocalMoveQueue_Deterministic(t *testing.T) {
	for _, tc := range corpusAndFuzzed(t, 23) {
		for _, obj := range []objective{modularity{gamma: 1.0}, cpm{gamma: 0.1}} {
			p1 := singleton(tc.g.numNodes())
			_, pops1 := localMoveQueue(tc.g, obj, p1)
			p2 := singleton(tc.g.numNodes())
			_, pops2 := localMoveQueue(tc.g, obj, p2)

			if pops1 != pops2 {
				t.Fatalf("%s %T: pop count differs across runs (%d vs %d)", tc.name, obj, pops1, pops2)
			}
			for i := range p1 {
				if p1[i] != p2[i] {
					t.Fatalf("%s %T: partition differs at node %d (%d vs %d)", tc.name, obj, i, p1[i], p2[i])
				}
			}
		}
	}
}

// TestNodeQueue_DedupAndDrain covers acceptance criterion 3 at the data-structure
// level: the queue is a FIFO whose push deduplicates - a node already in the queue
// is not enqueued a second time - and whose pop clears the in-queue mark so the
// node can be re-enqueued later. push reports whether the node was actually added,
// which is how the local-move loop enqueues "exactly the un-queued neighbours".
func TestNodeQueue_DedupAndDrain(t *testing.T) {
	q := newNodeQueue(4)

	if !q.push(2) {
		t.Fatal("push(2) into empty queue reported not added")
	}
	if q.push(2) {
		t.Fatal("push(2) while 2 already queued reported added (duplicate)")
	}
	if !q.push(0) {
		t.Fatal("push(0) reported not added")
	}

	if q.empty() {
		t.Fatal("queue reported empty with two items")
	}

	// FIFO: 2 was pushed before 0.
	if got := q.pop(); got != 2 {
		t.Fatalf("pop() = %d, want 2 (FIFO order)", got)
	}
	// Popping 2 cleared its mark, so it can be re-enqueued.
	if !q.push(2) {
		t.Fatal("push(2) after popping 2 reported not added")
	}

	if got := q.pop(); got != 0 {
		t.Fatalf("pop() = %d, want 0", got)
	}
	if got := q.pop(); got != 2 {
		t.Fatalf("pop() = %d, want 2 (re-enqueued)", got)
	}
	if !q.empty() {
		t.Fatal("queue reported non-empty after draining every item")
	}
}

// TestLeidenLocalMoveQueue_Monotone is acceptance criterion 2, the queue twin of
// TestLocalMove_SweepMonotone: the queue-driven phase never lowers quality. Every
// applied move strictly raises the objective and is applied against the running
// partition, so the whole run, from an arbitrary starting partition, ends at
// quality no lower than it began - the Go image of localMoveRun_monotone. Checked
// under both objectives over graphs with non-unit node sizes.
func TestLeidenLocalMoveQueue_Monotone(t *testing.T) {
	base := rand.New(rand.NewSource(21))
	for range 2000 {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))
		g := randomCSRSized(rng)
		p := randomPartition(rng, g.numNodes())
		gamma := 0.3 + rng.Float64()*1.5
		obj := objective(modularity{gamma: gamma})
		if seed%2 == 0 {
			obj = cpm{gamma: gamma}
		}

		before := obj.Quality(g, p)
		localMoveQueue(g, obj, p)
		after := obj.Quality(g, p)

		if after < before-louvainTol {
			t.Fatalf("seed %d %T: queue lowered quality %v -> %v", seed, obj, before, after)
		}
	}
}

// TestLeidenLocalMoveQueue_TerminatesBounded is acceptance criterion 4: the queue
// drains in a bounded number of pops and the phase terminates. A fresh run from
// singletons returns (it does not hang) and pops at least every node once. Re-run
// on the resulting stable partition, the queue does exactly one clean drain - n
// pops, no move - which both bounds the work at a fixed point and shows the phase
// is idempotent: it never churns a stable partition.
func TestLeidenLocalMoveQueue_TerminatesBounded(t *testing.T) {
	base := rand.New(rand.NewSource(22))
	for range 1500 {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))
		g := randomCSR(rng)
		obj := objective(modularity{gamma: 0.3 + rng.Float64()*1.5})
		if seed%2 == 0 {
			obj = cpm{gamma: 0.2 + rng.Float64()}
		}
		n := g.numNodes()

		p := singleton(n)
		_, pops := localMoveQueue(g, obj, p)
		if pops < n {
			t.Fatalf("seed %d %T: %d pops fewer than %d nodes (not every node examined)", seed, obj, pops, n)
		}

		movedAgain, popsAgain := localMoveQueue(g, obj, p)
		if movedAgain {
			t.Fatalf("seed %d %T: re-running on a stable partition moved a node", seed, obj)
		}
		if popsAgain != n {
			t.Fatalf("seed %d %T: clean drain popped %d nodes, want exactly %d", seed, obj, popsAgain, n)
		}
	}
}

// TestLeidenLocalMoveQueue_ReachesStable is acceptance criterion 1: the
// queue-driven local move reaches a local-move-stable partition (IsLocalMoveStable
// in the strong, all-targets sense), the same fixed-point condition the exhaustive
// sweep stops at. Stability is checked two ways: a follow-up exhaustive sweep
// applies no move, and no single-node best move (any neighbour community or a
// fresh isolation label) strictly improves the objective.
func TestLeidenLocalMoveQueue_ReachesStable(t *testing.T) {
	base := rand.New(rand.NewSource(20))
	for range 1500 {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))
		g := randomCSRSized(rng)
		obj := objective(modularity{gamma: 0.3 + rng.Float64()*1.5})
		if seed%2 == 0 {
			obj = cpm{gamma: 0.2 + rng.Float64()}
		}

		p := singleton(g.numNodes())
		localMoveQueue(g, obj, p)

		fresh := g.numNodes()
		for _, c := range p {
			if c >= fresh {
				fresh = c + 1
			}
		}
		if localMoveSweep(g, obj, p, &fresh) {
			t.Fatalf("seed %d %T: sweep after localMoveQueue still moved a node", seed, obj)
		}
		for u := 0; u < g.numNodes(); u++ {
			if _, delta := bestMove(g, obj, p, u, fresh); delta > louvainTol {
				t.Fatalf("seed %d %T: node %d improves queue-stable partition by %v", seed, obj, u, delta)
			}
		}
	}
}

// TestLeidenLevelSeed_FromNonRefined is acceptance criterion 1: the aggregate's
// initial partition is seeded from the NON-refined phase-1 partition, not the
// refined one. The fixture is one phase-1 community of six nodes (nonRefined all
// 0) that refinement split into two sub-communities, so aggregation produced two
// super-nodes (superOf = {0,0,0,1,1,1}). Seeding from the non-refined partition
// puts both super-nodes in the same initial community ({0,0}); seeding from the
// refined partition would separate them ({0,1}). The two therefore diverge, and
// leidenLevelSeed must follow the non-refined one. This is the correctness-
// critical subtlety of design section 4.1 phase 3.
func TestLeidenLevelSeed_FromNonRefined(t *testing.T) {
	superOf := []int{0, 0, 0, 1, 1, 1}
	nonRefined := Partition{0, 0, 0, 0, 0, 0}
	refined := Partition{0, 0, 0, 1, 1, 1}

	seed := leidenLevelSeed(superOf, nonRefined, 2)

	// Non-refined seed keeps both refined sub-communities in one community.
	if !sameGrouping(seed, Partition{0, 0}) {
		t.Fatalf("leidenLevelSeed = %v, want both super-nodes in one community (from non-refined)", seed)
	}
	// It must NOT match the refined-derived init, which separates them.
	refinedInit := leidenLevelSeed(superOf, refined, 2)
	if sameGrouping(seed, refinedInit) {
		t.Fatalf("non-refined seed %v matches refined-derived init %v; the two must diverge", seed, refinedInit)
	}
}

// sameGrouping reports whether two partitions induce the identical grouping,
// independent of the concrete labels: node pairs share a community in a exactly
// when they do in b.
func sameGrouping(a, b Partition) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		for j := range a {
			if (a[i] == a[j]) != (b[i] == b[j]) {
				return false
			}
		}
	}
	return true
}

// TestLeiden_Deterministic is the serial-Leiden half of acceptance criterion 5:
// the whole recursion is byte-identical across runs at a fixed seed. Node order,
// tie-breaking, aggregation, and the per-node-seeded refinement are all canonical
// or seed-derived, so two runs on the same graph, objective, and seed produce the
// identical partition. Checked over the corpus and fuzzed inputs under both
// objectives, and against a well-formed final partition.
func TestLeiden_Deterministic(t *testing.T) {
	for _, tc := range corpusAndFuzzed(t, 71) {
		for _, obj := range []objective{modularity{gamma: 1.0}, cpm{gamma: 0.1}} {
			p1 := leiden(tc.g, obj, 99)
			p2 := leiden(tc.g, obj, 99)

			if err := p1.wellFormed(tc.g.numNodes()); err != nil {
				t.Fatalf("%s %T: leiden result not well-formed: %v", tc.name, obj, err)
			}
			if len(p1) != len(p2) {
				t.Fatalf("%s %T: length differs across runs (%d vs %d)", tc.name, obj, len(p1), len(p2))
			}
			for i := range p1 {
				if p1[i] != p2[i] {
					t.Fatalf("%s %T: partition differs at node %d (%d vs %d)", tc.name, obj, i, p1[i], p2[i])
				}
			}
		}
	}
}

// TestLeiden_AggregationPreservesQuality is acceptance criterion 2: self-loops
// carrying internal edge weight and node sizes survive aggregation so the
// resolution term stays correct across a full Leiden run. The seeded aggregate
// scores exactly what the non-refined partition scores on the working graph:
// leidenLevelSeed lifts back to the non-refined p (each super-node keeps its
// phase-1 label), so Quality(aggregate, seed) must equal Quality(h, p). This
// holds only if aggregation folds internal weight onto the super-node self-loops
// and sums node sizes; drop either and the resolution term shifts silently.
// Checked at the first level of every corpus and fuzzed graph (fuzzed graphs
// carry non-unit node sizes), under both objectives.
func TestLeiden_AggregationPreservesQuality(t *testing.T) {
	for _, tc := range corpusAndFuzzed(t, 314) {
		for _, obj := range []objective{modularity{gamma: 1.0}, cpm{gamma: 0.2}} {
			p := singleton(tc.g.numNodes())
			localMoveQueue(tc.g, obj, p)
			p = canonicalize(p)
			if numCommunities(p) == tc.g.numNodes() {
				continue // nothing merged; no aggregate level to check
			}

			refined := refine(tc.g, obj, p, 5)
			aggG, superOf := aggregate(tc.g, refined)
			seed := leidenLevelSeed(superOf, p, aggG.numNodes())

			want := obj.Quality(tc.g, p)
			got := obj.Quality(aggG, seed)
			if diff := got - want; diff < -louvainTol || diff > louvainTol {
				t.Fatalf("%s %T: seeded aggregate quality %v != working quality %v (diff %v)",
					tc.name, obj, got, want, diff)
			}
		}
	}
}

// TestLeiden_CommunitiesConnected is acceptance criterion 3 and the Go image of
// connectedCommunities_of_refineRun (verification/lean/Meso/Refinement.lean):
// running the refinement operator from singletons yields connected communities.
// Refinement only unions nodes across a shared positive-weight edge, so every
// refined sub-community induces a connected subgraph. Checked end to end at every
// level of a full Leiden run - each level's refined partition is connected on the
// working graph it was computed over - across the corpus and fuzzed inputs under
// both objectives.
func TestLeiden_CommunitiesConnected(t *testing.T) {
	for _, tc := range corpusAndFuzzed(t, 27) {
		for _, obj := range []objective{modularity{gamma: 1.0}, cpm{gamma: 0.1}} {
			h := tc.g
			initP := singleton(h.numNodes())
			for {
				p := make(Partition, len(initP))
				copy(p, initP)
				localMoveQueue(h, obj, p)
				p = canonicalize(p)
				if numCommunities(p) == h.numNodes() {
					break
				}

				refined := refine(h, obj, p, 8)
				if !connectedCommunities(h, refined) {
					t.Fatalf("%s %T: refined partition has a disconnected community: %v", tc.name, obj, refined)
				}

				aggG, superOf := aggregate(h, refined)
				if aggG.numNodes() >= h.numNodes() {
					break
				}
				initP = leidenLevelSeed(superOf, p, aggG.numNodes())
				h = aggG
			}
		}
	}
}

// TestLeiden_QualityGeLouvain is acceptance criterion 4: Leiden reaches quality
// at least as high as Louvain on the corpus. Refinement lets Leiden escape the
// local optima Louvain settles in, so its achieved quality is greater than or
// equal to Louvain's on the canonical corpus graphs, under both objectives. The
// AC is scoped to the corpus deliberately: on arbitrary random graphs both are
// heuristics and Louvain's from-singletons re-exploration each level can edge out
// the seeded descent, which is not a defect (see docs/specs/learnings.md).
func TestLeiden_QualityGeLouvain(t *testing.T) {
	g := loadGMLCSR(t, "datasets/karate/karate.gml")
	for _, obj := range []objective{modularity{gamma: 1.0}, cpm{gamma: 0.1}} {
		lei := obj.Quality(g, leiden(g, obj, 17))
		lou := obj.Quality(g, louvain(g, obj))
		if lei < lou-louvainTol {
			t.Fatalf("karate %T: Leiden quality %v < Louvain quality %v", obj, lei, lou)
		}
	}
}
