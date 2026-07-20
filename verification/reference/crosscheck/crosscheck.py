#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = [
#     "python-igraph==0.11.8",
#     "leidenalg==0.10.2",
# ]
# ///
"""One-time spec-blessing cross-check for the meso Lean value-oracle.

Confirms that the Lean *formalization* of the quality functions matches the
reference implementations' definitions, on the committed fixtures. This is the
one thing a proof cannot check for itself: that the specification agrees with the
world's, not just that the code agrees with the specification.

It reads each committed input fixture and its Lean golden vector, recomputes the
quality of the *same partition* with the references, and compares:

- Modularity: igraph's ``Graph.modularity`` with a resolution parameter matches
  meso's modularity directly (both are Newman modularity; the fixtures have no
  self-loops, where conventions could otherwise diverge).

- CPM: the oracle emits ``cpmCanonicalQ``, meso's CPM restricted to distinct
  pairs ``i != j`` (the self-pair diagonal excluded), which is leidenalg's
  convention. It is compared directly against ``CPMVertexPartition.quality()``.
  meso's internal proof-side CPM (``cpmQ``, summed over all ordered pairs
  including the diagonal) differs from this only by that diagonal, a
  partition-independent quantity that is ``-gamma * N`` for a graph with unit node
  sizes and no self-loops; the identity is machine-checked in Lean as
  ``cpmCanonicalQ_eq``. The two share every optimum and guarantee.

- Connectivity: the golden ``predicates.connected`` flag is corroborated
  against igraph. For each community, the induced subgraph on its nodes must be
  connected (``Graph.induced_subgraph(...).is_connected()``); the flag is the AND over
  communities. The other guarantee predicates (``gammaDense``, ``gammaSeparated``,
  ``subsetOptimal``) are meso-specific CPM guarantees the references do not compute, so
  they are emitted for the Go guarantee tests but not cross-checked here.
"""

from __future__ import annotations

import json
import sys
from collections import defaultdict
from fractions import Fraction
from pathlib import Path
from typing import Any, NamedTuple, cast

import igraph as ig  # pyright: ignore[reportMissingTypeStubs]
import leidenalg as la  # pyright: ignore[reportMissingTypeStubs]

TOL = 1e-9


class Row(NamedTuple):
    """One compared case: the Lean golden value against the reference value."""

    fixture: str
    quality: str
    gamma: float
    lean: float
    ref: float
    rule: str
    ok: bool


def parse_rat(x: object) -> Fraction:
    """Parse an integer or a ``"p"`` / ``"p/q"`` string as an exact Fraction."""
    if isinstance(x, int):
        return Fraction(x)
    return Fraction(str(x))


def lean_value(v: dict[str, Any]) -> Fraction:
    """The Lean golden value as an exact Fraction from its num/den strings."""
    return Fraction(int(v["num"]), int(v["den"]))


def build_graph(inp: dict[str, Any]) -> ig.Graph:
    n = int(inp["n"])
    edges = [(int(e[0]), int(e[1])) for e in inp["edges"]]
    weights = [float(parse_rat(e[2])) for e in inp["edges"]]
    g = ig.Graph(n=n, edges=edges, directed=False)
    if weights:
        g.es["weight"] = weights
    return g


def all_communities_connected(g: ig.Graph, membership: list[int]) -> bool:
    """Whether every community's induced subgraph is connected (meso's connectivity
    guarantee). A single-node community is connected."""
    comms: dict[int, list[int]] = defaultdict(list)
    for v, c in enumerate(membership):
        comms[int(c)].append(v)
    for verts in comms.values():
        if not bool(g.induced_subgraph(verts).is_connected()):  # pyright: ignore[reportUnknownMemberType, reportUnknownArgumentType]
            return False
    return True


def check_fixture(inp_path: Path, gold_path: Path) -> list[Row]:
    inp = cast("dict[str, Any]", json.loads(inp_path.read_text(encoding="utf-8")))
    gold = cast("dict[str, Any]", json.loads(gold_path.read_text(encoding="utf-8")))
    g = build_graph(inp)
    rows: list[Row] = []
    for case_in, case_out in zip(inp["cases"], gold["cases"]):
        quality = str(case_in["quality"])
        gamma = float(parse_rat(case_in["gamma"]))
        membership = list(case_in["partition"])
        lean = float(lean_value(case_out["value"]))
        preds = case_out.get("predicates")
        if preds is not None and preds.get("connected") is not None:
            lean_conn = 1.0 if bool(preds["connected"]) else 0.0
            ref_conn = 1.0 if all_communities_connected(g, membership) else 0.0
            rows.append(Row(inp_path.stem, "connected", gamma, lean_conn, ref_conn,
                            "connectivity", abs(lean_conn - ref_conn) < TOL))
        if quality == "modularity":
            ref = float(g.modularity(membership, weights="weight", resolution=gamma))  # pyright: ignore[reportUnknownMemberType, reportUnknownArgumentType, reportArgumentType]
        elif quality == "cpm":
            part = la.CPMVertexPartition(
                g,
                initial_membership=membership,
                weights="weight",
                resolution_parameter=gamma,
            )
            ref = float(part.quality())  # pyright: ignore[reportUnknownMemberType, reportUnknownArgumentType]
        else:
            continue
        ok = abs(ref - lean) < TOL
        rows.append(Row(inp_path.stem, quality, gamma, lean, ref, "direct", ok))
    return rows


def main() -> int:
    root = Path(sys.argv[1]) if len(sys.argv) > 1 else Path("/work/oracle")
    inputs = sorted((root / "inputs").glob("*.json"))
    if not inputs:
        print(f"no input fixtures under {root / 'inputs'}", file=sys.stderr)
        return 2

    print(f"igraph {ig.__version__}, leidenalg {la.version}")
    print(f"{'fixture':<10} {'quality':<10} {'gamma':>6} "
          f"{'lean':>12} {'ref':>12} {'rule':<16} {'ok':>4}")
    all_ok = True
    for inp_path in inputs:
        gold_path = root / "golden" / inp_path.name
        if not gold_path.exists():
            print(f"missing golden vector for {inp_path.name}", file=sys.stderr)
            all_ok = False
            continue
        for row in check_fixture(inp_path, gold_path):
            all_ok = all_ok and row.ok
            print(f"{row.fixture:<10} {row.quality:<10} {row.gamma:>6.3g} "
                  f"{row.lean:>12.6f} {row.ref:>12.6f} {row.rule:<16} "
                  f"{'PASS' if row.ok else 'FAIL':>4}")

    print("\nALL PASS" if all_ok else "\nDISCREPANCIES FOUND")
    return 0 if all_ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
