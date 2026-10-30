package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreWorkdirInFile(t *testing.T) {
	testFile := filepath.Join(t.TempDir(), "env.json")

	if err := os.WriteFile(testFile, []byte(`{"a":1}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := StoreWorkdirInFile(t.Context(), testFile, "/tmp"); err != nil {
		t.Fatal(err)
	}

	env := map[string]any{}

	payload, err := os.ReadFile(fmt.Sprintf("%s.%d", testFile, os.Getppid()))
	if err != nil {
		t.Fatal(err)
	}

	if err := json.Unmarshal(payload, &env); err != nil {
		t.Fatal(err)
	}

	if env["irods_cwd"] != "/tmp" {
		t.Errorf("expected env['irods_cwd'] to be '/tmp', got '%s'", env["workdir"])
	}

	if env["a"] != float64(1) {
		t.Errorf("expected env['a'] to be '1', got '%s'", env["a"])
	}
}

// Test validateConfigFile directly to avoid GetRuntimeConfigFile path lookup
func TestValidateConfigFile_NotFound_WithConfigurator(t *testing.T) {
	ctx := context.WithValue(context.Background(), appKey, &App{
		configurator: func(ctx context.Context, environment string, args []string) error {
			return nil
		},
	})

	err := validateConfigFile("/nonexistent/path/.irods/irods_environment.json", ctx)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "configuration file not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}

	if !strings.Contains(errMsg, "configurator") {
		t.Errorf("expected guidance about configurator command, got: %v", err)
	}
}

func TestValidateConfigFile_NotFound_WithoutConfigurator(t *testing.T) {
	ctx := context.Background()

	err := validateConfigFile("/nonexistent/path/.irods/irods_environment.json", ctx)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "configuration file not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}

	if !strings.Contains(errMsg, "manually") {
		t.Errorf("expected guidance about setting up manually, got: %v", err)
	}
}

func TestValidateConfigFile_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "irods_environment.json")

	// Write invalid JSON
	if err := os.WriteFile(configFile, []byte("{invalid json}"), 0o600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	err := validateConfigFile(configFile, context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "invalid JSON") {
		t.Errorf("expected 'invalid JSON' error, got: %v", err)
	}
}

func TestValidateConfigFile_MissingRequiredFields_WithConfigurator(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "irods_environment.json")

	// Write valid JSON but missing required fields
	config := map[string]string{
		testIrodsHost: testExampleHost,
		// missing irods_port, irods_user_name, irods_zone_name
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configFile, data, 0o600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ctx := context.WithValue(context.Background(), appKey, &App{
		configurator: func(ctx context.Context, environment string, args []string) error {
			return nil
		},
	})

	err = validateConfigFile(configFile, ctx)
	if err == nil {
		t.Fatal("expected error for missing fields")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "missing required fields") {
		t.Errorf("expected 'missing required fields' error, got: %v", err)
	}

	if !strings.Contains(errMsg, testIrodsPort) {
		t.Errorf("expected error to mention '%s', got: %v", testIrodsPort, err)
	}

	if !strings.Contains(errMsg, "configurator") {
		t.Errorf("expected guidance about configurator, got: %v", err)
	}
}

func TestValidateConfigFile_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "irods_environment.json")

	// Write valid config with all required fields
	config := map[string]any{
		testIrodsHost:     testExampleHost,
		testIrodsPort:     1247,
		testIrodsUserName: testAlice,
		testIrodsZoneName: testProduction,
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configFile, data, 0o600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	err = validateConfigFile(configFile, context.Background())
	if err != nil {
		t.Errorf("expected no error for valid config, got: %v", err)
	}
}

func TestValidateConfigFile_ValidWithExtraFields(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "irods_environment.json")

	// Write valid config with extra optional fields
	config := map[string]any{
		testIrodsHost:        testExampleHost,
		testIrodsPort:        1247,
		testIrodsUserName:    testAlice,
		testIrodsZoneName:    testProduction,
		testIrodsAuthScheme:  testPamInteractive,
		testIrodsDefResource: "default",
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configFile, data, 0o600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ctx := context.WithValue(context.Background(), appKey, &App{
		configurator: func(ctx context.Context, environment string, args []string) error {
			return nil
		},
	})

	err = validateConfigFile(configFile, ctx)
	if err != nil {
		t.Errorf("expected no error for valid config with extra fields, got: %v", err)
	}
}
