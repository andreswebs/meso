package meso

// NumCommunities returns the number of communities. Labels are dense in
// [0, NumCommunities).
func (r *Result) NumCommunities() int { return len(r.members) }

// Members returns the keys of community label in dense index order, or nil
// when label is outside [0, NumCommunities). The slice is a copy the caller
// may modify.
func (r *Result) Members(label int) []string {
	if label < 0 || label >= len(r.members) {
		return nil
	}
	idx := r.members[label]
	keys := make([]string, len(idx))
	for k, i := range idx {
		keys[k] = r.g.keys[i]
	}
	return keys
}

// Cohesion returns the internal density of community label, in [0, 1]: the
// number of distinct edges with both endpoints inside it divided by k(k-1)/2
// for k members, or on a directed graph the number of distinct internal arcs
// divided by k(k-1). Self-loops are excluded and weights are ignored, so a
// unit-weight graph and a weighted one with the same edges agree. A community
// of fewer than two members, or an unknown label, has cohesion 0.
func (r *Result) Cohesion(label int) float64 {
	if label < 0 || label >= len(r.members) {
		return 0
	}
	members := r.members[label]
	k := len(members)
	if k < 2 {
		return 0
	}
	g := r.g.model
	internal := 0
	for _, i := range members {
		for _, j := range g.neighbors(i) {
			if r.part[j] == label {
				internal++
			}
		}
	}
	// An undirected edge sits in both endpoints' lists, so internal counts it
	// twice and k(k-1) is the matching doubled k(k-1)/2. A directed arc sits
	// once in its source's out-list, against k(k-1) ordered pairs.
	return float64(internal) / float64(k*(k-1))
}
