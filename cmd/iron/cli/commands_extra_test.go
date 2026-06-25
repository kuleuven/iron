package cli

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kuleuven/iron/api"
)

func TestLocalPathEndsWithSeparator(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/foo/", true},
		{"/foo", false},
		{"", false},
		{"plain", false},
	}

	for _, tt := range tests {
		if got := localPathEndsWithSeparator(tt.path); got != tt.want {
			t.Errorf("localPathEndsWithSeparator(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestHiddenColumnsSelection(t *testing.T) {
	defaults := []string{"a", "b", "c"}

	tests := []struct {
		name    string
		columns []string
		want    []string
	}{
		{"no changes", nil, []string{"a", "b", "c"}},
		{"add", []string{"+d"}, []string{"a", "b", "c", "d"}},
		{"remove", []string{"-b"}, []string{"a", "c"}},
		{"add and remove", []string{"-a", "+x"}, []string{"b", "c", "x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := append([]string(nil), defaults...)
			got := hiddenColumnsSelection(tt.columns, input)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("hiddenColumnsSelection(%v, %v) = %v, want %v", tt.columns, defaults, got, tt.want)
			}
		})
	}
}

type capturingPrinter struct {
	names []string
}

func (c *capturingPrinter) Setup(_, _, _, _ bool)           {}
func (c *capturingPrinter) Print(name string, _ api.Record) { c.names = append(c.names, name) }
func (c *capturingPrinter) Flush()                          {}

func TestFindFunc(t *testing.T) {
	captured := &capturingPrinter{}

	fn := findFunc(captured)

	if err := fn("/a", nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	boom := errors.New("boom")
	if fn("/x", nil, boom) != boom {
		t.Errorf("expected propagated error %v", boom)
	}

	if len(captured.names) != 1 || captured.names[0] != "/a" {
		t.Errorf("captured.names = %v, want [/a]", captured.names)
	}
}
