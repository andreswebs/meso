---
id: mes-e5yr
status: closed
deps: [mes-0isk]
links: []
created: 2026-07-14T03:40:30Z
type: task
priority: 3
assignee: Andre Silva
parent: mes-45a8
tags: [depth, metrics, step-11]
---

# Partition comparison metrics: `NMI`, `AMI`, `ARI`

The partition-similarity metrics used to score recovery against ground truth: normalized mutual information (`NMI`), adjusted mutual information (`AMI`), and adjusted Rand index (`ARI`). Independent of the algorithm - operates on two partitions. Design of record: `docs/meso-design.md` sections 6.1 and 6.3; step 11 of `docs/specs/001-initial-implementation/plan.md`.

## Design

Implement over the `Partition` type (CSR ticket). `NMI`/`AMI` per Vinh et al. with a deliberately chosen normalization and chance-adjustment; `ARI` from the contingency table. No Lean original (accuracy is empirical by design, `docs/meso-design.md` section 7). Pure functions on two label slices; canonical summation.

## Acceptance Criteria

TDD order. 1) Identical partitions score 1.0 for `NMI`, `AMI`, and `ARI`. 2) Independent/random partitions score near 0 for the adjusted variants (`AMI`, `ARI`). 3) `NMI`/`AMI` match hand-computed values on small contingency tables. 4) `ARI` matches hand-computed values on known edge cases. 5) Metrics are symmetric in their two arguments. `make validate` green.

## Notes

**2026-07-18T00:26:30Z**

Implemented NMI, AMI, ARI in metrics.go as pure functions over two Partitions (any int labels; compacted to dense indices). NMI/AMI per Vinh et al. (2010) with arithmetic-mean normalization (matches scikit-learn default); AMI's expected-MI uses the fixed-margins hypergeometric model with log-factorials to avoid overflow. ARI is Hubert-Arabie from the contingency table. Canonical summation (cells sorted by (row,col); rowSums/colSums iterated in ascending dense-label order) keeps results bit-stable. Length mismatch panics (gonum-style, programmer error). Degenerate both-single-community / n<=1 defined as 1.0; AMI denominator clamped to +/-eps like sklearn. Tests validated against an independent pure-Python (stdlib) reference since sklearn/numpy are absent: clean hand-derived anchors NMI(nested)=2/3, ARI(partial)=4/9, AMI/ARI(2x2 grid)=-0.5. 100% coverage on all metrics functions; make validate + make build green. Unblocks mes-sotf (LFR accuracy sweep).
