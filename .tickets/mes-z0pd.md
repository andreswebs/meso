---
id: mes-z0pd
status: closed
deps: [mes-hcvp]
links: []
created: 2026-07-14T03:40:30Z
type: task
priority: 1
assignee: Andre Silva
parent: mes-t76u
tags: [core, determinism, step-5]
---

# Determinism scaffolding: PRNG, canonical order, per-node seeded randomness

The determinism substrate: a seedable PRNG we own, canonical internal iteration order with stable tie-breaks, per-node refinement randomness derived from `hash(globalSeed, nodeID)`, and a canonicalization helper for order-independent inputs. Makes output a pure function of (input, seed, params). Design of record: `docs/meso-design.md` sections 4.4 and 4.5; step 5 of `docs/specs/001-initial-implementation/plan.md`.

## Design

Own the PRNG (do not rely on `math/rand` global state) so streams are reproducible and independent of Go version. Canonical order: sort adjacency, break ties by node index, everywhere the algorithm iterates. Per-node randomness `hash(globalSeed, nodeID)` makes refinement (M2) scheduling-independent, which is also what the parallel round (M4, `verification/lean/Meso/Round.lean`) relies on. Canonicalization helper relabels differently-ordered inputs to a canonical form so shuffled edge insertion yields identical output. No Lean theorem-to-test row here (determinism-by-sorting is the float property Lean deliberately keeps out of scope, `docs/meso-design.md` section 7); it underlies the parallel confluence tests in M4.

## Acceptance Criteria

TDD order. 1) Byte-identical Louvain partition across repeated runs at a fixed seed. 2) Differently-ordered inputs (shuffled edge insertion) canonicalize to identical output. 3) The owned PRNG produces the same stream for the same seed regardless of call site interleaving. 4) `hash(globalSeed, nodeID)` is stable and independent of node processing order (same value whichever order nodes are visited). `make validate` and `make test-race` green.

## Notes

**2026-07-17T21:08:54Z**

Determinism scaffolding delivered in prng.go + Builder.Canonical() + determinism_test.go/prng_test.go.

- Owned PRNG: splitmix64 (prng.go). newPRNG(seed) + next(); shared mix64 finalizer. Deliberately NOT math/rand (global state, cross-version drift). Verified our stream == canonical splitmix64 reference values (Vigna) for seed 0 and pinned them in TestPRNG_GoldenStream as a portability guard.
- Per-node randomness: nodeSeed(globalSeed, u) = mix64(mix64(seed) ^ mix64(u+goldenGamma)). Pure function of (seed,u), so order/scheduling independent (AC#4). No nodePRNG helper yet - refinement (M2/mes-w7ko) composes newPRNG(nodeSeed(seed,u)) at its first real call site (removed to keep lint's unused check green; no speculative code).
- Canonical order for inputs: Builder.Canonical() opt-in assigns dense indices by ascending key instead of first-seen, via densePositions() permutation applied through Build. Default first-seen path unchanged (builder_test still green). Shuffled edge insertion -> byte-identical model + partition (AC#2). Adjacency was already sorted; the remaining insertion-order dependence was the index assignment itself.
- Louvain byte-identical across repeated runs (AC#1) locked in TestLouvain_ByteIdenticalRepeated over corpus+200 fuzzed graphs x {modularity,cpm}; Louvain still carries no PRNG.
- make validate + make test-race green. No Lean theorem row (determinism-by-sorting is float property Lean keeps out of scope, design 7).
