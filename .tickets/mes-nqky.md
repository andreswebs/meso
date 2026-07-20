---
id: mes-nqky
status: closed
deps: [mes-orqz, mes-qvav]
links: [mes-5wqp, mes-vy3a, mes-wmzq]
created: 2026-07-17T02:32:15Z
type: task
priority: 1
assignee: Andre Silva
parent: mes-t76u
tags: [core, oracle, golden, verified]
---

# Go value-oracle harness: load Lean golden vectors, assert float64 within tolerance

The Go consumer of the Lean value-oracle: a reusable test harness that loads the committed golden vectors (`verification/oracle/golden/*.json`), builds each fixture graph via the `Builder`, runs the Go quality functions and move-deltas on each case's partition, and asserts the `float64` result lands within a float-rounding tolerance of the exact rational value the Lean oracle emits. It also exposes the per-case predicate flags (`connected`, `gammaDense`, `gammaSeparated`, `subsetOptimal`) for the guarantee tests to consume. This is the Go side of Phases F and G: the Lean side (the `mesoOracle` exe, exact rational vectors, and the igraph/leidenalg spec-blessing cross-check) is delivered and committed; this ticket builds the loader and tolerance assertions that the move-delta and guarantee tickets reuse. Design of record: `docs/specs/001-initial-implementation/meso-oracle.md`; `CORRESPONDENCE.md` divergence register (`float64` vs R).

## Design

Parse the golden JSON schema: `n`, `edges`, and per case {`quality`, `gamma`, `partition`, `value` {`num`,`den`,`approx`}, `deltas` [{`node`,`target`,`value`}], `predicates` {`connected`,`gammaDense`,`gammaSeparated`,`subsetOptimal`}}. Represent exact values as `big.Rat` from the `num/den` strings and compare Go `float64` against `big.Rat` within a per-quality tolerance (start `1e-9`; document the rationale). Build the graph via the `Builder` from the matching `inputs/*.json` (weights and sizes are ints or `p/q` strings, never floats; each undirected edge listed once; a self-loop is `i==j`). Expose a table-driven helper the quality, move-delta, and guarantee tests call, so the committed golden vectors are the single source of truth. `subsetOptimal` is null above the emitter's node bound (`maxSubsetOptimalN`); treat null as skip. Do NOT re-derive vectors in Go: regeneration is the Lean track (`make oracle-lean`); this ticket only consumes committed files. Keep the loader in an internal test-only helper package.

## Acceptance Criteria

TDD order. 1) The loader parses every committed golden file (triangle, path3, square, karate, dolphins, lesmis) without error, and exact `num/den` round-trip into `big.Rat`. 2) For each modularity case, the Go modularity of the case partition is within tolerance of the golden value; likewise CPM (canonical convention) for each cpm case. 3) For each case carrying `deltas`, the Go incremental move-delta is within tolerance of the golden exact delta (reused by the move-delta tickets). 4) Predicate flags load and are exposed to callers; a null `subsetOptimal` is skipped, not failed. 5) A deliberately perturbed Go quality (epsilon above tolerance) makes the harness fail, proving the check has teeth. 6) `make validate` and `make test-race` green. NOTE: producing or refreshing the vectors is the Lean value-oracle track (`make oracle-lean`), not this ticket; this ticket only consumes committed files.

## Notes

**2026-07-17T21:22:27Z**

Go value-oracle harness implemented as test-only helpers in package meso (oracle_test.go loader + comparison policy; oracle_harness_test.go acceptance tests). Loads inputs/*.json and golden/*.json, joins cases by index (with quality/gamma cross-check), builds each graph via the public Builder (nodes 0..n-1 registered in order so dense index == input index). Comparison uses the decided ULP+atol policy: reference is the correctly-rounded big.Rat (Float64), pass iff ulpDiff<=budget (16 value / 4 delta) OR |err|<=1e-12. Modularity compared directly (matches igraph); CPM compared in canonical convention via canonicalCPM = cpmQ - sum_i(w_ii - gamma*s_i^2) (cpmCanonicalQ_eq); CPM move-delta needs no correction since the diagonal is partition-independent. All 6 fixtures (triangle,path3,square,karate,dolphins,lesmis) green for values, deltas, and both predicate branches; subsetOptimal is *bool (null=skip). Teeth test proves a 1e-6 perturbation fails. make validate + test-race + build all green. Weights/sizes/gamma parse through big.Rat accepting ints or p/q strings. Blocking child mes-xb3w (modularity-optimal partition in input set) remains, gated on the Leiden core.
