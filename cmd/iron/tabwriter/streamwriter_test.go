package tabwriter

import (
	"bytes"
	"testing"
)

const expectedLengthStreamWriter = 263

func TestNewStreamWriter(t *testing.T) {
	var buf bytes.Buffer

	writer := &StreamWriter{
		Writer:       &buf,
		ColumnWidths: []int{15, 20, 25},
	}

	if _, err := writer.Write(example); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if buf.String() == "" {
		t.Errorf("Expected buffer to be non-empty")
	}

	if err := writer.Flush(); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(buf.String()) != expectedLengthStreamWriter {
		t.Errorf("Expected length %d, got %d", expectedLengthStreamWriter, len(buf.String()))
	}
}
