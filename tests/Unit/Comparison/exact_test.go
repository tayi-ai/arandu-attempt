package comparison_test

import (
	"errors"
	"math"
	"testing"

	"github.com/tayi-ai/arandu-attempt/comparison"
)

// TestExactMcNemarReproducesTheRecordedPValues compares with == the raw
// p-values the historical reducer recorded for the two B9 candidates, from the
// discordant counts of their paired provenance.
func TestExactMcNemarReproducesTheRecordedPValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                 string
		regressed, recovered int
		want                 float64
	}{
		{"r4-generalization", 6, 11, 0.332305908203125},
		{"r8-generalization", 8, 14, 0.28627872467041016},
	}
	for _, c := range cases {
		got, err := comparison.ExactMcNemar(c.regressed, c.recovered)
		if err != nil {
			t.Fatalf("%s: ExactMcNemar: %v", c.name, err)
		}
		if got != c.want {
			t.Errorf("%s: ExactMcNemar(%d, %d) = %v, want %v", c.name, c.regressed, c.recovered, got, c.want)
		}
	}
}

// TestExactMcNemarIsSymmetric keeps the test two-sided: which way the
// discordant pairs lean does not change how surprising the split is.
func TestExactMcNemarIsSymmetric(t *testing.T) {
	t.Parallel()

	for b := 0; b <= 40; b++ {
		for c := 0; c <= 40; c++ {
			forward, err := comparison.ExactMcNemar(b, c)
			if err != nil {
				t.Fatalf("ExactMcNemar(%d, %d): %v", b, c, err)
			}
			backward, err := comparison.ExactMcNemar(c, b)
			if err != nil {
				t.Fatalf("ExactMcNemar(%d, %d): %v", c, b, err)
			}
			if forward != backward {
				t.Fatalf("ExactMcNemar(%d, %d) = %v and ExactMcNemar(%d, %d) = %v", b, c, forward, c, b, backward)
			}
		}
	}
}

// TestExactMcNemarOfNoDiscordantPairIsOne holds the case with no evidence
// either way, where the binomial has no trial.
func TestExactMcNemarOfNoDiscordantPairIsOne(t *testing.T) {
	t.Parallel()

	got, err := comparison.ExactMcNemar(0, 0)
	if err != nil || got != 1 {
		t.Fatalf("ExactMcNemar(0, 0) = %v, %v, want 1", got, err)
	}
}

// TestExactMcNemarIsCappedAtOne holds an even split, whose doubled tail is
// above one, to probability one.
func TestExactMcNemarIsCappedAtOne(t *testing.T) {
	t.Parallel()

	for _, n := range []int{1, 5, 1000} {
		got, err := comparison.ExactMcNemar(n, n)
		if err != nil || got != 1 {
			t.Fatalf("ExactMcNemar(%d, %d) = %v, %v, want 1", n, n, got, err)
		}
	}
}

// TestExactMcNemarStaysExactForLargeCounts reaches counts whose binomial
// coefficients are far beyond float64 -- C(2000, 1000) is about 10^600 -- and
// a tail far below the smallest normal float64.
func TestExactMcNemarStaysExactForLargeCounts(t *testing.T) {
	t.Parallel()

	previous := -1.0
	for _, pair := range [][2]int{{900, 1100}, {950, 1050}, {990, 1010}} {
		got, err := comparison.ExactMcNemar(pair[0], pair[1])
		if err != nil {
			t.Fatalf("ExactMcNemar(%d, %d): %v", pair[0], pair[1], err)
		}
		if math.IsNaN(got) || math.IsInf(got, 0) || got <= 0 || got >= 1 {
			t.Fatalf("ExactMcNemar(%d, %d) = %v, want a probability strictly between 0 and 1", pair[0], pair[1], got)
		}
		if got <= previous {
			t.Fatalf("ExactMcNemar(%d, %d) = %v, not above %v for a less uneven split", pair[0], pair[1], got, previous)
		}
		previous = got
	}

	// All sixty discordant pairs one way: p = 2 * 2^-60 = 2^-59 exactly.
	got, err := comparison.ExactMcNemar(0, 60)
	if err != nil || got != math.Ldexp(1, -59) {
		t.Fatalf("ExactMcNemar(0, 60) = %v, %v, want 2^-59", got, err)
	}

	// Two thousand one way: the tail 2^-2000 underflows to zero.
	got, err = comparison.ExactMcNemar(2000, 0)
	if err != nil || got != 0 {
		t.Fatalf("ExactMcNemar(2000, 0) = %v, %v, want 0", got, err)
	}
}

// TestExactMcNemarRefusesANegativeCount refuses a count that cannot have been
// counted.
func TestExactMcNemarRefusesANegativeCount(t *testing.T) {
	t.Parallel()

	for _, pair := range [][2]int{{-1, 3}, {3, -1}} {
		if _, err := comparison.ExactMcNemar(pair[0], pair[1]); !errors.Is(err, comparison.ErrNegativeCount) {
			t.Fatalf("ExactMcNemar(%d, %d) error = %v, want ErrNegativeCount", pair[0], pair[1], err)
		}
	}
}

// TestSignTestIsTheExactBinomial holds 16 positive against 5 negative to
// 2 * (C(21,0) + ... + C(21,5)) / 2^21 = 2 * 27896 / 2097152.
func TestSignTestIsTheExactBinomial(t *testing.T) {
	t.Parallel()

	want := 2 * 27896.0 / 2097152
	if want != 0.02660369873046875 {
		t.Fatalf("reference arithmetic changed: %v", want)
	}
	for _, pair := range [][2]int{{16, 5}, {5, 16}} {
		got, err := comparison.SignTest(pair[0], pair[1])
		if err != nil {
			t.Fatalf("SignTest(%d, %d): %v", pair[0], pair[1], err)
		}
		if got != want {
			t.Fatalf("SignTest(%d, %d) = %v, want %v", pair[0], pair[1], got, want)
		}
	}
}

// TestSignTestOfNoSignIsOneAndRefusesANegativeCount holds the two edges of the
// sign test.
func TestSignTestOfNoSignIsOneAndRefusesANegativeCount(t *testing.T) {
	t.Parallel()

	if got, err := comparison.SignTest(0, 0); err != nil || got != 1 {
		t.Fatalf("SignTest(0, 0) = %v, %v, want 1", got, err)
	}
	if _, err := comparison.SignTest(4, -2); !errors.Is(err, comparison.ErrNegativeCount) {
		t.Fatalf("SignTest(4, -2) error = %v, want ErrNegativeCount", err)
	}
}
