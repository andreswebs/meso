---
id: mes-j1zl
status: closed
deps: [mes-c7sx, mes-fzue, mes-mvoo, mes-qr8n, mes-kvci]
links: []
created: 2026-07-17T02:40:48Z
type: task
priority: 2
assignee: Andre Silva
parent: mes-dvte
tags: [verification, lean, phase-g]
---
# G4. Emit predicate vectors

`Meso/OracleIO.lean` emits a predicates object per case (`connected`, `gammaDense`, `gammaSeparated`, `subsetOptimal`), each the decide of a proved mirror; the F4 cross-check corroborates the connectivity flag against `igraph`. `subsetOptimal` is null above a node-count bound.
