package meso

import "sort"

// objective is the internal optimisation interface Louvain (and, later, Leiden)
// runs against: a [QualityFunction] that also exposes the incremental move-delta
// of reassigning one node to a candidate community. modularity and cpm are its
// instances. The public constructors [Modularity] and [CPM] hand callers the
// QualityFunction view; the optimizers narrow it to objective internally, so the
// move loop is written once against both. Unifying the two deltas behind one
// interface is deferred from the delta tickets to here on purpose (see
// docs/specs/learnings.md): doing it earlier would have broken the build order.
type objective interface {
	QualityFunction
	moveDelta(g *csr, p Partition, u, target int) float64
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
	seen := make(map[int]struct{}, len(p))
	for _, c := range p {
		seen[c] = struct{}{}
	}
	return len(seen)
}

// canonicalize relabels p to dense community indices in [0, numCommunities(p)),
// numbering communities by ascending original label so the result is
// order-independent. The returned partition induces the same grouping as p (so
// its quality is unchanged) and is well-formed: every label lies in [0, len(p)),
// even when p carried the out-of-range isolation labels the local-move phase
// introduces.
func canonicalize(p Partition) Partition {
	dense := make(map[int]int, len(p))
	for _, c := range p {
		dense[c] = 0
	}
	labels := make([]int, 0, len(dense))
	for c := range dense {
		labels = append(labels, c)
	}
	sort.Ints(labels)
	for d, c := range labels {
		dense[c] = d
	}
	out := make(Partition, len(p))
	for i, c := range p {
		out[i] = dense[c]
	}
	return out
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
func bestMove(g *csr, obj objective, p Partition, u, isolateLabel int) (int, float64) {
	src := p[u]

	cands := make([]int, 0, len(g.neighbors(u))+1)
	seen := make(map[int]struct{})
	addCand := func(c int) {
		if c == src {
			return
		}
		if _, ok := seen[c]; ok {
			return
		}
		seen[c] = struct{}{}
		cands = append(cands, c)
	}
	for _, v := range g.neighbors(u) {
		addCand(p[v])
	}
	if g.directed {
		for _, v := range g.inNeighbors(u) {
			addCand(p[v])
		}
	}
	addCand(isolateLabel)
	sort.Ints(cands)

	bestTarget, bestDelta := src, 0.0
	for _, c := range cands {
		if d := obj.moveDelta(g, p, u, c); d > bestDelta {
			bestDelta, bestTarget = d, c
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
	for u := 0; u < g.numNodes(); u++ {
		target, delta := bestMove(g, obj, p, u, *fresh)
		if delta > moveImproveEps && target != p[u] {
			p[u] = target
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
