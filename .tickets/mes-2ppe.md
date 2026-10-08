---
id: mes-2ppe
status: open
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
