package api

import (
	"context"
	"io"
	"os"
	"testing"

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

func TestCreateDataObject(t *testing.T) {
	testConn.NextResponse = msg.FileDescriptor(1)

	file, err := testAPI.CreateDataObject(context.Background(), "test", os.O_CREATE|os.O_WRONLY)
	if err != nil {
		t.Fatal(err)
	}

	testConn.NextResponse = msg.EmptyResponse{}

	if err := file.Close(); err != nil {
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
