package meso

import (
	"errors"
	"math"
	"strconv"
	"testing"
)

// Native fuzz targets for the v0.2.0 structural measures (mes-slkl), sharing
// decodeGraph with the optimiser targets in fuzz_test.go.

// encodeEdges is decodeGraph's inverse for seed corpora: node count n (n <= 24)
// and unit-ish edges as (from, to) pairs.
func encodeEdges(n int, pairs ...[2]int) []byte {
	data := []byte{byte(n - 1)}
	for _, p := range pairs {
		data = append(data, byte(p[0]), byte(p[1]), 0)
	}
	return data
}

// measureSeeds are the closed-form fixtures of the Betweenness tests: a star,
// a path, a complete graph and a directed cycle (read undirected, a cycle).
func measureSeeds() [][]byte {
	var star, path, complete, cycle [][2]int
	for i := 1; i < 6; i++ {
		star = append(star, [2]int{0, i})
	}
	for i := range 6 {
		path = append(path, [2]int{i, i + 1})
		cycle = append(cycle, [2]int{i, (i + 1) % 7})
		for j := i + 1; j < 6; j++ {
			complete = append(complete, [2]int{i, j})
		}
	}
	return [][]byte{
		{},
		{0},
		encodeEdges(6, star...),
		encodeEdges(7, path...),
		encodeEdges(6, complete...),
		encodeEdges(7, cycle...),
		{3, 0, 0, 8, 0, 1, 0x85, 1, 2, 16, 3, 3, 4},
	}
}

// buildCanonicalFuzzGraph builds the decoded graph through a Canonical builder,
// applying ops forwards or in reverse, so two builds differ only in insertion
// order. Node sizes are not applied: no structural measure reads them.
func buildCanonicalFuzzGraph(t *testing.T, n int, ops []genOp, directed, reverse bool) *Graph {
	t.Helper()
	b := NewBuilder()
	if directed {
		b = NewDirectedBuilder()
	}
	b.Canonical()
	for i := range n {
		b.AddNodeWeight(strconv.Itoa(i), 1.0)
	}
	for k := range ops {
		op := ops[k]
		if reverse {
			op = ops[len(ops)-1-k]
		}
		if !op.node {
			b.AddEdge(strconv.Itoa(op.from), strconv.Itoa(op.to), op.w)
		}
	}
	return mustBuild(t, b)
}

// FuzzBetweenness: over arbitrary decoded graphs, Betweenness never panics,
// every value lies in [0, 1], the result is bit-identical across insertion
// order and repeated calls, and on small graphs it equals the definitional
// brute force.
func FuzzBetweenness(f *testing.F) {
	for _, s := range measureSeeds() {
		f.Add(s, false)
		f.Add(s, true)
	}
	f.Fuzz(func(t *testing.T, data []byte, directed bool) {
		n, ops := decodeGraph(data)
		fwd := buildCanonicalFuzzGraph(t, n, ops, directed, false)
		rev := buildCanonicalFuzzGraph(t, n, ops, directed, true)
		got, other := Betweenness(fwd), Betweenness(rev)
		if len(got) != n {
			t.Fatalf("%d values for %d nodes", len(got), n)
		}
		for k, v := range got {
			if !(v >= 0 && v <= 1) {
				t.Fatalf("node %s = %v outside [0, 1]", k, v)
			}
			if math.Float64bits(v) != math.Float64bits(other[k]) {
				t.Fatalf("node %s: %v forwards, %v reversed", k, v, other[k])
			}
		}
		if n > 10 {
			return
		}
		for k, w := range bruteBetweenness(fwd) {
			if !approxEqual(got[k], w) {
				t.Fatalf("node %s = %v, brute force %v", k, got[k], w)
			}
		}
	})
}

// FuzzSubgraph: for an arbitrary decoded graph and a member set drawn from
// mask, Subgraph is the induced subgraph (assertInduced) and satisfies the CSR
// invariants; with mode 1 an unknown key, and with mode 2 a repeated member,
// is reported through the documented sentinel.
func FuzzSubgraph(f *testing.F) {
	for _, s := range measureSeeds() {
		for _, mode := range []byte{0, 1, 2} {
			f.Add(s, false, uint32(0b1010110), mode)
			f.Add(s, true, uint32(0xffffffff), mode)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte, directed bool, mask uint32, mode byte) {
		n, ops := decodeGraph(data)
		g := buildFuzzGraph(t, n, ops, directed)
		var keys []string
		for i := range n {
			if mask&(1<<(i%32)) != 0 {
				keys = append(keys, strconv.Itoa(i))
			}
		}
		switch mode % 3 {
		case 1:
			_, err := Subgraph(g, append(keys, "absent"))
			if !errors.Is(err, ErrUnknownKey) {
				t.Fatalf("unknown key: error = %v, want ErrUnknownKey", err)
			}
			return
		case 2:
			if len(keys) == 0 {
				return
			}
			_, err := Subgraph(g, append(keys, keys[0]))
			if !errors.Is(err, ErrDuplicateKey) {
				t.Fatalf("duplicate key: error = %v, want ErrDuplicateKey", err)
			}
			return
		}
		sub, err := Subgraph(g, keys)
		if err != nil {
			t.Fatalf("Subgraph() error = %v", err)
		}
		if err := sub.model.checkInvariants(); err != nil {
			t.Fatalf("subgraph violates CSR invariants: %v", err)
		}
		assertInduced(t, g, sub, keys)
	})
}
