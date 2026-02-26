package msg

import (
	"bytes"
	"testing"
)

func TestPreprocessXMLQuotes(t *testing.T) {
	input := []byte("hello &#34;world&#34;")

	result, err := PreprocessXML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []byte("hello &quot;world&quot;")
	if !bytes.Equal(result, expected) {
		t.Errorf("PreprocessXML(%q) = %q, want %q", input, result, expected)
	}
}

func TestPreprocessXMLApostrophe(t *testing.T) {
	input := []byte("it&#39;s")

	result, err := PreprocessXML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []byte("it&apos;s")
	if !bytes.Equal(result, expected) {
		t.Errorf("PreprocessXML(%q) = %q, want %q", input, result, expected)
	}
}

func TestPreprocessXMLTab(t *testing.T) {
	input := []byte("col1&#x9;col2")

	result, err := PreprocessXML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []byte("col1\tcol2")
	if !bytes.Equal(result, expected) {
		t.Errorf("PreprocessXML(%q) = %q, want %q", input, result, expected)
	}
}

func TestPreprocessXMLNewline(t *testing.T) {
	input := []byte("line1&#xA;line2")

	result, err := PreprocessXML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []byte("line1\nline2")
	if !bytes.Equal(result, expected) {
		t.Errorf("PreprocessXML(%q) = %q, want %q", input, result, expected)
	}
}

func TestPreprocessXMLCarriageReturn(t *testing.T) {
	input := []byte("line1&#xD;line2")

	result, err := PreprocessXML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []byte("line1\rline2")
	if !bytes.Equal(result, expected) {
		t.Errorf("PreprocessXML(%q) = %q, want %q", input, result, expected)
	}
}

func TestPreprocessXMLPassthrough(t *testing.T) {
	input := []byte("<root>hello world</root>")

	result, err := PreprocessXML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Equal(result, input) {
		t.Errorf("PreprocessXML(%q) = %q, want unchanged", input, result)
	}
}

func TestPreprocessXMLInvalidUTF8(t *testing.T) {
	input := []byte{0xff, 0xfe}

	_, err := PreprocessXML(input)
	if err != ErrInvalidUTF8 {
		t.Errorf("expected ErrInvalidUTF8, got %v", err)
	}
}

func TestPreprocessXMLMixed(t *testing.T) {
	input := []byte("a&#34;b&#39;c&#x9;d&#xA;e&#xD;f")

	result, err := PreprocessXML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []byte("a&quot;b&apos;c\td\ne\rf")
	if !bytes.Equal(result, expected) {
		t.Errorf("PreprocessXML(%q) = %q, want %q", input, result, expected)
	}
}

func TestPostprocessXMLClean(t *testing.T) {
	input := []byte("hello world")

	result, err := PostprocessXML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Equal(result, input) {
		t.Errorf("PostprocessXML(%q) = %q, want unchanged", input, result)
	}
}

func TestPostprocessXMLAllowedControlChars(t *testing.T) {
	// Tab (0x09), newline (0x0A), carriage return (0x0D) are allowed
	input := []byte("tab\there\nnewline\rreturn")

	result, err := PostprocessXML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Equal(result, input) {
		t.Errorf("PostprocessXML should preserve allowed control chars, got %q", result)
	}
}

func TestPostprocessXMLInvalidControlChars(t *testing.T) {
	// 0x01 is not a valid XML character, should be replaced with Unicode replacement character
	input := []byte("hello\x01world")

	result, err := PostprocessXML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bytes.Contains(result, []byte{0x01}) {
		t.Error("expected invalid control char to be replaced")
	}

	if !bytes.Contains(result, []byte("\uFFFD")) {
		t.Error("expected replacement character in output")
	}
}

func TestPostprocessXMLInvalidUTF8(t *testing.T) {
	input := []byte{0xff, 0xfe}

	_, err := PostprocessXML(input)
	if err != ErrInvalidUTF8 {
		t.Errorf("expected ErrInvalidUTF8, got %v", err)
	}
}

func TestIsValidChar(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{'\t', true},     // 0x09
		{'\n', true},     // 0x0A
		{'\r', true},     // 0x0D
		{' ', true},      // 0x20
		{'A', true},      // 0x41
		{0x00, false},    // NULL
		{0x01, false},    // SOH
		{0x08, false},    // BS
		{0x0B, false},    // VT
		{0x0C, false},    // FF
		{0x0E, false},    // SO
		{0x1F, false},    // US
		{0xD800, false},  // surrogates start
		{0xDFFF, false},  // surrogates end
		{0xFFFE, false},  // BOM
		{0xFFFF, false},  // not a character
		{0xFFFD, true},   // replacement character
		{0x10000, true},  // start of supplementary plane
		{0x10FFFF, true}, // max Unicode
	}

	for _, tt := range tests {
		result := isValidChar(tt.r)
		if result != tt.want {
			t.Errorf("isValidChar(0x%X) = %v, want %v", tt.r, result, tt.want)
		}
	}
}

func TestPostprocessXMLEmpty(t *testing.T) {
	input := []byte("")

	result, err := PostprocessXML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestPreprocessXMLEmpty(t *testing.T) {
	input := []byte("")

	result, err := PreprocessXML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected empty result, got %q", result)
	}
}
