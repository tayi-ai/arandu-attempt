package comparison_test

import (
	"errors"
	"fmt"
	"math/big"
	"math/rand/v2"
	"testing"

	"github.com/tayi-ai/arandu-attempt/comparison"
)

// buildPairs is a comparison with the given number of examples in each
// transition, identified in the order they are built.
func buildPairs(preserved, regressed, recovered, persistent int) []comparison.Pair {
	out := []comparison.Pair{}
	add := func(count int, base, candidate bool) {
		for range count {
			out = append(out, comparison.Pair{
				ID:        fmt.Sprintf("example-%05d", len(out)),
				Base:      base,
				Candidate: candidate,
			})
		}
	}
	add(preserved, true, true)
	add(regressed, true, false)
	add(recovered, false, true)
	add(persistent, false, false)
	return out
}

// shuffled is a copy of pairs in an order fixed by seed.
func shuffled(pairs []comparison.Pair, seed uint64) []comparison.Pair {
	out := append([]comparison.Pair(nil), pairs...)
	r := rand.New(rand.NewPCG(seed, seed))
	r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// TestCountClassifiesEveryPairByItsTransition holds the four cells apart: a
// swap of any two of them changes a count.
func TestCountClassifiesEveryPairByItsTransition(t *testing.T) {
	t.Parallel()

	got, err := comparison.Count(shuffled(buildPairs(4, 3, 2, 1), 7))
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	want := comparison.Transitions{PreservedCorrect: 4, Regressed: 3, Recovered: 2, PersistentError: 1}
	if got != want {
		t.Fatalf("Count = %+v, want %+v", got, want)
	}
	if got.N() != 10 {
		t.Fatalf("N = %d, want 10", got.N())
	}
}

// TestCountRefusesAPairWithNoID keeps an unidentifiable example out of a paired
// statistic.
func TestCountRefusesAPairWithNoID(t *testing.T) {
	t.Parallel()

	pairs := buildPairs(2, 0, 0, 0)
	pairs[1].ID = ""
	if _, err := comparison.Count(pairs); !errors.Is(err, comparison.ErrEmptyID) {
		t.Fatalf("Count error = %v, want ErrEmptyID", err)
	}
}

// TestCountRefusesTheSameExampleTwice keeps one example from weighing twice.
func TestCountRefusesTheSameExampleTwice(t *testing.T) {
	t.Parallel()

	pairs := buildPairs(1, 1, 1, 0)
	pairs[2].ID = pairs[0].ID
	if _, err := comparison.Count(pairs); !errors.Is(err, comparison.ErrDuplicateID) {
		t.Fatalf("Count error = %v, want ErrDuplicateID", err)
	}
}

// TestCountRefusesNoPairs refuses a comparison that covers nothing.
func TestCountRefusesNoPairs(t *testing.T) {
	t.Parallel()

	if _, err := comparison.Count(nil); !errors.Is(err, comparison.ErrEmpty) {
		t.Fatalf("Count error = %v, want ErrEmpty", err)
	}
}

// TestRatesAreTheRecordedFractions reproduces the rates of the two recorded B9
// candidates from their transition counts: NVG is the accuracy difference the
// audit recorded, 0.244140625 and 0.29296875 percentage points.
func TestRatesAreTheRecordedFractions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		transitions     comparison.Transitions
		vgr, rr, nvg    *big.Rat
		deltaPercent    float64
		wantVGR, wantRR float64
	}{
		{
			name:         "r4-generalization",
			transitions:  comparison.Transitions{PreservedCorrect: 1596, Regressed: 6, Recovered: 11, PersistentError: 435},
			vgr:          big.NewRat(11, 2048),
			rr:           big.NewRat(6, 2048),
			nvg:          big.NewRat(5, 2048),
			deltaPercent: 0.244140625,
			wantVGR:      11.0 / 2048,
			wantRR:       6.0 / 2048,
		},
		{
			name:         "r8-generalization",
			transitions:  comparison.Transitions{PreservedCorrect: 1594, Regressed: 8, Recovered: 14, PersistentError: 432},
			vgr:          big.NewRat(14, 2048),
			rr:           big.NewRat(8, 2048),
			nvg:          big.NewRat(6, 2048),
			deltaPercent: 0.29296875,
			wantVGR:      14.0 / 2048,
			wantRR:       8.0 / 2048,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			r, err := c.transitions.Rates()
			if err != nil {
				t.Fatalf("Rates: %v", err)
			}
			if r.ExactVGR().Cmp(c.vgr) != 0 || r.ExactRR().Cmp(c.rr) != 0 || r.ExactNVG().Cmp(c.nvg) != 0 {
				t.Fatalf("exact rates = %s, %s, %s, want %s, %s, %s",
					r.ExactVGR().RatString(), r.ExactRR().RatString(), r.ExactNVG().RatString(),
					c.vgr.RatString(), c.rr.RatString(), c.nvg.RatString())
			}
			if r.VGR != c.wantVGR || r.RR != c.wantRR {
				t.Fatalf("VGR, RR = %v, %v, want %v, %v", r.VGR, r.RR, c.wantVGR, c.wantRR)
			}
			if 100*r.NVG != c.deltaPercent {
				t.Fatalf("100 * NVG = %v, want the recorded %v", 100*r.NVG, c.deltaPercent)
			}
		})
	}
}

// TestNVGIsRoundedOnceFromTheExactDifference holds NVG to the exact fraction:
// 3/10 - 1/10 subtracted in floating point is 0.19999999999999998, and the net
// gain of three recoveries and one regression in ten examples is 0.2.
func TestNVGIsRoundedOnceFromTheExactDifference(t *testing.T) {
	t.Parallel()

	r, err := comparison.Transitions{PreservedCorrect: 3, Regressed: 1, Recovered: 3, PersistentError: 3}.Rates()
	if err != nil {
		t.Fatalf("Rates: %v", err)
	}
	if r.NVG != 0.2 {
		t.Fatalf("NVG = %v, want 0.2 (VGR - RR in floats is %v)", r.NVG, r.VGR-r.RR)
	}
}

// TestExactRatesAreOwnedByTheCaller keeps a report that formats a fraction in
// place from changing the next one it reads.
func TestExactRatesAreOwnedByTheCaller(t *testing.T) {
	t.Parallel()

	r, err := comparison.Transitions{PreservedCorrect: 1, Regressed: 1, Recovered: 1, PersistentError: 1}.Rates()
	if err != nil {
		t.Fatalf("Rates: %v", err)
	}
	r.ExactVGR().SetInt64(99)
	r.ExactRR().SetInt64(99)
	r.ExactNVG().SetInt64(99)
	if r.ExactVGR().Cmp(big.NewRat(1, 4)) != 0 || r.ExactRR().Cmp(big.NewRat(1, 4)) != 0 || r.ExactNVG().Sign() != 0 {
		t.Fatalf("exact rates changed after the caller mutated a returned value: %s, %s, %s",
			r.ExactVGR().RatString(), r.ExactRR().RatString(), r.ExactNVG().RatString())
	}
}

// TestRatesRefuseWhatCannotBeARate refuses a comparison of no example, whose
// rates are undefined, and a negative count from a hand-filled Transitions.
func TestRatesRefuseWhatCannotBeARate(t *testing.T) {
	t.Parallel()

	if _, err := (comparison.Transitions{}).Rates(); !errors.Is(err, comparison.ErrEmpty) {
		t.Fatalf("Rates of no example: error = %v, want ErrEmpty", err)
	}
	negative := comparison.Transitions{PreservedCorrect: 5, Regressed: -1, Recovered: 1}
	if _, err := negative.Rates(); !errors.Is(err, comparison.ErrNegativeCount) {
		t.Fatalf("Rates of a negative count: error = %v, want ErrNegativeCount", err)
	}
}

// TestTheZeroRatesReportZero keeps the documented zero value from dividing by
// zero.
func TestTheZeroRatesReportZero(t *testing.T) {
	t.Parallel()

	var r comparison.Rates
	if r.ExactVGR().Sign() != 0 || r.ExactRR().Sign() != 0 || r.ExactNVG().Sign() != 0 {
		t.Fatal("the zero Rates reports a nonzero exact rate")
	}
}
