package transfer

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kuleuven/iron/api"
	"github.com/kuleuven/iron/msg"
)

func testVerifyLocalToRemote(t *testing.T, localHash, remoteHash []byte, localErr, remoteErr, wantErr error) {
	local, remote, err := compareChecksums(t.Context(),
		func(context.Context) ([]byte, error) { return localHash, localErr },
		func(context.Context) ([]byte, error) { return remoteHash, remoteErr })
	if !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want %v", err, wantErr)
	}

	if localErr != nil || remoteErr != nil {
		if local != nil || remote != nil {
			t.Fatal("expected nil checksums on computation error")
		}

		return
	}

	if !bytes.Equal(local, localHash) || !bytes.Equal(remote, remoteHash) {
		t.Fatalf("unexpected checksums: local %x, remote %x", local, remote)
	}

	if errors.Is(err, ErrChecksumMismatch) {
		want := fmt.Sprintf("%s: source: %s target: %s", ErrChecksumMismatch,
			base64.StdEncoding.EncodeToString(localHash), base64.StdEncoding.EncodeToString(remoteHash))
		if err.Error() != want {
			t.Errorf("got %q, want %q", err, want)
		}
	}
}

func TestVerifyLocalToRemoteCallbacks(t *testing.T) {
	checksumErr := errors.New("checksum failed")
	localHash := []byte(testStr)
	otherHash := []byte(testBlub)

	for _, tt := range []struct {
		name                string
		remoteHash          []byte
		localErr, remoteErr error
		wantErr             error
	}{
		{name: "match", remoteHash: localHash},
		{name: "mismatch", remoteHash: otherHash, wantErr: ErrChecksumMismatch},
		{name: "local error", localErr: checksumErr, wantErr: checksumErr},
		{name: "remote error", remoteErr: checksumErr, wantErr: checksumErr},
	} {
		t.Run(tt.name, func(t *testing.T) {
			testVerifyLocalToRemote(t, localHash, tt.remoteHash, tt.localErr, tt.remoteErr, tt.wantErr)
		})
	}
}

type checksumTestFile struct {
	api.File
	conn     api.Conn
	closeErr error
}

func (f checksumTestFile) Name() string { return testFile1 }

func (f checksumTestFile) CloseReturnConnection() (api.Conn, error) {
	return f.conn, f.closeErr
}

func (f checksumTestFile) Close() error {
	return f.conn.Close()
}

type checksumTestConn struct {
	api.MockConn
	closed   bool
	closeErr error
}

func (c *checksumTestConn) Close() error {
	c.closed = true

	return c.closeErr
}

func TestVerifyChecksumAndClose(t *testing.T) {
	checksum, err := api.ParseIrodsChecksum(testSha)
	if err != nil {
		t.Fatal(err)
	}

	closeErr := errors.New("close failed")
	requestErr := errors.New("checksum request failed")
	callbackErr := errors.New("checksum callback failed")

	for _, tt := range []struct {
		name                 string
		disabled             bool
		fileErr, connErr     error
		callbackErr, wantErr error
		response             any
	}{
		{name: "match", response: msg.String{String: testSha}},
		{name: "mismatch", response: msg.String{String: "sha2:AAAA"}, wantErr: ErrChecksumMismatch},
		{name: "disabled", disabled: true},
		{name: "connection close error", connErr: closeErr, response: msg.String{String: testSha}, wantErr: closeErr},
		{name: "request error", response: requestErr, wantErr: requestErr},
		{name: "callback error", response: msg.String{String: testSha}, callbackErr: callbackErr, wantErr: callbackErr},
	} {
		t.Run(tt.name, func(t *testing.T) {
			conn := &checksumTestConn{closeErr: tt.connErr}
			if tt.response != nil {
				conn.Add(msg.DATA_OBJ_CHKSUM_AN, msg.DataObjectRequest{Path: testFile1}, tt.response)
			}

			called := false
			err := verifyChecksumAndClose(t.Context(), !tt.disabled, func(context.Context) ([]byte, error) {
				called = true

				return checksum, tt.callbackErr
			}, checksumTestFile{conn: conn, closeErr: tt.fileErr})

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got error %v, want %v", err, tt.wantErr)
			}

			if !conn.closed || len(conn.Dialog) != 0 {
				t.Fatal("connection was not closed or checksum request was not consumed")
			}

			if called != (!tt.disabled && tt.fileErr == nil) {
				t.Errorf("unexpected checksum callback invocation: %v", called)
			}
		})
	}
}

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

func (f *fakeFileInfo) Name() string       { return testStr }
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
