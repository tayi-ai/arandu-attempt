package answer_test

import (
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/tayi-ai/arandu-attempt/answer"
)

// sixty is a sixty-digit integer: far past what a float64 holds exactly, so a
// comparison that went through floating point would call it equal to its
// neighbours.
const sixty = "123456789012345678901234567890123456789012345678901234567890"

// rat is the expected value, written the way math/big reads it.
func rat(t *testing.T, s string) *big.Rat {
	t.Helper()
	value, ok := new(big.Rat).SetString(s)
	if !ok {
		t.Fatalf("the test's own expected value %q does not parse", s)
	}
	return value
}

func TestParseAcceptsEveryDocumentedForm(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		raw  string
		want string
	}{
		{"7", "7"},
		{"007", "7"},
		{"0", "0"},
		{"-0", "0"},
		{"+5", "5"},
		{"-5", "-5"},
		{"\u22125", "-5"},
		{"\u2212 5", "-5"},
		{"0.5", "1/2"},
		{".5", "1/2"},
		{"-.5", "-1/2"},
		{"2.50", "5/2"},
		{"00.250", "1/4"},
		{"3/4", "3/4"},
		{"3 / 4", "3/4"},
		{"-3/4", "-3/4"},
		{"1.5/2", "3/4"},
		{`\frac{3}{4}`, "3/4"},
		{`\dfrac{3}{4}`, "3/4"},
		{`\tfrac{3}{4}`, "3/4"},
		{`\frac34`, "3/4"},
		{`\frac 3 4`, "3/4"},
		{`\frac{3}4`, "3/4"},
		{`\frac3{4}`, "3/4"},
		{`\frac{ 3 }{ 4 }`, "3/4"},
		{`\frac{12}{16}`, "3/4"},
		{`-\frac{3}{4}`, "-3/4"},
		{`- \frac{3}{4}`, "-3/4"},
		{`\frac{-3}{4}`, "-3/4"},
		{`\frac{3}{-4}`, "-3/4"},
		{"\\frac{\u22123}{4}", "-3/4"},
		{`-\frac{-3}{4}`, "3/4"},
		{`\frac{1.5}{2}`, "3/4"},
		{`\frac{1,000}{4}`, "250"},
		{"$5", "5"},
		{`\$5`, "5"},
		{`\$ 5`, "5"},
		{`\$18.90`, "189/10"},
		{`-\$5`, "-5"},
		{"$-5", "-5"},
		{"$5$", "5"},
		{`$\frac{3}{4}$`, "3/4"},
		{"$$5$$", "5"},
		{"5.", "5"},
		{"5 .", "5"},
		{`\frac{3}{4}.`, "3/4"},
		{"$5.$", "5"},
		{"1,000", "1000"},
		{"12,345,678", "12345678"},
		{"1,000.5", "2001/2"},
		{"-1,000", "-1000"},
		{"1.000", "1"},
		{"  42 \n", "42"},
		{"\t-3\t", "-3"},
		{`\text{5}`, "5"},
		{`\mathrm{5}`, "5"},
		{`\text{ 5 }`, "5"},
		{`\text {5}`, "5"},
		{`\text{\$5}`, "5"},
		{`\text{\mathrm{5}}`, "5"},
		{`\text{5.}`, "5"},
		{sixty, sixty},
		{"-" + sixty, "-" + sixty},
		{"0." + strings.Repeat("0", 59) + "1", "1/1" + strings.Repeat("0", 60)},
	} {
		got, err := answer.Parse(tc.raw)
		if err != nil {
			t.Errorf("Parse(%q) refused an accepted form: %v", tc.raw, err)
			continue
		}
		if want := rat(t, tc.want); got.Cmp(want) != 0 {
			t.Errorf("Parse(%q) = %s, want %s", tc.raw, got.RatString(), want.RatString())
		}
	}
}

func TestParseRefusesWithATypedError(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		raw  string
		want error
	}{
		// Nothing there.
		{"", answer.ErrEmpty},
		{"   ", answer.ErrEmpty},
		{"\t\n", answer.ErrEmpty},
		{`\text{}`, answer.ErrEmpty},
		{`\text{ }`, answer.ErrEmpty},
		{"$$", answer.ErrEmpty},

		// Not a plain number.
		{`\sqrt{2}`, answer.ErrNotNumeric},
		{`2\sqrt{2}`, answer.ErrNotNumeric},
		{`\frac{1}{\sqrt{2}}`, answer.ErrNotNumeric},
		{`\frac{\sqrt{3}}{2}`, answer.ErrNotNumeric},
		{`\pi`, answer.ErrNotNumeric},
		{`2\pi`, answer.ErrNotNumeric},
		{`\frac{\pi}{2}`, answer.ErrNotNumeric},
		{`\infty`, answer.ErrNotNumeric},
		{`-\infty`, answer.ErrNotNumeric},
		{"x", answer.ErrNotNumeric},
		{"x=5", answer.ErrNotNumeric},
		{"x = 5", answer.ErrNotNumeric},
		{"5x", answer.ErrNotNumeric},
		{"five", answer.ErrNotNumeric},
		{`\text{five}`, answer.ErrNotNumeric},
		{`\text{5 apples}`, answer.ErrNotNumeric},
		{`5\text{ cm}`, answer.ErrNotNumeric},
		{`\textbf{5}`, answer.ErrNotNumeric},
		{"(1, 2)", answer.ErrNotNumeric},
		{"(1,2)", answer.ErrNotNumeric},
		{"(0, 1]", answer.ErrNotNumeric},
		{"[0, 1)", answer.ErrNotNumeric},
		{"(5)", answer.ErrNotNumeric},
		{"2(3)", answer.ErrNotNumeric},
		{`\left(\frac{1}{2}\right)`, answer.ErrNotNumeric},
		{`\left( 3, 4 \right)`, answer.ErrNotNumeric},
		{`\{1, 2\}`, answer.ErrNotNumeric},
		{`\{5`, answer.ErrNotNumeric},
		{`5\}`, answer.ErrNotNumeric},
		{`\boxed{5}`, answer.ErrNotNumeric},
		{"10^3", answer.ErrNotNumeric},
		{"2^{10}", answer.ErrNotNumeric},
		{"x_1", answer.ErrNotNumeric},
		{"0x10", answer.ErrNotNumeric},
		{"1_000", answer.ErrNotNumeric},
		{"\u0663", answer.ErrNotNumeric},       // ARABIC-INDIC DIGIT THREE
		{"\uff15", answer.ErrNotNumeric},       // FULLWIDTH DIGIT FIVE
		{"\u0967\u0968", answer.ErrNotNumeric}, // DEVANAGARI ONE TWO
		{"5\u0663", answer.ErrNotNumeric},
		{"\u00bd", answer.ErrNotNumeric}, // VULGAR FRACTION ONE HALF
		{"5\u00b2", answer.ErrNotNumeric},
		{"--5", answer.ErrNotNumeric},
		{"+-5", answer.ErrNotNumeric},
		{`-\$-5`, answer.ErrNotNumeric},
		{`\$\$5`, answer.ErrNotNumeric},
		{"-", answer.ErrNotNumeric},
		{"$", answer.ErrNotNumeric},
		{".", answer.ErrNotNumeric},
		{"5..", answer.ErrNotNumeric},
		{"5!", answer.ErrNotNumeric},
		{"1/-2", answer.ErrNotNumeric},
		{"1/", answer.ErrNotNumeric},
		{`\frac{}{4}`, answer.ErrNotNumeric},
		{`\frac{3}`, answer.ErrNotNumeric},
		{`\frac`, answer.ErrNotNumeric},
		{`\frac-34`, answer.ErrNotNumeric},
		{`\frac{--3}{4}`, answer.ErrNotNumeric},
		{`\frac{\frac{1}{2}}{3}`, answer.ErrNotNumeric},
		{`\frac{\$1}{2}`, answer.ErrNotNumeric},
		{`\cfrac{1}{2}`, answer.ErrNotNumeric},
		{`{1 \over 2}`, answer.ErrNotNumeric},
		{`\text{5}\text{6}`, answer.ErrNotNumeric},

		// More than one reading.
		{"1,5", answer.ErrAmbiguous},
		{"1,00", answer.ErrAmbiguous},
		{"1,0000", answer.ErrAmbiguous},
		{"1234,567", answer.ErrAmbiguous},
		{"0,500", answer.ErrAmbiguous},
		{"01,000", answer.ErrAmbiguous},
		{"12,34,567", answer.ErrAmbiguous},
		{"1,000,00", answer.ErrAmbiguous},
		{"1.000,5", answer.ErrAmbiguous},
		{"1,", answer.ErrAmbiguous},
		{`10,\!000`, answer.ErrAmbiguous},
		{`1\,000`, answer.ErrAmbiguous},
		{"1 000", answer.ErrAmbiguous},
		{"1\u00a0000", answer.ErrAmbiguous},
		{"3 5", answer.ErrAmbiguous},
		{"3, 5", answer.ErrAmbiguous},
		{"3; 5", answer.ErrAmbiguous},
		{"5\u22123", answer.ErrAmbiguous},
		{"1.2.3", answer.ErrAmbiguous},
		{"1/2/3", answer.ErrAmbiguous},
		{`2\frac{1}{2}`, answer.ErrAmbiguous},
		{`\frac125`, answer.ErrAmbiguous},
		{`\frac{1}{2}3`, answer.ErrAmbiguous},
		{`\frac{1}{2}\frac{1}{2}`, answer.ErrAmbiguous},
		{`\frac{1,5}{2}`, answer.ErrAmbiguous},
		{`1{,}000`, answer.ErrAmbiguous},

		// Deliberately not read in this version.
		{"1e3", answer.ErrUnsupported},
		{"1E-3", answer.ErrUnsupported},
		{"2.5e+10", answer.ErrUnsupported},
		{"1.e3", answer.ErrUnsupported},
		{`1\times10^3`, answer.ErrUnsupported},
		{`1 \times 10^{3}`, answer.ErrUnsupported},
		{`2.5\cdot 10^{-4}`, answer.ErrUnsupported},
		{"1 \u00d7 10^3", answer.ErrUnsupported},
		{"1*10^3", answer.ErrUnsupported},
		{"50%", answer.ErrUnsupported},
		{`50\%`, answer.ErrUnsupported},
		{`50 \%`, answer.ErrUnsupported},
		{"50%.", answer.ErrUnsupported},
		{`\text{50\%}`, answer.ErrUnsupported},
		{"0.5%", answer.ErrUnsupported},

		// No value at all.
		{"1/0", answer.ErrDivisionByZero},
		{"5/0.0", answer.ErrDivisionByZero},
		{"0/0", answer.ErrDivisionByZero},
		{`\frac{1}{0}`, answer.ErrDivisionByZero},
		{`\frac10`, answer.ErrDivisionByZero},
		{`\frac{0}{0}`, answer.ErrDivisionByZero},
		{`\frac{1}{0.00}`, answer.ErrDivisionByZero},
		{`-\frac{1}{-0}`, answer.ErrDivisionByZero},

		// Braces that do not close.
		{`\frac{1}{2`, answer.ErrMalformed},
		{`\frac{1{}{2}`, answer.ErrMalformed},
		{`\text{5`, answer.ErrMalformed},
		{"5}", answer.ErrMalformed},
	} {
		got, err := answer.Parse(tc.raw)
		if err == nil {
			t.Errorf("Parse(%q) = %s, want a refusal wrapping %v", tc.raw, got.RatString(), tc.want)
			continue
		}
		if !errors.Is(err, tc.want) {
			t.Errorf("Parse(%q) refused with %v, want it to wrap %v", tc.raw, err, tc.want)
		}
		if got != nil {
			t.Errorf("Parse(%q) refused and still returned %s", tc.raw, got.RatString())
		}
	}
}

// TestParseRefusalsAreDistinct holds each refusal to one sentinel, so a caller
// counting why items were dropped counts each item once.
func TestParseRefusalsAreDistinct(t *testing.T) {
	t.Parallel()

	sentinels := []error{
		answer.ErrEmpty, answer.ErrNotNumeric, answer.ErrAmbiguous, answer.ErrUnsupported,
		answer.ErrDivisionByZero, answer.ErrMalformed, answer.ErrNoAnswer,
	}
	for _, raw := range []string{"", `\sqrt{2}`, "1,5", "1e3", "50%", "1/0", `\frac{1}{2`} {
		_, err := answer.Parse(raw)
		matched := 0
		for _, sentinel := range sentinels {
			if errors.Is(err, sentinel) {
				matched++
			}
		}
		if matched != 1 {
			t.Errorf("Parse(%q) returned %v, which wraps %d sentinels instead of one", raw, err, matched)
		}
	}
}

// TestParseComparesExactValues is the reason math/big is here. Each pair would
// compare equal through float64 or through a rounded decimal, and is not the
// same number.
func TestParseComparesExactValues(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ a, b string }{
		{"3.1416", "3.14159"},
		{"3.14", `\frac{22}{7}`},
		{"0.333", "1/3"},
		{"0.3333333333333333", `\frac{1}{3}`},
		{"0.1", "0.1000000000000000055511151231257827"},
		{sixty, "123456789012345678901234567890123456789012345678901234567891"},
		{"9007199254740993", "9007199254740992"},
		{"5", "-5"},
	} {
		a, err := answer.Parse(tc.a)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.a, err)
		}
		b, err := answer.Parse(tc.b)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.b, err)
		}
		if a.Cmp(b) == 0 {
			t.Errorf("%q and %q parsed to the same value %s", tc.a, tc.b, a.RatString())
		}
	}
}

// TestParseEqualNotations holds that the notation never changes the value:
// every spelling in a group is the same rational number.
func TestParseEqualNotations(t *testing.T) {
	t.Parallel()

	for _, group := range [][]string{
		{"0.75", `\frac{3}{4}`, "3/4", `\dfrac34`, `\tfrac{6}{8}`, "$0.75$", `\text{0.75}`, "0.750", ".75", "0.75."},
		{"-0", "0", "0.0", "+0", "\u22120", `-\frac{0}{5}`, "0/7"},
		{"2.50", "5/2", `\frac{5}{2}`, "2.5", `\$2.50`},
		{"007", "7", "7.", `\$7`, "7.000", "+7", `\frac{14}{2}`},
		{`-\frac{3}{4}`, `\frac{-3}{4}`, `\frac{3}{-4}`, "-0.75", "-3/4", "\u22120.75", `$-\frac34$`},
		{"1,000", "1000", "1000.0", `\frac{1,000}{1}`},
	} {
		first, err := answer.Parse(group[0])
		if err != nil {
			t.Fatalf("Parse(%q): %v", group[0], err)
		}
		for _, raw := range group[1:] {
			got, err := answer.Parse(raw)
			if err != nil {
				t.Errorf("Parse(%q): %v", raw, err)
				continue
			}
			if got.Cmp(first) != 0 {
				t.Errorf("Parse(%q) = %s, want %s like %q", raw, got.RatString(), first.RatString(), group[0])
			}
		}
	}
}

// TestParseReturnsAFreshValue keeps one parse from aliasing another: a caller
// that negates or scales a result must not change what the next call returns.
func TestParseReturnsAFreshValue(t *testing.T) {
	t.Parallel()

	a, err := answer.Parse("3/4")
	if err != nil {
		t.Fatal(err)
	}
	a.Neg(a)
	b, err := answer.Parse("3/4")
	if err != nil {
		t.Fatal(err)
	}
	if b.Cmp(rat(t, "3/4")) != 0 {
		t.Fatalf("a second Parse returned %s after the first result was negated", b.RatString())
	}
}

// TestParseRefusesDeepWrapping bounds how far wrappers are peeled, so an
// answer cannot make Parse recurse without end.
func TestParseRefusesDeepWrapping(t *testing.T) {
	t.Parallel()

	raw := strings.Repeat(`\text{`, 64) + "5" + strings.Repeat("}", 64)
	if _, err := answer.Parse(raw); !errors.Is(err, answer.ErrNotNumeric) {
		t.Fatalf("Parse of 64 nested \\text wrappers returned %v, want ErrNotNumeric", err)
	}
	shallow := strings.Repeat(`\text{`, 3) + "5" + strings.Repeat("}", 3)
	if _, err := answer.Parse(shallow); err != nil {
		t.Fatalf("Parse(%q): %v", shallow, err)
	}
}
