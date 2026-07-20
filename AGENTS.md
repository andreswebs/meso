# AI agent instructions

`meso` is a pure-Go, deterministic community-detection library (Leiden and
Louvain). It is a library, not a command: it ships no binary. See
[docs/meso-design.md](docs/meso-design.md) for the design of
record.

The repo is a Go multi-module workspace:

- `.` (`github.com/andreswebs/meso`) - the dependency-free core.
- `gonum/` (`github.com/andreswebs/meso/gonum`) - the optional gonum adapter, a
  separate module so the core never pulls gonum into consumers.

All commands run from the project root via `make` and fan out across every
module.

## Build & validation

| Command               | Purpose                                                      |
| --------------------- | ------------------------------------------------------------ |
| `make validate`       | Full quality gate: fmt-check, vet, lint, test (every module) |
| `make test`           | Run all tests (`go test ./...`) in every module              |
| `make test-race`      | Run tests with the race detector                             |
| `make build`          | Compile-check every module (produces no binary)              |
| `make vet`            | Run `go vet ./...`                                           |
| `make fmt`            | Format all Go source with `gofmt -w`                         |
| `make fmt-check`      | Fail if any files are not formatted                          |
| `make lint`           | Run `golangci-lint`                                          |
| `make tidy`           | Run `go mod tidy` in every module                            |
| `make cover`          | Run tests with coverage in every module                      |
| `make bench`          | Run the benchmark suite, writing results to `BENCH_OUT`      |
| `make bench-baseline` | Regenerate the committed baseline `benchmarks/baseline.txt`  |
| `make bench-check`    | Fail if a fresh run regresses against the committed baseline |
| `make bench-compare`  | Show a `benchstat` diff of the run vs the baseline           |
| `make verify-gobra`   | Run the Gobra proof through the pinned verifier image        |
| `make vulncheck`      | Scan every module for reachable known vulnerabilities        |
| `make clean`          | Remove test/coverage artifacts                               |

### Validating your work

After any code change, **always run `make validate`** from the project root
before considering the task complete. It enforces the full quality gate:

1. `fmt-check` - code must be properly formatted
2. `vet` - no suspicious constructs
3. `lint` - no lint violations (golangci-lint)
4. `test` - all tests must pass

If `make validate` fails at any step, fix the issue before proceeding. Do not
silence lint errors with `_ =` - handle them properly (return, log, or assert in
a test).

## License

`meso` is licensed GPL-3.0-or-later. This is deliberate: it lets the project
derive directly from the GPL reference implementations (see the plan), and it
binds importers, who become derivative works and must be GPLv3-compatible. Keep
new source files compatible with this license, and do not add dependencies whose
licenses conflict with GPLv3.

## Releases & supply-chain security

A library release is a git tag: `v*.*.*` for the core module, `gonum/v*.*.*`
for the nested gonum adapter. The Go module proxy serves the source with
`sum.golang.org` integrity, so releases build no source artifact. The tag
triggers the `Release` workflow, which runs the quality gate and publishes a
GitHub release carrying verifiable metadata only: the released module's SBOM
(SPDX JSON), `SHA256SUMS.txt` over it, Sigstore keyless signatures (cosign
bundles) on both, and SLSA build provenance (GitHub attestation). The release
notes carry the exact verification commands (`cosign verify-blob` and
`gh attestation verify`).

Known-vulnerability scanning runs as the `govulncheck` workflow (pull requests
plus a weekly schedule) via `make vulncheck`. Third-party actions are pinned by
commit SHA and kept current by Dependabot. An OpenSSF Scorecard lane is
deferred (tracked in the tickets).
