# meso research bibliography

Theoretical foundations for implementing a deterministic Go
community-detection library (Leiden and Louvain). Grouped by role in the project.
Each entry notes why it matters for `meso` and where to find it.

## 1. Core algorithms (must read)

These four define the algorithms `meso` implements and the quality functions it
optimizes.

### Traag, Waltman, van Eck (2019) - From Louvain to Leiden

- **Citation:** Traag, V. A., Waltman, L., & van Eck, N. J. (2019). From Louvain
  to Leiden: guaranteeing well-connected communities. _Scientific Reports_, 9, 5233.
- **DOI:** 10.1038/s41598-019-41695-z
- **Access:** Open access (CC BY). Nature Scientific Reports.
- **Why:** The primary source. Defines the three-phase Leiden algorithm (fast
  local move, refinement with well-connectedness gate, aggregation seeded from
  the non-refined partition) and states the formal guarantees (gamma-separation,
  gamma-connectivity, subset-optimality at convergence) that section 6/7 of the
  plan verify. Read the supplementary material too: it contains the exact move
  and refinement procedures.

### Blondel, Guillaume, Lambiotte, Lefebvre (2008) - Louvain

- **Citation:** Blondel, V. D., Guillaume, J.-L., Lambiotte, R., & Lefebvre, E.
  (2008). Fast unfolding of communities in large networks. _Journal of
  Statistical Mechanics: Theory and Experiment_, 2008(10), P10008.
- **DOI:** 10.1088/1742-5468/2008/10/P10008
- **arXiv:** 0803.0476
- **Access:** Open access via arXiv.
- **Why:** The Louvain baseline `meso` ships and uses as an internal differential
  oracle. Defines local moving + aggregation and the greedy modularity-gain move.

### Leicht, Newman (2008) - Community structure in directed networks

- **Citation:** Leicht, E. A., & Newman, M. E. J. (2008). Community structure in
  directed networks. _Physical Review Letters_, 100(11), 118703.
- **DOI:** 10.1103/PhysRevLett.100.118703
- **arXiv:** 0709.4500
- **Access:** Open access via arXiv.
- **Why:** The directed-modularity formulation `meso` uses for directed graphs
  (plan section 4.3). PRL itself is paywalled; use the arXiv version.

### Lancichinetti, Fortunato, Radicchi (2008) - LFR benchmark

- **Citation:** Lancichinetti, A., Fortunato, S., & Radicchi, F. (2008).
  Benchmark graphs for testing community detection algorithms. _Physical Review
  E_, 78(4), 046110.
- **DOI:** 10.1103/PhysRevE.78.046110
- **arXiv:** 0805.4770
- **Access:** Open access via arXiv.
- **Why:** The synthetic-benchmark generator with planted communities and a
  tunable mixing parameter, used for NMI/ARI accuracy scoring (plan section 6.1,
  6.3). The generator source is a separate download (see datasets bibliography).

## 2. Quality functions and resolution (must read)

### Newman, Girvan (2004) - Modularity

- **Citation:** Newman, M. E. J., & Girvan, M. (2004). Finding and evaluating
  community structure in networks. _Physical Review E_, 69(2), 026113.
- **DOI:** 10.1103/PhysRevE.69.026113
- **arXiv:** cond-mat/0308217
- **Access:** Open access via arXiv.
- **Why:** Original definition of the modularity quality function `Q` that
  Louvain/Leiden maximize by default.

### Newman (2006) - Modularity and community structure

- **Citation:** Newman, M. E. J. (2006). Modularity and community structure in
  networks. _PNAS_, 103(23), 8577-8582.
- **DOI:** 10.1073/pnas.0601602103
- **arXiv:** physics/0602124
- **Access:** Open access (PNAS + arXiv).
- **Why:** The spectral/matrix view of modularity; clarifies the null model and
  the modularity matrix, useful background for the quality-function core.

### Reichardt, Bornholdt (2006) - Statistical mechanics / resolution parameter

- **Citation:** Reichardt, J., & Bornholdt, S. (2006). Statistical mechanics of
  community detection. _Physical Review E_, 74(1), 016110.
- **DOI:** 10.1103/PhysRevE.74.016110
- **arXiv:** cond-mat/0603718
- **Access:** Open access via arXiv.
- **Why:** Introduces the resolution parameter (gamma) as a spin-glass
  Hamiltonian. This is the RB model and the origin of the resolution term
  `meso`'s modularity quality function exposes.

### Traag, Van Dooren, Nesterov (2011) - CPM (Constant Potts Model)

- **Citation:** Traag, V. A., Van Dooren, P., & Nesterov, Y. (2011). Narrow scope
  for resolution-limit-free community detection. _Physical Review E_, 84(1), 016114.
- **DOI:** 10.1103/PhysRevE.84.016114
- **arXiv:** 1104.3083
- **Access:** Open access via arXiv.
- **Why:** Defines the Constant Potts Model, the resolution-limit-free quality
  function `meso` implements alongside modularity (plan section 2, 4.3).

### Fortunato, Barthélemy (2007) - Resolution limit

- **Citation:** Fortunato, S., & Barthélemy, M. (2007). Resolution limit in
  community detection. _PNAS_, 104(1), 36-41.
- **DOI:** 10.1073/pnas.0605965104
- **arXiv:** physics/0607100
- **Access:** Open access (PNAS + arXiv).
- **Why:** Explains the modularity resolution limit that motivates the resolution
  parameter and CPM. Informs why `meso` offers CPM as an alternative objective.

## 3. Evaluation metrics (must read for the test harness)

### Danon, Díaz-Guilera, Duch, Arenas (2005) - NMI for community comparison

- **Citation:** Danon, L., Díaz-Guilera, A., Duch, J., & Arenas, A. (2005).
  Comparing community structure identification. _Journal of Statistical
  Mechanics_, 2005(09), P09008.
- **DOI:** 10.1088/1742-5468/2005/09/P09008
- **arXiv:** cond-mat/0505245
- **Access:** Open access via arXiv.
- **Why:** Establishes normalized mutual information (NMI) as the standard score
  for comparing a recovered partition against ground truth (plan section 6).

### Hubert, Arabie (1985) - Adjusted Rand Index

- **Citation:** Hubert, L., & Arabie, P. (1985). Comparing partitions. _Journal
  of Classification_, 2(1), 193-218.
- **DOI:** 10.1007/BF01908075
- **Access:** Paywalled (Springer). ARI is standard; the definition is widely
  reproduced. No open PDF expected.
- **Why:** Origin of the adjusted Rand index (ARI), the second accuracy metric in
  the plan's NMI/ARI scoring.

### Vinh, Epps, Bailey (2010) - Information-theoretic measures (adjusted-for-chance)

- **Citation:** Vinh, N. X., Epps, J., & Bailey, J. (2010). Information theoretic
  measures for clusterings comparison: variants, properties, normalization and
  correction for chance. _Journal of Machine Learning Research_, 11, 2837-2854.
- **Access:** Open access (JMLR).
- **Why:** Rigorous treatment of NMI variants and adjusted-for-chance versions
  (AMI). The reference for implementing the comparison metrics correctly and
  choosing a normalization.

## 4. Surveys and context (recommended)

### Fortunato (2010) - Community detection in graphs

- **Citation:** Fortunato, S. (2010). Community detection in graphs. _Physics
  Reports_, 486(3-5), 75-174.
- **DOI:** 10.1016/j.physrep.2009.11.002
- **arXiv:** 0906.0612
- **Access:** Open access via arXiv.
- **Why:** The comprehensive review of the whole field. Good for placing
  modularity, resolution, and benchmarks in context.

### Fortunato, Hric (2016) - Community detection in networks: a user guide

- **Citation:** Fortunato, S., & Hric, D. (2016). Community detection in networks:
  A user guide. _Physics Reports_, 659, 1-44.
- **DOI:** 10.1016/j.physrep.2016.09.002
- **arXiv:** 1608.00163
- **Access:** Open access via arXiv.
- **Why:** Modern, practical companion to the 2010 review; covers benchmark usage
  and metric pitfalls relevant to the test harness.

### Lancichinetti, Fortunato (2009) - Comparative analysis of algorithms

- **Citation:** Lancichinetti, A., & Fortunato, S. (2009). Community detection
  algorithms: A comparative analysis. _Physical Review E_, 80(5), 056117.
- **DOI:** 10.1103/PhysRevE.80.056117
- **arXiv:** 0908.1062
- **Access:** Open access via arXiv.
- **Why:** Establishes the LFR-based benchmarking methodology `meso`'s accuracy
  tier follows.

## 5. Test corpus source papers (recommended)

Original papers behind the canonical small graphs in the test corpus (plan
section 6.1). Read for ground-truth partitions and provenance; the graph data
itself is under [datasets](../../datasets/).

### Zachary (1977) - Karate club

- **Citation:** Zachary, W. W. (1977). An information flow model for conflict and
  fission in small groups. _Journal of Anthropological Research_, 33(4), 452-473.
- **DOI:** 10.1086/jar.33.4.3629752
- **Access:** Paywalled (JSTOR / University of Chicago Press). No open PDF
  expected; the graph and the two-faction ground truth are what matter and are
  bundled in every network library.
- **Why:** The canonical ground-truth benchmark: the friendship graph predicts
  the real club split.

### Lusseau et al. (2003) - Dolphin social network

- **Citation:** Lusseau, D., Schneider, K., Boisseau, O. J., Haase, P., Slooten,
  E., & Dawson, S. M. (2003). The bottlenose dolphin community of Doubtful Sound
  features a large proportion of long-lasting associations. _Behavioral Ecology
  and Sociobiology_, 54(4), 396-405.
- **DOI:** 10.1007/s00265-003-0651-y
- **Access:** Paywalled (Springer). Provenance only; the graph is in the datasets.
- **Why:** Source of the 62-node dolphins graph.

### Knuth (1993) - The Stanford GraphBase (Les Misérables)

- **Citation:** Knuth, D. E. (1993). _The Stanford GraphBase: A Platform for
  Combinatorial Computing._ Addison-Wesley.
- **Access:** Book (not a paper). The Les Misérables character co-occurrence
  network derives from this. Data is in the datasets.
- **Why:** Source of the Les Misérables co-occurrence graph.

## 6. Reference implementations (study directly - GPL, portable)

Not papers, but the plan (section 5) treats these as primary sources to port
from. Listed here for completeness; clone separately, do not vendor PDFs.

- **networkanalysis** (Java, CWTS Leiden - the authors' canonical implementation):
  <https://github.com/CWTSLeiden/networkanalysis>
- **leidenalg** (Python, V. Traag): <https://github.com/vtraag/leidenalg>
- **igraph** (C with Python/R bindings): <https://github.com/igraph/igraph>
- **gonum** (Go; target of the optional adapter): <https://github.com/gonum/gonum>

## 7. Formal verification (optional depth - plan section 7)

### Graciolli, Amin (2026) - You Don't Know Jack About Formal Verification

- **Citation:** Graciolli, G., & Amin, N. (2026). You Don't Know Jack About
  Formal Verification. _ACM Queue_.
- **Access:** ACM Queue (typically free to read on queue.acm.org once published).
- **Why:** The framing the plan adopts for the optional formal-verification tiers
  (choose properties, express them, let a verifier establish them). Verify the
  exact citation once published.

Supporting tool references (documentation, not papers):

- **Lean / Mathlib:** <https://leanprover.github.io> - anchor prover for
  design-level invariant, confluence, and theorem proofs (tiers V1-V3).
- **Gobra:** <https://github.com/viperproject/gobra> - separation-logic verifier
  targeting Go, for data-race freedom of the parallel core (tier V2).
- **TLA+ / TLC:** <https://lamport.azurewebsites.net/tla/tla.html> - optional
  design bug-finder for the synchronous-round scheme.
