#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.10"
# dependencies = []
# ///
"""Convert a Newman-format GML graph into a meso oracle input fixture.

Pure standard library, no third-party dependencies. Reads the ``.gml`` graph
(and an optional ground-truth CSV of ``node,community`` rows) and writes an
oracle input JSON: the graph plus a handful of scoring cases (anchor partitions
that are exactly checkable, and the ground truth where available). The emitted
file is the committed source of truth the Lean value-oracle reads; this script
documents how it was derived from the datasets/ graphs.

Node ids in the GML may be 0- or 1-based; they are mapped to dense 0-based
indices in first-seen order. Edge weight is the ``weight`` attribute, else
``value`` (Les Mis uses ``value`` for co-occurrence counts), else 1. Each
undirected edge is listed once in GML, matching the oracle's convention.

With ``--directed``, each edge is kept as a single arc ``source -> target`` (never
symmetrised), the emitted input carries ``"directed": true``, and the cases score
``directedModularity``. The GML must declare ``directed 1`` in its header; a graph
without it is rejected, so a directed run cannot silently misread an undirected
graph.

Usage:
    gml_to_input.py <graph.gml> <name> <out.json> [--ground-truth <csv>] [--directed]
"""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

INT_FIELD = re.compile(r"([A-Za-z_]\w*)\s+(-?\d+(?:\.\d+)?)")

Edge = list[int | float]


def block_re(kind: str) -> re.Pattern[str]:
    return re.compile(rf"\b{kind}\b\s*\[")


def blocks(text: str, kind: str) -> list[dict[str, str]]:
    """Extract each top-level ``kind [ ... ]`` block as a dict of its numeric
    fields (string fields like ``label "x"`` are ignored)."""
    out: list[dict[str, str]] = []
    for m in block_re(kind).finditer(text):
        depth, j = 1, m.end()
        while depth:
            if text[j] == "[":
                depth += 1
            elif text[j] == "]":
                depth -= 1
            j += 1
        body = text[m.end() : j - 1]
        out.append({k: v for k, v in INT_FIELD.findall(body)})
    return out


def num(s: str) -> int | float:
    f = float(s)
    return int(f) if f.is_integer() else f


def is_directed(text: str) -> bool:
    """Whether the GML header declares ``directed 1`` (outside any node/edge
    block; the top-level graph attribute)."""
    head = text[: block_re("node").search(text).start()] if block_re("node").search(text) else text
    m = re.search(r"\bdirected\b\s+(\d+)", head)
    return bool(m) and int(m.group(1)) == 1


def parse_gml(path: Path) -> tuple[int, list[Edge], dict[int, int]]:
    text = path.read_text()
    ids = [int(n["id"]) for n in blocks(text, "node")]
    index = {gid: k for k, gid in enumerate(ids)}
    edges: list[Edge] = []
    for e in blocks(text, "edge"):
        i, j = index[int(e["source"])], index[int(e["target"])]
        w = e.get("weight", e.get("value", "1"))
        edges.append([i, j, num(w)])
    return len(ids), edges, index


def ground_truth(path: Path, index: dict[int, int], n: int) -> list[int]:
    part = [0] * n
    for line in path.read_text().splitlines():
        line = line.strip()
        if not line or line.startswith("#") or line.startswith("node"):
            continue
        gid, comm = line.split(",")[:2]
        part[index[int(gid)]] = int(comm)
    return part


def main() -> int:
    args = sys.argv[1:]
    gt_path = None
    if "--ground-truth" in args:
        i = args.index("--ground-truth")
        gt_path = Path(args[i + 1])
        del args[i : i + 2]
    directed = "--directed" in args
    args = [a for a in args if a != "--directed"]
    if len(args) < 3:
        print(__doc__, file=sys.stderr)
        return 2
    gml, name, out = Path(args[0]), args[1], Path(args[2])

    if directed and not is_directed(gml.read_text()):
        print(f"{gml}: --directed given but the GML has no 'directed 1' header", file=sys.stderr)
        return 2

    n, edges, index = parse_gml(gml)

    singletons = list(range(n))
    all_in_one = [0] * n
    if directed:
        # The directed objective is the only one that reads an asymmetric graph;
        # the anchor partitions mirror the undirected set.
        cases: list[dict[str, object]] = [
            {"quality": "directedModularity", "gamma": "1", "partition": all_in_one},
            {"quality": "directedModularity", "gamma": "1", "partition": singletons},
        ]
    else:
        cases = [
            {"quality": "modularity", "gamma": "1", "partition": all_in_one},
            {"quality": "modularity", "gamma": "1", "partition": singletons},
            {"quality": "cpm", "gamma": "1", "partition": all_in_one},
            {"quality": "cpm", "gamma": "1", "partition": singletons},
        ]
    if gt_path is not None:
        gt = ground_truth(gt_path, index, n)
        cases.insert(
            0,
            {
                "quality": "directedModularity" if directed else "modularity",
                "gamma": "1",
                "partition": gt,
                "deltas": [{"node": 0, "target": gt[1]}],
            },
        )

    doc: dict[str, object] = {"_source": name, "n": n}
    if directed:
        doc["directed"] = True
    doc["edges"] = edges
    doc["cases"] = cases
    out.write_text(json.dumps(doc, indent=2) + "\n")
    print(f"{name}: n={n} edges={len(edges)} cases={len(cases)} directed={directed} -> {out}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
