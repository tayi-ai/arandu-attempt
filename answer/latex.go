package answer

import "strings"

// fracCommands are the fraction commands Parse reads. \dfrac and \tfrac are
// \frac at another size, which says nothing about the value.
var fracCommands = []string{`\frac`, `\dfrac`, `\tfrac`}

// boxCommands mark the final answer in MATH-style solutions.
var boxCommands = []string{`\boxed`, `\fbox`}

// command reports whether s opens with the LaTeX command name, and how long
// the name is. A name followed by another letter is a different command:
// \textbf is not \text and \fraction is not \frac.
func command(s, name string) (int, bool) {
	if !strings.HasPrefix(s, name) {
		return 0, false
	}
	if len(s) > len(name) && isASCIILetter(s[len(name)]) {
		return 0, false
	}
	return len(name), true
}

// fracCommand reports whether s opens with \frac, \dfrac or \tfrac, and how
// long the name is.
func fracCommand(s string) (int, bool) {
	for _, name := range fracCommands {
		if n, ok := command(s, name); ok {
			return n, true
		}
	}
	return 0, false
}

// lastCommand finds the last occurrence of any of the named commands in text,
// returning where it starts and where its name ends, or -1 and -1.
func lastCommand(text string, names ...string) (start, end int) {
	start, end = -1, -1
	for _, name := range names {
		limit := len(text)
		for {
			at := strings.LastIndex(text[:limit], name)
			if at < 0 {
				break
			}
			if _, ok := command(text[at:], name); !ok {
				limit = at
				continue
			}
			if at > start {
				start, end = at, at+len(name)
			}
			break
		}
	}
	return start, end
}

// closingBrace returns the index of the brace that closes the one at open.
//
// A backslash escapes the byte after it, so \{ and \} are content rather than
// grouping; \\ is an escaped backslash, which is why the skip is one byte and
// not a lookup of what follows.
func closingBrace(s string, open int) (int, bool) {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return -1, false
}

// balanced reports whether every unescaped brace in s is closed, in order.
func balanced(s string) bool {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '{':
			depth++
		case '}':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

// isDigit is an ASCII digit. Nothing wider: a numeral from another script is
// refused, never read.
func isDigit(b byte) bool { return '0' <= b && b <= '9' }

// isASCIILetter is a letter that can continue a LaTeX command name.
func isASCIILetter(b byte) bool { return 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z' }

// digitsAt returns the index of the first byte at or after from that is not
// an ASCII digit.
func digitsAt(s string, from int) int {
	for from < len(s) && isDigit(s[from]) {
		from++
	}
	return from
}
