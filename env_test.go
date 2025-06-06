package iron

import (
	"os"
	"testing"
)

func TestEnvLoadFromFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}

	defer os.Remove(f.Name())

	if err = os.WriteFile(f.Name(), []byte(`{"irods_zone_name": "testZone"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	env := Env{}

	if err = env.LoadFromFile(f.Name()); err != nil {
		t.Fatal(err)
	}

	if env.Zone != "testZone" {
		t.Fatalf("expected testZone, got %s", env.Zone)
	}
}
