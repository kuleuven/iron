package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/kuleuven/iron/msg"
	"github.com/kuleuven/iron/transfer"
)

func TestVersion(t *testing.T) {
	app := testApp(t)

	cmd := app.Command()
	cmd.SetArgs([]string{versionCommandName})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

var whoamiResponses = []any{
	msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 6,
		TotalRowCount:  1,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 201, ResultLen: 1, Values: []string{"10001"}},
			{AttributeIndex: 202, ResultLen: 1, Values: []string{testRods}},
			{AttributeIndex: 204, ResultLen: 1, Values: []string{testTestZoneName}},
			{AttributeIndex: 203, ResultLen: 1, Values: []string{testRodsAdmin}},
			{AttributeIndex: 208, ResultLen: 1, Values: []string{"10000"}},
			{AttributeIndex: 209, ResultLen: 1, Values: []string{"10000"}},
		},
	},
	msg.QueryResponse{
		RowCount:       2,
		AttributeCount: 1,
		TotalRowCount:  2,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 901, ResultLen: 1, Values: []string{testRods, testPublic}},
		},
	},
}

func TestWhoami(t *testing.T) {
	app := testApp(t)

	app.Workdir = testTestZone

	app.AddResponses(whoamiResponses)

	var buf bytes.Buffer

	cmd := app.Command()
	cmd.SetArgs([]string{"whoami"})
	cmd.SetOut(&buf)

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}

	out := buf.String()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// The remote workdir (testTestZone) and the local workdir (wd) must both be
	// reported alongside the user, zone, type and group memberships.
	for _, want := range []string{testRods, testTestZoneName, testRodsAdmin, testPublic, testTestZone, wd, "Remote workdir", "Local workdir"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected whoami output to contain %q, got:\n%s", want, out)
		}
	}

	// The implicit personal group equal to the user name must be filtered out,
	// leaving only "public" in the (sorted) groups list.
	if strings.Contains(out, testPublic+", "+testRods) {
		t.Errorf("expected personal group %q to be omitted, got:\n%s", testRods, out)
	}
}

func TestMkdir(t *testing.T) {
	app := testApp(t)

	app.AddResponse(msg.EmptyResponse{})

	cmd := app.Command()
	cmd.SetArgs([]string{"mkdir", testTestDir})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestRmdir(t *testing.T) {
	app := testApp(t)

	app.AddResponse(msg.CollectionOperationStat{})

	cmd := app.Command()
	cmd.SetArgs([]string{"rmdir", testTestDir})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestUnlock(t *testing.T) {
	app := testApp(t)

	app.Client.API.Admin = true

	app.AddResponse(msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 14,
		TotalRowCount:  1,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 401, ResultLen: 2, Values: []string{"1"}},
			{AttributeIndex: 500, ResultLen: 2, Values: []string{"1"}},
			{AttributeIndex: 406, ResultLen: 2, Values: []string{testGeneric}},
			{AttributeIndex: 404, ResultLen: 2, Values: []string{"2"}},
			{AttributeIndex: 407, ResultLen: 2, Values: []string{"1024000"}},
			{AttributeIndex: 411, ResultLen: 2, Values: []string{testRods}},
			{AttributeIndex: 412, ResultLen: 1, Values: []string{testTestZoneName}},
			{AttributeIndex: 415, ResultLen: 2, Values: []string{testChecksum}},
			{AttributeIndex: 413, ResultLen: 2, Values: []string{""}},
			{AttributeIndex: 409, ResultLen: 2, Values: []string{"resc"}},
			{AttributeIndex: 410, ResultLen: 2, Values: []string{testPath1}},
			{AttributeIndex: 422, ResultLen: 2, Values: []string{"demoResc;resc"}},
			{AttributeIndex: 419, ResultLen: 2, Values: []string{"10000"}},
			{AttributeIndex: 420, ResultLen: 2, Values: []string{"10000"}},
		},
	})

	app.AddResponse(msg.EmptyResponse{})
	app.AddResponse(msg.EmptyResponse{})

	cmd := app.Command()
	cmd.SetArgs([]string{"--admin", "unlock", "testfile"})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestUnlockDir(t *testing.T) {
	app := testApp(t)

	app.Client.API.Admin = true

	app.AddResponses(statResponses[:2])
	app.AddResponse(msg.QueryResponse{
		RowCount:       2,
		AttributeCount: 16,
		TotalRowCount:  2,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 401, ResultLen: 2, Values: []string{"1", "1"}},
			{AttributeIndex: 501, ResultLen: 2, Values: []string{testTestPath, testTestPath}},
			{AttributeIndex: 403, ResultLen: 2, Values: []string{"obj_name", "obj_name"}},
			{AttributeIndex: 500, ResultLen: 2, Values: []string{"1", "1"}},
			{AttributeIndex: 406, ResultLen: 2, Values: []string{testGeneric, testGeneric}},
			{AttributeIndex: 404, ResultLen: 2, Values: []string{"2", "4"}},
			{AttributeIndex: 407, ResultLen: 2, Values: []string{"1024000", "1024000"}},
			{AttributeIndex: 411, ResultLen: 2, Values: []string{testRods, testRods}},
			{AttributeIndex: 412, ResultLen: 2, Values: []string{testZoneShort, testZoneShort}},
			{AttributeIndex: 415, ResultLen: 2, Values: []string{testChecksum, testChecksum}},
			{AttributeIndex: 413, ResultLen: 2, Values: []string{"", ""}},
			{AttributeIndex: 409, ResultLen: 2, Values: []string{testResc1, testResc2}},
			{AttributeIndex: 410, ResultLen: 2, Values: []string{testPath1, "/path2"}},
			{AttributeIndex: 422, ResultLen: 2, Values: []string{testDemoRescResc1, testDemoRescResc2}},
			{AttributeIndex: 419, ResultLen: 2, Values: []string{"10000", "10000"}},
			{AttributeIndex: 420, ResultLen: 2, Values: []string{"10000", "10000"}},
		},
	})
	app.AddResponse(msg.QueryResponse{})

	// Fixes
	for range 4 {
		app.AddResponse(msg.EmptyResponse{})
	}

	cmd := app.Command()
	cmd.SetArgs([]string{"--admin", "unlock", testTestPath})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestTree(t *testing.T) {
	app := testApp(t)

	app.AddResponses(responses)

	app.AddResponse(msg.QueryResponse{})
	app.AddResponse(msg.QueryResponse{})

	cmd := app.Command()
	cmd.SetArgs([]string{"tree", testTestZone})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestList(t *testing.T) {
	app := testApp(t)

	app.AddResponses(responses)

	cmd := app.Command()
	cmd.SetArgs([]string{"ls", testTestZone})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
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
				{AttributeIndex: 711, ResultLen: 2, Values: []string{testOwn}},
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
	cmd.SetArgs([]string{"ls", "--acl", "--meta", testTestZone})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestListJSON(t *testing.T) {
	app := testApp(t)

	app.AddResponses(responses)

	cmd := app.Command()
	cmd.SetArgs([]string{"ls", "--json", testTestZone})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
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
			{AttributeIndex: 503, ResultLen: 1, Values: []string{testTestZoneColl}},
			{AttributeIndex: 504, ResultLen: 1, Values: []string{testZoneShort}},
			{AttributeIndex: 508, ResultLen: 1, Values: []string{"10000"}},
			{AttributeIndex: 509, ResultLen: 1, Values: []string{"2024"}},
			{AttributeIndex: 506, ResultLen: 1, Values: []string{"1"}},
		},
	},
	msg.QueryResponse{},
	msg.QueryResponse{},
	msg.QueryResponse{},
	msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 1,
		TotalRowCount:  1,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 407, ResultLen: 1, Values: []string{"100"}},
		},
	},
}

func TestStat(t *testing.T) {
	app := testApp(t)

	app.AddResponses(statResponses)

	cmd := app.Command()
	cmd.SetArgs([]string{"stat", testTestZoneColl})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestStatJSON(t *testing.T) {
	app := testApp(t)

	app.AddResponses(statResponses)

	cmd := app.Command()
	cmd.SetArgs([]string{"stat", "--json", testTestZoneColl})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestMv(t *testing.T) {
	app := testApp(t)

	app.AddResponses(statResponses[:2])
	app.AddResponse(msg.EmptyResponse{})

	cmd := app.Command()
	cmd.SetArgs([]string{"mv", testTestZoneColl, "/testzone/coll2"})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestRm(t *testing.T) {
	app := testApp(t)

	app.AddResponses(statResponses[:2])
	app.AddResponse(msg.CollectionOperationStat{})

	cmd := app.Command()
	cmd.SetArgs([]string{"rm", testTestZoneColl})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestCopy(t *testing.T) {
	app := testApp(t)

	app.AddResponse(msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 14,
		TotalRowCount:  1,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 401, ResultLen: 2, Values: []string{"1"}},
			{AttributeIndex: 500, ResultLen: 2, Values: []string{"1"}},
			{AttributeIndex: 406, ResultLen: 2, Values: []string{testGeneric}},
			{AttributeIndex: 404, ResultLen: 2, Values: []string{"0"}},
			{AttributeIndex: 407, ResultLen: 2, Values: []string{"1024000"}},
			{AttributeIndex: 411, ResultLen: 2, Values: []string{testRods}},
			{AttributeIndex: 412, ResultLen: 1, Values: []string{testTestZoneName}},
			{AttributeIndex: 415, ResultLen: 2, Values: []string{testChecksum}},
			{AttributeIndex: 413, ResultLen: 2, Values: []string{""}},
			{AttributeIndex: 409, ResultLen: 2, Values: []string{"resc"}},
			{AttributeIndex: 410, ResultLen: 2, Values: []string{testPath1}},
			{AttributeIndex: 422, ResultLen: 2, Values: []string{"demoResc;resc"}},
			{AttributeIndex: 419, ResultLen: 2, Values: []string{"10000"}},
			{AttributeIndex: 420, ResultLen: 2, Values: []string{"10000"}},
		},
	})
	app.AddResponse(msg.EmptyResponse{})

	cmd := app.Command()
	cmd.SetArgs([]string{"cp", "/testzone/coll/file", "/testzone/coll2/"})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestTouch(t *testing.T) {
	app := testApp(t)

	app.AddResponse(msg.QueryResponse{})
	app.AddResponse(msg.QueryResponse{})

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
	cmd.SetArgs([]string{"touch", testTestZoneObj1})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestChecksum(t *testing.T) {
	app := testApp(t)

	app.AddResponse(msg.String{
		String: "sha2:aaaa",
	})

	cmd := app.Command()
	cmd.SetArgs([]string{testChecksum, testTestZoneObj1})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestPWD(t *testing.T) {
	app := testApp(t)

	cmd := app.pwd()

	cmd.SetArgs([]string{})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestLocal(t *testing.T) {
	app := testApp(t)

	cmd := app.local()

	cmd.SetArgs([]string{testPwd})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}

	cmd.SetArgs([]string{"cd", "."})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}

	cmd.SetArgs([]string{"ls"})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestMetaList(t *testing.T) {
	app := testApp(t)

	app.AddResponses(statResponses[:len(statResponses)-2])

	cmd := app.Command()
	cmd.SetArgs([]string{testMeta, "ls", testTestZoneColl})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestMetaBasicOps(t *testing.T) {
	for _, op := range []string{"add", "rm", "set"} {
		app := testApp(t)

		app.AddResponses(statResponses[:2])
		app.AddResponse(msg.EmptyResponse{})

		cmd := app.Command()
		cmd.SetArgs([]string{testMeta, op, testTestZoneColl, "a", "b"})

		if err := cmd.ExecuteContext(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMetaUnset(t *testing.T) {
	app := testApp(t)

	app.AddResponses(statResponses[:3])
	app.AddResponse(msg.EmptyResponse{})
	app.AddResponse(msg.EmptyResponse{})

	cmd := app.Command()
	cmd.SetArgs([]string{testMeta, "unset", testTestZoneColl, "a"})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestCat(t *testing.T) {
	app := testApp(t)

	app.AddResponse(msg.FileDescriptor(1))
	app.Add(msg.DATA_OBJ_LSEEK_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
		Size:           0,
		Whence:         2,
	}, msg.SeekResponse{
		Offset: 100,
	})
	app.Add(msg.DATA_OBJ_LSEEK_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
		Size:           0,
		Whence:         0,
	}, msg.SeekResponse{
		Offset: 0o0,
	})
	app.AddBuffer(msg.DATA_OBJ_READ_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
		Size:           200,
	}, msg.ReadResponse(100), nil, bytes.Repeat([]byte("hello"), 20))
	app.AddResponse(msg.EmptyResponse{})

	cmd := app.Command()
	cmd.SetArgs([]string{"cat", "--threads", "1", testTestZoneObj1})

	transfer.BufferSize = 200

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestHead(t *testing.T) {
	app := testApp(t)

	app.AddResponse(msg.FileDescriptor(1))
	app.AddBuffer(msg.DATA_OBJ_READ_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
		Size:           4096,
	}, msg.ReadResponse(120), nil, bytes.Repeat([]byte("hello\n"), 20))
	app.AddResponse(msg.EmptyResponse{})

	cmd := app.Command()
	cmd.SetArgs([]string{"head", testTestZoneObj1})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestSave(t *testing.T) {
	app := testApp(t)

	app.AddResponse(msg.FileDescriptor(1))
	app.AddBuffer(msg.DATA_OBJ_WRITE_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
		Size:           6,
	}, msg.EmptyResponse{}, []byte("hello\n"), nil)
	app.AddResponse(msg.EmptyResponse{})

	cmd := app.Command()
	cmd.SetArgs([]string{"save", "--threads", "1", testTestZoneObj1})
	cmd.SetIn(strings.NewReader("hello\n"))

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestCD(t *testing.T) {
	app := testApp(t)

	// mock the workdir store
	app.workdirStore = func(_ context.Context, _ string) error {
		return nil
	}

	app.AddResponse(statResponses[1])
	app.AddResponse(msg.EmptyResponse{})

	cmd := app.Command()
	cmd.SetArgs([]string{"cd", testTestDir})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestSleep(t *testing.T) {
	app := testApp(t)

	cmd := app.Command()
	cmd.SetArgs([]string{"sleep", "0.1"})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestPS(t *testing.T) {
	app := testApp(t)

	app.AddResponse(msg.QueryResponse{
		AttributeCount: 9,
		RowCount:       1,
		ContinueIndex:  -1,
		TotalRowCount:  1,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 1000001, ResultLen: 1, Values: []string{"10"}},
			{AttributeIndex: 1000002, ResultLen: 1, Values: []string{"1764600000"}},
			{AttributeIndex: 1000003, ResultLen: 1, Values: []string{testUser}},
			{AttributeIndex: 1000004, ResultLen: 1, Values: []string{testZoneShort}},
			{AttributeIndex: 1000005, ResultLen: 1, Values: []string{testUser}},
			{AttributeIndex: 1000006, ResultLen: 1, Values: []string{testZoneShort}},
			{AttributeIndex: 1000007, ResultLen: 1, Values: []string{"1.2.3.4"}},
			{AttributeIndex: 1000008, ResultLen: 1, Values: []string{"example.org"}},
			{AttributeIndex: 1000009, ResultLen: 1, Values: []string{"iron"}},
		},
	})

	cmd := app.Command()
	cmd.SetArgs([]string{"ps"})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestQuery(t *testing.T) {
	app := testApp(t)

	app.AddResponse(msg.String{
		String: `{"DATA_NAME":{"R_DATA_MAIN":"data_name"}}`,
	})

	app.AddResponse(msg.String{
		String: `[["test", "1"]]`,
	})

	cmd := app.Command()

	cmd.SetArgs([]string{"query"})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}

	cmd.SetArgs([]string{"query", "SELECT DATA_NAME, DATA_SIZE"})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

var tokenizeTests = []struct {
	name     string
	query    string
	expected []string
}{
	{
		name:     "simple spaces",
		query:    "SELECT col1 col2 col3",
		expected: []string{testSelect, testCol1, testCol2, testCol3},
	},
	{
		name:     "commas without parentheses",
		query:    "col1,col2,col3",
		expected: []string{testCol1, testCol2, testCol3},
	},
	{
		name:     "mixed spaces and commas",
		query:    "SELECT col1, col2, col3",
		expected: []string{testSelect, testCol1, testCol2, testCol3},
	},
	{
		name:     "parentheses preserve contents",
		query:    "SELECT func(col1, col2) col3",
		expected: []string{testSelect, "func(col1, col2)", testCol3},
	},
	{
		name:     "nested parentheses",
		query:    "SELECT func(nested(a, b), c) col3",
		expected: []string{testSelect, "func(nested(a, b), c)", testCol3},
	},
	{
		name:     "empty string",
		query:    "",
		expected: []string{},
	},
	{
		name:     "single token",
		query:    testSelect,
		expected: []string{testSelect},
	},
	{
		name:     "only spaces",
		query:    "   ",
		expected: []string{},
	},
	{
		name:     "complex query",
		query:    "SELECT DISTINCT col1, func(col2, col3) WHERE col4",
		expected: []string{testSelect, "DISTINCT", testCol1, "func(col2, col3)", "WHERE", "col4"},
	},
}

func TestTokenize(t *testing.T) {
	for _, tt := range tokenizeTests {
		t.Run(tt.name, func(t *testing.T) {
			result := tokenize(tt.query)

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d tokens, got %d\nexpected: %v\ngot: %v",
					len(tt.expected), len(result), tt.expected, result)
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("token %d: expected %q, got %q", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

var guessColumnsTests = []struct {
	name     string
	query    string
	expected []string
}{
	{
		name:     "simple select",
		query:    "SELECT col1",
		expected: []string{testCol1},
	},
	{
		name:     "select multiple columns",
		query:    "SELECT col1 col2 col3",
		expected: []string{testCol1, testCol2, testCol3},
	},
	{
		name:     "select distinct",
		query:    "SELECT DISTINCT col1 col2",
		expected: []string{testCol1, testCol2},
	},
	{
		name:     "comma separated columns",
		query:    "SELECT col1,col2,col3",
		expected: []string{testCol1, testCol2, testCol3},
	},
	{
		name:     "stops at WHERE",
		query:    "SELECT col1 col2 WHERE col3",
		expected: []string{testCol1, testCol2},
	},
	{
		name:     "stops at GROUP",
		query:    "SELECT col1 col2 GROUP BY col3",
		expected: []string{testCol1, testCol2},
	},
	{
		name:     "stops at ORDER",
		query:    "SELECT col1 col2 ORDER BY col3",
		expected: []string{testCol1, testCol2},
	},
	{
		name:     "stops at LIMIT",
		query:    "SELECT col1 col2 LIMIT 10",
		expected: []string{testCol1, testCol2},
	},
	{
		name:     "stops at OFFSET",
		query:    "SELECT col1 col2 OFFSET 5",
		expected: []string{testCol1, testCol2},
	},
	{
		name:     "function with parentheses",
		query:    "SELECT func(col1, col2) col3",
		expected: []string{"func(col1, col2)", testCol3},
	},
	{
		name:     "mixed commas in tokens",
		query:    "SELECT col1, col2, col3",
		expected: []string{testCol1, testCol2, testCol3},
	},
	{
		name:     "empty query",
		query:    "",
		expected: []string{},
	},
	{
		name:     "only keywords",
		query:    "SELECT DISTINCT",
		expected: []string{},
	},
	{
		name:     "case insensitive keywords",
		query:    "select col1 where col2",
		expected: []string{testCol1},
	},
	{
		name:     "trailing commas",
		query:    "SELECT col1,col2,",
		expected: []string{testCol1, testCol2},
	},
}

func TestGuessColumns(t *testing.T) {
	for _, tt := range guessColumnsTests {
		t.Run(tt.name, func(t *testing.T) {
			result := guessColumns(tt.query)

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d columns, got %d\nexpected: %v\ngot: %v",
					len(tt.expected), len(result), tt.expected, result)
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("column %d: expected %q, got %q", i, tt.expected[i], result[i])
				}
			}
		})
	}
}
