package api

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/kuleuven/iron/msg"
)

func TestGetCollection(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 6,
		TotalRowCount:  1,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 500, ResultLen: 1, Values: []string{"1"}},
			{AttributeIndex: 503, ResultLen: 1, Values: []string{"rods"}},
			{AttributeIndex: 504, ResultLen: 1, Values: []string{"zone"}},
			{AttributeIndex: 508, ResultLen: 1, Values: []string{"10000"}},
			{AttributeIndex: 509, ResultLen: 1, Values: []string{"1"}},
			{AttributeIndex: 506, ResultLen: 1, Values: []string{"1"}},
		},
	})

	obj, err := testAPI.GetCollection(context.Background(), "/test/coll_name")
	if err != nil {
		t.Fatal(err)
	}

	if !obj.IsDir() {
		t.Error("object should be a dir")
	}

	if obj.Name() != "coll_name" {
		t.Errorf("object name should be %s, but is %s", "coll_name", obj.Name())
	}

	if obj.Size() != 0 {
		t.Errorf("object size should be %d, but is %d", 0, obj.Size())
	}

	if obj.Identifier() != 1 {
		t.Errorf("object id should be %d, but is %d", 1, obj.Identifier())
	}

	if expected := "/test/coll_name"; obj.Path != expected {
		t.Errorf("object path should be %s, but is %s", expected, obj.Path)
	}

	if obj.Mode() != os.FileMode(0o750)|os.ModeDir|os.ModeSetgid {
		t.Errorf("object mode should be %o, but is %o", os.FileMode(0o750)|os.ModeDir, obj.Mode())
	}

	if obj.ModTime() != time.Unix(1, 0) {
		t.Errorf("object modified time should be %s, but is %s", time.Unix(10000, 0), obj.ModTime())
	}
}

func TestGetDataObject(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.QueryResponse{
		RowCount:       2,
		AttributeCount: 14,
		TotalRowCount:  2,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 401, ResultLen: 2, Values: []string{"1", "1"}},
			{AttributeIndex: 500, ResultLen: 2, Values: []string{"1", "1"}},
			{AttributeIndex: 406, ResultLen: 2, Values: []string{"generic", "generic"}},
			{AttributeIndex: 404, ResultLen: 2, Values: []string{"0", "1"}},
			{AttributeIndex: 407, ResultLen: 2, Values: []string{"1024000", "1024000"}},
			{AttributeIndex: 411, ResultLen: 2, Values: []string{"rods", "rods"}},
			{AttributeIndex: 412, ResultLen: 1, Values: []string{"zone", "zone"}},
			{AttributeIndex: 415, ResultLen: 2, Values: []string{"checksum", "checksum"}},
			{AttributeIndex: 413, ResultLen: 2, Values: []string{"", ""}},
			{AttributeIndex: 409, ResultLen: 2, Values: []string{"resc1", "resc2"}},
			{AttributeIndex: 410, ResultLen: 2, Values: []string{"/path1", "/path2"}},
			{AttributeIndex: 422, ResultLen: 2, Values: []string{"demoResc;resc1", "demoResc;resc2"}},
			{AttributeIndex: 419, ResultLen: 2, Values: []string{"10000", "10000"}},
			{AttributeIndex: 420, ResultLen: 2, Values: []string{"10000", "10000"}},
		},
	})

	obj, err := testAPI.GetDataObject(context.Background(), "/test/test")
	if err != nil {
		t.Fatal(err)
	}

	if obj.IsDir() {
		t.Error("object should not be a dir")
	}

	if obj.Name() != "test" {
		t.Errorf("object name should be %s, but is %s", "test", obj.Name())
	}

	if obj.Size() != 1024000 {
		t.Errorf("object size should be %d, but is 7%d", 1024000, obj.Size())
	}

	if obj.Identifier() != 1 {
		t.Errorf("object id should be %d, but is %d", 1, obj.Identifier())
	}

	if expected := "/test/test"; obj.Path != expected {
		t.Errorf("object path should be %s, but is %s", expected, obj.Path)
	}

	if obj.Mode() != os.FileMode(0o640) {
		t.Errorf("object mode should be %o, but is %o", os.FileMode(0o640), obj.Mode())
	}

	if obj.ModTime() != time.Unix(10000, 0) {
		t.Errorf("object modified time should be %s, but is %s", time.Unix(10000, 0), obj.ModTime())
	}
}

func TestGetResource(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 11,
		TotalRowCount:  1,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 301, ResultLen: 1, Values: []string{"1"}},
			{AttributeIndex: 317, ResultLen: 1, Values: []string{"1"}},
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
	})

	_, err := testAPI.GetResource(context.Background(), "/test")
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetUser(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 6,
		TotalRowCount:  1,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 201, ResultLen: 1, Values: []string{"1"}},
			{AttributeIndex: 202, ResultLen: 1, Values: []string{"username"}},
			{AttributeIndex: 204, ResultLen: 1, Values: []string{"testZone"}},
			{AttributeIndex: 203, ResultLen: 1, Values: []string{"rodsuser"}},
			{AttributeIndex: 208, ResultLen: 1, Values: []string{"10000"}},
			{AttributeIndex: 209, ResultLen: 1, Values: []string{"10000"}},
		},
	})

	_, err := testAPI.GetUser(context.Background(), "/test")
	if err != nil {
		t.Fatal(err)
	}
}

func TestListUsers(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 6,
		TotalRowCount:  1,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 201, ResultLen: 1, Values: []string{"1"}},
			{AttributeIndex: 202, ResultLen: 1, Values: []string{"username"}},
			{AttributeIndex: 204, ResultLen: 1, Values: []string{"testZone"}},
			{AttributeIndex: 203, ResultLen: 1, Values: []string{"rodsuser"}},
			{AttributeIndex: 208, ResultLen: 1, Values: []string{"10000"}},
			{AttributeIndex: 209, ResultLen: 1, Values: []string{"10000"}},
		},
	})

	list, err := testAPI.ListUsers(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 1 {
		t.Errorf("expected 1 user, got %d", len(list))
	}
}

func TestListResources(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 11,
		TotalRowCount:  1,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 301, ResultLen: 1, Values: []string{"1"}},
			{AttributeIndex: 317, ResultLen: 1, Values: []string{"1"}},
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
	})

	list, err := testAPI.ListResources(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 1 {
		t.Errorf("expected 1 user, got %d", len(list))
	}
}

func TestListSubCollections(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 7,
		TotalRowCount:  1,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 500, ResultLen: 1, Values: []string{"1"}},
			{AttributeIndex: 501, ResultLen: 1, Values: []string{"coll_name"}},
			{AttributeIndex: 503, ResultLen: 1, Values: []string{"rods"}},
			{AttributeIndex: 504, ResultLen: 1, Values: []string{"zone"}},
			{AttributeIndex: 508, ResultLen: 1, Values: []string{"10000"}},
			{AttributeIndex: 509, ResultLen: 1, Values: []string{"1"}},
			{AttributeIndex: 506, ResultLen: 1, Values: []string{"0"}},
		},
	})

	_, err := testAPI.ListSubCollections(context.Background(), "/test")
	if err != nil {
		t.Fatal(err)
	}
}

func TestListDataObjects(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.QueryResponse{
		RowCount:       2,
		AttributeCount: 16,
		TotalRowCount:  2,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 401, ResultLen: 2, Values: []string{"1", "1"}},
			{AttributeIndex: 501, ResultLen: 2, Values: []string{"/test", "/test"}},
			{AttributeIndex: 403, ResultLen: 2, Values: []string{"obj_name", "obj_name"}},
			{AttributeIndex: 500, ResultLen: 2, Values: []string{"1", "1"}},
			{AttributeIndex: 406, ResultLen: 2, Values: []string{"generic", "generic"}},
			{AttributeIndex: 404, ResultLen: 2, Values: []string{"0", "1"}},
			{AttributeIndex: 407, ResultLen: 2, Values: []string{"1024000", "1024000"}},
			{AttributeIndex: 411, ResultLen: 2, Values: []string{"rods", "rods"}},
			{AttributeIndex: 412, ResultLen: 1, Values: []string{"zone", "zone"}},
			{AttributeIndex: 415, ResultLen: 2, Values: []string{"checksum", "checksum"}},
			{AttributeIndex: 413, ResultLen: 2, Values: []string{"", ""}},
			{AttributeIndex: 409, ResultLen: 2, Values: []string{"resc1", "resc2"}},
			{AttributeIndex: 410, ResultLen: 2, Values: []string{"/path1", "/path2"}},
			{AttributeIndex: 422, ResultLen: 2, Values: []string{"demoResc;resc1", "demoResc;resc2"}},
			{AttributeIndex: 419, ResultLen: 2, Values: []string{"10000", "10000"}},
			{AttributeIndex: 420, ResultLen: 2, Values: []string{"10000", "10000"}},
		},
	})

	_, err := testAPI.ListDataObjectsInCollection(context.Background(), "/test")
	if err != nil {
		t.Fatal(err)
	}
}

func TestListMetadataDataObject(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.QueryResponse{
		RowCount:       2,
		AttributeCount: 3,
		TotalRowCount:  2,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 600, ResultLen: 2, Values: []string{"key", "key"}},
			{AttributeIndex: 601, ResultLen: 2, Values: []string{"value", "1"}},
			{AttributeIndex: 602, ResultLen: 2, Values: []string{"unit", ""}},
		},
	})

	meta, err := testAPI.ListMetadata(context.Background(), "/test/object", DataObjectType)
	if err != nil {
		t.Fatal(err)
	}

	if len(meta) != 2 {
		t.Errorf("metadata size should be %d, but is %d", 2, len(meta))
	}
}

func TestListMetadataCollection(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.QueryResponse{
		RowCount:       2,
		AttributeCount: 3,
		TotalRowCount:  2,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 610, ResultLen: 2, Values: []string{"key", "key"}},
			{AttributeIndex: 611, ResultLen: 2, Values: []string{"value", "1"}},
			{AttributeIndex: 612, ResultLen: 2, Values: []string{"unit", ""}},
		},
	})

	meta, err := testAPI.ListMetadata(context.Background(), "/test", CollectionType)
	if err != nil {
		t.Fatal(err)
	}

	if len(meta) != 2 {
		t.Errorf("metadata size should be %d, but is %d", 2, len(meta))
	}
}

func TestListMetadataResource(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.QueryResponse{
		RowCount:       2,
		AttributeCount: 3,
		TotalRowCount:  2,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 630, ResultLen: 2, Values: []string{"key", "key"}},
			{AttributeIndex: 631, ResultLen: 2, Values: []string{"value", "1"}},
			{AttributeIndex: 632, ResultLen: 2, Values: []string{"unit", ""}},
		},
	})

	meta, err := testAPI.ListMetadata(context.Background(), "test", ResourceType)
	if err != nil {
		t.Fatal(err)
	}

	if len(meta) != 2 {
		t.Errorf("metadata size should be %d, but is %d", 2, len(meta))
	}
}

func TestListMetadataUser(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.QueryResponse{
		RowCount:       2,
		AttributeCount: 3,
		TotalRowCount:  2,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 640, ResultLen: 2, Values: []string{"key", "key"}},
			{AttributeIndex: 641, ResultLen: 2, Values: []string{"value", "1"}},
			{AttributeIndex: 642, ResultLen: 2, Values: []string{"unit", ""}},
		},
	})

	meta, err := testAPI.ListMetadata(context.Background(), "test", UserType)
	if err != nil {
		t.Fatal(err)
	}

	if len(meta) != 2 {
		t.Errorf("metadata size should be %d, but is %d", 2, len(meta))
	}
}

func TestListAccessCollection(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponses([]any{
		msg.QueryResponse{
			RowCount:       2,
			AttributeCount: 2,
			TotalRowCount:  2,
			ContinueIndex:  0,
			SQLResult: []msg.SQLResult{
				{AttributeIndex: 711, ResultLen: 2, Values: []string{"own", "read_object"}},
				{AttributeIndex: 713, ResultLen: 2, Values: []string{"1", "2"}},
			},
		},
		msg.QueryResponse{
			RowCount:       1,
			AttributeCount: 6,
			TotalRowCount:  1,
			ContinueIndex:  0,
			SQLResult: []msg.SQLResult{
				{AttributeIndex: 201, ResultLen: 1, Values: []string{"1"}},
				{AttributeIndex: 202, ResultLen: 1, Values: []string{"username"}},
				{AttributeIndex: 204, ResultLen: 1, Values: []string{"testZone"}},
				{AttributeIndex: 203, ResultLen: 1, Values: []string{"rodsuser"}},
				{AttributeIndex: 208, ResultLen: 1, Values: []string{"10000"}},
				{AttributeIndex: 209, ResultLen: 1, Values: []string{"10000"}},
			},
		},
	})

	_, err := testAPI.ListAccess(context.Background(), "/test/test", CollectionType)
	if err != nil {
		t.Fatal(err)
	}
}

func TestListAccessDataObject(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponses([]any{
		msg.QueryResponse{
			RowCount:       2,
			AttributeCount: 2,
			TotalRowCount:  2,
			ContinueIndex:  0,
			SQLResult: []msg.SQLResult{
				{AttributeIndex: 701, ResultLen: 2, Values: []string{"own", "read_object"}},
				{AttributeIndex: 703, ResultLen: 2, Values: []string{"1", "2"}},
			},
		},
		msg.QueryResponse{
			RowCount:       1,
			AttributeCount: 6,
			TotalRowCount:  1,
			ContinueIndex:  0,
			SQLResult: []msg.SQLResult{
				{AttributeIndex: 201, ResultLen: 1, Values: []string{"1"}},
				{AttributeIndex: 202, ResultLen: 1, Values: []string{"username"}},
				{AttributeIndex: 204, ResultLen: 1, Values: []string{"testZone"}},
				{AttributeIndex: 203, ResultLen: 1, Values: []string{"rodsuser"}},
				{AttributeIndex: 208, ResultLen: 1, Values: []string{"10000"}},
				{AttributeIndex: 209, ResultLen: 1, Values: []string{"10000"}},
			},
		},
	})

	_, err := testAPI.ListAccess(context.Background(), "/test/test", DataObjectType)
	if err != nil {
		t.Fatal(err)
	}
}
