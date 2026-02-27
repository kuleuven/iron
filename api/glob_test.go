//nolint:goconst
package api

import (
	"fmt"
	"testing"

	"github.com/kuleuven/iron/msg"
)

// collectionsResponse returns a mock QueryResponse for ListCollections with the given collection paths.
func collectionsResponse(paths ...string) msg.QueryResponse {
	n := len(paths)
	if n == 0 {
		return msg.QueryResponse{AttributeCount: 7}
	}

	ids := make([]string, n)
	owners := make([]string, n)
	zones := make([]string, n)
	ctimes := make([]string, n)
	mtimes := make([]string, n)
	inherits := make([]string, n)

	for i := range paths {
		ids[i] = "1"
		owners[i] = "rods"
		zones[i] = "zone"
		ctimes[i] = "10000"
		mtimes[i] = "10000"
		inherits[i] = "0"
	}

	return msg.QueryResponse{
		RowCount:       n,
		AttributeCount: 7,
		TotalRowCount:  n,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 500, ResultLen: 1, Values: ids},
			{AttributeIndex: 501, ResultLen: 1, Values: paths},
			{AttributeIndex: 503, ResultLen: 1, Values: owners},
			{AttributeIndex: 504, ResultLen: 1, Values: zones},
			{AttributeIndex: 508, ResultLen: 1, Values: ctimes},
			{AttributeIndex: 509, ResultLen: 1, Values: mtimes},
			{AttributeIndex: 506, ResultLen: 1, Values: inherits},
		},
	}
}

// dataObjectsResponse returns a mock QueryResponse for ListDataObjects with the given
// (collectionPath, objectName) pairs.
func dataObjectsResponse(items ...struct{ coll, name string }) msg.QueryResponse { //nolint:funlen
	n := len(items)
	if n == 0 {
		return msg.QueryResponse{AttributeCount: 16}
	}

	ids := make([]string, n)
	colls := make([]string, n)
	names := make([]string, n)
	collIDs := make([]string, n)
	dtypes := make([]string, n)
	replNums := make([]string, n)
	sizes := make([]string, n)
	owners := make([]string, n)
	ownerZones := make([]string, n)
	checksums := make([]string, n)
	statuses := make([]string, n)
	rescs := make([]string, n)
	physPaths := make([]string, n)
	rescHiers := make([]string, n)
	ctimes := make([]string, n)
	mtimes := make([]string, n)

	for i, item := range items {
		ids[i] = fmt.Sprintf("%d", i+1)
		colls[i] = item.coll
		names[i] = item.name
		collIDs[i] = "1"
		dtypes[i] = "generic"
		replNums[i] = "0"
		sizes[i] = "1024"
		owners[i] = "rods"
		ownerZones[i] = "zone"
		checksums[i] = ""
		statuses[i] = ""
		rescs[i] = "demoResc"
		physPaths[i] = "/vault/" + item.name
		rescHiers[i] = "demoResc"
		ctimes[i] = "10000"
		mtimes[i] = "10000"
	}

	return msg.QueryResponse{
		RowCount:       n,
		AttributeCount: 16,
		TotalRowCount:  n,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 401, ResultLen: 1, Values: ids},
			{AttributeIndex: 501, ResultLen: 1, Values: colls},
			{AttributeIndex: 403, ResultLen: 1, Values: names},
			{AttributeIndex: 500, ResultLen: 1, Values: collIDs},
			{AttributeIndex: 406, ResultLen: 1, Values: dtypes},
			{AttributeIndex: 404, ResultLen: 1, Values: replNums},
			{AttributeIndex: 407, ResultLen: 1, Values: sizes},
			{AttributeIndex: 411, ResultLen: 1, Values: owners},
			{AttributeIndex: 412, ResultLen: 1, Values: ownerZones},
			{AttributeIndex: 415, ResultLen: 1, Values: checksums},
			{AttributeIndex: 413, ResultLen: 1, Values: statuses},
			{AttributeIndex: 409, ResultLen: 1, Values: rescs},
			{AttributeIndex: 410, ResultLen: 1, Values: physPaths},
			{AttributeIndex: 422, ResultLen: 1, Values: rescHiers},
			{AttributeIndex: 419, ResultLen: 1, Values: ctimes},
			{AttributeIndex: 420, ResultLen: 1, Values: mtimes},
		},
	}
}

type collObj struct{ coll, name string }

func TestGlobRelativeStar(t *testing.T) {
	testAPI := newAPI()

	// Glob pattern: "*.txt" in root "/zone/home"
	// Expect: ListCollections (subcols matching *.txt) → none
	//         ListDataObjects (objects matching *.txt) → 2 matches
	testAPI.AddResponses([]any{
		collectionsResponse(),
		dataObjectsResponse(
			collObj{"/zone/home", "file1.txt"},
			collObj{"/zone/home", "file2.txt"},
		),
	})

	var paths []string

	err := testAPI.Glob(t.Context(), "/zone/home", "*.txt", func(path string, rec Record, err error) error {
		if err != nil {
			return err
		}

		paths = append(paths, path)

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(paths))
	}

	// Relative pattern → relative paths
	if paths[0] != "file1.txt" {
		t.Errorf("expected relative path 'file1.txt', got %q", paths[0])
	}

	if paths[1] != "file2.txt" {
		t.Errorf("expected relative path 'file2.txt', got %q", paths[1])
	}
}

func TestGlobAbsoluteStar(t *testing.T) {
	testAPI := newAPI()

	// Glob pattern: "/zone/home/*.txt" (absolute)
	testAPI.AddResponses([]any{
		collectionsResponse(),
		dataObjectsResponse(
			collObj{"/zone/home", "file1.txt"},
		),
	})

	var paths []string

	err := testAPI.Glob(t.Context(), "/zone/home", "/zone/home/*.txt", func(path string, rec Record, err error) error {
		if err != nil {
			return err
		}

		paths = append(paths, path)

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 1 {
		t.Fatalf("expected 1 match, got %d", len(paths))
	}

	// Absolute pattern → absolute path
	if paths[0] != "/zone/home/file1.txt" {
		t.Errorf("expected absolute path '/zone/home/file1.txt', got %q", paths[0])
	}
}

func TestGlobNestedPattern(t *testing.T) {
	testAPI := newAPI()

	// Glob pattern: "sub*/data.csv" in root "/zone/home"
	// First level: ListCollections matching sub* → 2 subcollections
	// Second level (subA): ListCollections → none, ListDataObjects → 1 match
	// Second level (subB): ListCollections → none, ListDataObjects → 1 match
	testAPI.AddResponses([]any{
		// Level 1: subcollections of /zone/home matching sub*
		collectionsResponse("/zone/home/subA", "/zone/home/subB"),
		// Level 2, subA: subcollections matching data.csv → none
		collectionsResponse(),
		// Level 2, subA: data objects matching data.csv
		dataObjectsResponse(collObj{"/zone/home/subA", "data.csv"}),
		// Level 2, subB: subcollections matching data.csv → none
		collectionsResponse(),
		// Level 2, subB: data objects matching data.csv
		dataObjectsResponse(collObj{"/zone/home/subB", "data.csv"}),
	})

	var paths []string

	err := testAPI.Glob(t.Context(), "/zone/home", "sub*/data.csv", func(path string, rec Record, err error) error {
		if err != nil {
			return err
		}

		paths = append(paths, path)

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(paths))
	}

	if paths[0] != "subA/data.csv" {
		t.Errorf("expected 'subA/data.csv', got %q", paths[0])
	}

	if paths[1] != "subB/data.csv" {
		t.Errorf("expected 'subB/data.csv', got %q", paths[1])
	}
}

func TestGlobQuestionMark(t *testing.T) {
	testAPI := newAPI()

	// Glob pattern: "file?.txt"
	testAPI.AddResponses([]any{
		collectionsResponse(),
		dataObjectsResponse(
			collObj{"/zone/home", "file1.txt"},
			collObj{"/zone/home", "file2.txt"},
		),
	})

	var paths []string

	err := testAPI.Glob(t.Context(), "/zone/home", "file?.txt", func(path string, rec Record, err error) error {
		if err != nil {
			return err
		}

		paths = append(paths, path)

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(paths))
	}
}

func TestGlobNoWildcard(t *testing.T) {
	testAPI := newAPI()

	// Glob pattern without wildcard does exact path lookup via GetRecord.
	// GetRecord tries GetDataObject first (returns no rows), then GetCollection.
	testAPI.AddResponses([]any{
		// GetDataObject → no rows
		msg.QueryResponse{AttributeCount: 14},
		// GetCollection → match
		msg.QueryResponse{
			RowCount:       1,
			AttributeCount: 6,
			TotalRowCount:  1,
			ContinueIndex:  0,
			SQLResult: []msg.SQLResult{
				{AttributeIndex: 500, ResultLen: 1, Values: []string{"1"}},
				{AttributeIndex: 503, ResultLen: 1, Values: []string{"rods"}},
				{AttributeIndex: 504, ResultLen: 1, Values: []string{"zone"}},
				{AttributeIndex: 508, ResultLen: 1, Values: []string{"10000"}},
				{AttributeIndex: 509, ResultLen: 1, Values: []string{"10000"}},
				{AttributeIndex: 506, ResultLen: 1, Values: []string{"0"}},
			},
		},
	})

	var paths []string

	err := testAPI.Glob(t.Context(), "/zone/home", "subdir", func(path string, rec Record, err error) error {
		if err != nil {
			return err
		}

		paths = append(paths, path)

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 1 {
		t.Fatalf("expected 1 match, got %d", len(paths))
	}

	if paths[0] != "subdir" {
		t.Errorf("expected 'subdir', got %q", paths[0])
	}
}

func TestGlobSkipAll(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponses([]any{
		collectionsResponse(),
		dataObjectsResponse(
			collObj{"/zone/home", "a.txt"},
			collObj{"/zone/home", "b.txt"},
		),
	})

	var count int

	err := testAPI.Glob(t.Context(), "/zone/home", "*.txt", func(path string, rec Record, err error) error {
		count++

		return SkipAll
	})
	if err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Errorf("expected walkFn called once before SkipAll, got %d", count)
	}
}

func TestGlobMatchesCollections(t *testing.T) {
	testAPI := newAPI()

	// Pattern matches a subcollection, not a data object
	testAPI.AddResponses([]any{
		collectionsResponse("/zone/home/archive"),
		dataObjectsResponse(),
	})

	var paths []string

	err := testAPI.Glob(t.Context(), "/zone/home", "arch*", func(path string, rec Record, err error) error {
		if err != nil {
			return err
		}

		paths = append(paths, path)

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 1 {
		t.Fatalf("expected 1 match, got %d", len(paths))
	}

	if paths[0] != "archive" {
		t.Errorf("expected 'archive', got %q", paths[0])
	}
}

func TestGlobNoMatches(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponses([]any{
		collectionsResponse(),
		dataObjectsResponse(),
	})

	var count int

	err := testAPI.Glob(t.Context(), "/zone/home", "*.xyz", func(path string, rec Record, err error) error {
		count++

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if count != 0 {
		t.Errorf("expected 0 matches, got %d", count)
	}
}

func TestGlobStaticPrefix(t *testing.T) {
	testAPI := newAPI()

	// Pattern "deep/nested/*.dat" — static prefix "deep/nested", wildcard "*.dat"
	// Only the last component has a wildcard, so "deep" and "nested" are traversed without queries
	testAPI.AddResponses([]any{
		collectionsResponse(),
		dataObjectsResponse(
			collObj{"/zone/home/deep/nested", "report.dat"},
		),
	})

	var paths []string

	err := testAPI.Glob(t.Context(), "/zone/home", "deep/nested/*.dat", func(path string, rec Record, err error) error {
		if err != nil {
			return err
		}

		paths = append(paths, path)

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 1 {
		t.Fatalf("expected 1 match, got %d", len(paths))
	}

	if paths[0] != "deep/nested/report.dat" {
		t.Errorf("expected 'deep/nested/report.dat', got %q", paths[0])
	}
}

func TestGlobToLike(t *testing.T) {
	tests := []struct {
		glob string
		like string
	}{
		{"*", "%"},
		{"*.txt", "%.txt"},
		{"file?.dat", "file_.dat"},
		{"[abc]test", "%test"},
		{"hello", "hello"},
		{"100%", `100\%`},
		{"col_name", `col\_name`},
		{`\*`, `*`},
		{`\?`, `?`},
		{"[a-z]*", "%%"},
	}

	for _, tt := range tests {
		t.Run(tt.glob, func(t *testing.T) {
			got := globToLike(tt.glob)
			if got != tt.like {
				t.Errorf("globToLike(%q) = %q, want %q", tt.glob, got, tt.like)
			}
		})
	}
}

func TestSplitGlobPrefix(t *testing.T) {
	tests := []struct {
		input    string
		wantDir  string
		wantLen  int
		wantPart string
	}{
		{"/zone/home/*.txt", "/zone/home", 1, "*.txt"},
		{"/zone/home/sub/file.txt", "/zone/home/sub/file.txt", 0, ""},
		{"/zone/*/data", "/zone", 2, "*"},
		{"/*", "/", 1, "*"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			dir, parts := splitGlobPrefix(tt.input)
			if dir != tt.wantDir {
				t.Errorf("splitGlobPrefix(%q) dir = %q, want %q", tt.input, dir, tt.wantDir)
			}

			if len(parts) != tt.wantLen {
				t.Errorf("splitGlobPrefix(%q) len(parts) = %d, want %d", tt.input, len(parts), tt.wantLen)
			}

			if tt.wantLen > 0 && parts[0] != tt.wantPart {
				t.Errorf("splitGlobPrefix(%q) parts[0] = %q, want %q", tt.input, parts[0], tt.wantPart)
			}
		})
	}
}

func TestGlobPath(t *testing.T) {
	tests := []struct {
		name   string
		root   string
		abs    string
		isAbs  bool
		expect string
	}{
		{"absolute", "/zone/home", "/zone/home/file.txt", true, "/zone/home/file.txt"},
		{"relative file", "/zone/home", "/zone/home/file.txt", false, "file.txt"},
		{"relative subdir", "/zone/home", "/zone/home/sub/file.txt", false, "sub/file.txt"},
		{"relative root", "/zone/home", "/zone/home", false, "."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := globPath(tt.root, tt.abs, tt.isAbs)
			if got != tt.expect {
				t.Errorf("globPath(%q, %q, %v) = %q, want %q", tt.root, tt.abs, tt.isAbs, got, tt.expect)
			}
		})
	}
}
