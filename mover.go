package meso

// localMover runs a level's fast local-move phase to a fixed point, mutating p
// in place. It is the seam between the serial sweep/queue phases and the
// synchronous-round parallel phase: the Louvain and Leiden level loops take a
// localMover so one aggregation-and-recursion driver serves both execution
// modes (serialLouvainMover, serialLeidenMover, parallelMover).
//
// localMover and parallelMover live in this file rather than parallel.go so
// that parallel.go, which is fed to the Gobra verifier, contains no function
// literals: the verifier encodes every member of its input files, and keeping
// the closure out of the input set sidesteps closure encoding entirely.
type localMover func(g *csr, obj objective, p Partition)

// parallelMover is the localMover that runs the synchronous-round parallel phase
// (parallelLocalMoveToStable) with the given worker count.
func parallelMover(workers int) localMover {
	return func(g *csr, obj objective, p Partition) {
		parallelLocalMoveToStable(g, obj, p, workers)
	}
}
