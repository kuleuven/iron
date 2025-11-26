package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/creativeprojects/go-selfupdate"
)

func TestUpdate(t *testing.T) {
	app := testApp(t)

	testPath := filepath.Join(t.TempDir(), "iron")

	WithVersion("devel")(app.App)
	WithUpdater(selfupdate.DefaultUpdater(), selfupdate.ParseSlug("kuleuven/iron"))(app.App)

	app.CheckUpdate(t.Context())

	// Put something at testPath
	if err := os.WriteFile(testPath, []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := app.Update(t.Context(), testPath, true); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateCommand(t *testing.T) {
	app := testApp(t)

	// Ensure we're actually not going to update the current version
	WithVersion("9999.0.0")(app.App)
	WithUpdater(selfupdate.DefaultUpdater(), selfupdate.ParseSlug("kuleuven/iron"))(app.App)

	cmd := app.Command()
	cmd.SetArgs([]string{"update"})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}
