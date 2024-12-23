package api

import (
	"context"
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
