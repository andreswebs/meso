---
id: mes-5wqp
status: closed
deps: [mes-z0pd]
links: [mes-nqky, mes-vy3a, mes-wmzq, mes-xb3w]
created: 2026-07-14T03:40:30Z
type: task
priority: 2
assignee: Andre Silva
parent: mes-t76u
tags: [core, golden, corpus, step-5]
---

# Golden corpus tests: karate, dolphins, Les Miserables

Commit golden partitions and quality for the canonical small graphs at a fixed seed, and pin them in-repo. The quality envelope is checked against frozen reference vectors (assumed already produced by the separate oracle harness). Design of record: `docs/meso-design.md` sections 6.1-6.2; step 5 of `docs/specs/001-initial-implementation/plan.md`.

## Design

Load `datasets/karate`, `datasets/dolphins`, `datasets/lesmis` (already in-repo as `.gml`/`.txt`). Run Louvain at a fixed seed; commit the exact Go partition and quality as golden files. Cross-check quality/community-count/connectivity against the frozen reference vectors (`docs/meso-design.md` section 6.2) - the harness that produces those vectors is OUT OF SCOPE here and assumed ready; this ticket only consumes committed vectors. Not exact-partition cross-language (different PRNGs). Go conventions: `testdata/` golden files, `-update` flag idiom for regeneration.

## Acceptance Criteria

TDD order. 1) Karate at fixed seed produces a committed golden partition and quality (exact Go bytes). 2) Dolphins and Les Miserables likewise. 3) The achieved quality falls within the frozen reference envelope (consumes committed vectors). 4) Re-running is byte-identical (leans on the determinism ticket). `make validate` green. NOTE: building/refreshing the frozen vectors is a separate oracle-harness track, not this ticket.

## Notes

**2026-07-17T02:29:41Z**

Oracle prerequisite delivered; framing updated (2026-07-16). The "frozen reference
vectors ... produced by a separate oracle harness (out of scope, assumed ready)" this
ticket consumes now exist and are committed: `verification/oracle/golden/{karate,dolphins,
lesmis}.json`, exact rational quality values emitted by the Lean value-oracle (`mesoOracle`).
The retired cross-language differential ENVELOPE (a range between two implementations) is
superseded: the check is now Go `float64` within a float-rounding tolerance of the exact
rational value, not membership in an envelope. The igraph/leidenalg spec-blessing cross-check
passes (`verification/reference`, ALL PASS). Only the Go core (Louvain + determinism, this
ticket's dep chain) remains blocking; the oracle side is done.

**2026-07-17T22:44:49Z**

Done. TestGoldenCorpus (golden_test.go) pins meso's deterministic Louvain output on karate/dolphins/lesmis as committed JSON goldens under testdata/golden/, via the public API only (NewBuilder, Louvain, Result.Communities/Quality) with the standard -update idiom (go test -run TestGoldenCorpus -update).

Achieved values match literature: karate Q=0.4188 (4 comms; optimum ~0.4198), dolphins Q=0.5185 (5), lesmis Q=0.5654 (6, weighted). AC#3's reference envelope is a loose published-literature band + community-count sanity check (guards against a grossly broken optimiser only); AC#4 determinism is checked by an in-process rerun and a byte-identical -update regen.

Golden partitions are canonicalised: nodes keyed by GML id in ascending numeric order, community labels renumbered by smallest member, modularity stored as shortest round-trippable decimal (FormatFloat 'g' -1 64), so genuine drift diffs but relabelling does not. GML loader handles karate's 1-based ids, lesmis's 0-based ids, and lesmis's 'value' edge weights.

NOTE for next: the exact-rational value-oracle check on meso's OWN converged partition is NOT here (committed oracle vectors only score anchor partitions: all-in-one, singletons, ground-truth split). That is the deferred mes-xb3w, which should add meso's Leiden/Louvain converged partition to verification/oracle/inputs/*.json and regenerate with make oracle-lean. make validate green.
