package answer_test

import (
	"strings"
	"testing"

	"github.com/tayi-ai/arandu-attempt/answer"
)

// TestTheVerifierIDIsStable pins the identifier recorded next to every
// verdict. Changing it silently would make two different rule sets look like
// one in the records.
func TestTheVerifierIDIsStable(t *testing.T) {
	t.Parallel()

	if answer.VerifierID != "math.numeric_exact_rational.v1" {
		t.Fatalf("VerifierID = %q", answer.VerifierID)
	}
}

func TestVerifyAcceptsTheSameRationalInAnyNotation(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		response, reference string
		extracted, source   string
	}{
		{"#### 0.75", `\frac{3}{4}`, "0.75", answer.SourceHash},
		{`\boxed{\frac{3}{4}}`, "0.75", `\frac{3}{4}`, answer.SourceBoxed},
		{"so it is 3/4", "0.75", "3/4", answer.SourceLastNumber},
		{"#### -0", "0", "-0", answer.SourceHash},
		{"#### 2.50", "5/2", "2.50", answer.SourceHash},
		{"#### 007", "7", "007", answer.SourceHash},
		{"#### 1,000", "1000", "1,000", answer.SourceHash},
		{"#### $1,000.", "1000", "$1,000.", answer.SourceHash},
		{`\boxed{\$18.90}`, "18.9", `\$18.90`, answer.SourceBoxed},
		{`$\boxed{-\frac{3}{4}}$`, `\frac{-3}{4}`, `-\frac{3}{4}`, answer.SourceBoxed},
		{`\boxed{\dfrac34}`, `\tfrac{3}{4}`, `\dfrac34`, answer.SourceBoxed},
		{`\boxed{\text{12}}`, "12", `\text{12}`, answer.SourceBoxed},
		{"The answer is 5.", "5", "5", answer.SourceLastNumber},
		{"The answer is \u22125.", "-5", "\u22125", answer.SourceLastNumber},
		{"#### " + sixty, sixty, sixty, answer.SourceHash},
	} {
		got := answer.Verify(tc.response, tc.reference)
		if !got.Correct || got.Reason != "" {
			t.Errorf("Verify(%q, %q) = %+v, want correct with no reason", tc.response, tc.reference, got)
		}
		if got.Extracted != tc.extracted || got.Source != tc.source {
			t.Errorf("Verify(%q, %q) extracted {%q %s}, want {%q %s}",
				tc.response, tc.reference, got.Extracted, got.Source, tc.extracted, tc.source)
		}
	}
}

func TestVerifyRejectsADifferentValue(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ response, reference string }{
		{"#### 3.1416", "3.14159"},
		{"#### 3.14", `\frac{22}{7}`},
		{"#### 0.333", "1/3"},
		{"#### 5", "-5"},
		{"#### 72", "27"},
		{`\boxed{\frac{3}{4}}`, `\frac{4}{3}`},
		{"#### " + sixty, "123456789012345678901234567890123456789012345678901234567891"},
		{"The answer is 42. Check: 42 - 40 = 2", "42"},
	} {
		got := answer.Verify(tc.response, tc.reference)
		if got.Correct {
			t.Errorf("Verify(%q, %q) called two different numbers correct", tc.response, tc.reference)
		}
		if !strings.Contains(got.Reason, "differs from reference") {
			t.Errorf("Verify(%q, %q) reason = %q, want it to name the mismatch", tc.response, tc.reference, got.Reason)
		}
		if got.Extracted == "" || got.Source == "" {
			t.Errorf("Verify(%q, %q) did not report what it extracted: %+v", tc.response, tc.reference, got)
		}
	}
}

// TestVerifyIsNeverCorrectWhenEitherSideFailsToParse is the property the
// package exists for. Each response is paired with every reference a lenient
// reader might have taken it to mean, and each broken reference with a
// response that would match its obvious reading. None may come back correct.
func TestVerifyIsNeverCorrectWhenEitherSideFailsToParse(t *testing.T) {
	t.Parallel()

	unreadableResponses := map[string][]string{
		"":                               {"0", "1"},
		"I do not know":                  {"0", "1"},
		`\boxed{}`:                       {"0"},
		`\boxed{}, so 5`:                 {"5"},
		"#### \n5":                       {"5"},
		"#### maybe 5":                   {"5"},
		`\boxed{\sqrt{2}}`:               {"2", "1.4142135623730951"},
		`\boxed{2\sqrt{2}}`:              {"2", "4"},
		`\boxed{\frac{1}{\sqrt{2}}}`:     {"1", "2", "0.7071067811865476"},
		`\boxed{3, 5}`:                   {"3", "5", "35"},
		`\boxed{(3, 5)}`:                 {"3", "5"},
		`\boxed{[0, 1)}`:                 {"0", "1"},
		`\boxed{2\frac{1}{2}}`:           {"2.5", "1", "5/2"},
		"#### 1,5":                       {"1.5", "15", "1", "5"},
		"#### 1,00":                      {"1", "100"},
		"#### 1e3":                       {"1000", "1", "3"},
		`#### 1\times10^3`:               {"1000"},
		"#### 50%":                       {"50", "0.5"},
		`\boxed{50\%}`:                   {"50", "0.5"},
		"#### x":                         {"0"},
		`\boxed{\pi}`:                    {"3.14159", "3"},
		"#### 1/0":                       {"0", "1"},
		`\boxed{5`:                       {"5"},
		`\boxed 5`:                       {"5"},
		"so $2^{10}$":                    {"10", "1024"},
		`$\sqrt{2}$`:                     {"2"},
		`$1\,000$`:                       {"0", "1000", "1"},
		"#### \u0663":                    {"3"},
		"\uff15":                         {"5"},
		`$\left(\frac{1}{2}\right)$`:     {"1/2"},
		"the answer is 25%.":             {"25", "0.25"},
		"#### 3.14159.. roughly":         {"3.14159"},
		"#### 12 apples":                 {"12"},
		`\boxed{x = 5}`:                  {"5"},
		`\boxed{\text{five}}`:            {"5"},
		`\boxed{\frac{\frac{1}{2}}{2}}`:  {"1/4"},
		"#### " + sixty + " " + sixty:    {sixty},
		`\boxed{1\text{ and }2}`:         {"1", "2"},
		`\boxed{\frac{1}{2}\frac{1}{2}}`: {"1/4", "1/2"},
	}
	for response, references := range unreadableResponses {
		for _, reference := range references {
			got := answer.Verify(response, reference)
			if got.Correct {
				t.Errorf("Verify(%q, %q) is correct although the response does not parse", response, reference)
			}
			if !strings.HasPrefix(got.Reason, "response: ") {
				t.Errorf("Verify(%q, %q) reason = %q, want it to blame the response", response, reference, got.Reason)
			}
		}
	}

	unreadableReferences := map[string][]string{
		"":             {"#### 0", `\boxed{}`, ""},
		"5%":           {"#### 5", "#### 0.05", "#### 5%"},
		"1e3":          {"#### 1000", "#### 1e3"},
		"1,5":          {"#### 1.5", "#### 15", "#### 1,5"},
		`\sqrt{25}`:    {"#### 5", `\boxed{\sqrt{25}}`},
		"x":            {"#### 0", `\boxed{x}`},
		"0/0":          {"#### 0", `\boxed{0/0}`},
		`\frac{1}{0}`:  {"#### 0", `\boxed{\frac{1}{0}}`},
		`\boxed{5}`:    {"#### 5", `\boxed{5}`},
		`\frac{1}{2`:   {"#### 0.5", `\boxed{\frac{1}{2}`},
		"#### 5":       {"#### 5"},
		"(1, 2)":       {`\boxed{(1, 2)}`, "#### 2"},
		"\u0663":       {"#### 3", "#### \u0663"},
		`\text{seven}`: {"#### 7", `\boxed{\text{seven}}`},
	}
	for reference, responses := range unreadableReferences {
		for _, response := range responses {
			got := answer.Verify(response, reference)
			if got.Correct {
				t.Errorf("Verify(%q, %q) is correct although the reference does not parse", response, reference)
			}
			if !strings.HasPrefix(got.Reason, "reference: ") {
				t.Errorf("Verify(%q, %q) reason = %q, want it to blame the reference", response, reference, got.Reason)
			}
		}
	}
}

// TestVerifyDoesNotFallBackToComparingText proves there is no string
// equality behind the numeric comparison: identical text that is not a number
// is refused, not matched.
func TestVerifyDoesNotFallBackToComparingText(t *testing.T) {
	t.Parallel()

	for _, same := range []string{`\sqrt{2}`, "x", "1,5", "50%", "1e3", `(1, 2)`, `\pi`, "1/0", "five"} {
		got := answer.Verify(`\boxed{`+same+`}`, same)
		if got.Correct {
			t.Errorf("Verify matched %q against itself as text", same)
		}
		if got.Extracted != same {
			t.Errorf("Verify(\\boxed{%s}) extracted %q", same, got.Extracted)
		}
	}
}

// TestVerifyReportsTheExtractionOfARefusal keeps a refused verdict
// inspectable: what was read, by which rule, and why it was refused.
func TestVerifyReportsTheExtractionOfARefusal(t *testing.T) {
	t.Parallel()

	got := answer.Verify("so $2^{10}$", "1024")
	if got.Correct || got.Extracted != "10" || got.Source != answer.SourceLastNumber ||
		!strings.Contains(got.Reason, "larger expression") {
		t.Fatalf("Verify of an exponent = %+v", got)
	}

	got = answer.Verify(`\boxed{\sqrt{2}}`, "bad reference x")
	if got.Correct || got.Extracted != `\sqrt{2}` || got.Source != answer.SourceBoxed ||
		!strings.HasPrefix(got.Reason, "reference: ") {
		t.Fatalf("Verify against a broken reference = %+v", got)
	}

	got = answer.Verify("nothing numeric", "5")
	if got.Correct || got.Extracted != "" || got.Source != "" || !strings.Contains(got.Reason, "no answer found") {
		t.Fatalf("Verify of a response with no number = %+v", got)
	}
}
