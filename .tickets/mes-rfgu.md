---
id: mes-rfgu
status: closed
deps: []
links: []
created: 2026-10-07T16:34:15Z
type: chore
priority: 1
assignee: Andre Silva
parent: mes-0jzi
tags: [toolchain, step-0, implementation]
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

## Notes

**2026-10-08T01:38:04Z**

Done. Both `go.mod` files now declare `go 1.27.1`. `go fix ./...` made no rewrites in either module; `make tidy` changed nothing. CI and release workflows pin golangci-lint `v2.14.0` (latest, built with Go 1.27.1); the action SHA was already the latest v9.3.0 and is unchanged. Lint: 0 issues in both modules.

Gates: `make validate`, `make test-race`, `make vulncheck` green in both modules; `make bench-check` green against both the old and the regenerated baseline.

Baseline: the committed `benchmarks/baseline.txt` was recorded on linux/arm64 with GOMAXPROCS 4 (not documented anywhere; Docker on the maintainer's Mac reports exactly 4 CPUs linux/aarch64). Regenerated it the same way: `docker run --rm --cpus=4 -v "$PWD:/work" -w /work golang:1.27.1-bookworm go test -run='^$' -bench=. -benchmem -benchtime=10x -count=6 . > benchmarks/baseline.txt`. Same 10 benchmarks x 6 samples as before. Record this procedure in the docs if the baseline machine matters (mes-z1rf regenerates again and should use the same command).

Toolchain effect, measured with an interleaved A/B in that container (HEAD on golang:1.26.5 vs this tree on golang:1.27.1): no significant ns/op change on any benchmark (geomean -1.5%). Against the old committed baseline the new one reads ~12% slower, which the A/B shows is machine drift since the original recording, not Go 1.27. allocs/op identical except planted-300: Leiden 6728 -> 6719, Louvain 4734 -> 4707. Reproduced under go1.26.5 vs go1.27.1 on darwin, so it is the toolchain; partitions and Quality() bits are identical under both toolchains on planted-300 and planted-800, so it is a pure compiler/runtime gain. B/op +16 bytes on most benchmarks, constant.
