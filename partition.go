package meso

import "fmt"

// Partition assigns each node a community label: the label of node i is p[i].
// It is the Go realization of the Lean Partition n := Fin n -> Nat (see
// verification/lean/Meso/Graph.lean and verification/lean/CORRESPONDENCE.md).
//
// Lean encodes well-formedness structurally, as a total function on Fin n; in
// Go it is the length-and-range check of wellFormed: a partition of n nodes has
// length n and every label in [0, n), since n nodes form at most n communities.
type Partition []int

// wellFormed reports whether p is a well-formed partition of n nodes, returning
// a descriptive error on the first violation and nil when p is well-formed: p
// has length n and every label lies in [0, n).
func (p Partition) wellFormed(n int) error {
	if len(p) != n {
		return fmt.Errorf("meso: partition length %d, want n = %d", len(p), n)
	}
	for i, c := range p {
		if c < 0 || c >= n {
			return fmt.Errorf("meso: partition label %d at node %d out of range [0, %d)", c, i, n)
		}
	}
	return nil
}
