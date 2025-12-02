package api

import (
	"testing"

	"github.com/kuleuven/iron/msg"
)

func TestProcs(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.QueryResponse{
		AttributeCount: 9,
		RowCount:       1,
		ContinueIndex:  -1,
		TotalRowCount:  1,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 1000001, ResultLen: 1, Values: []string{"10"}},
			{AttributeIndex: 1000002, ResultLen: 1, Values: []string{"1764600000"}},
			{AttributeIndex: 1000003, ResultLen: 1, Values: []string{"user"}},
			{AttributeIndex: 1000004, ResultLen: 1, Values: []string{"zone"}},
			{AttributeIndex: 1000005, ResultLen: 1, Values: []string{"user"}},
			{AttributeIndex: 1000006, ResultLen: 1, Values: []string{"zone"}},
			{AttributeIndex: 1000007, ResultLen: 1, Values: []string{"1.2.3.4"}},
			{AttributeIndex: 1000008, ResultLen: 1, Values: []string{"example.org"}},
			{AttributeIndex: 1000009, ResultLen: 1, Values: []string{"iron"}},
		},
	})

	result := testAPI.Procs(t.Context())

	for result.Next() {
		if err := result.Scan(); err != nil {
			t.Error(err)
		}
	}

	if err := result.Err(); err != nil {
		t.Error(err)
	}
}
