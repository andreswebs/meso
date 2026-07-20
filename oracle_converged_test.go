package meso

import (
	"math"
	"testing"
)

// oracleConvergedSeed is the documented, fixed seed under which meso's converged
// modularity partition per corpus graph was generated and committed to the
// oracle input set (verification/oracle/inputs/{karate,dolphins,lesmis}.json).
// meso is a pure function of (graph, options, seed), so this partition is stable
// and re-checked for drift below. It is the default seed, recorded explicitly so
// a future change to the default cannot silently move the committed partition.
const oracleConvergedSeed uint64 = 0

// convergedCorpus names the corpus graphs that carry a committed converged
// modularity case, with a loose band around each graph's published-literature
// modularity optimum used only to corroborate meso's own exact value out of band
// (not to assert optimality; modularity maximization is NP-hard, so meso's
// converged partition is a strong local optimum). karate's published optimum is
// about 0.4198, dolphins about 0.52, Les Miserables about 0.56.
var convergedCorpus = []struct {
	name   string
	litOpt float64
	band   float64
}{
	{"karate", 0.4198, 0.01},
	{"dolphins", 0.52, 0.02},
	{"lesmis", 0.56, 0.02},
}

// TestOracleConvergedPartitionPinned is acceptance criterion 4 (mes-xb3w): the
// committed converged case is meso's deterministic Leiden output for the
// documented seed, so re-running Leiden must reproduce a committed input case
// exactly. Because meso is deterministic this pins the partition and catches any
// drift in the optimiser as a test failure, the same discipline as the golden
// corpus (mes-5wqp) but on the value-oracle's converged case. Criterion 5's
// corroboration against the published optimum rides along: the achieved quality
// lands within a loose literature band without the optimum ever being hand-entered
// as the input partition.
func TestOracleConvergedPartitionPinned(t *testing.T) {
	for _, cg := range convergedCorpus {
		t.Run(cg.name, func(t *testing.T) {
			var in inputFile
			readOracleJSON(t, "verification/oracle/inputs/"+cg.name+".json", &in)
			g := buildOracleGraph(t, cg.name, &in)

			got := canonicalize(leiden(g.model, modularity{gamma: 1.0}, oracleConvergedSeed))

			// The recomputed converged partition must be one of the committed
			// modularity cases (the converged case added for this graph).
			idx := -1
			for i, c := range in.Cases {
				if c.Quality == "modularity" && partitionsEqual(Partition(c.Partition), got) {
					idx = i
					break
				}
			}
			if idx < 0 {
				t.Fatalf("%s: converged Leiden partition (seed %d) matches no committed input case; the pin has drifted: %v",
					cg.name, oracleConvergedSeed, []int(got))
			}

			// Corroborate the achieved modularity against the published optimum,
			// without hand-entering it (criterion 5).
			q := modularity{gamma: 1.0}.Quality(g.model, got)
			if math.Abs(q-cg.litOpt) > cg.band {
				t.Errorf("%s: converged modularity %v is outside the literature band %v +/- %v",
					cg.name, q, cg.litOpt, cg.band)
			}
		})
	}
}
