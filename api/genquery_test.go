package api

import (
	"testing"

	"github.com/kuleuven/iron/msg"
)

func TestGenericQuerySQL(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.String{
		String: `SELECT DATA_NAME, DATA_ID FROM R_DATA_MAIN`,
	})

	query, err := testAPI.GenericQuery("SELECT DATA_NAME, DATA_ID").SQL(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if query != `SELECT DATA_NAME, DATA_ID FROM R_DATA_MAIN` {
		t.Errorf("expected %s, got %s", `SELECT DATA_NAME, DATA_ID FROM R_DATA_MAIN`, query)
	}
}

func TestGenericQuery(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.String{
		String: `[["test", "1"]]`,
	})

	results := testAPI.GenericQuery("SELECT DATA_NAME, DATA_ID").Execute(t.Context())

	var items int

	for results.Next() {
		var (
			dataName string
			dataID   int64
		)

		if err := results.Scan(&dataName, &dataID); err != nil {
			t.Fatal(err)
		}

		if err := results.Scan(&dataName, &dataID, &dataID); err != ErrAttributeOutOfBound {
			t.Fatalf("expected %s, got %v", ErrAttributeOutOfBound, err)
		}

		items++
	}

	if err := results.Err(); err != nil {
		t.Fatal(err)
	}

	if items != 1 {
		t.Fatalf("expected 1 item, got %d", items)
	}

	if err := results.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestGenericQueryColumns(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.String{
		String: `{"DATA_NAME":{"R_DATA_MAIN":"data_name"}}`,
	})

	results, err := testAPI.GenericQueryColumns(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 item, got %d", len(results))
	}
}

func TestGenericQueryRow(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.String{
		String: `[["test", "1"]]`,
	})

	result := testAPI.GenericQueryRow("SELECT DATA_NAME, DATA_ID").Execute(t.Context())

	var (
		dataName string
		dataID   int64
	)

	if err := result.Scan(&dataName, &dataID); err != nil {
		t.Fatal(err)
	}
}
