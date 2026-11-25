package cli

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// UnescapeString processes Go-style escape sequences in a string.
// Supported sequences:
//   - \n, \r, \t, \b, \f, \v, \a, \0
//   - \xHH (hex byte)
//   - \uHHHH (unicode 16-bit)
//   - \UHHHHHHHH (unicode 32-bit)
//   - \\ (literal backslash)
//
// Invalid escape sequences return an error.
//
//nolint:gocognit,nestif,gocyclo,funlen,wsl
func UnescapeString(s string) (string, error) {
	var result strings.Builder

	i := 0

	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				result.WriteByte('\n')
			case 'r':
				result.WriteByte('\r')
			case 't':
				result.WriteByte('\t')
			case 'b':
				result.WriteByte('\b')
			case 'f':
				result.WriteByte('\f')
			case 'v':
				result.WriteByte('\v')
			case 'a':
				result.WriteByte('\a')
			case '0':
				result.WriteByte('\x00')
			case '\\':
				result.WriteByte('\\')
			case 'x':
				// \xHH - hex byte
				if i+2 >= len(s) {
					return "", fmt.Errorf("invalid hex escape sequence at position %d", i-1)
				}

				hexStr := s[i+1 : i+3]

				b, err := strconv.ParseUint(hexStr, 16, 8)
				if err != nil {
					return "", fmt.Errorf("invalid hex escape sequence '\\x%s' at position %d: %w", hexStr, i-1, err)
				}

				result.WriteByte(byte(b))

				i += 2
			case 'u':
				// \uHHHH - unicode 16-bit
				if i+4 >= len(s) {
					return "", fmt.Errorf("invalid unicode escape sequence at position %d", i-1)
				}

				hexStr := s[i+1 : i+5]

				r, err := strconv.ParseUint(hexStr, 16, 32)
				if err != nil {
					return "", fmt.Errorf("invalid unicode escape sequence '\\u%s' at position %d: %w", hexStr, i-1, err)
				}

				result.WriteRune(rune(r))

				i += 4
			case 'U':
				// \UHHHHHHHH - unicode 32-bit
				if i+8 >= len(s) {
					return "", fmt.Errorf("invalid unicode escape sequence at position %d", i-1)
				}

				hexStr := s[i+1 : i+9]

				r, err := strconv.ParseUint(hexStr, 16, 32)
				if err != nil {
					return "", fmt.Errorf("invalid unicode escape sequence '\\U%s' at position %d: %w", hexStr, i-1, err)
				}

				result.WriteRune(rune(r))

				i += 8
			default:
				return "", fmt.Errorf("invalid escape sequence '\\%c' at position %d", s[i], i-1)
			}

			i++
		} else {
			r, size := utf8.DecodeRuneInString(s[i:])
			result.WriteRune(r)

			i += size
		}
	}

	return result.String(), nil
}

// EscapeString returns a representation of the string with special characters escaped.
// This is useful for displaying strings that may contain non-printable characters.
// Escapes the same sequences as UnescapeString can process:
//   - \n, \r, \t, \b, \f, \v, \a, \0
//   - \xHH for other non-printable bytes
//   - \\ for literal backslash
//
// By default, printable Unicode characters are left as-is.
func EscapeString(s string) string {
	var result strings.Builder

	for _, r := range s {
		switch r {
		case '\n':
			result.WriteString("\\n")
		case '\r':
			result.WriteString("\\r")
		case '\t':
			result.WriteString("\\t")
		case '\b':
			result.WriteString("\\b")
		case '\f':
			result.WriteString("\\f")
		case '\v':
			result.WriteString("\\v")
		case '\a':
			result.WriteString("\\a")
		case '\x00':
			result.WriteString("\\0")
		case '\\':
			result.WriteString("\\\\")
		default:
			// Keep other printable characters as-is
			if r < 32 || r == 127 || (r >= 0x80 && !utf8.ValidRune(r)) {
				// Non-printable or invalid rune, use hex escape
				for _, b := range []byte(string(r)) {
					fmt.Fprintf(&result, "\\x%02x", b)
				}
			} else {
				result.WriteRune(r)
			}
		}
	}

	return result.String()
}

// Unescape conditionally unescapes a string unless the --literal flag is set.
// If unescaping fails, the original string is returned unchanged.
// This is used as a convenience method on the App to apply the Literal flag setting.
func (a *App) Unescape(s string) string {
	if a.Literal {
		return s
	}

	unescaped, err := UnescapeString(s)
	if err != nil {
		return s
	}

	return unescaped
}
