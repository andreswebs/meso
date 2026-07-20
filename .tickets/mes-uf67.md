---
id: mes-uf67
status: closed
deps: []
links: [mes-njtw]
created: 2026-07-14T03:33:33Z
type: chore
priority: 3
assignee: Andre Silva
tags: [ci, infra, supply-chain, step-0]
---
# CI skeleton and supply-chain hardening: GitHub Actions

Originally a deferred sub-item of implementation step 0
(`docs/specs/001-initial-implementation/plan.md`): stand up
`.github/workflows/ci.yml` running the quality gate. Scope expanded
(2026-07-20) to implement the full supply-chain hardening from the
release-engineering milestone (`docs/meso-design.md` section 9 and
milestone 6): Sigstore keyless signing, SLSA build provenance, and SBOM on
releases, plus a vulnerability-scanning lane.

## Current state (updated 2026-07-20, all phases done)

Six workflows live under `.github/workflows/`: `ci.yml` (quality gate),
`release.yml` (hardened per phase 1), `govulncheck.yml`, `lean-ci.yml`,
`lean-update.yml`, and the pre-existing `gobra.yml`. The composite
release action lives at `.github/actions/gh-release`. `make vulncheck`
added (govulncheck pinned in the Makefile, both modules scan clean).
Docs updated. The deferred Scorecard lane is tracked as mes-njtw
(linked).

## Decisions (settled 2026-07-20)

1. **Scope**: all skeleton workflows live (done), `release.yml` hardened,
   plus a new `govulncheck` lane. `gobra-nightly` is untouched.
2. **Benchmark gate**: `make bench-check` stays in PR CI. The baseline was
   designed for it (allocs/op deterministic, ns/op has cross-machine
   headroom). Demote to nightly only if runner noise flakes it, with data.
3. **Provenance**: both mechanisms, mirroring the release workflow of the
   feedwatch project: `cosign sign-blob --yes --bundle` (keyless, Sigstore
   bundle format, cosign v3+ required to verify) on the checksums file and
   the SBOM, plus `actions/attest-build-provenance` with
   `subject-checksums` for SLSA provenance verifiable via
   `gh attestation verify`.
4. **Artifacts**: checksums + SBOM only. meso is a library: consumers get
   source through the module proxy with `sum.golang.org` integrity, so no
   source artifact is built. Each release attaches the module's SBOM
   (anchore/sbom-action, syft, `spdx-json`), `SHA256SUMS.txt` over it, the
   two Sigstore bundles, and the provenance attestation.
5. **gonum releases**: per-module tags, one workflow. `release.yml`
   triggers on `v*.*.*` and `gonum/v*.*.*`; each run releases exactly the
   module its tag names (core tag -> core SBOM, gonum tag -> gonum SBOM).
6. **Extra lanes**: `govulncheck` in scope, as its own workflow (weekly
   cron + `workflow_dispatch` + `pull_request`). OpenSSF Scorecard is
   planned but deferred to a follow-up. CodeQL and harden-runner are out
   of scope.
7. **lean-update.yml**: `workflow_dispatch` only; the daily cron stays
   commented out. Mathlib bumps run deliberately.

## Implementation plan

### Phase 1: hardened release workflow (done)

Rewrite `release.yml`:

- Triggers: tags `v*.*.*` and `gonum/v*.*.*`. Derive the module directory
  and name from the tag prefix.
- Permissions: `contents: write`, `id-token: write`,
  `attestations: write`.
- Steps: checkout (full depth) -> setup-go -> golangci-lint ->
  `make validate` -> generate the released module's SBOM
  (anchore/sbom-action, `spdx-json`, `SYFT_SOURCE_NAME`/`_VERSION` set
  from the tag) -> write `SHA256SUMS.txt` over the SBOM -> install cosign
  (sigstore/cosign-installer v4, installs cosign v3.x) ->
  `cosign sign-blob --yes --bundle` on `SHA256SUMS.txt` and the SBOM ->
  `actions/attest-build-provenance` with
  `subject-checksums: SHA256SUMS.txt` -> publish the GitHub release via a
  local composite action (`.github/actions/gh-release`) that uploads the
  artifacts and embeds verification instructions (both
  `gh attestation verify` and `cosign verify-blob`) in the release notes,
  mirroring the feedwatch composite action.
- All new third-party actions pinned by commit SHA (cosign-installer,
  sbom-action, attest-build-provenance).

### Phase 2: govulncheck lane (done)

New `govulncheck.yml`: weekly cron (off-peak minute, same convention as
`gobra.yml`), `workflow_dispatch`, and `pull_request`. Runs a new
`make vulncheck` target that fans `govulncheck ./...` across both modules,
consistent with the repo convention that CI runs make targets. The
govulncheck version is pinned in the Makefile (`GOVULNCHECK_VERSION`,
v1.6.0 at implementation time).

### Phase 3: docs and closure (done)

- Updated the "Releases & supply-chain security" section in `AGENTS.md`
  (`CLAUDE.md` is a symlink): describes what a release produces and how to
  verify it, plus the `make vulncheck` row in the command table.
- Deferred Scorecard lane filed as mes-njtw and linked here.
- Validation: `actionlint` clean, `shellcheck` clean on the composite
  action's script, `markdownlint-cli2` clean on touched markdown,
  `make validate` green, `make vulncheck` green on both modules.

## Manual items (user, not automatable from this repo)

- Branch protection / rulesets for `main` (required checks: the CI job).
- First tag push to prove the release path end to end, then verify:
  `gh attestation verify` against the checksums and
  `cosign verify-blob --bundle` against both bundles.

## Acceptance Criteria

- `ci.yml`, `release.yml`, `lean-ci.yml`, `lean-update.yml`,
  `govulncheck.yml`, and `gobra.yml` are live under `.github/workflows/`.
- CI triggers on `push` to `main` and `pull_request`; runs `make validate`,
  `make test-race`, and `make bench-check` green on a clean checkout.
- Release runs on `v*.*.*` and `gonum/v*.*.*` tags and publishes: SBOM,
  `SHA256SUMS.txt`, two Sigstore bundles, provenance attestation, release
  notes with verification instructions.
- `make vulncheck` exists and the govulncheck workflow runs it.
- Every `uses:` pinned to a commit SHA; golangci-lint version matches
  `.golangci.yml` expectations; `actionlint` clean.
- No algorithm code involved.

## Notes

**2026-07-20T15:09:59Z**

Closing: all three phases implemented and validated. LIVE WORKFLOWS: ci (validate + test-race + bench-check on push/PR), release (hardened), govulncheck (weekly cron + PR + dispatch, via make vulncheck, govulncheck pinned v1.6.0), lean-ci, lean-update (manual), gobra-nightly (pre-existing). RELEASE PATH: per-module tags (v*.*.* core, gonum/v*.*.* adapter); each release publishes SBOM (anchore/sbom-action, spdx-json) + SHA256SUMS.txt + two cosign keyless Sigstore bundles + GitHub SLSA provenance attestation (subject-checksums), published by the local composite action .github/actions/gh-release with verification commands in the notes (mirrors feedwatch). No source artifact by decision: the module proxy + sum.golang.org own source integrity. GATES: actionlint, shellcheck, markdownlint, make validate, make vulncheck all green. DEFERRED: OpenSSF Scorecard lane -> mes-njtw (linked). USER MANUAL ITEMS: branch-protection rulesets on main; first tag push to prove the release path end to end (gh attestation verify + cosign verify-blob --bundle).
