package meso

// moveDelta returns the exact change in directed modularity from reassigning
// node u to community target under partition p: it equals
// Quality(move(p, u, target)) - Quality(p) up to floating-point rounding,
// computed incrementally rather than by two full O(n^2) evaluations.
//
// Derivation. With A_ij = w_ij - gamma*k_i^out*k_j^in/T the directed modularity
// is Q = (1/T) * sum_ij A_ij * delta(c_i, c_j), where T is the total arc weight.
// Unlike the undirected kernel, A is not symmetric, so moving node u perturbs
// both row u (the arcs out of u) and column u (the arcs into u); the j == u
// diagonal term is constant (u always shares a community with itself) and drops
// out, so u's self-loop never enters the edge term. Splitting the row and column
// sums over the target community t and the source with u removed (s \ {u}) gives
//
//	Q_after - Q_before = (1/T) * [ (Wout_ut - Wout_us) + (Win_ut - Win_us)
//	    - gamma*k_u^out*(Kin_t - Kin_{s\{u}})/T
//	    - gamma*k_u^in*(Kout_t - Kout_{s\{u}})/T ]
//
// with Wout_u* the arc weight from u into a community (u's out-arcs, self-loop
// excluded), Win_u* the arc weight from a community into u (u's in-arcs), and
// Kout_*, Kin_* the summed out- and in-degrees of a community. On a symmetric
// graph, where in equals out at every node, both direction terms coincide and
// this reduces exactly to [modularity.moveDelta]. A no-op move (target == s) and
// the T == 0 graph both return 0.
func (m directedModularity) moveDelta(g *csr, p Partition, u, target int) float64 {
	src := p[u]
	if target == src {
		return 0
	}

	total := g.twoM()
	if total == 0 {
		return 0
	}

	koutU := g.outDegree(u)
	kinU := g.inDegree(u)

	var koutTarget, kinTarget, koutSrc, kinSrc float64
	for v := 0; v < g.numNodes(); v++ {
		switch p[v] {
		case target:
			koutTarget += g.outDegree(v)
			kinTarget += g.inDegree(v)
		case src:
			koutSrc += g.outDegree(v)
			kinSrc += g.inDegree(v)
		}
	}
	koutSrcWithoutU := koutSrc - koutU
	kinSrcWithoutU := kinSrc - kinU

	var woutTarget, woutSrc float64
	outNb := g.neighbors(u)
	outW := g.neighborWeights(u)
	for k, v := range outNb {
		switch p[v] {
		case target:
			woutTarget += outW[k]
		case src:
			woutSrc += outW[k]
		}
	}

	var winTarget, winSrc float64
	inNb := g.inNeighbors(u)
	inW := g.inNeighborWeights(u)
	for k, v := range inNb {
		switch p[v] {
		case target:
			winTarget += inW[k]
		case src:
			winSrc += inW[k]
		}
	}

	edge := (woutTarget - woutSrc) + (winTarget - winSrc)
	null := m.gamma*koutU*(kinTarget-kinSrcWithoutU)/total +
		m.gamma*kinU*(koutTarget-koutSrcWithoutU)/total
	return (edge - null) / total
}
