package meso

// nodeQueue is the FIFO work-list that drives Leiden's fast local-move phase. It
// holds node indices in [0, n) and deduplicates: a node already waiting is not
// enqueued a second time, so the queue never holds two copies of the same node.
// This is the "in-queue bit set" of the design (section 4.1 phase 1): push checks
// and sets the mark, pop clears it, so a node can be re-enqueued once it has been
// processed but never while it is still pending.
//
// items grows append-only and head walks forward; a node is live while
// head <= its slot < len(items). Total pushes are bounded (see localMoveQueue),
// so the backing slice does not grow without bound over a run.
type nodeQueue struct {
	items  []int
	head   int
	queued []bool
}

// newNodeQueue returns an empty queue sized for n nodes (indices 0..n-1).
func newNodeQueue(n int) *nodeQueue {
	return &nodeQueue{queued: make([]bool, n)}
}

// push enqueues v unless it is already waiting, reporting whether it was added.
// The boolean lets the caller enqueue exactly the currently un-queued neighbours
// of a moved node without a separate membership check.
func (q *nodeQueue) push(v int) bool {
	if q.queued[v] {
		return false
	}
	q.queued[v] = true
	q.items = append(q.items, v)
	return true
}

// pop removes and returns the front node, clearing its in-queue mark so it may be
// enqueued again later. It must not be called on an empty queue.
func (q *nodeQueue) pop() int {
	v := q.items[q.head]
	q.head++
	q.queued[v] = false
	return v
}

// empty reports whether every enqueued node has been popped.
func (q *nodeQueue) empty() bool {
	return q.head >= len(q.items)
}

// localMoveDrain runs one queue-driven pass of the fast local-move phase: every
// node starts enqueued in ascending index order, and each popped node is
// reassigned to its best-gain target (best neighbour community or the fresh
// isolation label *fresh) when the gain exceeds moveImproveEps. A node that moves
// re-enqueues its neighbours that are not already waiting; a node already in the
// queue is never duplicated (nodeQueue.push). The pass ends when the queue drains.
// It mutates p in place, advances *fresh past every isolation label it consumes,
// and reports whether any node moved and how many nodes were popped.
//
// Because only an accepted move re-enqueues nodes, a node is re-examined exactly
// when one of its neighbours moves - far fewer visits than Louvain's blanket
// re-sweep of every node each round. Neighbour re-enqueue alone does not settle
// nodes whose gain shifts only through a community's aggregate degree/size (a
// non-adjacent node joining or leaving their community), which is why the fixed
// point needs the outer loop in localMoveQueue.
//
// Monotone: every applied move strictly raises the objective and is applied
// against the running p, so a drain never lowers quality - the Go image of
// localMoveRun_monotone; the queue only reorders the accepted moves.
func localMoveDrain(g *csr, obj objective, p Partition, fresh *int) (moved bool, pops int) {
	q := newNodeQueue(g.numNodes())
	for u := 0; u < g.numNodes(); u++ {
		q.push(u)
	}

	for !q.empty() {
		u := q.pop()
		pops++

		target, delta := bestMove(g, obj, p, u, *fresh)
		if delta <= moveImproveEps || target == p[u] {
			continue
		}

		p[u] = target
		if target == *fresh {
			*fresh++
		}
		moved = true

		for _, v := range g.neighbors(u) {
			q.push(v)
		}
	}
	return moved, pops
}

// leidenLevelSeed builds the initial partition for the next level's local move:
// each aggregate super-node (a refined sub-community) is seeded with the
// NON-refined phase-1 community it belongs to. superOf maps each base node to its
// super-node (the community-to-dense-index relabel aggregate returns) and
// nonRefined is the phase-1 partition; k is the aggregate node count.
//
// This is the correctness-critical subtlety of design section 4.1 phase 3: the
// aggregate network is built from the refined sub-communities, but its initial
// partition comes from the non-refined phase-1 assignment. Refinement only merges
// nodes that already share a phase-1 community, so every base node mapping to one
// super-node carries the same nonRefined label; the first one seen fixes the
// super-node's seed. The result is canonicalised to dense labels in [0, k).
func leidenLevelSeed(superOf []int, nonRefined Partition, k int) Partition {
	seed := make(Partition, k)
	set := make([]bool, k)
	for i, a := range superOf {
		if !set[a] {
			seed[a] = nonRefined[i]
			set[a] = true
		}
	}
	return canonicalize(seed)
}

// localMoveQueue runs the fast local-move phase of Leiden to a fixed point: the
// queue-driven replacement for Louvain's localMoveToStable sweep loop. It drains
// the neighbour-re-enqueue queue (localMoveDrain) repeatedly and stops when a
// whole drain applies no move, leaving p local-move-stable: no single-node
// reassignment strictly improves the objective (IsLocalMoveStable, the same
// stopping condition the exhaustive sweep reaches). It mutates p in place and
// reports whether any node moved and the total nodes popped across all drains.
//
// A clean drain pops every node once against a fixed p and moves none, which is
// exactly a no-move exhaustive sweep - so the queue and the sweep stop at the same
// fixed-point condition. A single drain does not suffice because a node's gain can
// shift through a community aggregate changed by a non-adjacent node, which
// neighbour re-enqueue never revisits; the extra drains catch that residue.
//
// Termination is the Go image of no_infinite_acceptedMove_run: every applied move
// strictly raises the objective by more than moveImproveEps, and the objective is
// bounded above over the finite set of node groupings, so only finitely many moves
// are ever accepted. Each drain past the first therefore either accepts at least
// one such move or is the terminal clean drain, and every drain pops a bounded
// number of nodes, so the total pop count is finite. The isolation labels grow
// without bound, but the objective depends on p only through its grouping, so that
// does not threaten termination.
func localMoveQueue(g *csr, obj objective, p Partition) (moved bool, pops int) {
	fresh := g.numNodes()
	for _, c := range p {
		if c >= fresh {
			fresh = c + 1
		}
	}

	for {
		drainMoved, drainPops := localMoveDrain(g, obj, p, &fresh)
		pops += drainPops
		if !drainMoved {
			return moved, pops
		}
		moved = true
	}
}

// leiden runs serial Leiden - the three phases of design section 4.1 iterated to
// stability - and returns the final community assignment of the base graph as a
// well-formed, canonically-labelled Partition. Each level:
//
//  1. Fast local moving (localMoveQueue) from the seeded partition, giving the
//     non-refined phase-1 partition p; the level's community structure is read off
//     p, lifted to the base graph.
//  2. Refinement (refine) within p's communities, giving the refined
//     sub-communities.
//  3. Aggregation over the refined sub-communities, with the aggregate's initial
//     partition seeded from the NON-refined p (leidenLevelSeed). Recurse from
//     phase 1 on the aggregate.
//
// The first level seeds phase 1 from singletons; each later level seeds it from
// the previous non-refined partition lifted onto the aggregate, so local moving
// resumes from the phase-1 grouping rather than from scratch. Refinement
// randomness is derived per node from seed (nodeSeed), so the result is a pure
// function of (g, obj, seed) and byte-identical across runs.
//
// Termination: the recursion stops when phase-1 merged nothing (its partition is
// discrete, the negation of the RunningLevelStep guard, as in louvainTrace) or
// when refinement does not shrink the graph (the aggregate has as many nodes as
// the working graph, so no further level could make progress). Every continued
// level strictly shrinks the working graph, so at most n levels are taken for n
// input nodes.
func leiden(g *csr, obj objective, seed uint64) Partition {
	return leidenWith(g, obj, seed, serialLeidenMover, singleton(g.numNodes()))
}

// serialLeidenMover is the default fast local-move phase for Leiden: the
// queue-driven drain to a fixed point (localMoveQueue). leidenWith takes it as a
// parameter so the synchronous-round parallel phase (parallelMover) can be
// swapped in for the parallel run without duplicating the three-phase loop.
func serialLeidenMover(g *csr, obj objective, p Partition) { localMoveQueue(g, obj, p) }

// leidenParallel runs Leiden with the synchronous-round parallel local-move
// phase (parallelLocalMoveToStable over workers goroutines) in place of the
// queue-driven phase 1, and returns the final base-graph partition. Refinement
// and aggregation are unchanged - refinement randomness is per-node seeded, so
// it too is scheduling-independent - so the whole run is a pure function of
// (g, obj, seed, workers) and byte-identical across core counts.
func leidenParallel(g *csr, obj objective, seed uint64, workers int) Partition {
	return leidenWith(g, obj, seed, parallelMover(workers), singleton(g.numNodes()))
}

// passSeed derives the refinement seed of a Leiden pass. Pass 0 uses the
// caller's seed unchanged, so a single-pass run is byte-identical to the
// pre-iterations behaviour; later passes derive a fresh seed through the same
// avalanche refinement uses per node. A pass re-run with an unchanged seed
// would make identical refinement draws and be a guaranteed no-op, so the
// per-pass derivation is what makes extra passes worth running at all.
func passSeed(seed uint64, pass int) uint64 {
	if pass == 0 {
		return seed
	}
	return nodeSeed(seed, pass)
}

// leidenIteratedLevels runs iterations full Leiden passes, each starting from
// the previous pass's base partition with its own derived refinement seed
// (passSeed), and returns the levels of the final pass, in order; the last is
// the run's result. This is the paper's outer iteration: a converged partition
// re-enters phase 1 at base granularity, so refinement gets to re-divide the
// final communities from singletons with fresh randomness, which a single pass
// never does after aggregation freezes its sub-community granularity. Every
// pass applies only improving steps from the partition it starts from, so
// quality is non-decreasing across passes, and the whole run stays a pure
// function of (g, obj, seed, iterations). Earlier passes only seed the next
// one, so their levels are not kept.
func leidenIteratedLevels(g *csr, obj objective, seed uint64, iterations int, mv localMover) []Partition {
	levels := []Partition{singleton(g.numNodes())}
	for pass := range iterations {
		levels = leidenLevelsWith(g, obj, passSeed(seed, pass), mv, levels[len(levels)-1])
	}
	return levels
}

// leidenWith runs the three-phase Leiden level loop using mv as the phase-1
// fast local-move, starting from the base-graph partition initial. It is
// leiden parameterised over that phase so the serial queue and the parallel
// synchronous-round mover share one driver; refinement (phase 2), aggregation
// and non-refined seeding (phase 3), the guards, and termination are identical
// for both. A pass from singletons is the classic single run; a pass from a
// previous result is one outer Leiden iteration (leidenIteratedLevels). initial is
// not mutated.
func leidenWith(g *csr, obj objective, seed uint64, mv localMover, initial Partition) Partition {
	levels := leidenLevelsWith(g, obj, seed, mv, initial)
	return levels[len(levels)-1]
}

// leidenLevelsWith is leidenWith reporting every level: the non-refined phase-1
// partition of each level lifted to the base graph, in order, the last being
// the pass's result. Because later levels aggregate the refined
// sub-communities, a level need not nest inside the next.
func leidenLevelsWith(g *csr, obj objective, seed uint64, mv localMover, initial Partition) []Partition {
	n := g.numNodes()
	baseOf := make([]int, n) // base node -> its node in the current working graph
	for i := range baseOf {
		baseOf[i] = i
	}

	h := g
	initP := initial
	var levels []Partition
	for {
		// Phase 1: fast local moving from the seeded partition.
		p := make(Partition, len(initP))
		copy(p, initP)
		mv(h, obj, p)
		p = canonicalize(p)

		// The level's communities come from the NON-refined phase-1 partition,
		// lifted to the base graph.
		base := make(Partition, n)
		for i := range base {
			base[i] = p[baseOf[i]]
		}
		levels = append(levels, canonicalize(base))

		if numCommunities(p) == h.numNodes() {
			break // phase-1 merged nothing; the partition is discrete.
		}

		// Phase 2: refinement within the phase-1 communities.
		refined := refine(h, obj, p, seed)

		// Phase 3: aggregate over the refined sub-communities.
		aggG, superOf := aggregate(h, refined)
		if aggG.numNodes() >= h.numNodes() {
			break // refinement did not shrink the graph; no further level can.
		}

		// Seed the aggregate's initial partition from the NON-refined partition.
		initP = leidenLevelSeed(superOf, p, aggG.numNodes())
		for i := range baseOf {
			baseOf[i] = superOf[baseOf[i]]
		}
		h = aggG
	}
	return levels
}
