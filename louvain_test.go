package meso

import (
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
)

// louvainTol is the absolute tolerance for the quality identities the serial
// Louvain tests assert (monotonicity across sweeps and levels, stability). It
// matches moveTol/aggTol: at corpus scale the float64 evaluations agree far
// inside this bound, and it sits above moveImproveEps so a partition the loop
// calls stable satisfies the stability checks here too.
const louvainTol = 1e-9

// loadGMLCSR reads a Newman-format GML graph (node/edge blocks with source,
// target, and optional value weight) into the internal csr via the public
// Builder. Unweighted edges default to weight 1. It is a minimal reader scoped
// to the Louvain corpus check; the full golden-corpus loader is a later ticket.
func loadGMLCSR(t *testing.T, path string) *csr {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	b := NewBuilder()
	var src, tgt string
	var w float64
	var inEdge, haveSrc, haveTgt bool
	for line := range strings.SplitSeq(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		switch f[0] {
		case "edge":
			inEdge, haveSrc, haveTgt, w = true, false, false, 1.0
		case "source":
			if inEdge && len(f) >= 2 {
				src, haveSrc = f[1], true
			}
		case "target":
			if inEdge && len(f) >= 2 {
				tgt, haveTgt = f[1], true
			}
		case "value", "weight":
			if inEdge && len(f) >= 2 {
				if v, err := strconv.ParseFloat(f[1], 64); err == nil {
					w = v
				}
			}
		case "]":
			if inEdge && haveSrc && haveTgt {
				b.AddEdge(src, tgt, w)
			}
			inEdge = false
		}
	}
	g, err := b.Build()
	if err != nil {
		t.Fatalf("build %s: %v", path, err)
	}
	return g.model
}

// TestLocalMove_SweepMonotone is the Go image of localMoveRun_monotone: one full
// local-move sweep never lowers modularity. Each accepted move strictly raises
// the objective and moves apply against the running partition, so the sweep is
// monotone from any starting partition.
func TestLocalMove_SweepMonotone(t *testing.T) {
	const trials = 2000
	base := rand.New(rand.NewSource(1))
	for range trials {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))
		g := randomCSR(rng)
		p := randomPartition(rng, g.numNodes())
		gamma := 0.3 + rng.Float64()*1.5
		m := modularity{gamma: gamma}

		before := m.Quality(g, p)
		fresh := g.numNodes()
		localMoveSweep(g, m, p, &fresh)
		after := m.Quality(g, p)

		if after < before-louvainTol {
			t.Fatalf("seed %d: sweep lowered modularity %v -> %v", seed, before, after)
		}
	}
}

// TestCPM_SweepMonotone is the CPM twin of TestLocalMove_SweepMonotone, over
// graphs with non-unit node sizes so the CPM size penalty is exercised.
func TestCPM_SweepMonotone(t *testing.T) {
	const trials = 2000
	base := rand.New(rand.NewSource(2))
	for range trials {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))
		g := randomCSRSized(rng)
		p := randomPartition(rng, g.numNodes())
		gamma := 0.3 + rng.Float64()*1.5
		m := cpm{gamma: gamma}

		before := m.Quality(g, p)
		fresh := g.numNodes()
		localMoveSweep(g, m, p, &fresh)
		after := m.Quality(g, p)

		if after < before-louvainTol {
			t.Fatalf("seed %d: sweep lowered CPM %v -> %v", seed, before, after)
		}
	}
}

// TestLeiden_QualityMonotoneAcrossLevels is the Go image of
// quality_monotone_of_stepwise: quality is non-decreasing across the whole
// multilevel run, from one recorded level's base partition to the next.
func TestLeiden_QualityMonotoneAcrossLevels(t *testing.T) {
	run := func(t *testing.T, obj objective, seed int64) {
		rng := rand.New(rand.NewSource(seed))
		g := randomCSRSized(rng)
		levels := louvainLevels(g, obj)
		for i := 0; i+1 < len(levels); i++ {
			lo := obj.Quality(g, levels[i])
			hi := obj.Quality(g, levels[i+1])
			if hi < lo-louvainTol {
				t.Fatalf("seed %d %T: level %d->%d lowered quality %v -> %v", seed, obj, i, i+1, lo, hi)
			}
		}
	}
	base := rand.New(rand.NewSource(3))
	for range 1000 {
		seed := base.Int63()
		run(t, modularity{gamma: 0.5 + rand.New(rand.NewSource(seed)).Float64()}, seed)
		run(t, cpm{gamma: 0.5 + rand.New(rand.NewSource(seed+1)).Float64()}, seed+1)
	}
}

// TestLeiden_LevelNonDecreasing is the Go image of le_modularity_aggregate_run:
// one level does not lose quality across the aggregation boundary. From a stable
// partition p on g, aggregating and running local moves on the aggregate reaches
// a q whose quality is at least that of p (aggregation invariance pins the
// aggregate singleton at Quality(g, p); local moving only climbs from there).
func TestLeiden_LevelNonDecreasing(t *testing.T) {
	testLevelBoundary(t, func(gamma float64) objective { return modularity{gamma: gamma} }, 4)
}

// TestLeiden_CPMLevelNonDecreasing is the CPM twin (le_cpm_aggregate_run).
func TestLeiden_CPMLevelNonDecreasing(t *testing.T) {
	testLevelBoundary(t, func(gamma float64) objective { return cpm{gamma: gamma} }, 5)
}

func testLevelBoundary(t *testing.T, mk func(float64) objective, srcSeed int64) {
	t.Helper()
	base := rand.New(rand.NewSource(srcSeed))
	for range 1000 {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))
		g := randomCSRSized(rng)
		obj := mk(0.4 + rng.Float64())

		p := singleton(g.numNodes())
		localMoveToStable(g, obj, p)
		beforeLevel := obj.Quality(g, p)

		aggG, _ := aggregate(g, p)
		q := singleton(aggG.numNodes())
		localMoveToStable(aggG, obj, q)
		afterLevel := obj.Quality(aggG, q)

		if afterLevel < beforeLevel-louvainTol {
			t.Fatalf("seed %d %T: level boundary lowered quality %v -> %v", seed, obj, beforeLevel, afterLevel)
		}
	}
}

// TestLeiden_QualityMonotoneWholeRun is the Go image of
// modularity_le_of_algorithmRun: the whole multilevel run never lowers quality
// below the initial singleton partition.
func TestLeiden_QualityMonotoneWholeRun(t *testing.T) {
	testWholeRun(t, func(gamma float64) objective { return modularity{gamma: gamma} }, 6)
}

// TestLeiden_CPMMonotoneWholeRun is the CPM twin (cpm_le_of_algorithmRun).
func TestLeiden_CPMMonotoneWholeRun(t *testing.T) {
	testWholeRun(t, func(gamma float64) objective { return cpm{gamma: gamma} }, 7)
}

func testWholeRun(t *testing.T, mk func(float64) objective, srcSeed int64) {
	t.Helper()
	base := rand.New(rand.NewSource(srcSeed))
	for range 1000 {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))
		g := randomCSRSized(rng)
		obj := mk(0.4 + rng.Float64())

		start := obj.Quality(g, singleton(g.numNodes()))
		final := obj.Quality(g, louvain(g, obj))
		if final < start-louvainTol {
			t.Fatalf("seed %d %T: whole run lowered quality %v -> %v", seed, obj, start, final)
		}
	}
}

// TestLocalMove_TerminatesBounded is the empirical mirror of
// no_infinite_acceptedMove_run: the local-move phase halts at a genuine fixed
// point. After localMoveToStable, a further sweep applies no move, and the
// stable quality is at least the starting quality.
func TestLocalMove_TerminatesBounded(t *testing.T) {
	base := rand.New(rand.NewSource(8))
	for range 1500 {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))
		g := randomCSR(rng)
		m := modularity{gamma: 0.3 + rng.Float64()*1.5}

		p := singleton(g.numNodes())
		start := m.Quality(g, p)
		localMoveToStable(g, m, p)
		stable := m.Quality(g, p)

		fresh := g.numNodes()
		for _, c := range p {
			if c >= fresh {
				fresh = c + 1
			}
		}
		if localMoveSweep(g, m, p, &fresh) {
			t.Fatalf("seed %d: sweep after localMoveToStable still moved a node", seed)
		}
		if stable < start-louvainTol {
			t.Fatalf("seed %d: stable quality %v below start %v", seed, stable, start)
		}
	}
}

// TestLeiden_LevelsTerminate / _LevelSizeDichotomy / _LevelsReachFixedPoint /
// _RunningLevelShrinks / _GuardedLevelsTerminate together mirror the
// Termination.lean cluster and the guard of CORRESPONDENCE.md section 4. They
// share one trace family over the corpus and fuzzed inputs.

// forEachTrace runs fn against a Louvain trace of every corpus/fuzzed graph
// under both objectives, so the termination tests each assert one property over
// the same population.
func forEachTrace(t *testing.T, seed int64, fn func(t *testing.T, name string, obj objective, g *csr, trace []louvainLevel)) {
	t.Helper()
	for _, tc := range corpusAndFuzzed(t, seed) {
		for _, obj := range []objective{modularity{gamma: 1.0}, cpm{gamma: 0.1}} {
			fn(t, tc.name, obj, tc.g, louvainTrace(tc.g, obj))
		}
	}
}

// TestLeiden_LevelsTerminate: the multilevel loop returns a non-empty, finite
// trace on every input - it does not hang (levelRun_reaches_fixedPoint).
func TestLeiden_LevelsTerminate(t *testing.T) {
	forEachTrace(t, 9, func(t *testing.T, name string, obj objective, _ *csr, trace []louvainLevel) {
		if len(trace) == 0 {
			t.Fatalf("%s %T: empty trace", name, obj)
		}
	})
}

// TestLeiden_LevelSizeDichotomy is the Go image of
// levelStep_size_lt_or_injective: every level either merged (and so is not the
// terminal level) or merged nothing, in which case it is the last level - a
// discrete-partition fixed point on which aggregation would do nothing.
func TestLeiden_LevelSizeDichotomy(t *testing.T) {
	forEachTrace(t, 10, func(t *testing.T, name string, obj objective, _ *csr, trace []louvainLevel) {
		for i, lv := range trace {
			if !lv.merged && i != len(trace)-1 {
				t.Fatalf("%s %T: non-merging level %d is not the last", name, obj, i)
			}
			if lv.merged && i == len(trace)-1 {
				t.Fatalf("%s %T: last level %d still merged", name, obj, i)
			}
		}
	})
}

// TestLeiden_LevelsReachFixedPoint is the Go image of
// levelRun_reaches_fixedPoint: the run stops at a level whose local-move phase
// merged nothing (the discrete partition), not by an arbitrary cutoff.
func TestLeiden_LevelsReachFixedPoint(t *testing.T) {
	forEachTrace(t, 11, func(t *testing.T, name string, obj objective, _ *csr, trace []louvainLevel) {
		if trace[len(trace)-1].merged {
			t.Fatalf("%s %T: final level still merged (no discrete-partition fixed point reached)", name, obj)
		}
	})
}

// TestLeiden_RunningLevelShrinks is the Go image of RunningLevelStep.size_lt:
// each running (merging) level strictly reduces the working-graph node count of
// the next level.
func TestLeiden_RunningLevelShrinks(t *testing.T) {
	forEachTrace(t, 12, func(t *testing.T, name string, obj objective, _ *csr, trace []louvainLevel) {
		for i, lv := range trace {
			if !lv.merged {
				continue
			}
			if i+1 >= len(trace) {
				t.Fatalf("%s %T: running level %d has no successor", name, obj, i)
			}
			if trace[i+1].working >= lv.working {
				t.Fatalf("%s %T: running level %d did not shrink working graph (%d -> %d)",
					name, obj, i, lv.working, trace[i+1].working)
			}
		}
	})
}

// TestLeiden_GuardedLevelsTerminate is the Go image of
// no_infinite_runningLevel_run: the guarded loop halts - it stops at a
// non-merging level and executes at most n levels for n input nodes.
func TestLeiden_GuardedLevelsTerminate(t *testing.T) {
	forEachTrace(t, 13, func(t *testing.T, name string, obj objective, g *csr, trace []louvainLevel) {
		if trace[len(trace)-1].merged {
			t.Fatalf("%s %T: loop stopped at a merging level", name, obj)
		}
		// Running levels number at most n (each strictly shrinks the graph); the
		// trace is those plus the one terminal non-merging level.
		if maxLevels := g.numNodes() + 1; len(trace) > maxLevels {
			t.Fatalf("%s %T: %d levels exceeds bound %d", name, obj, len(trace), maxLevels)
		}
	})
}

// TestConverge_StableNoImprovement is the Go image of
// IsLocalMoveStable.no_strict_improvement: at a local-move-stable partition no
// single-node move to any community strictly improves the objective. It checks
// every present community and a fresh isolation label per node, which is
// exhaustive up to sign (moving into a community a node has no edge to is never
// better than isolating it).
func TestConverge_StableNoImprovement(t *testing.T) {
	base := rand.New(rand.NewSource(10))
	for range 1500 {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))
		g := randomCSRSized(rng)
		obj := objective(modularity{gamma: 0.3 + rng.Float64()*1.5})
		if seed%2 == 0 {
			obj = cpm{gamma: 0.2 + rng.Float64()}
		}

		p := singleton(g.numNodes())
		localMoveToStable(g, obj, p)

		present := communityLabels(p)
		fresh := g.numNodes()
		for _, c := range present {
			if c >= fresh {
				fresh = c + 1
			}
		}
		targets := append(append([]int{}, present...), fresh)

		for u := 0; u < g.numNodes(); u++ {
			for _, c := range targets {
				if d := obj.moveDelta(g, p, u, c); d > louvainTol {
					t.Fatalf("seed %d %T: node %d -> community %d improves stable partition by %v",
						seed, obj, u, c, d)
				}
			}
		}
	}
}

// TestConverge_StableIsMoveFixedPoint is the Go image of
// IsLocalMove.quality_eq_of_stable: at a stable partition the best available
// local move leaves quality unchanged (its delta is zero within tolerance), so
// the partition is a genuine fixed point of the move dynamics.
func TestConverge_StableIsMoveFixedPoint(t *testing.T) {
	base := rand.New(rand.NewSource(11))
	for range 1500 {
		seed := base.Int63()
		rng := rand.New(rand.NewSource(seed))
		g := randomCSR(rng)
		obj := objective(modularity{gamma: 0.3 + rng.Float64()*1.5})

		p := singleton(g.numNodes())
		localMoveToStable(g, obj, p)

		fresh := g.numNodes()
		for _, c := range p {
			if c >= fresh {
				fresh = c + 1
			}
		}
		for u := 0; u < g.numNodes(); u++ {
			target, delta := bestMove(g, obj, p, u, fresh)
			if delta > louvainTol {
				t.Fatalf("seed %d: node %d best move (-> %d) improves stable partition by %v", seed, u, target, delta)
			}
			applied := obj.Quality(g, move(p, u, target))
			if math.Abs(applied-obj.Quality(g, p)) > louvainTol {
				t.Fatalf("seed %d: best move for node %d changed quality of stable partition", seed, u)
			}
		}
	}
}

// TestLouvain_KarateShape is acceptance criterion 8: serial Louvain on the
// karate club recovers a good modularity partition (well above trivial) with a
// small number of communities, the known shape of the graph.
func TestLouvain_KarateShape(t *testing.T) {
	g := loadGMLCSR(t, "datasets/karate/karate.gml")
	if g.numNodes() != 34 {
		t.Fatalf("karate NumNodes = %d, want 34", g.numNodes())
	}
	m := modularity{gamma: 1.0}
	p := louvain(g, m)
	if err := p.wellFormed(g.numNodes()); err != nil {
		t.Fatalf("louvain partition not well-formed: %v", err)
	}
	q := m.Quality(g, p)
	nc := numCommunities(p)
	t.Logf("karate: modularity = %.4f, communities = %d", q, nc)
	if q < 0.40 {
		t.Errorf("karate modularity = %.4f, want >= 0.40", q)
	}
	if nc < 2 || nc > 6 {
		t.Errorf("karate communities = %d, want in [2, 6]", nc)
	}
}

// fuzzGraph names a graph for the shared termination trace.
type fuzzGraph struct {
	name string
	g    *csr
}

// corpusAndFuzzed returns the karate corpus graph plus a spread of fuzzed random
// graphs for the termination checks.
func corpusAndFuzzed(t *testing.T, seed int64) []fuzzGraph {
	t.Helper()
	graphs := []fuzzGraph{{"karate", loadGMLCSR(t, "datasets/karate/karate.gml")}}
	base := rand.New(rand.NewSource(seed))
	for range 200 {
		rng := rand.New(rand.NewSource(base.Int63()))
		graphs = append(graphs, fuzzGraph{"fuzz", randomCSRSized(rng)})
	}
	return graphs
}
