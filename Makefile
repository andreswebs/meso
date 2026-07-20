GO       ?= go
GOLANGCI ?= golangci-lint
LAKE     ?= lake

# The Lean formal-verification development and its value-oracle.
# Separate from the Go qualitygate: it needs the Lean toolchain
# and is not part of `validate`.
LEAN_DIR   := verification/lean
ORACLE_IN  := verification/oracle/inputs
ORACLE_OUT := verification/oracle/golden

# Every module root (directory containing a go.mod). Commands that respect module
# boundaries (go test/vet/build/mod) are run once per module; gofmt operates on
# the whole tree at once and needs no per-module loop.
MODULES := . gonum

# Fuzz targets live in the core module. `make validate` already runs their seed
# corpus (go test executes seeds without a mutation budget); `make fuzz` drives a
# real mutation search on demand, one target per invocation.
FUZZ_TARGETS := FuzzLeidenLouvain FuzzFoldingTwoM FuzzDirected
FUZZTIME     ?= 30s

# Benchmarks live in the core module. `make bench` captures a run; `make
# bench-check` compares a fresh run against the committed baseline and fails on a
# regression (ns/op beyond tolerance or any allocs/op growth). `make validate`
# does not run them - it runs only the deterministic gate self-test.
BENCH_RE    := ^BenchmarkLeiden$$|^BenchmarkLouvain$$
BENCHTIME   ?= 10x
BENCHCOUNT  ?= 6
BASELINE    := benchmarks/baseline.txt
BENCH_OUT   ?= benchmarks/new.txt
BENCHSTAT_VERSION ?= latest

# The Gobra data-race-freedom proof runs through a digest-pinned container
# image; contributors need only Docker. Never part of `validate`. The pin is
# currently an interim image built from the andreswebs/gobra fork, carrying a
# shared-float64 encoding fix not yet landed upstream; re-pin the official
# ghcr.io/viperproject/gobra digest once the fix ships (see
# verification/gobra/README.md for the exit condition). GOBRA_INPUTS lists the
# files fed to the verifier, repo-relative: the annotated parallel core plus
# its companion specification file. GOBRA_INCLUDES holds the include
# directories searched for imported packages: the local trusted stubs for
# stdlib packages (runtime) that Gobra's bundled stubs do not cover.
GOBRA_IMAGE    ?= ghcr.io/andreswebs/gobra@sha256:d3fc8fd7b6d8ae1cbd5adcc2d4f93e770ce2b3f3140a68af5bd8052a6f8d6f76
GOBRA_INPUTS   := parallel.go verification/gobra/meso.gobra
GOBRA_INCLUDES := verification/gobra/stubs

# Known-vulnerability scanning via govulncheck: call-graph aware, so it only
# reports vulnerabilities in code the modules actually reach, and it covers
# the standard library and toolchain. Run per module, on demand and by the
# scheduled CI lane; never part of `validate` (a new CVE is a fact about the
# world, not about the change under review).
GOVULNCHECK_VERSION ?= v1.6.0

# Mutation testing runs on demand (like fuzz/bench), never in `make validate`. It
# drives go-gremlins over the core, mutating source and rerunning the suite per
# mutant. The efficacy gate lives in .gremlins.yaml: gremlins v0.6.0 ignores the
# --threshold-* CLI flags (they arrive as strings and fail an internal type
# assertion), so the committed threshold must be in the config file, not here.
# gremlins copies the whole module tree into a temp working dir per worker, so
# MUTATION_WORKERS stays serial by default; raise it where cores and temp space
# allow. The per-mutant timeout is (2s + measured-suite-time) * the coefficient,
# kept generous so honest mutants are not misreported as TIMED OUT (an
# infinite-loop mutant, e.g. a loop counter's ++ flipped to --, still hits the
# ceiling and is reported TIMED OUT).
GREMLINS_VERSION       ?= v0.6.0
MUTATION_WORKERS       ?= 1
MUTATION_TIMEOUT_COEFF ?= 8

.PHONY: help validate test test-race vet build lint fmt fmt-check tidy cover clean lean oracle-lean fuzz bench bench-baseline bench-check bench-compare mutation verify-gobra vulncheck

help: ## List available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

validate: fmt-check vet lint test ## Run the full quality gate

test: ## Run tests in every module
	@set -e; for m in $(MODULES); do echo "==> test $$m"; (cd $$m && $(GO) test -v ./...); done

# -v keeps the run visibly alive: under the race detector the core suite takes
# minutes (TestLFRAccuracySweep alone dominates), and a silent multi-minute run
# is indistinguishable from a hang.
test-race: ## Run tests with the race detector
	@set -e; for m in $(MODULES); do echo "==> test-race $$m"; (cd $$m && $(GO) test -race -v ./...); done

fuzz: ## Mutation-fuzz each native target (override FUZZTIME, default 30s each)
	@set -e; for t in $(FUZZ_TARGETS); do \
		echo "==> fuzz $$t ($(FUZZTIME))"; \
		$(GO) test -run='^$$' -fuzz="^$$t$$" -fuzztime=$(FUZZTIME) .; \
	done

mutation: ## Mutation-test the core; fails under the .gremlins.yaml efficacy gate
	$(GO) run github.com/go-gremlins/gremlins/cmd/gremlins@$(GREMLINS_VERSION) unleash . \
		--workers $(MUTATION_WORKERS) \
		--timeout-coefficient $(MUTATION_TIMEOUT_COEFF) \
		-E 'gonum/'

bench: ## Run the benchmark suite, writing results to BENCH_OUT
	$(GO) test -run='^$$' -bench='$(BENCH_RE)' -benchmem -benchtime=$(BENCHTIME) -count=$(BENCHCOUNT) . | tee $(BENCH_OUT)

bench-baseline: ## Regenerate the committed benchmark baseline
	$(MAKE) bench BENCH_OUT=$(BASELINE)

bench-check: ## Fail if a fresh run regresses against the committed baseline
	$(MAKE) bench BENCH_OUT=$(BENCH_OUT)
	MESO_BENCH_CURRENT=$(BENCH_OUT) $(GO) test -run='^TestBenchmarkRegressionGate$$' -v .

bench-compare: ## Show a benchstat diff of BENCH_OUT vs the baseline (installs benchstat)
	$(GO) run golang.org/x/perf/cmd/benchstat@$(BENCHSTAT_VERSION) $(BASELINE) $(BENCH_OUT)

vulncheck: ## Scan every module for reachable known vulnerabilities
	@set -e; for m in $(MODULES); do echo "==> vulncheck $$m"; (cd $$m && $(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...); done

vet: ## Run go vet in every module
	@set -e; for m in $(MODULES); do echo "==> vet $$m"; (cd $$m && $(GO) vet ./...); done

build: ## Compile-check every module (a library produces no binary)
	@set -e; for m in $(MODULES); do echo "==> build $$m"; (cd $$m && $(GO) build ./...); done

lint: ## Run golangci-lint in every module
	@set -e; for m in $(MODULES); do echo "==> lint $$m"; (cd $$m && $(GOLANGCI) run ./...); done

fmt: ## Format all Go source
	gofmt -w .

fmt-check: ## Fail if any Go source is not formatted
	@test -z "$$(gofmt -l .)" || (echo "files not formatted:"; gofmt -l .; exit 1)

tidy: ## Tidy dependencies in every module
	@set -e; for m in $(MODULES); do echo "==> tidy $$m"; (cd $$m && $(GO) mod tidy); done

# -v for the same reason as test-race: cover reruns the whole suite, silent for
# the duration of the slowest package otherwise. It is on demand, not part of
# the validate gate, so the extra lines cost nothing.
cover: ## Run tests with coverage in every module
	@set -e; for m in $(MODULES); do echo "==> cover $$m"; (cd $$m && $(GO) test -cover -v ./...); done

clean: ## Remove test/coverage artifacts
	@rm -f coverage.out benchmarks/new.txt
	@set -e; for m in $(MODULES); do (cd $$m && $(GO) clean ./...); done

# The image entrypoint is `java -Xss128m -jar gobra.jar` with workdir /gobra,
# where the jar lives; the repo must therefore mount elsewhere (/work).
verify-gobra: ## Verify the Gobra proof through the pinned verifier image
	docker run --rm -v "$(CURDIR):/work" $(GOBRA_IMAGE) -I $(addprefix /work/,$(GOBRA_INCLUDES)) -i $(addprefix /work/,$(GOBRA_INPUTS))

lean: ## Type-check the Lean formal-verification development
	cd $(LEAN_DIR) && $(LAKE) build

oracle-lean: ## Build the Lean value-oracle and regenerate committed golden vectors
	cd $(LEAN_DIR) && $(LAKE) build mesoOracle
	@set -e; for f in $(ORACLE_IN)/*.json; do \
		stem=$$(basename $$f .json); \
		echo "==> oracle $$stem"; \
		(cd $(LEAN_DIR) && $(LAKE) exe mesoOracle ../oracle/inputs/$$stem.json ../oracle/golden/$$stem.json); \
	done
