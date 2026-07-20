/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.Compute
import Meso.DirectedCompute
import Meso.Predicates
import Meso.DirectedPredicates
import Lean.Data.Json

/-!
# The value-oracle executable's IO glue

Reads a committed JSON input file describing graphs, partitions, and quality
parameters, evaluates the computable quality functions from `Meso.Compute`, and
writes exact golden vectors (rationals as `num`/`den`, plus a float `approx` for
review). The `mesoOracle` `lean_exe` (see `Main.lean` and `lakefile.toml`) is a
thin wrapper over `runFile` here.

This is unverified glue by design: the parser and printer are ordinary IO code,
not the oracle. The oracle is the arithmetic in `Meso.Compute`, whose values are
proved equal to the real model (F2). The graph is built through the verified
`WeightedGraphQ.ofRaw`, so a parser bug can produce the wrong graph but never an
ill-formed one, and the emitted number is always a genuine `modularityQ`/`cpmQ`.

Input schema (see `oracle/inputs/*.json`):

```json
{
  "n": 3,
  "edges": [[0, 1, 1], [1, 2, 1], [0, 2, 1]],
  "nodeSizes": [1, 1, 1],
  "cases": [
    { "quality": "modularity", "gamma": "1", "partition": [0, 0, 0],
      "deltas": [{ "node": 0, "target": 1 }] }
  ]
}
```

Weights and node sizes are integers (JSON numbers) or exact `"p/q"` strings;
never floats. Each undirected edge is listed once; a self-loop is `i == j`.

A top-level `"directed": true` marks a directed input: each edge is then a single
arc `from → to` (listed once, never symmetrised), the only accepted `quality` is
`"directedModularity"`, and the graph is built through the verified
`DirectedWeightedGraphQ.ofRaw`. The output envelope echoes `"directed": true`;
undirected outputs are unchanged.

Each output case also carries a `predicates` object with the boolean
guarantee values for its partition: `connected`, `gammaDense`, `gammaSeparated`, and
`subsetOptimal` (the last is `null` above `maxSubsetOptimalN`, since its decision
enumerates community subsets). Every flag is the value of a mirror proved equal to the
corresponding paper predicate (`Meso.Predicates`). A directed case's `predicates`
object is `{connected, gammaSeparated, subsetOptimal}` — no `gammaDense`, which is
CPM-only — from the directed mirrors (`Meso.DirectedPredicates`).
-/

open Lean (Json toJson)

namespace Meso.Oracle

/-- Which quality function a case scores. `directedModularity` is accepted only in
    a directed input (`"directed": true`), and is the only quality accepted there:
    the undirected qualities read a symmetric graph, the directed one an
    asymmetric graph, and the parser enforces the pairing. -/
inductive QKind
  | modularity
  | cpm
  | directedModularity
  deriving Repr, DecidableEq

/-- The canonical name of a quality function, echoed into the output. -/
def QKind.name : QKind → String
  | .modularity => "modularity"
  | .cpm => "cpm"
  | .directedModularity => "directedModularity"

/-- A move-delta request: the exact gain of moving `node` into community `target`. -/
structure DeltaReq where
  node : Nat
  target : Nat

/-- One scoring case against a fixed graph. -/
structure Case where
  quality : QKind
  gamma : Rat
  partition : Array Nat
  deltas : Array DeltaReq

/-- A parsed input file. `directed` marks a directed input (single arcs, directed
    quality, directed graph build). -/
structure Input where
  n : Nat
  edges : Array (Nat × Nat × Rat)
  sizes : Array Rat
  cases : Array Case
  directed : Bool

/-! ## Parsing -/

/-- Parse a rational from either an integer JSON number or a `"p"` / `"p/q"`
    string. Floats are rejected so no precision is ever lost. -/
def jsonToRat (j : Json) : Except String Rat :=
  match j.getInt? with
  | .ok i => .ok (i : Rat)
  | .error _ =>
    match j.getStr? with
    | .ok s =>
      match s.splitOn "/" with
      | [p] =>
        match p.trimAscii.toString.toInt? with
        | some a => .ok (a : Rat)
        | none => .error s!"not an integer: {s}"
      | [p, q] =>
        match p.trimAscii.toString.toInt?, q.trimAscii.toString.toInt? with
        | some a, some b =>
          if b = 0 then .error "zero denominator" else .ok ((a : Rat) / (b : Rat))
        | _, _ => .error s!"not a rational: {s}"
      | _ => .error s!"not a rational: {s}"
    | .error _ => .error "expected an integer or a \"p/q\" string"

/-- Parse one `[i, j, w]` edge triple. -/
def parseEdge (j : Json) : Except String (Nat × Nat × Rat) := do
  let a ← j.getArr?
  match a[0]?, a[1]?, a[2]? with
  | some x, some y, some z => do
    let i ← x.getNat?
    let jj ← y.getNat?
    let w ← jsonToRat z
    pure (i, jj, w)
  | _, _, _ => .error s!"edge must be [i, j, w]"

/-- Parse a quality-function name. -/
def parseQuality : String → Except String QKind
  | "modularity" => .ok .modularity
  | "cpm" => .ok .cpm
  | "directedModularity" => .ok .directedModularity
  | s => .error s!"unknown quality function: {s}"

/-- Parse one move-delta request `{ "node": _, "target": _ }`. -/
def parseDelta (j : Json) : Except String DeltaReq := do
  let node ← (← j.getObjVal? "node").getNat?
  let target ← (← j.getObjVal? "target").getNat?
  pure { node, target }

/-- Parse one scoring case. `deltas` is optional. -/
def parseCase (j : Json) : Except String Case := do
  let quality ← parseQuality (← (← j.getObjVal? "quality").getStr?)
  let gamma ← jsonToRat (← j.getObjVal? "gamma")
  let partition ← (← (← j.getObjVal? "partition").getArr?).mapM Json.getNat?
  let deltas ← match j.getObjVal? "deltas" with
    | .ok d => (← d.getArr?).mapM parseDelta
    | .error _ => pure #[]
  pure { quality, gamma, partition, deltas }

/-- Parse a whole input file. `nodeSizes` is optional (defaults to all ones), and
    so is `directed` (defaults to false). A directed input accepts only
    `directedModularity` cases and an undirected input accepts everything else:
    the quality reads the matching graph shape, so a mismatch is a parse error,
    never a silently mis-scored case. -/
def parseInput (j : Json) : Except String Input := do
  let n ← (← j.getObjVal? "n").getNat?
  let edges ← (← (← j.getObjVal? "edges").getArr?).mapM parseEdge
  let sizes ← match j.getObjVal? "nodeSizes" with
    | .ok s => (← s.getArr?).mapM jsonToRat
    | .error _ => pure #[]
  let directed ← match j.getObjVal? "directed" with
    | .ok d => d.getBool?
    | .error _ => pure false
  let cases ← (← (← j.getObjVal? "cases").getArr?).mapM parseCase
  for c in cases do
    if directed ∧ c.quality ≠ .directedModularity then
      throw s!"directed input requires quality \"directedModularity\", got {c.quality.name}"
    if ¬directed ∧ c.quality = .directedModularity then
      throw "quality \"directedModularity\" requires a directed input"
  pure { n, edges, sizes, cases, directed }

/-! ## Evaluation -/

/-- Build the verified rational graph from parsed edges and sizes. Parallel edges
    on the same ordered pair sum; missing node sizes default to `1`. Symmetry and
    nonnegativity are handled by `ofRaw`, not asserted here. -/
def buildGraph (n : Nat) (edges : Array (Nat × Nat × Rat)) (sizes : Array Rat) :
    WeightedGraphQ n :=
  WeightedGraphQ.ofRaw
    (fun i j => edges.foldl
      (fun acc e => if e.1 = i.val ∧ e.2.1 = j.val then acc + e.2.2 else acc) 0)
    (fun i => (sizes[i.val]?).getD 1)

/-- Build the verified rational directed graph from parsed arcs and sizes: the
    directed sibling of `buildGraph`. The same parallel-arc fold, but through
    `DirectedWeightedGraphQ.ofRaw`, which never symmetrises: an arc `i → j`
    contributes only to `weight i j`, so asymmetry is preserved. -/
def buildDirectedGraph (n : Nat) (edges : Array (Nat × Nat × Rat)) (sizes : Array Rat) :
    DirectedWeightedGraphQ n :=
  DirectedWeightedGraphQ.ofRaw
    (fun i j => edges.foldl
      (fun acc e => if e.1 = i.val ∧ e.2.1 = j.val then acc + e.2.2 else acc) 0)
    (fun i => (sizes[i.val]?).getD 1)

/-- Build a partition function from a label array; out-of-range nodes fall to `0`. -/
def buildPartition (n : Nat) (labels : Array Nat) : Partition n :=
  fun i => (labels[i.val]?).getD 0

/-- Score a partition under the chosen quality function. CPM reports the canonical
    (leidenalg) convention `cpmCanonicalQ`, so meso's emitted CPM matches the
    published literature; it equals the proof-side `cpmQ` minus a
    partition-independent diagonal (`cpmCanonicalQ_eq`). A `directedModularity`
    case can never reach this evaluator (the parser pairs it with the directed
    graph build), so that arm is an error, not a silent value. -/
def evalQuality {n : Nat} (kind : QKind) (G : WeightedGraphQ n) (γ : Rat)
    (p : Partition n) : Except String Rat :=
  match kind with
  | .modularity => .ok (modularityQ G γ p)
  | .cpm => .ok (cpmCanonicalQ G γ p)
  | .directedModularity => .error "directedModularity case reached the undirected evaluator"

/-- The exact move-delta under the chosen quality function. -/
def evalDelta {n : Nat} (kind : QKind) (G : WeightedGraphQ n) (γ : Rat)
    (p : Partition n) (v : Fin n) (target : Nat) : Except String Rat :=
  match kind with
  | .modularity => .ok (moveDeltaModularityQ G γ p v target)
  | .cpm => .ok (moveDeltaCpmQ G γ p v target)
  | .directedModularity => .error "directedModularity case reached the undirected evaluator"

/-- Node-count ceiling for emitting subset-optimality. Its decision enumerates every
    subset of every community (`2^|C|`), so it is emitted only for small fixtures and
    reported as `null` above the bound. Connectivity and the γ-predicates are polynomial
    and always emitted, even at corpus scale. -/
def maxSubsetOptimalN : Nat := 16

/-- The guarantee predicates for a case's partition, each the Bool value
    of a mirror proved equal to the paper predicate: connectivity (`ConnectedCommunitiesFast`,
    the efficient reachable-set decider), γ-density and γ-separation (`GammaDenseCommunitiesQ`
    / `GammaSeparatedCommunitiesQ`), and subset-optimality (`SubsetOptimalQ`, gated by
    `maxSubsetOptimalN`). The γ-predicates read the case's `γ`; they are CPM guarantees and
    are emitted for every case regardless of the scored quality function. -/
def evalPredicates {n : Nat} (G : WeightedGraphQ n) (γ : Rat) (p : Partition n) : Json :=
  Json.mkObj
    [ ("connected", Json.bool (decide (ConnectedCommunitiesFast G p))),
      ("gammaDense", Json.bool (decide (GammaDenseCommunitiesQ G γ p))),
      ("gammaSeparated", Json.bool (decide (GammaSeparatedCommunitiesQ G γ p))),
      ("subsetOptimal",
        if n ≤ maxSubsetOptimalN then Json.bool (decide (SubsetOptimalQ G γ p))
        else Json.null) ]

/-- The directed guarantee predicates for a case's partition, each the Bool value of
    a mirror proved equal to the directed real predicate (`Meso.DirectedPredicates`):
    weak connectivity (`DirectedConnectedCommunitiesFast`), directed γ-separation
    (`DirectedGammaSeparatedCommunitiesQ`), and the directed subset bound
    (`DirectedSubsetOptimalQ`, gated by `maxSubsetOptimalN`, a characterization
    flag per the Phase 3 triage). There is no `gammaDense` key: γ-density is a CPM
    guarantee and directed CPM is unmodelled by design. -/
def evalDirectedPredicates {n : Nat} (G : DirectedWeightedGraphQ n) (γ : Rat)
    (p : Partition n) : Json :=
  Json.mkObj
    [ ("connected", Json.bool (decide (DirectedConnectedCommunitiesFast G p))),
      ("gammaSeparated", Json.bool (decide (DirectedGammaSeparatedCommunitiesQ G γ p))),
      ("subsetOptimal",
        if n ≤ maxSubsetOptimalN then Json.bool (decide (DirectedSubsetOptimalQ G γ p))
        else Json.null) ]

/-! ## Output -/

/-- A float approximation of a rational, for human review only. -/
def ratToFloat (q : Rat) : Float := Float.ofInt q.num / Float.ofInt q.den

/-- Emit an exact rational as `{ num, den, approx }`, num/den as strings so no
    JSON-number precision is ever involved. -/
def ratToJson (q : Rat) : Json :=
  Json.mkObj
    [ ("num", Json.str (toString q.num)),
      ("den", Json.str (toString q.den)),
      ("approx", Json.str (toString (ratToFloat q))) ]

/-- Evaluate one case into its output JSON object. -/
def evalCase (n : Nat) (G : WeightedGraphQ n) (c : Case) : Except String Json := do
  let p := buildPartition n c.partition
  let q ← evalQuality c.quality G c.gamma p
  let deltas ← c.deltas.mapM fun d => do
    if h : d.node < n then
      let val ← evalDelta c.quality G c.gamma p ⟨d.node, h⟩ d.target
      pure <| Json.mkObj
        [ ("node", toJson d.node), ("target", toJson d.target), ("value", ratToJson val) ]
    else .error s!"delta node {d.node} out of range (n = {n})"
  pure <| Json.mkObj
    [ ("quality", Json.str c.quality.name),
      ("gamma", Json.str (toString c.gamma)),
      ("value", ratToJson q),
      ("deltas", Json.arr deltas),
      ("predicates", evalPredicates G c.gamma p) ]

/-- Evaluate one directed case into its output JSON object: the directed sibling of
    `evalCase`. The quality is always `directedModularityQ` and the delta its
    from-scratch difference `moveDeltaDirectedModularityQ`, both proved equal to the
    real directed model (`directedModularityQ_eq`, `moveDeltaDirectedModularityQ_eq`). -/
def evalDirectedCase (n : Nat) (G : DirectedWeightedGraphQ n) (c : Case) :
    Except String Json := do
  if c.quality ≠ .directedModularity then
    .error s!"{c.quality.name} case reached the directed evaluator"
  let p := buildPartition n c.partition
  let q := directedModularityQ G c.gamma p
  let deltas ← c.deltas.mapM fun d => do
    if h : d.node < n then
      let val := moveDeltaDirectedModularityQ G c.gamma p ⟨d.node, h⟩ d.target
      pure <| Json.mkObj
        [ ("node", toJson d.node), ("target", toJson d.target), ("value", ratToJson val) ]
    else .error s!"delta node {d.node} out of range (n = {n})"
  pure <| Json.mkObj
    [ ("quality", Json.str c.quality.name),
      ("gamma", Json.str (toString c.gamma)),
      ("value", ratToJson q),
      ("deltas", Json.arr deltas),
      ("predicates", evalDirectedPredicates G c.gamma p) ]

/-- The provenance block: the generator, the Lean toolchain (read from
    `lean-toolchain` in the working directory), and the input-set identifier. -/
def provenance (inputSet : String) : IO Json := do
  let toolchain ← (do
    try
      let s ← IO.FS.readFile "lean-toolchain"
      pure s.trimAscii.toString
    catch _ => pure "unknown")
  pure <| Json.mkObj
    [ ("generator", Json.str "mesoOracle"),
      ("leanToolchain", Json.str toolchain),
      ("inputSet", Json.str inputSet) ]

/-- Read an input file, evaluate every case, and write the golden vectors. A
    directed input dispatches to the directed graph build and evaluators and its
    envelope carries `"directed": true`; the undirected envelope is unchanged. -/
def runFile (inPath outPath : System.FilePath) : IO Unit := do
  let s ← IO.FS.readFile inPath
  let j ← IO.ofExcept (Json.parse s)
  let input ← IO.ofExcept (parseInput j)
  let caseJsons ← IO.ofExcept <|
    if input.directed then
      let G := buildDirectedGraph input.n input.edges input.sizes
      input.cases.mapM (evalDirectedCase input.n G)
    else
      let G := buildGraph input.n input.edges input.sizes
      input.cases.mapM (evalCase input.n G)
  let prov ← provenance (inPath.fileStem.getD "unknown")
  let out := Json.mkObj <|
    [ ("provenance", prov), ("n", toJson input.n) ]
      ++ (if input.directed then [("directed", Json.bool true)] else [])
      ++ [ ("cases", Json.arr caseJsons) ]
  IO.FS.writeFile outPath (out.pretty ++ "\n")
  IO.println s!"mesoOracle: wrote {outPath}"

end Meso.Oracle
