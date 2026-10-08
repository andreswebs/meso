package gonum_test

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/andreswebs/meso"
	mesogonum "github.com/andreswebs/meso/gonum"
	"gonum.org/v1/gonum/graph/simple"
)

// gonumKarate builds Zachary's karate club from a copy of the core's corpus
// GML (inside this module, so the test runs from the module cache too) as an
// unweighted gonum graph over the GML node ids.
func gonumKarate(t *testing.T) *simple.UndirectedGraph {
	t.Helper()
	data, err := os.ReadFile("testdata/karate.gml")
	if err != nil {
		t.Fatalf("read karate: %v", err)
	}
	g := simple.NewUndirectedGraph()
	var src int64
	for line := range strings.SplitSeq(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			continue
		}
		v, err := strconv.ParseInt(f[1], 10, 64)
		if err != nil {
			continue
		}
		switch f[0] {
		case "id":
			if g.Node(v) == nil {
				g.AddNode(simple.Node(v))
			}
		case "source":
			src = v
		case "target":
			g.SetEdge(g.NewEdge(simple.Node(src), simple.Node(v)))
		}
	}
	return g
}

func TestBetweenness_MatchesCoreOnKarate(t *testing.T) {
	mg, err := mesogonum.Build(gonumKarate(t))
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	got, err := mesogonum.Betweenness(mg)
	if err != nil {
		t.Fatalf("Betweenness() error = %v", err)
	}
	want := meso.Betweenness(mg)
	if len(got) != 34 || len(got) != len(want) {
		t.Fatalf("Betweenness() has %d values, want 34", len(got))
	}
	for k, w := range want {
		id, _ := strconv.ParseInt(k, 10, 64)
		if got[id] != w {
			t.Errorf("node %d = %v, core %v", id, got[id], w)
		}
	}
	if v := got[1]; v < 0.4375 || v > 0.4377 {
		t.Errorf("node 1 = %v, want about 0.4376", v)
	}
}

func TestMembers_MatchesCoreOnKarate(t *testing.T) {
	mg, err := mesogonum.Build(gonumKarate(t))
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	res, err := meso.Leiden(mg, meso.WithSeed(1))
	if err != nil {
		t.Fatalf("Leiden() error = %v", err)
	}
	total := 0
	for l := range res.NumCommunities() {
		got, err := mesogonum.Members(res, l)
		if err != nil {
			t.Fatalf("Members(%d) error = %v", l, err)
		}
		want := res.Members(l)
		if len(got) != len(want) {
			t.Fatalf("Members(%d) has %d ids, core %d", l, len(got), len(want))
		}
		for i, k := range want {
			if strconv.FormatInt(got[i], 10) != k {
				t.Errorf("Members(%d)[%d] = %d, core %q (dense order)", l, i, got[i], k)
			}
		}
		total += len(got)
	}
	if total != 34 {
		t.Errorf("members cover %d nodes, want 34", total)
	}
	if ids, err := mesogonum.Members(res, res.NumCommunities()); ids != nil || err != nil {
		t.Errorf("Members(out of range) = (%v, %v), want (nil, nil)", ids, err)
	}
}

func TestHelpers_RejectForeignKeys(t *testing.T) {
	g, err := meso.NewBuilder().AddEdge("alice", "bob", 1).AddEdge("bob", "carol", 1).Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if _, err := mesogonum.Betweenness(g); err == nil {
		t.Error("Betweenness() on non-numeric keys: error = nil, want error")
	}
	res, err := meso.Louvain(g)
	if err != nil {
		t.Fatalf("Louvain() error = %v", err)
	}
	if _, err := mesogonum.Members(res, 0); err == nil {
		t.Error("Members() on non-numeric keys: error = nil, want error")
	}
}
