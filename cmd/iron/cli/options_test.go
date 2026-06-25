package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kuleuven/iron/scramble"
)

func writeEncodedAuthFile(t *testing.T, pattern, password string, uidOffset int) string {
	t.Helper()

	tmpFile, err := os.CreateTemp(t.TempDir(), pattern)
	if err != nil {
		t.Fatal(err)
	}

	encoded := scramble.EncodeIrodsA(password, os.Getuid()+uidOffset, time.Now())
	if _, err := tmpFile.Write(encoded); err != nil {
		t.Fatal(err)
	}

	tmpFile.Close()

	return tmpFile.Name()
}

func assertErrorContains(t *testing.T, err error, errorContains string) {
	t.Helper()

	if err == nil {
		t.Errorf("expected error but got none")

		return
	}

	if errorContains != "" && !strings.Contains(err.Error(), errorContains) {
		t.Errorf("expected error to contain '%s', got: %v", errorContains, err)
	}
}

func validateReadBack(uid *int, expected string) func(t *testing.T, filePath string) {
	return func(t *testing.T, filePath string) {
		t.Helper()

		result, err := ReadAuthFile(filePath, uid)
		if err != nil {
			t.Errorf("failed to read back written file: %v", err)

			return
		}

		if result != expected {
			t.Errorf("expected to read back %q, got %q", expected, result)
		}
	}
}

func validatePermAndReadBack(uid *int, expected string) func(t *testing.T, filePath string) {
	return func(t *testing.T, filePath string) {
		t.Helper()

		fi, err := os.Stat(filePath)
		if err != nil {
			t.Errorf("failed to stat created file: %v", err)

			return
		}

		if fi.Mode().Perm() != 0o600 {
			t.Errorf("expected file permissions 0600, got %o", fi.Mode().Perm())
		}

		validateReadBack(uid, expected)(t, filePath)
	}
}

type readAuthFileCase struct {
	skip           bool
	name           string
	setupFile      func(t *testing.T) (string, func())
	expectedResult string
	expectNotEqual bool
	expectError    bool
	errorContains  string
}

func (tt readAuthFileCase) run(t *testing.T) {
	if tt.skip {
		t.Skip("skipping test")
	}

	authFile, cleanup := tt.setupFile(t)
	defer cleanup()

	result, err := ReadAuthFile(authFile, nil)

	if tt.expectError {
		assertErrorContains(t, err, tt.errorContains)

		return
	}

	if err != nil {
		t.Errorf("unexpected error: %v", err)

		return
	}

	if (result == tt.expectedResult) == tt.expectNotEqual {
		t.Errorf("expected result to be '%v', got '%v'", tt.expectedResult, result)
	}
}

func TestReadAuthFile(t *testing.T) {
	tests := []readAuthFileCase{
		{
			name: "successful read and decode",
			setupFile: func(t *testing.T) (string, func()) {
				return writeEncodedAuthFile(t, "test_auth_*", "testpass0", 0), func() {}
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
			name: testPermissionDeny,
			setupFile: func(t *testing.T) (string, func()) {
				name := writeEncodedAuthFile(t, "test_auth_perm_*", "", 0)

				// Remove read permissions
				if err := os.Chmod(name, 0o000); err != nil {
					t.Fatal(err)
				}

				return name, func() { os.Chmod(name, 0o644) }
			},
			expectError:   true,
			errorContains: testPermissionDeny,
		},
		{
			name: "decode error",
			setupFile: func(t *testing.T) (string, func()) {
				// Invalid encoded content (wrong uid)
				return writeEncodedAuthFile(t, "test_auth_decode_error_*", "testpass1", 3935), func() {}
			},
			expectError:    false,
			expectedResult: "testpass1",
			expectNotEqual: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

type writeAuthFileCase struct {
	skip          bool
	name          string
	authFile      string
	uid           *int
	password      string
	setupDir      func(t *testing.T, tmpDir string)
	expectError   bool
	errorContains string
	validateFile  func(t *testing.T, filePath string)
}

func (tt writeAuthFileCase) run(t *testing.T) {
	if tt.skip {
		t.Skip("skipping test")
	}

	dir := t.TempDir()

	tt.setupDir(t, dir)

	authFile := filepath.Join(dir, ".irodsA")
	if tt.authFile != "" {
		authFile = tt.authFile
	}

	err := WriteAuthFile(authFile, tt.password, tt.uid)

	if tt.expectError {
		assertErrorContains(t, err, tt.errorContains)

		return
	}

	if err != nil {
		t.Errorf("unexpected error: %v", err)

		return
	}

	if tt.validateFile != nil {
		tt.validateFile(t, authFile)
	}
}

func TestWriteAuthFile(t *testing.T) {
	uid := 9999

	tests := []writeAuthFileCase{
		{
			name:         "create new file successfully",
			password:     testNewPassword,
			setupDir:     func(t *testing.T, dir string) {},
			validateFile: validatePermAndReadBack(nil, testNewPassword),
		},
		{
			name:         "create new file successfully",
			password:     testNewPassword,
			setupDir:     func(t *testing.T, dir string) {},
			uid:          &uid,
			validateFile: validatePermAndReadBack(&uid, testNewPassword),
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
			expectError:  false,
			validateFile: validateReadBack(nil, "updatedpassword"),
		},
		{
			name:         "empty password",
			password:     "",
			setupDir:     func(t *testing.T, tmpDir string) {},
			validateFile: validateReadBack(nil, ""),
		},
		{
			name:         "long password",
			password:     strings.Repeat("a", 1000),
			setupDir:     func(t *testing.T, tmpDir string) {},
			validateFile: validateReadBack(nil, strings.Repeat("a", 1000)),
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
			errorContains: testPermissionDeny,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func TestPersistentState(t *testing.T) {
	testFile := filepath.Join(t.TempDir(), ".irodsA.json")

	testMap := map[string]any{
		testStr: "value",
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

	if testMap[testStr] != "value" {
		t.Errorf("expected testMap['test'] to be 'value', got '%s'", testMap[testStr])
	}
}

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
