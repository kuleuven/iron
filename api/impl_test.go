package api

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"gitea.icts.kuleuven.be/coz/iron/msg"
)

func TestCreateCollection(t *testing.T) {
	testConn.NextResponse = msg.EmptyResponse{}

	if err := testAPI.CreateCollection(context.Background(), "test"); err != nil {
		t.Fatal(err)
	}
}

func TestCreateCollectionAll(t *testing.T) {
	testConn.NextResponse = msg.EmptyResponse{}

	if err := testAPI.CreateCollectionAll(context.Background(), "test"); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteCollection(t *testing.T) {
	testConn.NextResponse = msg.CollectionOperationStat{}

	if err := testAPI.DeleteCollection(context.Background(), "test", true); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteCollectionAll(t *testing.T) {
	testConn.NextResponse = msg.CollectionOperationStat{}

	if err := testAPI.DeleteCollectionAll(context.Background(), "test", true); err != nil {
		t.Fatal(err)
	}
}

func TestRenameCollection(t *testing.T) {
	testConn.NextResponse = msg.EmptyResponse{}

	if err := testAPI.RenameCollection(context.Background(), "test", "test2"); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteDataObject(t *testing.T) {
	testConn.NextResponse = msg.EmptyResponse{}

	if err := testAPI.DeleteDataObject(context.Background(), "test", true); err != nil {
		t.Fatal(err)
	}
}

func TestRenameDataObject(t *testing.T) {
	testConn.NextResponse = msg.EmptyResponse{}

	if err := testAPI.RenameDataObject(context.Background(), "test", "test2"); err != nil {
		t.Fatal(err)
	}
}

func TestCopyDataObject(t *testing.T) {
	testConn.NextResponse = msg.EmptyResponse{}

	if err := testAPI.CopyDataObject(context.Background(), "test", "test2"); err != nil {
		t.Fatal(err)
	}
}

func TestOpenDataObject(t *testing.T) {
	testConn.NextResponses = []any{
		msg.FileDescriptor(1),
		msg.SeekResponse{
			Offset: 100,
		},
		msg.ReadResponse(11),
		msg.EmptyResponse{},
		msg.EmptyResponse{},
	}

	testConn.NextBin = []byte("testcontent")

	file, err := testAPI.OpenDataObject(context.Background(), "test", os.O_WRONLY|os.O_APPEND)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := file.Read(make([]byte, 15)); err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}

	if _, err := file.Write([]byte("test")); err != nil {
		t.Fatal(err)
	}

	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTouchDataObject(t *testing.T) {
	testConn.NextResponses = []any{
		msg.FileDescriptor(1),
		msg.GetDescriptorInfoResponse{
			DataObjectInfo: map[string]any{
				"replica_number":     1,
				"resource_hierarchy": "string",
			},
		},
		msg.EmptyResponse{},
		msg.EmptyResponse{},
	}

	testConn.NextBin = []byte("testcontent")

	file, err := testAPI.OpenDataObject(context.Background(), "test", os.O_WRONLY)
	if err != nil {
		t.Fatal(err)
	}

	err = file.Touch(time.Now())
	if err != nil {
		t.Fatal(err)
	}

	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTruncateDataObject(t *testing.T) {
	testConn.NextResponses = []any{
		msg.FileDescriptor(1),
		msg.GetDescriptorInfoResponse{
			DataObjectInfo: map[string]any{
				"replica_number":     1,
				"resource_hierarchy": "string",
			},
		},
		msg.EmptyResponse{},
		msg.EmptyResponse{},
	}

	testConn.NextBin = []byte("testcontent")

	file, err := testAPI.OpenDataObject(context.Background(), "test", os.O_WRONLY)
	if err != nil {
		t.Fatal(err)
	}

	err = file.Truncate(10)
	if err != nil {
		t.Fatal(err)
	}

	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateDataObjectTruncate(t *testing.T) {
	testConn.NextResponses = []any{
		msg.FileDescriptor(1),
		msg.GetDescriptorInfoResponse{
			DataObjectInfo: map[string]any{
				"replica_number":     1,
				"resource_hierarchy": "string",
			},
		},
		msg.GetDescriptorInfoResponse{
			DataObjectInfo: map[string]any{
				"replica_number":     1,
				"resource_hierarchy": "string",
			},
		},
		msg.EmptyResponse{},
		msg.EmptyResponse{},
	}

	testConn2 := &mockConn{}

	testConn2.NextResponses = []any{
		msg.FileDescriptor(2),
		msg.EmptyResponse{},
	}

	file, err := testAPI.CreateDataObject(context.Background(), "test", os.O_CREATE|os.O_WRONLY)
	if err != nil {
		t.Fatal(err)
	}

	file2, err := file.Reopen(testConn2, os.O_WRONLY)
	if err != nil {
		t.Fatal(err)
	}

	err = file2.Truncate(10)
	if err != nil {
		t.Fatal(err)
	}

	ch := make(chan error)

	go func() {
		ch <- file.Close()
	}()

	go func() {
		ch <- file2.Close()
	}()

	if err := <-ch; err != nil {
		t.Fatal(err)
	}

	if err := <-ch; err != nil {
		t.Fatal(err)
	}
}
