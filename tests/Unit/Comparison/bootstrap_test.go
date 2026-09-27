package comparison_test

import (
	"errors"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/tayi-ai/arandu-attempt/comparison"
)

// config95 is a 95% interval of the given size and seed.
func config95(resamples int, seed uint64) comparison.BootstrapConfig {
	return comparison.BootstrapConfig{Resamples: resamples, Seed: seed, Confidence: 0.95}
}

// discordant is a comparison of 500 examples with many discordant pairs, so a
// bound moves when the stream of indices behind it moves.
func discordant() []comparison.Pair { return buildPairs(150, 100, 150, 100) }

// losses is a set of distinct per-example differences.
func losses() []float64 {
	r := rand.New(rand.NewPCG(3, 5))
	out := make([]float64, 400)
	for i := range out {
		out[i] = 0.1 + 0.5*r.NormFloat64()
	}
	return out
}

// TestPercentileIndicesMatchTheHistoricalReducer holds the index rule to the
// indices the historical reducer read for 30000 resamples at 95%.
func TestPercentileIndicesMatchTheHistoricalReducer(t *testing.T) {
	t.Parallel()

	cases := []struct {
		resamples    int
		confidence   float64
		lower, upper int
	}{
		{30000, 0.95, 750, 29250},
		{1000, 0.95, 25, 975},
		{30000, 0.99, 150, 29850},
	}
	for _, c := range cases {
		lower, upper, err := comparison.PercentileIndices(c.resamples, c.confidence)
		if err != nil {
			t.Fatalf("PercentileIndices(%d, %v): %v", c.resamples, c.confidence, err)
		}
		if lower != c.lower || upper != c.upper {
			t.Errorf("PercentileIndices(%d, %v) = %d, %d, want %d, %d", c.resamples, c.confidence, lower, upper, c.lower, c.upper)
		}
	}
}

// TestPercentileIndicesReadTheConfidenceAsWritten holds 0.9 to nine tenths:
// evaluated on the binary float64, the lower index of 30000 resamples would be
// 1499.
func TestPercentileIndicesReadTheConfidenceAsWritten(t *testing.T) {
	t.Parallel()

	// A variable, because a constant expression is evaluated exactly.
	confidence := 0.9
	if naive := math.Floor(30000 * (1 - confidence) / 2); naive != 1499 {
		t.Fatalf("the binary evaluation no longer lands off by one (%v); the case below no longer tells the two apart", naive)
	}
	lower, upper, err := comparison.PercentileIndices(30000, 0.9)
	if err != nil || lower != 1500 || upper != 28500 {
		t.Fatalf("PercentileIndices(30000, 0.9) = %d, %d, %v, want 1500, 28500", lower, upper, err)
	}
}

// TestTheBootstrapRefusesAnInvalidConfiguration refuses, through every entry
// point, a configuration whose interval would not mean what it says.
func TestTheBootstrapRefusesAnInvalidConfiguration(t *testing.T) {
	t.Parallel()

	invalid := []comparison.BootstrapConfig{
		{Resamples: 999, Confidence: 0.95},
		{Resamples: 0, Confidence: 0.95},
		{Resamples: -30000, Confidence: 0.95},
		{Resamples: 30000, Confidence: 0},
		{Resamples: 30000, Confidence: 1},
		{Resamples: 30000, Confidence: -0.95},
		{Resamples: 30000, Confidence: 95},
		{Resamples: 30000, Confidence: math.NaN()},
		{Resamples: 30000, Confidence: math.Inf(1)},
		// floor(1000 * 0.0005) is 0: no statistic lies beyond either bound.
		{Resamples: 1000, Confidence: 0.999},
	}
	for _, cfg := range invalid {
		if _, _, err := comparison.PercentileIndices(cfg.Resamples, cfg.Confidence); !errors.Is(err, comparison.ErrInvalidConfig) {
			t.Errorf("PercentileIndices(%+v) error = %v, want ErrInvalidConfig", cfg, err)
		}
		if _, err := comparison.BootstrapPaired(discordant(), cfg); !errors.Is(err, comparison.ErrInvalidConfig) {
			t.Errorf("BootstrapPaired(%+v) error = %v, want ErrInvalidConfig", cfg, err)
		}
		if _, err := comparison.BootstrapMean(losses(), cfg); !errors.Is(err, comparison.ErrInvalidConfig) {
			t.Errorf("BootstrapMean(%+v) error = %v, want ErrInvalidConfig", cfg, err)
		}
	}
}

// TestBootstrapPairedRefusesWhatCountRefuses keeps the bootstrap from
// resampling a set Count would not have counted.
func TestBootstrapPairedRefusesWhatCountRefuses(t *testing.T) {
	t.Parallel()

	cfg := config95(1000, 1)
	if _, err := comparison.BootstrapPaired(nil, cfg); !errors.Is(err, comparison.ErrEmpty) {
		t.Errorf("no pairs: error = %v, want ErrEmpty", err)
	}
	noID := discordant()
	noID[3].ID = ""
	if _, err := comparison.BootstrapPaired(noID, cfg); !errors.Is(err, comparison.ErrEmptyID) {
		t.Errorf("empty id: error = %v, want ErrEmptyID", err)
	}
	twice := discordant()
	twice[10].ID = twice[400].ID
	if _, err := comparison.BootstrapPaired(twice, cfg); !errors.Is(err, comparison.ErrDuplicateID) {
		t.Errorf("duplicate id: error = %v, want ErrDuplicateID", err)
	}
}

// TestBootstrapMeanRefusesWhatHasNoMean refuses no values, a value that is not
// a number, and values whose mean a float64 cannot hold.
func TestBootstrapMeanRefusesWhatHasNoMean(t *testing.T) {
	t.Parallel()

	cfg := config95(1000, 1)
	if _, err := comparison.BootstrapMean(nil, cfg); !errors.Is(err, comparison.ErrEmpty) {
		t.Errorf("no values: error = %v, want ErrEmpty", err)
	}
	for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		values := losses()
		values[17] = bad
		if _, err := comparison.BootstrapMean(values, cfg); !errors.Is(err, comparison.ErrNonFinite) {
			t.Errorf("value %v: error = %v, want ErrNonFinite", bad, err)
		}
	}
	huge := []float64{math.MaxFloat64, math.MaxFloat64, math.MaxFloat64}
	if _, err := comparison.BootstrapMean(huge, cfg); !errors.Is(err, comparison.ErrNonFinite) {
		t.Errorf("overflowing mean: error = %v, want ErrNonFinite", err)
	}
}

// TestTheBootstrapIsDeterministicForASeed holds the same configuration and the
// same input to the same interval, bit for bit.
func TestTheBootstrapIsDeterministicForASeed(t *testing.T) {
	t.Parallel()

	cfg := config95(2000, 113)
	first, err := comparison.BootstrapPaired(discordant(), cfg)
	if err != nil {
		t.Fatalf("BootstrapPaired: %v", err)
	}
	second, err := comparison.BootstrapPaired(discordant(), cfg)
	if err != nil {
		t.Fatalf("BootstrapPaired: %v", err)
	}
	if first != second {
		t.Fatalf("BootstrapPaired drew %+v and then %+v from the same seed", first, second)
	}

	firstMean, err := comparison.BootstrapMean(losses(), cfg)
	if err != nil {
		t.Fatalf("BootstrapMean: %v", err)
	}
	secondMean, err := comparison.BootstrapMean(losses(), cfg)
	if err != nil {
		t.Fatalf("BootstrapMean: %v", err)
	}
	if firstMean != secondMean {
		t.Fatalf("BootstrapMean drew %+v and then %+v from the same seed", firstMean, secondMean)
	}
}

// TestTheSeedSelectsTheStream holds a different seed to a different interval,
// so a seed that stopped reaching the generator is noticed.
func TestTheSeedSelectsTheStream(t *testing.T) {
	t.Parallel()

	one, err := comparison.BootstrapMean(losses(), config95(2000, 1))
	if err != nil {
		t.Fatalf("BootstrapMean: %v", err)
	}
	two, err := comparison.BootstrapMean(losses(), config95(2000, 2))
	if err != nil {
		t.Fatalf("BootstrapMean: %v", err)
	}
	if one.Lower == two.Lower && one.Upper == two.Upper {
		t.Fatalf("seeds 1 and 2 drew the same mean interval [%v, %v]", one.Lower, one.Upper)
	}

	// Paired bounds sit on a grid of 1/500, so two seeds may agree on both
	// bounds by chance; five may not all agree.
	var intervals []comparison.Interval
	for seed := uint64(1); seed <= 5; seed++ {
		in, err := comparison.BootstrapPaired(discordant(), config95(2000, seed))
		if err != nil {
			t.Fatalf("BootstrapPaired: %v", err)
		}
		intervals = append(intervals, in)
	}
	for _, in := range intervals[1:] {
		if in.Lower != intervals[0].Lower || in.Upper != intervals[0].Upper {
			return
		}
	}
	t.Fatalf("seeds 1 to 5 all drew the paired interval [%v, %v]", intervals[0].Lower, intervals[0].Upper)
}

// TestBootstrapPairedDoesNotDependOnInputOrder holds shuffled pairs to the
// interval of the same pairs in ID order.
func TestBootstrapPairedDoesNotDependOnInputOrder(t *testing.T) {
	t.Parallel()

	cfg := config95(2000, 113)
	want, err := comparison.BootstrapPaired(discordant(), cfg)
	if err != nil {
		t.Fatalf("BootstrapPaired: %v", err)
	}
	for seed := uint64(1); seed <= 3; seed++ {
		got, err := comparison.BootstrapPaired(shuffled(discordant(), seed), cfg)
		if err != nil {
			t.Fatalf("BootstrapPaired: %v", err)
		}
		if got != want {
			t.Fatalf("shuffle %d drew %+v, the ID order drew %+v", seed, got, want)
		}
	}
}

// TestBootstrapMeanDoesNotDependOnInputOrder holds shuffled values to the
// interval of the same values in their original order.
func TestBootstrapMeanDoesNotDependOnInputOrder(t *testing.T) {
	t.Parallel()

	cfg := config95(2000, 113)
	want, err := comparison.BootstrapMean(losses(), cfg)
	if err != nil {
		t.Fatalf("BootstrapMean: %v", err)
	}
	values := losses()
	r := rand.New(rand.NewPCG(9, 9))
	r.Shuffle(len(values), func(i, j int) { values[i], values[j] = values[j], values[i] })
	got, err := comparison.BootstrapMean(values, cfg)
	if err != nil {
		t.Fatalf("BootstrapMean: %v", err)
	}
	if got != want {
		t.Fatalf("shuffled values drew %+v, the original order drew %+v", got, want)
	}
}

// TestTheBootstrapOfAConstantHasNoWidth holds an input with nothing to vary to
// an interval that is exactly its point.
func TestTheBootstrapOfAConstantHasNoWidth(t *testing.T) {
	t.Parallel()

	cfg := config95(1000, 113)
	paired := []struct {
		name  string
		pairs []comparison.Pair
		want  float64
	}{
		{"all preserved", buildPairs(40, 0, 0, 0), 0},
		{"all persistent", buildPairs(0, 0, 0, 40), 0},
		{"all recovered", buildPairs(0, 0, 40, 0), 1},
		{"all regressed", buildPairs(0, 40, 0, 0), -1},
	}
	for _, c := range paired {
		in, err := comparison.BootstrapPaired(c.pairs, cfg)
		if err != nil {
			t.Fatalf("%s: BootstrapPaired: %v", c.name, err)
		}
		if in.Lower != c.want || in.Upper != c.want || in.Point != c.want {
			t.Errorf("%s: interval %+v, want every bound at %v", c.name, in, c.want)
		}
	}

	constant := make([]float64, 37)
	for i := range constant {
		constant[i] = 0.1
	}
	in, err := comparison.BootstrapMean(constant, cfg)
	if err != nil {
		t.Fatalf("BootstrapMean: %v", err)
	}
	if in.Lower != in.Point || in.Upper != in.Point {
		t.Fatalf("constant values drew %+v, want Lower == Upper == Point", in)
	}
}

// TestTheIntervalContainsThePoint holds the observed statistic inside an
// interval of nonzero width.
func TestTheIntervalContainsThePoint(t *testing.T) {
	t.Parallel()

	cfg := config95(2000, 113)
	paired, err := comparison.BootstrapPaired(discordant(), cfg)
	if err != nil {
		t.Fatalf("BootstrapPaired: %v", err)
	}
	mean, err := comparison.BootstrapMean(losses(), cfg)
	if err != nil {
		t.Fatalf("BootstrapMean: %v", err)
	}
	for name, in := range map[string]comparison.Interval{"paired": paired, "mean": mean} {
		if !(in.Lower < in.Point && in.Point < in.Upper) {
			t.Errorf("%s interval %+v does not contain its point strictly", name, in)
		}
	}
}

// TestBootstrapPairedPointIsTheNetVerifiedGain ties the paired interval to the
// rates: its point is the NVG of the same pairs.
func TestBootstrapPairedPointIsTheNetVerifiedGain(t *testing.T) {
	t.Parallel()

	pairs := discordant()
	transitions, err := comparison.Count(pairs)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	rates, err := transitions.Rates()
	if err != nil {
		t.Fatalf("Rates: %v", err)
	}
	in, err := comparison.BootstrapPaired(pairs, config95(1000, 1))
	if err != nil {
		t.Fatalf("BootstrapPaired: %v", err)
	}
	if in.Point != rates.NVG {
		t.Fatalf("Point = %v, NVG = %v", in.Point, rates.NVG)
	}
}

// TestThePairedIntervalHasTheWidthOfItsData checks that the resampling draws
// what it claims to: with P(-1) = 0.2, P(0) = 0.5 and P(1) = 0.3 over 500
// examples, the mean difference has standard error sqrt(0.49 / 500), so a 95%
// interval is about 2 * 1.96 * 0.0313 = 0.1227 wide. A bootstrap that drew
// fewer examples, a biased index, or the wrong statistic misses that by far
// more than the 15% allowed here.
func TestThePairedIntervalHasTheWidthOfItsData(t *testing.T) {
	t.Parallel()

	in, err := comparison.BootstrapPaired(discordant(), config95(5000, 113))
	if err != nil {
		t.Fatalf("BootstrapPaired: %v", err)
	}
	expected := 2 * 1.959963984540054 * math.Sqrt(0.49/500)
	width := in.Upper - in.Lower
	if math.Abs(width-expected) > 0.15*expected {
		t.Fatalf("interval %+v is %v wide, want within 15%% of %v", in, width, expected)
	}
	if center := (in.Lower + in.Upper) / 2; math.Abs(center-in.Point) > 0.15*expected {
		t.Fatalf("interval %+v is centred at %v, far from its point %v", in, center, in.Point)
	}
}

// TestTheIntervalRecordsHowItWasDrawn keeps what a report prints beside the
// bounds equal to what drew them.
func TestTheIntervalRecordsHowItWasDrawn(t *testing.T) {
	t.Parallel()

	cfg := comparison.BootstrapConfig{Resamples: 1000, Seed: 42, Confidence: 0.9}
	in, err := comparison.BootstrapMean(losses(), cfg)
	if err != nil {
		t.Fatalf("BootstrapMean: %v", err)
	}
	if in.Resamples != 1000 || in.Seed != 42 || in.Confidence != 0.9 || in.LowerIndex != 50 || in.UpperIndex != 950 {
		t.Fatalf("interval records %+v, want 1000 resamples, seed 42, confidence 0.9, indices 50 and 950", in)
	}
}

// TestTheHistoricalProtocolRunsAtItsRecordedScale draws the two B9
// candidates' comparisons -- 2048 examples each -- with the recorded protocol:
// 30000 resamples, seed 113, 95%. The recorded intervals came from MT19937 and
// these come from another generator over other example identities, so only
// what does not depend on the stream is held: the indices, the point, the grid
// the bounds sit on, and that the interval contains zero, which is what made
// both recorded decisions a rejection.
func TestTheHistoricalProtocolRunsAtItsRecordedScale(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                     string
		preserved, reg, rec, per int
		point                    float64
	}{
		{"r4-generalization", 1596, 6, 11, 435, 5.0 / 2048},
		{"r8-generalization", 1594, 8, 14, 432, 6.0 / 2048},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			pairs := shuffled(buildPairs(c.preserved, c.reg, c.rec, c.per), 113)
			in, err := comparison.BootstrapPaired(pairs, config95(30000, 113))
			if err != nil {
				t.Fatalf("BootstrapPaired: %v", err)
			}
			if in.LowerIndex != 750 || in.UpperIndex != 29250 {
				t.Fatalf("indices %d and %d, want 750 and 29250", in.LowerIndex, in.UpperIndex)
			}
			if in.Point != c.point {
				t.Fatalf("Point = %v, want %v", in.Point, c.point)
			}
			for _, bound := range []float64{in.Lower, in.Upper} {
				if scaled := bound * 2048; scaled != math.Trunc(scaled) {
					t.Fatalf("bound %v is not a multiple of 1/2048", bound)
				}
			}
			if !(in.Lower < 0 && 0 < in.Upper) {
				t.Fatalf("interval [%v, %v] excludes zero", in.Lower, in.Upper)
			}
			t.Logf("interval [%v/2048, %v/2048]", in.Lower*2048, in.Upper*2048)
		})
	}
}

// TestTheStreamIsPinned holds one interval of each kind to the value this
// package drew when it was written. Determinism within one build is held
// above; this is what holds it across builds, so an interval a report recorded
// can be drawn again. A change to the generator, its seeding, the index
// method or the summation order fails here, and is a change to every interval
// recorded before it.
func TestTheStreamIsPinned(t *testing.T) {
	t.Parallel()

	values := make([]float64, 300)
	for i := range values {
		values[i] = float64(i%13)/8 - 0.5
	}
	mean, err := comparison.BootstrapMean(values, config95(1000, 113))
	if err != nil {
		t.Fatalf("BootstrapMean: %v", err)
	}
	paired, err := comparison.BootstrapPaired(discordant(), config95(1000, 113))
	if err != nil {
		t.Fatalf("BootstrapPaired: %v", err)
	}
	if mean.Lower != 0.18916666666666668 || mean.Upper != 0.30041666666666667 || mean.Point != 0.2475 {
		t.Errorf("mean interval [%v, %v] point %v, pinned [0.18916666666666668, 0.30041666666666667] point 0.2475", mean.Lower, mean.Upper, mean.Point)
	}
	if paired.Lower != 0.036 || paired.Upper != 0.162 || paired.Point != 0.1 {
		t.Errorf("paired interval [%v, %v] point %v, pinned [0.036, 0.162] point 0.1", paired.Lower, paired.Upper, paired.Point)
	}
}
