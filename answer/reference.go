package answer

import (
	"errors"
	"fmt"
	"math/big"
)

// Reference is a dataset's final answer: the text Verify takes as its
// reference, and the exact number that text denotes.
type Reference struct {
	// Raw is the answer text as the dataset wrote it, trimmed.
	Raw string
	// Value is the number Raw denotes.
	Value *big.Rat
}

// ParseGSM8KReference reads the final answer of a GSM8K solution: the text
// after its last "####", to the end of that line.
//
// A solution with no "####" is refused with ErrNoAnswer and not with
// ErrNotNumeric: that is a broken row, and a filter keeping numeric items
// would otherwise drop it without anybody seeing it. An answer that is there
// and that Parse refuses is returned as an error wrapping ErrNotNumeric and
// the reason Parse gave.
func ParseGSM8KReference(solution string) (Reference, error) {
	raw, ok := hashAnswer(solution)
	if !ok {
		return Reference{}, fmt.Errorf("%w: the GSM8K solution has no \"####\" line", ErrNoAnswer)
	}
	return numericReference("GSM8K", raw)
}

// ParseMATHReference reads the final answer of a MATH solution: the argument
// of its last \boxed{...} or \fbox{...}.
//
// It never falls back to the last number of the solution, since ground truth
// is not guessed. A solution with no \boxed is refused with ErrNoAnswer, one
// whose braces do not delimit the answer with ErrMalformed, and an answer
// that Parse refuses with an error wrapping ErrNotNumeric and the reason
// Parse gave, so that \boxed{2\sqrt{2}} and \boxed{(1,2)} are left out of a
// numeric set.
func ParseMATHReference(solution string) (Reference, error) {
	start, end := lastCommand(solution, boxCommands...)
	if start < 0 {
		return Reference{}, fmt.Errorf("%w: the MATH solution has no \\boxed answer", ErrNoAnswer)
	}
	raw, err := boxedArgument(solution, start, end)
	if err != nil {
		return Reference{}, err
	}
	return numericReference("MATH", raw)
}

// numericReference parses raw as the reference answer of dataset.
func numericReference(dataset, raw string) (Reference, error) {
	value, err := Parse(raw)
	if err == nil {
		return Reference{Raw: raw, Value: value}, nil
	}
	if errors.Is(err, ErrNotNumeric) {
		return Reference{}, fmt.Errorf("%s answer %q: %w", dataset, raw, err)
	}
	return Reference{}, fmt.Errorf("%w: %s answer %q: %w", ErrNotNumeric, dataset, raw, err)
}
