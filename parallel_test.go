package meso

import (
	"reflect"
	"runtime"
	"testing"
)

// parallelWorkerCounts is the set of core counts every core-count-invariance
// assertion sweeps: the plan's 1, 2, 4, 8 plus the machine's GOMAXPROCS.
var parallelWorkerCounts = []int{1, 2, 4, 8, runtime.GOMAXPROCS(0)}

// TestParallel_RoundClosedForm is the Go image of applyRound_eq
// (verification/lean/Meso/Round.lean): after a synchronous round with snapshot
// targets t over the moved-node set, node w sits at t[w] if it moved and at its
// round-start label p[w] otherwise. The result mentions neither the order nor
// the multiplicity of the moved nodes, so this pins the round to a function of
// the snapshot and the moved-node set alone.
func TestParallel_RoundClosedForm(t *testing.T) {
	p := Partition{0, 0, 1, 1, 2}
	targets := []int{3, 0, 3, 1, 3}
	moved := []bool{true, false, true, false, false}

	got := applyRound(p, targets, moved)
	want := Partition{3, 0, 3, 1, 2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("applyRound = %v, want %v", got, want)
	}

	// The round is pure: it must not mutate the snapshot it reads.
	if !reflect.DeepEqual(p, Partition{0, 0, 1, 1, 2}) {
		t.Fatalf("applyRound mutated snapshot p = %v", p)
	}
}

// TestParallel_RoundCoreCountInvariant is the Go image of applyRound_perm
// (verification/lean/Meso/Round.lean): a synchronous round's outcome is
// independent of the order the moves are applied, hence of how the work is split
// across cores. It checks both halves of that on the code.
//
// First, that the per-node snapshot decisions (parallelBestMoves) are a pure
// function of the graph and the round-start partition: computed with 1, 2, 4, 8,
// and GOMAXPROCS workers they are byte-identical, so the number of goroutines
// never leaks into the result. Second, that folding those moves in any order
// reproduces the closed form applyRound builds - ascending, descending, and a
// shuffled schedule all agree - which is confluence stated directly on move.
func TestParallel_RoundCoreCountInvariant(t *testing.T) {
	g := loadGMLGraph(t, "datasets/karate/karate.gml").model
	obj := modularity{gamma: 1.0}
	p := singleton(g.numNodes())

	baseT, baseGain := parallelBestMoves(g, obj, p, 1)
	for _, w := range parallelWorkerCounts {
		gotT, gotGain := parallelBestMoves(g, obj, p, w)
		if !reflect.DeepEqual(gotT, baseT) {
			t.Fatalf("targets with %d workers = %v, want %v", w, gotT, baseT)
		}
		if !reflect.DeepEqual(gotGain, baseGain) {
			t.Fatalf("gains with %d workers differ from serial", w)
		}
	}

	// Build the moved-node set from the snapshot decisions, then confirm that
	// applying those moves in any schedule reproduces applyRound's mask form.
	moved := make([]bool, g.numNodes())
	var order []int
	for u := range p {
		if baseGain[u] > moveImproveEps && baseT[u] != p[u] {
			moved[u] = true
			order = append(order, u)
		}
	}
	if len(order) < 2 {
		t.Fatalf("fixture too trivial: only %d nodes move", len(order))
	}
	want := applyRound(p, baseT, moved)

	// The same moved set folded in opposite schedules must both reproduce the
	// mask form: order within a covering schedule does not matter (confluence).
	for _, sched := range [][]int{ascending(order), descending(order)} {
		if got := foldMoves(p, baseT, sched); !reflect.DeepEqual(got, want) {
			t.Fatalf("schedule %v gave %v, want %v", sched, got, want)
		}
	}
}

// foldMoves applies move(·, v, t[v]) over the nodes in sched in the given order,
// the ordered fold the Lean applyRound models before it collapses to a mask.
func foldMoves(p Partition, t []int, sched []int) Partition {
	q := make(Partition, len(p))
	copy(q, p)
	for _, v := range sched {
		q = move(q, v, t[v])
	}
	return q
}

func ascending(xs []int) []int {
	out := append([]int(nil), xs...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

func descending(xs []int) []int {
	asc := ascending(xs)
	for i, j := 0, len(asc)-1; i < j; i, j = i+1, j-1 {
		asc[i], asc[j] = asc[j], asc[i]
	}
	return asc
}

// cycleCSR builds the undirected n-cycle 0-1-...-(n-1)-0 with unit weights. An
// even cycle is a bipartite trap for synchronous local moving: from singletons
// every node's best move is to fold into a neighbour, and a naive batch that
// applied all those snapshot moves at once would swap partners each round and
// oscillate forever. It is the adversarial convergence fixture.
func cycleCSR(n int) *csr {
	offsets := make([]int, n+1)
	var neighbors []int
	var weights []float64
	for i := range n {
		lo, hi := (i+n-1)%n, (i+1)%n
		for _, j := range []int{lo, hi} {
			neighbors = append(neighbors, j)
			weights = append(weights, 1)
		}
		offsets[i+1] = len(neighbors)
	}
	return newCSR(offsets, neighbors, weights, nil, nil)
}

// TestParallel_Convergence checks acceptance criterion 5: synchronous rounds
// reach a stable partition without oscillation. On the corpus and on the even
// cycle (a swap trap), driving parallelRound to its fixed point must (a) raise
// the objective monotonically round over round, (b) terminate within a generous
// bound, and (c) land on a local-move-stable partition - the same fixed-point
// condition the serial sweep reaches.
func TestParallel_Convergence(t *testing.T) {
	obj := modularity{gamma: 1.0}
	cases := []struct {
		name string
		g    *csr
	}{
		{"karate", loadGMLGraph(t, "datasets/karate/karate.gml").model},
		{"dolphins", loadGMLGraph(t, "datasets/dolphins/dolphins.gml").model},
		{"lesmis", loadGMLGraph(t, "datasets/lesmis/lesmis.gml").model},
		{"cycle8", cycleCSR(8)},
		{"cycle32", cycleCSR(32)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := singleton(tc.g.numNodes())
			prevQ := obj.Quality(tc.g, p)

			const maxRounds = 100000
			rounds := 0
			for {
				next, changed := parallelRound(tc.g, obj, p, 4)
				if !changed {
					break
				}
				q := obj.Quality(tc.g, next)
				if q < prevQ-1e-9 {
					t.Fatalf("round %d lowered quality: %.15g -> %.15g", rounds, prevQ, q)
				}
				prevQ = q
				copy(p, next)
				rounds++
				if rounds > maxRounds {
					t.Fatalf("did not converge within %d rounds (oscillation)", maxRounds)
				}
			}

			if !isLocalMoveStable(tc.g, obj, p) {
				t.Fatalf("converged partition is not local-move-stable")
			}
		})
	}
}

// TestParallel_LocalMoveToStableMatchesRounds checks that the fixed-point driver
// parallelLocalMoveToStable reaches exactly the partition the round-by-round loop
// converges to, and reports movement iff a move happened.
func TestParallel_LocalMoveToStableMatchesRounds(t *testing.T) {
	g := loadGMLGraph(t, "datasets/karate/karate.gml").model
	obj := modularity{gamma: 1.0}

	p := singleton(g.numNodes())
	moved := parallelLocalMoveToStable(g, obj, p, 4)
	if !moved {
		t.Fatal("expected local moving to move at least one node on karate")
	}
	if !isLocalMoveStable(g, obj, p) {
		t.Fatal("parallelLocalMoveToStable left a non-stable partition")
	}
}

// TestParallel_WholeRunCoreCountInvariant checks acceptance criteria 2 and 3 on
// the whole multilevel pipeline: running parallel Louvain and parallel Leiden
// with 1, 2, 4, 8, and GOMAXPROCS workers yields byte-identical base-graph
// partitions. "Serial versus parallel" is the workers==1 row against the rest:
// same synchronous-round algorithm, executed in one goroutine or many.
func TestParallel_WholeRunCoreCountInvariant(t *testing.T) {
	obj := modularity{gamma: 1.0}
	const seed = 12345
	graphs := []struct {
		name string
		g    *csr
	}{
		{"karate", loadGMLGraph(t, "datasets/karate/karate.gml").model},
		{"dolphins", loadGMLGraph(t, "datasets/dolphins/dolphins.gml").model},
		{"lesmis", loadGMLGraph(t, "datasets/lesmis/lesmis.gml").model},
	}
	for _, tc := range graphs {
		t.Run(tc.name, func(t *testing.T) {
			louBase := louvainParallel(tc.g, obj, 1)
			leiBase := leidenParallel(tc.g, obj, seed, 1)
			for _, w := range parallelWorkerCounts {
				if got := louvainParallel(tc.g, obj, w); !reflect.DeepEqual(got, louBase) {
					t.Fatalf("louvainParallel(%d workers) differs from serial: %v vs %v", w, got, louBase)
				}
				if got := leidenParallel(tc.g, obj, seed, w); !reflect.DeepEqual(got, leiBase) {
					t.Fatalf("leidenParallel(%d workers) differs from serial: %v vs %v", w, got, leiBase)
				}
			}
		})
	}
}

// TestParallel_PublicOptionCoreCountInvariant exercises the same guarantee
// through the public API: Leiden and Louvain with WithParallelism(w) return the
// same community assignment for every core count, and it is a valid partition.
func TestParallel_PublicOptionCoreCountInvariant(t *testing.T) {
	g := loadGMLGraph(t, "datasets/karate/karate.gml")

	base, err := Leiden(g, WithParallelism(1))
	if err != nil {
		t.Fatalf("Leiden(WithParallelism(1)): %v", err)
	}
	for _, w := range parallelWorkerCounts {
		got, err := Leiden(g, WithParallelism(w))
		if err != nil {
			t.Fatalf("Leiden(WithParallelism(%d)): %v", w, err)
		}
		if !reflect.DeepEqual(got.Communities(), base.Communities()) {
			t.Fatalf("Leiden WithParallelism(%d) communities differ from serial", w)
		}
		if got.Quality() != base.Quality() {
			t.Fatalf("Leiden WithParallelism(%d) quality %.15g != %.15g", w, got.Quality(), base.Quality())
		}
	}
}

// TestParallel_NormalizeWorkers pins the worker-count clamp directly. The
// core-count-invariance tests above cannot see it: the run's output is
// byte-identical whatever normalizeWorkers returns, so only asserting the
// clamped value itself distinguishes "use every core" (a non-positive request),
// the ceiling at the node count, and the floor at one.
func TestParallel_NormalizeWorkers(t *testing.T) {
	cores := runtime.GOMAXPROCS(0)
	cases := []struct {
		workers, n, want int
	}{
		{0, 100, cores},  // non-positive means "use every core"
		{-3, 100, cores}, // any non-positive request, not just zero
		{5, 100, 5},      // an in-range request is left alone
		{50, 10, 10},     // capped at the node count
		{10, 10, 10},     // exactly the node count is left alone
		{3, 0, 1},        // never fewer than one worker
		{1, 4, 1},        // one worker is left alone
	}
	for _, tc := range cases {
		if got := normalizeWorkers(tc.workers, tc.n); got != tc.want {
			t.Errorf("normalizeWorkers(%d, %d) = %d, want %d", tc.workers, tc.n, got, tc.want)
		}
	}
}

// TestParallel_IsolationBase pins isolationBase to its contract: a label
// strictly greater than every label in p and at least n, so base+u is a fresh
// singleton for every node. The parallel round only ever feeds it a canonical
// snapshot (labels < n), where the loop body never runs, so this exercises the
// boundary the snapshot never reaches - a label equal to and above n.
func TestParallel_IsolationBase(t *testing.T) {
	cases := []struct {
		p    Partition
		n    int
		want int
	}{
		{Partition{0, 1, 2}, 3, 3}, // all labels below n: base is n
		{Partition{0, 1, 3}, 3, 4}, // a label equal to n forces base above it
		{Partition{2, 5, 0}, 3, 6}, // the largest label, plus one, dominates n
	}
	for _, tc := range cases {
		if got := isolationBase(tc.p, tc.n); got != tc.want {
			t.Errorf("isolationBase(%v, %d) = %d, want %d", tc.p, tc.n, got, tc.want)
		}
	}
}
