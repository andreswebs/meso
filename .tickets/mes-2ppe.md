---
id: mes-2ppe
status: closed
deps: [mes-ojdd, mes-8x09, mes-6mfk, mes-i21e]
links: []
created: 2026-10-07T16:34:16Z
type: task
priority: 2
assignee: Andre Silva
parent: mes-0jzi
tags: [docs, step-8, implementation]
---
# Design of record, package docs, and correspondence divergence entry for structural measures

Step 8 of `docs/specs/002-structural-measures/plan.md`, parent mes-0jzi. Make the design of record and the package docs describe the whole v0.2.0 API, and record the verification gap honestly.

## Design

- `docs/meso-design.md`: new "Structural measures" section carrying the plan's "Pinned semantics" (builder zero-weight rule, accessors, cohesion, subgraph, betweenness normalization and determinism argument). Update section 2 (scope now mesoscale structure, not only community detection), section 3 (accessor, `Subgraph`, `Betweenness` API; the section 3 sketch also still shows a `part.Levels()` call, check whether it exists), section 6 (empirical tier for these measures), and section 10 (a v0.2.0 milestone).
- `doc.go`: the opening sentence says "community-detection library"; widen the scope statement and add a `Betweenness` and `Subgraph` example. Mirror in `README.md`.
- Runnable `Example` tests for `Betweenness`, `Subgraph` and `Cohesion` (the repo has none today).
- `verification/lean/CORRESPONDENCE.md` section 3, "Divergence register": an entry stating betweenness, cohesion, the induced subgraph and the accessors are outside the Lean model and validated empirically (brute force, closed forms, networkx references), so the gap cannot pass as coverage. Also note the builder zero-weight rule against the model's `weight` function if a row needs it.

## Warnings

- Documentation must not cite local or gitignored paths, only repository paths.
- No em-dashes in prose (house style).

## Acceptance Criteria

1) Every new exported symbol has godoc and appears in the design doc. 2) `Example` tests compile and pass. 3) The divergence-register entry exists. 4) `markdownlint-cli2` clean on every touched markdown file (project config if present, else the maintainer's global one). `make validate` green in both modules (fmt-check, vet, lint, test).

## Notes

**2026-10-08T02:20:11Z**

Done.
- docs/meso-design.md: new subsection 4.6 "Structural measures" (builder zero-weight rule, accessor semantics, Cohesion, Subgraph, Betweenness normalization and determinism argument, the allocation-free accumulation, and what is deliberately not provided). It is a subsection, not a new top-level section, so the many "section N" citations across code, tickets and docs stay valid. Also updated: section 1 non-goals (no longer contradicts the measures), section 2 scope bullet, section 3 API bullet and sketch, section 6.2 empirical-tier bullet, section 10 milestone 8, section 11 Brandes (2001) reference.
- The section 3 sketch called `part.Levels()`, which has no public counterpart (only the internal `louvainLevels`). The sketch now shows real API and states the hierarchy is in scope but not yet public. Raised for discussion rather than decided.
- doc.go: scope statement widened; new "# Structural measures" section at the end of the package doc.
- README.md: description, a usage snippet, and a goal bullet.
- example_test.go (package meso_test, the repo's first examples): `ExampleBetweenness`, `ExampleResult_Cohesion`, `ExampleSubgraph` on two bridged triangles; outputs hand-checked (bridge nodes 0.6 = 6 cross pairs / 10).
- verification/lean/CORRESPONDENCE.md divergence register: entry stating the structural measures are outside the model and validated empirically; the zero-weight builder rule matches the model's positive-weight edge relation (Connectivity.lean), so no modelled quantity changes.
markdownlint clean on all touched markdown; `make validate` green.
