package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kuleuven/iron/api"
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
		{testOwn, testOwn},
		{"read object", testRead},
		{"read_object", testRead},
		{"write object", testWrite},
		{"write_object", testWrite},
		{"modify_object", testWrite},
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

type fakeRecord struct {
	name string
	sys  any
}

func (f *fakeRecord) Name() string             { return f.name }
func (f *fakeRecord) Size() int64              { return 0 }
func (f *fakeRecord) Mode() os.FileMode        { return 0 }
func (f *fakeRecord) ModTime() time.Time       { return time.Unix(0, 0) }
func (f *fakeRecord) IsDir() bool              { return false }
func (f *fakeRecord) Sys() any                 { return f.sys }
func (f *fakeRecord) Metadata() []api.Metadata { return nil }
func (f *fakeRecord) Access() []api.Access     { return nil }
func (f *fakeRecord) Type() api.ObjectType     { return api.DataObjectType }

func TestTablePrinterReplicaColumn(t *testing.T) {
	var buf bytes.Buffer

	tp := &TablePrinter{Writer: &tabwriter.TabWriter{Writer: &buf}}
	tp.Setup(false, false, false, true)

	obj := &api.DataObject{
		Replicas: []api.Replica{
			{Number: 0, Status: "1", ResourceHierarchy: testDemoRescResc1},
			{Number: 1, Status: "1", ResourceHierarchy: testDemoRescResc2},
		},
	}

	tp.Print("data.txt", &fakeRecord{name: "data.txt", sys: obj})
	tp.Flush()

	out := buf.String()

	if !strings.Contains(out, "RESOURCE") {
		t.Errorf("expected RESOURCE header, got:\n%s", out)
	}

	for _, want := range []string{testDemoRescResc1, testDemoRescResc2} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestTablePrinterNoReplicaColumn(t *testing.T) {
	var buf bytes.Buffer

	tp := &TablePrinter{Writer: &tabwriter.TabWriter{Writer: &buf}}
	tp.Setup(false, false, false, false)

	obj := &api.DataObject{
		Replicas: []api.Replica{
			{Number: 0, Status: "1", ResourceHierarchy: testDemoRescResc1},
		},
	}

	tp.Print("data.txt", &fakeRecord{name: "data.txt", sys: obj})
	tp.Flush()

	out := buf.String()

	if strings.Contains(out, "RESOURCE") || strings.Contains(out, testDemoRescResc1) {
		t.Errorf("did not expect resource column without --replica, got:\n%s", out)
	}
}
