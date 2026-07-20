#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = ["networkx==3.4.2"]
# ///
"""Generate the committed LFR accuracy-benchmark suite for meso.

One-time, out-of-band generator (never in CI). It writes a fixed, versioned set
of LFR benchmark graphs with planted ground truth, plus a manifest recording the
parameters and realized mixing of each graph. The scoring side (NMI/AMI/ARI of
meso's partition against the planted ground truth) loads these committed graphs;
it does not run this generator.

The suite pins networkx's LFR generator by version and derives every random seed
deterministically from the grid indices, so re-running reproduces the same suite
byte for byte. Regenerate only when the grid or the generator version changes.

Reproduce, from the repository root, with uv installed:

    uv run datasets/lfr/generate.py

Design of record: docs/specs/001-initial-implementation/meso-oracle.md (accuracy sweep).
"""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any, cast

import networkx as nx
from networkx.generators.community import LFR_benchmark_graph  # pyright: ignore[reportUnknownVariableType]

# Fixed grid. networkx's average_degree rejection sampler is unreliable at
# tau1=2, so the degree floor is set with min_degree (the documented reliable
# path); the realized average degree is recorded per graph in the manifest.
N = 1000
TAU1 = 2.0  # degree exponent
TAU2 = 1.5  # community-size exponent (networkx requires > 1, so not the classic 1)
MIN_DEGREE = 10
MAX_DEGREE = 50
MU_GRID = [0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7]
REALIZATIONS = 5
REGIMES = {
    # regime: (min_community, max_community)
    "S": (10, 50),   # small communities: the harder recovery regime
    "B": (20, 100),  # big communities
}
MAX_ATTEMPTS = 30  # seed retries per graph (LFR sampling can miss on a seed)
MAX_ITERS = 5000

OUT_ROOT = Path(__file__).resolve().parent


def base_seed(regime_idx: int, mu_idx: int, realiz: int) -> int:
    """Deterministic, collision-free seed base per grid cell.

    Reserves a gap of MAX_ATTEMPTS per cell so retry seeds never overlap
    another cell's seeds, keeping the whole suite reproducible.
    """
    cell = (regime_idx * len(MU_GRID) + mu_idx) * REALIZATIONS + realiz
    return cell * MAX_ATTEMPTS + 1


def generate_graph(
    regime: str, mu: float, regime_idx: int, mu_idx: int, realiz: int
) -> tuple[Any, int]:
    minc, maxc = REGIMES[regime]
    start = base_seed(regime_idx, mu_idx, realiz)
    for attempt in range(MAX_ATTEMPTS):
        seed = start + attempt
        try:
            # networkx's Graph is ambiguously typed across pyright/Pylance
            # versions and its generators erase return types, so treat the graph
            # as Any at this library boundary.
            g: Any = cast("Any", LFR_benchmark_graph(
                n=N, tau1=TAU1, tau2=TAU2, mu=mu,
                min_degree=MIN_DEGREE, max_degree=MAX_DEGREE,
                min_community=minc, max_community=maxc,
                max_iters=MAX_ITERS, seed=seed,
            ))
            # networkx's LFR emits self-loops; strip them so the committed graph
            # is a simple undirected graph. Parallel edges are already collapsed
            # (LFR_benchmark_graph returns a Graph, not a MultiGraph).
            g.remove_edges_from([(v, v) for v in g if g.has_edge(v, v)])
            return g, seed
        except nx.ExceededMaxIterations:
            continue
    raise SystemExit(f"could not generate LFR graph for regime={regime} mu={mu} r={realiz}")


def community_labels(g: Any) -> list[int]:
    """Map each node to a dense 0-based community id from LFR's node attribute."""
    comms = sorted({frozenset(g.nodes[v]["community"]) for v in g}, key=lambda c: min(c))
    label = {c: i for i, c in enumerate(comms)}
    node_comm: dict[int, int] = {}
    for c, i in label.items():
        for v in c:
            node_comm[v] = i
    return [node_comm[v] for v in range(int(g.number_of_nodes()))]


def realized_mu(g: Any, labels: list[int]) -> float:
    """Mean per-node fraction of edges leaving the node's community (LFR's mu)."""
    total = 0.0
    for v in g:
        deg = int(g.degree(v))
        if deg == 0:
            continue
        ext = sum(1 for u in g.neighbors(v) if labels[u] != labels[v])
        total += ext / deg
    return total / int(g.number_of_nodes())


def write_graph(regime: str, mu: float, realiz: int, g: Any, labels: list[int]) -> dict[str, Any]:
    tag = f"mu{int(round(mu * 100)):03d}-r{realiz}"
    regime_dir = OUT_ROOT / regime
    regime_dir.mkdir(parents=True, exist_ok=True)
    edges_path = regime_dir / f"{tag}.txt"
    gt_path = regime_dir / f"{tag}-ground-truth.csv"

    # Edge list: 1-based node ids, each undirected edge once (i < j).
    with edges_path.open("w") as f:
        f.write(f"# LFR benchmark graph: regime={regime} mu={mu} realization={realiz}\n")
        f.write("# undirected, unweighted; 1-based node ids; each edge listed once.\n")
        for i, j in sorted((min(a, b) + 1, max(a, b) + 1) for a, b in g.edges()):
            f.write(f"{i} {j}\n")

    # Ground truth: 1-based node id -> planted community id.
    with gt_path.open("w") as f:
        f.write("node,community\n")
        f.write(f"# LFR planted communities: regime={regime} mu={mu} realization={realiz}\n")
        for v in range(g.number_of_nodes()):
            f.write(f"{v + 1},{labels[v]}\n")

    avg_deg = sum(d for _, d in g.degree()) / g.number_of_nodes()
    return {
        "edges_file": str(edges_path.relative_to(OUT_ROOT)),
        "ground_truth_file": str(gt_path.relative_to(OUT_ROOT)),
        "regime": regime,
        "n": g.number_of_nodes(),
        "edges": g.number_of_edges(),
        "num_communities": len(set(labels)),
        "tau1": TAU1,
        "tau2": TAU2,
        "min_degree": MIN_DEGREE,
        "max_degree": MAX_DEGREE,
        "avg_degree": round(avg_deg, 4),
        "nominal_mu": mu,
        "realized_mu": round(realized_mu(g, labels), 4),
        "realization": realiz,
    }


def main() -> int:
    records: list[dict[str, Any]] = []
    for regime_idx, regime in enumerate(REGIMES):
        for mu_idx, mu in enumerate(MU_GRID):
            for realiz in range(REALIZATIONS):
                g, seed = generate_graph(regime, mu, regime_idx, mu_idx, realiz)
                labels = community_labels(g)
                rec = write_graph(regime, mu, realiz, g, labels)
                rec["seed"] = seed
                records.append(rec)
                print(
                    f"{regime} mu={mu} r={realiz}: n={rec['n']} m={rec['edges']} "
                    f"comms={rec['num_communities']} realized_mu={rec['realized_mu']} seed={seed}",
                    file=sys.stderr,
                )

    manifest: dict[str, Any] = {
        "generator": "networkx.LFR_benchmark_graph",
        "generator_version": nx.__version__,
        "grid": {
            "n": N, "tau1": TAU1, "tau2": TAU2,
            "min_degree": MIN_DEGREE, "max_degree": MAX_DEGREE,
            "mu": MU_GRID, "realizations": REALIZATIONS,
            "regimes": {k: {"min_community": v[0], "max_community": v[1]} for k, v in REGIMES.items()},
        },
        "graphs": records,
    }
    manifest_path = OUT_ROOT / "manifest.json"
    with manifest_path.open("w") as f:
        json.dump(manifest, f, indent=2)
        f.write("\n")
    print(f"wrote {len(records)} graphs and {manifest_path.name}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
