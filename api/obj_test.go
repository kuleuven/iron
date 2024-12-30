package api

import (
	"context"
	"testing"

	"gitea.icts.kuleuven.be/coz/iron/msg"
)

func TestGetCollection(t *testing.T) {
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

func TestGetDataObject(t *testing.T) {
	testConn.NextResponse = msg.QueryResponse{
		RowCount:       2,
		AttributeCount: 13,
		TotalRowCount:  2,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 401, ResultLen: 2, Values: []string{"1", "1"}},
			{AttributeIndex: 500, ResultLen: 2, Values: []string{"1", "1"}},
			{AttributeIndex: 407, ResultLen: 2, Values: []string{"1024000", "1024000"}},
			{AttributeIndex: 406, ResultLen: 2, Values: []string{"generic", "generic"}},
			{AttributeIndex: 404, ResultLen: 2, Values: []string{"0", "1"}},
			{AttributeIndex: 411, ResultLen: 2, Values: []string{"rods", "rods"}},
			{AttributeIndex: 415, ResultLen: 2, Values: []string{"checksum", "checksum"}},
			{AttributeIndex: 413, ResultLen: 2, Values: []string{"", ""}},
			{AttributeIndex: 409, ResultLen: 2, Values: []string{"resc1", "resc2"}},
			{AttributeIndex: 410, ResultLen: 2, Values: []string{"/path1", "/path2"}},
			{AttributeIndex: 422, ResultLen: 2, Values: []string{"demoResc;resc1", "demoResc;resc2"}},
			{AttributeIndex: 419, ResultLen: 2, Values: []string{"10000", "10000"}},
			{AttributeIndex: 420, ResultLen: 2, Values: []string{"10000", "10000"}},
		},
	}

	_, err := testAPI.GetDataObject(context.Background(), "/test/test")
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetResource(t *testing.T) {
	testConn.NextResponse = msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 10,
		TotalRowCount:  1,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 301, ResultLen: 1, Values: []string{"1"}},
			{AttributeIndex: 302, ResultLen: 1, Values: []string{"resc_name"}},
			{AttributeIndex: 303, ResultLen: 1, Values: []string{"testZone"}},
			{AttributeIndex: 304, ResultLen: 1, Values: []string{"posix"}},
			{AttributeIndex: 305, ResultLen: 1, Values: []string{"class"}},
			{AttributeIndex: 306, ResultLen: 1, Values: []string{"server"}},
			{AttributeIndex: 307, ResultLen: 1, Values: []string{"/path/to/resc"}},
			{AttributeIndex: 316, ResultLen: 1, Values: []string{"context string"}},
			{AttributeIndex: 311, ResultLen: 1, Values: []string{"10000"}},
			{AttributeIndex: 312, ResultLen: 1, Values: []string{"10000"}},
		},
	}

	_, err := testAPI.GetResource(context.Background(), "/test")
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetUser(t *testing.T) {
	testConn.NextResponse = msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 6,
		TotalRowCount:  1,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 201, ResultLen: 1, Values: []string{"1"}},
			{AttributeIndex: 202, ResultLen: 1, Values: []string{"username"}},
			{AttributeIndex: 203, ResultLen: 1, Values: []string{"rodsuser"}},
			{AttributeIndex: 204, ResultLen: 1, Values: []string{"testZone"}},
			{AttributeIndex: 208, ResultLen: 1, Values: []string{"10000"}},
			{AttributeIndex: 209, ResultLen: 1, Values: []string{"10000"}},
		},
	}

	_, err := testAPI.GetUser(context.Background(), "/test")
	if err != nil {
		t.Fatal(err)
	}
}

func TestListSubCollections(t *testing.T) {
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

	_, err := testAPI.ListSubCollections(context.Background(), "/test")
	if err != nil {
		t.Fatal(err)
	}
}

func TestListDataObjects(t *testing.T) {
	testConn.NextResponse = msg.QueryResponse{
		RowCount:       2,
		AttributeCount: 13,
		TotalRowCount:  2,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 401, ResultLen: 2, Values: []string{"1", "1"}},
			{AttributeIndex: 500, ResultLen: 2, Values: []string{"1", "1"}},
			{AttributeIndex: 407, ResultLen: 2, Values: []string{"1024000", "1024000"}},
			{AttributeIndex: 406, ResultLen: 2, Values: []string{"generic", "generic"}},
			{AttributeIndex: 404, ResultLen: 2, Values: []string{"0", "1"}},
			{AttributeIndex: 411, ResultLen: 2, Values: []string{"rods", "rods"}},
			{AttributeIndex: 415, ResultLen: 2, Values: []string{"checksum", "checksum"}},
			{AttributeIndex: 413, ResultLen: 2, Values: []string{"", ""}},
			{AttributeIndex: 409, ResultLen: 2, Values: []string{"resc1", "resc2"}},
			{AttributeIndex: 410, ResultLen: 2, Values: []string{"/path1", "/path2"}},
			{AttributeIndex: 422, ResultLen: 2, Values: []string{"demoResc;resc1", "demoResc;resc2"}},
			{AttributeIndex: 419, ResultLen: 2, Values: []string{"10000", "10000"}},
			{AttributeIndex: 420, ResultLen: 2, Values: []string{"10000", "10000"}},
		},
	}

	_, err := testAPI.ListDataObjects(context.Background(), "/test")
	if err != nil {
		t.Fatal(err)
	}
}
