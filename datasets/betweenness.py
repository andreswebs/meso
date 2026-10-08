#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = ["networkx==3.4.2"]
# ///
"""Generate the committed networkx betweenness references for meso's corpus.

One-time, out-of-band generator (never in CI). For each corpus GML it writes
`betweenness.csv` beside the GML: one `key,value` row per node, the key being
the GML node `id` (the key meso's test loader uses) and the value networkx's
`betweenness_centrality(G, normalized=True)` with no weight argument, printed
with full round-trip precision. That matches meso's `Betweenness` semantics:
unweighted shortest paths, self-loops ignored, parallel edges folded.

GML files that list an edge more than once (celegansneural) are read as
multigraphs and collapsed to a simple graph, the same folding meso's Builder
does. Self-loops are dropped; they cannot lie on a shortest path between two
distinct nodes.

Reproduce, from the repository root, with uv installed:

    uv run datasets/betweenness.py

Regenerate only when a corpus GML or the pinned networkx version changes.
"""

from __future__ import annotations

import csv
import sys
from pathlib import Path
from typing import Any, cast

import networkx as nx

CORPUS = ("karate", "dolphins", "lesmis", "celegansneural")


def load(path: Path) -> Any:
    # The graph stays Any at this library boundary: networkx's runtime classes
    # are not generic while its bundled stubs are, so no single annotation
    # satisfies both pyright and ty. Values are narrowed where they are read.
    text = path.read_text(encoding="utf-8")
    try:
        graph: Any = cast("Any", nx.parse_gml(text, label="id"))  # pyright: ignore[reportUnknownMemberType]
    except nx.NetworkXError:
        graph = cast("Any", nx.parse_gml(text.replace("graph\n[", "graph\n[\n  multigraph 1", 1), label="id"))  # pyright: ignore[reportUnknownMemberType]
    simple: Any = cast("Any", nx.DiGraph(graph) if graph.is_directed() else nx.Graph(graph))
    simple.remove_edges_from(list(nx.selfloop_edges(simple)))  # pyright: ignore[reportUnknownMemberType]
    return simple


def write(graph: Any, out: Path) -> None:
    values = cast("dict[int, float]", nx.betweenness_centrality(graph, normalized=True))  # pyright: ignore[reportUnknownMemberType]
    with out.open("w", newline="", encoding="utf-8") as f:
        writer = csv.writer(f, lineterminator="\n")
        writer.writerow(["key", "betweenness"])
        for node in sorted(values):
            writer.writerow([node, repr(values[node])])


def main() -> int:
    root = Path(__file__).resolve().parent
    for name in CORPUS:
        graph = load(root / name / f"{name}.gml")
        out = root / name / "betweenness.csv"
        write(graph, out)
        kind = "directed" if graph.is_directed() else "undirected"
        print(f"{name}: {kind}, {graph.number_of_nodes()} nodes, {graph.number_of_edges()} edges -> {out.relative_to(root.parent)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
