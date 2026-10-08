# meso

`meso` is a pure-Go, deterministic library for the mesoscale structure of a
weighted graph, the level between individual nodes and the whole network. It
detects communities with the Leiden algorithm (Traag, Waltman, van Eck, 2019),
with Louvain as its baseline, reports the multilevel hierarchy of each run,
and measures the structure around the communities: node betweenness
centrality, community cohesion, and canonical induced subgraphs.

```go
g, err := meso.NewBuilder().Canonical().
    AddEdge("a", "b", 1).AddEdge("b", "c", 1).AddEdge("c", "a", 1).
    AddEdge("c", "x", 1).AddEdge("x", "y", 1).
    Build()

res, err := meso.Leiden(g, meso.WithSeed(42))
for l := range res.NumCommunities() {
    fmt.Println(res.Members(l), res.Cohesion(l))
}

for l := range res.NumLevels() {               // the multilevel hierarchy
    fmt.Println(l, res.Level(l), res.LevelQuality(l))
}

bc := meso.Betweenness(g)                       // normalized to [0, 1]
sub, err := meso.Subgraph(g, res.Members(0))   // canonical, re-runnable
```

See [docs/meso-design.md](docs/meso-design.md) for the design of record.

## Goals

- A faithful, best-in-class Leiden implementation, with Louvain as the baseline
  and a shared quality-function core.
- Bit-reproducible output for a given input, seed, and parameters, including
  under parallelism.
- Structural measures on the same graph that is partitioned, so consumers
  never maintain a second adjacency to analyse it.
- Idiomatic, dependency-free public API with an optional gonum adapter.

## Modules

- `github.com/andreswebs/meso` - the dependency-free core.
- `github.com/andreswebs/meso/gonum` - an optional adapter to gonum's graph
  types, shipped as a separate nested module so the core never pulls gonum into
  consumers that do not want it.

## Development

All commands run from the project root via `make`; see `make help`. The full
quality gate is `make validate` (fmt-check, vet, lint, test) across every
module.

## Authors

**Andre Silva** - [@andreswebs](https://github.com/andreswebs)

## License

This project is licensed under the [GPL-3.0-or-later](LICENSE). Linking `meso`
makes the importing program a derivative work that must itself be GPLv3-
compatible.
