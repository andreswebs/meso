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
