package meso

import (
	"runtime"
	"sync"
)

// applyRound is the closed form of a synchronous local-move round
// (verification/lean/Meso/Round.lean, applyRound_eq): given the round-start
// snapshot p, each node's snapshot-decided target t, and the set of moved nodes
// marked in moved, it returns the partition in which every moved node sits at
// its target t[w] and every other node stays at p[w].
//
// The Lean model folds the single-node reassignments move(·, v, t[v]) over a
// work-list; because distinct-node moves commute and same-node moves are
// idempotent, that fold collapses to this mask. Encoding the round as the mask
// rather than an ordered fold makes its two guarantees structural: the outcome
// cannot depend on the order the moves are applied (applyRound_perm) because no
// order is representable, and it is a pure function of (p, t, moved set) alone.
// @ requires len(t) == len(p) && len(moved) == len(p)
// @ requires forall i int :: 0 <= i && i < len(p) ==> acc(&p[i], _) && acc(&t[i], _) && acc(&moved[i], _)
func applyRound(p Partition, t []int, moved []bool) Partition {
	q := make(Partition, len(p))
	// @ invariant len(q) == len(p)
	// @ invariant forall i int :: 0 <= i && i < len(p) ==> acc(&p[i], _) && acc(&t[i], _) && acc(&moved[i], _)
	// @ invariant forall i int :: 0 <= i && i < len(q) ==> acc(&q[i])
	for w := range p {
		if moved[w] {
			q[w] = t[w]
		} else {
			q[w] = p[w]
		}
	}
	return q
}

// normalizeWorkers clamps a requested worker count to something sensible for a
// graph of n nodes: a non-positive request means "use every core"
// (runtime.GOMAXPROCS), and no run needs more workers than it has nodes to hand
// out or fewer than one. The result is named so the postcondition can bind it;
// the bound discharges the division by workers in parallelBestMoves.
// @ requires 1 <= n
// @ ensures 1 <= res && res <= n
func normalizeWorkers(workers, n int) (res int) {
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	if workers > n {
		workers = n
	}
	if workers < 1 {
		workers = 1
	}
	return workers
}

// isolationBase returns a community label strictly greater than every label in
// p and at least n, so isolationBase(p, n) + u is a distinct fresh label for
// every node u in [0, n). The parallel round hands each node its own isolation
// target this way: two nodes that both split off in the same round land in
// different fresh singletons rather than colliding, and the choice is a pure
// function of the snapshot, not of the schedule (contrast the serial sweep's
// shared, sequentially-advanced fresh counter, which cannot be evaluated
// independently per node).
// @ requires acc(partitionMem(p), _)
func isolationBase(p Partition, n int) int {
	base := n
	// @ unfold acc(partitionMem(p), _)
	// @ invariant forall i int :: 0 <= i && i < len(p) ==> acc(&p[i], _)
	for _, c := range p {
		if c >= base {
			base = c + 1
		}
	}
	return base
}

// bestMovesRange computes the snapshot best move for every node in [lo, hi):
// it writes only t[lo:hi] and gain[lo:hi] and reads g, obj, and p immutably,
// with node u's isolation candidate the distinct fresh label base+u. That
// disjoint write footprint is the unit the Gobra data-race proof specifies.
// @ requires acc(csrMem(g), _) && acc(partitionMem(p), _)
// @ requires 0 <= lo && lo <= hi && hi <= len(p)
// @ requires hi <= len(t) && len(t) == len(gain)
// @ preserves forall i int :: lo <= i && i < hi ==> acc(&t[i]) && acc(&gain[i])
func bestMovesRange(g *csr, obj objective, p Partition, t []int, gain []float64, base, lo, hi int) {
	// @ invariant acc(csrMem(g), _) && acc(partitionMem(p), _)
	// @ invariant lo <= u && u <= hi
	// @ invariant forall i int :: lo <= i && i < hi ==> acc(&t[i]) && acc(&gain[i])
	for u := lo; u < hi; u++ {
		t[u], gain[u] = bestMove(g, obj, p, u, base+u)
	}
}

// bestMovesWorker is one goroutine's share of the parallelBestMoves fan-out:
// a named function rather than a literal so the Gobra annotation never has to
// specify a closure. It fills its chunk, then settles the wait-group debt by
// paying back the chunk's permissions.
// @ requires acc(csrMem(g), _) && acc(partitionMem(p), _)
// @ requires 0 <= lo && lo <= hi && hi <= len(p)
// @ requires hi <= len(t) && len(t) == len(gain)
// @ requires forall i int :: lo <= i && i < hi ==> acc(&t[i]) && acc(&gain[i])
// @ requires wg.UnitDebt(rangeAcc{t, gain, lo, hi})
func bestMovesWorker(wg *sync.WaitGroup, g *csr, obj objective, p Partition, t []int, gain []float64, base, lo, hi int) {
	defer wg.Done()
	bestMovesRange(g, obj, p, t, gain, base, lo, hi)
	// @ fold rangeAcc{t, gain, lo, hi}()
	// @ wg.PayDebt(rangeAcc{t, gain, lo, hi})
}

// parallelBestMoves computes, for every node against the frozen round-start
// snapshot p, its best target community and the resulting gain (bestMove, the
// same per-node search the serial sweep uses). Node u's isolation candidate is
// the distinct fresh label isolationBase(p, n) + u.
//
// The work is split into contiguous node-index ranges, one per worker goroutine;
// each goroutine writes only its own slots of t and gain and reads only the
// shared, immutable g, obj, and p, so there is no data race and no lock. Because
// every node's decision depends solely on the snapshot - never on another node's
// same-round result - the output is byte-identical whatever the worker count,
// which is the code-level half of applyRound_perm's core-count independence.
// @ requires acc(csrMem(g), _) && acc(partitionMem(p), _)
// @ requires len(p) == g.numNodes()
// @ ensures acc(csrMem(g), _) && acc(partitionMem(p), _)
// @ ensures len(t) == g.numNodes() && len(gain) == len(t)
// @ ensures forall i int :: 0 <= i && i < len(t) ==> acc(&t[i]) && acc(&gain[i])
func parallelBestMoves(g *csr, obj objective, p Partition, workers int) (t []int, gain []float64) {
	n := g.numNodes()
	t = make([]int, n)
	gain = make([]float64, n)
	if n == 0 {
		return t, gain
	}
	base := isolationBase(p, n)

	workers = normalizeWorkers(workers, n)
	if workers == 1 {
		bestMovesRange(g, obj, p, t, gain, base, 0, n)
		return t, gain
	}

	var wg /*@@@*/ sync.WaitGroup
	// @ wg.Init()
	chunk := (n + workers - 1) / workers
	// @ ghost los := seq[int]{}
	// @ ghost his := seq[int]{}
	// @ ghost tokens := seq[pred()]{}
	// @ invariant acc(csrMem(g), _) && acc(partitionMem(p), _)
	// @ invariant n == len(p)
	// @ invariant 1 <= chunk
	// @ invariant 0 <= lo
	// @ invariant len(los) == len(tokens) && len(his) == len(tokens)
	// @ invariant len(tokens) == 0 ==> lo == 0
	// @ invariant len(tokens) > 0 ==> los[0] == 0
	// @ invariant len(tokens) > 0 ==> his[len(tokens)-1] == (lo < n ? lo : n)
	// @ invariant forall j int :: 0 <= j && j < len(tokens) ==> 0 <= los[j] && los[j] < his[j] && his[j] <= n
	// @ invariant forall j int :: 0 <= j && j < len(tokens)-1 ==> his[j] == los[j+1]
	// @ invariant forall j int :: 0 <= j && j < len(tokens) ==> tokens[j] == rangeAcc{t, gain, los[j], his[j]}
	// @ invariant forall j int :: 0 <= j && j < len(tokens) ==> wg.TokenById(tokens[j], j)
	// @ invariant len(tokens) == 0 ==> wg.WaitGroupP()
	// @ invariant len(tokens) > 0 ==> acc(wg.WaitGroupP(), 1/2) && acc(wg.WaitGroupStarted(), 1/2)
	// @ invariant !wg.WaitMode()
	// @ invariant forall i int :: (lo < n ? lo : n) <= i && i < n ==> acc(&t[i]) && acc(&gain[i])
	for lo := 0; lo < n; lo += chunk {
		hi := min(lo+chunk, n)
		wg.Add(1 /*@, 1/2, PredTrue{} @*/)
		// @ ghost if lo == 0 { wg.Start(1/2, PredTrue{}) }
		// @ wg.GenerateTokenAndDebt(rangeAcc{t, gain, lo, hi})
		// @ fold wg.TokenById(rangeAcc{t, gain, lo, hi}, len(tokens))
		// @ los = los ++ seq[int]{lo}
		// @ his = his ++ seq[int]{hi}
		// @ tokens = tokens ++ seq[pred()]{rangeAcc{t, gain, lo, hi}}
		go bestMovesWorker(&wg, g, obj, p, t, gain, base, lo, hi)
	}
	// @ wg.SetWaitMode(1/2, 1/2)
	wg.Wait( /*@ 1/1, tokens @*/ )
	// @ redeem(t, gain, los, his, tokens, 0)
	return t, gain
}

// parallelRound applies one synchronous local-move round against the frozen
// snapshot p and returns the next partition together with whether it changed.
// Every node's target is decided from p alone (parallelBestMoves), the moved set
// is every node whose gain clears moveImproveEps, and the moves are combined by
// applyRound - a function of the snapshot and the moved set, independent of the
// number of workers.
//
// Confluence (applyRound_perm) gives determinism but not monotonicity: because
// each node prices its move against the snapshot, two independent snapshot moves
// interact through the null-model degree sums, so applying the whole moved set
// at once can fail to improve - or, on a bipartite trap like an even cycle,
// swap partners and oscillate. The design-4.5 mitigation chosen here is
// "strictly-positive-gain acceptance with deterministic conflict resolution":
//
//   - Compute the round's exact realized delta by summing the incremental
//     move-deltas in ascending node order against a running copy (a telescoping
//     sum that equals Quality(applyRound(...)) - Quality(p), evaluated in a fixed
//     order so it is byte-identical across worker counts).
//   - If it strictly improves, accept the whole round.
//   - Otherwise the batch conflicted, so resolve deterministically by applying
//     only the single highest-gain move (smallest node index breaking ties).
//     A lone move's realized delta is exactly its snapshot gain, which cleared
//     moveImproveEps, so this branch strictly improves too.
//
// Either branch raises the objective by more than moveImproveEps, so - exactly
// as in the serial loop's termination argument (no_infinite_acceptedMove_run) -
// only finitely many rounds can change the partition. Every input to the accept/
// resolve decision is a pure function of the snapshot, so the returned partition
// is independent of the schedule and the worker count.
//
// It is deliberately outside the Gobra proof scope (R2) and marked trusted:
// its body uses copy on a defined slice type and float compound assignment,
// which the Gobra frontend rejects even for unverified members.
// @ trusted
func parallelRound(g *csr, obj objective, p Partition, workers int) (Partition, bool) {
	t, gain := parallelBestMoves(g, obj, p, workers)

	moved := make([]bool, len(p))
	anyMoved := false
	for u := range p {
		if gain[u] > moveImproveEps && t[u] != p[u] {
			moved[u] = true
			anyMoved = true
		}
	}
	if !anyMoved {
		return p, false
	}

	// Exact realized delta of the full round, summed in ascending node order.
	q := make(Partition, len(p))
	copy(q, p)
	var realized float64
	for u := range p {
		if moved[u] {
			realized += obj.moveDelta(g, q, u, t[u])
			q[u] = t[u]
		}
	}
	if realized > moveImproveEps {
		return applyRound(p, t, moved), true
	}

	// Conflict resolution: the batch did not improve, so take only the single
	// best move (smallest index on a tie), which is exactly monotone.
	best := -1
	for u := range p {
		if moved[u] && (best < 0 || gain[u] > gain[best]) {
			best = u
		}
	}
	only := make([]bool, len(p))
	only[best] = true
	return applyRound(p, t, only), true
}

// parallelLocalMoveToStable runs synchronous parallel rounds (parallelRound)
// until a round changes nothing, mutating p in place to a local-move-stable
// partition - no single-node reassignment strictly improves the objective, the
// same fixed point the serial sweep reaches. It reports whether any node moved.
//
// The loop terminates because every changing round strictly raises the objective
// by more than moveImproveEps and the objective takes finitely many values over
// the finite set of node groupings. The result is a pure function of (g, obj, p):
// each round is, so their fixed point is, independent of the worker count.
//
// Like parallelRound it is outside the Gobra proof scope (R2) and marked
// trusted (its body uses copy on a defined slice type).
// @ trusted
func parallelLocalMoveToStable(g *csr, obj objective, p Partition, workers int) bool {
	moved := false
	for {
		next, changed := parallelRound(g, obj, p, workers)
		if !changed {
			return moved
		}
		copy(p, next)
		moved = true
	}
}
