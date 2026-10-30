package tests

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kuleuven/iron"
)

const (
	testEnvVarNotSet   = "env var not set"
	testValidConfig    = "valid"
	testWorkdir        = "/tempZone/home/testuser"
	testConfigFileName = "iron_test_config_*.json"
)

func TestIntegrationConfigPath(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "env var set",
			envValue: "/path/to/config.json",
			expected: "/path/to/config.json",
		},
		{
			name:     testEnvVarNotSet,
			envValue: "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("IRON_INTEGRATION_TEST_CONFIG", tt.envValue)

			result := integrationConfigPath()
			if result != tt.expected {
				t.Errorf("integrationConfigPath() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestShouldSkipIntegration(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		fileOK   bool
		expected bool
	}{
		{
			name:     testEnvVarNotSet,
			envValue: "",
			fileOK:   false,
			expected: true,
		},
		{
			name:     "env var set but file doesn't exist",
			envValue: "/nonexistent/path/config.json",
			fileOK:   false,
			expected: true,
		},
		{
			name:     "env var set and file exists",
			envValue: "",
			fileOK:   true,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.fileOK {
				tmpFile, err := os.CreateTemp(t.TempDir(), testConfigFileName)
				if err != nil {
					t.Fatalf("failed to create temp file: %v", err)
				}

				tmpFile.Close()
				t.Setenv("IRON_INTEGRATION_TEST_CONFIG", tmpFile.Name())
			} else {
				if tt.envValue != "" {
					t.Setenv("IRON_INTEGRATION_TEST_CONFIG", tt.envValue)
				} else {
					t.Setenv("IRON_INTEGRATION_TEST_CONFIG", "")
				}
			}

			result := ShouldSkipIntegration(t)
			if result != tt.expected {
				t.Errorf("ShouldSkipIntegration() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func testLoadIntegrationConfig(t *testing.T, tt struct {
	name        string
	envValue    string
	fileContent string
	check       func(t *testing.T, config *iron.Env)
},
) {
	if tt.envValue != "" {
		tmpFile, err := os.CreateTemp(t.TempDir(), testConfigFileName)
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}

		if _, err := tmpFile.WriteString(tt.fileContent); err != nil {
			t.Fatalf("failed to write temp file: %v", err)
		}

		tmpFile.Close()

		t.Setenv("IRON_INTEGRATION_TEST_CONFIG", tmpFile.Name())
	} else {
		t.Setenv("IRON_INTEGRATION_TEST_CONFIG", "")
	}

	config := LoadIntegrationEnv(t)

	if tt.check != nil {
		tt.check(t, config)
	}
}

func TestLoadIntegrationConfig(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		fileContent string
		check       func(t *testing.T, config *iron.Env)
	}{
		{
			name:     testEnvVarNotSet,
			envValue: "",
			check: func(t *testing.T, config *iron.Env) {
				if config != nil {
					t.Errorf("LoadIntegrationConfig() = %v, want nil", config)
				}
			},
		},
		{
			name:        testValidConfig,
			envValue:    testValidConfig,
			fileContent: fmt.Sprintf(`{"irods_cwd": "%s", "irods_host": "localhost"}`, testWorkdir),
			check: func(t *testing.T, config *iron.Env) {
				if config == nil {
					t.Error("LoadIntegrationConfig() returned nil, want config")
				}

				if config.WorkingDirectory != testWorkdir {
					t.Errorf("irods_cwd mismatch: got %v", config.WorkingDirectory)
				}

				if config.Host != "localhost" {
					t.Errorf("irods_host mismatch: got %v", config.Host)
				}
			},
		},
		{
			name:        "empty JSON object",
			envValue:    "empty",
			fileContent: `{}`,
			check: func(t *testing.T, config *iron.Env) {
				if config == nil {
					t.Error("LoadIntegrationConfig() returned nil, want empty config")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testLoadIntegrationConfig(t, tt)
		})
	}
}

func TestGenerateUniqueCollectionName(t *testing.T) {
	name1 := generateUniqueCollectionName()
	name2 := generateUniqueCollectionName()

	// Check format: iron_test_<timestamp>_<random>
	if !strings.HasPrefix(name1, "iron_test_") {
		t.Errorf("generateUniqueCollectionName() = %q, want prefix 'iron_test_'", name1)
	}

	// Check that names are unique (they should be since we generate them sequentially)
	if name1 == name2 {
		t.Errorf("generateUniqueCollectionName() generated duplicate names: %q", name1)
	}

	// Validate the format more strictly
	parts := strings.Split(name1, "_")
	if len(parts) != 4 {
		t.Errorf("generateUniqueCollectionName() format incorrect: got %d parts, want 4", len(parts))
	}

	if parts[0] != "iron" || parts[1] != "test" {
		t.Errorf("generateUniqueCollectionName() prefix incorrect: got %s_%s, want iron_test", parts[0], parts[1])
	}
}

func TestGenerateUniqueCollectionNameConcurrency(t *testing.T) {
	const numGoroutines = 100

	names := make([]string, numGoroutines)

	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)

	for i := range numGoroutines {
		wg.Add(1)

		go func(idx int) {
			defer wg.Done()

			name := generateUniqueCollectionName()

			mu.Lock()

			names[idx] = name
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	// Verify all names follow the correct format and have the prefix
	for _, name := range names {
		if !strings.HasPrefix(name, "iron_test_") {
			t.Errorf("Name doesn't have correct prefix: %q", name)
		}

		parts := strings.Split(name, "_")
		if len(parts) != 4 {
			t.Errorf("Name has wrong format: %q", name)
		}
	}
}

func TestCreateTestCollection(t *testing.T) {
	// Test that the function generates valid collection names with expected format
	collectionName := generateUniqueCollectionName()

	// Verify the function would create a valid path
	if !strings.HasPrefix(collectionName, "iron_test_") {
		t.Errorf("generateUniqueCollectionName() produced invalid name: %q", collectionName)
	}

	// Verify the name has proper format with underscores
	parts := strings.Split(collectionName, "_")
	if len(parts) != 4 {
		t.Errorf("collection name format invalid: %q, expected 4 parts", collectionName)
	}
}

func TestCreateTestCollectionPathConstruction(t *testing.T) {
	// Test that path construction works correctly by testing the path building logic
	basePath := "/zone/home/test"
	collectionName := generateUniqueCollectionName()
	fullPath := filepath.Join(basePath, collectionName)

	// Normalize path separators to iRODS style (always forward slashes)
	fullPath = filepath.ToSlash(fullPath)

	// Verify path construction
	if !strings.HasPrefix(fullPath, basePath) {
		t.Errorf("path doesn't start with base path: %q", fullPath)
	}

	if strings.Contains(fullPath, "\\") {
		t.Errorf("path contains backslashes: %q", fullPath)
	}
}

func TestRemoveTestCollection(t *testing.T) {
	// Test that RemoveTestCollection handles nil client gracefully
	collectionPath := "/zone/home/test/iron_test_123_45678"
	RemoveTestCollection(t, nil, collectionPath)

	// Should complete without error
}

func TestGetIntegrationWorkdir(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		fileContent string
		expected    string
	}{
		{
			name:     testEnvVarNotSet,
			envValue: "",
			expected: "",
		},
		{
			name:        testValidConfig,
			envValue:    testValidConfig,
			fileContent: fmt.Sprintf(`{"irods_cwd": "%s"}`, testWorkdir),
			expected:    testWorkdir,
		},
		{
			name:        "config without irods_cwd",
			envValue:    "empty",
			fileContent: `{"irods_host": "localhost"}`,
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				tmpFile, err := os.CreateTemp(t.TempDir(), testConfigFileName)
				if err != nil {
					t.Fatalf("failed to create temp file: %v", err)
				}

				if _, err := tmpFile.WriteString(tt.fileContent); err != nil {
					t.Fatalf("failed to write temp file: %v", err)
				}

				tmpFile.Close()

				t.Setenv("IRON_INTEGRATION_TEST_CONFIG", tmpFile.Name())
			} else {
				t.Setenv("IRON_INTEGRATION_TEST_CONFIG", "")
			}

			result := GetIntegrationWorkdir(t)

			if result != tt.expected {
				t.Errorf("GetIntegrationWorkdir() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func testLoadIntegrationEnv(t *testing.T, tt struct {
	name        string
	envValue    string
	fileContent string
	shouldBeNil bool
},
) {
	if tt.envValue != "" {
		tmpFile, err := os.CreateTemp(t.TempDir(), testConfigFileName)
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}

		if _, err := tmpFile.WriteString(tt.fileContent); err != nil {
			t.Fatalf("failed to write temp file: %v", err)
		}

		tmpFile.Close()

		t.Setenv("IRON_INTEGRATION_TEST_CONFIG", tmpFile.Name())
	} else {
		t.Setenv("IRON_INTEGRATION_TEST_CONFIG", "")
	}

	env := LoadIntegrationEnv(t)

	if tt.shouldBeNil && env != nil {
		t.Errorf("LoadIntegrationEnv() returned non-nil when env var not set")
	}

	if !tt.shouldBeNil && env == nil {
		t.Error("LoadIntegrationEnv() returned nil, want env")
	}
}

func TestLoadIntegrationEnv(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		fileContent string
		shouldBeNil bool
	}{
		{
			name:        testEnvVarNotSet,
			envValue:    "",
			shouldBeNil: true,
		},
		{
			name:        testValidConfig,
			envValue:    testValidConfig,
			fileContent: `{"irods_host": "localhost", "irods_port": 1247}`,
			shouldBeNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testLoadIntegrationEnv(t, tt)
		})
	}
}

func TestNewTestClient(t *testing.T) {
	// Test skipping when integration tests are disabled
	t.Setenv("IRON_INTEGRATION_TEST_CONFIG", "")

	// Verify that ShouldSkipIntegration returns true when env var is not set
	if !ShouldSkipIntegration(t) {
		t.Error("ShouldSkipIntegration() should return true when env var not set")
	}
}

// TestIntegrationHelpersCohesion tests that helper functions work together correctly
func TestIntegrationHelpersCohesion(t *testing.T) {
	// Create a temporary config file
	configData := map[string]any{
		"irods_cwd":  testWorkdir,
		"irods_host": "localhost",
	}

	jsonData, err := json.Marshal(configData)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	tmpFile, err := os.CreateTemp(t.TempDir(), testConfigFileName)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	if _, err := tmpFile.Write(jsonData); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	tmpFile.Close()

	t.Setenv("IRON_INTEGRATION_TEST_CONFIG", tmpFile.Name())

	// Test the cohesion of multiple functions
	// ShouldSkipIntegration should return false with valid config
	if ShouldSkipIntegration(t) {
		t.Error("ShouldSkipIntegration() should return false with valid config")
	}

	// LoadIntegrationEnv should return the config
	config := LoadIntegrationEnv(t)
	if config == nil {
		t.Error("LoadIntegrationEnv() returned nil")
	}

	// GetIntegrationWorkdir should return the workdir from the config
	workdir := GetIntegrationWorkdir(t)
	if workdir != testWorkdir {
		t.Errorf("GetIntegrationWorkdir() = %q, want %q", workdir, testWorkdir)
	}
}

func TestGenerateUniqueCollectionNameFormat(t *testing.T) {
	name := generateUniqueCollectionName()

	// Verify exact format using timestamp check
	now := time.Now().Unix()

	if !strings.HasPrefix(name, "iron_test_") {
		t.Errorf("name should start with 'iron_test_': %q", name)
	}

	// Extract timestamp from name
	parts := strings.Split(name, "_")
	if len(parts) != 4 {
		t.Fatalf("name should have 4 parts separated by underscores: %q has %d", name, len(parts))
	}

	// Verify timestamp is recent (within last 5 seconds)
	var timestamp int64

	if _, err := fmt.Sscanf(parts[2], "%d", &timestamp); err != nil {
		t.Fatalf("failed to parse timestamp from name %q: %v", name, err)
	}

	if timestamp < now-5 || timestamp > now+5 {
		t.Errorf("timestamp in name is not recent: %d vs current %d", timestamp, now)
	}
}
