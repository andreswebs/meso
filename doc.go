// Package meso is a pure-Go, deterministic library for the mesoscale structure
// of graphs: community detection and the structural measures around it.
//
// meso recovers the mesoscale structure of a weighted graph, the level between
// individual nodes and the whole network. Communities come from the Leiden
// algorithm (Traag, Waltman, van Eck, 2019) with Louvain as its baseline;
// alongside them meso measures node betweenness centrality and community
// cohesion, and extracts induced subgraphs. Output is bit-reproducible for a
// given input, seed, and parameters, including under parallelism.
//
// The core is dependency-free. An optional gonum adapter ships as a separate
// nested module, github.com/andreswebs/meso/gonum, so importers who do not want
// gonum never pull it in.
//
// A [Builder] is the entry point: it maps arbitrary caller keys to dense node
// indices, accepts weighted edges and optional node weights, and folds
// self-loops and parallel edges into edge weights, producing an immutable
// [Graph].
//
//	g, err := meso.NewBuilder().
//		AddEdge("a", "b", 1.0).
//		AddEdge("b", "c", 2.0).
//		Build()
//
// [Leiden] and [Louvain] consume a Graph and return a [Result] with the detected
// communities (keyed by the caller's keys) and the achieved quality. The
// objective, resolution, and seed are set through options.
//
//	res, err := meso.Leiden(g, meso.WithQuality(meso.Modularity(1.0)), meso.WithSeed(42))
//	// res.Communities(), res.Quality()
//
// A run is byte-identical for a given graph, options, and seed.
//
// Directed graphs are supported. Build one with [NewDirectedBuilder], where
// AddEdge(from, to, w) records an arc from -> to, and optimise it with
// [DirectedModularity], the Leicht-Newman directed objective whose null model
// credits an arc against its source's out-strength and its target's in-strength.
// A directed graph must be run with DirectedModularity: the undirected
// [Modularity] and [CPM] null models mis-score directed arcs and are rejected.
// On a symmetric graph DirectedModularity reduces to Modularity. CPM stays
// undirected by design.
//
// # Structural measures
//
// A Graph exposes its folded structure through [Graph.Keys], [Graph.NumEdges],
// [Graph.Degree], [Graph.Neighbors] and [Graph.Weight], so callers can reconcile
// meso's answers with their own edge sets. An edge whose folded weight is zero
// is not stored, so every reported edge has positive weight. A Result reports
// [Result.NumCommunities], each community's [Result.Members], and its
// [Result.Cohesion], the internal edge density. The multilevel run's hierarchy
// is available through [Result.NumLevels], [Result.Level] and
// [Result.LevelQuality], finest level first, the last being the result.
//
// [Betweenness] computes node betweenness centrality by Brandes' algorithm over
// unweighted shortest paths, normalized to [0, 1]. [Subgraph] returns the
// canonically indexed graph induced by a key set, so a second Leiden pass over
// one community is a pure function of its members:
//
//	bc := meso.Betweenness(g)
//	sub, err := meso.Subgraph(g, res.Members(0))
//	inner, err := meso.Leiden(sub, meso.WithSeed(42))
//
// These measures are structural: they count edges and hops and ignore edge and
// node weights, except that Subgraph carries every weight over unchanged.
package meso
