package answer_test

import (
	"errors"
	"testing"

	"github.com/tayi-ai/arandu-attempt/answer"
)

const gsm8kSolution = "Natalia sold 48/2 = <<48/2=24>>24 clips in May.\n" +
	"Natalia sold 48+24 = <<48+24=72>>72 clips altogether in April and May.\n" +
	"#### 72"

func TestParseGSM8KReferenceReadsTheHashLine(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		solution, raw, value string
	}{
		{gsm8kSolution, "72", "72"},
		{"He earns <<2*3=6>>6 more.\n#### 1,000", "1,000", "1000"},
		{"The balance drops by <<5-8=-3>>3.\n#### -3", "-3", "-3"},
		{"#### 5\nstray\n#### 6\n", "6", "6"},
		{"#### 0.5  ", "0.5", "1/2"},
	} {
		got, err := answer.ParseGSM8KReference(tc.solution)
		if err != nil {
			t.Errorf("ParseGSM8KReference(%q): %v", tc.solution, err)
			continue
		}
		if got.Raw != tc.raw || got.Value.Cmp(rat(t, tc.value)) != 0 {
			t.Errorf("ParseGSM8KReference(%q) = {%q %s}, want {%q %s}", tc.solution, got.Raw, got.Value.RatString(), tc.raw, tc.value)
		}
	}
}

// TestABrokenGSM8KRowIsNotNonNumeric keeps a row with no "####" visible. A
// filter keeping numeric items drops ErrNotNumeric; a missing answer line is a
// broken row, and dropping it with the rest would hide it.
func TestABrokenGSM8KRowIsNotNonNumeric(t *testing.T) {
	t.Parallel()

	for _, solution := range []string{"", "Natalia sold 72 clips altogether.", `\boxed{72}`} {
		_, err := answer.ParseGSM8KReference(solution)
		if !errors.Is(err, answer.ErrNoAnswer) || errors.Is(err, answer.ErrNotNumeric) {
			t.Errorf("ParseGSM8KReference(%q) returned %v, want ErrNoAnswer and not ErrNotNumeric", solution, err)
		}
	}
}

func TestParseGSM8KReferenceRefusesANonNumericAnswer(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		solution string
		cause    error
	}{
		{"#### ", answer.ErrEmpty},
		{"#### 1,5", answer.ErrAmbiguous},
		{"#### 25%", answer.ErrUnsupported},
		{"#### 1e3", answer.ErrUnsupported},
		{"#### 1/0", answer.ErrDivisionByZero},
		{"#### seventy-two", answer.ErrNotNumeric},
		{"#### 72 clips", answer.ErrNotNumeric},
	} {
		_, err := answer.ParseGSM8KReference(tc.solution)
		if !errors.Is(err, answer.ErrNotNumeric) || !errors.Is(err, tc.cause) {
			t.Errorf("ParseGSM8KReference(%q) returned %v, want ErrNotNumeric and %v", tc.solution, err, tc.cause)
		}
	}
}

func TestParseMATHReferenceReadsTheLastBox(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		solution, raw, value string
	}{
		{`Therefore the probability is $\boxed{\frac{3}{4}}$.`, `\frac{3}{4}`, "3/4"},
		{`So $x = \boxed{\dfrac{1}{2}}$.`, `\dfrac{1}{2}`, "1/2"},
		{`The sum is $\boxed{-\frac{5}{6}}$.`, `-\frac{5}{6}`, "-5/6"},
		{`We get $\boxed{12}$.`, "12", "12"},
		{`Rounded, $\boxed{0.25}$.`, "0.25", "1/4"},
		{`It costs $\boxed{\$18.90}$.`, `\$18.90`, "189/10"},
		{`First $\boxed{1}$, and finally $\boxed{2}$.`, "2", "2"},
		{`A case: $\fbox{7}$.`, "7", "7"},
		{`About $\boxed{1,000}$ people.`, "1,000", "1000"},
	} {
		got, err := answer.ParseMATHReference(tc.solution)
		if err != nil {
			t.Errorf("ParseMATHReference(%q): %v", tc.solution, err)
			continue
		}
		if got.Raw != tc.raw || got.Value.Cmp(rat(t, tc.value)) != 0 {
			t.Errorf("ParseMATHReference(%q) = {%q %s}, want {%q %s}", tc.solution, got.Raw, got.Value.RatString(), tc.raw, tc.value)
		}
	}
}

func TestParseMATHReferenceRefusesANonNumericAnswer(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		solution string
		cause    error
	}{
		{`$\boxed{2\sqrt{2}}$`, answer.ErrNotNumeric},
		{`$\boxed{\frac{1}{\sqrt{2}}}$`, answer.ErrNotNumeric},
		{`$\boxed{\pi}$`, answer.ErrNotNumeric},
		{`$\boxed{(1,2)}$`, answer.ErrNotNumeric},
		{`$\boxed{[0, 1)}$`, answer.ErrNotNumeric},
		{`$\boxed{x^2+1}$`, answer.ErrNotNumeric},
		{`$\boxed{\text{(C)}}$`, answer.ErrNotNumeric},
		{`$\boxed{10\%}$`, answer.ErrUnsupported},
		{`$\boxed{1,5}$`, answer.ErrAmbiguous},
		{`$\boxed{3, 5}$`, answer.ErrAmbiguous},
		{`$\boxed{10,\!000}$`, answer.ErrAmbiguous},
		{`$\boxed{}$`, answer.ErrEmpty},
	} {
		_, err := answer.ParseMATHReference(tc.solution)
		if !errors.Is(err, answer.ErrNotNumeric) || !errors.Is(err, tc.cause) {
			t.Errorf("ParseMATHReference(%q) returned %v, want ErrNotNumeric and %v", tc.solution, err, tc.cause)
		}
	}
}

// TestParseMATHReferenceNeverGuesses holds the reference to its box: without
// one there is no answer, even when the solution ends with a number, because
// ground truth that was guessed is not ground truth.
func TestParseMATHReferenceNeverGuesses(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		solution string
		want     error
	}{
		{"The answer is 5.", answer.ErrNoAnswer},
		{"#### 5", answer.ErrNoAnswer},
		{"", answer.ErrNoAnswer},
		{`$\boxed{5$`, answer.ErrMalformed},
		{`$\boxed 5$`, answer.ErrMalformed},
	} {
		got, err := answer.ParseMATHReference(tc.solution)
		if !errors.Is(err, tc.want) || errors.Is(err, answer.ErrNotNumeric) {
			t.Errorf("ParseMATHReference(%q) returned %v, want %v and not ErrNotNumeric", tc.solution, err, tc.want)
		}
		if got.Value != nil {
			t.Errorf("ParseMATHReference(%q) refused and still returned %s", tc.solution, got.Value.RatString())
		}
	}
}

// TestAReferenceFeedsVerify ties the helpers to Verify: the Raw a helper
// returns is a reference Verify reads to the same value.
func TestAReferenceFeedsVerify(t *testing.T) {
	t.Parallel()

	math, err := answer.ParseMATHReference(`Hence $\boxed{\frac{3}{4}}$.`)
	if err != nil {
		t.Fatal(err)
	}
	if got := answer.Verify("the answer is 0.75", math.Raw); !got.Correct {
		t.Errorf("Verify against the MATH reference %q = %+v", math.Raw, got)
	}

	gsm8k, err := answer.ParseGSM8KReference(gsm8kSolution)
	if err != nil {
		t.Fatal(err)
	}
	if got := answer.Verify(`so \boxed{72}`, gsm8k.Raw); !got.Correct {
		t.Errorf("Verify against the GSM8K reference %q = %+v", gsm8k.Raw, got)
	}
	if got := answer.Verify(`so \boxed{27}`, gsm8k.Raw); got.Correct {
		t.Errorf("Verify called 27 correct against %q", gsm8k.Raw)
	}
}
