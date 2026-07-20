package meso

// QualityFunction scores a partition of a graph: higher is better. It is the
// objective the optimizers (Louvain, Leiden) maximize, and the single shape
// shared by modularity, CPM, and directed modularity so the move loop can be
// written once against the interface.
//
// Quality is the full from-scratch evaluation of the objective over the whole
// graph. It is deliberately simple and slow (the O(n^2) double sum for
// modularity), so it is obviously correct and serves as the oracle the
// incremental move-delta is property-checked against. The method takes the
// internal csr, so quality functions are meso-internal; callers select one
// through constructors such as [Modularity].
type QualityFunction interface {
	Quality(g *csr, p Partition) float64
}

// modularity is the modularity objective with resolution parameter gamma. Use
// [Modularity] to construct it.
type modularity struct {
	gamma float64
}

// Modularity returns the modularity quality function with resolution parameter
// gamma. Gamma scales the null-model (expected-weight) term: gamma = 1 is
// classical modularity, higher gamma favours smaller communities and lower
// gamma favours larger ones.
func Modularity(gamma float64) QualityFunction {
	return modularity{gamma: gamma}
}

var _ QualityFunction = modularity{}

// Quality evaluates modularity with resolution gamma:
//
//	Q = (1 / 2m) * sum_ij (w_ij - gamma * k_i * k_j / 2m) * delta(c_i, c_j)
//
// where w_ij is the weight between nodes i and j (the self-loop weight on the
// diagonal), k_i the weighted degree, 2m the total, and delta is 1 when i and j
// share a community. This is the literal double-sum definition from the Lean
// model (verification/lean/Meso/Quality.lean), computed in canonical row-major
// order so repeated evaluation is bit-identical. A graph with 2m == 0 has Q = 0.
func (m modularity) Quality(g *csr, p Partition) float64 {
	twoM := g.twoM()
	if twoM == 0 {
		return 0
	}

	n := g.numNodes()
	sum := 0.0
	for i := range n {
		ki := g.degree(i)
		for j := range n {
			if p[i] != p[j] {
				continue
			}
			sum += g.weight(i, j) - m.gamma*ki*g.degree(j)/twoM
		}
	}
	return sum / twoM
}
