package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kuleuven/iron/scramble"
)

func TestReadAuthFile(t *testing.T) { //nolint:gocognit,funlen
	tests := []struct {
		skip           bool
		name           string
		setupFile      func(t *testing.T) (string, func())
		expectedResult string
		expectNotEqual bool
		expectError    bool
		errorContains  string
	}{
		{
			name: "successful read and decode",
			setupFile: func(t *testing.T) (string, func()) {
				tmpFile, err := os.CreateTemp(t.TempDir(), "test_auth_*")
				if err != nil {
					t.Fatal(err)
				}

				// Write encoded content (uid=1000, password="testpass")
				encoded := scramble.EncodeIrodsA("testpass0", os.Getuid(), time.Now())
				if _, err := tmpFile.Write(encoded); err != nil {
					t.Fatal(err)
				}

				tmpFile.Close()

				return tmpFile.Name(), func() { os.Remove(tmpFile.Name()) }
			},
			expectedResult: "testpass0",
			expectError:    false,
		},
		{
			name: "file does not exist",
			setupFile: func(t *testing.T) (string, func()) {
				return "/nonexistent/path/auth_file", func() {}
			},
			expectError:   true,
			errorContains: "no such file or directory",
		},
		{
			skip: os.Getuid() == 0,
			name: "permission denied",
			setupFile: func(t *testing.T) (string, func()) {
				tmpFile, err := os.CreateTemp(t.TempDir(), "test_auth_perm_*")
				if err != nil {
					t.Fatal(err)
				}

				tmpFile.Close()

				// Remove read permissions
				if err := os.Chmod(tmpFile.Name(), 0o000); err != nil {
					t.Fatal(err)
				}

				return tmpFile.Name(), func() {
					os.Chmod(tmpFile.Name(), 0o644) // Restore permissions for cleanup
					os.Remove(tmpFile.Name())
				}
			},
			expectError:   true,
			errorContains: "permission denied",
		},
		{
			name: "decode error",
			setupFile: func(t *testing.T) (string, func()) {
				tmpFile, err := os.CreateTemp(t.TempDir(), "test_auth_decode_error_*")
				if err != nil {
					t.Fatal(err)
				}

				// Write invalid encoded content (wrong uid)
				encoded := scramble.EncodeIrodsA("testpass1", os.Getuid()+3935, time.Now())

				if _, err := tmpFile.Write(encoded); err != nil {
					t.Fatal(err)
				}

				tmpFile.Close()

				return tmpFile.Name(), func() { os.Remove(tmpFile.Name()) }
			},
			expectError:    false,
			expectedResult: "testpass1",
			expectNotEqual: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip("skipping test")
			}

			authFile, cleanup := tt.setupFile(t)
			defer cleanup()

			result, err := ReadAuthFile(authFile)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}

				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error to contain '%s', got: %v", tt.errorContains, err)
				}

				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if (result == tt.expectedResult) == tt.expectNotEqual {
				t.Errorf("expected result to be '%v', got '%v'", tt.expectedResult, result)
			}
		})
	}
}

func TestWriteAuthFile(t *testing.T) { //nolint:gocognit,funlen
	tests := []struct {
		skip          bool
		name          string
		authFile      string
		password      string
		setupDir      func(t *testing.T, tmpDir string)
		expectError   bool
		errorContains string
		validateFile  func(t *testing.T, filePath string)
	}{
		{
			name:        "create new file successfully",
			password:    "newpassword",
			setupDir:    func(t *testing.T, dir string) {},
			expectError: false,
			validateFile: func(t *testing.T, filePath string) {
				// Check file permissions
				fi, err := os.Stat(filePath)
				if err != nil {
					t.Errorf("failed to stat created file: %v", err)
					return
				}

				if fi.Mode().Perm() != 0o600 {
					t.Errorf("expected file permissions 0600, got %o", fi.Mode().Perm())
				}

				// Check file content by reading it back
				result, err := ReadAuthFile(filePath)
				if err != nil {
					t.Errorf("failed to read back written file: %v", err)
					return
				}

				if result != "newpassword" {
					t.Errorf("expected to read back 'newpassword', got '%s'", result)
				}
			},
		},
		{
			name:     "overwrite existing file",
			password: "updatedpassword",
			setupDir: func(t *testing.T, tmpDir string) {
				// Create existing file with different content
				authFile := filepath.Join(tmpDir, ".irodsA")
				if err := os.WriteFile(authFile, []byte("oldcontent"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			expectError: false,
			validateFile: func(t *testing.T, filePath string) {
				// Check that content was overwritten
				result, err := ReadAuthFile(filePath)
				if err != nil {
					t.Errorf("failed to read back written file: %v", err)
					return
				}

				if result != "updatedpassword" {
					t.Errorf("expected to read back 'updatedpassword', got '%s'", result)
				}
			},
		},
		{
			name:        "empty password",
			password:    "",
			setupDir:    func(t *testing.T, tmpDir string) {},
			expectError: false,
			validateFile: func(t *testing.T, filePath string) {
				result, err := ReadAuthFile(filePath)
				if err != nil {
					t.Errorf("failed to read back written file: %v", err)
					return
				}

				if result != "" {
					t.Errorf("expected to read back empty string, got '%s'", result)
				}
			},
		},
		{
			name:        "long password",
			password:    strings.Repeat("a", 1000),
			setupDir:    func(t *testing.T, tmpDir string) {},
			expectError: false,
			validateFile: func(t *testing.T, filePath string) {
				result, err := ReadAuthFile(filePath)
				if err != nil {
					t.Errorf("failed to read back written file: %v", err)
					return
				}

				expected := strings.Repeat("a", 1000)
				if result != expected {
					t.Errorf("expected to read back long password, got length %d instead of %d", len(result), len(expected))
				}
			},
		},
		{
			skip:     os.Getuid() == 0,
			name:     "permission denied on directory",
			password: "testpass",
			setupDir: func(t *testing.T, tmpDir string) {
				// Remove write permissions from directory
				if err := os.Chmod(tmpDir, 0o444); err != nil {
					t.Fatal(err)
				}
			},
			expectError:   true,
			errorContains: "permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip("skipping test")
			}

			dir := t.TempDir()

			tt.setupDir(t, dir)

			authFile := filepath.Join(dir, ".irodsA")
			if tt.authFile != "" {
				authFile = tt.authFile
			}

			err := WriteAuthFile(authFile, tt.password)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}

				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error to contain '%s', got: %v", tt.errorContains, err)
				}

				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)

				return
			}

			if tt.validateFile != nil {
				tt.validateFile(t, authFile)
			}
		})
	}
}

func TestPersistentState(t *testing.T) {
	testFile := filepath.Join(t.TempDir(), ".irodsA.json")

	testMap := map[string]any{
		"test": "value",
	}

	state := &persistentState{
		file: testFile,
	}

	if err := state.Save(testMap); err != nil {
		t.Errorf("failed to save state: %v", err)
	}

	testMap = map[string]any{}

	if err := state.Load(testMap); err != nil {
		t.Errorf("failed to load state: %v", err)
	}

	if testMap["test"] != "value" {
		t.Errorf("expected testMap['test'] to be 'value', got '%s'", testMap["test"])
	}
}
