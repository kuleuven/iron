package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/kuleuven/iron/msg"
	"github.com/kuleuven/iron/transfer"
)

func TestVersion(t *testing.T) {
	app := testApp(t)

	cmd := app.Command()
	cmd.SetArgs([]string{"version"})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestMkdir(t *testing.T) {
	app := testApp(t)

	app.AddResponse(msg.EmptyResponse{})

	cmd := app.Command()
	cmd.SetArgs([]string{"mkdir", "testdir"})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestRmdir(t *testing.T) {
	app := testApp(t)

	app.AddResponse(msg.CollectionOperationStat{})

	cmd := app.Command()
	cmd.SetArgs([]string{"rmdir", "testdir"})

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
	cmd.SetArgs([]string{"tree", "/testzone"})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestList(t *testing.T) {
	app := testApp(t)

	app.AddResponses(responses)

	cmd := app.Command()
	cmd.SetArgs([]string{"ls", "/testzone"})

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

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestListJSON(t *testing.T) {
	app := testApp(t)

	app.AddResponses(responses)

	cmd := app.Command()
	cmd.SetArgs([]string{"ls", "--json", "/testzone"})

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
	cmd.SetArgs([]string{"stat", "/testzone/coll"})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestStatJSON(t *testing.T) {
	app := testApp(t)

	app.AddResponses(statResponses)

	cmd := app.Command()
	cmd.SetArgs([]string{"stat", "--json", "/testzone/coll"})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestMv(t *testing.T) {
	app := testApp(t)

	app.AddResponses(statResponses[:2])
	app.AddResponse(msg.EmptyResponse{})

	cmd := app.Command()
	cmd.SetArgs([]string{"mv", "/testzone/coll", "/testzone/coll2"})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestRm(t *testing.T) {
	app := testApp(t)

	app.AddResponses(statResponses[:2])
	app.AddResponse(msg.CollectionOperationStat{})

	cmd := app.Command()
	cmd.SetArgs([]string{"rm", "/testzone/coll"})

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
			{AttributeIndex: 406, ResultLen: 2, Values: []string{"generic"}},
			{AttributeIndex: 404, ResultLen: 2, Values: []string{"0"}},
			{AttributeIndex: 407, ResultLen: 2, Values: []string{"1024000"}},
			{AttributeIndex: 411, ResultLen: 2, Values: []string{"rods"}},
			{AttributeIndex: 412, ResultLen: 1, Values: []string{"testzone"}},
			{AttributeIndex: 415, ResultLen: 2, Values: []string{"checksum"}},
			{AttributeIndex: 413, ResultLen: 2, Values: []string{""}},
			{AttributeIndex: 409, ResultLen: 2, Values: []string{"resc"}},
			{AttributeIndex: 410, ResultLen: 2, Values: []string{"/path1"}},
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
	cmd.SetArgs([]string{"touch", "/testzone/obj1"})

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
	cmd.SetArgs([]string{"checksum", "/testzone/obj1"})

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

	cmd.SetArgs([]string{"pwd"})

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
	cmd.SetArgs([]string{"meta", "ls", "/testzone/coll"})

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
		cmd.SetArgs([]string{"meta", op, "/testzone/coll", "a", "b"})

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
	cmd.SetArgs([]string{"meta", "unset", "/testzone/coll", "a"})

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
	cmd.SetArgs([]string{"cat", "--threads", "1", "/testzone/obj1"})

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
	cmd.SetArgs([]string{"head", "/testzone/obj1"})

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
	cmd.SetArgs([]string{"save", "--threads", "1", "/testzone/obj1"})
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
	cmd.SetArgs([]string{"cd", "testdir"})

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
			{AttributeIndex: 1000003, ResultLen: 1, Values: []string{"user"}},
			{AttributeIndex: 1000004, ResultLen: 1, Values: []string{"zone"}},
			{AttributeIndex: 1000005, ResultLen: 1, Values: []string{"user"}},
			{AttributeIndex: 1000006, ResultLen: 1, Values: []string{"zone"}},
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
		expected: []string{"SELECT", "col1", "col2", "col3"},
	},
	{
		name:     "commas without parentheses",
		query:    "col1,col2,col3",
		expected: []string{"col1", "col2", "col3"},
	},
	{
		name:     "mixed spaces and commas",
		query:    "SELECT col1, col2, col3",
		expected: []string{"SELECT", "col1", "col2", "col3"},
	},
	{
		name:     "parentheses preserve contents",
		query:    "SELECT func(col1, col2) col3",
		expected: []string{"SELECT", "func(col1, col2)", "col3"},
	},
	{
		name:     "nested parentheses",
		query:    "SELECT func(nested(a, b), c) col3",
		expected: []string{"SELECT", "func(nested(a, b), c)", "col3"},
	},
	{
		name:     "empty string",
		query:    "",
		expected: []string{},
	},
	{
		name:     "single token",
		query:    "SELECT",
		expected: []string{"SELECT"},
	},
	{
		name:     "only spaces",
		query:    "   ",
		expected: []string{},
	},
	{
		name:     "complex query",
		query:    "SELECT DISTINCT col1, func(col2, col3) WHERE col4",
		expected: []string{"SELECT", "DISTINCT", "col1", "func(col2, col3)", "WHERE", "col4"},
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
		expected: []string{"col1"},
	},
	{
		name:     "select multiple columns",
		query:    "SELECT col1 col2 col3",
		expected: []string{"col1", "col2", "col3"},
	},
	{
		name:     "select distinct",
		query:    "SELECT DISTINCT col1 col2",
		expected: []string{"col1", "col2"},
	},
	{
		name:     "comma separated columns",
		query:    "SELECT col1,col2,col3",
		expected: []string{"col1", "col2", "col3"},
	},
	{
		name:     "stops at WHERE",
		query:    "SELECT col1 col2 WHERE col3",
		expected: []string{"col1", "col2"},
	},
	{
		name:     "stops at GROUP",
		query:    "SELECT col1 col2 GROUP BY col3",
		expected: []string{"col1", "col2"},
	},
	{
		name:     "stops at ORDER",
		query:    "SELECT col1 col2 ORDER BY col3",
		expected: []string{"col1", "col2"},
	},
	{
		name:     "stops at LIMIT",
		query:    "SELECT col1 col2 LIMIT 10",
		expected: []string{"col1", "col2"},
	},
	{
		name:     "stops at OFFSET",
		query:    "SELECT col1 col2 OFFSET 5",
		expected: []string{"col1", "col2"},
	},
	{
		name:     "function with parentheses",
		query:    "SELECT func(col1, col2) col3",
		expected: []string{"func(col1, col2)", "col3"},
	},
	{
		name:     "mixed commas in tokens",
		query:    "SELECT col1, col2, col3",
		expected: []string{"col1", "col2", "col3"},
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
		expected: []string{"col1"},
	},
	{
		name:     "trailing commas",
		query:    "SELECT col1,col2,",
		expected: []string{"col1", "col2"},
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
