package meso

import "fmt"

// Option configures a Leiden or Louvain run. Options are applied in order over a
// default configuration (modularity at resolution 1, seed 0); a later option
// overrides an earlier one.
type Option func(*runConfig)

// runConfig is the resolved configuration of a run: the quality function to
// optimise, the PRNG seed for refinement randomness, the number of Leiden
// passes, and an optional resolution override.
type runConfig struct {
	quality    QualityFunction
	seed       uint64
	iterations int
	gamma      float64
	gammaSet   bool
	parallel   bool
	workers    int
}

// WithQuality selects the quality function to optimise, such as [Modularity] or
// [CPM]. The default is Modularity(1.0).
func WithQuality(q QualityFunction) Option {
	return func(c *runConfig) { c.quality = q }
}

// WithSeed sets the PRNG seed that drives Leiden's refinement randomness, so a
// run is a pure function of (graph, options, seed). The default seed is 0.
// Louvain is deterministic and ignores the seed.
func WithSeed(seed uint64) Option {
	return func(c *runConfig) { c.seed = seed }
}

// WithIterations sets the number of full Leiden passes to run. Each pass after
// the first restarts from the previous pass's partition with fresh refinement
// randomness derived from the seed, so quality never decreases with more
// passes and runtime grows linearly with the count. The default is 1; a count
// below 1 is rejected. Louvain has no refinement randomness and ignores this
// option.
//
// The reason to set a count above 1: a single Leiden pass can return
// communities that still admit a strictly improving split. Refinement only
// re-divides the communities found within a pass, so a community assembled
// across aggregation levels is never re-examined at node granularity;
// reference implementations exhibit the same per-run behaviour. The Leiden
// paper's subset-optimality guarantee is asymptotic over repeated randomized
// iterations, which is what extra passes supply. More passes reduce, but never
// eliminate, the chance of such partitions.
func WithIterations(iterations int) Option {
	return func(c *runConfig) { c.iterations = iterations }
}

// WithResolution overrides the resolution parameter gamma of the selected quality
// function, without rebuilding it: WithQuality(Modularity(1.0)) together with
// WithResolution(2.0) optimises modularity at gamma 2.0. Higher gamma favours
// smaller communities.
func WithResolution(gamma float64) Option {
	return func(c *runConfig) {
		c.gamma = gamma
		c.gammaSet = true
	}
}

// WithParallelism runs the fast local-move phase in the synchronous-round
// parallel mode (design section 4.5) using the given number of worker
// goroutines; a value <= 0 means one worker per available core
// (runtime.GOMAXPROCS). The output is byte-identical regardless of the worker
// count - the round decides every move from the round-start snapshot and applies
// them in a fixed order - so parallelism buys speed without sacrificing
// reproducibility. Without this option a run is serial.
//
// Synchronous rounds reach a different local-move fixed point than the default
// serial phase, so a parallel run's partition may differ from the serial run's
// on the same input; both are valid community assignments. A run is a pure
// function of (graph, options), and with a fixed seed byte-identical across
// machines and core counts.
func WithParallelism(workers int) Option {
	return func(c *runConfig) {
		c.parallel = true
		c.workers = workers
	}
}

// resolve applies the options over the defaults and returns the internal
// objective to optimise together with the resolved run configuration (seed and
// parallelism). The quality function is always a meso-provided modularity or cpm
// (external types cannot implement QualityFunction, whose method takes the
// unexported csr), so the objective assertion holds; a resolution override
// rebuilds the objective with the new gamma.
func resolve(opts []Option) (objective, runConfig, error) {
	c := runConfig{quality: Modularity(1.0), iterations: 1}
	for _, opt := range opts {
		opt(&c)
	}

	if c.iterations < 1 {
		return nil, c, fmt.Errorf("meso: WithIterations requires at least 1 pass, got %d", c.iterations)
	}
	obj, ok := c.quality.(objective)
	if !ok {
		return nil, c, fmt.Errorf("meso: quality function %T is not optimisable", c.quality)
	}
	if c.gammaSet {
		switch obj.(type) {
		case modularity:
			obj = modularity{gamma: c.gamma}
		case cpm:
			obj = cpm{gamma: c.gamma}
		case directedModularity:
			obj = directedModularity{gamma: c.gamma}
		default:
			return nil, c, fmt.Errorf("meso: WithResolution is not supported for %T", obj)
		}
	}
	return obj, c, nil
}

// checkObjectiveGraph rejects an objective that does not match the graph's
// directedness. A directed graph must be optimised with [DirectedModularity]:
// the undirected modularity and CPM null models use a symmetric degree/size
// product that mis-scores directed arcs, and directed CPM is out of scope by
// design (CPM stays undirected). An undirected graph accepts any objective,
// including DirectedModularity, which reduces to undirected modularity there.
func checkObjectiveGraph(g *Graph, obj objective) error {
	if _, ok := obj.(directedModularity); g.model.directed && !ok {
		return fmt.Errorf("meso: a directed graph requires WithQuality(DirectedModularity(...)); %T is undirected-only", obj)
	}
	return nil
}

// Result is the outcome of a community-detection run: the community each node was
// assigned to, and the achieved quality. Communities are reported in the caller's
// keys via the source Graph's reverse index.
type Result struct {
	g       *Graph
	part    Partition
	quality float64
	members [][]int

	levels       []Partition
	levelQuality []float64
}

// newResult wraps a dense partition of g. The per-label member lists are built
// here, once, so every accessor is a read and a Result is safe to share
// between goroutines.
func newResult(g *Graph, p Partition, quality float64) *Result {
	k := 0
	for _, c := range p {
		k = max(k, c+1)
	}
	members := make([][]int, k)
	for i, c := range p {
		members[c] = append(members[c], i)
	}
	return &Result{g: g, part: p, quality: quality, members: members}
}

// newRunResult wraps the level partitions of a run, coarsest last: the last
// level is the result. Each level keeps its quality under obj, so a level can
// later be reported as a Result of its own.
func newRunResult(g *Graph, obj objective, levels []Partition) *Result {
	qualities := make([]float64, len(levels))
	for i, p := range levels {
		qualities[i] = obj.Quality(g.model, p)
	}
	last := len(levels) - 1
	r := newResult(g, levels[last], qualities[last])
	r.levels = levels
	r.levelQuality = qualities
	return r
}

// Communities returns the community label of every node, keyed by the caller's
// original key. Two keys share a label exactly when their nodes are in the same
// community; labels are dense indices in [0, number of communities).
func (r *Result) Communities() map[string]int {
	comm := make(map[string]int, len(r.part))
	for i, c := range r.part {
		comm[r.g.keys[i]] = c
	}
	return comm
}

// Quality returns the achieved quality of the partition under the objective the
// run optimised.
func (r *Result) Quality() float64 { return r.quality }

// Leiden runs the Leiden algorithm (Traag, Waltman, van Eck, 2019) on g and
// returns the detected communities and achieved quality. It optimises the quality
// function chosen by [WithQuality] (modularity at resolution 1 by default), using
// the seed from [WithSeed] for refinement randomness, so the result is
// byte-identical for a given graph, options, and seed.
//
// A directed graph must be run with [DirectedModularity] (build it with
// [NewDirectedBuilder]); any other quality function is rejected because its null
// model mis-scores directed arcs.
func Leiden(g *Graph, opts ...Option) (*Result, error) {
	obj, cfg, err := resolve(opts)
	if err != nil {
		return nil, err
	}
	if err := checkObjectiveGraph(g, obj); err != nil {
		return nil, err
	}

	mv := serialLeidenMover
	if cfg.parallel {
		mv = parallelMover(cfg.workers)
	}
	return newRunResult(g, obj, leidenIteratedLevels(g.model, obj, cfg.seed, cfg.iterations, mv)), nil
}

// Louvain runs the Louvain algorithm (local moving plus aggregation, no
// refinement) on g and returns the detected communities and achieved quality. It
// is the deterministic baseline Leiden layers refinement on: it optimises the
// [WithQuality] objective (modularity at resolution 1 by default) and, having no
// randomness, ignores [WithSeed].
//
// As with [Leiden], a directed graph must be run with [DirectedModularity]; any
// other quality function is rejected.
func Louvain(g *Graph, opts ...Option) (*Result, error) {
	obj, cfg, err := resolve(opts)
	if err != nil {
		return nil, err
	}
	if err := checkObjectiveGraph(g, obj); err != nil {
		return nil, err
	}

	mv := serialLouvainMover
	if cfg.parallel {
		mv = parallelMover(cfg.workers)
	}
	trace := louvainTraceWith(g.model, obj, mv)
	levels := make([]Partition, len(trace))
	for i, lv := range trace {
		levels[i] = lv.base
	}
	return newRunResult(g, obj, levels), nil
}
