// Package comparison measures a candidate model against its base on the same
// examples, which is the readout that decides whether the candidate may be
// promoted.
//
// Every statistic here is paired: each example is scored once by the base and
// once by the candidate, and what is compared is how the two scores of the
// same example relate. Two accuracies measured on different examples, or on
// the same examples without their identities, cannot be compared this way,
// and nothing in the package accepts them.
//
// The readout has three rates over the N examples of a comparison:
//
//	VGR = recovered / N              the verified gain rate
//	RR  = regressed / N              the regression rate
//	NVG = VGR - RR                   the net verified gain
//
// and two exact tests of whether the discordant examples lean one way, plus
// percentile bootstrap intervals for the accuracy difference and for the mean
// of any per-example paired difference.
//
// The package uses nothing outside the standard library, touches no file and
// starts no goroutine.
package comparison

import (
	"errors"
	"fmt"
	"math/big"
)

// ErrEmpty is returned when there is nothing to measure: no pairs, no values,
// or a Transitions that counts no example.
var ErrEmpty = errors.New("comparison: nothing to compare")

// ErrEmptyID is returned for a pair with no example identity.
var ErrEmptyID = errors.New("comparison: pair has an empty id")

// ErrDuplicateID is returned when two pairs name the same example. The same
// example counted twice weighs twice, and a paired statistic that weighs one
// example twice is measuring a different set than the one it names.
var ErrDuplicateID = errors.New("comparison: pair id appears more than once")

// ErrNegativeCount is returned for a count below zero.
var ErrNegativeCount = errors.New("comparison: count is negative")

// ErrNonFinite is returned for a value that is NaN or infinite.
var ErrNonFinite = errors.New("comparison: value is not finite")

// ErrInvalidConfig is returned for a BootstrapConfig the bootstrap refuses.
var ErrInvalidConfig = errors.New("comparison: invalid bootstrap configuration")

// Pair is one example scored by both models.
//
// ID is the example's identity, and is what makes the pair a pair: it is the
// key the scores were joined on and the order the bootstrap resamples in.
// Base and Candidate say whether each model answered the example correctly.
type Pair struct {
	ID        string
	Base      bool
	Candidate bool
}

// Transitions counts the pairs of a comparison by how the example moved from
// the base to the candidate.
type Transitions struct {
	// PreservedCorrect counts examples both models answered correctly.
	PreservedCorrect int
	// Regressed counts examples the base answered correctly and the
	// candidate did not.
	Regressed int
	// Recovered counts examples the base answered wrongly and the candidate
	// answered correctly. These are the gains of VGR.
	Recovered int
	// PersistentError counts examples neither model answered correctly.
	PersistentError int
}

// N is the number of examples counted.
func (t Transitions) N() int {
	return t.PreservedCorrect + t.Regressed + t.Recovered + t.PersistentError
}

// Count classifies every pair by its transition.
//
// It refuses an empty slice, a pair with an empty ID and two pairs with the
// same ID, since each of those is a comparison of something other than the
// examples it claims to cover.
func Count(pairs []Pair) (Transitions, error) {
	if err := validatePairs(pairs); err != nil {
		return Transitions{}, err
	}

	var t Transitions
	for _, p := range pairs {
		switch {
		case p.Base && p.Candidate:
			t.PreservedCorrect++
		case p.Base:
			t.Regressed++
		case p.Candidate:
			t.Recovered++
		default:
			t.PersistentError++
		}
	}
	return t, nil
}

// validatePairs refuses what Count and BootstrapPaired both refuse.
func validatePairs(pairs []Pair) error {
	if len(pairs) == 0 {
		return fmt.Errorf("%w: no pairs", ErrEmpty)
	}
	seen := make(map[string]struct{}, len(pairs))
	for i, p := range pairs {
		if p.ID == "" {
			return fmt.Errorf("%w: pair %d", ErrEmptyID, i)
		}
		if _, ok := seen[p.ID]; ok {
			return fmt.Errorf("%w: %q", ErrDuplicateID, p.ID)
		}
		seen[p.ID] = struct{}{}
	}
	return nil
}

// Rates is the promotion readout of one comparison.
//
// The float fields are each rounded once from the exact fraction, so NVG is
// the nearest float64 to (recovered - regressed) / N, and can differ in the
// last bit from VGR - RR subtracted in floating point. The Exact accessors
// return the fractions themselves, for a report that prints what was counted
// rather than what a float64 could hold.
//
// The zero value reports zero for every rate.
type Rates struct {
	// VGR is recovered / N, the verified gain rate.
	VGR float64
	// RR is regressed / N, the regression rate.
	RR float64
	// NVG is VGR - RR, the net verified gain. It is also the candidate's
	// accuracy minus the base's.
	NVG float64

	recovered int64
	regressed int64
	examples  int64
}

// Rates computes VGR, RR and NVG.
//
// It refuses a negative count, since Transitions is a plain struct a caller can
// fill from a recorded report, and refuses a Transitions that counts no
// example, whose rates are undefined.
func (t Transitions) Rates() (Rates, error) {
	for _, c := range []int{t.PreservedCorrect, t.Regressed, t.Recovered, t.PersistentError} {
		if c < 0 {
			return Rates{}, fmt.Errorf("%w: %+v", ErrNegativeCount, t)
		}
	}
	n := t.N()
	if n == 0 {
		return Rates{}, fmt.Errorf("%w: the transitions count no example", ErrEmpty)
	}

	r := Rates{
		recovered: int64(t.Recovered),
		regressed: int64(t.Regressed),
		examples:  int64(n),
	}
	r.VGR = ratFloat(r.ExactVGR())
	r.RR = ratFloat(r.ExactRR())
	r.NVG = ratFloat(r.ExactNVG())
	return r, nil
}

// ExactVGR is recovered / N as an exact fraction. The value is a new
// allocation the caller owns.
func (r Rates) ExactVGR() *big.Rat { return r.fraction(r.recovered) }

// ExactRR is regressed / N as an exact fraction. The value is a new allocation
// the caller owns.
func (r Rates) ExactRR() *big.Rat { return r.fraction(r.regressed) }

// ExactNVG is (recovered - regressed) / N as an exact fraction. The value is a
// new allocation the caller owns.
func (r Rates) ExactNVG() *big.Rat { return r.fraction(r.recovered - r.regressed) }

// fraction is numerator / N, and zero when N is zero.
func (r Rates) fraction(numerator int64) *big.Rat {
	if r.examples == 0 {
		return new(big.Rat)
	}
	return big.NewRat(numerator, r.examples)
}

// ratFloat is the float64 nearest to x.
func ratFloat(x *big.Rat) float64 {
	f, _ := x.Float64()
	return f
}
