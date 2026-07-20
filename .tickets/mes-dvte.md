---
id: mes-dvte
status: closed
deps: []
links: []
created: 2026-07-17T02:40:47Z
type: epic
priority: 1
assignee: Andre Silva
tags: [verification, lean]
---
# Formal verification (Lean 4): the Leiden/Louvain guarantees and value-oracle

Model-level formal verification of the Leiden/Louvain guarantees plus the computable value-oracle, per section 7 of `docs/meso-design.md`. Retroactive record of the Lean tier (formerly tracked in `verification/lean/TODO.md`, now this epic and its child tickets, with status in `verification/lean/README.md` and the theorem-to-test bridge in `verification/lean/CORRESPONDENCE.md`): design invariants (A), CPM and convergence machinery (B), the three paper theorems (C), the concurrency track (D), hardening gaps (E), the computable value-oracle (F), and the predicate vectors (G). All phases complete, including D2 (Gobra data-race freedom over the shipped `parallel.go`; scope, trusted assumptions, and runbook in `verification/gobra/README.md`).

## Notes

**2026-07-20T14:05:23Z**

Closing the epic: all 24 children closed. Phases A-G proved in Lean (design invariants, CPM/convergence, the three paper theorems, synchronous-round confluence, hardening, value-oracle, predicate vectors; no sorryAx anywhere), and D2 landed last: machine-checked data-race freedom, memory safety, and crash safety of the parallel fork-join in parallel.go via Gobra (mes-hret, closed 2026-07-20; 0 errors through the digest-pinned interim image, nightly CI lane implemented). Standing items that outlive the epic, tracked in verification/gobra/README.md: the upstream Gobra float fix (R4 exit condition: re-pin the official image and drop the fork) and the R6 nightly-to-blocking promotion after 10 consecutive greens.
