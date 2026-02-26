package api

import (
	"testing"

	"github.com/kuleuven/iron/msg"
)

func TestGenericResultRows(t *testing.T) {
	gr := &GenericResult{
		rows: [][]string{
			{"a", "b"},
			{"c", "d"},
		},
	}

	rows := gr.Rows()
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	if rows[0][0] != "a" || rows[1][1] != "d" {
		t.Errorf("unexpected row values: %v", rows)
	}
}

func TestGenericResultErr(t *testing.T) {
	gr := &GenericResult{}
	if gr.Err() != nil {
		t.Error("expected nil error for empty result")
	}

	gr2 := &GenericResult{err: ErrNoRowFound}
	if gr2.Err() != ErrNoRowFound {
		t.Errorf("expected ErrNoRowFound, got %v", gr2.Err())
	}
}

func TestGenericResultNextEmpty(t *testing.T) {
	gr := &GenericResult{rows: [][]string{}}

	if gr.Next() {
		t.Error("expected Next() to return false for empty result")
	}
}

func TestGenericResultScanError(t *testing.T) {
	gr := &GenericResult{err: ErrNoRowFound}

	var s string

	err := gr.Scan(&s)
	if err != ErrNoRowFound {
		t.Errorf("expected ErrNoRowFound, got %v", err)
	}
}

func TestGenericResultScanNilRow(t *testing.T) {
	gr := &GenericResult{}

	var s string

	err := gr.Scan(&s)
	if err != ErrRowOutOfBound {
		t.Errorf("expected ErrRowOutOfBound, got %v", err)
	}
}

func TestGenericSingleRowResultRow(t *testing.T) {
	grr := &GenericSingleRowResult{
		row: []string{"hello", "42"},
	}

	row := grr.Row()
	if len(row) != 2 || row[0] != "hello" {
		t.Errorf("unexpected row: %v", row)
	}
}

func TestGenericSingleRowResultErr(t *testing.T) {
	grr := &GenericSingleRowResult{}
	if grr.Err() != nil {
		t.Error("expected nil error")
	}

	grr2 := &GenericSingleRowResult{err: ErrNoRowFound}
	if grr2.Err() != ErrNoRowFound {
		t.Errorf("expected ErrNoRowFound, got %v", grr2.Err())
	}
}

func TestGenericSingleRowResultScanError(t *testing.T) {
	grr := &GenericSingleRowResult{err: ErrNoRowFound}

	var s string

	err := grr.Scan(&s)
	if err != ErrNoRowFound {
		t.Errorf("expected ErrNoRowFound, got %v", err)
	}
}

func TestGenericSingleRowResultScanTooManyDest(t *testing.T) {
	grr := &GenericSingleRowResult{
		row: []string{"hello"},
	}

	var a, b string

	err := grr.Scan(&a, &b)
	if err != ErrAttributeOutOfBound {
		t.Errorf("expected ErrAttributeOutOfBound, got %v", err)
	}
}

func TestGenericResultClose(t *testing.T) {
	gr := &GenericResult{
		rows: [][]string{{"a"}},
	}

	if err := gr.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gr.rows != nil {
		t.Error("expected rows to be nil after Close")
	}
}

func TestGenericQueryRowNoRows(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.String{
		String: `[]`,
	})

	result := testAPI.GenericQueryRow("SELECT DATA_NAME").Execute(t.Context())

	if result.Err() != ErrNoRowFound {
		t.Errorf("expected ErrNoRowFound, got %v", result.Err())
	}
}

func TestGenericQueryRowScan(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.String{
		String: `[["hello", "42"]]`,
	})

	result := testAPI.GenericQueryRow("SELECT DATA_NAME, DATA_ID").Execute(t.Context())

	var (
		name string
		id   int64
	)

	if err := result.Scan(&name, &id); err != nil {
		t.Fatal(err)
	}

	if name != "hello" || id != 42 {
		t.Errorf("expected (hello, 42), got (%s, %d)", name, id)
	}
}

func TestGenericResultIteration(t *testing.T) {
	gr := &GenericResult{
		rows: [][]string{
			{"a", "1"},
			{"b", "2"},
			{"c", "3"},
		},
	}

	var count int
	for gr.Next() {
		count++

		var (
			s string
			i int64
		)

		if err := gr.Scan(&s, &i); err != nil {
			t.Fatal(err)
		}
	}

	if count != 3 {
		t.Errorf("expected 3 iterations, got %d", count)
	}

	if gr.Err() != nil {
		t.Errorf("unexpected error: %v", gr.Err())
	}
}

func TestErrorCodeNilError(t *testing.T) {
	code, ok := ErrorCode(nil)
	if ok {
		t.Error("expected ok=false for nil error")
	}

	if code != 0 {
		t.Errorf("expected code=0, got %d", code)
	}
}
