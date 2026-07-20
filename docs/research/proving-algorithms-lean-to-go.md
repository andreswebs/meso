# Proving algorithms: from a Lean model to Go code

An explanation of what it means to formally prove an algorithm, and why a proof
written in Lean does not, on its own, say anything about the Go code that ships.
It bounds a single topic: the gap between a verified design and a deployed
implementation, and the ways that gap can be closed. It is background reading
for `meso`'s formal-verification tiers (see the plan's section 7), not
instructions for carrying them out.

## What "proving an algorithm" actually means

You do not prove an algorithm. A proof is always a proof of a proposition. The
accurate phrasing is that you prove a definition satisfies a specification.
Three ingredients are always present:

- A definition: the algorithm written as a function in Lean's logic, for example
  `leiden : Graph -> Partition`.
- A specification: a proposition stating what "correct" means for it. For Leiden
  that might be that the result is a valid partition, that modularity never
  decreases across a pass, that the output is deterministic given a fixed seed,
  and that the procedure terminates.
- A proof: a term whose type is that proposition. Lean's kernel type-checks it.
  If it checks, the proposition holds for that definition.

So "I proved Leiden" really means "I proved that this Lean function has these
properties." That distinction is the key to everything else here.

## The gap: a proof is about the Lean object, not the Go code

Stated plainly: a Lean proof says nothing, by itself, about a Go program. It is a
machine-checked fact about a mathematical object defined in Lean. The Go code is
a different artifact, and nothing transfers automatically.

The useful frame is the Trusted Computing Base (TCB): the set of things that must
be trusted for the belief "the Go code has property P" to be justified. A proof
does not eliminate trust; it shrinks and relocates it. The chain runs roughly:

1. The specification correctly captures what was meant. No tool checks this. A
   vacuous or wrong spec makes the proof worthless. This is the eternal weak
   link.
2. Lean's kernel is sound. It is small and heavily scrutinized, so this is
   reasonable to trust.
3. The Lean definition matches the algorithm intended.
4. The Go code faithfully implements the Lean definition. This is the gap.
5. The Go compiler, runtime, and hardware are correct.

When the algorithm is re-implemented in Go by hand, step 4 is bridged by human
translation plus testing, not by proof. So strictly, the Lean proof does not
carry over to Go. What one owns is a proven-correct design, plus an unproven but
testable claim that Go realizes that design.

## Three ways to bridge the gap

There are three general strategies, and they differ in whether they close the gap
by proof, by trust, or by evidence.

### Extraction and code generation (closes it by trust)

Some provers emit executable code from the verified definition: Coq to OCaml,
Isabelle to SML, Haskell, or Scala, `F*` to C (this is how the `HACL*`
cryptography in Firefox and the Linux kernel is produced), and Lean compiles
itself to C.
Running extracted code means running, modulo trusting the extractor, the same
thing that was proven. The costs are real: there is no mature Lean-to-Go
extraction, extracted code is rarely idiomatic or fast, and the extractor joins
the TCB. For targeting Go specifically, this route does not help today.

### Verifying the target code directly (closes it by proof)

Skip the model and prove properties about the real program using a verifier built
for that language: Dafny, Why3 and Frama-C for C, Verus and Creusot for Rust, `F*`
and `Low*`, and for Go specifically Gobra, a separation-logic verifier from ETH
Zurich. Here there is no translation gap, because the proof is about the actual
code. The cost is that Lean is no longer the tool and these verifiers are heavy.
For a library like `meso` this is likely overkill for functional correctness,
though Gobra is the honest answer to "can the Go itself be proven," and the plan
reserves it for data-race freedom of the parallel core.

### Proving the model, re-implementing, and testing the correspondence (closes it by evidence)

This is the pragmatic industrial standard and the right fit for `meso`: prove the
properties in Lean about a reference model, hand-write the same algorithm in Go,
and bridge step 4 with evidence rather than proof. The proof gives confidence
that the algorithm is correct; the tests give confidence that Go faithfully
realizes it. The two are complementary, and neither substitutes for the other.

## Why this fits meso, and what the strongest evidence-bridge looks like

`meso` is an unusually good candidate for the third strategy because its
correctness properties are already stated as theorems in the Leiden paper, so the
specification, the hardest and least checkable link in the TCB, is largely
pre-decided by the literature.

The strongest bridge, short of adopting a Go verifier for functional correctness,
comes from keeping the Lean model and the Go implementation as close as possible
and then attacking step 4 from several directions at once. Writing the Lean model
to mirror the Go structure, with the same representation choices where feasible,
makes the translation "obviously the same" and shrinks the residual trust. The
properties worth proving are the ones a naive implementation could plausibly get
wrong: determinism under a fixed tie-break rule, that the output is genuinely a
partition (a covering of the nodes with no node in two communities), modularity
monotonicity per pass, termination, and the Leiden-specific connectivity
guarantee that Louvain lacks.

That last property is the clearest illustration of a proof earning its keep at the
design level. Leiden exists precisely because Louvain can produce internally
disconnected communities, a bug that is silent (a worse partition, never a crash)
and easy to miss in testing. Forcing an invariant proof confronts exactly that
class of error before any Go is written.

The evidence that then holds the Go faithful to the proven model is the same
material developed elsewhere in `meso`'s research notes. Property-based tests
encode the proven properties directly on the Go artifact. Differential testing
runs the compiled Lean model and the Go code over a corpus and compares results,
which is the most direct possible test of the translation itself; this is the
Lean value-oracle role described in
[specs/001-initial-implementation/meso-oracle.md](../specs/001-initial-implementation/meso-oracle.md), and the move-delta identity in
[move-delta-verification.md](move-delta-verification.md) is one concrete instance
of it. An explicit correspondence table, mapping each Lean definition to the Go
function that implements it, lets a reviewer audit the translation that step 4
depends on.

## Perspective

- Prefer "prove that this definition meets this specification" over "prove the
  algorithm." "Verify the algorithm" is fine colloquially, but the precise
  phrasing keeps the model-versus-code gap in view.
- A Lean proof is a guarantee about the design, not the deployment. It never
  transfers to Go automatically.
- Of the three bridges, only verifying the Go directly closes the gap by proof;
  extraction closes it by trust in the extractor, and re-implementation closes it
  by evidence. For a heuristic library, a verified design plus an exhaustively
  tested implementation plus proven race-freedom is a defensible
  cost-to-assurance trade, and it is the posture the plan adopts.
- Even a proof that never touches the Go pays for itself, because it forces the
  specification to be precise and it catches design-level bugs that testing tends
  to miss.

## Related reading

- [move-delta-verification.md](move-delta-verification.md): the incremental-gain
  identity, a concrete property proven in Lean and tested in Go.
- [specs/001-initial-implementation/meso-oracle.md](../specs/001-initial-implementation/meso-oracle.md): where the executed Lean model
  serves as `meso`'s sole standing value-oracle (the cross-language differential
  harness having been retired).
- The plan's section 7 (`docs/meso-design.md`): the tiered formal-verification
  posture (V0 through V3, Gobra, and TLA+) this discussion underpins.
