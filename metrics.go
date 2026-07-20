package meso

import (
	"math"
	"sort"
)

// Partition-comparison metrics score how similar two labelings of the same node
// set are, independent of the detection algorithm. They are the recovery scores
// used to grade a partition against planted ground truth (design section 6.3).
//
// NMI is normalized mutual information; AMI is its chance-adjusted form; ARI is
// the adjusted Rand index. NMI and AMI follow Vinh, Epps, and Bailey (2010),
// "Information Theoretic Measures for Clusterings Comparison", with the
// arithmetic-mean normalization (as in scikit-learn's default). ARI is the
// Hubert-Arabie (1985) adjustment of the Rand index computed from the
// contingency table.
//
// Accuracy is empirical by design (design section 7), so these metrics have no
// Lean original; they are validated against hand-computed values on small
// contingency tables. All three are pure functions of the two label slices,
// summed in a canonical order so repeated evaluation is bit-identical, and are
// symmetric in their arguments up to floating-point rounding.

// contingencyCell is one non-empty entry of the contingency table: count nodes
// carry label row in the first partition and label col in the second.
type contingencyCell struct {
	row, col, count int
}

// contingency is the contingency table of two partitions of the same n nodes:
// the non-empty joint cells in canonical (row, col) order, and the per-label
// totals of each partition. rowSums and colSums are indexed by the dense labels
// produced by compactLabels, so every entry is strictly positive.
type contingency struct {
	cells   []contingencyCell
	rowSums []int
	colSums []int
	n       int
}

// compactLabels renumbers the arbitrary integer labels of p to dense indices
// [0, k) in order of first appearance, returning the renumbered slice and the
// number of distinct labels k. Renumbering makes the metrics invariant to the
// specific label values and lets the per-label totals be plain slices.
func compactLabels(p Partition) (dense []int, k int) {
	index := make(map[int]int, len(p))
	dense = make([]int, len(p))
	for i, c := range p {
		j, ok := index[c]
		if !ok {
			j = len(index)
			index[c] = j
		}
		dense[i] = j
	}
	return dense, len(index)
}

// crossTabulate builds the contingency table of a and b, which must label the
// same node set. It panics if the two partitions have different lengths, since
// comparing labelings of different node sets is a programmer error.
func crossTabulate(a, b Partition) contingency {
	if len(a) != len(b) {
		panic("meso: partition length mismatch in comparison metric")
	}
	da, ra := compactLabels(a)
	db, rb := compactLabels(b)

	joint := make(map[[2]int]int, len(a))
	rowSums := make([]int, ra)
	colSums := make([]int, rb)
	for i := range da {
		joint[[2]int{da[i], db[i]}]++
		rowSums[da[i]]++
		colSums[db[i]]++
	}

	cells := make([]contingencyCell, 0, len(joint))
	for k, count := range joint {
		cells = append(cells, contingencyCell{row: k[0], col: k[1], count: count})
	}
	sort.Slice(cells, func(i, j int) bool {
		if cells[i].row != cells[j].row {
			return cells[i].row < cells[j].row
		}
		return cells[i].col < cells[j].col
	})

	return contingency{cells: cells, rowSums: rowSums, colSums: colSums, n: len(a)}
}

// comb2 returns the number of unordered pairs C(x, 2) = x(x-1)/2.
func comb2(x int) float64 {
	return float64(x) * float64(x-1) / 2
}

// mutualInfo returns the mutual information (in nats) of the two partitions
// summarized by ct, summed over the non-empty cells in canonical order:
//
//	MI = sum_ij (n_ij/n) * ln(n * n_ij / (a_i * b_j))
func mutualInfo(ct contingency) float64 {
	n := float64(ct.n)
	mi := 0.0
	for _, c := range ct.cells {
		nij := float64(c.count)
		mi += (nij / n) * math.Log(n*nij/(float64(ct.rowSums[c.row])*float64(ct.colSums[c.col])))
	}
	return mi
}

// entropy returns the Shannon entropy (in nats) of the label distribution given
// by sizes over n nodes: H = -sum (s/n) * ln(s/n). Every size is strictly
// positive (they come from compacted labels), so there is no log of zero.
func entropy(sizes []int, n int) float64 {
	fn := float64(n)
	h := 0.0
	for _, s := range sizes {
		p := float64(s) / fn
		h -= p * math.Log(p)
	}
	return h
}

// NMI returns the normalized mutual information of partitions a and b: their
// mutual information divided by the arithmetic mean of their entropies. It is 1
// for identical partitions (up to relabeling) and 0 when they share no
// information. Note NMI is not adjusted for chance, so independent partitions
// score above 0 on average; use [AMI] when a chance-corrected score is needed. a
// and b must have the same length; NMI panics otherwise.
//
//	NMI = MI(a, b) / ((H(a) + H(b)) / 2)
//
// When both partitions are a single community (both entropies zero) the value is
// defined as 1, matching the identical-partition case.
func NMI(a, b Partition) float64 {
	ct := crossTabulate(a, b)
	ha := entropy(ct.rowSums, ct.n)
	hb := entropy(ct.colSums, ct.n)
	denom := (ha + hb) / 2
	if denom == 0 {
		return 1.0
	}
	return mutualInfo(ct) / denom
}

// logFactorials returns a table t of length n+1 where t[k] = ln(k!), built by
// prefix sums of ln(k). It lets expectedMI evaluate the hypergeometric weight
// as a difference of log-factorials without overflowing on the factorials
// themselves.
func logFactorials(n int) []float64 {
	t := make([]float64, n+1)
	for k := 2; k <= n; k++ {
		t[k] = t[k-1] + math.Log(float64(k))
	}
	return t
}

// expectedMI returns the expected mutual information (in nats) of two random
// partitions with the same per-label totals as rowSums and colSums, under the
// hypergeometric (fixed-margins permutation) model of Vinh et al. (2010),
// equation 24a. It is the chance baseline that [AMI] subtracts. The i,j,nij
// loops run in ascending order so the sum is canonical.
func expectedMI(rowSums, colSums []int, n int) float64 {
	fn := float64(n)
	logFact := logFactorials(n)
	emi := 0.0
	for _, ai := range rowSums {
		for _, bj := range colSums {
			lo := max(ai+bj-n, 1)
			hi := min(bj, ai)
			for nij := lo; nij <= hi; nij++ {
				fnij := float64(nij)
				term := (fnij / fn) * math.Log(fn*fnij/(float64(ai)*float64(bj)))
				logWeight := logFact[ai] + logFact[bj] + logFact[n-ai] + logFact[n-bj] -
					logFact[n] - logFact[nij] - logFact[ai-nij] - logFact[bj-nij] - logFact[n-ai-bj+nij]
				emi += term * math.Exp(logWeight)
			}
		}
	}
	return emi
}

// AMI returns the adjusted mutual information of partitions a and b: their
// mutual information corrected for chance against the expected value under the
// permutation model, then normalized by the arithmetic mean of their entropies
// (Vinh et al., 2010). It is 1 for identical partitions (up to relabeling),
// about 0 for independent partitions, and can be negative. Unlike [NMI] it is
// safe to compare across different numbers of communities. a and b must have the
// same length; AMI panics otherwise.
//
//	AMI = (MI - E[MI]) / ((H(a) + H(b))/2 - E[MI])
//
// When both partitions are a single community the value is defined as 1. Near a
// degenerate denominator the divisor is clamped to the smallest normal-scale
// epsilon, as in scikit-learn, so the result stays finite.
func AMI(a, b Partition) float64 {
	ct := crossTabulate(a, b)
	if len(ct.rowSums) <= 1 && len(ct.colSums) <= 1 {
		return 1.0
	}

	mi := mutualInfo(ct)
	emi := expectedMI(ct.rowSums, ct.colSums, ct.n)
	normalizer := (entropy(ct.rowSums, ct.n) + entropy(ct.colSums, ct.n)) / 2

	const eps = 2.220446049250313e-16 // math.Nextafter(1, 2) - 1
	denom := normalizer - emi
	if denom < 0 {
		denom = math.Min(denom, -eps)
	} else {
		denom = math.Max(denom, eps)
	}
	return (mi - emi) / denom
}

// ARI returns the adjusted Rand index of partitions a and b, the pair-counting
// agreement corrected for chance (Hubert and Arabie, 1985). It is 1 for
// identical partitions (up to relabeling), about 0 for independent partitions,
// and can be negative when agreement is below chance. a and b must have the same
// length; ARI panics otherwise.
//
//	ARI = (sum_ij C(n_ij,2) - E) / (0.5*(sum_i C(a_i,2) + sum_j C(b_j,2)) - E)
//	E   = sum_i C(a_i,2) * sum_j C(b_j,2) / C(n,2)
//
// where n_ij is the contingency count and a_i, b_j the per-label totals. When
// the denominator is zero (both partitions are trivial in the same way, for
// example all-singletons vs all-singletons) the value is defined as 1.
func ARI(a, b Partition) float64 {
	ct := crossTabulate(a, b)
	if ct.n <= 1 {
		return 1.0
	}

	sumComb := 0.0
	for _, c := range ct.cells {
		sumComb += comb2(c.count)
	}
	sumA := 0.0
	for _, s := range ct.rowSums {
		sumA += comb2(s)
	}
	sumB := 0.0
	for _, s := range ct.colSums {
		sumB += comb2(s)
	}

	expected := sumA * sumB / comb2(ct.n)
	maxIndex := (sumA + sumB) / 2
	if maxIndex == expected {
		return 1.0
	}
	return (sumComb - expected) / (maxIndex - expected)
}
