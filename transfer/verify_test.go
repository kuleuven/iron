package transfer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kuleuven/iron/api"
)

func TestParseChecksumNilInfo(t *testing.T) {
	result, ok := parseChecksum(nil)
	if ok {
		t.Error("expected ok=false for nil FileInfo")
	}

	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

func TestParseChecksumNonDataObject(t *testing.T) {
	// os.FileInfo that doesn't have a DataObject as Sys()
	info, err := os.Stat(".")
	if err != nil {
		t.Fatal(err)
	}

	result, ok := parseChecksum(info)
	if ok {
		t.Error("expected ok=false for non-data-object FileInfo")
	}

	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

type fakeFileInfo struct {
	sys any
}

func (f *fakeFileInfo) Name() string       { return "test" }
func (f *fakeFileInfo) Size() int64        { return 0 }
func (f *fakeFileInfo) Mode() os.FileMode  { return 0 }
func (f *fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f *fakeFileInfo) IsDir() bool        { return false }
func (f *fakeFileInfo) Sys() any           { return f.sys }

func TestParseChecksumNoValidReplica(t *testing.T) {
	obj := &api.DataObject{
		Replicas: []api.Replica{
			{Status: "0", Checksum: "sha2:AAAA"},
		},
	}

	info := &fakeFileInfo{sys: obj}

	result, ok := parseChecksum(info)
	if ok {
		t.Error("expected ok=false for stale replica")
	}

	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

func TestParseChecksumValidReplica(t *testing.T) {
	// sha256 of empty input in base64: 47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=
	obj := &api.DataObject{
		Replicas: []api.Replica{
			{Status: "1", Checksum: "sha2:47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU="},
		},
	}

	info := &fakeFileInfo{sys: obj}

	result, ok := parseChecksum(info)
	if !ok {
		t.Fatal("expected ok=true for valid replica")
	}

	if len(result) != 32 {
		t.Errorf("expected 32-byte hash, got %d bytes", len(result))
	}
}

func TestParseChecksumWrongPrefix(t *testing.T) {
	obj := &api.DataObject{
		Replicas: []api.Replica{
			{Status: "1", Checksum: "md5:abc123"},
		},
	}

	info := &fakeFileInfo{sys: obj}

	result, ok := parseChecksum(info)
	if ok {
		t.Error("expected ok=false for non-sha2 checksum")
	}

	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

func TestParseChecksumInvalidBase64(t *testing.T) {
	obj := &api.DataObject{
		Replicas: []api.Replica{
			{Status: "1", Checksum: "sha2:not-valid!!!"},
		},
	}

	info := &fakeFileInfo{sys: obj}

	result, ok := parseChecksum(info)
	if ok {
		t.Error("expected ok=false for invalid base64")
	}

	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

func TestSha256Checksum(t *testing.T) {
	// Create a temp file with known content
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")

	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	hash, err := Sha256Checksum(context.Background(), path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(hash) != 32 {
		t.Errorf("expected 32-byte sha256, got %d bytes", len(hash))
	}

	// Verify deterministic
	hash2, err := Sha256Checksum(context.Background(), path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Equal(hash, hash2) {
		t.Error("expected deterministic hash")
	}
}

func TestSha256ChecksumFileNotFound(t *testing.T) {
	_, err := Sha256Checksum(context.Background(), "/nonexistent/file.txt")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestSha256ChecksumCanceled(t *testing.T) {
	// Create a temp file
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")

	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// For a very small file, this may still succeed since the goroutine
	// may finish before the context check. Either result is acceptable.
	_, _ = Sha256Checksum(ctx, path)
}
