package meso

import (
	"maps"
	"math"
	"testing"
)

// iterationsFixture builds the mes-niic n=4 directed fixture with node
// registration order 0,1,2,3. The order is load-bearing: the builder interns
// keys in first-seen order, and under this labelling a single Leiden pass
// converges to the all-in-one partition (Q=0) although the split {0,1},{2,3}
// scores 1/27 (docs/research/directed-modularity-triage.md section 6).
func iterationsFixture(t *testing.T) *Graph {
	t.Helper()
	b := NewDirectedBuilder()
	for _, k := range []string{"0", "1", "2", "3"} {
		b.AddNodeWeight(k, 1)
	}
	b.AddEdge("0", "1", 1)
	b.AddEdge("0", "3", 1)
	b.AddEdge("1", "0", 3)
	b.AddEdge("1", "2", 1)
	b.AddEdge("1", "3", 5)
	b.AddEdge("2", "0", 2)
	b.AddEdge("3", "2", 5)
	g, err := b.Build()
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	return g
}

// TestLeiden_IterationsCharacterization pins the mes-niic finding on the n=4
// fixture at seed 0. A single pass locks into the all-in-one partition:
// phase 1 settles in the {0,2},{1,3} basin, refinement cannot split
// internally cohesive pairs, aggregation freezes that granularity, and the
// aggregate level merges to all-in-one. Extra passes re-refine the converged
// community at node granularity with fresh randomness and escape to the
// optimal split {0,1},{2,3} (Q = 1/27); quality is non-decreasing in the pass
// count because every pass starts from the previous partition and applies
// only improving steps.
func TestLeiden_IterationsCharacterization(t *testing.T) {
	g := iterationsFixture(t)
	quality := WithQuality(DirectedModularity(1))

	groups := func(r *Result) map[int][]string {
		byComm := make(map[int][]string)
		for k, c := range r.Communities() {
			byComm[c] = append(byComm[c], k)
		}
		return byComm
	}

	single, err := Leiden(g, quality)
	if err != nil {
		t.Fatalf("Leiden: %v", err)
	}
	if n := len(groups(single)); n != 1 {
		t.Fatalf("single pass: want the stuck all-in-one partition, got %d communities %v",
			n, single.Communities())
	}
	if q := single.Quality(); math.Abs(q) > 1e-12 {
		t.Errorf("single pass: want Q=0, got %g", q)
	}

	// 8 is the minimal escaping pass count at seed 0, a pinned artifact of the
	// per-pass seed derivation (passSeed); re-pin if that derivation changes.
	// Escape is probabilistic per pass (~13.5% on this fixture), never
	// guaranteed at any fixed count.
	const escaped = 8
	multi, err := Leiden(g, quality, WithIterations(escaped))
	if err != nil {
		t.Fatalf("Leiden(WithIterations(%d)): %v", escaped, err)
	}
	comm := multi.Communities()
	if comm["0"] != comm["1"] || comm["2"] != comm["3"] || comm["0"] == comm["2"] {
		t.Errorf("%d passes: want split {0,1},{2,3}, got %v", escaped, comm)
	}
	if q, want := multi.Quality(), 1.0/27; math.Abs(q-want) > 1e-12 {
		t.Errorf("%d passes: want Q=1/27=%g, got %g", escaped, want, q)
	}

	prev := single.Quality()
	for k := 2; k <= 2*escaped; k++ {
		r, err := Leiden(g, quality, WithIterations(k))
		if err != nil {
			t.Fatalf("Leiden(WithIterations(%d)): %v", k, err)
		}
		if r.Quality() < prev {
			t.Errorf("quality decreased from %g to %g at %d passes", prev, r.Quality(), k)
		}
		prev = r.Quality()
	}
}

// TestLeiden_IterationsDeterministic: a multi-pass run is a pure function of
// (graph, options, seed): repeated serial runs agree, and parallel runs agree
// with each other across worker counts.
func TestLeiden_IterationsDeterministic(t *testing.T) {
	g := iterationsFixture(t)
	opts := []Option{WithQuality(DirectedModularity(1)), WithSeed(41), WithIterations(5)}

	run := func(extra ...Option) map[string]int {
		r, err := Leiden(g, append(append([]Option{}, opts...), extra...)...)
		if err != nil {
			t.Fatalf("Leiden: %v", err)
		}
		return r.Communities()
	}

	serial := run()
	for i := range 3 {
		if got := run(); !maps.Equal(got, serial) {
			t.Fatalf("serial run %d differs: %v vs %v", i, got, serial)
		}
	}

	parBase := run(WithParallelism(1))
	for _, w := range []int{2, 4, 8} {
		if got := run(WithParallelism(w)); !maps.Equal(got, parBase) {
			t.Fatalf("parallel run with %d workers differs: %v vs %v", w, got, parBase)
		}
	}
}

// TestLouvain_IgnoresIterations: Louvain has no refinement randomness to
// re-draw, so extra passes change nothing; the option is accepted and ignored,
// matching WithSeed's contract.
func TestLouvain_IgnoresIterations(t *testing.T) {
	g := iterationsFixture(t)
	quality := WithQuality(DirectedModularity(1))

	plain, err := Louvain(g, quality)
	if err != nil {
		t.Fatalf("Louvain: %v", err)
	}
	iterated, err := Louvain(g, quality, WithIterations(5))
	if err != nil {
		t.Fatalf("Louvain(WithIterations(5)): %v", err)
	}
	if !maps.Equal(plain.Communities(), iterated.Communities()) {
		t.Errorf("Louvain result changed under WithIterations: %v vs %v",
			plain.Communities(), iterated.Communities())
	}
}

// TestLeiden_IterationsValidation: WithIterations requires a pass count of at
// least 1; zero or negative counts are configuration errors, and the default
// (option absent) is valid.
func TestLeiden_IterationsValidation(t *testing.T) {
	g := iterationsFixture(t)
	quality := WithQuality(DirectedModularity(1))

	for _, k := range []int{0, -3} {
		if _, err := Leiden(g, quality, WithIterations(k)); err == nil {
			t.Errorf("Leiden(WithIterations(%d)): want error, got nil", k)
		}
	}
	if _, err := Leiden(g, quality, WithIterations(1)); err != nil {
		t.Errorf("Leiden(WithIterations(1)): unexpected error: %v", err)
	}
}
