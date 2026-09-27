package comparison

import (
	"fmt"
	"math"
	"math/big"
	"math/bits"
	"math/rand/v2"
	"sort"
	"strconv"
)

// MinResamples is the fewest resamples a bootstrap accepts. Below it the tail
// of a 95% interval holds a handful of statistics, and the bound read from it
// moves with the seed more than with the data.
const MinResamples = 1000

// BootstrapConfig is how a percentile bootstrap interval is drawn.
//
// The same configuration and the same input always give the same Interval:
// the generator is math/rand/v2's PCG seeded with (Seed, 0), each resampled
// index is drawn from its 64-bit words by one fixed method, and each statistic
// is summed in one fixed order, with no multiplication a compiler could fuse.
//
// That is reproducibility within this package only. The historical Python
// reducer drew its indices from MT19937 through random.Random, so the same
// Seed there names a different stream of indices, and an interval computed
// here for the same data and the same Seed agrees with the historical one in
// distribution and not bit for bit. The percentile rule is the historical one;
// the stream is not. A recorded historical interval is read from its report,
// never recomputed with this package and presented as the same number.
type BootstrapConfig struct {
	// Resamples is how many bootstrap statistics are drawn, at least
	// MinResamples.
	Resamples int
	// Seed selects the generator's stream.
	Seed uint64
	// Confidence is the interval's nominal coverage, strictly between 0 and
	// 1: 0.95 for a 95% interval.
	Confidence float64
}

// Interval is a percentile bootstrap confidence interval, carrying what it was
// drawn with so a report can print it beside the bounds.
type Interval struct {
	// Lower and Upper are the bounds: the sorted bootstrap statistics at
	// LowerIndex and UpperIndex.
	Lower float64
	Upper float64
	// Point is the statistic on the input itself, not resampled.
	Point float64

	// Resamples, Seed and Confidence are the configuration the interval
	// was drawn with.
	Resamples  int
	Seed       uint64
	Confidence float64
	// LowerIndex and UpperIndex are where the bounds were read in the
	// sorted statistics, counted from zero. See PercentileIndices.
	LowerIndex int
	UpperIndex int
}

// PercentileIndices is where a percentile interval's bounds are read in the B
// sorted bootstrap statistics, counted from zero:
//
//	k     = floor(B · (1 - Confidence) / 2)
//	lower = k
//	upper = B - k
//
// For B = 30000 at 0.95 that is 750 and 29250, the indices the historical
// selection reducer read.
//
// Confidence is taken as the shortest decimal that names its float64, and the
// formula is evaluated in exact arithmetic. The float64 written 0.9 is
// slightly more than 9/10, and evaluating the formula on that binary value
// puts the lower bound for B = 30000 at 1499 rather than 1500; read as the
// decimal the caller wrote, it is 1500.
//
// It refuses fewer than MinResamples, a Confidence that is not strictly
// between 0 and 1, and a combination that leaves k at zero, whose upper index
// would fall past the last statistic.
func PercentileIndices(resamples int, confidence float64) (lower, upper int, err error) {
	if resamples < MinResamples {
		return 0, 0, fmt.Errorf("%w: %d resamples, and the minimum is %d", ErrInvalidConfig, resamples, MinResamples)
	}
	if math.IsNaN(confidence) || confidence <= 0 || confidence >= 1 {
		return 0, 0, fmt.Errorf("%w: confidence %v is not strictly between 0 and 1", ErrInvalidConfig, confidence)
	}

	c, ok := new(big.Rat).SetString(strconv.FormatFloat(confidence, 'g', -1, 64))
	if !ok {
		return 0, 0, fmt.Errorf("%w: confidence %v cannot be read as a decimal", ErrInvalidConfig, confidence)
	}
	tail := new(big.Rat).Sub(big.NewRat(1, 1), c)
	tail.Mul(tail, big.NewRat(int64(resamples), 2))
	// tail is positive, so truncating the quotient is the floor.
	k := new(big.Int).Quo(tail.Num(), tail.Denom())
	if k.Sign() == 0 {
		return 0, 0, fmt.Errorf("%w: %d resamples at confidence %v leave no statistic beyond either bound", ErrInvalidConfig, resamples, confidence)
	}

	lower = int(k.Int64())
	return lower, resamples - lower, nil
}

// BootstrapPaired is a percentile bootstrap interval for the candidate's
// accuracy minus the base's.
//
// Each pair contributes candidate minus base, one of -1, 0 and 1, and each
// bootstrap statistic is the mean of len(pairs) of those contributions drawn
// with replacement. Point is the observed difference, which is also the NVG of
// the same pairs. The pairs are sorted by ID ascending, comparing the bytes of
// the string, before anything is drawn, so the interval depends on the set of
// pairs and not on the order the caller collected them in.
//
// It refuses what Count refuses and what PercentileIndices refuses. See
// BootstrapConfig for what the interval does and does not reproduce.
func BootstrapPaired(pairs []Pair, cfg BootstrapConfig) (Interval, error) {
	if err := validatePairs(pairs); err != nil {
		return Interval{}, err
	}

	sorted := append([]Pair(nil), pairs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	differences := make([]float64, len(sorted))
	for i, p := range sorted {
		differences[i] = float64(correct(p.Candidate) - correct(p.Base))
	}
	return bootstrap(differences, cfg)
}

// BootstrapMean is a percentile bootstrap interval for the mean of per-example
// paired differences, such as the candidate's loss minus the base's on each
// example.
//
// Each bootstrap statistic is the mean of len(values) values drawn with
// replacement, and Point is the mean of the values themselves. The values are
// sorted ascending before anything is drawn, so the interval depends on the
// values and not on the order the caller collected them in.
//
// It refuses an empty slice, a NaN or infinite value, a mean that overflows
// float64, and what PercentileIndices refuses. See BootstrapConfig for what
// the interval does and does not reproduce.
func BootstrapMean(values []float64, cfg BootstrapConfig) (Interval, error) {
	if len(values) == 0 {
		return Interval{}, fmt.Errorf("%w: no values", ErrEmpty)
	}
	sorted := make([]float64, len(values))
	for i, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return Interval{}, fmt.Errorf("%w: value %d is %v", ErrNonFinite, i, v)
		}
		sorted[i] = v
	}
	sort.Float64s(sorted)
	return bootstrap(sorted, cfg)
}

// bootstrap draws cfg.Resamples means of values with replacement and reads the
// percentile bounds from them. values is non-empty, finite and already in the
// order the draws index into.
func bootstrap(values []float64, cfg BootstrapConfig) (Interval, error) {
	lower, upper, err := PercentileIndices(cfg.Resamples, cfg.Confidence)
	if err != nil {
		return Interval{}, err
	}

	n := uint64(len(values))
	source := rand.NewPCG(cfg.Seed, 0)
	statistics := make([]float64, cfg.Resamples)
	for b := range statistics {
		sum := 0.0
		for range n {
			sum += values[draw(source, n)]
		}
		statistics[b] = sum / float64(n)
	}
	sort.Float64s(statistics)

	sum := 0.0
	for _, v := range values {
		sum += v
	}

	interval := Interval{
		Lower:      statistics[lower],
		Upper:      statistics[upper],
		Point:      sum / float64(n),
		Resamples:  cfg.Resamples,
		Seed:       cfg.Seed,
		Confidence: cfg.Confidence,
		LowerIndex: lower,
		UpperIndex: upper,
	}
	for _, v := range []float64{interval.Lower, interval.Upper, interval.Point} {
		if math.IsInf(v, 0) || math.IsNaN(v) {
			return Interval{}, fmt.Errorf("%w: the mean overflows float64", ErrNonFinite)
		}
	}
	return interval, nil
}

// draw is an index uniform on [0, n), n > 0, by Lemire's multiply-and-reject
// method over the generator's 64-bit words.
//
// It is written here rather than taken from rand.Rand so the stream of indices
// is fixed by this package, and not by how a Go release maps a word onto a
// range.
func draw(source *rand.PCG, n uint64) uint64 {
	hi, lo := bits.Mul64(source.Uint64(), n)
	if lo < n {
		threshold := -n % n
		for lo < threshold {
			hi, lo = bits.Mul64(source.Uint64(), n)
		}
	}
	return hi
}

// correct is 1 for a correct answer and 0 for a wrong one.
func correct(answered bool) int {
	if answered {
		return 1
	}
	return 0
}
