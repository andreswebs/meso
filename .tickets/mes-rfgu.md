---
id: mes-rfgu
status: open
deps: []
links: []
created: 2026-10-07T16:34:15Z
type: chore
priority: 1
assignee: Andre Silva
parent: mes-0jzi
tags: [toolchain, step-0]
---
# Go 1.27 toolchain in both modules, CI golangci-lint bump, baseline regen

Step 0 of `docs/specs/002-structural-measures/plan.md`, parent mes-0jzi. Move both modules to Go 1.27 before any feature code so every later ticket builds, lints and benchmarks on one toolchain.

## Evidence (verified 2026-10-07)

- `go.mod` and `gonum/go.mod` both declare `go 1.26.5`. The installed toolchain is `go1.27.1 darwin/arm64`.
- CI reads `go-version-file: go.mod` in `.github/workflows/ci.yml`, `govulncheck.yml` and `release.yml`, so the Go version follows `go.mod` with no workflow edit.
- golangci-lint is pinned separately: `version: v2.12.2` under `golangci/golangci-lint-action` in `ci.yml` and `release.yml`. The local binary is `golangci-lint 2.14.0 built with go1.27.1`. A linter built with an older Go typically refuses to load a module whose `go` directive is newer, so the CI pin almost certainly needs to move.
- `make bench-baseline` regenerates `benchmarks/baseline.txt`; `make bench-check` gates against it.

## Steps

1. Re-check the evidence above; stop and report if anything changed since 2026-10-07.
2. Set the `go` directive to `1.27.1` (the installed patch) in both `go.mod` files.
3. Run `go fix ./...` in each module and review the modernizer rewrites; keep only behaviour-preserving ones.
4. `make tidy`.
5. Bump `version:` for golangci-lint in `ci.yml` and `release.yml` to a release built with Go 1.27 (2.14.0 or newer). Keep the action itself pinned by commit SHA as the repo requires; bump the action SHA only if the new lint version needs it.
6. Fix any new lint findings properly (no `_ =` silencing).
7. `make bench-baseline` and commit-ready the new `benchmarks/baseline.txt`.

## Warnings

- `gonum/go.mod` requires `github.com/andreswebs/meso v0.1.0`, whose own `go` directive stays 1.26.5. That is fine; do not add a `replace`.
- `go fix` output must not change any golden, oracle or determinism result; if a test changes, the rewrite was not behaviour-preserving.

## Acceptance Criteria

1) Both `go.mod` files declare Go 1.27.x. 2) `make validate` and `make test-race` green in both modules. 3) `make bench-check` green against the regenerated baseline. 4) `make vulncheck` green. 5) CI workflows pin a golangci-lint built with Go 1.27. The close note records the exact lint version chosen and any `go fix` rewrites kept.
