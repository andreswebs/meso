/-
Copyright (c) 2026 Andre Silva. All rights reserved.
Released under the GNU General Public License v3.0 or later.
See the LICENSE file in the meso repository root.
-/
import Meso.OracleIO

/-!
# `mesoOracle`: the value-oracle executable

Reads a JSON input file of graphs and scoring cases, evaluates the computable
quality functions, and writes exact golden vectors. See `Meso.OracleIO` for the
schema.
-/

def main (args : List String) : IO UInt32 := do
  match args with
  | [inPath, outPath] =>
    Meso.Oracle.runFile inPath outPath
    pure 0
  | _ =>
    IO.eprintln "usage: mesoOracle <input.json> <output.json>"
    pure 1
