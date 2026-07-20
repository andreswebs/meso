package meso_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/andreswebs/meso"
)

// updateLFR regenerates the committed recovery envelope instead of asserting
// against it: `go test -run TestLFRAccuracySweep -update-lfr`. It mirrors the
// golden-corpus -update idiom (a different flag name because both test files
// compile into one binary), so a reviewed change to meso's recovery on the
// frozen LFR suite is a one-command refresh.
var updateLFR = flag.Bool("update-lfr", false, "regenerate the committed LFR recovery envelope")

// lfrSeed is the fixed refinement seed for the accuracy sweep, so recovery is a
// pure function of the frozen graphs.
const lfrSeed = 1

// envelopeTol is the band half-width around the committed recovery scores. It
// absorbs nothing but genuine drift: meso is deterministic, so a correct,
// unchanged optimiser reproduces the envelope exactly. The band exists only so
// a reviewed, benign change need not land exactly on the stored decimals.
const envelopeTol = 0.02

// lowMixingNMIFloor is the literature-derived sanity floor: at the easiest
// nominal mixing in the grid, Leiden recovers planted communities almost
// perfectly, so mean NMI must clear this even if the envelope were regenerated
// against a broken optimiser (the independent guard, as in the golden corpus).
const lowMixingNMIFloor = 0.85

// recovery is the mean NMI and ARI of Leiden's partition against the planting,
// averaged over a regime/mixing cell's realizations.
type recovery struct {
	Regime string  `json:"regime"`
	Mu     float64 `json:"mu"`
	NMI    float64 `json:"nmi"`
	ARI    float64 `json:"ari"`
}

// cellKey identifies a regime/mixing cell in the sweep grid.
type cellKey struct {
	regime string
	mu     float64
}

// scoredRealizations is how many realizations of each regime/mixing cell the
// sweep scores. Scoring the first realization of every cell (rather than all
// five) keeps the always-on CI cost near ten seconds while still covering the
// full regime and mixing grid; the remaining realizations stay committed as
// frozen vectors for the benchmark and mutation tiers.
//
// ponytail: one realization per cell for CI speed; raise to average all five
// (smoother envelope) if the accuracy tier gets its own slow lane.
const scoredRealizations = 1

// sweepEntries selects the graphs to score. It always covers both regimes and
// the whole mixing grid; -short narrows to the small-community regime so local
// iteration is a couple of seconds.
func sweepEntries(m lfrManifest) []lfrEntry {
	out := make([]lfrEntry, 0, len(m.Graphs))
	for _, e := range m.Graphs {
		if e.Realization >= scoredRealizations {
			continue
		}
		if testing.Short() && e.Regime != "S" {
			continue
		}
		out = append(out, e)
	}
	return out
}

// runLFRSweep scores meso's recovery on the selected committed graphs and
// averages NMI/ARI within each regime/mixing cell, returning the cells in a
// canonical order (regime, then ascending mixing) so the envelope is
// reproducible.
func runLFRSweep(t *testing.T, m lfrManifest) []recovery {
	t.Helper()

	type acc struct {
		nmi, ari float64
		count    int
	}
	cells := make(map[cellKey]*acc)
	for _, entry := range sweepEntries(m) {
		lg := loadLFRGraph(t, entry)
		res, err := meso.Leiden(lg.g, meso.WithSeed(lfrSeed))
		if err != nil {
			t.Fatalf("Leiden(%s): %v", entry.EdgesFile, err)
		}
		detected := detectedPartition(t, lg, res)

		k := cellKey{regime: entry.Regime, mu: entry.NominalMu}
		a := cells[k]
		if a == nil {
			a = &acc{}
			cells[k] = a
		}
		a.nmi += meso.NMI(lg.planted, detected)
		a.ari += meso.ARI(lg.planted, detected)
		a.count++
	}

	out := make([]recovery, 0, len(cells))
	for k, a := range cells {
		n := float64(a.count)
		out = append(out, recovery{Regime: k.regime, Mu: k.mu, NMI: a.nmi / n, ARI: a.ari / n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Regime != out[j].Regime {
			return out[i].Regime < out[j].Regime
		}
		return out[i].Mu < out[j].Mu
	})
	return out
}

// detectedPartition converts a Result's caller-keyed communities into a dense
// partition aligned to the planting: partition[i] is the detected community of
// dense node i, so NMI/ARI compare like-indexed labelings.
func detectedPartition(t *testing.T, lg lfrGraph, res *meso.Result) []int {
	t.Helper()
	comm := res.Communities()
	part := make([]int, lg.g.NumNodes())
	for key, c := range comm {
		idx, ok := lg.g.Index(key)
		if !ok {
			t.Fatalf("detected key %q absent from graph", key)
		}
		part[idx] = c
	}
	return part
}

func envelopePath() string { return filepath.Join("testdata", "lfr", "envelope.json") }

func writeEnvelope(t *testing.T, env []recovery) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(envelopePath()), 0o755); err != nil {
		t.Fatalf("mkdir envelope dir: %v", err)
	}
	raw, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	if err := os.WriteFile(envelopePath(), append(raw, '\n'), 0o644); err != nil {
		t.Fatalf("write envelope: %v", err)
	}
}

func readEnvelope(t *testing.T) []recovery {
	t.Helper()
	raw, err := os.ReadFile(envelopePath())
	if err != nil {
		t.Fatalf("read envelope (regenerate with -update-lfr): %v", err)
	}
	var env []recovery
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("parse envelope: %v", err)
	}
	return env
}

// TestLFRAccuracySweep is acceptance criteria 3 and 4: recovery is high at low
// mixing and degrades monotonically as mixing rises, and every cell stays within
// the committed reference envelope. The envelope is a frozen vector regenerated
// with -update-lfr; the low-mixing NMI floor is an independent literature-derived
// sanity check.
func TestLFRAccuracySweep(t *testing.T) {
	m := loadLFRManifest(t)
	got := runLFRSweep(t, m)

	if *updateLFR {
		writeEnvelope(t, got)
	}
	want := readEnvelope(t)

	// Criterion 4: within the committed reference envelope.
	assertWithinEnvelope(t, got, want)

	// Criterion 3: high at low mixing, monotone degradation as mixing rises.
	byRegime := groupByRegime(got)
	for regime, cells := range byRegime {
		if cells[0].NMI < lowMixingNMIFloor {
			t.Errorf("regime %s: low-mixing (mu=%.2f) NMI = %.4f, want >= %.2f",
				regime, cells[0].Mu, cells[0].NMI, lowMixingNMIFloor)
		}
		assertMonotoneDegradation(t, regime, cells)
	}
}

// assertWithinEnvelope checks the sweep reproduces the committed cells (same
// grid) and each NMI/ARI lands within the band.
func assertWithinEnvelope(t *testing.T, got, want []recovery) {
	t.Helper()
	// A full run must cover the whole committed grid; -short scores a subset, so
	// only its scored cells are checked against the envelope.
	if !testing.Short() && len(got) != len(want) {
		t.Fatalf("sweep produced %d cells, envelope has %d", len(got), len(want))
	}
	index := make(map[cellKey]recovery, len(want))
	for _, w := range want {
		index[cellKey{regime: w.Regime, mu: w.Mu}] = w
	}
	for _, g := range got {
		w, ok := index[cellKey{regime: g.Regime, mu: g.Mu}]
		if !ok {
			t.Errorf("cell %s/mu=%.2f absent from envelope", g.Regime, g.Mu)
			continue
		}
		if diff := abs(g.NMI - w.NMI); diff > envelopeTol {
			t.Errorf("cell %s/mu=%.2f NMI = %.4f, envelope %.4f (|Δ|=%.4f > %.2f)",
				g.Regime, g.Mu, g.NMI, w.NMI, diff, envelopeTol)
		}
		if diff := abs(g.ARI - w.ARI); diff > envelopeTol {
			t.Errorf("cell %s/mu=%.2f ARI = %.4f, envelope %.4f (|Δ|=%.4f > %.2f)",
				g.Regime, g.Mu, g.ARI, w.ARI, diff, envelopeTol)
		}
	}
}

// assertMonotoneDegradation checks NMI is non-increasing as mixing rises. Cells
// are pre-sorted by ascending mixing. A small tolerance absorbs realization
// noise so a negligible uptick between adjacent cells is not a failure, while a
// real reversal (recovery improving as the problem gets harder) still fails.
func assertMonotoneDegradation(t *testing.T, regime string, cells []recovery) {
	t.Helper()
	const noise = 0.03
	for i := 1; i < len(cells); i++ {
		if cells[i].NMI > cells[i-1].NMI+noise {
			t.Errorf("regime %s: NMI rose with mixing (mu %.2f: %.4f -> mu %.2f: %.4f)",
				regime, cells[i-1].Mu, cells[i-1].NMI, cells[i].Mu, cells[i].NMI)
		}
	}
	// End to end the hardest mixing must recover strictly less than the easiest.
	if cells[len(cells)-1].NMI >= cells[0].NMI {
		t.Errorf("regime %s: hardest mixing (mu=%.2f, NMI=%.4f) did not degrade below easiest (mu=%.2f, NMI=%.4f)",
			regime, cells[len(cells)-1].Mu, cells[len(cells)-1].NMI, cells[0].Mu, cells[0].NMI)
	}
}

func groupByRegime(cells []recovery) map[string][]recovery {
	out := make(map[string][]recovery)
	for _, c := range cells {
		out[c.Regime] = append(out[c.Regime], c)
	}
	return out
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
