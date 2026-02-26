package api

import (
	"encoding/base64"
	"errors"
	"testing"
)

func TestObjectTypeString(t *testing.T) {
	tests := []struct {
		input    ObjectType
		expected string
	}{
		{UserType, "user"},
		{CollectionType, "collection"},
		{DataObjectType, "data_object"},
		{ResourceType, "resource"},
		{ObjectType("X"), "X"},
		{ObjectType(""), ""},
	}

	for _, tt := range tests {
		result := tt.input.String()
		if result != tt.expected {
			t.Errorf("ObjectType(%q).String() = %q, want %q", string(tt.input), result, tt.expected)
		}
	}
}

func TestAsAdmin(t *testing.T) {
	api := &API{
		Username: "user",
		Zone:     "zone",
		Admin:    false,
	}

	admin := api.AsAdmin()

	if !admin.Admin {
		t.Error("expected Admin to be true")
	}

	if admin.Username != "user" || admin.Zone != "zone" {
		t.Error("expected Username and Zone to be preserved")
	}

	// Original should not be modified
	if api.Admin {
		t.Error("expected original API Admin to remain false")
	}
}

func TestWithDefaultResource(t *testing.T) {
	api := &API{
		Username:        "user",
		DefaultResource: "original",
	}

	modified := api.WithDefaultResource("newResource")

	if modified.DefaultResource != "newResource" {
		t.Errorf("expected DefaultResource='newResource', got %q", modified.DefaultResource)
	}

	// Original unchanged
	if api.DefaultResource != "original" {
		t.Errorf("expected original DefaultResource='original', got %q", api.DefaultResource)
	}
}

func TestWithNumThreads(t *testing.T) {
	api := &API{
		Username:   "user",
		NumThreads: 0,
	}

	modified := api.WithNumThreads(4)

	if modified.NumThreads != 4 {
		t.Errorf("expected NumThreads=4, got %d", modified.NumThreads)
	}

	// Original unchanged
	if api.NumThreads != 0 {
		t.Errorf("expected original NumThreads=0, got %d", api.NumThreads)
	}
}

func TestWithReplicaNumber(t *testing.T) {
	api := &API{
		Username: "user",
	}

	modified := api.WithReplicaNumber(2)

	if modified.ReplicaNumber == nil {
		t.Fatal("expected ReplicaNumber to be set")
	}

	if *modified.ReplicaNumber != 2 {
		t.Errorf("expected ReplicaNumber=2, got %d", *modified.ReplicaNumber)
	}

	// Original unchanged
	if api.ReplicaNumber != nil {
		t.Error("expected original ReplicaNumber to remain nil")
	}
}

func TestSplit(t *testing.T) {
	tests := []struct {
		path     string
		wantDir  string
		wantBase string
	}{
		{"/zone/home/user", "/zone/home", "user"},
		{"/zone/home", "/zone", "home"},
		{"/zone", "/", "zone"},
		{"/", "/", ""},
		{"file.txt", "", "file.txt"},
		{"/a/b/c/d", "/a/b/c", "d"},
		{"", "", ""},
	}

	for _, tt := range tests {
		dir, base := Split(tt.path)
		if dir != tt.wantDir || base != tt.wantBase {
			t.Errorf("Split(%q) = (%q, %q), want (%q, %q)", tt.path, dir, base, tt.wantDir, tt.wantBase)
		}
	}
}

func TestComparePaths(t *testing.T) {
	tests := []struct {
		a, b string
		want int // -1, 0, 1
	}{
		{"/a/b", "/a/b", 0},
		{"/a/b", "/a/c", -1},
		{"/a/c", "/a/b", 1},
		{"/a", "/a/b", -1},
		{"/a/b", "/a", 1},
		{"/a/b/c", "/a/b/d", -1},
		{"/", "/", 0},
		{"/a", "/b", -1},
		{"/b", "/a", 1},
	}

	for _, tt := range tests {
		result := ComparePaths(tt.a, tt.b)
		switch {
		case tt.want < 0 && result >= 0:
			t.Errorf("ComparePaths(%q, %q) = %d, want < 0", tt.a, tt.b, result)
		case tt.want > 0 && result <= 0:
			t.Errorf("ComparePaths(%q, %q) = %d, want > 0", tt.a, tt.b, result)
		case tt.want == 0 && result != 0:
			t.Errorf("ComparePaths(%q, %q) = %d, want 0", tt.a, tt.b, result)
		}
	}
}

func TestParseIrodsChecksum(t *testing.T) {
	// Valid sha2 checksum
	hash := make([]byte, 32)
	for i := range hash {
		hash[i] = byte(i)
	}

	encoded := "sha2:" + base64.StdEncoding.EncodeToString(hash)

	result, err := ParseIrodsChecksum(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(result))
	}

	for i, b := range result {
		if b != byte(i) {
			t.Errorf("byte %d: expected %d, got %d", i, i, b)
			break
		}
	}
}

func TestParseIrodsChecksumMissingPrefix(t *testing.T) {
	_, err := ParseIrodsChecksum("md5:abc123")
	if err == nil {
		t.Fatal("expected error for missing sha2 prefix")
	}

	if !errors.Is(err, ErrChecksumNotFound) {
		t.Errorf("expected ErrChecksumNotFound, got %v", err)
	}
}

func TestParseIrodsChecksumInvalidBase64(t *testing.T) {
	_, err := ParseIrodsChecksum("sha2:not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestParseIrodsChecksumEmpty(t *testing.T) {
	_, err := ParseIrodsChecksum("")
	if err == nil {
		t.Fatal("expected error for empty string")
	}
}
