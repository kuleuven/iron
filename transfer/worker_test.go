package transfer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kuleuven/iron/api"
	"github.com/kuleuven/iron/msg"
)

var responses = []any{
	msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 6,
		TotalRowCount:  1,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 500, ResultLen: 1, Values: []string{"1"}},
			{AttributeIndex: 503, ResultLen: 1, Values: []string{"/test"}},
			{AttributeIndex: 504, ResultLen: 1, Values: []string{"zone"}},
			{AttributeIndex: 508, ResultLen: 1, Values: []string{"10000"}},
			{AttributeIndex: 509, ResultLen: 1, Values: []string{"2024"}},
			{AttributeIndex: 506, ResultLen: 1, Values: []string{"1"}},
		},
	},
	msg.QueryResponse{},
	msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 15,
		TotalRowCount:  1,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 401, ResultLen: 2, Values: []string{"4"}},
			{AttributeIndex: 403, ResultLen: 2, Values: []string{"file1"}},
			{AttributeIndex: 402, ResultLen: 2, Values: []string{"1"}},
			{AttributeIndex: 406, ResultLen: 2, Values: []string{"generic"}},
			{AttributeIndex: 404, ResultLen: 2, Values: []string{"0"}},
			{AttributeIndex: 407, ResultLen: 2, Values: []string{"4"}},
			{AttributeIndex: 411, ResultLen: 2, Values: []string{"rods"}},
			{AttributeIndex: 412, ResultLen: 2, Values: []string{"zone"}},
			{AttributeIndex: 415, ResultLen: 2, Values: []string{"checksum"}},
			{AttributeIndex: 413, ResultLen: 2, Values: []string{""}},
			{AttributeIndex: 409, ResultLen: 2, Values: []string{"resc1"}},
			{AttributeIndex: 410, ResultLen: 2, Values: []string{"/path1"}},
			{AttributeIndex: 422, ResultLen: 2, Values: []string{"demoResc;resc1"}},
			{AttributeIndex: 419, ResultLen: 2, Values: []string{"10000"}},
			{AttributeIndex: 420, ResultLen: 2, Values: []string{"10000"}},
		},
	},
}

func TestClientUpload(t *testing.T) { //nolint:funlen
	testConn0 := &api.MockConn{}

	testIndexAPI := &api.API{
		Username: "testuser",
		Zone:     "testzone",
		Connect: func(context.Context) (api.Conn, error) {
			return testConn0, nil
		},
		DefaultResource: "demoResc",
	}

	testConn0.AddResponse(msg.EmptyResponse{}) // mkdir
	testConn0.AddResponses(responses)          // walk

	testConn1 := &api.MockConn{}
	testConn2 := &api.MockConn{}

	var n int

	testTransferAPI := &api.API{
		Username: "testuser",
		Zone:     "testzone",
		Connect: func(context.Context) (api.Conn, error) {
			n++

			if n%2 == 1 {
				return testConn1, nil
			}

			return testConn2, nil
		},
		DefaultResource: "demoResc",
	}

	testConn1.Add(msg.DATA_OBJ_UNLINK_AN, msg.DataObjectRequest{
		Path: "/test/file1",
	}, msg.EmptyResponse{})

	kv := msg.SSKeyVal{}
	kv.Add(msg.DATA_TYPE_KW, "generic")
	kv.Add(msg.DEST_RESC_NAME_KW, "demoResc")
	testConn2.Add(msg.DATA_OBJ_OPEN_AN, msg.DataObjectRequest{
		Path:       "/test/file2",
		CreateMode: 420,
		OpenFlags:  577,
		KeyVals:    kv,
	}, msg.FileDescriptor(1))
	testConn2.Add(msg.GET_FILE_DESCRIPTOR_INFO_APN, msg.GetDescriptorInfoRequest{
		FileDescriptor: 1,
	}, msg.GetDescriptorInfoResponse{
		DataObjectInfo: map[string]any{
			"replica_number":     1,
			"resource_hierarchy": "blub",
		},
		ReplicaToken: "testToken",
	})
	testConn2.AddBuffer(msg.DATA_OBJ_WRITE_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
		Size:           100,
	}, msg.EmptyResponse{}, bytes.Repeat([]byte("test"), 25), nil)
	testConn2.AddBuffer(msg.DATA_OBJ_WRITE_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
		Size:           100,
	}, msg.EmptyResponse{}, bytes.Repeat([]byte("test"), 25), nil)

	testConn2.Add(msg.DATA_OBJ_CLOSE_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
	}, msg.EmptyResponse{})

	kv = msg.SSKeyVal{}
	kv.Add(msg.RESC_HIER_STR_KW, "blub")
	kv.Add(msg.REPLICA_TOKEN_KW, "testToken")
	testConn1.Add(msg.DATA_OBJ_OPEN_AN, msg.DataObjectRequest{
		Path:      "/test/file2",
		OpenFlags: 1,
		KeyVals:   kv,
	}, msg.FileDescriptor(2))
	testConn1.Add(msg.DATA_OBJ_LSEEK_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 2,
		Offset:         200,
	}, msg.SeekResponse{Offset: 200})
	testConn1.AddBuffer(msg.DATA_OBJ_WRITE_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 2,
		Size:           100,
	}, msg.EmptyResponse{}, bytes.Repeat([]byte("test"), 25), nil)
	testConn1.AddBuffer(msg.DATA_OBJ_WRITE_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 2,
		Size:           100,
	}, msg.EmptyResponse{}, bytes.Repeat([]byte("test"), 25), nil)
	testConn1.Add(msg.REPLICA_CLOSE_APN, msg.CloseDataObjectReplicaRequest{
		FileDescriptor: 2,
	}, msg.EmptyResponse{})

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "file2"), bytes.Repeat([]byte("test"), 100), 0o600); err != nil {
		t.Fatal(err)
	}

	BufferSize = 100
	MinimumRangeSize = 200
	CopyBufferDelay = 500 * time.Millisecond

	worker := New(testIndexAPI, testTransferAPI, Options{
		MaxThreads: 2,
		Output:     os.Stdout,
		Delete:     true,
	})

	worker.UploadDir(t.Context(), dir, "/test")

	if err := worker.Wait(); err != nil {
		t.Error(err)
	}
}

func TestClientDownload(t *testing.T) { //nolint:funlen
	dir := t.TempDir()

	os.Mkdir(filepath.Join(dir, "file1"), 0o700)
	os.Mkdir(filepath.Join(dir, "file1/subfolder"), 0o700)

	for i := range 4 {
		testConn0 := &api.MockConn{}

		testIndexAPI := &api.API{
			Username: "testuser",
			Zone:     "testzone",
			Connect: func(context.Context) (api.Conn, error) {
				return testConn0, nil
			},
			DefaultResource: "demoResc",
		}

		testConn0.AddResponses(responses) // walk

		testConn1 := &api.MockConn{}

		testTransferAPI := &api.API{
			Username: "testuser",
			Zone:     "testzone",
			Connect: func(context.Context) (api.Conn, error) {
				return testConn1, nil
			},
			DefaultResource: "demoResc",
		}

		kv := msg.SSKeyVal{}
		kv.Add(msg.DATA_TYPE_KW, "generic")
		kv.Add(msg.DEST_RESC_NAME_KW, "demoResc")
		testConn1.Add(msg.DATA_OBJ_OPEN_AN, msg.DataObjectRequest{
			Path:       "/test/file1",
			CreateMode: 420,
			KeyVals:    kv,
		}, msg.FileDescriptor(1))
		testConn1.Add(msg.DATA_OBJ_LSEEK_AN, msg.OpenedDataObjectRequest{
			FileDescriptor: 1,
			Whence:         2,
		}, msg.SeekResponse{Offset: 4})
		testConn1.Add(msg.DATA_OBJ_LSEEK_AN, msg.OpenedDataObjectRequest{
			FileDescriptor: 1,
		}, msg.SeekResponse{Offset: 0})
		testConn1.AddBuffer(msg.DATA_OBJ_READ_AN, msg.OpenedDataObjectRequest{
			FileDescriptor: 1,
			Size:           100,
		}, msg.ReadResponse(4), nil, []byte("test"))
		testConn1.Add(msg.DATA_OBJ_CLOSE_AN, msg.OpenedDataObjectRequest{
			FileDescriptor: 1,
		}, msg.EmptyResponse{})

		BufferSize = 100
		MinimumRangeSize = 200

		worker := New(testIndexAPI, testTransferAPI, Options{
			MaxThreads: 1,
			Exclusive:  i%2 == 1,
		})

		worker.DownloadDir(t.Context(), dir, "/test")

		if err := worker.Wait(); err != nil {
			t.Error(err)
		}

		if i == 0 {
			continue
		}

		if contents, err := os.ReadFile(filepath.Join(dir, "file1")); err != nil {
			t.Fatal(err)
		} else if string(contents) != "test" {
			t.Errorf("expected 'test', got '%s'", string(contents))
		}
	}
}

func TestClientVerify(t *testing.T) {
	testConn := &api.MockConn{}

	testAPI := &api.API{
		Username: "testuser",
		Zone:     "testzone",
		Connect: func(context.Context) (api.Conn, error) {
			return testConn, nil
		},
		DefaultResource: "demoResc",
	}

	kv := msg.SSKeyVal{}
	kv.Add(msg.DEST_RESC_NAME_KW, "demoResc")

	testConn.Add(msg.DATA_OBJ_CHKSUM_AN, msg.DataObjectRequest{
		Path:    "/test/file1",
		KeyVals: kv,
	}, msg.Checksum{
		Checksum: "sha2:jMuGXraweIxVs1RAFTHRM8Nbk/mrfSZwERQ3YzMHvy8=",
	})

	f, err := os.CreateTemp(t.TempDir(), "test")
	if err != nil {
		t.Fatal(err)
	}

	defer os.Remove(f.Name())

	_, err = f.Write(bytes.Repeat([]byte("test"), 100))
	if err != nil {
		t.Fatal(err)
	}

	if err = f.Close(); err != nil {
		t.Fatal(err)
	}

	if err := Verify(t.Context(), testAPI, f.Name(), "/test/file1"); err != nil {
		t.Error(err)
	}
}

func TestClientRemoveDir(t *testing.T) {
	testConn0 := &api.MockConn{}

	testIndexAPI := &api.API{
		Username: "testuser",
		Zone:     "testzone",
		Connect: func(context.Context) (api.Conn, error) {
			return testConn0, nil
		},
		DefaultResource: "demoResc",
	}

	testConn0.AddResponses(responses) // walk

	testConn1 := &api.MockConn{}

	testTransferAPI := &api.API{
		Username: "testuser",
		Zone:     "testzone",
		Connect: func(context.Context) (api.Conn, error) {
			return testConn1, nil
		},
		DefaultResource: "demoResc",
	}

	testConn1.Add(msg.DATA_OBJ_UNLINK_AN, msg.DataObjectRequest{
		Path: "/test/file1",
	}, msg.EmptyResponse{})

	testConn1.Add(msg.RM_COLL_AN, msg.CreateCollectionRequest{
		Name: "/test",
	}, msg.CollectionOperationStat{})

	worker := New(testIndexAPI, testTransferAPI, Options{
		MaxThreads: 1,
	})

	worker.RemoveDir(t.Context(), "/test")

	if err := worker.Wait(); err != nil {
		t.Error(err)
	}
}

func TestClientCopyDir(t *testing.T) {
	for range 10 {
		testConn0 := &api.MockConn{}

		var i atomic.Int32

		testIndexAPI := &api.API{
			Username: "testuser",
			Zone:     "testzone",
			Connect: func(context.Context) (api.Conn, error) {
				if count := i.Add(1); count == 2 || count == 3 {
					// Deliberately sleep for first two calls to order
					// both calls to Walk
					time.Sleep(time.Duration(count) * time.Second / 10)
				}

				return testConn0, nil
			},
			DefaultResource: "demoResc",
		}

		testConn0.AddResponse(msg.EmptyResponse{}) // mkdir
		testConn0.AddResponses(responses)          // walk 1
		testConn0.AddResponses(responses[:2])      // walk 2
		testConn0.AddResponse(msg.QueryResponse{}) // walk 2

		testConn1 := &api.MockConn{}

		testTransferAPI := &api.API{
			Username: "testuser",
			Zone:     "testzone",
			Connect: func(context.Context) (api.Conn, error) {
				return testConn1, nil
			},
			DefaultResource: "demoResc",
		}

		testConn1.AddResponse(msg.EmptyResponse{}) // Either a copy or a remove

		worker := New(testIndexAPI, testTransferAPI, Options{
			MaxThreads: 1,
			Delete:     true,
		})

		worker.CopyDir(t.Context(), "/test", "/test")

		if err := worker.Wait(); err != nil {
			t.Fatal(err)
		}
	}
}
