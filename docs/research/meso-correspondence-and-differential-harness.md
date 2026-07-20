# meso: correspondence documentation and differential-testing harness

> **Status note (2026-07-16, superseded in part).** The oracle design of record
> is now [../specs/001-initial-implementation/meso-oracle.md](../specs/001-initial-implementation/meso-oracle.md). The decision: the Lean
> model is `meso`'s sole standing numeric oracle. **Bridge C** below (the
> cross-language differential harness that commits three-way golden vectors) is
> **retired**; the references are kept only for porting and a one-time
> spec-blessing cross-check. **Bridge B'** (the computable Lean oracle), sketched
> here as optional, is **promoted to the numeric oracle of record** (Phase F in
> [../../verification/lean/README.md](../../verification/lean/README.md)). Bridges A
> and B (correspondence documentation and guarantee property tests) are unchanged.
> Read the rest of this document as the reasoning that led there, not as the
> current plan.

A concrete sketch of the two artifacts that bridge the "step 4" gap for meso
(see [proving-algorithms-lean-to-go.md](proving-algorithms-lean-to-go.md)): the
Go code faithfully implements the Lean model. Grounded in what exists today
(`verification/lean/Meso/{Graph,Quality,Move}.lean`, the plan sections 3, 4, 6, 7) rather than a generic template.

## The project-specific twist you have to design around

Two facts about meso change the generic advice:

1. **The Lean model is deliberately noncomputable.** `modularity` in
   `Meso/Quality.lean` is `noncomputable def ... : ℝ`, defined over the reals,
   and `degree`/`twoM` are real-valued sums. You cannot `lake exe` it and get a
   number. So Lean is **not** your numeric oracle out of the box.
2. **The plan's differential oracle is cross-language, not cross-prover.** Per
   plan 6.2, the numeric oracle is the Java `networkanalysis` and Python
   `leidenalg`/igraph references, run in a Docker harness, frozen to golden
   vectors. Lean's job is to pin the _exact statements_ of the invariants; Go
   then property-tests those same statements empirically.

So for meso there are three distinct bridges, and the document should keep them
separate:

| Bridge                         | What it connects            | Mechanism                                   | Trust it removes                                   |
| ------------------------------ | --------------------------- | ------------------------------------------- | -------------------------------------------------- |
| A. Structural correspondence   | Lean defs ↔ Go symbols      | Hand-audited mapping table                  | "Go models the same objects"                       |
| B. Guarantee property tests    | Lean theorems ↔ Go tests    | Same proposition, checked empirically in Go | "Go preserves the proved invariants"               |
| C. Cross-language differential | Go ↔ Java/Python references | Golden vectors + delta comparison           | "Go's numbers match an independent implementation" |

Bridge C is the plan's section 6. Bridges A and B are what "correspondence
documentation" means. An optional **Bridge B'** (a computable Lean oracle) is
sketched at the end for when property tests are not enough.

---

## Part 1: Correspondence documentation

Lives at `verification/lean/CORRESPONDENCE.md`, next to the model it describes,
and is treated as a reviewed artifact (a wrong row here is exactly the residual
trust we are trying to shrink). Three tables.

### 1a. Representation correspondence

Maps each mathematical object to its Go realization. The point is to make each
row "obviously the same" so a reviewer can check it by eye.

| Lean (`Meso`)                         | Go (`package meso`)                                                     | Notes on the encoding gap                                                                                        |
| ------------------------------------- | ----------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| `WeightedGraph n` over `Fin n`        | internal `csr` (offset/neighbor/weight arrays) over dense `int` indices | `Fin n` ↔ `[0,n)` dense indices; the builder's key→index map is outside the model                                |
| `weight : Fin n → Fin n → ℝ`          | `csr` adjacency lookup, `float64`                                       | dense function ↔ sparse CSR; symmetry (`weight_symm`) is a Go builder invariant, asserted in tests               |
| `weight_nonneg`, `nodeSize_nonneg`    | validated in `Build()`, returns error on negative                       | model assumes them as fields; Go must _enforce_ them at the boundary                                             |
| `nodeSize : Fin n → ℝ`                | `csr.nodeSize []float64`                                                | preserved through aggregation (plan 4.1); the correspondence must be re-checked for the aggregate graph          |
| `degree`, `twoM`                      | `csr.degree(i)`, `csr.twoM()`                                           | real sum ↔ `float64` sum; canonical (sorted) summation order is the FP property that matters (plan 7)            |
| `Partition n := Fin n → ℕ`            | `[]int` (community label per node)                                      | total function ↔ length-`n` slice; well-formedness is structural in Lean, a slice-length + range invariant in Go |
| `modularity G γ p` (ℝ, noncomputable) | `Modularity.Quality(...)` `float64`                                     | reals ↔ float; **not** bit-identical, compared within delta (see Bridge C)                                       |
| `move p v c = Function.update p v c`  | `localMove` single reassignment                                         | `Function.update` ↔ one slice write; direct                                                                      |

### 1b. Theorem-to-test correspondence (Bridge B)

Every proved (or planned) Lean theorem gets a named Go test that checks the same
proposition on the corpus and on fuzzed inputs. This is the row-by-row heart of
the doc.

| Lean theorem                                 | Statement                                                                     | Go guarantee test                                                                                   | Status              |
| -------------------------------------------- | ----------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- | ------------------- |
| `move_self`                                  | moving a node to its own community is a no-op                                 | `TestLocalMove_SelfIsNoOp`                                                                          | Lean: proved        |
| `modularity_bestMove_ge`                     | a best single-node move never lowers modularity (staying is always an option) | `TestLocalMove_BestMoveNonDecreasing` (assert `Q(after) >= Q(before) - eps` for each accepted move) | Lean: proved        |
| `IsLocalMove.modularity_le`                  | one local-move step is monotone                                               | folded into the above per-step assertion                                                            | Lean: proved        |
| `localMoveRun_monotone`                      | a whole local-move sweep is monotone                                          | `TestLocalMove_SweepMonotone` (record `Q` after each sweep, assert non-decreasing)                  | Lean: proved        |
| `quality_monotone_of_stepwise`               | global monotonicity across levels from stepwise                               | `TestLeiden_QualityMonotoneAcrossLevels` (plan 6.3)                                                 | Lean: proved        |
| _(planned)_ `Aggregate` quality non-decrease | aggregation does not lower quality                                            | `TestAggregate_QualityPreserved`                                                                    | Lean: open (README) |
| _(planned)_ `Connectivity`                   | every community is a connected subgraph                                       | `TestLeiden_CommunitiesConnected` (plan 6.3)                                                        | Lean: open          |
| _(planned)_ termination                      | iteration halts                                                               | Go: bounded-iteration test + no-timeout fuzz                                                        | Lean: open          |

The discipline: **no Lean theorem is considered "landed" until its
correspondence row names a green Go test.** A proved theorem with no Go test is
a guarantee about the model that the shipped code might silently violate.

### 1c. Divergence register

An explicit list of places where Go intentionally departs from the model, so
reviewers know these are decisions, not bugs:

- Go uses `float64`; the model uses ℝ. Consequence: comparisons are delta-based,
  and summation order is canonicalized (sorted adjacency) so the delta is tight
  and stable.
- The model assumes symmetry/nonnegativity as structure; Go validates them at
  `Build()` and returns an error. The "garbage in" cases the model never sees
  are Go's responsibility.
- Directed modularity (plan 4.3) and CPM have no Lean model yet; their rows are
  marked "unmodelled, tested only against references."

---

## Part 2: The differential-testing harness

Two independent things share the word "differential." Keep them in separate
directories.

### 2a. Cross-language reference differential (Bridge C, the plan's 6.2)

```txt
verification/
  oracle/
    Dockerfile            # JVM + networkanalysis.jar, Python + leidenalg + igraph
    run_corpus.py         # runs the corpus at fixed seeds, emits vectors.json
    corpus/               # karate.edgelist, dolphins, lesmis, lfr/*
  testdata/
    golden/
      karate.modularity.json
      dolphins.modularity.json
      ...
```

The Docker oracle runs offline and on demand (a `make oracle-refresh` target),
committing golden vectors. Main CI stays pure-Go and reads the committed
vectors, so the JVM/Python toolchain is never on the fast path.

Golden vector schema (one file per graph+params, three-way to establish the
"reference envelope" of plan 5):

```json
{
  "graph": "karate",
  "params": { "quality": "modularity", "resolution": 1.0, "seed": 42 },
  "references": {
    "networkanalysis": { "quality": 0.4198, "num_communities": 4 },
    "leidenalg": { "quality": 0.4188, "num_communities": 4 }
  },
  "envelope": { "quality_min": 0.4188, "quality_max": 0.4198 },
  "ground_truth_nmi_min": 0.85
}
```

The Go side compares within the envelope, never exact partitions (different
PRNGs make exact impossible cross-language, plan 6.2):

```go
func TestDifferential_ReferenceEnvelope(t *testing.T) {
    for _, gv := range loadGoldenVectors(t, "testdata/golden") {
        g := loadCorpusGraph(t, gv.Graph)
        part, err := meso.Leiden(g,
            meso.WithQuality(meso.Modularity(gv.Params.Resolution)),
            meso.WithSeed(gv.Params.Seed))
        require.NoError(t, err)

        q := part.Quality()
        // Within the reference envelope, not equal to any single reference.
        assert.GreaterOrEqual(t, q, gv.Envelope.QualityMin-qualityDelta)
        assert.LessOrEqual(t, q, gv.Envelope.QualityMax+qualityDelta)
        assert.Equal(t, gv.References.NetworkAnalysis.NumCommunities, part.NumCommunities())
        assertAllCommunitiesConnected(t, g, part) // the guarantee, not a number
        if gv.GroundTruthNMIMin > 0 {
            assert.GreaterOrEqual(t, nmi(part, groundTruth(gv.Graph)), gv.GroundTruthNMIMin)
        }
    }
}
```

What is compared (plan 6.2): quality within a tight delta, community count, the
connectivity guarantee, NMI/ARI against planted ground truth. Not exact
partitions.

### 2b. Guarantee property tests (Bridge B)

These encode the Lean theorem statements from table 1b directly, in Go, over
fuzzed inputs. Use native `go test -fuzz` plus a PBT library (`rapid`) for
structured generation. Example mirroring `localMoveRun_monotone`:

```go
func TestLocalMove_SweepMonotone(t *testing.T) {
    rapid.Check(t, func(r *rapid.T) {
        g := genWeightedGraph(r) // symmetric, nonneg, sizes >= 0: the WeightedGraph fields
        run := meso.LeidenTrace(g, meso.WithSeed(rapid.Int64().Draw(r, "seed")))
        for i := 1; i < len(run.QualityByLevel); i++ {
            // localMoveRun_monotone / quality_monotone_of_stepwise, empirically:
            if run.QualityByLevel[i] < run.QualityByLevel[i-1]-eps {
                r.Fatalf("quality decreased at level %d: %v -> %v",
                    i, run.QualityByLevel[i-1], run.QualityByLevel[i])
            }
        }
    })
}
```

The generator `genWeightedGraph` is the Go analogue of the `WeightedGraph`
structure fields: enforce `weight_symm`, `weight_nonneg`, `nodeSize_nonneg` in
the generator so the tested domain matches the model's domain exactly. A
generator that produces graphs the model excludes is a silent correspondence
break.

### 2c. Optional Bridge B': a computable Lean oracle

Property tests sample; they can miss a case a proof would catch. If you want an
actual runnable Lean oracle (not just cross-language references), the noncomputable
model blocks you. The standard move:

1. Add a **computable** rational mirror in Lean, e.g. `Meso/Compute.lean`:
   `def modularityQ (G : WeightedGraphQ n) (γ : ℚ) (p : Partition n) : ℚ` over ℚ
   with `Fin n → Fin n → ℚ` weights.
2. **Prove it agrees** with the real model on rational inputs:
   `theorem modularityQ_eq (…) : (modularityQ G γ p : ℝ) = modularity G.toReal γ p`.
   This is the load-bearing step: it makes the runnable thing provably the same
   as the proved thing.
3. `lake exe mesoOracle` reads corpus graphs (rational weights), prints
   `modularityQ` as an exact fraction.
4. A Go test diffs `part.Quality()` (float) against the exact rational within a
   float-rounding delta.

This buys you an oracle whose correctness is itself proved, closing the numeric
gap by proof rather than by a second independent implementation. It costs a
parallel computable model and the equivalence proof, so it is a later tier, not
a v1 need. The cross-language references (2a) are the cheaper numeric oracle and
come first.

### 2d. Determinism differential (plan 6.4)

Not a model bridge, but belongs in the harness: run each algorithm N times at a
fixed seed and assert byte-identical partitions across serial/parallel and core
counts. This is what guards the section 4.5 synchronous-round scheme; the Lean
V2 confluence proof (README) is the design-level counterpart.

```go
func TestDeterminism_ByteIdenticalAcrossCores(t *testing.T) {
    g := loadCorpusGraph(t, "karate")
    want := runLeiden(t, g, seed, 1) // 1 goroutine
    for _, cores := range []int{2, 4, 8} {
        got := runLeiden(t, g, seed, cores)
        assert.Equal(t, want.CanonicalBytes(), got.CanonicalBytes())
    }
}
```

---

## Wiring it together

Directory layout:

```txt
verification/
  lean/           # the model + CORRESPONDENCE.md (Part 1)
  oracle/         # Docker cross-language oracle (2a)
  testdata/golden # committed reference vectors (2a)
guarantee_test.go # Bridge B property tests (2b)
differential_test.go
determinism_test.go
```

Make targets (fold into the existing `make validate` fan-out per CLAUDE.md):

| Target                | Runs                                                  | On CI?       |
| --------------------- | ----------------------------------------------------- | ------------ |
| `make test`           | property (B), differential-vs-golden (C), determinism | yes, pure-Go |
| `make oracle-refresh` | Docker oracle, regenerates golden vectors             | on demand    |
| `make lean`           | `lake build` + `#print axioms` check                  | separate job |
| `make oracle-lean`    | build + run the computable Lean oracle (B', if built) | on demand    |

The invariant that keeps the whole thing honest: a proved Lean theorem, a row in
`CORRESPONDENCE.md`, and a green Go guarantee test are added together. Any one of
the three without the other two is a gap, and the register in 1c is where the
known, deliberate gaps live so they cannot masquerade as coverage.
