package api

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/kuleuven/iron/msg"
)

func TestColumnHelpers(t *testing.T) {
	tests := []struct {
		name             string
		col              Column
		wantInt          int
		wantAggregation  int
	}{
		{"Min", Min(msg.ICAT_COLUMN_DATA_SIZE), int(msg.ICAT_COLUMN_DATA_SIZE), 2},
		{"Max", Max(msg.ICAT_COLUMN_DATA_SIZE), int(msg.ICAT_COLUMN_DATA_SIZE), 3},
		{"Sum", Sum(msg.ICAT_COLUMN_DATA_SIZE), int(msg.ICAT_COLUMN_DATA_SIZE), 4},
		{"Avg", Avg(msg.ICAT_COLUMN_DATA_SIZE), int(msg.ICAT_COLUMN_DATA_SIZE), 5},
		{"Count", Count(msg.ICAT_COLUMN_DATA_SIZE), int(msg.ICAT_COLUMN_DATA_SIZE), 6},
	}

	for _, tt := range tests {
		if tt.col.Int() != tt.wantInt {
			t.Errorf("%s.Int() = %d, want %d", tt.name, tt.col.Int(), tt.wantInt)
		}

		if tt.col.AggregationLevel() != tt.wantAggregation {
			t.Errorf("%s.AggregationLevel() = %d, want %d", tt.name, tt.col.AggregationLevel(), tt.wantAggregation)
		}
	}
}

func TestCollectionAccessors(t *testing.T) {
	c := &Collection{
		ID:    7,
		Path:  "/zone/home/user/foo",
		Owner: "user",
	}

	if c.Identifier() != 7 {
		t.Errorf("Identifier() = %d, want 7", c.Identifier())
	}

	if c.ObjectType() != CollectionType {
		t.Errorf("ObjectType() = %s, want CollectionType", c.ObjectType())
	}

	if got, ok := c.Sys().(*Collection); !ok || got != c {
		t.Errorf("Sys() did not return *Collection pointer")
	}

	if c.Mode() != os.FileMode(0o750)|os.ModeDir {
		t.Errorf("non-inheritance Mode() = %v", c.Mode())
	}

	c.Inheritance = true
	if c.Mode() != os.FileMode(0o750)|os.ModeDir|os.ModeSetgid {
		t.Errorf("inheritance Mode() = %v", c.Mode())
	}
}

func TestDataObjectAccessors(t *testing.T) {
	d := &DataObject{
		ID:   42,
		Path: "/zone/home/user/bar",
	}

	if d.Identifier() != 42 {
		t.Errorf("Identifier() = %d, want 42", d.Identifier())
	}

	if d.ObjectType() != DataObjectType {
		t.Errorf("ObjectType() = %s, want DataObjectType", d.ObjectType())
	}

	if got, ok := d.Sys().(*DataObject); !ok || got != d {
		t.Errorf("Sys() did not return *DataObject pointer")
	}
}

func TestResourceAccessors(t *testing.T) {
	r := &Resource{ID: 9}

	if r.Identifier() != 9 {
		t.Errorf("Identifier() = %d, want 9", r.Identifier())
	}

	if r.ObjectType() != ResourceType {
		t.Errorf("ObjectType() = %s, want ResourceType", r.ObjectType())
	}
}

func TestUserAccessors(t *testing.T) {
	u := &User{ID: 11}

	if u.Identifier() != 11 {
		t.Errorf("Identifier() = %d, want 11", u.Identifier())
	}

	if u.ObjectType() != UserType {
		t.Errorf("ObjectType() = %s, want UserType", u.ObjectType())
	}
}

// fakeFileInfo is a minimal os.FileInfo implementation for record tests.
type fakeFileInfo struct {
	name  string
	size  int64
	mode  os.FileMode
	isDir bool
}

func (f *fakeFileInfo) Name() string       { return f.name }
func (f *fakeFileInfo) Size() int64        { return f.size }
func (f *fakeFileInfo) Mode() os.FileMode  { return f.mode }
func (f *fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f *fakeFileInfo) IsDir() bool        { return f.isDir }
func (f *fakeFileInfo) Sys() any           { return nil }

func TestRecordAccessors(t *testing.T) {
	meta := []Metadata{{Name: "k", Value: "v"}}
	acl := []Access{{User: User{Name: "alice"}}}

	dir := &record{
		FileInfo:       &fakeFileInfo{name: "coll", isDir: true},
		metadata:       meta,
		access:         acl,
		collectionSize: 1234,
	}

	if got := dir.Metadata(); len(got) != 1 || got[0].Name != "k" {
		t.Errorf("Metadata() = %v, want %v", got, meta)
	}

	if got := dir.Access(); len(got) != 1 || got[0].User.Name != "alice" {
		t.Errorf("Access() = %v, want %v", got, acl)
	}

	if dir.Size() != 1234 {
		t.Errorf("dir Size() = %d, want 1234", dir.Size())
	}

	if dir.Type() != CollectionType {
		t.Errorf("dir Type() = %s, want CollectionType", dir.Type())
	}

	file := &record{
		FileInfo:       &fakeFileInfo{name: "obj", size: 500, isDir: false},
		collectionSize: 9999,
	}

	if file.Size() != 500 {
		t.Errorf("file Size() = %d, want 500 (collectionSize must be ignored)", file.Size())
	}

	if file.Type() != DataObjectType {
		t.Errorf("file Type() = %s, want DataObjectType", file.Type())
	}
}

func TestHandleWalkError(t *testing.T) {
	api := &API{}
	target := errors.New("boom")

	names := []string{"/a", "/b", "/c"}
	var seen []string

	err := api.handleWalkError(func(path string, _ Record, err error) error {
		seen = append(seen, path)

		if !errors.Is(err, target) {
			t.Errorf("walkFn received err=%v, want %v", err, target)
		}

		return nil
	}, names, target)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if len(seen) != len(names) {
		t.Errorf("seen = %v, want %v", seen, names)
	}

	// Returning an error should stop iteration and propagate.
	stop := errors.New("stop")

	err = api.handleWalkError(func(_ string, _ Record, _ error) error {
		return stop
	}, names, target)
	if !errors.Is(err, stop) {
		t.Errorf("expected stop sentinel, got %v", err)
	}
}

func TestHandleName(t *testing.T) {
	h := &handle{
		object: &object{path: "/zone/home/user/file"},
	}

	if h.Name() != "/zone/home/user/file" {
		t.Errorf("Name() = %s, want /zone/home/user/file", h.Name())
	}
}

func TestGenericSingleRowQuerySQL(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.String{String: "SELECT 1"})

	sql, err := testAPI.GenericQueryRow("SELECT DATA_NAME").SQL(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if sql != "SELECT 1" {
		t.Errorf("SQL() = %q, want SELECT 1", sql)
	}
}

func TestModifyModificationTime(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.EmptyResponse{})

	if err := testAPI.ModifyModificationTime(t.Context(), "/test/file", time.Unix(1700000000, 0)); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyChecksum(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.EmptyResponse{})

	if err := testAPI.VerifyChecksum(t.Context(), "/test/file"); err != nil {
		t.Fatal(err)
	}
}
