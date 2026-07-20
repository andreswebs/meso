package meso

import (
	"math/rand"
	"testing"
)

// TestPRNG_SameSeedSameStream is the determinism substrate's tracer bullet: two
// generators seeded identically must emit identical value streams, so a run's
// randomness is a pure function of its seed (design section 4.4).
func TestPRNG_SameSeedSameStream(t *testing.T) {
	const seed = 0x1234_5678_9abc_def0
	a := newPRNG(seed)
	b := newPRNG(seed)
	for i := range 1000 {
		if x, y := a.next(), b.next(); x != y {
			t.Fatalf("draw %d: a.next() = %d, b.next() = %d, want equal", i, x, y)
		}
	}
}

// TestPRNG_GoldenStream pins the first outputs of the seed-0 stream to the
// canonical splitmix64 reference values (Vigna). It is a portability guard: if
// the generator's constants or step ever drift, every seeded run's output would
// shift silently, breaking cross-machine and cross-Go-version reproducibility
// (design section 4.4). Matching the published reference also documents that
// meso's PRNG is exactly splitmix64, not a bespoke variant.
func TestPRNG_GoldenStream(t *testing.T) {
	want := []uint64{
		0xe220a8397b1dcdaf,
		0x6e789e6aa1b965f4,
		0x06c45d188009454f,
	}
	r := newPRNG(0)
	for i, w := range want {
		if got := r.next(); got != w {
			t.Fatalf("seed-0 draw %d = %#016x, want %#016x", i, got, w)
		}
	}
}

// TestPRNG_InterleavingIndependent asserts AC#3: because each generator owns its
// state (no process-global source), interleaving draws from two same-seed
// generators does not perturb either stream. Whatever the call-site interleaving,
// a generator seeded with S produces the same sequence as a lone generator
// seeded with S. This is what lets the parallel round (M4) draw per-node
// randomness on any core without the schedule affecting the result.
func TestPRNG_InterleavingIndependent(t *testing.T) {
	const seed = 42

	reference := newPRNG(seed)
	want := make([]uint64, 500)
	for i := range want {
		want[i] = reference.next()
	}

	a := newPRNG(seed)
	b := newPRNG(seed)
	// Interleave a's draws with a variable number of unrelated b draws, then
	// confirm a still walks the reference stream untouched.
	for i, w := range want {
		for j := 0; j <= i%3; j++ {
			b.next()
		}
		if got := a.next(); got != w {
			t.Fatalf("draw %d: interleaved a.next() = %d, want %d", i, got, w)
		}
	}
}

// TestNodeSeed_StableAndOrderIndependent asserts AC#4: a node's derived seed is
// a pure function of (globalSeed, nodeID), so it is the same value no matter the
// order nodes are visited. This is what makes refinement (M2) and the
// synchronous parallel round (M4) scheduling-independent - a node draws the same
// choices whichever sweep position or core handles it.
func TestNodeSeed_StableAndOrderIndependent(t *testing.T) {
	const globalSeed = 0xdead_beef_cafe_1234
	const n = 256

	ascending := make([]uint64, n)
	for u := range n {
		ascending[u] = nodeSeed(globalSeed, u)
	}

	// Recompute in a shuffled order; each node must land on the same value.
	order := rand.New(rand.NewSource(1)).Perm(n)
	shuffled := make([]uint64, n)
	for _, u := range order {
		shuffled[u] = nodeSeed(globalSeed, u)
	}
	for u := range n {
		if ascending[u] != shuffled[u] {
			t.Fatalf("node %d: seed depends on visit order (%d vs %d)", u, ascending[u], shuffled[u])
		}
	}

	// Stable across repeated calls.
	for u := range n {
		if got := nodeSeed(globalSeed, u); got != ascending[u] {
			t.Fatalf("node %d: nodeSeed not stable (%d then %d)", u, ascending[u], got)
		}
	}
}

// TestNodeSeed_DistinctAcrossNodesAndSeeds guards against a degenerate mixer:
// distinct nodes under one global seed, and one node under distinct global
// seeds, must not collapse to the same derived seed in the common case.
func TestNodeSeed_DistinctAcrossNodesAndSeeds(t *testing.T) {
	const globalSeed = 7

	seen := make(map[uint64]int, 1024)
	for u := range 1024 {
		s := nodeSeed(globalSeed, u)
		if prev, ok := seen[s]; ok {
			t.Fatalf("nodes %d and %d collide on seed %d", prev, u, s)
		}
		seen[s] = u
	}

	if a, b := nodeSeed(1, 0), nodeSeed(2, 0); a == b {
		t.Fatalf("global seeds 1 and 2 give node 0 the same seed %d", a)
	}
}
