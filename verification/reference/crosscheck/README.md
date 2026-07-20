# meso reference cross-check (spec-blessing the oracle)

The one-time spec-blessing cross-check for the Lean value-oracle. It confirms the
thing a proof cannot check for itself: that the Lean formalisation of the quality
functions matches the reference implementations' definitions, not just that the Go
code will match the Lean formalisation. See
[../../../docs/specs/001-initial-implementation/meso-oracle.md](../../../docs/specs/001-initial-implementation/meso-oracle.md) for the design.

This is out of band. It is not part of any CI path. Re-run it only when a
reference version, the Lean formalisation, or the committed fixtures change.

## What it does

`crosscheck.py` reads each committed input fixture
(`verification/oracle/inputs/*.json`) and its Lean golden vector
(`verification/oracle/golden/*.json`), recomputes the quality of the same
partition with the references, and compares against the Lean exact rational value.

- Modularity: compared directly against igraph's `Graph.modularity` with a
  resolution parameter.
- CPM: compared directly against leidenalg's `CPMVertexPartition.quality()`; the
  oracle emits the canonical CPM convention (see the finding below).
- Connectivity: the golden `predicates.connected` flag is
  corroborated against igraph, requiring each community's induced subgraph to be
  connected (`induced_subgraph(...).is_connected()`). The other guarantee predicates
  (`gammaDense`, `gammaSeparated`, `subsetOptimal`) are meso-specific CPM guarantees the
  references do not compute, so they are emitted for the Go guarantee tests but not
  cross-checked here.

The Java `networkanalysis` CLI only produces a clustering; it cannot report the
quality of a given partition, so it cannot bless the quality-function value and is
not used here. (Blessing Java's value would require a small program against its
`QualityFunction` API rather than the jar's CLI.)

## Recorded result (2026-07-17)

References: igraph 0.11.8, leidenalg 0.10.2 (Python 3.11, pinned in the script's
PEP 723 inline metadata and provisioned by uv on a `debian:trixie-slim` base).
Oracle:
Lean toolchain `leanprover/lean4:v4.31.0`. Fixtures: `triangle`, `path3`,
`square`, and the real corpus `karate` (34), `dolphins` (62), and weighted
`lesmis` (77) (26 cases across both quality functions, several resolutions and
partitions, including each graph's anchor partitions, karate's ground-truth
two-faction split, and tiny-fixture cases that flip each guarantee predicate to
its failing branch).

Outcome: ALL PASS.

- Modularity is blessed directly: meso's modularity equals igraph's Newman
  modularity on every case, exactly (within float tolerance), including karate's
  ground-truth two-faction split at `1453/4056 ~ 0.3582`, the known literature
  value. The fixtures carry no self-loops, the one place the conventions could
  otherwise diverge.
- CPM is blessed directly, after alignment. meso now emits the canonical
  (leidenalg) CPM value, `cpmCanonicalQ`: meso's CPM restricted to distinct pairs
  `i != j`, i.e. with the self-pair diagonal excluded. It equals
  `CPMVertexPartition.quality()` on every case (for example karate all-in-one at
  `-966`, lesmis at `-4212`).
- Connectivity is corroborated: the golden `predicates.connected`
  flag agrees with igraph on every case, at corpus scale (each anchor partition's
  communities induce connected subgraphs), and on both branches, including the
  path3 `[0,1,0]` case whose disconnected community igraph confirms as
  `connected` false. The γ-density, γ-separation, and subset-optimality flags are
  emitted for the Go guarantee tests but not cross-checked, as the references do not
  compute them.

## Reproduce

From the repository root, in a container (the pinned, reproducible path):

```sh
docker build -t meso-reference-python verification/reference/crosscheck
docker run --rm -v "$PWD/verification/oracle:/work/oracle" meso-reference-python
```

`crosscheck.py` is a self-contained uv script: its shebang is
`#!/usr/bin/env -S uv run --script` and its dependencies are pinned in PEP 723
inline metadata, so with [uv](https://docs.astral.sh/uv/) installed it also runs
directly, resolving the references into an ephemeral environment:

```sh
uv run verification/reference/crosscheck/crosscheck.py verification/oracle
```

Exit status is `0` when every case agrees, `1` on any discrepancy. Regenerate the
golden vectors first (`make oracle-lean`) if the fixtures changed.

## Licensing

The references are GPL. The cross-check invokes them as separate processes in a
container and records only derived numbers and this note, never their code.
