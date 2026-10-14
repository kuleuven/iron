package iron

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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

	if env.Zone != testZoneName {
		t.Fatalf("expected testZone, got %s", env.Zone)
	}
}

func TestEnvMarshal(t *testing.T) {
	var (
		zero int
		one  = 1
	)

	envs := []Env{
		{Zone: testZoneName},
		{Zone: testZoneName, AuthenticationUID: &zero},
		{Zone: testZoneName, AuthenticationUID: &one},
	}

	for i, env := range envs {
		payload, err := json.Marshal(env)
		if err != nil {
			t.Fatal(err)
		}

		var unmarshaled Env

		if err = json.Unmarshal(payload, &unmarshaled); err != nil {
			t.Fatal(err)
		}

		assertUID(t, i, payload, unmarshaled, i > 0)
	}
}

func assertUID(t *testing.T, i int, payload []byte, unmarshaled Env, expected bool) {
	t.Helper()

	const field = "irods_authentication_uid"

	if got := bytes.Contains(payload, []byte(field)); got != expected {
		t.Errorf("[%d] expected payload containing %s to be %v, got %s", i, field, expected, payload)
	}

	if got := unmarshaled.AuthenticationUID != nil; got != expected {
		t.Errorf("[%d] expected unmarshaled.AuthenticationUID non-nil to be %v", i, expected)
	}
}

func TestPersistentState(t *testing.T) {
	testFile := filepath.Join(t.TempDir(), ".irodsA.json")

	testMap := map[string]any{
		testStr: "value",
	}

	state := &FilePersistentState{
		AuthenticationFile: testFile,
		StateFile:          testFile + ".json",
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
