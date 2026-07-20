# Directed guarantee scout (Phase 3 triage harness)

The counterexample-scouting harness for the directed-modularity verification
project's Phase 3 (guarantee triage). It generates random directed graphs
across a parameter grid, runs meso's directed Leiden through the public API
only, drives each returned partition to a verified move-and-merge fixed point
with its own independently recomputed objective, and checks the three
candidate directed guarantees: weak connectivity of returned communities,
gamma-separation, and subset-optimality. The full mathematical record of the
verdicts and derivations is the research doc
`docs/research/directed-modularity-triage.md` in the repository; this README
is the tool-local summary.

This is out of band, the Go analogue of `../crosscheck/crosscheck.py`: a standalone
module (its `go.mod` resolves meso from the working tree via a `replace`
directive), never part of any CI path or the `make validate` gate. Being a
separate module is load-bearing: it can only import meso's exported surface,
so every bound is recomputed independently from the generated edge list plus
the returned partition, and a shared formula bug cannot blind it. Re-run it on
demand, e.g. if the directed objective or the directed pipeline changes.

## How to run

```sh
go run .                 # full grid, prints per-guarantee counts
go run . -hunt-subset    # search small graphs for a minimal subset
                         # counterexample and print it as an edge list
```

A startup self-test pins the scout's objective to the hand-computed fixture
values of `directed_quality_test.go` (also machine-checked in Lean) and
verifies the symmetric reduction of the bound expressions; the program aborts
if any of it fails. Seeds derive deterministically from grid coordinates, so
every logged hit is reproducible from its config line.

## Parameter grid

3240 samples: n in {5, 8, 12, 16, 24, 40}; arc density in {0.1, 0.3, 0.5};
weights in {unit, integer 1..5, continuous (0,1]}; regimes {uniform, DAG,
source/sink, cycle-seeded, symmetric}; self-loops on and off; gamma in
{0.5, 1, 2}; 2 seeds per cell. Subset checks are exhaustive for communities
up to 12 members and sampled (500 subsets) above.

## Recorded result (2026-07-18)

meso at working tree of this date; python-igraph 1.0.0 for the convention
check (`.local` script, not committed). Outcome, over 3240 runs and 15228
returned communities:

- Scout-vs-meso convention: the scout's independent directed Q matched
  `Result.Quality()` on every sample (max error 1.1e-16), and igraph's
  `modularity(..., directed=True)` matched exactly on all fixture cases.
- Weak connectivity: 0 violations. Strong connectivity: 5880 of 15228
  communities fail (39 percent), so the guarantee is weak connectivity.
- Gamma-separation: 0 violations at raw outputs and at verified-converged
  partitions; minimum slack 0 (tight, not vacuous). Merge closed form vs
  from-scratch Q difference: max error 4.3e-15.
- Subset-optimality: REFUTED as an output property. 110 violating subsets
  (55 distinct splits) in 43 of 3240 samples at verified-converged
  partitions, including 8 symmetric-regime samples (so it is inherited
  modularity behaviour, not a directed regression). Minimal fixture: n=4,
  found by `-hunt-subset`, hand-verified, recorded in the research doc.
  Split closed form vs from-scratch: max error 7.1e-15.
- Convergence driving: 396 of 3240 raw outputs admitted a further improving
  single-node move or merge under the scout's exhaustive scan (meso stops at
  its epsilon over neighbour candidates); 0 samples discarded.
