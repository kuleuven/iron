package api

import (
	"context"
	"testing"

	"gitea.icts.kuleuven.be/coz/iron/msg"
)

var responses = []any{
	msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 4,
		TotalRowCount:  1,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 500, ResultLen: 1, Values: []string{"1"}},
			{AttributeIndex: 503, ResultLen: 1, Values: []string{"/test"}},
			{AttributeIndex: 508, ResultLen: 1, Values: []string{"10000"}},
			{AttributeIndex: 509, ResultLen: 1, Values: []string{"2024"}},
		},
	},
	msg.QueryResponse{
		RowCount:       2,
		AttributeCount: 5,
		TotalRowCount:  2,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 500, ResultLen: 1, Values: []string{"2", "3"}},
			{AttributeIndex: 501, ResultLen: 1, Values: []string{"/test/test'2", "/test/test3"}},
			{AttributeIndex: 503, ResultLen: 1, Values: []string{"rods", "user"}},
			{AttributeIndex: 508, ResultLen: 1, Values: []string{"10000", "10000"}},
			{AttributeIndex: 509, ResultLen: 1, Values: []string{"2024", "2025"}},
		},
	},
	msg.QueryResponse{
		RowCount:       2,
		AttributeCount: 14,
		TotalRowCount:  2,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 401, ResultLen: 2, Values: []string{"4", "4"}},
			{AttributeIndex: 403, ResultLen: 2, Values: []string{"file1", "file1"}},
			{AttributeIndex: 402, ResultLen: 2, Values: []string{"1", "1"}},
			{AttributeIndex: 406, ResultLen: 2, Values: []string{"generic", "generic"}},
			{AttributeIndex: 404, ResultLen: 2, Values: []string{"0", "1"}},
			{AttributeIndex: 407, ResultLen: 2, Values: []string{"1024000", "1024000"}},
			{AttributeIndex: 411, ResultLen: 2, Values: []string{"rods", "rods"}},
			{AttributeIndex: 415, ResultLen: 2, Values: []string{"checksum", "checksum"}},
			{AttributeIndex: 413, ResultLen: 2, Values: []string{"", ""}},
			{AttributeIndex: 409, ResultLen: 2, Values: []string{"resc1", "resc2"}},
			{AttributeIndex: 410, ResultLen: 2, Values: []string{"/path1", "/path2"}},
			{AttributeIndex: 422, ResultLen: 2, Values: []string{"demoResc;resc1", "demoResc;resc2"}},
			{AttributeIndex: 419, ResultLen: 2, Values: []string{"10000", "10000"}},
			{AttributeIndex: 420, ResultLen: 2, Values: []string{"10000", "10000"}},
		},
	},
	msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 3,
		TotalRowCount:  1,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 711, ResultLen: 2, Values: []string{"own"}},
			{AttributeIndex: 713, ResultLen: 2, Values: []string{"500"}},
			{AttributeIndex: 500, ResultLen: 2, Values: []string{"1"}},
		},
	},
	msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 6,
		TotalRowCount:  1,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 201, ResultLen: 1, Values: []string{"500"}},
			{AttributeIndex: 202, ResultLen: 1, Values: []string{"username"}},
			{AttributeIndex: 204, ResultLen: 1, Values: []string{"testZone"}},
			{AttributeIndex: 203, ResultLen: 1, Values: []string{"rodsuser"}},
			{AttributeIndex: 208, ResultLen: 1, Values: []string{"10000"}},
			{AttributeIndex: 209, ResultLen: 1, Values: []string{"10000"}},
		},
	},
	msg.QueryResponse{AttributeCount: 4},
	msg.QueryResponse{AttributeCount: 3},
	msg.QueryResponse{AttributeCount: 4},
	msg.QueryResponse{AttributeCount: 3},
	msg.QueryResponse{AttributeCount: 4},

	msg.QueryResponse{AttributeCount: 5},
	msg.QueryResponse{AttributeCount: 14},

	msg.QueryResponse{AttributeCount: 5},
	msg.QueryResponse{AttributeCount: 14},
}

func TestWalk(t *testing.T) {
	testConn.NextResponses = responses

	err := testAPI.Walk(context.Background(), "/test", func(path string, info Record, err error) error {
		return err
	}, FetchAccess, FetchMetadata)
	if err != nil {
		t.Fatal(err)
	}
}

func TestWalkSkip(t *testing.T) {
	testConn.NextResponses = responses

	err := testAPI.Walk(context.Background(), "/test", func(path string, info Record, err error) error {
		if path == "/test" {
			return SkipSubDirs
		}

		if path == "/test/test3" {
			return SkipDir
		}

		return err
	}, FetchAccess, FetchMetadata)
	if err != nil {
		t.Fatal(err)
	}
}

func TestWalkSkipAll(t *testing.T) {
	testConn.NextResponses = responses[:6]

	err := testAPI.Walk(context.Background(), "/test", func(path string, info Record, err error) error {
		return SkipAll
	}, FetchAccess, FetchMetadata)
	if err != nil {
		t.Fatal(err)
	}
}
