package meso

import (
	"math/big"
	"testing"
)

// TestOracleToleranceHasTeeth proves the check is not vacuous: a Go value
// perturbed above tolerance is rejected, at both a nonzero magnitude and the
// exact-zero case the absolute floor guards.
func TestOracleToleranceHasTeeth(t *testing.T) {
	// A correctly-rounded value passes even under the tightest budget.
	third := big.NewRat(1, 3)
	if !withinTolerance(wantFloat(third), third, deltaBudgetULP) {
		t.Fatal("correctly-rounded 1/3 rejected")
	}
	// A relative perturbation of 1e-6 is far beyond 16 ULP and 1e-12 atol.
	if withinTolerance(wantFloat(third)*(1+1e-6), third, qualityBudgetULP) {
		t.Error("perturbed 1/3 accepted; check has no teeth")
	}
	// Near zero, ULP distance is meaningless; the atol floor must still reject a
	// value 1e-6 away from an exact zero.
	zero := big.NewRat(0, 1)
	if withinTolerance(1e-6, zero, qualityBudgetULP) {
		t.Error("1e-6 accepted against exact zero; atol floor has no teeth")
	}

	// End to end: perturbing an actual Go quality above tolerance fails the
	// harness's own comparison.
	f := loadOracleFixture(t, "triangle")
	c := f.Cases[1] // modularity [0,1,2], value -1/3
	got := f.goQuality(t, c)
	if !withinTolerance(got, c.Want, qualityBudgetULP) {
		t.Fatalf("unperturbed Go modularity %v rejected", got)
	}
	if withinTolerance(got+1e-6, c.Want, qualityBudgetULP) {
		t.Error("Go modularity perturbed by 1e-6 accepted; harness has no teeth")
	}
}

// TestOraclePredicates checks that the proved guarantee flags load and expose
// both branches (not only the passing one), and that a null subsetOptimal above
// the emitter's node bound is a skip, not a failure.
func TestOraclePredicates(t *testing.T) {
	path3 := loadOracleFixture(t, "path3")
	// path3 [0,1,0]: node 0 and node 2 share a community with no edge between
	// them, so the community is disconnected.
	if got := path3.Cases[2].Predicates.Connected; got {
		t.Errorf("path3 case 2 connected = %v, want false", got)
	}
	if so := path3.Cases[2].Predicates.SubsetOptimal; so == nil || *so {
		t.Errorf("path3 case 2 subsetOptimal = %v, want non-nil false", so)
	}

	tri := loadOracleFixture(t, "triangle")
	// triangle [0,0,1] at gamma=1/2: a dense cut between communities.
	if got := tri.Cases[4].Predicates.GammaSeparated; got {
		t.Errorf("triangle case 4 gammaSeparated = %v, want false", got)
	}

	sq := loadOracleFixture(t, "square")
	if got := sq.Cases[2].Predicates.GammaDense; got == nil || !*got {
		t.Errorf("square case 2 gammaDense = %v, want non-nil true", got)
	}
	if so := sq.Cases[3].Predicates.SubsetOptimal; so == nil || *so {
		t.Errorf("square case 3 subsetOptimal = %v, want non-nil false", so)
	}

	// Corpus graphs exceed the emitter's subsetOptimal node bound, so the flag is
	// null and every consumer must skip it rather than treat nil as false.
	karate := loadOracleFixture(t, "karate")
	for i, c := range karate.Cases {
		if c.Predicates.SubsetOptimal != nil {
			t.Errorf("karate case %d subsetOptimal = %v, want nil (skipped)", i, *c.Predicates.SubsetOptimal)
		}
	}
}

// TestOracleDeltas checks that Go's incremental move-delta matches the exact
// rational delta the oracle emits for each requested (node, target) move.
func TestOracleDeltas(t *testing.T) {
	checked := 0
	for _, name := range oracleFixtureNames {
		t.Run(name, func(t *testing.T) {
			f := loadOracleFixture(t, name)
			for i, c := range f.Cases {
				for _, d := range c.Deltas {
					got := f.goDelta(t, c, d)
					if !withinTolerance(got, d.Want, deltaBudgetULP) {
						t.Errorf("case %d delta node=%d target=%d: got %v, want %v (%s)",
							i, d.Node, d.Target, got, wantFloat(d.Want), d.Want)
					}
					checked++
				}
			}
		})
	}
	if checked == 0 {
		t.Fatal("no deltas checked across any fixture")
	}
}

// TestOracleLoadsAll parses every committed fixture (both files), builds each
// graph, and confirms every exact value and delta round-tripped into a big.Rat.
func TestOracleLoadsAll(t *testing.T) {
	for _, name := range oracleFixtureNames {
		t.Run(name, func(t *testing.T) {
			f := loadOracleFixture(t, name)
			if len(f.Cases) == 0 {
				t.Fatalf("%s: no cases", name)
			}
			for i, c := range f.Cases {
				if c.Want == nil {
					t.Errorf("%s case %d: nil value", name, i)
				}
				for j, d := range c.Deltas {
					if d.Want == nil {
						t.Errorf("%s case %d delta %d: nil value", name, i, j)
					}
				}
			}
		})
	}
}

// TestOracleQualityValues checks that Go's quality of each case partition lands
// within tolerance of the exact rational oracle value, across every fixture:
// modularity as is (matches igraph) and CPM in the canonical convention.
func TestOracleQualityValues(t *testing.T) {
	modCases, cpmCases := 0, 0
	for _, name := range oracleFixtureNames {
		t.Run(name, func(t *testing.T) {
			f := loadOracleFixture(t, name)
			for i, c := range f.Cases {
				got := f.goQuality(t, c)
				if !withinTolerance(got, c.Want, qualityBudgetULP) {
					t.Errorf("case %d %s: got %v, want %v (%s)", i, c.Quality, got, wantFloat(c.Want), c.Want)
				}
				switch c.Quality {
				case "modularity":
					modCases++
				case "cpm":
					cpmCases++
				}
			}
		})
	}
	if modCases == 0 || cpmCases == 0 {
		t.Fatalf("expected both modularity and cpm cases, got mod=%d cpm=%d", modCases, cpmCases)
	}
}

// TestOracleLoadsTriangle is the tracer bullet: the loader reads one committed
// fixture (its input and golden vectors), builds the graph, and round-trips the
// exact num/den strings into big.Rat.
func TestOracleLoadsTriangle(t *testing.T) {
	f := loadOracleFixture(t, "triangle")

	if f.Graph.NumNodes() != 3 {
		t.Fatalf("triangle NumNodes = %d, want 3", f.Graph.NumNodes())
	}
	if len(f.Cases) != 6 {
		t.Fatalf("triangle cases = %d, want 6", len(f.Cases))
	}

	// First case: modularity gamma=1 on [0,0,0], exact value 0/1.
	c := f.Cases[0]
	if c.Quality != "modularity" {
		t.Errorf("case 0 quality = %q, want modularity", c.Quality)
	}
	if want := big.NewRat(0, 1); c.Want.Cmp(want) != 0 {
		t.Errorf("case 0 value = %s, want %s", c.Want, want)
	}

	// Second case carries an exact delta of 1/9.
	d := f.Cases[1].Deltas
	if len(d) != 1 {
		t.Fatalf("case 1 deltas = %d, want 1", len(d))
	}
	if want := big.NewRat(1, 9); d[0].Want.Cmp(want) != 0 {
		t.Errorf("case 1 delta = %s, want %s", d[0].Want, want)
	}
}
