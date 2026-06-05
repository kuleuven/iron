//nolint:goconst
package cli

import (
	"bytes"
	"testing"

	"github.com/kuleuven/iron/cmd/iron/tabwriter"
)

func TestBracket(t *testing.T) {
	tests := []struct {
		i, n int
		want string
	}{
		{0, 1, "───"},
		{0, 3, "─┬─"},
		{1, 3, " ├─"},
		{2, 3, " └─"},
	}

	for _, tt := range tests {
		if got := bracket(tt.i, tt.n); got != tt.want {
			t.Errorf("bracket(%d, %d) = %q, want %q", tt.i, tt.n, got, tt.want)
		}
	}
}

func TestFormatPermission(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"own", "own"},
		{"read object", "read"},
		{"read_object", "read"},
		{"write object", "write"},
		{"write_object", "write"},
		{"modify_object", "write"},
		{"delete_object", "delete"},
		{"other", "other"},
	}

	for _, tt := range tests {
		if got := formatPermission(tt.in); got != tt.want {
			t.Errorf("formatPermission(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTablePrinterFlush(t *testing.T) {
	var buf bytes.Buffer

	w := &tabwriter.TabWriter{Writer: &buf}

	tp := &TablePrinter{Writer: w}
	tp.Flush() // should not panic and should flush the underlying writer.
}
