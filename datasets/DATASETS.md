# meso test-corpus datasets

Canonical small graphs with known community structure, for the correctness and
accuracy tiers of the test suite (plan section 6.1). All are downloaded from
Mark Newman's network data repository
(<https://websites.umich.edu/~mejn/netdata/>) in GML format; each directory keeps
the original `.gml` plus the upstream `.txt` provenance/citation note.

GML is trivial to parse (node/edge blocks with `id`, `label`, `value`, `weight`);
`meso`'s fixtures can read it directly or a small converter can emit the CSR the
core expects.

## Inventory

| Dataset        | Nodes | Edges | Weighted | Directed | Ground truth in file       | Source paper       |
| -------------- | ----: | ----: | -------- | -------- | -------------------------- | ------------------ |
| karate         |    34 |    78 | no       | no       | no (added separately)      | Zachary 1977       |
| dolphins       |    62 |   159 | no       | no       | no (see below)             | Lusseau 2003       |
| lesmis         |    77 |   254 | yes      | no       | n/a (unsupervised)         | Knuth 1993         |
| football       |   115 |   613 | no       | no       | yes (`value` = conference) | Girvan-Newman 2002 |
| polbooks       |   105 |   441 | no       | no       | yes (`value` = leaning)    | V. Krebs (unpub.)  |
| celegansneural |   297 |  2359 | yes      | yes      | n/a (unsupervised)         | White et al. 1986  |

The three graphs named in the plan are karate, dolphins, and lesmis. football
and polbooks are included as extra labeled benchmarks: their GML nodes carry a
`value` attribute that is the accepted ground-truth partition (12 football
conferences; liberal/neutral/conservative for polbooks), so they give ready
NMI/ARI targets without any extra file. celegansneural is the directed corpus
graph for the directed value-oracle: a weighted, directed neural network with no
ground-truth partition, used as a directed-modularity target at scale (directed
verification phase 5, epic `mes-crz3`).

## Ground truth

- **karate:** Newman's GML has no community labels. The canonical two-faction
  split (Mr. Hi's group vs the Officer's group) is provided in
  [karate/karate-ground-truth.csv](karate/karate-ground-truth.csv), 1-based node
  ids matching `karate.gml`. It matches networkx `karate_club_graph()`'s `club`
  attribute. This is the benchmark: modularity maximization recovers a few fine
  communities that merge into these two real factions.
- **dolphins:** Newman's GML carries only dolphin names (`label`), not the
  community split. The accepted two-group partition is from Lusseau & Newman,
  "Identifying the role that animals play in their social networks" (2004), and
  is bundled in igraph and several benchmark repos. It is intentionally NOT
  hand-transcribed here to avoid an unverified vector; pull it from an
  authoritative labeled source (igraph's `dolphins` or the KONECT copy) when the
  dolphins accuracy fixture is wired up.
- **lesmis:** a weighted co-occurrence network with no canonical ground-truth
  partition; used as a modularity/CPM target, not for NMI/ARI.
- **football / polbooks:** ground truth is the in-file `value` attribute.

## Betweenness references

`karate`, `dolphins`, `lesmis` and `celegansneural` each carry a
`betweenness.csv`: one `key,betweenness` row per node, the key being the GML
node `id`, the value networkx's `betweenness_centrality(G, normalized=True)`
over unweighted shortest paths at full float precision. They are the external
reference for meso's `Betweenness`, which matches them to within a few units in
the last place.

- Generator: `betweenness.py`, a self-contained `uv` script pinning networkx
  3.4.2. It reads each GML by node id, collapses duplicate edges (celegansneural
  lists some arcs twice) and drops self-loops, matching how meso's builder folds
  them. Reproduce, from the repository root, with
  `uv run datasets/betweenness.py`.
- Regenerate only when a corpus GML or the pinned networkx version changes; the
  Go test fails if a CSV's key set drifts from its graph.

## LFR synthetic benchmarks

LFR benchmark graphs (Lancichinetti-Fortunato-Radicchi): synthetic graphs with
planted communities and a tunable mixing parameter, across a difficulty sweep.
These score recovery accuracy (NMI/AMI/ARI) against planted ground truth. The
design of record is the accuracy-sweep section of
[docs/specs/001-initial-implementation/meso-oracle.md](../docs/specs/001-initial-implementation/meso-oracle.md).

A fixed, versioned suite is committed under `lfr/`, generated once and out of band
(never in CI); the scorer loads these committed graphs rather than generating at
test time.

- Papers: see ../research/bibliography.md sections 1 (LFR 2008) and 4
  (comparative analysis 2009).
- Generator: `lfr/generate.py`, a self-contained `uv` script wrapping
  `networkx.generators.community.LFR_benchmark_graph` (pinned by version), with
  deterministic per-graph seeds so re-running reproduces the suite. Reproduce with
  `uv run datasets/lfr/generate.py`.
- Layout: `lfr/<regime>/mu<NNN>-r<k>.txt` (1-based edge list) and its
  `...-ground-truth.csv` planted partition, per graph. `lfr/manifest.json` records
  every graph's parameters, seed, edge and community counts, realized average
  degree, and realized mixing.
- Grid: `N = 1000`, `τ1 = 2`, `τ2 = 1.5`, `min_degree = 10`, `max_degree = 50`,
  community regimes S (10 to 50) and B (20 to 100), nominal `μ ∈ {0.1 … 0.7}`, 5
  realizations per point (70 graphs). networkx's `μ` runs low, so nominal
  `0.1 … 0.7` realizes about `0.15 … 0.90` (recorded); thresholds key off the
  recorded realized `μ`. Self-loops networkx emits are stripped.

## Licensing / distribution note

These are third-party research datasets, redistributed for local testing only. Cite the source papers (see the bibliography) in
any fixture that uses them. Do not vendor them into the published module without
confirming each dataset's terms.
