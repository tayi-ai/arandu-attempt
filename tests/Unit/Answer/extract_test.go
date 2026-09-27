package answer_test

import (
	"errors"
	"testing"

	"github.com/tayi-ai/arandu-attempt/answer"
)

func TestExtractFollowsThePriorityOfTheRules(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		text   string
		raw    string
		source string
	}{
		{"gsm8k line", "She sold 48/2 = <<48/2=24>>24 clips.\n#### 72", "72", answer.SourceHash},
		{"last hash wins", "#### 5 ... #### 6", "6", answer.SourceHash},
		{"last hash on a later line", "#### 5\nthen\n#### 6\n", "6", answer.SourceHash},
		{"hash ends at its line", "#### 72\n\nSo the answer is 80.", "72", answer.SourceHash},
		{"hash ends at a carriage return", "#### 72\r\n80", "72", answer.SourceHash},
		{"hash beats boxed", `\boxed{3} and then #### 4`, "4", answer.SourceHash},
		{"hash beats a later boxed", "#### 4\n\\boxed{3}", "4", answer.SourceHash},
		{"hash text is kept verbatim", "#### $1,000.", "$1,000.", answer.SourceHash},
		{"empty hash does not fall back", "#### \n42", "", answer.SourceHash},
		{"hash words do not fall back", "#### I am not sure, maybe 42", "I am not sure, maybe 42", answer.SourceHash},
		{"last boxed wins", `so \boxed{5} but actually \boxed{6}`, "6", answer.SourceBoxed},
		{"fbox", `\fbox{7}`, "7", answer.SourceBoxed},
		{"fbox after boxed", `\boxed{5} then \fbox{8}`, "8", answer.SourceBoxed},
		{"boxed after fbox", `\fbox{8} then \boxed{5}`, "5", answer.SourceBoxed},
		{"boxed beats the last number", `\boxed{5} and 9 more`, "5", answer.SourceBoxed},
		{"nested braces", `\boxed{\frac{1}{2}}`, `\frac{1}{2}`, answer.SourceBoxed},
		{"deeply nested braces", `$\boxed{\text{\frac{1}{2}}}$.`, `\text{\frac{1}{2}}`, answer.SourceBoxed},
		{"escaped braces do not close", `\boxed{\{1\}} x`, `\{1\}`, answer.SourceBoxed},
		{"escaped closing brace is content", `\boxed{5\}} 7`, `5\}`, answer.SourceBoxed},
		{"escaped backslash before a brace", `\boxed{5\\} 7`, `5\\`, answer.SourceBoxed},
		{"space before the brace", `\boxed {5}`, "5", answer.SourceBoxed},
		{"signed fraction", `$\boxed{-\frac{3}{4}}$.`, `-\frac{3}{4}`, answer.SourceBoxed},
		{"boxed content is trimmed", `\boxed{ 12 }`, "12", answer.SourceBoxed},
		{"empty boxed does not fall back", `\boxed{} and 5`, "", answer.SourceBoxed},
		{"a longer command is not boxed", `\boxedeq{5} is 7`, "7", answer.SourceLastNumber},
		{"trailing period", "The answer is 5.", "5", answer.SourceLastNumber},
		{"last, not first", "I have 3 apples and eat 2", "2", answer.SourceLastNumber},
		{"positional, not semantic", "The answer is 42. Check: 42 - 40 = 2", "2", answer.SourceLastNumber},
		{"thousands", "The total is 1,000,000.", "1,000,000", answer.SourceLastNumber},
		{"trailing comma", "we get 12, which is even", "12", answer.SourceLastNumber},
		{"negative", "It is -7.", "-7", answer.SourceLastNumber},
		{"unicode minus", "It is \u22127.", "\u22127", answer.SourceLastNumber},
		{"binary minus is not a sign", "the range 3-5", "5", answer.SourceLastNumber},
		{"minus after a parenthesis is not a sign", "so (8)-5", "5", answer.SourceLastNumber},
		{"minus after a brace is not a sign", `$\frac{1}{2}-3$`, "3", answer.SourceLastNumber},
		{"decimal", "x = 3.25, done", "3.25", answer.SourceLastNumber},
		{"leading point", "about .5 of it", ".5", answer.SourceLastNumber},
		{"slash fraction", "the ratio is 3/4.", "3/4", answer.SourceLastNumber},
		{"latex fraction whole", `so $\frac{3}{4}$ of the cake`, `\frac{3}{4}`, answer.SourceLastNumber},
		{"latex fraction with sign", `so $-\dfrac{3}{4}$`, `-\dfrac{3}{4}`, answer.SourceLastNumber},
		{"latex fraction shorthand", `so $\frac34$`, `\frac34`, answer.SourceLastNumber},
		{"percent is kept for parse to refuse", "It rose 25%.", "25%", answer.SourceLastNumber},
		{"escaped percent is kept", `It rose 25\%.`, `25\%`, answer.SourceLastNumber},
		{"currency", "costs $18.90 total", "18.90", answer.SourceLastNumber},
		{"escaped currency", `costs \$18.90 total`, "18.90", answer.SourceLastNumber},
		{"math delimiters", `so \(5\)`, "5", answer.SourceLastNumber},
		{"markdown bold", "**Answer: 5**", "5", answer.SourceLastNumber},
		{"parenthesised", `the value $\left(5\right)$ ... 7 (seven)`, "7", answer.SourceLastNumber},
	} {
		got, err := answer.Extract(tc.text)
		if err != nil {
			t.Errorf("%s: Extract(%q) refused: %v", tc.name, tc.text, err)
			continue
		}
		if got.Raw != tc.raw || got.Source != tc.source {
			t.Errorf("%s: Extract(%q) = {%q %s}, want {%q %s}", tc.name, tc.text, got.Raw, got.Source, tc.raw, tc.source)
		}
	}
}

func TestExtractRefusesWhatItCannotDelimit(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		text   string
		want   error
		raw    string
		source string
	}{
		{"empty", "", answer.ErrNoAnswer, "", ""},
		{"no digits", "no digits here at all", answer.ErrNoAnswer, "", ""},
		{"unicode digits are not numbers", "\u0663 and \uff15", answer.ErrNoAnswer, "", ""},
		{"unclosed boxed", `\boxed{5`, answer.ErrMalformed, "", answer.SourceBoxed},
		{"unclosed nested boxed", `\boxed{\frac{1}{2}`, answer.ErrMalformed, "", answer.SourceBoxed},
		{"boxed without braces", `\boxed 5`, answer.ErrMalformed, "", answer.SourceBoxed},
		{"boxed without braces before a group", `\boxed 5 {6}`, answer.ErrMalformed, "", answer.SourceBoxed},
		{"bare boxed", `the answer is \boxed`, answer.ErrMalformed, "", answer.SourceBoxed},
		{"exponent in braces", `so $2^{10}$`, answer.ErrAmbiguous, "10", answer.SourceLastNumber},
		{"exponent", "so x^2", answer.ErrAmbiguous, "2", answer.SourceLastNumber},
		{"negative exponent", "so 10^-3", answer.ErrAmbiguous, "-3", answer.SourceLastNumber},
		{"radicand", `$\sqrt{2}$`, answer.ErrAmbiguous, "2", answer.SourceLastNumber},
		{"radicand with spaces", `$\sqrt{1 + 2}$`, answer.ErrAmbiguous, "2", answer.SourceLastNumber},
		{"coefficient", "so 5x", answer.ErrAmbiguous, "5", answer.SourceLastNumber},
		{"variable index", "so x2", answer.ErrAmbiguous, "2", answer.SourceLastNumber},
		{"subscript", "so a_1", answer.ErrAmbiguous, "1", answer.SourceLastNumber},
		{"ordinal", "on the 2nd", answer.ErrAmbiguous, "2", answer.SourceLastNumber},
		{"scientific", "about 1e3", answer.ErrAmbiguous, "3", answer.SourceLastNumber},
		{"superscript", "area 5\u00b2", answer.ErrAmbiguous, "5", answer.SourceLastNumber},
		{"another script before", "\u06635", answer.ErrAmbiguous, "5", answer.SourceLastNumber},
		{"thin space thousands", `$1\,000$`, answer.ErrAmbiguous, "000", answer.SourceLastNumber},
		{"negative space thousands", `$10,\!000$`, answer.ErrAmbiguous, "000", answer.SourceLastNumber},
		{"thick space thousands", `$10\;000$`, answer.ErrAmbiguous, "000", answer.SourceLastNumber},
		{"braced comma thousands", `$1{,}000$`, answer.ErrAmbiguous, "000", answer.SourceLastNumber},
		{"under pi", `so $\pi/2$`, answer.ErrAmbiguous, "2", answer.SourceLastNumber},
		{"over a variable", "so 5/x", answer.ErrAmbiguous, "5", answer.SourceLastNumber},
		{"before a command", `so $5\pi$`, answer.ErrAmbiguous, "5", answer.SourceLastNumber},
		{"coefficient of a radical", `so $2\sqrt{2}$`, answer.ErrAmbiguous, "2", answer.SourceLastNumber},
		{"fraction in parentheses", `$\left(\frac{1}{2}\right)$`, answer.ErrAmbiguous, `\frac{1}{2}`, answer.SourceLastNumber},
		{"fraction in an exponent", `$e^{\frac{1}{2}}$`, answer.ErrAmbiguous, `\frac{1}{2}`, answer.SourceLastNumber},
		{"inside text", `\text{The answer is 5}`, answer.ErrAmbiguous, "5", answer.SourceLastNumber},
	} {
		got, err := answer.Extract(tc.text)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: Extract(%q) returned %v, want it to wrap %v", tc.name, tc.text, err, tc.want)
			continue
		}
		if got.Raw != tc.raw || got.Source != tc.source {
			t.Errorf("%s: Extract(%q) reported {%q %s}, want {%q %s}", tc.name, tc.text, got.Raw, got.Source, tc.raw, tc.source)
		}
	}
}

// TestExtractThenParseRefusesAnEmptyBox holds the two halves of an empty
// answer together: Extract reports what the response committed to, and Parse
// is what refuses it.
func TestExtractThenParseRefusesAnEmptyBox(t *testing.T) {
	t.Parallel()

	for _, text := range []string{`\boxed{}`, `\boxed{ }`, `the answer is $\boxed{}$, i.e. 4`, "#### \n4"} {
		got, err := answer.Extract(text)
		if err != nil {
			t.Fatalf("Extract(%q): %v", text, err)
		}
		if _, err := answer.Parse(got.Raw); !errors.Is(err, answer.ErrEmpty) {
			t.Errorf("Parse of the answer in %q returned %v, want ErrEmpty", text, err)
		}
	}
}
