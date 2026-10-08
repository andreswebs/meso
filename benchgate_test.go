package meso

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// benchSample is one benchmark line's reported metrics: nanoseconds, bytes, and
// allocations per operation. A zero value means the metric was absent from the
// line (a benchmark without b.ReportAllocs reports no B/op or allocs/op).
type benchSample struct {
	nsPerOp     float64
	bytesPerOp  float64
	allocsPerOp float64
}

// parseBenchmarkOutput reads `go test -bench` output and groups the samples by
// benchmark name, stripping the trailing "-N" GOMAXPROCS suffix so a baseline
// captured under one core count compares against a run under another. A line is
// a benchmark result when its first field starts with "Benchmark" and it carries
// a "ns/op" metric; every other line (headers, PASS, ok) is ignored.
func parseBenchmarkOutput(r io.Reader) (map[string][]benchSample, error) {
	out := make(map[string][]benchSample)
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 4 || !strings.HasPrefix(fields[0], "Benchmark") {
			continue
		}
		sample, ok, err := parseBenchFields(fields[1:])
		if err != nil {
			return nil, fmt.Errorf("benchmark %q: %w", fields[0], err)
		}
		if !ok {
			continue
		}
		name := stripProcSuffix(fields[0])
		out[name] = append(out[name], sample)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// parseBenchFields reads the value/unit pairs after a benchmark's name and
// iteration count. It returns ok=false when the line carries no ns/op metric,
// which filters out non-result lines whose first field happens to start with
// "Benchmark".
func parseBenchFields(fields []string) (benchSample, bool, error) {
	var s benchSample
	haveNs := false
	for i := 0; i+1 < len(fields); i++ {
		unit := fields[i+1]
		dst := map[string]*float64{
			"ns/op":     &s.nsPerOp,
			"B/op":      &s.bytesPerOp,
			"allocs/op": &s.allocsPerOp,
		}[unit]
		if dst == nil {
			continue
		}
		v, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return benchSample{}, false, fmt.Errorf("metric %s: %w", unit, err)
		}
		*dst = v
		if unit == "ns/op" {
			haveNs = true
		}
	}
	return s, haveNs, nil
}

// benchThresholds is the slack the gate allows before calling a change a
// regression, as a fraction of the baseline: a current median above
// baseline*(1+fraction) is flagged. ns/op gets headroom for machine variance;
// allocs/op is deterministic per graph, so its fraction is normally 0 (no
// hot-path allocation growth allowed, per design section 8).
type benchThresholds struct {
	nsFraction     float64
	allocsFraction float64
}

// regression is one flagged metric: the benchmark, which metric regressed, and
// the baseline and current medians that triggered it.
type regression struct {
	name     string
	metric   string
	baseline float64
	current  float64
}

// detectRegressions compares current benchmark medians against a baseline and
// returns every metric that grew beyond its threshold, sorted by name then
// metric for a stable report. Only benchmarks present in both sets are compared;
// a baseline entry absent from the current run is reported as a missing
// benchmark so an accidental deletion cannot pass silently. A baseline median of
// zero is skipped (no meaningful ratio).
func detectRegressions(base, cur map[string][]benchSample, thr benchThresholds) []regression {
	var regs []regression
	for name, bs := range base {
		cs, ok := cur[name]
		if !ok {
			regs = append(regs, regression{name: name, metric: "missing"})
			continue
		}
		bm, cm := summarize(bs), summarize(cs)
		if exceeds(bm.nsPerOp, cm.nsPerOp, thr.nsFraction) {
			regs = append(regs, regression{name, "ns/op", bm.nsPerOp, cm.nsPerOp})
		}
		if exceeds(bm.allocsPerOp, cm.allocsPerOp, thr.allocsFraction) {
			regs = append(regs, regression{name, "allocs/op", bm.allocsPerOp, cm.allocsPerOp})
		}
	}
	sort.Slice(regs, func(i, j int) bool {
		if regs[i].name != regs[j].name {
			return regs[i].name < regs[j].name
		}
		return regs[i].metric < regs[j].metric
	})
	return regs
}

// exceeds reports whether current is more than fraction above baseline. A
// non-positive baseline is never a regression: there is no ratio to grow.
func exceeds(baseline, current, fraction float64) bool {
	if baseline <= 0 {
		return false
	}
	return current > baseline*(1+fraction)
}

// summarize reduces a benchmark's samples to their per-metric medians, the
// robust central tendency benchstat also reports. An empty slice yields a zero
// sample.
func summarize(samples []benchSample) benchSample {
	if len(samples) == 0 {
		return benchSample{}
	}
	return benchSample{
		nsPerOp:     median(samples, func(s benchSample) float64 { return s.nsPerOp }),
		bytesPerOp:  median(samples, func(s benchSample) float64 { return s.bytesPerOp }),
		allocsPerOp: median(samples, func(s benchSample) float64 { return s.allocsPerOp }),
	}
}

// median returns the median of one metric across samples. For an even count it
// averages the two middle values, the standard definition.
func median(samples []benchSample, sel func(benchSample) float64) float64 {
	vs := make([]float64, len(samples))
	for i, s := range samples {
		vs[i] = sel(s)
	}
	sort.Float64s(vs)
	n := len(vs)
	if n%2 == 1 {
		return vs[n/2]
	}
	return (vs[n/2-1] + vs[n/2]) / 2
}

// stripProcSuffix removes a trailing "-N" (the GOMAXPROCS the benchmark ran
// under) from a benchmark name so names compare across core counts.
func stripProcSuffix(name string) string {
	i := strings.LastIndexByte(name, '-')
	if i < 0 {
		return name
	}
	if _, err := strconv.Atoi(name[i+1:]); err != nil {
		return name
	}
	return name[:i]
}

// sampleBenchOutput is a fragment of `go test -bench` output with two runs of a
// benchmark, exercising the name-suffix stripping and the value/unit pairing the
// parser must handle.
const sampleBenchOutput = `goos: linux
goarch: arm64
pkg: github.com/andreswebs/meso
BenchmarkLeiden/karate-8    	    9789	    122333 ns/op	    4096 B/op	      12 allocs/op
BenchmarkLeiden/karate-8    	    9801	    120001 ns/op	    4096 B/op	      12 allocs/op
BenchmarkLouvain/karate-8   	   19011	     61000 ns/op	    2048 B/op	       8 allocs/op
PASS
ok  	github.com/andreswebs/meso	3.456s
`

func TestParseBenchmarkOutput(t *testing.T) {
	got, err := parseBenchmarkOutput(strings.NewReader(sampleBenchOutput))
	if err != nil {
		t.Fatalf("parseBenchmarkOutput() error = %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("parseBenchmarkOutput() found %d benchmarks, want 2", len(got))
	}

	leiden, ok := got["BenchmarkLeiden/karate"]
	if !ok {
		t.Fatalf("parseBenchmarkOutput() missing BenchmarkLeiden/karate; got keys %v", keysOf(got))
	}
	if len(leiden) != 2 {
		t.Fatalf("BenchmarkLeiden/karate has %d samples, want 2", len(leiden))
	}
	if got, want := leiden[0].nsPerOp, 122333.0; got != want {
		t.Errorf("leiden[0].nsPerOp = %v, want %v", got, want)
	}
	if got, want := leiden[1].nsPerOp, 120001.0; got != want {
		t.Errorf("leiden[1].nsPerOp = %v, want %v", got, want)
	}
	if got, want := leiden[0].allocsPerOp, 12.0; got != want {
		t.Errorf("leiden[0].allocsPerOp = %v, want %v", got, want)
	}
	if got, want := leiden[0].bytesPerOp, 4096.0; got != want {
		t.Errorf("leiden[0].bytesPerOp = %v, want %v", got, want)
	}
}

// TestDetectRegressions is the self-test of the regression gate (acceptance
// criterion 3): a deliberately slowed-down and a newly-allocating benchmark must
// be flagged, while runs within tolerance must not be. It does not run the real
// benchmarks - it feeds the comparator a synthetic before/after so the gate's
// own logic is exercised deterministically under `make validate`.
func TestDetectRegressions(t *testing.T) {
	base := map[string][]benchSample{
		"BenchmarkLeiden/small":  {{nsPerOp: 100, allocsPerOp: 5}},
		"BenchmarkLouvain/small": {{nsPerOp: 200, allocsPerOp: 10}},
		"BenchmarkLeiden/big":    {{nsPerOp: 1000, allocsPerOp: 20}},
	}
	cur := map[string][]benchSample{
		"BenchmarkLeiden/small":  {{nsPerOp: 160, allocsPerOp: 5}},   // 60% slower: ns regression
		"BenchmarkLouvain/small": {{nsPerOp: 205, allocsPerOp: 12}},  // 2.5% slower ok, +2 allocs: allocs regression
		"BenchmarkLeiden/big":    {{nsPerOp: 1100, allocsPerOp: 20}}, // 10% slower: within tolerance
	}
	thr := benchThresholds{nsFraction: 0.20, allocsFraction: 0}

	regs := detectRegressions(base, cur, thr)

	want := []string{
		"BenchmarkLeiden/small ns/op",
		"BenchmarkLouvain/small allocs/op",
	}
	var got []string
	for _, r := range regs {
		got = append(got, r.name+" "+r.metric)
	}
	if strings.Join(got, "; ") != strings.Join(want, "; ") {
		t.Errorf("detectRegressions() flagged %v, want %v", got, want)
	}
}

// TestBenchmarkBaselineValid guards the committed baseline (acceptance criterion
// 2): it must parse, cover both algorithms across the small-to-large tier and
// the structural measures (mes-z1rf), and
// carry non-zero ns/op and allocs/op for every sample. A truncated or malformed
// baseline fails here under `make validate`, before the gate ever compares
// against it.
func TestBenchmarkBaselineValid(t *testing.T) {
	got := loadBenchFile(t, baselinePath)

	want := []string{
		"BenchmarkLeiden/karate", "BenchmarkLouvain/karate",
		"BenchmarkLeiden/planted-300", "BenchmarkLouvain/planted-300",
		"BenchmarkLeiden/planted-800", "BenchmarkLouvain/planted-800",
		"BenchmarkBetweenness/karate", "BenchmarkBetweenness/lesmis", "BenchmarkBetweenness/lfr-S-mu030",
		"BenchmarkSubgraph/karate-largest",
	}
	for _, name := range want {
		samples, ok := got[name]
		if !ok {
			t.Errorf("baseline %s missing %s", baselinePath, name)
			continue
		}
		for i, s := range samples {
			if s.nsPerOp <= 0 {
				t.Errorf("%s sample %d: nsPerOp = %v, want > 0", name, i, s.nsPerOp)
			}
			if s.allocsPerOp <= 0 {
				t.Errorf("%s sample %d: allocsPerOp = %v, want > 0", name, i, s.allocsPerOp)
			}
		}
	}
}

// TestBenchmarkRegressionGate is the enforcement entry point the `bench-check`
// Make target drives: it compares a freshly captured benchmark run (path in
// MESO_BENCH_CURRENT) against the committed baseline and fails on any regression.
// It is skipped when the env var is unset, so `make validate` never runs the
// slow benchmarks - the deterministic self-test in TestDetectRegressions covers
// the gate logic there.
func TestBenchmarkRegressionGate(t *testing.T) {
	current := os.Getenv("MESO_BENCH_CURRENT")
	if current == "" {
		t.Skip("set MESO_BENCH_CURRENT to a benchmark output file to run the gate")
	}
	base := loadBenchFile(t, baselinePath)
	cur := loadBenchFile(t, current)

	thr := benchThresholds{
		nsFraction:     envFraction("MESO_BENCH_NS_FRACTION", 1.0),
		allocsFraction: envFraction("MESO_BENCH_ALLOCS_FRACTION", 0.0),
	}
	for _, r := range detectRegressions(base, cur, thr) {
		if r.metric == "missing" {
			t.Errorf("benchmark %s present in baseline but absent from %s", r.name, current)
			continue
		}
		t.Errorf("regression: %s %s %.0f -> %.0f (+%.1f%%, allowed +%.1f%%)",
			r.name, r.metric, r.baseline, r.current,
			100*(r.current/r.baseline-1), 100*thresholdFor(r.metric, thr))
	}
}

// baselinePath is the committed reference the regression gate compares against.
const baselinePath = "benchmarks/baseline.txt"

// loadBenchFile reads and parses a benchmark-output file, failing the test if it
// cannot be read or parsed.
func loadBenchFile(t *testing.T, path string) map[string][]benchSample {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	got, err := parseBenchmarkOutput(f)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return got
}

// envFraction reads a fractional threshold override from the environment,
// falling back to def when unset or unparseable.
func envFraction(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

// thresholdFor returns the fraction that applies to a metric, for the failure
// message only.
func thresholdFor(metric string, thr benchThresholds) float64 {
	if metric == "allocs/op" {
		return thr.allocsFraction
	}
	return thr.nsFraction
}

// keysOf returns the sorted-insensitive key set of a parse result, for failure
// messages only.
func keysOf(m map[string][]benchSample) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
