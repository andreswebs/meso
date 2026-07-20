package meso

// move returns a copy of p with node v reassigned to community c, leaving every
// other node's label unchanged. It is the Go image of the Lean move operator
// (verification/lean/Meso/Move.lean): move(p, v, p[v]) equals p (a no-op).
func move(p Partition, v, c int) Partition {
	q := make(Partition, len(p))
	copy(q, p)
	q[v] = c
	return q
}

// moveDelta returns the exact change in modularity from reassigning node u to
// community target under partition p: it equals Quality(move(p, u, target)) -
// Quality(p) up to floating-point rounding, computed incrementally rather than
// by two full O(n^2) evaluations.
//
// Derivation. With A_ij = w_ij - gamma*k_i*k_j/2m the modularity is
// Q = (1/2m) * sum_ij A_ij * delta(c_i, c_j). Moving only node u changes just
// row u and column u of the double sum, and A is symmetric, so
//
//	Q_after - Q_before = (2/2m) * sum_{j != u} A_uj * (delta(t, c_j) - delta(s, c_j))
//
// where s is u's source community and t the target. The j == u term is constant
// (u always shares a community with itself) and drops out, so the moved node's
// self-loop weight never enters the edge term; it enters only through k_u in the
// null-model term. Splitting the remaining sum over the target community t and
// the source community with u removed (s \ {u}) gives
//
//	Q_after - Q_before = (2/2m) * [ (W_ut - W_us) - gamma*k_u*(K_t - K_{s\{u}})/2m ]
//
// with W_ut, W_us the edge weight from u into each community (self-loop
// excluded) and K_t, K_{s\{u}} = K_s - k_u the summed weighted degrees. A no-op
// move (target == s) and the 2m == 0 graph both return 0.
func (m modularity) moveDelta(g *csr, p Partition, u, target int) float64 {
	src := p[u]
	if target == src {
		return 0
	}

	twoM := g.twoM()
	if twoM == 0 {
		return 0
	}

	ku := g.degree(u)

	var kTarget, kSrc float64
	for v := 0; v < g.numNodes(); v++ {
		switch p[v] {
		case target:
			kTarget += g.degree(v)
		case src:
			kSrc += g.degree(v)
		}
	}
	kSrcWithoutU := kSrc - ku

	var wTarget, wSrc float64
	nb := g.neighbors(u)
	nw := g.neighborWeights(u)
	for k, v := range nb {
		switch p[v] {
		case target:
			wTarget += nw[k]
		case src:
			wSrc += nw[k]
		}
	}

	return (2 / twoM) * ((wTarget - wSrc) - m.gamma*ku*(kTarget-kSrcWithoutU)/twoM)
}

// moveDelta returns the exact change in CPM from reassigning node u to community
// target under partition p: it equals Quality(move(p, u, target)) - Quality(p)
// up to floating-point rounding, computed incrementally rather than by two full
// O(n^2) evaluations.
//
// Derivation. With B_ij = w_ij - gamma*s_i*s_j the CPM is
// Q = sum_ij B_ij * delta(c_i, c_j) (no 2m normalisation, so B, unlike the
// modularity kernel, does not depend on the whole graph). Moving only node u
// changes just row u and column u, and B is symmetric, so
//
//	Q_after - Q_before = 2 * sum_{j != u} B_uj * (delta(t, c_j) - delta(s, c_j))
//
// where s is u's source community and t the target. The j == u term (the
// diagonal B_uu = w_uu - gamma*s_u^2) is constant because u always shares a
// community with itself, so it drops out: the moved node's self-loop weight and
// its own size-squared penalty never enter the delta. Splitting the remaining
// sum over the target community t and the source with u removed (s \ {u}) gives
//
//	Q_after - Q_before = 2 * [ (W_ut - W_us) - gamma*s_u*(S_t - S_{s\{u}}) ]
//
// with W_ut, W_us the edge weight from u into each community (self-loop
// excluded, since it is the dropped diagonal) and S_t, S_{s\{u}} = S_s - s_u the
// summed node sizes. A no-op move (target == s) returns 0. Unlike modularity
// there is no 2m factor, so a zero-weight graph needs no special case.
func (m cpm) moveDelta(g *csr, p Partition, u, target int) float64 {
	src := p[u]
	if target == src {
		return 0
	}

	su := g.nodeSize(u)

	var sTarget, sSrc float64
	for v := 0; v < g.numNodes(); v++ {
		switch p[v] {
		case target:
			sTarget += g.nodeSize(v)
		case src:
			sSrc += g.nodeSize(v)
		}
	}
	sSrcWithoutU := sSrc - su

	var wTarget, wSrc float64
	nb := g.neighbors(u)
	nw := g.neighborWeights(u)
	for k, v := range nb {
		switch p[v] {
		case target:
			wTarget += nw[k]
		case src:
			wSrc += nw[k]
		}
	}

	return 2 * ((wTarget - wSrc) - m.gamma*su*(sTarget-sSrcWithoutU))
}
