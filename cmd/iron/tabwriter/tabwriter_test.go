package tabwriter

import (
	"bytes"
	"testing"
)

var example = []byte("EXAMPLE\tTABLE\tHEADER\nThis\tis\ta\ttest\ttable\twith\t\033[1;31mmultiple\033[0m\tcolumns\tand\trows.\nEven\tMore\tColumns\tAnd\tRows.")

var expectedLength = 144

func TestNewTabWriter(t *testing.T) {
	var buf bytes.Buffer

	writer := &TabWriter{
		Writer: &buf,
	}

	if _, err := writer.Write(example); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if err := writer.Flush(); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(buf.String()) != expectedLength {
		t.Errorf("Expected length %d, got %d", expectedLength, len(buf.String()))
	}
}
