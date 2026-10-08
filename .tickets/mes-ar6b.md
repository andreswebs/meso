---
id: mes-ar6b
status: open
deps: [mes-i21e]
links: []
created: 2026-10-07T16:34:15Z
type: task
priority: 2
assignee: Andre Silva
parent: mes-0jzi
tags: [centrality, datasets, step-6, implementation]
---
# networkx betweenness reference CSVs for karate, dolphins, lesmis, celegansneural

Step 6 of `docs/specs/002-structural-measures/plan.md`, parent mes-0jzi. External reference values for `Betweenness` (mes-i21e) on the corpus, at full float precision.

## Design

A PEP 723 script under `datasets/` (run with `uv run`, depends on networkx) reads a corpus GML and writes `betweenness.csv` (key, value with full `repr` precision) beside it, for `karate`, `dolphins`, `lesmis` (undirected) and `celegansneural` (directed). It calls `betweenness_centrality(G, normalized=True)` with no weight argument, matching meso's unweighted semantics. Commit the script and the four CSVs. A Go test loads each graph with `loadGMLGraph` (`golden_test.go`) and compares. The script is out-of-band tooling like `verification/oracle/tools/gml_to_input.py`; CI stays pure Go.

## Evidence (verified 2026-10-07)

- `datasets/karate/karate.gml` uses 1-based ids, so meso keys are `"1"` to `"34"`; the published values are node 1 about 0.4376, node 34 about 0.3040, node 33 about 0.1452.
- `datasets/celegansneural/celegansneural.gml` declares `directed 1` and carries weights; no corpus GML declares `multigraph`.
- `loadGMLGraph` builds with `AddNodeWeight(id, 1)` per node and `AddEdge(src, tgt, w)` per edge.

## Warnings

- The networkx graph must be a simple `Graph`/`DiGraph` so parallel edges fold as meso folds them; if `read_gml` raises on a duplicate edge, read as a multigraph and convert to a simple graph explicitly.
- Self-loops must not change the networkx result either; drop them in the script if networkx counts them differently.
- Keys in the CSV must be the GML `id`, the same key `loadGMLGraph` uses, not the `label`.

## Acceptance Criteria

1) All 34 karate values within 1e-9 of networkx; the three published spot values match to 4 decimals. 2) dolphins, lesmis and celegansneural within 1e-9. 3) A CSV round-trip check: every CSV key is a graph key and vice versa. 4) `DATASETS.md` documents the new files and the regeneration command. `markdownlint-cli2` clean on `DATASETS.md`. `make validate` green in both modules (fmt-check, vet, lint, test).
