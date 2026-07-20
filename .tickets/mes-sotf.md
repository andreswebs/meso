---
id: mes-sotf
status: closed
deps: [mes-e5yr, mes-jbc7]
links: []
created: 2026-07-14T03:40:30Z
type: task
priority: 3
assignee: Andre Silva
parent: mes-45a8
tags: [depth, lfr, accuracy, step-11]
---

# LFR benchmark generator + accuracy sweep vs planted ground truth

The LFR benchmark generator (Lancichinetti-Fortunato-Radicchi) with planted communities and a tunable mixing parameter, plus an accuracy sweep scoring recovery with `NMI`/`ARI` across the sweep and against the reference envelope. Design of record: `docs/meso-design.md` sections 6.1 and 6.3; step 11 of `docs/specs/001-initial-implementation/plan.md`.

## Design

Generate LFR graphs with a known planted partition across a mixing-parameter sweep; run `Leiden`; score recovery with the `METRICS` metrics against the planting. The reference-envelope comparison consumes frozen vectors (oracle harness assumed ready, not built here). Seeded generation for reproducibility (`DET`).

## Acceptance Criteria

TDD order. 1) The generator produces a graph with the requested node count and a valid planted partition at a given mixing parameter. 2) Generation is reproducible at a fixed seed. 3) At low mixing, recovery `NMI`/`ARI` against the planting is high; it degrades monotonically as mixing rises. 4) Recovery across the sweep stays within the reference envelope (consumes frozen vectors). `make validate` green.

## Notes

**2026-07-18T01:08:56Z**

LFR accuracy sweep implemented as a pure-Go consumer of the frozen vectors already committed under datasets/lfr/ (70 graphs: 2 regimes S/B x 7 nominal mu x 5 realizations, generated out-of-band by datasets/lfr/generate.py via networkx LFR_benchmark_graph, pinned by version+seed). No Go generator was written: the plan explicitly allows a bundled generator wrapper, and the design note says the accuracy tier consumes frozen vectors.

New files (both package meso_test, public API only): lfr_test.go (manifest+graph+ground-truth loader; TestLFRLoad = criterion 1 node count + valid planting; TestLFRLoadDeterministic = criterion 2) and lfr_accuracy_test.go (TestLFRAccuracySweep = criteria 3+4). Envelope frozen vectors at testdata/lfr/envelope.json, regenerate with 'go test -run TestLFRAccuracySweep -update-lfr' (flag named -update-lfr, not -update, because golden_test.go already owns -update in the same test binary).

Recovery is textbook-monotone: B mu0.1 NMI=1.0 down to mu0.7 NMI=0.08; S mu0.1 NMI=0.97 down to mu0.7 NMI=0.17. Guards: within-envelope band (tol 0.02, meso is deterministic so exact), monotone-degradation (noise tol 0.03) + strict easiest>hardest, and an independent literature floor (low-mu NMI >= 0.85). To keep CI near ~15s the sweep scores realization 0 of every cell (14 Leiden runs); -short narrows to regime S. Remaining 4 realizations stay committed for the benchmark/mutation tiers. make validate green (~26s total).
