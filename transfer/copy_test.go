package transfer

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/kuleuven/iron/api"
	"github.com/kuleuven/iron/msg"
)

func TestClientUpload(t *testing.T) { //nolint:funlen
	testConn1 := &api.MockConn{}
	testConn2 := &api.MockConn{}

	var n int

	testAPI := &api.API{
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

	kv := msg.SSKeyVal{}
	kv.Add(msg.DATA_TYPE_KW, "generic")
	kv.Add(msg.DEST_RESC_NAME_KW, "demoResc")
	testConn1.Add(msg.DATA_OBJ_OPEN_AN, msg.DataObjectRequest{
		Path:       "test",
		CreateMode: 420,
		OpenFlags:  577,
		KeyVals:    kv,
	}, msg.FileDescriptor(1))
	testConn1.Add(msg.GET_FILE_DESCRIPTOR_INFO_APN, msg.GetDescriptorInfoRequest{
		FileDescriptor: 1,
	}, msg.GetDescriptorInfoResponse{
		DataObjectInfo: map[string]any{
			"replica_number":     1,
			"resource_hierarchy": "blub",
		},
		ReplicaToken: "testToken",
	})
	testConn1.AddBuffer(msg.DATA_OBJ_WRITE_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
		Size:           100,
	}, msg.EmptyResponse{}, bytes.Repeat([]byte("test"), 25), nil)
	testConn1.AddBuffer(msg.DATA_OBJ_WRITE_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
		Size:           100,
	}, msg.EmptyResponse{}, bytes.Repeat([]byte("test"), 25), nil)

	testConn1.Add(msg.DATA_OBJ_CLOSE_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
	}, msg.EmptyResponse{})

	kv = msg.SSKeyVal{}
	kv.Add(msg.RESC_HIER_STR_KW, "blub")
	kv.Add(msg.REPLICA_TOKEN_KW, "testToken")
	testConn2.Add(msg.DATA_OBJ_OPEN_AN, msg.DataObjectRequest{
		Path:      "test",
		OpenFlags: 1,
		KeyVals:   kv,
	}, msg.FileDescriptor(2))
	testConn2.Add(msg.DATA_OBJ_LSEEK_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 2,
		Offset:         200,
	}, msg.SeekResponse{Offset: 200})
	testConn2.AddBuffer(msg.DATA_OBJ_WRITE_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 2,
		Size:           100,
	}, msg.EmptyResponse{}, bytes.Repeat([]byte("test"), 25), nil)
	testConn2.AddBuffer(msg.DATA_OBJ_WRITE_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 2,
		Size:           100,
	}, msg.EmptyResponse{}, bytes.Repeat([]byte("test"), 25), nil)
	testConn2.Add(msg.REPLICA_CLOSE_APN, msg.CloseDataObjectReplicaRequest{
		FileDescriptor: 2,
	}, msg.EmptyResponse{})

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

	BufferSize = 100
	MinimumRangeSize = 200
	CopyBufferDelay = 500 * time.Millisecond

	worker := New(testAPI, testAPI, Options{
		MaxThreads: 2,
	})

	worker.Upload(context.Background(), f.Name(), "test")

	if err := worker.Wait(); err != nil {
		t.Error(err)
	}
}

func TestClientDownload(t *testing.T) {
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
	kv.Add(msg.DATA_TYPE_KW, "generic")
	kv.Add(msg.DEST_RESC_NAME_KW, "demoResc")
	testConn.Add(msg.DATA_OBJ_OPEN_AN, msg.DataObjectRequest{
		Path:       "test",
		CreateMode: 420,
		KeyVals:    kv,
	}, msg.FileDescriptor(1))
	testConn.Add(msg.DATA_OBJ_LSEEK_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
		Whence:         2,
	}, msg.SeekResponse{Offset: 4})
	testConn.Add(msg.DATA_OBJ_LSEEK_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
	}, msg.SeekResponse{Offset: 0})
	testConn.AddBuffer(msg.DATA_OBJ_READ_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
		Size:           100,
	}, msg.ReadResponse(4), nil, []byte("test"))
	testConn.Add(msg.DATA_OBJ_CLOSE_AN, msg.OpenedDataObjectRequest{
		FileDescriptor: 1,
	}, msg.EmptyResponse{})

	f, err := os.CreateTemp(t.TempDir(), "test")
	if err != nil {
		t.Fatal(err)
	}

	defer os.Remove(f.Name())

	if err = f.Close(); err != nil {
		t.Fatal(err)
	}

	BufferSize = 100
	MinimumRangeSize = 200

	worker := New(testAPI, testAPI, Options{
		MaxThreads: 1,
	})

	worker.Download(context.Background(), f.Name(), "test")

	if err := worker.Wait(); err != nil {
		t.Error(err)
	}

	if contents, err := os.ReadFile(f.Name()); err != nil {
		t.Fatal(err)
	} else if string(contents) != "test" {
		t.Errorf("expected 'test', got '%s'", string(contents))
	}
}
