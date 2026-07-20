---
id: mes-c7sx
status: closed
deps: [mes-ntmq, mes-5ko0]
links: []
created: 2026-07-17T02:40:47Z
type: task
priority: 2
assignee: Andre Silva
parent: mes-dvte
tags: [verification, lean, phase-f]
---
# F3. The mesoOracle executable

`lean_exe mesoOracle` over `Meso/OracleIO.lean` reads committed JSON inputs and writes exact golden vectors (`num`/`den` strings plus `approx`) via the verified `WeightedGraphQ.ofRaw`; `make oracle-lean` regenerates every committed vector.
