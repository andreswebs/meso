package meso

import (
	"cmp"
	"slices"
	"sort"
	"sync"
)

// objective is the internal optimisation interface Louvain and Leiden run
// against: a [QualityFunction] that also prices the reassignment of one node to
// a candidate community. moveDelta prices a single move from scratch, at a cost
// linear in the graph; newPricer returns the [pricer] the optimizers use
// instead, which keeps the per-community totals a move-delta needs so each
// candidate costs only the moved node's degree. modularity, cpm and
// directedModularity are its instances. The public constructors [Modularity] and [CPM] hand callers the
// QualityFunction view; the optimizers narrow it to objective internally, so the
// move loop is written once against both. Unifying the two deltas behind one
// interface is deferred from the delta tickets to here on purpose (see
// docs/specs/learnings.md): doing it earlier would have broken the build order.
type objective interface {
	QualityFunction
	moveDelta(g *csr, p Partition, u, target int) float64
	newPricer(g *csr, p Partition) pricer
}

var (
	_ objective = modularity{}
	_ objective = cpm{}
	_ objective = directedModularity{}
)

// moveImproveEps is the strict-improvement threshold for accepting a local move:
// a move is applied only when its incremental delta exceeds it. A true no-op
// contaminated by float rounding (delta on the order of 1e-14 at corpus scale)
// is therefore never accepted, which is what stops float noise from
// ping-ponging a node between two communities forever and breaking termination.
// It sits well above the observed rounding floor and below both any real quality
// gap on the target graphs and the tolerance the stability tests assert, so a
// partition this loop calls stable is stable under those checks too.
const moveImproveEps = 1e-12

// singleton returns the discrete partition of k nodes: each node alone in its
// own community, node i labelled i. It is the Go image of the Lean singleton
// (fun A => (A : ℕ)) and the starting partition of every local-move phase.
func singleton(k int) Partition {
	p := make(Partition, k)
	for i := range p {
		p[i] = i
	}
	return p
}

// numCommunities returns the number of distinct community labels in p, the Go
// image of the Lean numComm. It equals len(p) exactly when p is the discrete
// partition (every node its own community) - the fixed point at which the
// multilevel loop stops.
func numCommunities(p Partition) int {
	_, k := denseLabels(p)
	return k
}

// denseLabels maps every node's label to its rank among the distinct labels of
// p, taken in ascending order, and returns those ranks with the number of
// distinct labels. Ranking by ascending label makes the result independent of
// node order.
//
// Labels are usually small non-negative integers (at most n starting labels
// plus the isolation labels a serial pass mints), so the ranks come from a
// table indexed by label. A partition whose labels range too widely for such a
// table - the parallel round hands every node its own isolation label past the
// largest one in use - is ranked by sorting its distinct labels instead.
func denseLabels(p Partition) (dense []int, k int) {
	dense = make([]int, len(p))
	lo, hi := 0, -1
	for _, c := range p {
		lo, hi = min(lo, c), max(hi, c)
	}

	if lo >= 0 && hi < 4*len(p)+16 {
		rank := make([]int, hi+1)
		for _, c := range p {
			rank[c] = 1
		}
		for c, present := range rank {
			if present != 0 {
				rank[c] = k
				k++
			}
		}
		for i, c := range p {
			dense[i] = rank[c]
		}
		return dense, k
	}

	rank := make(map[int]int, len(p))
	for _, c := range p {
		rank[c] = 0
	}
	labels := make([]int, 0, len(rank))
	for c := range rank {
		labels = append(labels, c)
	}
	sort.Ints(labels)
	for d, c := range labels {
		rank[c] = d
	}
	for i, c := range p {
		dense[i] = rank[c]
	}
	return dense, len(labels)
}

// canonicalize relabels p to dense community indices in [0, numCommunities(p)),
// numbering communities by ascending original label so the result is
// order-independent. The returned partition induces the same grouping as p (so
// its quality is unchanged) and is well-formed: every label lies in [0, len(p)),
// even when p carried the out-of-range isolation labels the local-move phase
// introduces.
func canonicalize(p Partition) Partition {
	dense, _ := denseLabels(p)
	return dense
}

// bestMove returns the community that most improves the objective for node u
// under partition p, together with that improvement. The candidates are u's
// current community (staying put, delta 0), the community of each neighbour, and
// isolateLabel (splitting u into a fresh singleton). Distinct neighbour
// communities are examined in ascending label order so ties resolve
// deterministically to the smallest label, independent of adjacency order.
//
// Staying put is always a candidate with delta 0, so the returned delta is never
// negative - the Go image of move_best_ge. Including isolation makes the search
// exhaustive up to sign: moving u into a community it has no edge to is never
// better than isolating it (the edge term is zero and the null-model penalty
// only grows), so a partition with no improving neighbour-or-isolate move has no
// improving move at all - IsLocalMoveStable in the strong, all-targets sense the
// convergence guarantees are stated from.
//
// On a directed graph the candidate communities are gathered from both u's
// out-neighbours and its in-neighbours: a directed arc couples u to a community
// in either direction, and the directed move-delta prices both, so a community
// reached only through an in-arc must still be considered. On an undirected graph
// the in-adjacency mirrors the out-adjacency, so the in pass adds nothing new and
// the behaviour is unchanged.
func bestMove(g *csr, pr pricer, p Partition, u, isolateLabel int) (int, float64) {
	s := moveScratchPool.Get().(*moveScratch)
	defer moveScratchPool.Put(s)
	return s.bestMove(g, pr, p, u, isolateLabel)
}

// moveScratchPool lends working storage to the package-level bestMove, which
// the parallel round calls from many goroutines with no scratch of its own to
// pass. A scratch is held by one goroutine from Get to Put, so workers never
// share one.
var moveScratchPool = sync.Pool{New: func() any { return new(moveScratch) }}

// communityLink is the weight between the node being priced and one community.
type communityLink struct {
	label int
	w     linkWeight
}

// moveScratch is the working storage of bestMove, kept between calls so a
// serial local-move pass prices every node without allocating. It must not be
// shared between goroutines; the zero value is ready to use.
type moveScratch struct {
	links []communityLink
}

// bestMove is the package-level bestMove using s for working storage.
//
// It gathers u's link weight to every neighbouring community in one pass over
// u's adjacency, then prices each candidate from those sums (pricer.price), so
// a node costs O(d log d) for degree d however many communities its neighbours
// span. Each arc is recorded with its community label and the records are
// stably sorted by label, so the arcs into one community are summed in
// adjacency order - the order a per-candidate rescan would use - and the
// candidates come out in ascending label order.
func (s *moveScratch) bestMove(g *csr, pr pricer, p Partition, u, isolateLabel int) (int, float64) {
	src := p[u]

	links := s.links[:0]
	nw := g.neighborWeights(u)
	for k, v := range g.neighbors(u) {
		links = append(links, communityLink{label: p[v], w: linkWeight{out: nw[k]}})
	}
	if g.directed {
		inW := g.inNeighborWeights(u)
		for k, v := range g.inNeighbors(u) {
			links = append(links, communityLink{label: p[v], w: linkWeight{in: inW[k]}})
		}
	}
	// Isolation is a candidate with no link weight; listing it here slots it
	// into the ascending label order alongside the neighbour communities.
	links = append(links, communityLink{label: isolateLabel})
	slices.SortStableFunc(links, func(a, b communityLink) int { return cmp.Compare(a.label, b.label) })

	// Fold each run of equal labels into its first record.
	k := 0
	for _, l := range links[1:] {
		if l.label == links[k].label {
			links[k].w.out += l.w.out
			links[k].w.in += l.w.in
			continue
		}
		k++
		links[k] = l
	}
	links = links[:k+1]
	s.links = links

	var toSrc linkWeight
	for i := range links {
		if !g.directed {
			// The in-adjacency of an undirected graph is its out-adjacency.
			links[i].w.in = links[i].w.out
		}
		if links[i].label == src {
			toSrc = links[i].w
		}
	}

	bestTarget, bestDelta := src, 0.0
	for _, l := range links {
		if l.label == src {
			continue
		}
		if d := pr.price(u, src, l.label, toSrc, l.w); d > bestDelta {
			bestDelta, bestTarget = d, l.label
		}
	}
	return bestTarget, bestDelta
}

// localMoveSweep performs one pass of the fast local-move phase: it visits every
// node once in ascending index order and, for each, applies its best improving
// move (best neighbour community or isolation) when the improvement exceeds
// moveImproveEps, updating p in place. It reports whether any node moved.
//
// Each applied move strictly raises the objective, and moves are applied against
// the running p, so a sweep never lowers quality - the Go image of
// localMoveRun_monotone. fresh threads the next unused label for isolation
// across sweeps so split-off singletons never collide; it is advanced whenever a
// node is isolated.
func localMoveSweep(g *csr, obj objective, p Partition, fresh *int) bool {
	moved := false
	pr := obj.newPricer(g, p)
	var scratch moveScratch
	for u := 0; u < g.numNodes(); u++ {
		target, delta := scratch.bestMove(g, pr, p, u, *fresh)
		if src := p[u]; delta > moveImproveEps && target != src {
			p[u] = target
			pr.moved(u, src, target)
			if target == *fresh {
				*fresh++
			}
			moved = true
		}
	}
	return moved
}

// localMoveToStable runs local-move sweeps until a whole sweep applies no move,
// leaving p at a local-move-stable partition: no single-node reassignment
// strictly improves the objective (IsLocalMoveStable). It reports whether any
// move was applied.
//
// Termination is the Go image of no_infinite_acceptedMove_run: every applied
// move strictly raises the objective, which takes finitely many values over the
// finite set of node groupings, so only finitely many moves can be applied. The
// isolation labels grow without bound, but the objective depends on p only
// through its grouping, so that does not threaten termination.
func localMoveToStable(g *csr, obj objective, p Partition) bool {
	fresh := g.numNodes()
	for _, c := range p {
		if c >= fresh {
			fresh = c + 1
		}
	}
	everMoved := false
	for localMoveSweep(g, obj, p, &fresh) {
		everMoved = true
	}
	return everMoved
}

// louvainLevel records one level of the multilevel recursion, for the invariant
// and termination checks: the base-graph partition produced up to and including
// this level, the node count of the working graph the level's local move ran on,
// and whether that phase merged anything (its partition is non-discrete). merged
// is the negation of the RunningLevelStep guard (CORRESPONDENCE.md section 4):
// the loop takes another level only while it is true.
type louvainLevel struct {
	base    Partition
	working int
	merged  bool
}

// louvainTrace runs serial Louvain - local-move sweeps to a fixed point, then
// aggregation, recursing - and returns one louvainLevel per level taken. It is
// the guarded outer loop of CORRESPONDENCE.md section 4: after each level's
// local-move phase it stops as soon as the phase merged nothing (the working
// partition is discrete), rather than aggregating unconditionally. That stop
// test is the negation of the RunningLevelStep guard, so termination is
// inherited (no_infinite_runningLevel_run): each running level strictly shrinks
// the working graph (RunningLevelStep.size_lt), so the loop takes at most n
// levels for n input nodes.
func louvainTrace(g *csr, obj objective) []louvainLevel {
	return louvainTraceWith(g, obj, serialLouvainMover)
}

// serialLouvainMover is the default local-move phase for Louvain: sweep to a
// fixed point (localMoveToStable). louvainTraceWith takes it as a parameter so
// the synchronous-round parallel phase (parallelMover) can be substituted for
// the parallel run without duplicating the level loop.
func serialLouvainMover(g *csr, obj objective, p Partition) { localMoveToStable(g, obj, p) }

// louvainTraceWith runs the Louvain level loop using mv as the local-move phase.
// It is louvainTrace parameterised over the phase-1 mover so the serial sweep
// and the parallel synchronous-round mover share one aggregation-and-recursion
// driver; the guard, base lifting, and termination are identical for both.
func louvainTraceWith(g *csr, obj objective, mv localMover) []louvainLevel {
	n := g.numNodes()
	baseOf := make([]int, n) // base node -> its node in the current working graph
	for i := range baseOf {
		baseOf[i] = i
	}

	h := g
	var trace []louvainLevel
	for {
		p := singleton(h.numNodes())
		mv(h, obj, p)
		merged := numCommunities(p) < h.numNodes()

		base := make(Partition, n)
		for i := range base {
			base[i] = p[baseOf[i]]
		}
		trace = append(trace, louvainLevel{base: canonicalize(base), working: h.numNodes(), merged: merged})

		if !merged {
			break
		}

		aggG, superOf := aggregate(h, p)
		for i := range baseOf {
			baseOf[i] = superOf[baseOf[i]]
		}
		h = aggG
	}
	return trace
}

// louvainLevels returns the base-graph partition produced at each level of the
// serial Louvain run, in order; the last is the final result. Quality is
// non-decreasing along the slice (quality_monotone_of_stepwise).
func louvainLevels(g *csr, obj objective) []Partition {
	trace := louvainTrace(g, obj)
	levels := make([]Partition, len(trace))
	for i, lv := range trace {
		levels[i] = lv.base
	}
	return levels
}

// louvain runs serial Louvain (local moving plus aggregation, no refinement) and
// returns the final community assignment of the base graph as a well-formed,
// canonically-labelled Partition. It is the internal differential baseline
// Leiden layers refinement on, and a deterministic community-detection algorithm
// in its own right: node order, tie-breaking, and aggregation are all canonical,
// so the result is fixed without any randomness.
func louvain(g *csr, obj objective) Partition {
	trace := louvainTrace(g, obj)
	return trace[len(trace)-1].base
}

// louvainParallel runs Louvain with the synchronous-round parallel local-move
// phase (parallelLocalMoveToStable over workers goroutines) in place of the
// serial sweep, and returns the final base-graph partition. Aggregation and the
// level loop are unchanged and already deterministic, so the whole run is a pure
// function of (g, obj, workers) and byte-identical across core counts. Because
// synchronous rounds reach a different local-move fixed point than the serial
// sweep, its partition may differ from louvain's; both are valid.
func louvainParallel(g *csr, obj objective, workers int) Partition {
	trace := louvainTraceWith(g, obj, parallelMover(workers))
	return trace[len(trace)-1].base
}
