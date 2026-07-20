# Gobra: an orientation guide for setting it up in a Go project

An orientation for teams considering [Gobra](https://github.com/viperproject/gobra),
the deductive verifier for Go from ETH Zurich's Programming Methodology group.
It covers what the tool proves, what infrastructure it needs, how annotations
attach to a codebase, what ships with the tool versus what must be written, and
the known limitations to weigh before committing. It is generic background, not
a walkthrough for any particular codebase.

## What Gobra is and what it proves

Gobra is an automated, modular verifier for heap-manipulating, concurrent Go
programs, built on the [Viper verification infrastructure](https://www.pm.inf.ethz.ch/research/viper.html).
It takes Go code annotated with logical assertions (preconditions,
postconditions, loop invariants, predicates) and translates it into the Viper
intermediate verification language; an SMT-based backend (Z3) then discharges
the proof obligations. The design is described in the CAV 2021 paper
[Gobra: Modular Specification and Verification of Go Programs](https://arxiv.org/abs/2105.13840).

Out of the box, a successful verification establishes:

- Memory safety: no nil dereferences, no out-of-bounds accesses.
- Crash safety: no panics from failed runtime checks.
- Data-race freedom: no two goroutines access the same location
  concurrently unless both accesses are reads.
- Partial correctness: any user-written contracts hold.

The race-freedom guarantee comes from permission-based separation logic. Every
heap location carries an access permission; reading requires holding a non-zero
fraction of it, writing requires holding all of it. Spawning a goroutine
transfers the permissions its precondition demands, and synchronization
primitives (mutexes, channels, wait groups) are the vehicles through which
permissions move back. A program that verifies cannot race, because two writers
would need more than one full permission to the same location.

Verification is modular: each function is verified against the contracts of
what it calls, not against their bodies. This makes verification compositional
and incremental, but it also means everything on the proof path needs at least
a minimal contract, including third-party and standard-library code (see the
stubs section below).

## Annotation styles: `.gobra` files versus annotated `.go` files

Gobra accepts input in two forms, and the choice shapes the whole workflow.

The first form is standalone files with the `.gobra` extension: Go-like source
where specification constructs (`requires`, `ensures`, `invariant`, `pred`,
`ghost`, `pure`, `trusted`, `decreases`) are first-class syntax. This suits
models, specification-only stubs, and ghost-code libraries.

The second form, and the one that fits verifying a real codebase, is
annotating the actual `.go` files with specially marked comments. Annotations
are ordinary Go comments prefixed with `// @` (line form) or wrapped in
`/*@ ... @*/` (inline form), so they cost nothing at runtime, do not affect the
build, and survive `gofmt`. This is the style used at scale by
[VerifiedSCION](https://github.com/viperproject/VerifiedSCION), the largest
public Gobra project, which proved memory safety, crash safety, and
race-freedom of a production router's packet-processing code. Representative
examples of the syntax on real code:

```go
// @ requires acc(&s.Field, R20)
// @ ensures  err == nil ==> acc(res.Mem(), R15)
func (s *T) Method() (res net.Addr, err error)
```

```go
// @ invariant forall j int :: 0 <= j && j < 8 ==> acc(&buf[j])
// @ decreases 8 - i
for i := 0; i < 8; i += 2 { ... }
```

```go
func Serialize(b Buffer /*@ , ghost ubuf []byte @*/) error
```

Ghost statements (`fold`, `unfold`, lemma calls) interleave with real
statements as `// @` lines inside function bodies. Definitions that have no
runtime counterpart at all, such as predicates and pure ghost functions,
conventionally live in companion `.gobra` files next to the package they
support, keeping the shipped sources readable.

The main tradeoff of inline annotation is comment noise in production files
and the risk of annotation drift: annotations are only checked when the
verifier runs, so CI must run it on every change to the annotated packages, or
the specs silently rot.

## Toolchain and installation options

Gobra is a JVM application. The moving parts:

- Java 11 or later, 64-bit.
- The Z3 SMT solver, pointed to by the `Z3_EXE` environment variable.
- Optionally Boogie (`BOOGIE_EXE`) for the alternative Carbon backend; the
  default Silicon backend needs only Z3 and is the usual choice.

Three ways to obtain and run it, in decreasing order of convenience:

1. **Docker image.** The project publishes a container image at
   `ghcr.io/viperproject/gobra` that bundles the JVM, Z3, and the Gobra JAR.
   This is the lowest-friction option for both local runs and CI, and it is
   the natural thing to wrap in a `make verify` target so contributors need no
   Java or Z3 installed.
2. **Prebuilt JAR.** Run with `java -Xss128m -jar gobra.jar -i <files>`. The
   large thread stack (`-Xss`) is required; the verifier recurses deeply. Use
   `-h` to list flags; the ones that matter early are input selection, include
   directories for stubs, backend choice, and per-run timeouts.
3. **Build from source.** Clone [viperproject/gobra](https://github.com/viperproject/gobra),
   `git submodule update --init --recursive`, then `sbt compile` (sbt 1.4.4+).
   Only needed to patch the tool or track unreleased features.

Editor support exists as [gobra-ide](https://github.com/viperproject/gobra-ide),
a VS Code extension that manages the tool dependencies itself and gives
per-function verification feedback, and gobra-mode for Emacs. The
[Gobra book](https://viperproject.github.io/gobra-book/) hosts an online
playground for trying specifications without installing anything.

## Continuous integration

The official [gobra-action](https://github.com/viperproject/gobra-action)
integrates verification into GitHub Actions. It is Docker-based and actively
maintained. Its inputs cover the operational knobs that matter in practice:

- `projectLocation`: path to the Go project to verify.
- `caching`: reuse of previous verification results; the action writes a
  `cache.json` artifact that can be persisted across runs with the standard
  cache action, so unchanged functions are not re-verified.
- `globalTimeout` and `packageTimeout`: wall-clock ceilings for the whole run
  and per package.
- `javaXss`: the JVM stack-size setting.
- `viperBackend`: backend selection, e.g. Silicon variants.

It also emits a `stats.json` with per-member timing, useful for spotting which
functions dominate verification time.

Budget expectations: SMT-backed verification of annotated concurrent code
takes minutes, not seconds, and can be timeout-sensitive as specs grow. The
common posture is to keep verification out of the fast per-push CI lane and
run it as a nightly or on-demand job, or gate it to pull requests that touch
the annotated packages, with caching enabled in all cases.

## What ships with the tool versus what you write

Because verification is modular, every function the annotated code calls needs
a contract. Three sources supply them:

**Built-in standard-library stubs.** Gobra bundles specification stubs for a
small set of standard-library packages (under `src/main/resources/stubs` in
the repository). Notably, `sync` is covered: `sync.Mutex` gets the classic
lock-invariant treatment (`SetInv`, `Lock`, `Unlock`), and `sync.WaitGroup`
gets a debt-and-token protocol in which `Add` issues permission debts, each
goroutine's `Done` settles one, and `Wait` redeems tokens that return the
goroutines' permissions to the caller. That protocol is the standard mechanism
for proving fork-join patterns race-free, and it is verbose: expect ghost
parameters and ghost statements around every wait-group operation. Channels
are supported natively with permission-carrying send and receive
specifications, initialized through ghost operations.

**Community specification libraries.**
[gobra-libs](https://github.com/viperproject/gobra-libs) is the standard
library of the verifier itself: definitions and lemmas for mathematical sets,
sequences, dictionaries, and Go maps, distilled from large verification
projects. VerifiedSCION's repository additionally contains battle-tested
utility predicates (for byte slices and slice ranges, among others) worth
studying or vendoring; slice-range permission predicates are the workhorse of
any proof about goroutines writing disjoint chunks of a shared slice.

**Your own trusted stubs.** Anything else the code imports needs a stub you
write yourself, marked `trusted` so Gobra assumes rather than checks it. These
are usually tiny (a signature plus a one-line postcondition), but each one is
an axiom: keep them minimal, audit them, and quarantine them in a dedicated
directory so reviewers can see the trusted computing base at a glance.

## Language coverage and known limitations

Gobra supports a large subset of Go, including structs, interfaces (with
behavioral subtyping obligations on implementations), first-order functions,
goroutines, channels, and defer. It is nonetheless a research tool, and the
gaps matter when scoping work:

- **Closures.** Supported via specification entailments (from
  [Milizia's ETH thesis](https://ethz.ch/content/dam/ethz/special-interest/infk/chair-program-method/pm/documents/Education/Theses/Stefano_Milizia_MS_Report.pdf)),
  but entailment proofs are manual. Code that passes capturing function
  literals across goroutine boundaries is often cheaper to refactor into named
  top-level functions than to verify as-is.
- **Floating point.** Initial support landed in 2022 and conversions in 2023,
  but rough edges remain open in the tracker (literal handling, scientific
  notation performance). Proofs that only need race-freedom can sidestep this
  entirely: give float-heavy functions footprint-only contracts (what they may
  read and write) and mark their bodies `trusted`, keeping arithmetic outside
  the proof.
- **Interfaces on the proof path.** Supported, but every implementation must
  be proven to satisfy the interface's contracts, so a spec on an interface
  method fans out proof work across all implementations. Footprint-only specs
  keep that fan-out shallow.
- **Incompleteness and churn.** Real-world code routinely needs auxiliary
  lemmas, ghost code, or idiom rewrites before it verifies, and tool updates
  occasionally regress previously passing proofs. Pin the verifier version
  (the Docker image tag or JAR release) the same way you pin any toolchain.
- **Scope of guarantees.** Race-freedom and safety are per the modular,
  contract-based model; global liveness properties such as deadlock freedom
  are largely outside it, though termination of individual functions can be
  proven with `decreases` clauses.

The honest framing for planning: budget for verification as proof engineering,
not as running a linter. The permission choreography around concurrency
primitives is well-documented but labor-intensive, and the published large
projects all report needing project-specific ghost libraries.

## A typical repository layout

A shape that has worked in public projects:

```text
verification/
  gobra/
    README.md          verification status and how to run the proof
    stubs/             trusted contracts for unspecified imports
    <pkg>.gobra        companion predicates and ghost code per package
<pkg>/*.go             real sources, annotated inline with // @ comments
Makefile               a verify target wrapping the Docker image
.github/workflows/     gobra-action in a nightly or on-demand lane
```

The principles behind it: annotations live with the code they specify, ghost
definitions live beside rather than inside the shipped sources, everything
trusted is quarantined and reviewable in one place, and no contributor needs a
JVM to build or test the project itself.

## Suggested first steps in a new project

1. Run the tutorial examples through the Docker image or the online
   playground to internalize the permission model before touching real code.
2. Pick the smallest self-contained concurrent unit in the codebase and scope
   the proof to it; treat everything it calls as trusted footprint-only
   contracts initially.
3. Annotate bottom-up: leaf helpers first with minimal contracts, then loop
   invariants, then the goroutine fork-join with the wait-group or channel
   protocol last, since it is the expensive part.
4. Wire the Makefile target and CI lane early, with caching, so the proof is
   continuously checked from the first verified function onward.

## References

- Gobra repository: <https://github.com/viperproject/gobra>
- Official tutorial: <https://github.com/viperproject/gobra/blob/master/docs/tutorial.md>
- Gobra book (guide and playground): <https://viperproject.github.io/gobra-book/>
- ETH project page: <https://www.pm.inf.ethz.ch/research/gobra.html>
- CAV 2021 paper (extended version): <https://arxiv.org/abs/2105.13840>
- CI action: <https://github.com/viperproject/gobra-action>
- VS Code extension: <https://github.com/viperproject/gobra-ide>
- Specification library: <https://github.com/viperproject/gobra-libs>
- Flagship case study: <https://github.com/viperproject/VerifiedSCION>
- Viper infrastructure: <https://www.pm.inf.ethz.ch/research/viper.html>
- Closure verification thesis: <https://ethz.ch/content/dam/ethz/special-interest/infk/chair-program-method/pm/documents/Education/Theses/Stefano_Milizia_MS_Report.pdf>
