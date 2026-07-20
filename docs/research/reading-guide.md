# meso reading guide

A phased path through the [bibliography](bibliography.md) for implementing
`meso`. The order is deliberate and does not match the bibliography's grouping:
read the objective before the algorithm, and read validation alongside building
the test harness. Depth targets are explicit. Not every paper deserves the same
attention.

Companion file: [notation-cheatsheet.md](notation-cheatsheet.md). Fill it in as
you read Phases 1 and 2; it is the single most useful artifact for the
implementation because every paper uses its own notation.

## How to read these papers

- Read the physics papers for results, not derivations. When a paper spends
  pages on a spin-glass or Hamiltonian derivation, extract the result and the
  modified formula; do not reproduce the physics.
- Keep the notation cheat-sheet current. Each objective written in one
  consistent notation saves constant re-derivation later.
- Treat the reference implementations as primary sources. For an implementation
  project, when a paper is ambiguous, the CWTS `networkanalysis` Java code is
  ground truth. This matters most for `meso`'s determinism goal, where
  tie-breaking, iteration order, and RNG seeding are decisive.

## Prerequisites (patch before Phase 1 if shaky)

Non-negotiable: graph theory basics (degree, adjacency matrix, weighted and
directed graphs, connectivity, subgraphs) and the null-model idea (modularity is
observed edges minus expected edges under a random baseline). If either is
shaky, read Barabási, _Network Science_, chapters 2 and 9 first (free online).

Helpful but skippable: statistical mechanics (Potts/Ising, Hamiltonians). This
is vocabulary, not machinery you must reproduce. Read "minimize the Hamiltonian"
as "maximize the quality function." Information theory (entropy, mutual
information) is needed for Phase 3 metrics; Cover and Thomas is the reference.

## Phase 0: orient (half a day)

| Paper                              | Depth                | Goal                                                                            |
| ---------------------------------- | -------------------- | ------------------------------------------------------------------------------- |
| Fortunato, Hric (2016), user guide | Skim first ~30 pages | Build vocabulary and a map of the field; it name-drops nearly every other paper |
| Fortunato (2010), review           | Reference only       | Dip into specific sections as needed; do not read cover to cover                |

## Phase 1: the objective (the "what")

The quality function _is_ the definition of a community. Read in this order.

| #   | Paper                                          | Depth                    | Focus                                                                                         |
| --- | ---------------------------------------------- | ------------------------ | --------------------------------------------------------------------------------------------- |
| 1   | Newman, Girvan (2004), modularity              | Careful; derive `Q` once | The founding definition of `Q` and the null model                                             |
| 2   | Fortunato, Barthélemy (2007), resolution limit | Careful; short           | The flaw that motivates everything after: `Q` cannot see communities below a graph-wide scale |
| 3   | Reichardt, Bornholdt (2006), stat-mech         | Result, skim derivation  | The resolution parameter gamma as a scale knob                                                |
| 4   | Traag, Van Dooren, Nesterov (2011), CPM        | Careful                  | Why CPM is resolution-limit-free by construction                                              |
| 5   | Newman (2006), modularity structure            | Skim; optional           | Spectral/matrix view of `Q`; read only if it aids implementation                              |

After Phase 1 you should be able to write `Q`, `Q_gamma`, and CPM in one
consistent notation (the cheat-sheet).

## Phase 2: the algorithm (the "how")

The core of the project. Read the reference code alongside the papers.

| #   | Paper                                  | Depth                                     | Focus                                                                                                                             |
| --- | -------------------------------------- | ----------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| 1   | Blondel et al. (2008), Louvain         | Understand cold                           | The two-phase local-move + aggregate loop and the greedy modularity-gain move                                                     |
| 2   | Traag, Waltman, van Eck (2019), Leiden | Implementation depth; read the supplement | The three-phase scheme, the refinement phase and _why_ it exists (disconnected communities in Louvain), and the formal guarantees |
| 3   | Leicht, Newman (2008), directed        | Focused; defer                            | Directed modularity; read only when adding directed-graph support                                                                 |

For Phase 2 item 2, read the Nature paper and the arXiv version together, plus
the supplementary material, which contains the exact move and refinement
procedures. Pay attention to tie-breaking and RNG use: these determine whether
`meso`'s results are reproducible.

## Phase 3: validation (the "is it right")

Read alongside building the test harness, not before. Item 2 comes before
implementing NMI.

| #   | Paper                                        | Depth                       | Focus                                                                                              |
| --- | -------------------------------------------- | --------------------------- | -------------------------------------------------------------------------------------------------- |
| 1   | Danon et al. (2005), NMI                     | Careful                     | NMI as the standard partition-comparison score                                                     |
| 2   | Vinh et al. (2010), clustering comparison    | Before implementing metrics | Which NMI normalization and which adjusted-for-chance variant to pick; avoids a known footgun      |
| 3   | Lancichinetti et al. (2008), LFR benchmark   | Careful                     | The planted-community generator and the mixing parameter                                           |
| 4   | Lancichinetti, Fortunato (2009), comparative | Method                      | How to run mixing-parameter sweeps and read accuracy curves                                        |
| -   | Hubert, Arabie (1985), ARI                   | Skip the paper              | Paywalled and unnecessary; the ARI formula is standard, implement from a reliable secondary source |

## Phase 4: provenance (read once)

Data provenance, not theory. Skim abstracts so the datasets can be described
honestly; the graphs and ground-truth labels are what matter and are bundled
everywhere.

- Zachary (1977), karate club
- Lusseau et al. (2003), dolphins
- Knuth (1993), Stanford GraphBase (Les Misérables)

## Deferred: formal verification

The formal-verification section of the bibliography is a separate skill tree
(Lean, separation logic, TLA+) and is orthogonal to the community-detection
theory. Do not let it slow Phases 1 through 3. Return to it only when pursuing
the verification tiers.

## Reference implementations (study in Phase 2)

- networkanalysis (Java, CWTS): the authors' canonical implementation and the
  ground truth for ambiguous points.
- leidenalg (Python, V. Traag).
- igraph (C).
- gonum (Go): target of the optional adapter.
