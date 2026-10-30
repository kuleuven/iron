package tests

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kuleuven/iron"
	"github.com/kuleuven/iron/api"
)

// integrationConfigPath returns the path to the integration test configuration file.
// The path is read from the IRON_INTEGRATION_TEST_CONFIG environment variable.
// Returns empty string if the environment variable is not set.
func integrationConfigPath() string {
	return os.Getenv("IRON_INTEGRATION_TEST_CONFIG")
}

// ShouldSkipIntegration returns true if integration tests should be skipped.
// This happens when IRON_INTEGRATION_TEST_CONFIG is not set or points to a non-existent file.
// Use this in integration tests like: if ShouldSkipIntegration(t) { t.Skip() }
func ShouldSkipIntegration(t testing.TB) bool {
	configPath := integrationConfigPath()
	if configPath == "" {
		t.Log("IRON_INTEGRATION_TEST_CONFIG is not set")

		return true
	}

	_, err := os.Stat(configPath)
	if err != nil {
		t.Logf("IRON_INTEGRATION_TEST_CONFIG points to a non-existent file: %v", err)

		return true
	}

	return false
}

// LoadIntegrationEnv loads the iRODS environment configuration from the file
// specified by IRON_INTEGRATION_TEST_CONFIG environment variable.
// Returns nil if the environment variable is not set or the file cannot be read.
// Returns an error if the file is present but invalid.
func LoadIntegrationEnv(t testing.TB) *iron.Env {
	configPath := integrationConfigPath()
	if configPath == "" {
		return nil
	}

	var env iron.Env

	err := env.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("failed to read integration config from %s: %v", configPath, err)
	}

	return &env
}

// generateUniqueCollectionName generates a unique collection name for integration tests.
// The name format is: iron_test_<timestamp>_<random>
func generateUniqueCollectionName() string {
	timestamp := time.Now().Unix()

	cryptoRand := make([]byte, 4)
	_, _ = rand.Read(cryptoRand)

	randomNum := int(binary.BigEndian.Uint32(cryptoRand)) % 100000

	return fmt.Sprintf("iron_test_%d_%05d", timestamp, randomNum)
}

// CreateTestCollection creates a temporary subcollection for integration testing.
// The collection is created under the specified base path using the provided API client.
// Collection names are unique (based on timestamp and random number) to avoid conflicts.
// Returns the full path to the created collection and any error that occurred.
// The caller is responsible for cleaning up the collection (typically using t.Cleanup).
func CreateTestCollection(t testing.TB, apiClient *api.API, basePath string) string {
	if apiClient == nil {
		t.Fatal("api client is nil")
	}

	collectionName := generateUniqueCollectionName()
	fullPath := filepath.Join(basePath, collectionName)

	// Normalize path separators to iRODS style (always forward slashes)
	fullPath = filepath.ToSlash(fullPath)

	ctx := t.Context()
	if err := apiClient.CreateCollectionAll(ctx, fullPath); err != nil {
		t.Fatalf("failed to create test collection %s: %v", fullPath, err)
	}

	t.Logf("created test collection: %s", fullPath)

	return fullPath
}

// RemoveTestCollection removes a test collection created by CreateTestCollection.
// This function logs errors but does not fail the test, making it suitable for use
// with t.Cleanup() to ensure cleanup happens even if the test fails.
// It uses the provided API client to remove the collection.
func RemoveTestCollection(t testing.TB, apiClient *api.API, collectionPath string) {
	if apiClient == nil {
		return
	}

	if collectionPath == "" {
		return
	}

	ctx := context.Background()
	if err := apiClient.DeleteCollectionAll(ctx, collectionPath, true); err != nil {
		t.Logf("warning: failed to remove test collection %s: %v", collectionPath, err)
	}
}

// GetIntegrationWorkdir returns the working directory from the integration configuration.
// This is typically the base path under which test collections should be created.
// Returns empty string if configuration is not available or doesn't contain the expected field.
func GetIntegrationWorkdir(t testing.TB) string {
	config := LoadIntegrationEnv(t)
	if config == nil {
		return ""
	}

	return config.WorkingDirectory
}

// NewTestClient creates a new iron.Client for integration testing.
// Skips the test if integration tests are disabled.
// Returns nil and fails the test if the environment cannot be loaded or client creation fails.
func NewTestClient(t testing.TB, ctx context.Context) *iron.Client {
	if ShouldSkipIntegration(t) {
		t.Skip("integration tests disabled")
	}

	env := LoadIntegrationEnv(t)
	if env == nil {
		t.Fatal("failed to load integration config")
	}

	client, err := iron.New(ctx, *env, iron.Option{
		MaxConns:   4,
		ClientName: "integration-test",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Register cleanup to close the client
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Logf("warning: failed to close client: %v", err)
		}
	})

	return client
}
