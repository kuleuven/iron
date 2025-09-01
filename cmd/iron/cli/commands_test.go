package cli

import (
	"testing"

	"github.com/kuleuven/iron/msg"
)

func TestMkdir(t *testing.T) {
	app := testApp(t)

	app.AddResponse(msg.EmptyResponse{})

	cmd := app.Command()
	cmd.SetArgs([]string{"mkdir", "testdir"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestRmdir(t *testing.T) {
	app := testApp(t)

	app.AddResponse(msg.CollectionOperationStat{})

	cmd := app.Command()
	cmd.SetArgs([]string{"rmdir", "testdir"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestTree(t *testing.T) {
	app := testApp(t)

	app.AddResponses(responses)

	app.AddResponse(msg.QueryResponse{})
	app.AddResponse(msg.QueryResponse{})

	cmd := app.Command()
	cmd.SetArgs([]string{"tree", "/testzone"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestList(t *testing.T) {
	app := testApp(t)

	app.AddResponses(responses)

	cmd := app.Command()
	cmd.SetArgs([]string{"ls", "/testzone"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestListExtra(t *testing.T) {
	app := testApp(t)

	app.AddResponses(responses)

	app.AddResponses([]any{
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
		msg.QueryResponse{},
		msg.QueryResponse{},
		msg.QueryResponse{},
		msg.QueryResponse{},
		msg.QueryResponse{},
	})

	cmd := app.Command()
	cmd.SetArgs([]string{"ls", "--acl", "--meta", "/testzone"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestListJSON(t *testing.T) {
	app := testApp(t)

	app.AddResponses(responses)

	cmd := app.Command()
	cmd.SetArgs([]string{"ls", "--json", "/testzone"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

var statResponses = []any{
	msg.QueryResponse{},
	msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 6,
		TotalRowCount:  1,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 500, ResultLen: 1, Values: []string{"1"}},
			{AttributeIndex: 503, ResultLen: 1, Values: []string{"/testzone/coll"}},
			{AttributeIndex: 504, ResultLen: 1, Values: []string{"zone"}},
			{AttributeIndex: 508, ResultLen: 1, Values: []string{"10000"}},
			{AttributeIndex: 509, ResultLen: 1, Values: []string{"2024"}},
			{AttributeIndex: 506, ResultLen: 1, Values: []string{"1"}},
		},
	},
	msg.QueryResponse{},
	msg.QueryResponse{},
	msg.QueryResponse{},
}

func TestStat(t *testing.T) {
	app := testApp(t)

	app.AddResponses(statResponses)

	cmd := app.Command()
	cmd.SetArgs([]string{"stat", "/testzone/coll"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestStatJSON(t *testing.T) {
	app := testApp(t)

	app.AddResponses(statResponses)

	cmd := app.Command()
	cmd.SetArgs([]string{"stat", "--json", "/testzone/coll"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestMv(t *testing.T) {
	app := testApp(t)

	app.AddResponses(statResponses[:2])
	app.AddResponse(msg.EmptyResponse{})

	cmd := app.Command()
	cmd.SetArgs([]string{"mv", "/testzone/coll", "/testzone/coll2"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestRm(t *testing.T) {
	app := testApp(t)

	app.AddResponses(statResponses[:2])
	app.AddResponse(msg.CollectionOperationStat{})

	cmd := app.Command()
	cmd.SetArgs([]string{"rm", "-r", "/testzone/coll"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestTouch(t *testing.T) {
	app := testApp(t)

	app.AddResponse(msg.FileDescriptor(1))
	app.AddResponse(msg.GetDescriptorInfoResponse{
		DataObjectInfo: map[string]any{
			"replica_number":     1,
			"resource_hierarchy": "string",
		},
	})
	app.AddResponse(msg.EmptyResponse{})
	app.AddResponse(msg.EmptyResponse{})

	cmd := app.Command()
	cmd.SetArgs([]string{"touch", "/testzone/obj1"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestChecksum(t *testing.T) {
	app := testApp(t)

	app.AddResponse(msg.Checksum{
		Checksum: "sha2:aaaa",
	})

	cmd := app.Command()
	cmd.SetArgs([]string{"checksum", "/testzone/obj1"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestPWD(t *testing.T) {
	app := testApp(t)

	cmd := app.pwd()

	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestLocal(t *testing.T) {
	app := testApp(t)

	cmd := app.local()

	cmd.SetArgs([]string{"pwd"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	cmd.SetArgs([]string{"cd", "."})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}
