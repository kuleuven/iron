package api

import (
	"testing"
	"time"

	"github.com/kuleuven/iron/msg"
)

func TestQuery(t *testing.T) {
	resp := msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 6,
		TotalRowCount:  1,
		ContinueIndex:  1,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 500, ResultLen: 1, Values: []string{"1"}},
			{AttributeIndex: 501, ResultLen: 1, Values: []string{"coll_name"}},
			{AttributeIndex: 508, ResultLen: 1, Values: []string{"10000"}},
			{AttributeIndex: 404, ResultLen: 1, Values: []string{"1"}},
			{AttributeIndex: 998, ResultLen: 1, Values: []string{"5.5"}},
			{AttributeIndex: 999, ResultLen: 1, Values: []string{"0"}},
		},
	}

	testAPI := newAPI()

	testAPI.AddResponses([]any{resp, resp})

	resp.ContinueIndex = 0

	testAPI.AddResponse(resp)

	results := testAPI.AsAdmin().Query(msg.ICAT_COLUMN_COLL_ID, msg.ICAT_COLUMN_COLL_NAME, msg.ICAT_COLUMN_COLL_CREATE_TIME, msg.ICAT_COLUMN_DATA_REPL_NUM, 998, 999).With(Equal(msg.ICAT_COLUMN_COLL_ID, 1)).Limit(2).Execute(t.Context())

	var items int

	for results.Next() {
		var (
			collID   int64
			collName string
			collTime time.Time
			uintr    uint16
			floatr   float64
			boolr    bool
		)

		if err := results.Scan(&collID, &collName, &collTime, &uintr, &floatr, &boolr); err != nil {
			t.Fatal(err)
		}

		if err := results.Scan(&collID, &collName, &collTime, &uintr, &floatr, &boolr, &boolr); err != ErrAttributeOutOfBound {
			t.Fatalf("expected %s, got %v", ErrAttributeOutOfBound, err)
		}

		items++
	}

	if err := results.Err(); err != nil {
		t.Fatal(err)
	}

	if items != 2 {
		t.Fatalf("expected 1 item, got %d", items)
	}

	if err := results.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestParseTime(t *testing.T) {
	_, err := parseTime("9999")
	if err != nil {
		t.Fatal(err)
	}

	_, err = parseTime("bad")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestQueryRow(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 6,
		TotalRowCount:  1,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 500, ResultLen: 1, Values: []string{"1"}},
			{AttributeIndex: 501, ResultLen: 1, Values: []string{"coll_name"}},
			{AttributeIndex: 508, ResultLen: 1, Values: []string{"10000"}},
			{AttributeIndex: 404, ResultLen: 1, Values: []string{"1"}},
			{AttributeIndex: 998, ResultLen: 1, Values: []string{"5.5"}},
			{AttributeIndex: 999, ResultLen: 1, Values: []string{"0"}},
		},
	})

	results := testAPI.QueryRow(msg.ICAT_COLUMN_COLL_ID, msg.ICAT_COLUMN_COLL_NAME, msg.ICAT_COLUMN_COLL_CREATE_TIME, msg.ICAT_COLUMN_DATA_REPL_NUM, 998, 999).Where(msg.ICAT_COLUMN_COLL_ID, "= '1'").Execute(t.Context())

	var (
		collID   int64
		collName string
		collTime time.Time
		uintr    uint16
		floatr   float64
		boolr    bool
	)

	if err := results.Scan(&collID, &collName, &collTime, &uintr, &floatr, &boolr); err != nil {
		t.Fatal(err)
	}
}
