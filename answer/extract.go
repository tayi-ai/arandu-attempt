package answer

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The extraction rules, as Extraction.Source and Result.Source name them.
const (
	// SourceHash is the text after the last "####", GSM8K style.
	SourceHash = "hash"
	// SourceBoxed is the argument of the last \boxed or \fbox.
	SourceBoxed = "boxed"
	// SourceLastNumber is the last number in the text.
	SourceLastNumber = "last_number"
)

// Extraction is the answer a response committed to, before it is read as a
// number.
type Extraction struct {
	// Raw is the answer text as the response wrote it, trimmed.
	Raw string
	// Source is the rule that found it: SourceHash, SourceBoxed or
	// SourceLastNumber.
	Source string
}

// Extract finds the answer a response committed to.
//
// The rules are tried in order, and the first that applies decides:
//
//  1. the text after the last "####", to the end of its line (GSM8K style);
//  2. the argument of the last \boxed{...} or \fbox{...}, braces balanced;
//  3. the last number in the text.
//
// A rule applies when its marker is present, not when its content parses. A
// response that wrote "#### I don't know" committed to that, and reading its
// last number instead would score an answer it never gave; Parse is what then
// refuses the content. An empty Raw from rule 1 or 2 is returned without an
// error for the same reason, and Parse refuses it with ErrEmpty.
//
// The last-number rule is positional and blind to meaning, so it refuses with
// ErrAmbiguous when that number is glued to a larger expression: the exponent
// of 2^{10}, the 2 of \sqrt{2}, the 000 of 1\,000, the 5 of 5x. The digits
// alone would be a misread. A \frac, \dfrac or \tfrac is taken whole, a sign
// directly before the number is kept, and a % right after it is kept so that
// Parse refuses the percentage instead of the rule dropping it.
//
// Extract returns ErrNoAnswer when no rule applies and ErrMalformed when a
// \boxed has no braced argument or its braces never close. The Extraction
// returned with an error still names the rule and, where there is one, the
// text it looked at.
func Extract(text string) (Extraction, error) {
	if raw, ok := hashAnswer(text); ok {
		return Extraction{Raw: raw, Source: SourceHash}, nil
	}
	if start, end := lastCommand(text, boxCommands...); start >= 0 {
		raw, err := boxedArgument(text, start, end)
		return Extraction{Raw: raw, Source: SourceBoxed}, err
	}
	return lastNumber(text)
}

// hashAnswer is the text after the last "####" in text, up to the end of its
// line, and whether there is one.
func hashAnswer(text string) (string, bool) {
	at := strings.LastIndex(text, "####")
	if at < 0 {
		return "", false
	}
	line := text[at+len("####"):]
	if end := strings.IndexAny(line, "\r\n"); end >= 0 {
		line = line[:end]
	}
	return strings.TrimSpace(line), true
}

// boxedArgument is the braced argument of the command spanning text[start:end].
func boxedArgument(text string, start, end int) (string, error) {
	name := text[start:end]
	rest := text[end:]
	open := len(rest) - len(strings.TrimLeftFunc(rest, unicode.IsSpace))
	if open == len(rest) || rest[open] != '{' {
		return "", fmt.Errorf("%w: %s has no braced argument", ErrMalformed, name)
	}
	closing, ok := closingBrace(rest, open)
	if !ok {
		return "", fmt.Errorf("%w: the braces of the last %s never close", ErrMalformed, name)
	}
	return strings.TrimSpace(rest[open+1 : closing]), nil
}

// lastNumber is the last number in text, or a refusal to say which one it is.
//
// A number is a run of ASCII digits that may carry a decimal point, thousands
// commas and a fraction slash, each only when a digit follows it, so the
// period ending "is 5." is not part of it. A \frac, \dfrac or \tfrac with its
// two arguments is one number.
func lastNumber(text string) (Extraction, error) {
	start, end := -1, -1
	for i := 0; i < len(text); {
		if n, ok := fracCommand(text[i:]); ok {
			if stop, ok := fracEnd(text, i+n); ok {
				start, end = i, stop
				i = stop
				continue
			}
			i += n
			continue
		}
		if isDigit(text[i]) || text[i] == '.' && i+1 < len(text) && isDigit(text[i+1]) {
			j := i + 1
			for j < len(text) {
				if isDigit(text[j]) {
					j++
					continue
				}
				if strings.IndexByte(".,/", text[j]) >= 0 && j+1 < len(text) && isDigit(text[j+1]) {
					j++
					continue
				}
				break
			}
			start, end = i, j
			i = j
			continue
		}
		i++
	}
	if start < 0 {
		return Extraction{}, fmt.Errorf("%w: the text has no \\boxed, no \"####\" and no ASCII number", ErrNoAnswer)
	}

	start = withSign(text, start)
	end = withPercent(text, end)
	found := Extraction{Raw: text[start:end], Source: SourceLastNumber}
	if embedded(text, start, end) {
		return found, fmt.Errorf("%w: the last number %q is part of a larger expression", ErrAmbiguous, found.Raw)
	}
	return found, nil
}

// fracEnd is where the fraction whose name ends at from finishes, and whether
// its two arguments are there to delimit it.
func fracEnd(text string, from int) (int, bool) {
	_, rest, err := fracArgument(text[from:])
	if err != nil {
		return 0, false
	}
	_, rest, err = fracArgument(rest)
	if err != nil {
		return 0, false
	}
	return len(text) - len(rest), true
}

// withSign moves start back over a sign that belongs to the number: one that
// does not follow something it could be subtracting from, as in "x = -5" but
// not "3-5".
func withSign(text string, start int) int {
	r, width := utf8.DecodeLastRuneInString(text[:start])
	if r != '-' && r != '+' && r != '\u2212' {
		return start
	}
	before, _ := utf8.DecodeLastRuneInString(text[:start-width])
	if start-width > 0 && (unicode.IsLetter(before) || unicode.IsNumber(before) || strings.ContainsRune(")]}", before)) {
		return start
	}
	return start - width
}

// withPercent moves end forward over a % or \% written right after the number.
func withPercent(text string, end int) int {
	switch {
	case strings.HasPrefix(text[end:], "%"):
		return end + 1
	case strings.HasPrefix(text[end:], `\%`):
		return end + 2
	}
	return end
}

// embedded reports whether the number at text[start:end] is glued to a larger
// expression, so that reading it alone would read part of something else.
//
// Before it: a letter or numeral (x2, or a digit from another script), an
// exponent or subscript mark, a brace (\sqrt{2}, x^{2}, 1{,}000), a slash
// (\pi/2), or a LaTeX spacing escape (1\,000, 10,\!000, 1\;000). After it: a
// letter or numeral (5x, 2nd, 1e3, a superscript 2), an exponent or subscript
// mark, a closing brace, a slash (5/x), or a LaTeX command (5\pi, 2\sqrt{2}).
//
// \$5 and \(5\) are not embedded: a currency mark and a math delimiter say
// nothing about which number is meant.
func embedded(text string, start, end int) bool {
	if start > 0 {
		before, _ := utf8.DecodeLastRuneInString(text[:start])
		if unicode.IsLetter(before) || unicode.IsNumber(before) || strings.ContainsRune(`^_{}\,!/`, before) {
			return true
		}
		if start >= 2 && text[start-2] == '\\' && strings.IndexByte(";: >", text[start-1]) >= 0 {
			return true
		}
	}
	if end < len(text) {
		after, _ := utf8.DecodeRuneInString(text[end:])
		if unicode.IsLetter(after) || unicode.IsNumber(after) || strings.ContainsRune("^_}/", after) {
			return true
		}
		if after == '\\' && end+1 < len(text) && isASCIILetter(text[end+1]) {
			return true
		}
	}
	return false
}
