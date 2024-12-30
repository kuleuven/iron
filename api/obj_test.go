package api

import (
	"context"
	"testing"

	"gitea.icts.kuleuven.be/coz/iron/msg"
)

func TestObj(t *testing.T) {
	testConn.NextResponse = msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 5,
		TotalRowCount:  1,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 500, ResultLen: 1, Values: []string{"1"}},
			{AttributeIndex: 501, ResultLen: 1, Values: []string{"coll_name"}},
			{AttributeIndex: 503, ResultLen: 1, Values: []string{"rods"}},
			{AttributeIndex: 508, ResultLen: 1, Values: []string{"10000"}},
			{AttributeIndex: 509, ResultLen: 1, Values: []string{"1"}},
		},
	}

	_, err := testAPI.GetCollection(context.Background(), "/test")
	if err != nil {
		t.Fatal(err)
	}
}
