package iron

import (
	"bytes"
	"encoding/json"
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

func TestEnvMarshal(t *testing.T) {
	var (
		zero int
		one  = 1
	)

	envs := []Env{
		{Zone: "testZone"},
		{Zone: "testZone", IrodsAuthenticationUID: &zero},
		{Zone: "testZone", IrodsAuthenticationUID: &one},
	}

	for i, env := range envs {
		payload, err := json.Marshal(env)
		if err != nil {
			t.Fatal(err)
		}

		expected := i > 0

		if !expected && bytes.Contains(payload, []byte("irods_authentication_uid")) {
			t.Errorf("[%d] expected payload to not contain irods_authentication_uid, got %s", i, payload)
		}

		if expected && !bytes.Contains(payload, []byte("irods_authentication_uid")) {
			t.Errorf("[%d] expected payload to contain irods_authentication_uid, got %s", i, payload)
		}

		var unmarshaled Env

		if err = json.Unmarshal(payload, &unmarshaled); err != nil {
			t.Fatal(err)
		}

		if expected && unmarshaled.IrodsAuthenticationUID == nil {
			t.Errorf("[%d] expected unmarshaled.IrodsAuthenticationUID to not be nil", i)
		}

		if !expected && unmarshaled.IrodsAuthenticationUID != nil {
			t.Errorf("[%d] expected unmarshaled.IrodsAuthenticationUID to be nil", i)
		}
	}
}
