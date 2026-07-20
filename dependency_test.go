package meso_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestCoreDependencyGraphFreeOfGonum is acceptance criterion 3 of the gonum
// adapter (ticket mes-b0id): the core module (.) must never pull gonum into a
// consumer's build graph. It lists the transitive build dependencies of every
// core package and asserts none is a gonum package. The optional adapter keeps
// gonum confined to its own module (github.com/andreswebs/meso/gonum) precisely
// so this invariant holds.
func TestCoreDependencyGraphFreeOfGonum(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", "./...").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps failed: %v\n%s", err, out)
	}
	for pkg := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if strings.HasPrefix(pkg, "gonum.org/") || pkg == "github.com/andreswebs/meso/gonum" {
			t.Errorf("core build graph depends on %q; the core must stay gonum-free", pkg)
		}
	}
}
