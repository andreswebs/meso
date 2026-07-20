# meso

`meso` is a pure-Go, deterministic community-detection library. It recovers the
mesoscale structure of a weighted graph, the level between individual nodes and
the whole network, using the Leiden algorithm (Traag, Waltman, van Eck, 2019)
with Louvain as its baseline.

See [docs/meso-design.md](docs/meso-design.md) for the design of record.

## Goals

- A faithful, best-in-class Leiden implementation, with Louvain as the baseline
  and a shared quality-function core.
- Bit-reproducible output for a given input, seed, and parameters, including
  under parallelism.
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
