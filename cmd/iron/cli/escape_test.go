package cli

import (
	"fmt"
	"testing"
)

//nolint:funlen,goconst
func TestUnescapeString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		wantErr  bool
		errMsg   string
	}{
		// Basic escape sequences
		{"hello", "hello", false, ""},
		{"hello\\nworld", "hello\nworld", false, ""},
		{"hello\\rworld", "hello\rworld", false, ""},
		{"hello\\tworld", "hello\tworld", false, ""},
		{"hello\\bworld", "hello\bworld", false, ""},
		{"hello\\fworld", "hello\fworld", false, ""},
		{"hello\\vworld", "hello\vworld", false, ""},
		{"hello\\aworld", "hello\aworld", false, ""},

		// Null byte
		{"hello\\0world", "hello\x00world", false, ""},

		// Double backslash
		{"hello\\\\world", "hello\\world", false, ""},
		{"\\\\", "\\", false, ""},

		// Hex escapes
		{"\\x41", "A", false, ""},  // 0x41 = 'A'
		{"\\x0d", "\r", false, ""}, // 0x0d = CR
		{"\\x20", " ", false, ""},  // 0x20 = space
		{"hello\\x20world", "hello world", false, ""},

		// Unicode escapes (16-bit)
		{"\\u0041", "A", false, ""},  // U+0041 = 'A'
		{"\\u000d", "\r", false, ""}, // U+000D = CR
		{"\\u03B1", "α", false, ""},  // U+03B1 = Greek alpha

		// Unicode escapes (32-bit)
		{"\\U00000041", "A", false, ""}, // U+00000041 = 'A'
		{"\\U0001F600", "😀", false, ""}, // U+1F600 = grinning face

		// Mixed escapes
		{"line1\\nline2\\ttab", "line1\nline2\ttab", false, ""},
		{"file\\x0dname", "file\rname", false, ""},

		// Error cases: incomplete hex
		{"\\x", "", true, "invalid hex escape sequence"},
		{"\\x0", "", true, "invalid hex escape sequence"},
		{"hello\\x", "", true, "invalid hex escape sequence"},

		// Error cases: incomplete unicode
		{"\\u", "", true, "invalid unicode escape sequence"},
		{"\\u00", "", true, "invalid unicode escape sequence"},
		{"\\u004", "", true, "invalid unicode escape sequence"},

		// Error cases: invalid hex
		{"\\xGG", "", true, "invalid hex escape sequence"},
		{"\\u00GG", "", true, "invalid unicode escape sequence"},

		// Error cases: invalid escape character
		{"\\q", "", true, "invalid escape sequence"},
		{"\\#", "", true, "invalid escape sequence"},

		// Carriage return (the original problem)
		{"file\\rname", "file\rname", false, ""},
		{"file\\x0dname", "file\rname", false, ""},
		{"file\\u000dname", "file\rname", false, ""},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("case_%d_%s", i, tt.input), func(t *testing.T) {
			got, err := UnescapeString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("UnescapeString(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}

			if err != nil && tt.errMsg != "" && tt.errMsg != fmt.Sprintf("%v", err) {
				if !contains(fmt.Sprintf("%v", err), tt.errMsg) {
					t.Errorf("UnescapeString(%q) error = %v, want error containing %q", tt.input, err, tt.errMsg)
				}
			}

			if got != tt.expected {
				t.Errorf("UnescapeString(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestEscapeString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"hello\nworld", "hello\\nworld"},
		{"hello\rworld", "hello\\rworld"},
		{"hello\tworld", "hello\\tworld"},
		{"hello\bworld", "hello\\bworld"},
		{"hello\fworld", "hello\\fworld"},
		{"hello\vworld", "hello\\vworld"},
		{"hello\aworld", "hello\\aworld"},
		{"hello\x00world", "hello\\0world"},
		{"hello\\world", "hello\\\\world"},

		// Non-printable ASCII (should be hex-escaped)
		{"\x01test", "\\x01test"},
		{"test\x1f", "test\\x1f"},
		{"\x7ftest", "\\x7ftest"}, // DEL character

		// Carriage return
		{"file\rname", "file\\rname"},
		{"line1\rline2", "line1\\rline2"},

		// Unicode characters should be preserved
		{"hello α world", "hello α world"},
		{"café", "café"},
		{"😀", "😀"},

		// Mixed
		{"hello\nworld\ttab\rCR", "hello\\nworld\\ttab\\rCR"},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("case_%d_%s", i, tt.input), func(t *testing.T) {
			got := EscapeString(tt.input)
			if got != tt.expected {
				t.Errorf("EscapeString(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	// Test that UnescapeString(EscapeString(s)) == s for various inputs
	testStrings := []string{
		"simple",
		"hello world",
		"file\rname",
		"line1\nline2\ttab",
		"with\x00null",
		"unicode α café 😀",
	}

	for _, original := range testStrings {
		t.Run(original, func(t *testing.T) {
			escaped := EscapeString(original)

			unescaped, err := UnescapeString(escaped)
			if err != nil {
				t.Fatalf("UnescapeString(%q) failed: %v", escaped, err)
			}

			if unescaped != original {
				t.Errorf("Round trip failed for %q:\n  original: %q\n  escaped: %q\n  unescaped: %q", original, original, escaped, unescaped)
			}
		})
	}
}

func contains(s, substr string) bool {
	return substr == "" || (s != "" && (s[0:len(substr)] == substr || contains(s[1:], substr)))
}
