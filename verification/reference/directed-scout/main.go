// Command directed-scout is the Phase 3 guarantee-triage harness for directed
// modularity (ticket mes-qmch). It generates random directed graphs across a
// parameter grid, runs meso's directed Leiden to convergence through the public
// API only, and hunts counterexamples to three candidate directed guarantees:
// weak connectivity of returned communities, gamma-separation (no community
// merge improves), and subset-optimality (no subset split improves).
//
// Every candidate bound and every quality value is recomputed here independently
// from the generated edge list and the returned partition, so a shared formula
// bug in meso cannot blind the scout. meso is used only to build the graph and
// to run Leiden.
//
// This module is out-of-band reference tooling, the Go analogue of
// ../crosscheck.py: committed and reproducible, never part of the build gate or
// CI. Run it manually with `go run .` from this directory.
package main

import (
	"flag"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"os"
	"sort"
	"strconv"

	"github.com/andreswebs/meso"
)

const (
	improveEps       = 1e-9 // a move/merge must beat this to count as improving
	boundTol         = 1e-9 // bound-violation tolerance
	qualityTol       = 1e-8 // scout-Q vs meso-Q agreement tolerance
	maxDrive         = 2000 // safety cap on convergence-driving steps
	subsetExhaustCap = 12   // exhaustive subset check up to this community size
	subsetSamples    = 500  // sampled subsets per community above the cap
)

// dgraph is the scout's own dense directed weighted graph: w[i][j] is the
// weight of the arc i -> j. Degrees and total arc weight follow the conventions
// pinned in the Lean model (self-loops count once in kout, kin, and m).
type dgraph struct {
	n    int
	w    [][]float64
	kout []float64
	kin  []float64
	m    float64
}

func newDgraph(n int) *dgraph {
	g := &dgraph{n: n, w: make([][]float64, n)}
	for i := range g.w {
		g.w[i] = make([]float64, n)
	}
	return g
}

// finalize computes kout, kin, and m from the weight matrix.
func (g *dgraph) finalize() {
	g.kout = make([]float64, g.n)
	g.kin = make([]float64, g.n)
	g.m = 0
	for i := 0; i < g.n; i++ {
		for j := 0; j < g.n; j++ {
			g.kout[i] += g.w[i][j]
			g.kin[j] += g.w[i][j]
		}
	}
	for i := 0; i < g.n; i++ {
		g.m += g.kout[i]
	}
}

// directedQ is the scout's independent Leicht-Newman directed modularity:
// Q = (1/m) * sum_ij (w_ij - gamma*kout_i*kin_j/m) * delta(p_i, p_j).
func (g *dgraph) directedQ(gamma float64, p []int) float64 {
	if g.m == 0 {
		return 0
	}
	sum := 0.0
	for i := 0; i < g.n; i++ {
		for j := 0; j < g.n; j++ {
			if p[i] != p[j] {
				continue
			}
			sum += g.w[i][j] - gamma*g.kout[i]*g.kin[j]/g.m
		}
	}
	return sum / g.m
}

// cut is the ordered arc weight from A into B: e(A,B) = sum_{i in A, j in B} w_ij.
func (g *dgraph) cut(a, b []int) float64 {
	s := 0.0
	for _, i := range a {
		for _, j := range b {
			s += g.w[i][j]
		}
	}
	return s
}

// communities groups node indices by partition label, in ascending label order.
func communities(p []int) [][]int {
	byLabel := map[int][]int{}
	for i, c := range p {
		byLabel[c] = append(byLabel[c], i)
	}
	labels := make([]int, 0, len(byLabel))
	for c := range byLabel {
		labels = append(labels, c)
	}
	sort.Ints(labels)
	out := make([][]int, 0, len(labels))
	for _, c := range labels {
		out = append(out, byLabel[c])
	}
	return out
}

// commDegrees returns each community's summed out- and in-degree.
func (g *dgraph) commDegrees(comms [][]int) (kout, kin []float64) {
	kout = make([]float64, len(comms))
	kin = make([]float64, len(comms))
	for c, nodes := range comms {
		for _, i := range nodes {
			kout[c] += g.kout[i]
			kin[c] += g.kin[i]
		}
	}
	return kout, kin
}

// applyBestImprovement scans every single-node move (to any occupied community
// or to a fresh label) and every whole-community merge, all priced from scratch
// with directedQ, and applies the best strictly-improving one. It reports
// whether an improvement was applied and whether it was a merge (a level
// -stability gap) rather than a single-node move.
func applyBestImprovement(g *dgraph, gamma float64, p []int) (applied, wasMerge bool) {
	base := g.directedQ(gamma, p)
	labels := map[int]struct{}{}
	fresh := 0
	for _, c := range p {
		labels[c] = struct{}{}
		if c >= fresh {
			fresh = c + 1
		}
	}
	targets := make([]int, 0, len(labels)+1)
	for c := range labels {
		targets = append(targets, c)
	}
	targets = append(targets, fresh)
	sort.Ints(targets)

	bestDelta := 0.0
	bestKind := ""
	var bestU, bestTo, bestA, bestB int
	q := make([]int, len(p))

	for u := range p {
		for _, c := range targets {
			if c == p[u] {
				continue
			}
			copy(q, p)
			q[u] = c
			if d := g.directedQ(gamma, q) - base; d > bestDelta {
				bestDelta, bestKind, bestU, bestTo = d, "move", u, c
			}
		}
	}
	occupied := targets[:len(targets)-1]
	for ai := 0; ai < len(occupied); ai++ {
		for bi := ai + 1; bi < len(occupied); bi++ {
			copy(q, p)
			for i := range q {
				if q[i] == occupied[bi] {
					q[i] = occupied[ai]
				}
			}
			if d := g.directedQ(gamma, q) - base; d > bestDelta {
				bestDelta, bestKind, bestA, bestB = d, "merge", occupied[ai], occupied[bi]
			}
		}
	}

	if bestDelta <= improveEps {
		return false, false
	}
	switch bestKind {
	case "move":
		p[bestU] = bestTo
	case "merge":
		for i := range p {
			if p[i] == bestB {
				p[i] = bestA
			}
		}
	}
	return true, bestKind == "merge"
}

// driveConverged applies best improvements until neither a single-node move nor
// a community merge strictly improves, returning the step count (moves, merges)
// and whether the cap was hit (caller discards the sample).
func driveConverged(g *dgraph, gamma float64, p []int) (moves, merges int, capped bool) {
	for steps := 0; steps < maxDrive; steps++ {
		applied, wasMerge := applyBestImprovement(g, gamma, p)
		if !applied {
			return moves, merges, false
		}
		if wasMerge {
			merges++
		} else {
			moves++
		}
	}
	return moves, merges, true
}

// weaklyConnected reports whether the community induces a connected subgraph of
// the underlying undirected graph (an edge wherever w_ij > 0 or w_ji > 0;
// self-loops are not connectivity edges).
func (g *dgraph) weaklyConnected(nodes []int) bool {
	return g.connected(nodes, func(i, j int) bool { return g.w[i][j] > 0 || g.w[j][i] > 0 })
}

// stronglyConnected reports whether every node of the community reaches every
// other through directed arcs staying inside the community.
func (g *dgraph) stronglyConnected(nodes []int) bool {
	if !g.connected(nodes, func(i, j int) bool { return g.w[i][j] > 0 }) {
		return false
	}
	return g.connected(nodes, func(i, j int) bool { return g.w[j][i] > 0 })
}

// connected runs BFS from the first node over the given edge relation
// restricted to the community, and reports whether all members are reached.
// For an asymmetric relation this checks reachability from nodes[0]; combined
// with the transposed relation it decides strong connectivity.
func (g *dgraph) connected(nodes []int, edge func(i, j int) bool) bool {
	if len(nodes) <= 1 {
		return true
	}
	inComm := map[int]bool{}
	for _, i := range nodes {
		inComm[i] = true
	}
	seen := map[int]bool{nodes[0]: true}
	queue := []int{nodes[0]}
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		for _, j := range nodes {
			if !seen[j] && edge(i, j) && inComm[j] {
				seen[j] = true
				queue = append(queue, j)
			}
		}
	}
	return len(seen) == len(nodes)
}

// gammaSepCheck evaluates the candidate directed gamma-separation bound
//
//	e(C,D) + e(D,C) <= (gamma/m) * (Kout_C*Kin_D + Kout_D*Kin_C)
//
// for every unordered community pair, returning the violation count and the
// minimum slack (rhs - lhs; negative means violated). It also cross-checks the
// closed-form merge gain (lhs-rhs)/m against the from-scratch Q difference,
// returning the maximum absolute discrepancy.
func (g *dgraph) gammaSepCheck(gamma float64, p []int, comms [][]int) (viol int, minSlack, maxGainErr float64) {
	kout, kin := g.commDegrees(comms)
	minSlack = math.Inf(1)
	q := make([]int, len(p))
	for a := 0; a < len(comms); a++ {
		for b := a + 1; b < len(comms); b++ {
			lhs := g.cut(comms[a], comms[b]) + g.cut(comms[b], comms[a])
			rhs := gamma * (kout[a]*kin[b] + kout[b]*kin[a]) / g.m
			slack := rhs - lhs
			if slack < minSlack {
				minSlack = slack
			}
			if slack < -boundTol {
				viol++
			}

			copy(q, p)
			la, lb := p[comms[a][0]], p[comms[b][0]]
			for i := range q {
				if q[i] == lb {
					q[i] = la
				}
			}
			fromScratch := g.directedQ(gamma, q) - g.directedQ(gamma, p)
			closedForm := (lhs - rhs) / g.m
			if e := math.Abs(fromScratch - closedForm); e > maxGainErr {
				maxGainErr = e
			}
		}
	}
	return viol, minSlack, maxGainErr
}

// subsetCheck evaluates the candidate directed subset-optimality bound
//
//	e(S,T) + e(T,S) >= (gamma/m) * (Kout_S*Kin_T + Kout_T*Kin_S),  T = C \ S
//
// for every proper nonempty subset S of every community C (exhaustive up to
// subsetExhaustCap members, sampled above), returning the violation count, the
// number of communities with at least one violation, the minimum slack
// (lhs - rhs), whether any community was sampled rather than exhausted, and the
// maximum discrepancy of the closed-form split gain -(lhs-rhs)/m against the
// from-scratch difference (checked on the first few subsets per community).
func (g *dgraph) subsetCheck(gamma float64, p []int, comms [][]int, rng *rand.Rand) (viol, violComms int, minSlack float64, sampled bool, maxGainErr float64) {
	minSlack = math.Inf(1)
	fresh := 0
	for _, c := range p {
		if c >= fresh {
			fresh = c + 1
		}
	}
	q := make([]int, len(p))
	for _, comm := range comms {
		k := len(comm)
		if k < 2 {
			continue
		}
		koutN := make([]float64, k)
		kinN := make([]float64, k)
		for x, i := range comm {
			koutN[x] = g.kout[i]
			kinN[x] = g.kin[i]
		}

		commViol := 0
		checkMask := func(mask uint64, crossCheck bool) {
			var s, t []int
			var koutS, kinS, koutT, kinT float64
			for x := 0; x < k; x++ {
				if mask&(1<<uint(x)) != 0 {
					s = append(s, comm[x])
					koutS += koutN[x]
					kinS += kinN[x]
				} else {
					t = append(t, comm[x])
					koutT += koutN[x]
					kinT += kinN[x]
				}
			}
			lhs := g.cut(s, t) + g.cut(t, s)
			rhs := gamma * (koutS*kinT + koutT*kinS) / g.m
			slack := lhs - rhs
			if slack < minSlack {
				minSlack = slack
			}
			if slack < -boundTol {
				commViol++
			}
			if crossCheck {
				copy(q, p)
				for _, i := range s {
					q[i] = fresh
				}
				fromScratch := g.directedQ(gamma, q) - g.directedQ(gamma, p)
				closedForm := -(lhs - rhs) / g.m
				if e := math.Abs(fromScratch - closedForm); e > maxGainErr {
					maxGainErr = e
				}
			}
		}

		if k <= subsetExhaustCap {
			for mask := uint64(1); mask < (1<<uint(k))-1; mask++ {
				checkMask(mask, mask <= 3)
			}
		} else {
			sampled = true
			full := (uint64(1) << uint(k)) - 1
			for s := 0; s < subsetSamples; s++ {
				mask := rng.Uint64() & full
				if mask == 0 || mask == full {
					continue
				}
				checkMask(mask, s < 3)
			}
		}
		viol += commViol
		if commViol > 0 {
			violComms++
		}
	}
	return viol, violComms, minSlack, sampled, maxGainErr
}

// config is one cell of the scouting grid.
type config struct {
	n         int
	density   float64
	weights   string // unit | int | cont
	regime    string // uniform | dag | srcsink | cycle | symmetric
	selfLoops bool
	gamma     float64
	rep       int
}

func (c config) String() string {
	return fmt.Sprintf("n=%d dens=%.1f w=%s regime=%s loops=%v gamma=%.1f rep=%d",
		c.n, c.density, c.weights, c.regime, c.selfLoops, c.gamma, c.rep)
}

// seed derives a deterministic per-config seed so any hit is reproducible from
// the logged config string alone.
func (c config) seed() uint64 {
	h := fnv.New64a()
	fmt.Fprint(h, c.String())
	return h.Sum64()
}

// generate builds the scout's dense graph for a config. At least one arc is
// guaranteed so meso never sees an arcless graph.
func generate(c config, rng *rand.Rand) *dgraph {
	g := newDgraph(c.n)
	weight := func() float64 {
		switch c.weights {
		case "unit":
			return 1
		case "int":
			return float64(1 + rng.IntN(5))
		default:
			return rng.Float64() + 1e-3
		}
	}

	isSource := make([]bool, c.n) // no in-arcs
	isSink := make([]bool, c.n)   // no out-arcs
	if c.regime == "srcsink" {
		for i := 0; i < c.n; i++ {
			switch rng.IntN(5) {
			case 0:
				isSource[i] = true
			case 1:
				isSink[i] = true
			}
		}
	}

	addArc := func(i, j int) {
		if isSink[i] || isSource[j] {
			return
		}
		g.w[i][j] = weight()
	}

	switch c.regime {
	case "dag":
		for i := 0; i < c.n; i++ {
			for j := i + 1; j < c.n; j++ {
				if rng.Float64() < c.density {
					addArc(i, j)
				}
			}
		}
	case "symmetric":
		for i := 0; i < c.n; i++ {
			for j := i + 1; j < c.n; j++ {
				if rng.Float64() < c.density {
					w := weight()
					g.w[i][j] = w
					g.w[j][i] = w
				}
			}
		}
	case "cycle":
		perm := rng.Perm(c.n)
		for x := range perm {
			addArc(perm[x], perm[(x+1)%c.n])
		}
		fallthrough
	default: // uniform, srcsink, and cycle's extra arcs
		for i := 0; i < c.n; i++ {
			for j := 0; j < c.n; j++ {
				if i != j && g.w[i][j] == 0 && rng.Float64() < c.density {
					addArc(i, j)
				}
			}
		}
	}

	if c.selfLoops {
		for i := 0; i < c.n; i++ {
			if !isSink[i] && !isSource[i] && rng.Float64() < 0.1 {
				g.w[i][i] = weight()
			}
		}
	}

	arcs := 0
	for i := 0; i < c.n; i++ {
		for j := 0; j < c.n; j++ {
			if g.w[i][j] > 0 {
				arcs++
			}
		}
	}
	if arcs == 0 {
		g.w[0][1%c.n] = 1
	}
	g.finalize()
	return g
}

// runLeiden builds the meso graph through the public directed builder and runs
// Leiden with DirectedModularity, returning the partition indexed by node and
// meso's reported quality.
func runLeiden(g *dgraph, gamma float64, seed uint64) ([]int, float64, error) {
	b := meso.NewDirectedBuilder()
	for i := 0; i < g.n; i++ {
		b.AddNodeWeight(strconv.Itoa(i), 1)
	}
	for i := 0; i < g.n; i++ {
		for j := 0; j < g.n; j++ {
			if g.w[i][j] > 0 {
				b.AddEdge(strconv.Itoa(i), strconv.Itoa(j), g.w[i][j])
			}
		}
	}
	mg, err := b.Build()
	if err != nil {
		return nil, 0, fmt.Errorf("build: %w", err)
	}
	res, err := meso.Leiden(mg, meso.WithQuality(meso.DirectedModularity(gamma)), meso.WithSeed(seed))
	if err != nil {
		return nil, 0, fmt.Errorf("leiden: %w", err)
	}
	comm := res.Communities()
	if len(comm) != g.n {
		return nil, 0, fmt.Errorf("communities returned %d of %d nodes", len(comm), g.n)
	}
	p := make([]int, g.n)
	for i := 0; i < g.n; i++ {
		c, ok := comm[strconv.Itoa(i)]
		if !ok {
			return nil, 0, fmt.Errorf("node %d missing from result", i)
		}
		p[i] = c
	}
	return p, res.Quality(), nil
}

// selfTest pins the scout's independent objective to the hand-computed values
// of directed_quality_test.go's asymmetric fixture (also machine-checked in
// Lean, Meso/DirectedCompute.lean), and verifies the symmetric reduction of the
// bound expressions on a triangle. Aborts the program on any mismatch: a scout
// that disagrees with the pinned semantics would manufacture or hide
// counterexamples.
func selfTest() {
	g := newDgraph(3)
	g.w[0][1] = 2
	g.w[0][2] = 1
	g.w[1][0] = 1
	g.finalize()
	cases := []struct {
		gamma float64
		p     []int
		want  float64
	}{
		{1, []int{0, 0, 0}, 0},
		{1, []int{0, 1, 2}, -0.3125},
		{2, []int{0, 1, 2}, -0.625},
		{1, []int{0, 0, 1}, 0},
	}
	for _, tc := range cases {
		got := g.directedQ(tc.gamma, tc.p)
		if math.Abs(got-tc.want) > 1e-12 {
			fmt.Fprintf(os.Stderr, "self-test FAILED: Q(gamma=%v, p=%v) = %v, want %v\n",
				tc.gamma, tc.p, got, tc.want)
			os.Exit(1)
		}
	}
	// Merge gain: from singletons at gamma=1, merging {1} into {0} lands on
	// {0,0,1} with Q=0, so the gain is +5/16. Closed form must agree.
	p := []int{0, 1, 2}
	comms := communities(p)
	_, _, gainErr := g.gammaSepCheck(1, p, comms)
	if gainErr > 1e-12 {
		fmt.Fprintf(os.Stderr, "self-test FAILED: merge closed form off by %v\n", gainErr)
		os.Exit(1)
	}

	// Symmetric reduction on a unit triangle: the directed bound must be
	// exactly twice the undirected bound on both sides, i.e. equivalent.
	t := newDgraph(3)
	for _, e := range [][2]int{{0, 1}, {1, 2}, {0, 2}} {
		t.w[e[0]][e[1]] = 1
		t.w[e[1]][e[0]] = 1
	}
	t.finalize()
	tp := []int{0, 0, 1}
	tc := communities(tp)
	kout, kin := t.commDegrees(tc)
	lhs := t.cut(tc[0], tc[1]) + t.cut(tc[1], tc[0])
	rhs := 1.0 * (kout[0]*kin[1] + kout[1]*kin[0]) / t.m
	// undirected: e(C,D) <= gamma*K_C*K_D/2m with e the one-orientation cut
	uLhs := t.cut(tc[0], tc[1])
	uRhs := 1.0 * kout[0] * kout[1] / t.m
	if math.Abs(lhs-2*uLhs) > 1e-12 || math.Abs(rhs-2*uRhs) > 1e-12 {
		fmt.Fprintf(os.Stderr, "self-test FAILED: symmetric reduction mismatch\n")
		os.Exit(1)
	}
	fmt.Println("self-test: OK (fixture values, merge closed form, symmetric reduction)")
}

// stats accumulates grid-wide results for one regime bucket.
type stats struct {
	samples          int
	discarded        int
	drivenSamples    int
	driveMoves       int
	driveMerges      int
	mergeGapSamples  int // samples where driving applied at least one merge
	qualityMismatch  int
	maxQualityErr    float64
	weakViol         int
	strongFail       int
	commsChecked     int
	sepViolRaw       int
	sepViolConverged int
	sepMinSlack      float64
	maxMergeGainErr  float64
	subViol          int
	subViolComms     int
	subViolSamples   int
	subMinSlack      float64
	subSampledAny    bool
	maxSplitGainErr  float64
}

// huntSubsetFixture searches small graphs, ascending in size, for a converged
// partition violating the subset bound, and prints the first hit as an explicit
// edge list plus partition plus violating subset, so the counterexample
// survives independently of this harness. Exits after the first hit.
func huntSubsetFixture() {
	for n := 4; n <= 14; n++ {
		for _, dens := range []float64{0.2, 0.3, 0.5, 0.7} {
			for _, w := range []string{"unit", "int", "cont"} {
				for _, regime := range []string{"uniform", "dag", "cycle", "symmetric"} {
					for _, gamma := range []float64{0.5, 1, 2} {
						for rep := 0; rep < 50; rep++ {
							c := config{n, dens, w, regime, false, gamma, rep}
							if huntOne(c) {
								return
							}
						}
					}
				}
			}
		}
	}
	fmt.Println("hunt: no subset violation found up to n=14")
}

// huntOne runs one hunt cell; on a subset violation at a converged partition it
// prints the full fixture and returns true.
func huntOne(c config) bool {
	rng := rand.New(rand.NewPCG(c.seed(), c.seed()))
	g := generate(c, rng)
	p, _, err := runLeiden(g, c.gamma, c.seed())
	if err != nil {
		return false
	}
	if _, _, capped := driveConverged(g, c.gamma, p); capped {
		return false
	}
	comms := communities(p)
	for _, comm := range comms {
		k := len(comm)
		if k < 2 || k > subsetExhaustCap {
			continue
		}
		for mask := uint64(1); mask < (1<<uint(k))-1; mask++ {
			var s, t []int
			var koutS, kinS, koutT, kinT float64
			for x := 0; x < k; x++ {
				if mask&(1<<uint(x)) != 0 {
					s = append(s, comm[x])
					koutS += g.kout[comm[x]]
					kinS += g.kin[comm[x]]
				} else {
					t = append(t, comm[x])
					koutT += g.kout[comm[x]]
					kinT += g.kin[comm[x]]
				}
			}
			lhs := g.cut(s, t) + g.cut(t, s)
			rhs := c.gamma * (koutS*kinT + koutT*kinS) / g.m
			if lhs >= rhs-boundTol {
				continue
			}
			fmt.Printf("SUBSET VIOLATION FIXTURE [%s] seed=%d\n", c, c.seed())
			fmt.Printf("arcs (i -> j : w):\n")
			for i := 0; i < g.n; i++ {
				for j := 0; j < g.n; j++ {
					if g.w[i][j] > 0 {
						fmt.Printf("  %d -> %d : %g\n", i, j, g.w[i][j])
					}
				}
			}
			fmt.Printf("m=%g gamma=%g\n", g.m, c.gamma)
			fmt.Printf("converged partition: %v\n", p)
			fmt.Printf("community C=%v, subset S=%v, T=C\\S=%v\n", comm, s, t)
			fmt.Printf("e(S,T)+e(T,S) = %g < (gamma/m)(KoutS*KinT+KoutT*KinS) = %g\n", lhs, rhs)
			q := make([]int, len(p))
			copy(q, p)
			freshLabel := 0
			for _, cc := range p {
				if cc >= freshLabel {
					freshLabel = cc + 1
				}
			}
			for _, i := range s {
				q[i] = freshLabel
			}
			fmt.Printf("from-scratch split gain: %g (positive = split improves)\n",
				g.directedQ(c.gamma, q)-g.directedQ(c.gamma, p))
			return true
		}
	}
	return false
}

func main() {
	hunt := flag.Bool("hunt-subset", false,
		"search small graphs for a minimal subset-optimality counterexample and print it")
	flag.Parse()
	selfTest()
	if *hunt {
		huntSubsetFixture()
		return
	}

	grid := struct {
		ns        []int
		densities []float64
		weights   []string
		regimes   []string
		loops     []bool
		gammas    []float64
		reps      int
	}{
		ns:        []int{5, 8, 12, 16, 24, 40},
		densities: []float64{0.1, 0.3, 0.5},
		weights:   []string{"unit", "int", "cont"},
		regimes:   []string{"uniform", "dag", "srcsink", "cycle", "symmetric"},
		loops:     []bool{false, true},
		gammas:    []float64{0.5, 1, 2},
		reps:      2,
	}

	total := stats{sepMinSlack: math.Inf(1), subMinSlack: math.Inf(1)}
	sym := stats{sepMinSlack: math.Inf(1), subMinSlack: math.Inf(1)} // symmetric regime only (warm-up)

	report := func(kind string, c config, detail string) {
		fmt.Printf("!! %s at [%s] seed=%d: %s\n", kind, c, c.seed(), detail)
	}

	for _, n := range grid.ns {
		for _, dens := range grid.densities {
			for _, w := range grid.weights {
				for _, regime := range grid.regimes {
					for _, loops := range grid.loops {
						for _, gamma := range grid.gammas {
							for rep := 0; rep < grid.reps; rep++ {
								c := config{n, dens, w, regime, loops, gamma, rep}
								runOne(c, &total, &sym, report)
							}
						}
					}
				}
			}
		}
	}

	printStats := func(name string, s *stats) {
		fmt.Printf("\n== %s ==\n", name)
		fmt.Printf("samples: %d (discarded: %d)\n", s.samples, s.discarded)
		fmt.Printf("scout-Q vs meso-Q: %d mismatches, max |err| = %.3g\n", s.qualityMismatch, s.maxQualityErr)
		fmt.Printf("convergence driving: %d samples needed driving (%d moves, %d merges applied; %d samples had merge gaps)\n",
			s.drivenSamples, s.driveMoves, s.driveMerges, s.mergeGapSamples)
		fmt.Printf("connectivity (raw output, %d communities): weak violations = %d, strong failures = %d\n",
			s.commsChecked, s.weakViol, s.strongFail)
		fmt.Printf("gamma-separation: raw violations = %d, converged violations = %d, min slack = %.3g, max merge-gain err = %.3g\n",
			s.sepViolRaw, s.sepViolConverged, s.sepMinSlack, s.maxMergeGainErr)
		fmt.Printf("subset-optimality (converged): %d violations in %d communities across %d samples, min slack = %.3g, sampled=%v, max split-gain err = %.3g\n",
			s.subViol, s.subViolComms, s.subViolSamples, s.subMinSlack, s.subSampledAny, s.maxSplitGainErr)
	}
	printStats("ALL REGIMES", &total)
	printStats("SYMMETRIC ONLY (undirected modularity warm-up)", &sym)
}

// runOne executes one grid cell end to end and folds its results into the
// accumulators.
func runOne(c config, total, sym *stats, report func(kind string, c config, detail string)) {
	rng := rand.New(rand.NewPCG(c.seed(), c.seed()))
	g := generate(c, rng)

	p, mesoQ, err := runLeiden(g, c.gamma, c.seed())
	if err != nil {
		report("RUN ERROR", c, err.Error())
		total.discarded++
		return
	}

	accs := []*stats{total}
	if c.regime == "symmetric" {
		accs = append(accs, sym)
	}
	for _, s := range accs {
		s.samples++
	}

	// Convention guard: the scout's independent Q must match meso's reported Q
	// on the returned partition.
	scoutQ := g.directedQ(c.gamma, p)
	if e := math.Abs(scoutQ - mesoQ); e > qualityTol {
		report("QUALITY MISMATCH", c, fmt.Sprintf("scout %v vs meso %v", scoutQ, mesoQ))
		for _, s := range accs {
			s.qualityMismatch++
		}
	}
	for _, s := range accs {
		if e := math.Abs(scoutQ - mesoQ); e > s.maxQualityErr {
			s.maxQualityErr = e
		}
	}

	// Connectivity is judged at meso's raw output: it is a claim about what
	// Leiden returns, not about a scout-driven partition.
	rawComms := communities(p)
	for _, s := range accs {
		s.commsChecked += len(rawComms)
	}
	for _, comm := range rawComms {
		if !g.weaklyConnected(comm) {
			report("WEAK CONNECTIVITY VIOLATION", c, fmt.Sprintf("community %v", comm))
			for _, s := range accs {
				s.weakViol++
			}
		}
		if !g.stronglyConnected(comm) {
			for _, s := range accs {
				s.strongFail++
			}
		}
	}

	// Gamma-separation at the raw output (is meso level-stable as returned?).
	sepRaw, _, _ := g.gammaSepCheck(c.gamma, p, rawComms)
	for _, s := range accs {
		s.sepViolRaw += sepRaw
	}

	// Drive to a verified fixed point; bound judgments below are only made at
	// converged partitions (a violation at a non-converged partition is a
	// convergence artifact, not a counterexample).
	moves, merges, capped := driveConverged(g, c.gamma, p)
	if capped {
		report("DRIVE CAP HIT", c, "sample discarded")
		for _, s := range accs {
			s.discarded++
			s.samples--
		}
		return
	}
	for _, s := range accs {
		if moves+merges > 0 {
			s.drivenSamples++
		}
		s.driveMoves += moves
		s.driveMerges += merges
		if merges > 0 {
			s.mergeGapSamples++
		}
	}

	comms := communities(p)
	sepViol, sepSlack, mergeErr := g.gammaSepCheck(c.gamma, p, comms)
	if sepViol > 0 {
		report("GAMMA-SEPARATION VIOLATION (converged)", c, fmt.Sprintf("%d pairs, min slack %v", sepViol, sepSlack))
	}
	if mergeErr > 1e-6 {
		report("MERGE-GAIN FORMULA MISMATCH", c, fmt.Sprintf("max err %v", mergeErr))
	}
	subViol, subComms, subSlack, subSampled, splitErr := g.subsetCheck(c.gamma, p, comms, rng)
	if subViol > 0 {
		report("SUBSET-OPTIMALITY VIOLATION (converged)", c,
			fmt.Sprintf("%d subsets in %d communities, min slack %v", subViol, subComms, subSlack))
	}
	if splitErr > 1e-6 {
		report("SPLIT-GAIN FORMULA MISMATCH", c, fmt.Sprintf("max err %v", splitErr))
	}

	for _, s := range accs {
		s.sepViolConverged += sepViol
		if sepSlack < s.sepMinSlack {
			s.sepMinSlack = sepSlack
		}
		if mergeErr > s.maxMergeGainErr {
			s.maxMergeGainErr = mergeErr
		}
		s.subViol += subViol
		s.subViolComms += subComms
		if subViol > 0 {
			s.subViolSamples++
		}
		if subSlack < s.subMinSlack {
			s.subMinSlack = subSlack
		}
		s.subSampledAny = s.subSampledAny || subSampled
		if splitErr > s.maxSplitGainErr {
			s.maxSplitGainErr = splitErr
		}
	}
}
