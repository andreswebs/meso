package meso

// prng is meso's owned pseudo-random generator: a splitmix64 stream. meso does
// not draw from math/rand, whose algorithm and process-global state are not
// guaranteed stable across Go versions and whose global source is shared with
// unrelated call sites. Owning the generator makes a run's randomness a pure
// function of its seed - reproducible across machines, Go releases, and
// independent of whatever else in the process draws random numbers. This is the
// PRNG half of the determinism model (design section 4.4); the canonical
// iteration order is the other half.
//
// splitmix64 is chosen for being tiny, fast, and fully specified by three
// constants, so the stream is trivially portable. It has a period of 2^64 and
// passes the standard statistical batteries, which is ample for tie-breaking
// and gain-weighted merge choices in refinement (M2); meso needs reproducible
// randomness, not cryptographic strength.
type prng struct {
	state uint64
}

// goldenGamma is splitmix64's additive step, the odd 64-bit approximation of the
// golden ratio 2^64 / phi. Adding it each step walks the 2^64 state space in a
// full cycle before the finalizer avalanches the bits.
const goldenGamma = 0x9e3779b97f4a7c15

// newPRNG returns a generator seeded with the given value. Every seed yields a
// distinct, reproducible stream.
func newPRNG(seed uint64) *prng {
	return &prng{state: seed}
}

// mix64 is the splitmix64 finalizer: a bijective avalanche that scrambles an
// input so adjacent values map to unrelated outputs. It is the shared primitive
// behind both the stream (next) and per-node seeding (nodeSeed).
func mix64(z uint64) uint64 {
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// next advances the stream by one step and returns the next 64-bit value.
func (r *prng) next() uint64 {
	r.state += goldenGamma
	return mix64(r.state)
}

// float64 returns the next stream value as a float64 uniformly distributed in
// [0, 1). It keeps the top 53 bits of a stream word - the width of a float64
// significand - so every result is a representable multiple of 2^-53, the
// standard construction of a uniform double from a 64-bit generator. Refinement
// (M2) draws from it for the gain-weighted merge choice.
func (r *prng) float64() float64 {
	return float64(r.next()>>11) / (1 << 53)
}

// nodeSeed derives node u's private seed from the global seed. It is a pure
// function of (globalSeed, u): the same value however nodes are ordered or
// scheduled, which is what makes refinement (M2) and the synchronous parallel
// round (M4) independent of core count and sweep position. The global seed and
// the node index are avalanched separately and then combined, so adjacent node
// indices and adjacent global seeds diverge immediately rather than aliasing.
// A consumer composes it with newPRNG to obtain node u's private stream
// (newPRNG(nodeSeed(seed, u))); refinement (M2) adds that call site when it
// first needs the stream.
func nodeSeed(globalSeed uint64, u int) uint64 {
	return mix64(mix64(globalSeed) ^ mix64(uint64(u)+goldenGamma))
}
