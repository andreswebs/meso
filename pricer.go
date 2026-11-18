package meso

// pricer prices single-node moves for one objective against one graph without
// re-reading the whole graph per candidate. It holds the constants of the graph
// the move-delta needs (per-node degrees or sizes, the total weight) and the
// summed mass of every community, so delta costs O(deg u) where a from-scratch
// evaluation costs O(n + m).
//
// A pricer is built from a partition (objective.newPricer) and must be told of
// every reassignment applied to that partition afterwards (moved); between the
// two it prices moves against exactly the partition it has been told about. It
// is safe for concurrent delta calls as long as no moved call runs alongside
// them, which is how the synchronous parallel round uses it: workers price
// against the frozen round-start snapshot, and moves are applied only after
// they have all finished.
type pricer interface {
	// delta returns the change in the objective from reassigning node u to
	// community target under p, the partition the pricer is tracking.
	delta(p Partition, u, target int) float64

	// price is delta with u's link weights already summed: toSrc and toTarget
	// are the weight between u and its current community src, and between u
	// and target, self-loop excluded. A caller pricing many targets for one
	// node gathers every community's link weight in a single pass over u's
	// adjacency and calls price per target, where delta would rescan the
	// adjacency each time.
	price(u, src, target int, toSrc, toTarget linkWeight) float64

	// moved records that node u was reassigned from community src to
	// community target, keeping the community masses in step with the
	// partition.
	moved(u, src, target int)
}

// communityMass is the summed mass (weighted degree, or node size) of each
// community, keyed by community label. A label with no entry has mass 0, which
// is exactly the mass of a fresh, empty community.
//
// Labels are sparse and unbounded: the local-move phase mints fresh isolation
// labels past n, so no slice has a fixed size that fits them all. The labels a
// serial pass meets are almost all below 2n (the n starting labels plus one
// isolation label per split), so those live in low, indexed by label, and only
// the rest fall through to high. high is only ever read by key, never ranged
// over, so Go's randomised map iteration order cannot reach any result.
type communityMass struct {
	low  []float64
	high map[int]float64
}

// newCommunityMass sums mass over the members of every community of p, visiting
// nodes in ascending index order. That fixed order makes each total a pure
// function of (p, mass), independent of how the partition was reached.
func newCommunityMass(p Partition, mass []float64) *communityMass {
	t := &communityMass{low: make([]float64, 2*len(p))}
	for v, c := range p {
		t.add(c, mass[v])
	}
	return t
}

// of returns the mass of community c.
func (t *communityMass) of(c int) float64 {
	if c >= 0 && c < len(t.low) {
		return t.low[c]
	}
	return t.high[c]
}

func (t *communityMass) add(c int, m float64) {
	if c >= 0 && c < len(t.low) {
		t.low[c] += m
		return
	}
	if t.high == nil {
		t.high = make(map[int]float64)
	}
	t.high[c] += m
}

// shift moves mass m from community src to community target.
func (t *communityMass) shift(src, target int, m float64) {
	t.add(src, -m)
	t.add(target, m)
}

// linkWeight is the weight joining one node to one community: out over the
// node's out-arcs into the community, in over its in-arcs from the community.
// On an undirected graph the two coincide and the undirected objectives read
// out alone.
type linkWeight struct {
	out, in float64
}

// linkWeights sums the weight between node u and each of the two communities a
// and b under p, in adjacency order: the scan a single move-delta needs.
func linkWeights(g *csr, p Partition, u, a, b int) (toA, toB linkWeight) {
	nb := g.neighbors(u)
	nw := g.neighborWeights(u)
	for k, v := range nb {
		switch p[v] {
		case a:
			toA.out += nw[k]
		case b:
			toB.out += nw[k]
		}
	}
	inNb := g.inNeighbors(u)
	inW := g.inNeighborWeights(u)
	for k, v := range inNb {
		switch p[v] {
		case a:
			toA.in += inW[k]
		case b:
			toB.in += inW[k]
		}
	}
	return toA, toB
}
