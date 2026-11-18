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
// Quality(p) up to floating-point rounding. It is the from-scratch form of
// [modularityPricer.delta]: it builds a pricer for p, at O(n + m), and prices
// the one move. The optimizers never call it per candidate; they build one
// pricer per pass and keep it current.
func (m modularity) moveDelta(g *csr, p Partition, u, target int) float64 {
	return m.newPricer(g, p).delta(p, u, target)
}

// modularityPricer is the [pricer] for modularity: every node's weighted
// degree, the total weight 2m, and the summed degree of each community.
type modularityPricer struct {
	g     *csr
	gamma float64
	twoM  float64
	deg   []float64
	kTot  *communityMass
}

func (m modularity) newPricer(g *csr, p Partition) pricer {
	deg := make([]float64, g.numNodes())
	twoM := 0.0
	for i := range deg {
		deg[i] = g.degree(i)
		twoM += deg[i]
	}
	return &modularityPricer{g: g, gamma: m.gamma, twoM: twoM, deg: deg, kTot: newCommunityMass(p, deg)}
}

func (pr *modularityPricer) moved(u, src, target int) {
	pr.kTot.shift(src, target, pr.deg[u])
}

// delta returns the change in modularity from reassigning node u to community
// target, in O(deg u).
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
func (pr *modularityPricer) delta(p Partition, u, target int) float64 {
	src := p[u]
	if target == src {
		return 0
	}
	toTarget, toSrc := linkWeights(pr.g, p, u, target, src)
	return pr.price(u, src, target, toSrc, toTarget)
}

func (pr *modularityPricer) price(u, src, target int, toSrc, toTarget linkWeight) float64 {
	if target == src {
		return 0
	}

	twoM := pr.twoM
	if twoM == 0 {
		return 0
	}

	ku := pr.deg[u]
	kTarget := pr.kTot.of(target)
	kSrcWithoutU := pr.kTot.of(src) - ku
	wTarget, wSrc := toTarget.out, toSrc.out

	return (2 / twoM) * ((wTarget - wSrc) - pr.gamma*ku*(kTarget-kSrcWithoutU)/twoM)
}

// moveDelta returns the exact change in CPM from reassigning node u to community
// target under partition p: it equals Quality(move(p, u, target)) - Quality(p)
// up to floating-point rounding. It is the from-scratch form of
// [cpmPricer.delta]: it builds a pricer for p, at O(n), and prices the one
// move. The optimizers never call it per candidate; they build one pricer per
// pass and keep it current.
func (m cpm) moveDelta(g *csr, p Partition, u, target int) float64 {
	return m.newPricer(g, p).delta(p, u, target)
}

// cpmPricer is the [pricer] for CPM: the summed node size of each community.
// Node sizes are stored on the graph, and CPM has no 2m normalisation, so
// there are no other constants to keep.
type cpmPricer struct {
	g     *csr
	gamma float64
	sTot  *communityMass
}

func (m cpm) newPricer(g *csr, p Partition) pricer {
	return &cpmPricer{g: g, gamma: m.gamma, sTot: newCommunityMass(p, g.nodeSizes)}
}

func (pr *cpmPricer) moved(u, src, target int) {
	pr.sTot.shift(src, target, pr.g.nodeSize(u))
}

// delta returns the change in CPM from reassigning node u to community target,
// in O(deg u).
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
func (pr *cpmPricer) delta(p Partition, u, target int) float64 {
	src := p[u]
	if target == src {
		return 0
	}
	toTarget, toSrc := linkWeights(pr.g, p, u, target, src)
	return pr.price(u, src, target, toSrc, toTarget)
}

func (pr *cpmPricer) price(u, src, target int, toSrc, toTarget linkWeight) float64 {
	if target == src {
		return 0
	}

	su := pr.g.nodeSize(u)
	sTarget := pr.sTot.of(target)
	sSrcWithoutU := pr.sTot.of(src) - su
	wTarget, wSrc := toTarget.out, toSrc.out

	return 2 * ((wTarget - wSrc) - pr.gamma*su*(sTarget-sSrcWithoutU))
}
