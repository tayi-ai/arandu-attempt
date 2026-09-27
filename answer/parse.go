package answer

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// minus is U+2212, the sign typeset output and some tokenizers write in place
// of the ASCII hyphen.
const minus = "\u2212"

// maxLayers bounds how many wrappers Parse peels (math-mode dollars, \text,
// \mathrm), so an answer built to recurse cannot.
const maxLayers = 8

// scientific matches a mantissa with an exponent: 1e3, 2.5E-4, 1\times10^3,
// 3 \cdot 10^{2}.
var scientific = regexp.MustCompile(`[0-9.][eE][+\-\x{2212}]?[0-9]|(\\times|\\cdot|\x{00d7}|\x{00b7}|\*)\s*\{?\s*10\s*\^`)

// allowedCommands are the LaTeX commands a number may be written with. Any
// other names something that is not a plain number: \sqrt, \pi, \infty, \left.
var allowedCommands = map[string]bool{
	"frac": true, "dfrac": true, "tfrac": true, "text": true, "mathrm": true,
}

// Parse reads raw as one exact rational number, or refuses it.
//
// It accepts an integer (leading zeros included), a decimal such as 0.5, .5 or
// 2.50, a fraction a/b, and \frac, \dfrac or \tfrac with braced arguments or
// the single-digit shorthand \frac34. Around the number it accepts surrounding
// whitespace, one sign (U+2212 included), one currency mark ($ or \$), one
// trailing period, math-mode dollars around the whole answer, and \text{...}
// or \mathrm{...} around the whole answer when what is inside is itself a
// number. A comma is accepted only as a thousands separator in the form 1,000
// or 12,345,678.
//
// Everything else is refused with an error wrapping ErrEmpty, ErrNotNumeric,
// ErrAmbiguous, ErrUnsupported, ErrDivisionByZero or ErrMalformed. A verifier
// that read an unfamiliar form one way would be right only when the author
// happened to mean that way.
func Parse(raw string) (*big.Rat, error) {
	return parseLayer(raw, 0)
}

// parseLayer is Parse at a given wrapper depth.
func parseLayer(raw string, depth int) (*big.Rat, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, fmt.Errorf("%w: nothing to read in %q", ErrEmpty, raw)
	}
	s = dropTrailingPeriod(s)
	if inner, ok := unwrap(s); ok {
		if depth == maxLayers {
			return nil, fmt.Errorf("%w: %q nests more than %d wrappers", ErrNotNumeric, raw, maxLayers)
		}
		return parseLayer(inner, depth+1)
	}

	negative, body, err := prefix(s)
	if err != nil {
		return nil, err
	}
	value, rest, err := parseBody(body)
	if err == nil && strings.TrimSpace(rest) == "" {
		if negative {
			value.Neg(value)
		}
		return value, nil
	}
	return nil, refusal(s, rest, err)
}

// dropTrailingPeriod removes the full stop that ends a sentence, once. A
// second one stays, so "5.." is not a number.
func dropTrailingPeriod(s string) string {
	if len(s) > 1 && s[len(s)-1] == '.' {
		return strings.TrimRightFunc(s[:len(s)-1], unicode.IsSpace)
	}
	return s
}

// unwrap returns the inside of a wrapper that spans all of s and says nothing
// about the value: math-mode dollars, \text{...} or \mathrm{...}.
func unwrap(s string) (string, bool) {
	if len(s) >= 2 && s[0] == '$' && s[len(s)-1] == '$' && s[len(s)-2] != '\\' {
		return s[1 : len(s)-1], true
	}
	for _, name := range []string{`\text`, `\mathrm`} {
		n, ok := command(s, name)
		if !ok {
			continue
		}
		rest := s[n:]
		open := len(rest) - len(strings.TrimLeftFunc(rest, unicode.IsSpace))
		if open < len(rest) && rest[open] == '{' {
			if end, ok := closingBrace(rest, open); ok && end == len(rest)-1 {
				return rest[open+1 : end], true
			}
		}
	}
	return "", false
}

// leadingSign reports whether s opens with a sign, whether it negates, and how
// many bytes it takes.
func leadingSign(s string) (negative bool, width int) {
	switch {
	case strings.HasPrefix(s, "-"):
		return true, 1
	case strings.HasPrefix(s, "+"):
		return false, 1
	case strings.HasPrefix(s, minus):
		return true, len(minus)
	}
	return false, 0
}

// prefix takes the sign and the currency mark off the front of s. One of each
// is allowed, in either order; two signs or two currency marks are not how
// anyone writes one number.
func prefix(s string) (negative bool, body string, err error) {
	signs, currencies := 0, 0
	for {
		if neg, width := leadingSign(s); width > 0 {
			negative = neg
			signs++
			s = s[width:]
		} else if strings.HasPrefix(s, `\$`) {
			currencies++
			s = s[len(`\$`):]
		} else if strings.HasPrefix(s, "$") {
			currencies++
			s = s[1:]
		} else {
			break
		}
		s = strings.TrimLeftFunc(s, unicode.IsSpace)
	}
	if signs > 1 {
		return false, s, fmt.Errorf("%w: more than one sign before %q", ErrNotNumeric, s)
	}
	if currencies > 1 {
		return false, s, fmt.Errorf("%w: more than one currency mark before %q", ErrNotNumeric, s)
	}
	return negative, s, nil
}

// parseBody reads one unsigned number off the front of s, either a \frac or a
// decimal optionally over another, and returns what follows it.
func parseBody(s string) (*big.Rat, string, error) {
	if n, ok := fracCommand(s); ok {
		return parseFrac(s, n)
	}
	numerator, rest, err := parseDecimal(s)
	if err != nil {
		return nil, rest, err
	}
	after := strings.TrimLeftFunc(rest, unicode.IsSpace)
	if !strings.HasPrefix(after, "/") {
		return numerator, rest, nil
	}
	denominator, rest, err := parseDecimal(strings.TrimLeftFunc(after[1:], unicode.IsSpace))
	if err != nil {
		return nil, rest, err
	}
	return divide(numerator, denominator, s, rest)
}

// parseDecimal reads unsigned ASCII digits off the front of s: an integer,
// optionally grouped by thousands, then an optional fractional part.
//
// A comma is read only as a thousands separator, and only where it cannot be
// anything else: a first group of one to three digits that does not start with
// 0, then groups of exactly three. 1,5 and 1,00 are decimal commas in much of
// the world and thousands separators missing a digit in the rest, and nobody
// writes five hundred as 0,500; each is refused as ambiguous rather than read
// either way. The period is always the decimal point, as it is in both
// datasets, so 1.000 is one.
func parseDecimal(s string) (*big.Rat, string, error) {
	i := digitsAt(s, 0)
	whole := s[:i]
	var digits strings.Builder
	digits.WriteString(whole)

	if i > 0 && i < len(s) && s[i] == ',' {
		if len(whole) > 3 || whole[0] == '0' {
			return nil, s, fmt.Errorf("%w: %q has a comma that is not a thousands separator", ErrAmbiguous, s)
		}
		for i < len(s) && s[i] == ',' {
			end := digitsAt(s, i+1)
			if end-(i+1) != 3 {
				return nil, s, fmt.Errorf("%w: %q has a comma that is not a thousands separator", ErrAmbiguous, s)
			}
			digits.WriteString(s[i+1 : end])
			i = end
		}
	}

	fraction := ""
	if i+1 < len(s) && s[i] == '.' && isDigit(s[i+1]) {
		end := digitsAt(s, i+1)
		fraction = s[i+1 : end]
		i = end
	}
	if whole == "" && fraction == "" {
		return nil, s, fmt.Errorf("%w: %q does not start with a digit", ErrNotNumeric, s)
	}

	digits.WriteString(fraction)
	numerator, _ := new(big.Int).SetString(digits.String(), 10)
	denominator := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(len(fraction))), nil)
	return new(big.Rat).SetFrac(numerator, denominator), s[i:], nil
}

// parseFrac reads a \frac, \dfrac or \tfrac at the front of s, whose name is n
// bytes long.
func parseFrac(s string, n int) (*big.Rat, string, error) {
	top, rest, err := fracArgument(s[n:])
	if err != nil {
		return nil, rest, err
	}
	bottom, rest, err := fracArgument(rest)
	if err != nil {
		return nil, rest, err
	}
	numerator, err := parseArgument(top)
	if err != nil {
		return nil, rest, err
	}
	denominator, err := parseArgument(bottom)
	if err != nil {
		return nil, rest, err
	}
	return divide(numerator, denominator, s, rest)
}

// fracArgument splits one \frac argument off the front of s. An argument is a
// braced group or, as TeX reads \frac34, a single digit.
func fracArgument(s string) (arg, rest string, err error) {
	s = strings.TrimLeftFunc(s, unicode.IsSpace)
	switch {
	case s == "":
		return "", s, fmt.Errorf("%w: \\frac is missing an argument", ErrNotNumeric)
	case s[0] == '{':
		end, ok := closingBrace(s, 0)
		if !ok {
			return "", s, fmt.Errorf("%w: a \\frac argument never closes in %q", ErrMalformed, s)
		}
		return s[1:end], s[end+1:], nil
	case isDigit(s[0]):
		return s[:1], s[1:], nil
	}
	return "", s, fmt.Errorf("%w: a \\frac argument is a braced group or one digit, not %q", ErrNotNumeric, s)
}

// parseArgument reads the inside of one \frac argument: a signed decimal and
// nothing else. \frac{1}{\sqrt{2}} and a \frac inside a \frac are refused
// rather than evaluated.
func parseArgument(arg string) (*big.Rat, error) {
	s := strings.TrimSpace(arg)
	if s == "" {
		return nil, fmt.Errorf("%w: a \\frac argument is empty", ErrNotNumeric)
	}
	negative, width := leadingSign(s)
	value, rest, err := parseDecimal(strings.TrimLeftFunc(s[width:], unicode.IsSpace))
	if err == nil && strings.TrimSpace(rest) == "" {
		if negative {
			value.Neg(value)
		}
		return value, nil
	}
	return nil, refusal(s, rest, err)
}

// divide is numerator over denominator, refusing a zero denominator.
func divide(numerator, denominator *big.Rat, s, rest string) (*big.Rat, string, error) {
	if denominator.Sign() == 0 {
		return nil, rest, fmt.Errorf("%w: %q", ErrDivisionByZero, s)
	}
	return numerator.Quo(numerator, denominator), rest, nil
}

// refusal names why s is not one number. What s contains outranks where the
// grammar stopped: 2\sqrt{2} is refused for its radical, not for the text
// after the 2. When nothing it contains rules a number out, the grammar's own
// error stands, and a second number left over makes the answer ambiguous.
func refusal(s, rest string, err error) error {
	if reason := classify(s); reason != nil {
		return reason
	}
	if err != nil {
		return err
	}
	if strings.ContainsAny(rest, "0123456789") {
		return fmt.Errorf("%w: %q holds more than one number", ErrAmbiguous, s)
	}
	return fmt.Errorf("%w: %q is not a plain number", ErrNotNumeric, s)
}

// classify returns the reason s cannot be one plain number, judged by what it
// contains, or nil when nothing in it rules one out.
func classify(s string) error {
	if !balanced(s) {
		return fmt.Errorf("%w: %q has braces that do not balance", ErrMalformed, s)
	}
	// Scientific notation is refused in this version. Neither dataset writes a
	// final answer that way, and reading 1e3 as 1000 would let a float printed
	// by a tool pass for an exact answer.
	if scientific.MatchString(s) {
		return fmt.Errorf("%w: %q is in scientific notation", ErrUnsupported, s)
	}

	for i := 0; i < len(s); {
		r, width := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == '\\':
			end := i + 1
			for end < len(s) && isASCIILetter(s[end]) {
				end++
			}
			name := s[i+1 : end]
			if name != "" {
				if !allowedCommands[name] {
					return fmt.Errorf("%w: %q contains \\%s", ErrNotNumeric, s, name)
				}
				i = end
				continue
			}
			// An escaped symbol such as \$, \% or \, is skipped here. What it
			// leaves unread, a set's \{ or a stray \}, the grammar refuses.
			_, escaped := utf8.DecodeRuneInString(s[end:])
			i = end + escaped
			continue
		case strings.ContainsRune("()[]", r):
			return fmt.Errorf("%w: %q is a tuple, an interval or a grouping, not a number", ErrNotNumeric, s)
		case unicode.IsLetter(r):
			return fmt.Errorf("%w: %q contains the letter %q", ErrNotNumeric, s, r)
		case r == '^' || r == '_':
			return fmt.Errorf("%w: %q contains an exponent or a subscript", ErrNotNumeric, s)
		}
		i += width
	}

	// A percentage is refused rather than read. GSM8K writes the number of
	// percent (#### 25 for 25%), while MATH writes 25\% in some solutions and
	// 0.25 in others, so 25% equals 25 against one reference and 0.25 against
	// another. Either reading is a guess that is right for one dataset and
	// wrong for the other.
	if strings.HasSuffix(strings.TrimRightFunc(s, unicode.IsSpace), "%") {
		return fmt.Errorf("%w: %q is a percentage", ErrUnsupported, s)
	}
	return nil
}
