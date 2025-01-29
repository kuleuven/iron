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

func TestCreateCollectionAllAdmin(t *testing.T) {
	testConn.NextResponses = []any{
		&msg.IRODSError{
			Code: -818000,
		},
		&msg.IRODSError{
			Code: -1105000,
		},
		msg.EmptyResponse{},
		msg.EmptyResponse{},
	}

	if err := testAPI.AsAdmin().CreateCollectionAll(context.Background(), "/test/home/test/path/to/folder"); err != nil {
		t.Fatal(err)
	}
}

func TestCreateCollectionAllAdminFailure(t *testing.T) {
	testConn.NextResponses = []any{
		&msg.IRODSError{
			Code: -818000,
		},
		&msg.IRODSError{
			Code: -1,
		},
	}

	if err, ok := testAPI.AsAdmin().CreateCollectionAll(context.Background(), "/test/home/test/path/to/folder").(*msg.IRODSError); !ok || err.Code != -818000 {
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

func TestReplicateDataObject(t *testing.T) {
	testConn.NextResponse = msg.EmptyResponse{}

	if err := testAPI.ReplicateDataObject(context.Background(), "test", "otherResource"); err != nil {
		t.Fatal(err)
	}
}

func TestTrimDataObject(t *testing.T) {
	testConn.NextResponse = msg.EmptyResponse{}

	if err := testAPI.TrimDataObject(context.Background(), "test", "otherResource"); err != nil {
		t.Fatal(err)
	}
}

func TestTrimDataObjectReplica(t *testing.T) {
	testConn.NextResponse = msg.EmptyResponse{}

	if err := testAPI.TrimDataObjectReplica(context.Background(), "test", 0); err != nil {
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

	file, err := testAPI.OpenDataObject(context.Background(), "test", os.O_WRONLY|os.O_TRUNC)
	if err != nil {
		t.Fatal(err)
	}

	size, err := file.Size()
	if err != nil {
		t.Fatal(err)
	}

	if size != 0 {
		t.Fatalf("expected size 0, got %d", size)
	}

	err = file.Touch(time.Now())
	if err != nil {
		t.Fatal(err)
	}

	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenDataObjectSize(t *testing.T) {
	testConn.NextResponses = []any{
		msg.FileDescriptor(1),
		msg.SeekResponse{
			Offset: 100,
		},
		msg.EmptyResponse{},
	}

	testConn.NextBin = []byte("testcontent")

	file, err := testAPI.OpenDataObject(context.Background(), "test", os.O_WRONLY|os.O_APPEND)
	if err != nil {
		t.Fatal(err)
	}

	size, err := file.Size()
	if err != nil {
		t.Fatal(err)
	}

	if size != 100 {
		t.Fatalf("expected size 100, got %d", size)
	}

	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenDataObjectSize2(t *testing.T) {
	testConn.NextResponses = []any{
		msg.FileDescriptor(1),
		msg.SeekResponse{
			Offset: 100,
		},
		msg.SeekResponse{
			Offset: 0,
		},
		msg.EmptyResponse{},
	}

	testConn.NextBin = []byte("testcontent")

	file, err := testAPI.OpenDataObject(context.Background(), "test", os.O_WRONLY)
	if err != nil {
		t.Fatal(err)
	}

	size, err := file.Size()
	if err != nil {
		t.Fatal(err)
	}

	if size != 100 {
		t.Fatalf("expected size 100, got %d", size)
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

func TestModifyAccess(t *testing.T) {
	testConn.NextResponse = msg.EmptyResponse{}

	if err := testAPI.ModifyAccess(context.Background(), "/test", "test", "own", false); err != nil {
		t.Fatal(err)
	}

	testConn.NextResponse = msg.EmptyResponse{}

	if err := testAPI.ModifyAccess(context.Background(), "/test", "test#remoteZone", "own", false); err != nil {
		t.Fatal(err)
	}
}

func TestSetCollectionInheritance(t *testing.T) {
	testConn.NextResponse = msg.EmptyResponse{}

	if err := testAPI.SetCollectionInheritance(context.Background(), "/test", true, true); err != nil {
		t.Fatal(err)
	}

	testConn.NextResponse = msg.EmptyResponse{}

	if err := testAPI.SetCollectionInheritance(context.Background(), "/test", false, false); err != nil {
		t.Fatal(err)
	}
}

func TestAddMetadata(t *testing.T) {
	testConn.NextResponse = msg.EmptyResponse{}

	if err := testAPI.AddMetadata(context.Background(), "/test", CollectionType, Metadata{
		Name:  "test",
		Value: "test",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveMetadata(t *testing.T) {
	testConn.NextResponse = msg.EmptyResponse{}

	if err := testAPI.RemoveMetadata(context.Background(), "/test", CollectionType, Metadata{
		Name:  "test",
		Value: "test",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSetMetadata(t *testing.T) {
	testConn.NextResponse = msg.EmptyResponse{}

	if err := testAPI.SetMetadata(context.Background(), "/test", CollectionType, Metadata{
		Name:  "test",
		Value: "test",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestModifyMetadata(t *testing.T) {
	testConn.NextResponse = msg.EmptyResponse{}

	add := []Metadata{
		{
			Name:  "test",
			Value: "test",
		},
	}

	remove := []Metadata{
		{
			Name:  "test2",
			Value: "test",
		},
	}

	if err := testAPI.ModifyMetadata(context.Background(), "/test", CollectionType, add, remove); err != nil {
		t.Fatal(err)
	}
}
