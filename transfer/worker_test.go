package transfer

import (
	"bytes"
	"context"
	"io"
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
		Username: testUser,
		Zone:     testZone,
		Connect: func(context.Context) (api.Conn, error) {
			return testConn0, nil
		},
		DefaultResource: testDemoResc,
	}

	testConn0.AddResponse(msg.EmptyResponse{}) // mkdir
	testConn0.AddResponses(responses)          // walk

	testConn1 := &api.MockConn{}
	testConn2 := &api.MockConn{}

	var n int

	testTransferAPI := &api.API{
		Username: testUser,
		Zone:     testZone,
		Connect: func(context.Context) (api.Conn, error) {
			n++

			if n%2 == 1 {
				return testConn1, nil
			}

			return testConn2, nil
		},
		DefaultResource: testDemoResc,
	}

	testConn1.Add(msg.DATA_OBJ_UNLINK_AN, msg.DataObjectRequest{
		Path: testFile1,
	}, msg.EmptyResponse{})

	kv := msg.SSKeyVal{}
	kv.Add(msg.DATA_TYPE_KW, "generic")
	kv.Add(msg.DEST_RESC_NAME_KW, testDemoResc)
	testConn2.Add(msg.DATA_OBJ_OPEN_AN, msg.DataObjectRequest{
		Path:       testFile2,
		CreateMode: 420,
		OpenFlags:  577,
		KeyVals:    kv,
	}, msg.FileDescriptor(1))
	testConn2.Add(msg.GET_FILE_DESCRIPTOR_INFO_APN, msg.GetDescriptorInfoRequest{
		FileDescriptor: 1,
	}, msg.GetDescriptorInfoResponse{
		DataObjectInfo: map[string]any{
			testReplicaNumber: 1,
			testResourceHier:  testBlub,
		},
		ReplicaToken: testToken,
	})
	testConn2.AddBuffer(msg.DATA_OBJ_WRITE_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
		Size:           100,
	}, msg.EmptyResponse{}, bytes.Repeat([]byte(testStr), 25), nil)
	testConn2.AddBuffer(msg.DATA_OBJ_WRITE_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
		Size:           100,
	}, msg.EmptyResponse{}, bytes.Repeat([]byte(testStr), 25), nil)

	testConn2.Add(msg.DATA_OBJ_CLOSE_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
	}, msg.EmptyResponse{})

	kv = msg.SSKeyVal{}
	kv.Add(msg.RESC_HIER_STR_KW, testBlub)
	kv.Add(msg.REPLICA_TOKEN_KW, testToken)
	testConn1.Add(msg.DATA_OBJ_OPEN_AN, msg.DataObjectRequest{
		Path:      testFile2,
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
	}, msg.EmptyResponse{}, bytes.Repeat([]byte(testStr), 25), nil)
	testConn1.AddBuffer(msg.DATA_OBJ_WRITE_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 2,
		Size:           100,
	}, msg.EmptyResponse{}, bytes.Repeat([]byte(testStr), 25), nil)
	testConn1.Add(msg.REPLICA_CLOSE_APN, msg.CloseDataObjectReplicaRequest{
		FileDescriptor: 2,
	}, msg.EmptyResponse{})

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "file2"), bytes.Repeat([]byte(testStr), 100), 0o600); err != nil {
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

func TestFromStream(t *testing.T) { //nolint:funlen
	testConn1 := &api.MockConn{}
	testConn2 := &api.MockConn{}

	var n int

	testTransferAPI := &api.API{
		Username: testUser,
		Zone:     testZone,
		Connect: func(context.Context) (api.Conn, error) {
			n++

			if n%2 == 1 {
				return testConn1, nil
			}

			return testConn2, nil
		},
		DefaultResource: testDemoResc,
	}

	kv := msg.SSKeyVal{}
	kv.Add(msg.DATA_TYPE_KW, "generic")
	kv.Add(msg.DEST_RESC_NAME_KW, testDemoResc)
	testConn1.Add(msg.DATA_OBJ_OPEN_AN, msg.DataObjectRequest{
		Path:       testFile2,
		CreateMode: 420,
		OpenFlags:  577,
		KeyVals:    kv,
	}, msg.FileDescriptor(1))
	// The order of the next two calls is not defined, but usually it is this
	testConn1.Add(msg.GET_FILE_DESCRIPTOR_INFO_APN, msg.GetDescriptorInfoRequest{
		FileDescriptor: 1,
	}, msg.GetDescriptorInfoResponse{
		DataObjectInfo: map[string]any{
			testReplicaNumber: 1,
			testResourceHier:  testBlub,
		},
		ReplicaToken: testToken,
	})
	testConn1.AddBuffer(msg.DATA_OBJ_WRITE_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
		Size:           100,
	}, msg.EmptyResponse{}, bytes.Repeat([]byte(testStr), 25), nil)
	testConn1.Add(msg.DATA_OBJ_LSEEK_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
		Offset:         200,
	}, msg.SeekResponse{Offset: 0})
	testConn1.AddBuffer(msg.DATA_OBJ_WRITE_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
		Size:           100,
	}, msg.EmptyResponse{}, bytes.Repeat([]byte(testStr), 25), nil)
	testConn1.Add(msg.DATA_OBJ_CLOSE_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
	}, msg.EmptyResponse{})

	kv = msg.SSKeyVal{}
	kv.Add(msg.RESC_HIER_STR_KW, testBlub)
	kv.Add(msg.REPLICA_TOKEN_KW, testToken)
	testConn2.Add(msg.DATA_OBJ_OPEN_AN, msg.DataObjectRequest{
		Path:      testFile2,
		OpenFlags: 1,
		KeyVals:   kv,
	}, msg.FileDescriptor(2))
	testConn2.Add(msg.DATA_OBJ_LSEEK_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 2,
		Offset:         100,
	}, msg.SeekResponse{Offset: 100})
	testConn2.AddBuffer(msg.DATA_OBJ_WRITE_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 2,
		Size:           100,
	}, msg.EmptyResponse{}, bytes.Repeat([]byte(testStr), 25), nil)
	testConn2.Add(msg.DATA_OBJ_LSEEK_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 2,
		Offset:         300,
	}, msg.SeekResponse{Offset: 200})
	testConn2.AddBuffer(msg.DATA_OBJ_WRITE_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 2,
		Size:           100,
	}, msg.EmptyResponse{}, bytes.Repeat([]byte(testStr), 25), nil)
	testConn2.Add(msg.REPLICA_CLOSE_APN, msg.CloseDataObjectReplicaRequest{
		FileDescriptor: 2,
	}, msg.EmptyResponse{})

	BufferSize = 100
	CopyBufferDelay = 500 * time.Millisecond

	worker := New(testTransferAPI, testTransferAPI, Options{
		MaxThreads: 2,
		Output:     os.Stdout,
		Delete:     true,
	})

	worker.FromStream(t.Context(), "stream", bytes.NewReader(bytes.Repeat([]byte(testStr), 100)), testFile2, false)

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
			Username: testUser,
			Zone:     testZone,
			Connect: func(context.Context) (api.Conn, error) {
				return testConn0, nil
			},
			DefaultResource: testDemoResc,
		}

		testConn0.AddResponses(responses) // walk

		testConn1 := &api.MockConn{}

		testTransferAPI := &api.API{
			Username: testUser,
			Zone:     testZone,
			Connect: func(context.Context) (api.Conn, error) {
				return testConn1, nil
			},
			DefaultResource: testDemoResc,
		}

		kv := msg.SSKeyVal{}
		kv.Add(msg.DATA_TYPE_KW, "generic")
		kv.Add(msg.DEST_RESC_NAME_KW, testDemoResc)
		testConn1.Add(msg.DATA_OBJ_OPEN_AN, msg.DataObjectRequest{
			Path:       testFile1,
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
		}, msg.ReadResponse(4), nil, []byte(testStr))
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
		} else if string(contents) != testStr {
			t.Errorf("expected 'test', got '%s'", string(contents))
		}
	}
}

func TestToStream(t *testing.T) { //nolint:funlen
	testConn1 := &api.MockConn{}
	testConn2 := &api.MockConn{}

	var n int

	testTransferAPI := &api.API{
		Username: testUser,
		Zone:     testZone,
		Connect: func(context.Context) (api.Conn, error) {
			n++

			if n%2 == 1 {
				return testConn1, nil
			}

			return testConn2, nil
		},
		DefaultResource: testDemoResc,
	}

	kv := msg.SSKeyVal{}
	kv.Add(msg.DATA_TYPE_KW, "generic")
	kv.Add(msg.DEST_RESC_NAME_KW, testDemoResc)
	testConn1.Add(msg.DATA_OBJ_OPEN_AN, msg.DataObjectRequest{
		Path:       testFile1,
		CreateMode: 420,
		KeyVals:    kv,
	}, msg.FileDescriptor(1))
	testConn1.Add(msg.DATA_OBJ_LSEEK_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
		Whence:         2,
	}, msg.SeekResponse{Offset: 304})
	testConn1.Add(msg.DATA_OBJ_LSEEK_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
	}, msg.SeekResponse{Offset: 0})
	// The following two are run in unspecified order, but usually in this order
	testConn1.Add(msg.GET_FILE_DESCRIPTOR_INFO_APN, msg.GetDescriptorInfoRequest{
		FileDescriptor: 1,
	}, msg.GetDescriptorInfoResponse{
		DataObjectInfo: map[string]any{
			testReplicaNumber: 1,
			testResourceHier:  testBlub,
		},
		ReplicaToken: testToken,
	})
	testConn1.AddBuffer(msg.DATA_OBJ_READ_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
		Size:           100,
	}, msg.ReadResponse(100), nil, bytes.Repeat([]byte(testStr), 25))
	testConn1.Add(msg.DATA_OBJ_LSEEK_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
		Offset:         200,
	}, msg.SeekResponse{Offset: 200})
	testConn1.AddBuffer(msg.DATA_OBJ_READ_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
		Size:           100,
	}, msg.ReadResponse(100), nil, bytes.Repeat([]byte(testStr), 25))
	testConn1.Add(msg.DATA_OBJ_CLOSE_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
	}, msg.EmptyResponse{})

	kv = msg.SSKeyVal{}
	kv.Add(msg.RESC_HIER_STR_KW, testBlub)
	kv.Add(msg.REPLICA_TOKEN_KW, testToken)
	testConn2.Add(msg.DATA_OBJ_OPEN_AN, msg.DataObjectRequest{
		Path:    testFile1,
		KeyVals: kv,
	}, msg.FileDescriptor(2))
	testConn2.Add(msg.DATA_OBJ_LSEEK_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 2,
		Offset:         100,
	}, msg.SeekResponse{Offset: 100})
	testConn2.AddBuffer(msg.DATA_OBJ_READ_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 2,
		Size:           100,
	}, msg.ReadResponse(100), nil, bytes.Repeat([]byte(testStr), 25))
	testConn2.Add(msg.DATA_OBJ_LSEEK_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 2,
		Offset:         300,
	}, msg.SeekResponse{Offset: 300})
	testConn2.AddBuffer(msg.DATA_OBJ_READ_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 2,
		Size:           100,
	}, msg.ReadResponse(4), nil, []byte("stop"))
	testConn2.Add(msg.REPLICA_CLOSE_APN, msg.CloseDataObjectReplicaRequest{
		FileDescriptor: 2,
	}, msg.EmptyResponse{})

	BufferSize = 100
	CopyBufferDelay = 500 * time.Millisecond

	worker := New(nil, testTransferAPI, Options{
		MaxThreads: 2,
	})

	worker.ToStream(t.Context(), testStr, io.Discard, testFile1)

	if err := worker.Wait(); err != nil {
		t.Error(err)
	}
}

func TestClientVerify(t *testing.T) {
	testConn := &api.MockConn{}

	testAPI := &api.API{
		Username: testUser,
		Zone:     testZone,
		Connect: func(context.Context) (api.Conn, error) {
			return testConn, nil
		},
		DefaultResource: testDemoResc,
	}

	kv := msg.SSKeyVal{}
	kv.Add(msg.DEST_RESC_NAME_KW, testDemoResc)

	testConn.Add(msg.DATA_OBJ_CHKSUM_AN, msg.DataObjectRequest{
		Path:    testFile1,
		KeyVals: kv,
	}, msg.String{
		String: testSha,
	})

	f, err := os.CreateTemp(t.TempDir(), testStr)
	if err != nil {
		t.Fatal(err)
	}

	defer os.Remove(f.Name())

	_, err = f.Write(bytes.Repeat([]byte(testStr), 100))
	if err != nil {
		t.Fatal(err)
	}

	fi, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}

	if err = f.Close(); err != nil {
		t.Fatal(err)
	}

	obj := &api.DataObject{
		Replicas: []api.Replica{},
	}

	if _, _, err := VerifyLocalToRemote(testAPI, nil)(t.Context(), f.Name(), testFile1, fi, obj); err != nil {
		t.Error(err)
	}
}

func TestClientVerifyRemote(t *testing.T) {
	testConn := &api.MockConn{}

	testAPI := &api.API{
		Username: testUser,
		Zone:     testZone,
		Connect: func(context.Context) (api.Conn, error) {
			return testConn, nil
		},
		DefaultResource: testDemoResc,
	}

	kv := msg.SSKeyVal{}
	kv.Add(msg.DEST_RESC_NAME_KW, testDemoResc)

	for range 2 {
		testConn.AddResponse(msg.String{
			String: testSha,
		})
	}

	obj := &api.DataObject{
		Replicas: []api.Replica{},
	}

	if _, _, err := VerifyRemoteToRemote(testAPI, nil)(t.Context(), testFile1, testFile2, obj, obj); err != nil {
		t.Error(err)
	}
}

func TestClientRemoveDir(t *testing.T) {
	testConn0 := &api.MockConn{}

	testIndexAPI := &api.API{
		Username: testUser,
		Zone:     testZone,
		Connect: func(context.Context) (api.Conn, error) {
			return testConn0, nil
		},
		DefaultResource: testDemoResc,
	}

	testConn0.AddResponses(responses) // walk

	testConn1 := &api.MockConn{}

	testTransferAPI := &api.API{
		Username: testUser,
		Zone:     testZone,
		Connect: func(context.Context) (api.Conn, error) {
			return testConn1, nil
		},
		DefaultResource: testDemoResc,
	}

	testConn1.Add(msg.DATA_OBJ_UNLINK_AN, msg.DataObjectRequest{
		Path: testFile1,
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

func TestClientComputeChecksums(t *testing.T) {
	testConn0 := &api.MockConn{}

	testIndexAPI := &api.API{
		Username: testUser,
		Zone:     testZone,
		Connect: func(context.Context) (api.Conn, error) {
			return testConn0, nil
		},
		DefaultResource: testDemoResc,
	}

	testConn0.AddResponses(responses) // walk

	testConn1 := &api.MockConn{}

	testTransferAPI := &api.API{
		Username: testUser,
		Zone:     testZone,
		Connect: func(context.Context) (api.Conn, error) {
			return testConn1, nil
		},
		DefaultResource: testDemoResc,
	}

	testConn1.Add(msg.DATA_OBJ_CHKSUM_AN, msg.DataObjectRequest{
		Path: testFile1,
		KeyVals: msg.SSKeyVal{
			Length: 2,
			Keys: []msg.KeyWord{
				"destRescName",
				"forceChksum",
			},
			Values: []string{
				testDemoResc,
				"",
			},
		},
	}, msg.String{
		String: testSha,
	})

	worker := New(testIndexAPI, testTransferAPI, Options{
		MaxThreads:         1,
		IntegrityChecksums: true,
	})

	worker.ComputeChecksums(t.Context(), "/test")

	if err := worker.Wait(); err != nil {
		t.Error(err)
	}
}

func TestClientCopyDir(t *testing.T) {
	for range 10 {
		testConn0 := &api.MockConn{}

		var i atomic.Int32

		testIndexAPI := &api.API{
			Username: testUser,
			Zone:     testZone,
			Connect: func(context.Context) (api.Conn, error) {
				if count := i.Add(1); count == 2 || count == 3 {
					// Deliberately sleep for first two calls to order
					// both calls to Walk
					time.Sleep(time.Duration(count) * time.Second / 10)
				}

				return testConn0, nil
			},
			DefaultResource: testDemoResc,
		}

		testConn0.AddResponse(msg.EmptyResponse{}) // mkdir
		testConn0.AddResponses(responses)          // walk 1
		testConn0.AddResponses(responses[:2])      // walk 2
		testConn0.AddResponse(msg.QueryResponse{}) // walk 2

		testConn1 := &api.MockConn{}

		testTransferAPI := &api.API{
			Username: testUser,
			Zone:     testZone,
			Connect: func(context.Context) (api.Conn, error) {
				return testConn1, nil
			},
			DefaultResource: testDemoResc,
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

func TestWorkerCopyCollectionMetadata(t *testing.T) {
	testConn := &api.MockConn{}

	testAPI := &api.API{
		Username: testUser,
		Zone:     testZone,
		Connect: func(context.Context) (api.Conn, error) {
			return testConn, nil
		},
		DefaultResource: testDemoResc,
	}

	// With CopyMetadata enabled, copyCollection should create the collection
	// and copy the AVU metadata (two requests).
	testConn.AddResponse(msg.EmptyResponse{}) // CreateCollection
	testConn.AddResponse(msg.EmptyResponse{}) // CopyMetadata

	worker := New(testAPI, testAPI, Options{
		MaxThreads:   1,
		CopyMetadata: true,
	})

	if err := worker.copyCollection(t.Context(), Task{Path: testCopySrc, IrodsPath: testCopyDest}); err != nil {
		t.Fatal(err)
	}

	if len(testConn.Dialog) != 0 {
		t.Errorf("expected all dialog to be consumed, %d remaining", len(testConn.Dialog))
	}
}

func TestWorkerCopyCollectionWithoutMetadata(t *testing.T) {
	testConn := &api.MockConn{}

	testAPI := &api.API{
		Username: testUser,
		Zone:     testZone,
		Connect: func(context.Context) (api.Conn, error) {
			return testConn, nil
		},
		DefaultResource: testDemoResc,
	}

	// Without CopyMetadata, copyCollection should only create the collection.
	testConn.AddResponse(msg.EmptyResponse{}) // CreateCollection

	worker := New(testAPI, testAPI, Options{
		MaxThreads: 1,
	})

	if err := worker.copyCollection(t.Context(), Task{Path: testCopySrc, IrodsPath: testCopyDest}); err != nil {
		t.Fatal(err)
	}

	if len(testConn.Dialog) != 0 {
		t.Errorf("expected all dialog to be consumed, %d remaining", len(testConn.Dialog))
	}
}

func TestWorkerCopyActionMetadata(t *testing.T) {
	testConn := &api.MockConn{}

	testAPI := &api.API{
		Username: testUser,
		Zone:     testZone,
		Connect: func(context.Context) (api.Conn, error) {
			return testConn, nil
		},
		DefaultResource: testDemoResc,
	}

	// With CopyMetadata enabled, copyAction should copy the data object and
	// then copy the AVU metadata (two requests).
	testConn.AddResponse(msg.EmptyResponse{}) // CopyDataObject
	testConn.AddResponse(msg.EmptyResponse{}) // CopyMetadata

	worker := New(testAPI, testAPI, Options{
		MaxThreads:   1,
		CopyMetadata: true,
	})

	worker.copyAction(t.Context(), Task{Path: testCopySrc, IrodsPath: testCopyDest, Size: 4})

	if err := worker.Wait(); err != nil {
		t.Fatal(err)
	}

	if len(testConn.Dialog) != 0 {
		t.Errorf("expected all dialog to be consumed, %d remaining", len(testConn.Dialog))
	}
}
