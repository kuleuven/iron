package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
