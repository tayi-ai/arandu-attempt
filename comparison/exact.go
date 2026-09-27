package comparison

import (
	"fmt"
	"math/big"
)

// ExactMcNemar is the exact two-sided McNemar test of a paired comparison: the
// probability, if the candidate were no better and no worse than the base, of
// discordant pairs splitting at least as unevenly as regressed and recovered
// did.
//
// Only the discordant pairs carry information, so with n = regressed +
// recovered the test is the two-sided binomial test of n trials at 1/2:
//
//	p = min(1, 2 · P(X <= min(regressed, recovered))),  X ~ Binomial(n, 1/2)
//
// and p is 1 when n is 0. The tail is summed as integers and divided by 2^n in
// exact arithmetic, rounded to float64 once and then doubled, which is the
// order the historical selection reducer computed it in, so its recorded
// p-values are reproduced bit for bit. Nothing overflows for large n: the
// binomial coefficients are held exactly, and a tail below the smallest
// float64 rounds to zero. The work grows with n · min(regressed, recovered).
//
// The test is symmetric in its arguments. It refuses a negative count.
func ExactMcNemar(regressed, recovered int) (float64, error) {
	return exactTwoSided(regressed, recovered)
}

// SignTest is the exact two-sided sign test: the probability, if positive and
// negative differences were equally likely, of a split at least as uneven as
// positive against negative.
//
// Ties carry no sign and are excluded by the caller before counting. The test
// is the same two-sided binomial test at 1/2 as ExactMcNemar, computed the same
// exact way, and is symmetric in its arguments. It refuses a negative count.
func SignTest(positive, negative int) (float64, error) {
	return exactTwoSided(positive, negative)
}

// exactTwoSided is min(1, 2 · P(X <= min(a, b))) with X ~ Binomial(a+b, 1/2).
func exactTwoSided(a, b int) (float64, error) {
	if a < 0 || b < 0 {
		return 0, fmt.Errorf("%w: %d and %d", ErrNegativeCount, a, b)
	}
	n := a + b
	if n < 0 {
		return 0, fmt.Errorf("%w: %d + %d overflows", ErrNegativeCount, a, b)
	}
	if n == 0 {
		return 1, nil
	}

	k := min(a, b)
	sum := new(big.Int)
	coefficient := big.NewInt(1) // C(n, 0)
	step := new(big.Int)
	for i := 0; i <= k; i++ {
		sum.Add(sum, coefficient)
		// C(n, i+1) = C(n, i) · (n - i) / (i + 1), and the division is exact.
		coefficient.Mul(coefficient, step.SetInt64(int64(n-i)))
		coefficient.Quo(coefficient, step.SetInt64(int64(i+1)))
	}

	denominator := new(big.Int).Lsh(big.NewInt(1), uint(n))
	tail, _ := new(big.Rat).SetFrac(sum, denominator).Float64()
	return min(1, 2*tail), nil
}
