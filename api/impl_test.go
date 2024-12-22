package api

import (
	"context"
	"testing"

	"gitea.icts.kuleuven.be/coz/iron/msg"
)

func TestCreateCollection(t *testing.T) {
	testConn.NextResponse = msg.CreateCollectionResponse{}

	if err := testAPI.Admin().CreateCollection(context.Background(), "test"); err != nil {
		t.Fatal(err)
	}
}

func TestCreateCollectionAll(t *testing.T) {
	testConn.NextResponse = msg.CreateCollectionResponse{}

	if err := testAPI.Admin().CreateCollectionAll(context.Background(), "test"); err != nil {
		t.Fatal(err)
	}
}
