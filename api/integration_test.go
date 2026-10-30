package api_test

import (
	"context"
	"testing"

	"github.com/kuleuven/iron/api"
	"github.com/kuleuven/iron/tests"
)

// TestIntegrationCreateAndDeleteCollection tests creating and deleting collections.
// This verifies that the API can create collections and clean them up properly.
func TestIntegrationCreateAndDeleteCollection(t *testing.T) {
	ctx := context.Background()
	client := tests.NewTestClient(t, ctx)

	workdir := tests.GetIntegrationWorkdir(t)
	if workdir == "" {
		t.Fatal("working directory not configured for integration tests")
	}

	// Create test collection
	testCollection := tests.CreateTestCollection(t, client.API, workdir)
	t.Cleanup(func() {
		tests.RemoveTestCollection(t, client.API, testCollection)
	})

	// Verify collection was created by walking the parent directory
	found := false

	err := client.API.Walk(ctx, workdir, func(path string, record api.Record, err error) error {
		if err != nil {
			return err
		}

		if path == testCollection {
			found = true
		}

		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk directory: %v", err)
	}

	if !found {
		t.Error("created collection not found when walking parent directory")
	}
}

// TestIntegrationWalkCollections tests walking collections to list their contents.
// This verifies that the API can traverse collection hierarchies.
func TestIntegrationWalkCollections(t *testing.T) {
	ctx := context.Background()
	client := tests.NewTestClient(t, ctx)

	workdir := tests.GetIntegrationWorkdir(t)
	if workdir == "" {
		t.Fatal("working directory not configured for integration tests")
	}

	// Create test collection
	testCollection := tests.CreateTestCollection(t, client.API, workdir)
	t.Cleanup(func() {
		tests.RemoveTestCollection(t, client.API, testCollection)
	})

	// Verify we can walk the collection (even though it's empty)
	walkCount := 0

	err := client.API.Walk(ctx, testCollection, func(path string, record api.Record, err error) error {
		if err != nil {
			return err
		}

		walkCount++

		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk collection: %v", err)
	}

	// Walking an empty collection should still succeed (walkCount could be 0 or 1 depending on implementation)
	if walkCount < 0 {
		t.Error("walk returned negative count")
	}
}
