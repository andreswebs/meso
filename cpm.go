package meso

// cpm is the Constant Potts Model objective with resolution parameter gamma. Use
// [CPM] to construct it.
type cpm struct {
	gamma float64
}

// CPM returns the Constant Potts Model quality function with resolution
// parameter gamma. Where modularity penalises a within-community pair by a
// degree-based, graph-size-dependent null model (gamma*k_i*k_j/2m), CPM
// penalises it by a flat node-size term gamma*s_i*s_j with no 2m normalisation.
// Gamma is therefore an absolute internal-density threshold: a community is worth
// keeping only while its internal weight exceeds gamma times its squared size
// (see [IsGammaDense]-style reasoning), and higher gamma favours smaller
// communities.
func CPM(gamma float64) QualityFunction {
	return cpm{gamma: gamma}
}

var _ QualityFunction = cpm{}

// Quality evaluates CPM with resolution gamma:
//
//	Q = sum_ij (w_ij - gamma * s_i * s_j) * delta(c_i, c_j)
//
// where w_ij is the weight between nodes i and j (the self-loop weight on the
// diagonal), s_i node i's size, and delta is 1 when i and j share a community.
// This is the literal double-sum definition from the Lean model
// (verification/lean/Meso/CPM.lean); unlike modularity there is no 2m
// normalisation, so CPM carries the units of edge weight. Computed in canonical
// row-major order so repeated evaluation is bit-identical. A zero-node graph has
// Q = 0.
func (m cpm) Quality(g *csr, p Partition) float64 {
	n := g.numNodes()
	sum := 0.0
	for i := range n {
		si := g.nodeSize(i)
		for j := range n {
			if p[i] != p[j] {
				continue
			}
			sum += g.weight(i, j) - m.gamma*si*g.nodeSize(j)
		}
	}
	return sum
}

// communityInternalWeight returns e_c: the total weight between node pairs both
// assigned to community c, self-loops included (the diagonal i == j terms). It is
// the from-scratch block sum of the Lean communityInternalWeight
// (verification/lean/Meso/CPM.lean), O(n^2) over the graph and the internal-edge
// half of the per-community CPM decomposition e_c - gamma*S_c^2.
func communityInternalWeight(g *csr, p Partition, c int) float64 {
	n := g.numNodes()
	sum := 0.0
	for i := range n {
		if p[i] != c {
			continue
		}
		for j := range n {
			if p[j] == c {
				sum += g.weight(i, j)
			}
		}
	}
	return sum
}

// blockWeight returns e(a, b): the total weight over ordered node pairs with the
// first node in community a and the second in community b, self-loops and both
// orientations of each internal off-diagonal edge included. It is the from-scratch
// Go image of the Lean blockWeight (verification/lean/Meso/Aggregate.lean): the
// diagonal block blockWeight(a, a) equals community a's internal weight
// (communityInternalWeight), and the off-diagonal block a != b is the cut weight
// between the two communities, the quantity the refinement gate compares against
// gamma*S_a*S_b. O(n^2) over the graph, for the gate and the density checks, not
// the hot path.
func blockWeight(g *csr, p Partition, a, b int) float64 {
	n := g.numNodes()
	sum := 0.0
	for i := range n {
		if p[i] != a {
			continue
		}
		for j := range n {
			if p[j] == b {
				sum += g.weight(i, j)
			}
		}
	}
	return sum
}

// communitySize returns S_c: the summed node size of community c's members, the
// quantity squared in the CPM penalty (Lean communitySize).
func communitySize(g *csr, p Partition, c int) float64 {
	n := g.numNodes()
	sum := 0.0
	for i := range n {
		if p[i] == c {
			sum += g.nodeSize(i)
		}
	}
	return sum
}

// cpmMergeGain returns the CPM gain of merging the singleton communities of
// nodes a and b: starting from a partition where a and b each sit alone,
// reassigning a into b's community raises CPM by exactly
//
//	2 * (w_ab - gamma * s_a * s_b)
//
// This is the Go image of the Lean cpm_merge_two_singletons theorem
// (verification/lean/Meso/Separation.lean): the general single-node move-delta
// specialised to the aggregate level, where each community is a single node. Its
// sign is what gamma-separation reads off - two nodes are gamma-separated exactly
// when this gain is nonpositive, so no merge would improve CPM. w_ab is the
// self-loop weight when a == b, but the formula is intended for distinct nodes.
func cpmMergeGain(g *csr, gamma float64, a, b int) float64 {
	return 2 * (g.weight(a, b) - gamma*g.nodeSize(a)*g.nodeSize(b))
}

// isGammaDense reports whether community c is gamma-dense under partition p: its
// internal weight covers the resolution term gamma*S_c^2, equivalently its CPM
// contribution e_c - gamma*S_c^2 is nonnegative (Lean IsGammaDense and
// isGammaDense_iff). It is the CPM notion of a community that is internally
// well-knit relative to gamma, the basis for the paper's gamma-connectivity
// guarantees.
func isGammaDense(g *csr, gamma float64, p Partition, c int) bool {
	s := communitySize(g, p, c)
	return gamma*s*s <= communityInternalWeight(g, p, c)
}
