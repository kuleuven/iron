package tabwriter

import "testing"

func TestEmptyCell(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"plain text", "hello", ""},
		{"with newline", "hello\n", "\n"},
		{"with form feed", "hi\f", "\f"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := emptyCell(tt.in); got != tt.want {
				t.Errorf("emptyCell(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
