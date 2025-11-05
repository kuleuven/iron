package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUpdate(t *testing.T) {
	app := testApp(t)

	testPath := filepath.Join(t.TempDir(), "iron")

	WithVersion("devel")(app.App)

	app.CheckUpdate()

	// Put something at testPath
	if err := os.WriteFile(testPath, []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := app.Update(testPath, true); err != nil {
		t.Fatal(err)
	}
}
