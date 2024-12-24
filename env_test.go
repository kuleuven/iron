package iron

import (
	"os"
	"testing"
)

func TestEnvLoadFromFile(t *testing.T) {
	f, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}

	defer os.Remove(f.Name())

	if _, err = f.Write([]byte(`{"irods_zone_name": "testZone"}`)); err != nil {
		t.Fatal(err)
	}

	if err = f.Close(); err != nil {
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
