package meso

import (
	"math"
	"math/rand"
	"sort"
	"testing"
)

// aggTol is the absolute tolerance for the aggregation-invariance identities.
// Aggregating and scoring the singleton partition reproduces the original score
// exactly in exact arithmetic (Lean modularity_aggregate_eq / cpm_aggregate_eq);
// float64 summation in a different order agrees far inside this bound.
const aggTol = 1e-9

// aggFixtureCSR is a hand-worked 4-node graph used to pin the aggregate's exact
// numbers. Communities {0,1} and {2,3} (labelled 5 and 7 to exercise the relabel)
// with non-unit node sizes, an internal self-loop, internal edges, and one cross
// edge:
//
//	edge 0-1 weight 2   (internal to comm 5)
//	edge 2-3 weight 4   (internal to comm 7)
//	edge 1-2 weight 1   (cross 5-7)
//	self-loop on 0 weight 3
//	node sizes: 0->2, 1->1, 2->3, 3->1
func aggFixtureCSR() *csr {
	return newCSR(
		[]int{0, 1, 3, 5, 6},
		[]int{1, 0, 2, 1, 3, 2},
		[]float64{2, 2, 1, 1, 4, 4},
		[]float64{3, 0, 0, 0},
		[]float64{2, 1, 3, 1},
	)
}

// aggFixturePartition labels aggFixtureCSR's two communities 5 and 7.
func aggFixturePartition() Partition { return Partition{5, 5, 7, 7} }

// singletonPartition is the aggregate's own partition: each of k super-nodes in
// its own community, the Go image of the Lean singleton (fun A => (A : ℕ)).
func singletonPartition(k int) Partition {
	p := make(Partition, k)
	for i := range p {
		p[i] = i
	}
	return p
}

// communityLabels returns the distinct community labels of p, sorted ascending.
func communityLabels(p Partition) []int {
	seen := make(map[int]struct{}, len(p))
	for _, c := range p {
		seen[c] = struct{}{}
	}
	labels := make([]int, 0, len(seen))
	for c := range seen {
		labels = append(labels, c)
	}
	sort.Ints(labels)
	return labels
}

// TestAggregate_TwoMAndDenseLabels checks the two structural invariants of a
// level boundary: total edge weight survives (aggregate twoM == original twoM,
// Lean aggregate_twoM) and communities relabel to dense indices [0, k).
func TestAggregate_TwoMAndDenseLabels(t *testing.T) {
	g := aggFixtureCSR()
	p := aggFixturePartition()

	agg, superOf := aggregate(g, p)

	if err := agg.checkInvariants(); err != nil {
		t.Fatalf("aggregate produced malformed csr: %v", err)
	}

	k := len(communityLabels(p))
	if agg.numNodes() != k {
		t.Errorf("aggregate numNodes() = %d, want %d", agg.numNodes(), k)
	}

	if got, want := agg.twoM(), g.twoM(); math.Abs(got-want) > aggTol {
		t.Errorf("aggregate twoM() = %v, want %v (preserved)", got, want)
	}

	// superOf maps onto exactly [0, k) and respects the partition's equivalence.
	for i := range p {
		if superOf[i] < 0 || superOf[i] >= k {
			t.Fatalf("superOf[%d] = %d out of range [0, %d)", i, superOf[i], k)
		}
		for j := range p {
			if (superOf[i] == superOf[j]) != (p[i] == p[j]) {
				t.Errorf("superOf disagrees with p at (%d,%d): superOf %d,%d vs p %d,%d",
					i, j, superOf[i], superOf[j], p[i], p[j])
			}
		}
	}
	seen := make(map[int]bool)
	for _, s := range superOf {
		seen[s] = true
	}
	for d := range k {
		if !seen[d] {
			t.Errorf("dense label %d never assigned; relabel is not onto [0, %d)", d, k)
		}
	}
}

// TestAggregate_SelfLoopAndSizesPreserved pins the correctness-critical folding
// on aggFixtureCSR by hand: a community's internal weight (its members' own
// self-loops plus twice each internal off-diagonal edge) lands on the super-node
// self-loop, node sizes sum, and the cross weight becomes the aggregate edge.
func TestAggregate_SelfLoopAndSizesPreserved(t *testing.T) {
	agg, superOf := aggregate(aggFixtureCSR(), aggFixturePartition())

	// Labels 5,7 relabel to dense 0,1 in ascending order.
	if superOf[0] != 0 || superOf[1] != 0 || superOf[2] != 1 || superOf[3] != 1 {
		t.Fatalf("superOf = %v, want [0 0 1 1]", superOf)
	}

	// Node sizes sum: comm 0 = 2+1 = 3, comm 1 = 3+1 = 4.
	if got := agg.nodeSize(0); got != 3 {
		t.Errorf("super-node 0 size = %v, want 3", got)
	}
	if got := agg.nodeSize(1); got != 4 {
		t.Errorf("super-node 1 size = %v, want 4", got)
	}

	// Internal weight on the self-loop: comm 0 = self-loop 3 + 2*edge(0-1)=4 -> 7;
	// comm 1 = 2*edge(2-3)=8.
	if got := agg.selfLoops[0]; got != 7 {
		t.Errorf("super-node 0 self-loop = %v, want 7", got)
	}
	if got := agg.selfLoops[1]; got != 8 {
		t.Errorf("super-node 1 self-loop = %v, want 8", got)
	}

	// The single cross edge 1-2 of weight 1 becomes the symmetric aggregate edge.
	if got := agg.weight(0, 1); got != 1 {
		t.Errorf("aggregate edge weight(0,1) = %v, want 1", got)
	}
	if got := agg.weight(1, 0); got != 1 {
		t.Errorf("aggregate edge weight(1,0) = %v, want 1", got)
	}

	// Degrees are the communities' total degrees (Lean aggregate_degree).
	if got := agg.degree(0); got != 8 {
		t.Errorf("super-node 0 degree = %v, want 8", got)
	}
	if got := agg.degree(1); got != 9 {
		t.Errorf("super-node 1 degree = %v, want 9", got)
	}
}

// communityDegree returns K_c, the summed weighted degree of community c's
// members, the quantity squared in the modularity resolution term.
func communityDegree(g *csr, p Partition, c int) float64 {
	sum := 0.0
	for i := 0; i < g.numNodes(); i++ {
		if p[i] == c {
			sum += g.degree(i)
		}
	}
	return sum
}

// TestAggregate_ModularityCommunitySum is the Go image of Lean
// modularity_eq_communitySum: modularity equals the sum over communities of the
// within-community block contribution e_c - gamma*K_c^2/2m, all over 2m. This is
// the decomposition the aggregate self-loop realises, so it must hold before the
// graph-to-graph invariance can.
func TestAggregate_ModularityCommunitySum(t *testing.T) {
	const trials = 3000
	base := rand.New(rand.NewSource(7))
	for range trials {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))

		g := randomCSR(rng)
		p := randomPartition(rng, g.numNodes())
		gamma := 0.3 + rng.Float64()*1.5
		m := modularity{gamma: gamma}

		twoM := g.twoM()
		var want float64
		if twoM != 0 {
			sum := 0.0
			for _, c := range communityLabels(p) {
				ec := communityInternalWeight(g, p, c)
				kc := communityDegree(g, p, c)
				sum += ec - gamma*kc*kc/twoM
			}
			want = sum / twoM
		}

		if got := m.Quality(g, p); math.Abs(got-want) > aggTol {
			t.Fatalf("seed %d: Quality %v != community-sum form %v (diff %g)",
				seed, got, want, math.Abs(got-want))
		}
	}
}

// TestAggregate_QualityPreserved is the Go image of Lean modularity_aggregate_eq:
// scoring the aggregate's singleton partition (each super-node alone) equals
// scoring p on the original graph, so a level neither raises nor lowers
// modularity. Checked on the hand fixture and over random graphs.
func TestAggregate_QualityPreserved(t *testing.T) {
	check := func(t *testing.T, g *csr, p Partition, gamma float64) {
		t.Helper()
		m := modularity{gamma: gamma}
		agg, _ := aggregate(g, p)
		got := m.Quality(agg, singletonPartition(agg.numNodes()))
		want := m.Quality(g, p)
		if math.Abs(got-want) > aggTol {
			t.Fatalf("modularity(aggregate, singleton) = %v, want modularity(g, p) = %v (diff %g)",
				got, want, math.Abs(got-want))
		}
	}

	t.Run("fixture", func(t *testing.T) {
		check(t, aggFixtureCSR(), aggFixturePartition(), 1.0)
	})

	t.Run("random", func(t *testing.T) {
		const trials = 3000
		base := rand.New(rand.NewSource(11))
		for range trials {
			seed := base.Int63()
			rng := rand.New(rand.NewSource(seed))
			g := randomCSR(rng)
			p := randomPartition(rng, g.numNodes())
			gamma := 0.3 + rng.Float64()*1.5
			m := modularity{gamma: gamma}
			agg, _ := aggregate(g, p)
			got := m.Quality(agg, singletonPartition(agg.numNodes()))
			want := m.Quality(g, p)
			if math.Abs(got-want) > aggTol {
				t.Fatalf("seed %d: modularity(aggregate, singleton) %v != modularity(g, p) %v (diff %g)",
					seed, got, want, math.Abs(got-want))
			}
		}
	})
}

// TestAggregate_CPMPreserved is the Go image of Lean cpm_aggregate_eq: the CPM
// twin of TestAggregate_QualityPreserved. The super-node self-loop carries e_c
// and its summed size carries S_c, so the aggregate's diagonal term
// e_c - gamma*S_c^2 reproduces each community contribution exactly.
func TestAggregate_CPMPreserved(t *testing.T) {
	check := func(t *testing.T, g *csr, p Partition, gamma float64) {
		t.Helper()
		m := cpm{gamma: gamma}
		agg, _ := aggregate(g, p)
		got := m.Quality(agg, singletonPartition(agg.numNodes()))
		want := m.Quality(g, p)
		if math.Abs(got-want) > aggTol {
			t.Fatalf("cpm(aggregate, singleton) = %v, want cpm(g, p) = %v (diff %g)",
				got, want, math.Abs(got-want))
		}
	}

	t.Run("fixture", func(t *testing.T) {
		check(t, aggFixtureCSR(), aggFixturePartition(), 0.5)
	})

	t.Run("random", func(t *testing.T) {
		const trials = 3000
		base := rand.New(rand.NewSource(13))
		for range trials {
			seed := base.Int63()
			rng := rand.New(rand.NewSource(seed))
			g := randomCSR(rng)
			p := randomPartition(rng, g.numNodes())
			gamma := 0.3 + rng.Float64()*1.5
			m := cpm{gamma: gamma}
			agg, _ := aggregate(g, p)
			got := m.Quality(agg, singletonPartition(agg.numNodes()))
			want := m.Quality(g, p)
			if math.Abs(got-want) > aggTol {
				t.Fatalf("seed %d: cpm(aggregate, singleton) %v != cpm(g, p) %v (diff %g)",
					seed, got, want, math.Abs(got-want))
			}
		}
	})
}

// TestAggregate_RoundTrip checks that a partition found on the aggregate lifts
// back to the base graph consistently: expanding the aggregate singleton recovers
// p's community structure, and, more generally, scoring any aggregate partition
// equals scoring its expansion on the base graph (the level-composition property
// Louvain and Leiden rely on to keep recursing).
func TestAggregate_RoundTrip(t *testing.T) {
	// Expanding the singleton partition recovers p's equivalence classes.
	g := aggFixtureCSR()
	p := aggFixturePartition()
	agg, superOf := aggregate(g, p)
	lifted := expand(superOf, singletonPartition(agg.numNodes()))
	for i := range p {
		for j := range p {
			if (lifted[i] == lifted[j]) != (p[i] == p[j]) {
				t.Fatalf("expanded singleton disagrees with p at (%d,%d)", i, j)
			}
		}
	}

	// Scoring an aggregate partition equals scoring its base expansion.
	const trials = 3000
	base := rand.New(rand.NewSource(17))
	for range trials {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))
		g := randomCSR(rng)
		p := randomPartition(rng, g.numNodes())
		gamma := 0.3 + rng.Float64()*1.5
		agg, superOf := aggregate(g, p)
		aggP := randomPartition(rng, agg.numNodes())
		lifted := expand(superOf, aggP)

		for _, qf := range []QualityFunction{modularity{gamma: gamma}, cpm{gamma: gamma}} {
			got := qf.Quality(agg, aggP)
			want := qf.Quality(g, lifted)
			if math.Abs(got-want) > aggTol {
				t.Fatalf("seed %d %T: Quality(agg, aggP) %v != Quality(g, expand) %v (diff %g)",
					seed, qf, got, want, math.Abs(got-want))
			}
		}
	}
}
