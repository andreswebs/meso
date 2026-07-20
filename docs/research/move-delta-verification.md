# Move-delta verification

The local-move step of Louvain and Leiden does not recompute the quality
function from scratch. It computes the _gain_ of moving a node `i` from its
current community into a candidate community `c` using an incremental formula.
Correctness of the whole optimizer rests on one identity:

> The incremental gain `delta` computed for a move equals
> `Q(after) - Q(before)` exactly (up to floating-point tolerance).

This is the most bug-prone line in any modularity optimizer: it is an algebraic
simplification of a difference of sums, and the null-model term is easy to get
subtly wrong (wrong factor of `2m`, self-loops mishandled, degree of the moved
node double-counted). Getting it wrong produces an optimizer that still runs and
still returns communities, just wrong ones, which makes the bug hard to notice
without a dedicated check.

Two layers cover this identity:

- **Now (test harness):** a property-based test in Go, specified below.
- **Later (verification tier V1):** a Lean proof of the identity as a theorem
  over the formal definition of the objective. See the
  [notation cheat-sheet](notation-cheatsheet.md) and the plan's section 7. The
  Lean proof gives the airtight "for all graphs" guarantee that property testing
  only approximates; the property test gives immediate, CI-cheap coverage.

## Property-based test specification

One test per objective (`Q`, `Q_gamma`, CPM, directed modularity), because each
has its own delta formula. Same structure for all.

### Generator

Produce a random instance per trial:

- A random graph: `n` in a small range (say 2 to 50 nodes); edges added with a
  random probability, or a random edge count. Include, across trials:
  - weighted edges (not just 0/1), since the weight paths differ from the
    unweighted paths;
  - self-loops, because they are the classic off-by-a-factor case;
  - at least some disconnected graphs and isolated nodes.
  - for the directed objective, asymmetric edge sets.
- A random starting partition of the `n` nodes into communities.
- A random legal move: pick a node `i` and a target community `c` (which may be
  a currently empty community, i.e. `i` moving out on its own, and may be `i`'s
  current community, i.e. a no-op that must yield `delta = 0`).
- For parameterized objectives, a random resolution `gamma` drawn from a range
  that includes values below and above 1.

Seed the generator deterministically and log the seed on failure so any
counterexample is reproducible, consistent with `meso`'s determinism goal.

### Oracle and assertion

For each trial:

1. Compute `Q_before` with a direct, from-scratch implementation of the
   objective (the simplest correct definition, not the incremental one). This
   direct evaluator is the oracle and should be dead simple even if slow.
2. Apply the move to get the new partition; compute `Q_after` directly.
3. Compute `delta` with the incremental move formula under test.
4. Assert `abs(delta - (Q_after - Q_before)) <= tol`.

Choose `tol` relative to the magnitudes involved (an absolute epsilon such as
`1e-9` is usually fine at these graph sizes; scale it if sizes grow). Because
the oracle recomputes from scratch, small graphs are sufficient and keep each
trial cheap; run many trials (thousands) rather than large graphs.

### Cases to assert explicitly (beyond random trials)

- No-op move (`c` is `i`'s current community): `delta == 0` exactly.
- Moving the only node of a singleton community to another community, and the
  reverse.
- Self-loop on the moved node: exercised by the generator, but worth a fixed
  regression case once a correct value is known.
- Empty-target move (node leaves to form its own singleton).

### Directed objective note

For directed modularity the move changes both in- and out-degree contributions;
the oracle must use the directed definition (`k_i^out k_j^in / m`) and the delta
formula must be the directed one. Do not reuse the undirected oracle.

## Relationship to the Lean target

The Go property test and the Lean theorem state the same identity at different
strengths. When the V1 Lean proof lands, keep the property test: it guards the
Go implementation against divergence from the proven math (the proof is about
the definition, not the code), and it runs in CI where the proof does not.

There is a stronger option once the Lean model exists: use the executed Lean
definition as the value-oracle for the expected `delta`, rather than a
from-scratch Go evaluator. The identity is proven for all graphs in Lean, so the
Go test then checks that Go's incremental delta reproduces the proven Lean
formula on generated inputs, which is exactly the oracle role. Go still owns the
inputs; Lean emits the expected value over the rationals. This is now the design
of record, not an option: the Lean value-oracle is `meso`'s sole standing numeric
oracle. See [specs/001-initial-implementation/meso-oracle.md](../specs/001-initial-implementation/meso-oracle.md).
