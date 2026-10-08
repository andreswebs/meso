package meso

// NumLevels returns the number of levels of the multilevel run that produced r.
// Level NumLevels()-1 is the result itself.
func (r *Result) NumLevels() int { return len(r.levels) }

// Level returns the community label of every node at level i of the run, keyed
// by the caller's keys, with labels dense in [0, number of communities at that
// level); nil when i is outside [0, NumLevels). A level is the base-graph
// partition reached after that level's local moving, and quality never
// decreases from one level to the next (see [Result.LevelQuality]); the last
// level equals [Result.Communities].
//
// Louvain's levels nest: every community at level i+1 is a union of level-i
// communities. Leiden's need not: from the second level on, Leiden aggregates
// the refined sub-communities of the previous level, so a coarser level can
// divide a finer level's community differently. Under [WithIterations] the
// levels are those of the final pass.
func (r *Result) Level(i int) map[string]int {
	if i < 0 || i >= len(r.levels) {
		return nil
	}
	p := r.levels[i]
	labels := make(map[string]int, len(p))
	for n, c := range p {
		labels[r.g.keys[n]] = c
	}
	return labels
}

// LevelQuality returns the quality of level i under the objective the run
// optimised, or 0 when i is outside [0, NumLevels). LevelQuality(NumLevels()-1)
// equals [Result.Quality].
func (r *Result) LevelQuality(i int) float64 {
	if i < 0 || i >= len(r.levelQuality) {
		return 0
	}
	return r.levelQuality[i]
}
