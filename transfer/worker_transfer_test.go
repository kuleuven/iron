package transfer

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kuleuven/iron/api"
	"github.com/kuleuven/iron/msg"
)

func TestWorkerFileTransfers(t *testing.T) {
	for _, remoteCopy := range []bool{false, true} {
		name := "ToWriter"
		if remoteCopy {
			name = "RemoteCopy"
		}

		t.Run(name, func(t *testing.T) {
			testWorkerFileTransfers(t, remoteCopy)
		})
	}
}

func TestWorkerFromReader(t *testing.T) {
	for _, tc := range []fileTransferTest{
		{name: "content", data: testStr},
		{name: "empty"},
		{name: "dry run", data: testStr, dryRun: true},
		{name: "integrity verification", data: testStr, integrity: true},
		{name: "checksum mismatch", data: testStr, integrity: true, mismatch: true, wantErr: ErrChecksumMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.runUpload(t)
		})
	}
}

func (tc fileTransferTest) runUpload(t *testing.T) {
	t.Helper()

	local := filepath.Join(t.TempDir(), testLocalTxt)
	if err := os.WriteFile(local, []byte(tc.data), 0o600); err != nil {
		t.Fatal(err)
	}

	r, err := os.Open(local)
	if err != nil {
		t.Fatal(err)
	}

	defer r.Close()

	stat, err := r.Stat()
	if err != nil {
		t.Fatal(err)
	}

	target := tc.target(true)
	targetAPI := mockTransferAPI(target)

	var progress []Progress

	worker := New(targetAPI, targetAPI, Options{
		DryRun:             tc.dryRun,
		IntegrityChecksums: tc.integrity,
		ProgressHandler: func(ctx context.Context, p Progress) {
			progress = append(progress, p)
		},
	})
	worker.FromReader(t.Context(), &fileReader{name: local, stat: stat, File: r}, testCopyDest)

	if err := worker.Wait(); !errors.Is(err, tc.wantErr) {
		t.Fatalf("Wait() = %v, want %v", err, tc.wantErr)
	}

	assertTransferDialogs(t, target)
	tc.assertProgress(t, progress)

	_, err = r.Stat()
	if !tc.dryRun && !errors.Is(err, os.ErrClosed) {
		t.Errorf("reader should be closed after upload, got %v", err)
	}
}

func testWorkerFileTransfers(t *testing.T, remoteCopy bool) {
	t.Helper()

	readErr := errors.New("read failed")
	for _, tc := range []fileTransferTest{
		{name: "content", data: testStr},
		{name: "empty"},
		{name: "dry run", data: testStr, dryRun: true},
		{name: "integrity verification", data: testStr, integrity: true},
		{name: "checksum mismatch", data: testStr, integrity: true, mismatch: true, wantErr: ErrChecksumMismatch},
		{name: "read error", data: testStr, readErr: readErr, wantErr: readErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, remoteCopy)
		})
	}
}

type fileTransferTest struct {
	name      string
	data      string
	dryRun    bool
	integrity bool
	mismatch  bool
	readErr   error
	wantErr   error
}

func (tc fileTransferTest) run(t *testing.T, remoteCopy bool) {
	t.Helper()

	source, target := tc.source(remoteCopy), tc.target(remoteCopy)
	sourceAPI, targetAPI := mockTransferAPI(source), mockTransferAPI(target)

	var progress []Progress

	worker := New(sourceAPI, sourceAPI, Options{
		DryRun:             tc.dryRun,
		IntegrityChecksums: tc.integrity,
		ProgressHandler: func(ctx context.Context, p Progress) {
			progress = append(progress, p)
		},
	})
	worker.RemoteTransferPool = targetAPI
	worker.RemoteIndexPool = targetAPI

	local := filepath.Join(t.TempDir(), testLocalTxt)
	if remoteCopy {
		worker.RemoteCopy(t.Context(), testCopySrc, testCopyDest)
	} else {
		worker.Download(t.Context(), local, testCopySrc)
	}

	if err := worker.Wait(); !errors.Is(err, tc.wantErr) {
		t.Fatalf("Wait() = %v, want %v", err, tc.wantErr)
	}

	assertTransferDialogs(t, source, target)
	tc.assertProgress(t, progress)

	if !remoteCopy && !tc.dryRun {
		want := tc.data
		if tc.readErr != nil {
			want = ""
		}

		assertDownloadedFile(t, local, want, tc.mismatch)
	}
}

func (tc fileTransferTest) assertProgress(t *testing.T, progress []Progress) {
	t.Helper()

	if len(progress) == 0 {
		t.Fatal("no progress reported")
	}

	last := progress[len(progress)-1]
	if last.Size != int64(len(tc.data)) || last.FinishedAt.IsZero() {
		t.Errorf("unexpected final progress: %+v", last)
	}

	if tc.wantErr == nil && last.Transferred != last.Size {
		t.Errorf("transferred = %d, want %d", last.Transferred, last.Size)
	}
}

func (tc fileTransferTest) target(remoteCopy bool) *api.MockConn {
	conn := &api.MockConn{}
	if !remoteCopy || tc.dryRun {
		return conn
	}

	conn.AddResponse(msg.FileDescriptor(2))

	if tc.data != "" && tc.readErr == nil {
		conn.AddBuffer(msg.DATA_OBJ_WRITE_AN, msg.OpenedDataObjectRequest{
			FileDescriptor: 2,
			Size:           len(tc.data),
		}, msg.EmptyResponse{}, []byte(tc.data), nil)
	}

	conn.AddResponse(msg.EmptyResponse{})

	if tc.integrity {
		conn.AddResponse(transferTestChecksum(tc.data, tc.mismatch))
	}

	if tc.mismatch {
		var kv msg.SSKeyVal

		kv.Add(msg.FORCE_FLAG_KW, "")
		conn.Add(msg.DATA_OBJ_UNLINK_AN, msg.DataObjectRequest{
			Path:    testCopyDest,
			KeyVals: kv,
		}, msg.EmptyResponse{})
	}

	return conn
}

func transferTestChecksum(data string, mismatch bool) msg.String {
	if mismatch {
		data += testStr
	}

	hash := sha256.Sum256([]byte(data))

	return msg.String{String: shaPrefix + base64.StdEncoding.EncodeToString(hash[:])}
}

func mockTransferAPI(conn *api.MockConn) *api.API {
	return &api.API{
		Connect: func(context.Context) (api.Conn, error) {
			return conn, nil
		},
	}
}

func (tc fileTransferTest) source(remoteCopy bool) *api.MockConn {
	conn := &api.MockConn{}
	conn.AddResponse(msg.FileDescriptor(1))
	conn.AddResponse(msg.SeekResponse{Offset: int64(len(tc.data))})

	if tc.data != "" {
		conn.AddResponse(msg.SeekResponse{})
	}

	if tc.data != "" && !tc.dryRun {
		var response any = msg.ReadResponse(len(tc.data))

		if tc.readErr != nil {
			response = tc.readErr
		}

		conn.AddBuffer(msg.DATA_OBJ_READ_AN, msg.OpenedDataObjectRequest{
			FileDescriptor: 1,
			Size:           int(BufferSize),
		}, response, nil, []byte(tc.data))
	}

	conn.AddResponse(msg.EmptyResponse{})

	if tc.integrity {
		conn.AddResponse(transferTestChecksum(tc.data, !remoteCopy && tc.mismatch))
	}

	return conn
}

func assertTransferDialogs(t *testing.T, conns ...*api.MockConn) {
	t.Helper()

	for i, conn := range conns {
		if len(conn.Dialog) != 0 {
			t.Errorf("connection %d has %d unconsumed requests", i, len(conn.Dialog))
		}
	}
}

func assertDownloadedFile(t *testing.T, path, want string, removed bool) {
	t.Helper()

	data, err := os.ReadFile(path)
	if removed {
		if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("checksum mismatch should remove file, got %v", err)
		}

		return
	}

	if err != nil {
		t.Fatal(err)
	}

	if string(data) != want {
		t.Errorf("downloaded %q, want %q", data, want)
	}
}
