// Package answer decides whether a response to a math problem reached the
// dataset's final numeric answer, and refuses whenever it cannot know.
//
// It is the ground truth a reward is computed from, so a wrong "correct" costs
// more than a wrong "incorrect": the first teaches the policy that a mistake
// pays. Every rule here leans the same way. The first extraction rule that
// applies decides, and its failure is not rescued by a later one; a notation
// that datasets read differently is refused rather than read one way; and two
// answers are equal only when they are the same rational number, computed
// exactly with math/big rather than compared as floats or as strings.
//
// Extract finds the answer text in a response, Parse reads it as a *big.Rat,
// and Verify does both and compares the result with a reference.
// ParseGSM8KReference and ParseMATHReference take the reference out of a
// dataset solution and report whether it is numeric at all.
package answer

import (
	"errors"
	"fmt"
)

// VerifierID names the semantics of Verify: exact rational equality, after the
// extraction rules of Extract and the notation Parse accepts. A change to what
// is extracted, accepted or refused is a new identifier, so a recorded verdict
// always says which rules produced it.
const VerifierID = "math.numeric_exact_rational.v1"

// The refusals. Every error returned by this package wraps at least one of
// them, so a caller branches with errors.Is and never on message text.
var (
	// ErrEmpty is an answer with nothing in it: an empty string, blank text,
	// or an empty \boxed{}.
	ErrEmpty = errors.New("answer: empty")
	// ErrNotNumeric is an answer that is not one plain number: a variable, a
	// radical, pi, a tuple, an interval, a word, or a numeral outside ASCII.
	ErrNotNumeric = errors.New("answer: not a number")
	// ErrAmbiguous is an answer that could be read as more than one number: a
	// comma that is not a thousands separator, two numbers where one was
	// expected, or a last number glued to a larger expression.
	ErrAmbiguous = errors.New("answer: ambiguous")
	// ErrUnsupported is a number written in a notation this version refuses on
	// purpose: a percentage or scientific notation.
	ErrUnsupported = errors.New("answer: unsupported notation")
	// ErrDivisionByZero is a fraction whose denominator is zero.
	ErrDivisionByZero = errors.New("answer: division by zero")
	// ErrMalformed is LaTeX whose braces do not delimit an answer: an unclosed
	// group, or \boxed with no braced argument.
	ErrMalformed = errors.New("answer: malformed")
	// ErrNoAnswer is text in which no extraction rule found an answer at all.
	ErrNoAnswer = errors.New("answer: no answer found")
)

// Result is the verdict on one response.
type Result struct {
	// Correct is true only when the response's answer and the reference both
	// parse and denote the same rational number.
	Correct bool
	// Extracted is the answer text taken from the response; empty when no
	// rule found one.
	Extracted string
	// Source is the extraction rule that found it: SourceHash, SourceBoxed or
	// SourceLastNumber; empty when no rule found one.
	Source string
	// Reason says why Correct is false, and is empty when it is true.
	Reason string
}

// Verify extracts the final answer from response and compares it with
// reference, the dataset's final answer string already taken out of its
// solution.
//
// The reference is read with Parse, never extracted: it is ground truth, and a
// reference that does not parse is a dataset problem reported in Reason, with
// Correct false whatever the response says. There is no fallback to comparing
// text, so two identical answers that are not numbers are not correct.
func Verify(response, reference string) Result {
	found, extractErr := Extract(response)
	result := Result{Extracted: found.Raw, Source: found.Source}

	want, err := Parse(reference)
	if err != nil {
		result.Reason = "reference: " + err.Error()
		return result
	}
	if extractErr != nil {
		result.Reason = "response: " + extractErr.Error()
		return result
	}
	got, err := Parse(found.Raw)
	if err != nil {
		result.Reason = "response: " + err.Error()
		return result
	}
	if got.Cmp(want) != 0 {
		result.Reason = fmt.Sprintf("response %s differs from reference %s", got.RatString(), want.RatString())
		return result
	}
	result.Correct = true
	return result
}
