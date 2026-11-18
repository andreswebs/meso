package meso

// QualityFunction scores a partition of a graph: higher is better. It is the
// objective the optimizers (Louvain, Leiden) maximize, and the single shape
// shared by modularity, CPM, and directed modularity so the move loop can be
// written once against the interface.
//
// Quality is the full from-scratch evaluation of the objective over the whole
// graph. Each objective is defined by a double sum over same-community node
// pairs; the evaluation groups that sum by community, which needs only each
// community's internal weight and its total degree or size, so it costs
// O(n + m). It is the oracle the incremental move-delta is property-checked
// against. The method takes the internal csr, so quality functions are
// meso-internal; callers select one through constructors such as [Modularity].
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
// share a community. This is the double-sum definition from the Lean model
// (verification/lean/Meso/Quality.lean), grouped by community:
//
//	Q = (1 / 2m) * sum_c (W_c - gamma * K_c^2 / 2m)
//
// with W_c the weight inside community c and K_c its summed degree. The
// communities are added in order of first appearance, which depends only on the
// grouping, so repeated evaluation is bit-identical and relabelling a partition
// does not change the result. A graph with 2m == 0 has Q = 0.
func (m modularity) Quality(g *csr, p Partition) float64 {
	twoM := g.twoM()
	if twoM == 0 {
		return 0
	}

	of, k := communityIndex(p)
	internal := internalWeights(g, of, k)
	kTot := make([]float64, k)
	for i, c := range of {
		kTot[c] += g.degree(i)
	}
	sum := 0.0
	for c := range k {
		sum += internal[c] - m.gamma*kTot[c]*kTot[c]/twoM
	}
	return sum / twoM
}

// communityIndex numbers the communities of p densely in order of first
// appearance, returning each node's community index and the community count.
// The numbering depends only on which nodes share a label, never on the label
// values.
func communityIndex(p Partition) (of []int, k int) {
	of, k = denseLabels(p)
	renumber := make([]int, k)
	for c := range renumber {
		renumber[c] = -1
	}
	next := 0
	for i, c := range of {
		if renumber[c] < 0 {
			renumber[c] = next
			next++
		}
		of[i] = renumber[c]
	}
	return of, k
}

// internalWeights returns, for each of the k communities indexed by of, the
// weight inside it: the sum of w_ij over ordered pairs (i, j) of its members,
// self-loops included. An undirected edge is stored from both endpoints and so
// counts twice, a directed arc once, matching the double sum.
func internalWeights(g *csr, of []int, k int) []float64 {
	internal := make([]float64, k)
	for i, c := range of {
		internal[c] += g.selfLoops[i]
		nw := g.neighborWeights(i)
		for x, j := range g.neighbors(i) {
			if of[j] == c {
				internal[c] += nw[x]
			}
		}
	}
	return internal
}
