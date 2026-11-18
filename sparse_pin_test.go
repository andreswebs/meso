package meso

import (
	"fmt"
	"hash/fnv"
	"testing"
)

// partitionFingerprint condenses a run's result to "communities:hash", the hash
// taken over every node's community label in node-index order.
func partitionFingerprint(g *Graph, res *Result) string {
	comm := res.Communities()
	h := fnv.New64a()
	for i := range g.NumNodes() {
		_, _ = fmt.Fprintf(h, "%d,", comm[g.Key(i)])
	}
	return fmt.Sprintf("%d:%016x", res.NumCommunities(), h.Sum64())
}

// TestSparsePartitionPinned pins the exact partition of every algorithm,
// execution mode, and objective on a sparse hub-and-components graph, with unit
// and with non-integer weights, so a change meant to preserve output is held to
// byte-identical partitions. The serial fingerprints predate the incremental
// community totals of the local-move phase; the parallel ones follow the
// re-pricing conflict resolution of parallelRound.
func TestSparsePartitionPinned(t *testing.T) {
	const n = 2000
	type run struct {
		name string
		do   func(*Graph, ...Option) (*Result, error)
		opts []Option
	}
	runs := []run{
		{"leiden", Leiden, []Option{WithSeed(7)}},
		{"leiden-parallel", Leiden, []Option{WithSeed(7), WithParallelism(4)}},
		{"louvain", Louvain, nil},
		{"louvain-parallel", Louvain, []Option{WithParallelism(4)}},
	}
	type objCase struct {
		name     string
		q        QualityFunction
		directed bool
	}
	objs := []objCase{
		{"modularity", Modularity(1), false},
		{"cpm", CPM(0.05), false},
		{"directed", DirectedModularity(1), true},
	}

	for _, oc := range objs {
		for _, fractional := range []bool{false, true} {
			g := sparseGraph(n, 11, oc.directed, fractional)
			weights := "unit"
			if fractional {
				weights = "fractional"
			}
			for _, r := range runs {
				name := oc.name + "/" + weights + "/" + r.name
				t.Run(name, func(t *testing.T) {
					opts := append([]Option{WithQuality(oc.q)}, r.opts...)
					res, err := r.do(g, opts...)
					if err != nil {
						t.Fatal(err)
					}
					got := partitionFingerprint(g, res)
					if want := sparsePins[name]; got != want {
						t.Errorf("fingerprint %q, want %q", got, want)
					}
				})
			}
		}
	}
}

var sparsePins = map[string]string{
	"modularity/unit/leiden":                 "84:0e8a640a3d41bd93",
	"modularity/unit/leiden-parallel":        "83:f879d18985b3a667",
	"modularity/unit/louvain":                "84:277310f3d15ad219",
	"modularity/unit/louvain-parallel":       "83:410f0863b57812fd",
	"modularity/fractional/leiden":           "79:18e5db20312d0266",
	"modularity/fractional/leiden-parallel":  "77:0876df18f927e127",
	"modularity/fractional/louvain":          "79:e411f15b9182d6ab",
	"modularity/fractional/louvain-parallel": "79:715100a7b06217f4",
	"cpm/unit/leiden":                        "326:321014854d14471d",
	"cpm/unit/leiden-parallel":               "334:40adc2f7c3bf890b",
	"cpm/unit/louvain":                       "320:461835c7f3e0f0f0",
	"cpm/unit/louvain-parallel":              "331:33553c1c56c27072",
	"cpm/fractional/leiden":                  "427:d0bddeb66850fc77",
	"cpm/fractional/leiden-parallel":         "432:4adf084b5a1d4445",
	"cpm/fractional/louvain":                 "431:ac9a4f4cb96eebee",
	"cpm/fractional/louvain-parallel":        "431:02426fbc4e02fa3a",
	"directed/unit/leiden":                   "82:3e0df0c2bbdc1200",
	"directed/unit/leiden-parallel":          "83:f879d18985b3a667",
	"directed/unit/louvain":                  "82:43dcd2399d6c545f",
	"directed/unit/louvain-parallel":         "83:410f0863b57812fd",
	"directed/fractional/leiden":             "79:4637a24ab4fd8356",
	"directed/fractional/leiden-parallel":    "77:0876df18f927e127",
	"directed/fractional/louvain":            "79:e411f15b9182d6ab",
	"directed/fractional/louvain-parallel":   "79:715100a7b06217f4",
}
