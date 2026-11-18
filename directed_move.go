package meso

// moveDelta returns the exact change in directed modularity from reassigning
// node u to community target under partition p: it equals
// Quality(move(p, u, target)) - Quality(p) up to floating-point rounding. It is
// the from-scratch form of [directedPricer.delta]: it builds a pricer for p, at
// O(n + m), and prices the one move. The optimizers never call it per
// candidate; they build one pricer per pass and keep it current.
func (m directedModularity) moveDelta(g *csr, p Partition, u, target int) float64 {
	return m.newPricer(g, p).delta(p, u, target)
}

// directedPricer is the [pricer] for directed modularity: every node's weighted
// out- and in-degree, the total arc weight, and the summed out- and in-degree
// of each community.
type directedPricer struct {
	g       *csr
	gamma   float64
	total   float64
	degOut  []float64
	degIn   []float64
	koutTot *communityMass
	kinTot  *communityMass
}

func (m directedModularity) newPricer(g *csr, p Partition) pricer {
	n := g.numNodes()
	degOut := make([]float64, n)
	degIn := make([]float64, n)
	total := 0.0
	for i := range n {
		degOut[i] = g.outDegree(i)
		degIn[i] = g.inDegree(i)
		total += degOut[i]
	}
	return &directedPricer{
		g: g, gamma: m.gamma, total: total, degOut: degOut, degIn: degIn,
		koutTot: newCommunityMass(p, degOut),
		kinTot:  newCommunityMass(p, degIn),
	}
}

func (pr *directedPricer) moved(u, src, target int) {
	pr.koutTot.shift(src, target, pr.degOut[u])
	pr.kinTot.shift(src, target, pr.degIn[u])
}

// delta returns the change in directed modularity from reassigning node u to
// community target, in O(deg u) over u's out- and in-arcs.
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
func (pr *directedPricer) delta(p Partition, u, target int) float64 {
	src := p[u]
	if target == src {
		return 0
	}
	toTarget, toSrc := linkWeights(pr.g, p, u, target, src)
	return pr.price(u, src, target, toSrc, toTarget)
}

func (pr *directedPricer) price(u, src, target int, toSrc, toTarget linkWeight) float64 {
	if target == src {
		return 0
	}

	total := pr.total
	if total == 0 {
		return 0
	}

	koutU := pr.degOut[u]
	kinU := pr.degIn[u]
	koutTarget := pr.koutTot.of(target)
	kinTarget := pr.kinTot.of(target)
	koutSrcWithoutU := pr.koutTot.of(src) - koutU
	kinSrcWithoutU := pr.kinTot.of(src) - kinU

	edge := (toTarget.out - toSrc.out) + (toTarget.in - toSrc.in)
	null := pr.gamma*koutU*(kinTarget-kinSrcWithoutU)/total +
		pr.gamma*kinU*(koutTarget-koutSrcWithoutU)/total
	return (edge - null) / total
}
