package meso

// directedModularity is the Leicht-Newman directed modularity objective with
// resolution parameter gamma. Use [DirectedModularity] to construct it.
type directedModularity struct {
	gamma float64
}

// DirectedModularity returns the Leicht-Newman directed modularity quality
// function with resolution parameter gamma. It is the directed analogue of
// [Modularity]: the null model uses each pair's separate out- and in-degrees
// (k_i^out * k_j^in / m) rather than the symmetric k_i * k_j / 2m, so an arc
// i -> j is credited against i's out-strength and j's in-strength.
//
// It reads the directed CSR's separable out- and in-adjacencies (see [csr]).
// On an undirected graph, where in-degree equals out-degree and the total m
// equals 2m, it reduces exactly to [Modularity]. Gamma scales the null-model
// term as in [Modularity].
//
// Unlike modularity and CPM, directed modularity has no Lean model: directed
// support is a deliberate descope (the Lean weight-symmetry assumption is
// load-bearing), so its correctness rests on the directed reference oracle, not
// a proof.
func DirectedModularity(gamma float64) QualityFunction {
	return directedModularity{gamma: gamma}
}

var _ QualityFunction = directedModularity{}

// Quality evaluates directed modularity with resolution gamma:
//
//	Q = (1 / m) * sum_ij (w_ij - gamma * k_i^out * k_j^in / m) * delta(c_i, c_j)
//
// where w_ij is the weight of the arc i -> j (the self-loop weight on the
// diagonal), k_i^out and k_j^in the weighted out- and in-degrees, m the total
// arc weight (sum_i k_i^out, which the CSR's [csr.twoM] returns for a directed
// graph), and delta is 1 when i and j share a community. Computed in canonical
// row-major order so repeated evaluation is bit-identical. A graph with m == 0
// has Q = 0.
func (m directedModularity) Quality(g *csr, p Partition) float64 {
	total := g.twoM()
	if total == 0 {
		return 0
	}

	n := g.numNodes()
	sum := 0.0
	for i := range n {
		kOut := g.outDegree(i)
		for j := range n {
			if p[i] != p[j] {
				continue
			}
			sum += g.weight(i, j) - m.gamma*kOut*g.inDegree(j)/total
		}
	}
	return sum / total
}
