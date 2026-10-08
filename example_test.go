package meso_test

import (
	"fmt"

	"github.com/andreswebs/meso"
)

// twoTriangles is two triangles joined by the single bridge c-x.
func twoTriangles() *meso.Graph {
	g, err := meso.NewBuilder().Canonical().
		AddEdge("a", "b", 1).AddEdge("b", "c", 1).AddEdge("c", "a", 1).
		AddEdge("x", "y", 1).AddEdge("y", "z", 1).AddEdge("z", "x", 1).
		AddEdge("c", "x", 1).
		Build()
	if err != nil {
		panic(err)
	}
	return g
}

func ExampleBetweenness() {
	g := twoTriangles()
	bc := meso.Betweenness(g)
	for _, k := range g.Keys() {
		fmt.Printf("%s %.4f\n", k, bc[k])
	}
	// Output:
	// a 0.0000
	// b 0.0000
	// c 0.6000
	// x 0.6000
	// y 0.0000
	// z 0.0000
}

func ExampleResult_Cohesion() {
	res, err := meso.Leiden(twoTriangles(), meso.WithSeed(1))
	if err != nil {
		panic(err)
	}
	for l := range res.NumCommunities() {
		fmt.Println(res.Members(l), res.Cohesion(l))
	}
	// Output:
	// [a b c] 1
	// [x y z] 1
}

func ExampleSubgraph() {
	g := twoTriangles()
	sub, err := meso.Subgraph(g, []string{"z", "y", "x", "c"})
	if err != nil {
		panic(err)
	}
	fmt.Println(sub.Keys(), sub.NumEdges())
	w, ok := sub.Weight("c", "x")
	fmt.Println(w, ok)
	fmt.Println(sub.Neighbors("c"))
	// Output:
	// [c x y z] 4
	// 1 true
	// [x]
}

func ExampleResult_Level() {
	// Twelve triangles in a ring, each joined to the next by one edge.
	b := meso.NewBuilder().Canonical()
	node := func(t, i int) string { return fmt.Sprintf("t%02d-%d", t, i) }
	for t := range 12 {
		b.AddEdge(node(t, 0), node(t, 1), 1).AddEdge(node(t, 1), node(t, 2), 1).AddEdge(node(t, 2), node(t, 0), 1)
		b.AddEdge(node(t, 2), node((t+1)%12, 0), 1)
	}
	g, err := b.Build()
	if err != nil {
		panic(err)
	}
	res, err := meso.Louvain(g)
	if err != nil {
		panic(err)
	}
	for l := range res.NumLevels() {
		communities := map[int]bool{}
		for _, c := range res.Level(l) {
			communities[c] = true
		}
		fmt.Printf("level %d: %d communities, quality %.4f\n", l, len(communities), res.LevelQuality(l))
	}
	// The last level is the pass that found nothing more to merge, so it
	// repeats the one before it.

	// Output:
	// level 0: 12 communities, quality 0.6667
	// level 1: 6 communities, quality 0.7083
	// level 2: 6 communities, quality 0.7083
}
