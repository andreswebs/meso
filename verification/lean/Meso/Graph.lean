/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Mathlib

/-!
# Weighted graph model

The finite weighted graph `meso` operates on, modelled over `Fin n`. This mirrors
the CSR core of the Go library at the mathematical level: symmetric, nonnegative
edge weights plus per-node sizes. Node sizes matter for the resolution term and
must be preserved through aggregation (see the plan, section 4.1); they are
carried here from the start so the aggregation model can respect them.

Self-loops are permitted (`weight i i` may be nonzero): aggregation folds a
community's internal edge weight into a self-loop on the aggregate node.
-/

namespace Meso

/-- A finite, undirected, weighted graph on `n` nodes. -/
structure WeightedGraph (n : ℕ) where
  /-- Edge weight between two nodes. -/
  weight : Fin n → Fin n → ℝ
  /-- The graph is undirected. -/
  weight_symm : ∀ i j, weight i j = weight j i
  /-- Weights are nonnegative. -/
  weight_nonneg : ∀ i j, 0 ≤ weight i j
  /-- Node size / weight (preserved through aggregation). -/
  nodeSize : Fin n → ℝ
  /-- Node sizes are nonnegative. -/
  nodeSize_nonneg : ∀ i, 0 ≤ nodeSize i

namespace WeightedGraph

variable {n : ℕ} (G : WeightedGraph n)

/-- Weighted degree of a node: total incident edge weight `k_i = ∑_j w_{ij}`. -/
def degree (i : Fin n) : ℝ := ∑ j, G.weight i j

/-- `2m`: twice the total edge weight of the graph, `∑_i k_i = ∑_{i,j} w_{ij}`. -/
def twoM : ℝ := ∑ i, G.degree i

lemma degree_nonneg (i : Fin n) : 0 ≤ G.degree i :=
  Finset.sum_nonneg fun j _ => G.weight_nonneg i j

lemma twoM_nonneg : 0 ≤ G.twoM :=
  Finset.sum_nonneg fun i _ => G.degree_nonneg i

end WeightedGraph

end Meso
