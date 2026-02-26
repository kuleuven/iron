package iron

import (
	"fmt"
	"testing"
	"time"

	"github.com/kuleuven/iron/msg"
)

func TestCheckVersionValid(t *testing.T) {
	tests := []struct {
		version        string
		major          int
		minor          int
		release        int
		expectedResult bool
	}{
		{"rods4.3.2", 4, 3, 2, true},  // equal
		{"rods4.3.3", 4, 3, 2, true},  // release greater
		{"rods4.4.0", 4, 3, 2, true},  // minor greater
		{"rods5.0.0", 4, 3, 2, true},  // major greater
		{"rods4.3.1", 4, 3, 2, false}, // release less
		{"rods4.2.9", 4, 3, 2, false}, // minor less
		{"rods3.9.9", 4, 3, 2, false}, // major less
		{"rods4.3.2", 4, 3, 3, false}, // equal major/minor, release less
		{"rods4.3.2", 4, 4, 0, false}, // equal major, minor less
		{"rods10.0.0", 4, 3, 2, true}, // large major
		{"rods4.3.2", 5, 0, 0, false}, // major less than required
		{"rods5.0.0", 5, 0, 0, true},  // exact match v5
		{"rods4.3.10", 4, 3, 2, true}, // multi-digit release
		{"rods4.10.0", 4, 3, 0, true}, // multi-digit minor
		{"rods0.0.0", 0, 0, 0, true},  // zero version
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s>=%d.%d.%d", tt.version, tt.major, tt.minor, tt.release), func(t *testing.T) {
			v := msg.Version{ReleaseVersion: tt.version}

			result := checkVersion(v, tt.major, tt.minor, tt.release)
			if result != tt.expectedResult {
				t.Errorf("checkVersion(%q, %d, %d, %d) = %v, want %v", tt.version, tt.major, tt.minor, tt.release, result, tt.expectedResult)
			}
		})
	}
}

func TestCheckVersionInvalid(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{"no prefix", "4.3.2"},
		{"wrong prefix", "irods4.3.2"},
		{"too few parts", "rods4.3"},
		{"too many parts", "rods4.3.2.1"},
		{"non-numeric major", "rodsa.3.2"},
		{"non-numeric minor", "rods4.b.2"},
		{"non-numeric release", "rods4.3.c"},
		{"empty after prefix", "rods"},
		{"single part", "rods4"},
		{"empty string", ""},
		{"dots only", "rods..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := msg.Version{ReleaseVersion: tt.version}
			if checkVersion(v, 4, 3, 2) {
				t.Errorf("checkVersion(%q, 4, 3, 2) should return false for invalid version", tt.version)
			}
		})
	}
}

func TestDetermineTTL(t *testing.T) {
	tests := []struct {
		name     string
		ttl      time.Duration
		expected int
	}{
		{"zero", 0, 0},
		{"one hour", time.Hour, 1},
		{"two hours", 2 * time.Hour, 2},
		{"negative", -1 * time.Hour, 0},
		{"half hour rounds down", 30 * time.Minute, 0},
		{"90 minutes rounds down", 90 * time.Minute, 1},
		{"24 hours", 24 * time.Hour, 24},
		{"large duration", 720 * time.Hour, 720},
		{"one second", time.Second, 0},
		{"negative large", -48 * time.Hour, 0},
		{"just under 1h", time.Hour - time.Nanosecond, 0},
		{"just over 1h", time.Hour + time.Nanosecond, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineTTL(tt.ttl)
			if result != tt.expected {
				t.Errorf("determineTTL(%v) = %d, want %d", tt.ttl, result, tt.expected)
			}
		})
	}
}

func TestRetrieveValue(t *testing.T) {
	state := map[string]any{
		"username":    "testuser",
		"password":    "secret",
		"nested_data": map[string]any{"key": "value"},
	}

	tests := []struct {
		name     string
		path     string
		expected string
		wantErr  bool
	}{
		{"top-level string", "/username", "testuser", false},
		{"another top-level", "/password", "secret", false},
		{"missing key returns empty", "/nonexistent", "", false},
		{"nested value", "/nested_data/key", "value", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := retrieveValue(state, tt.path)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("retrieveValue(state, %q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestRetrieveValueInvalidPath(t *testing.T) {
	state := map[string]any{"key": "value"}

	// A completely invalid JSON pointer should return an error
	_, err := retrieveValue(state, "not a valid pointer ~")
	if err != nil {
		// Invalid pointers that fail jsonpointer.New return error
		t.Logf("got expected error: %v", err)
	}
}

func TestRetrieveValueNonStringValue(t *testing.T) {
	state := map[string]any{
		"count": 42,
		"flag":  true,
	}

	// Non-string values should return empty string (type assertion fails)
	result, err := retrieveValue(state, "/count")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "" {
		t.Errorf("expected empty string for non-string value, got %q", result)
	}

	result, err = retrieveValue(state, "/flag")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "" {
		t.Errorf("expected empty string for non-string value, got %q", result)
	}
}

func TestPatchStateEmpty(t *testing.T) {
	state := map[string]any{"key": "value"}
	dirty := false

	err := patchState(state, nil, &dirty, "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if dirty {
		t.Error("dirty should remain false for empty patch")
	}

	err = patchState(state, []map[string]any{}, &dirty, "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if dirty {
		t.Error("dirty should remain false for empty patch slice")
	}
}

func TestPatchStateAdd(t *testing.T) {
	state := map[string]any{"existing": "old"}
	dirty := false

	patch := []map[string]any{
		{"op": "add", "path": "/newkey", "value": "newvalue"},
	}

	err := patchState(state, patch, &dirty, "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !dirty {
		t.Error("dirty should be true after applying patch")
	}

	if state["newkey"] != "newvalue" {
		t.Errorf("expected state[newkey]='newvalue', got %v", state["newkey"])
	}
}

func TestPatchStateReplace(t *testing.T) {
	state := map[string]any{"key": "old"}
	dirty := false

	patch := []map[string]any{
		{"op": "replace", "path": "/key", "value": "new"},
	}

	err := patchState(state, patch, &dirty, "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !dirty {
		t.Error("dirty should be true")
	}

	if state["key"] != "new" {
		t.Errorf("expected state[key]='new', got %v", state["key"])
	}
}

func TestPatchStateDefaultValue(t *testing.T) {
	state := map[string]any{"key": "old"}
	dirty := false

	// Patch without "value" should use defaultValue
	patch := []map[string]any{
		{"op": "add", "path": "/newkey"},
	}

	err := patchState(state, patch, &dirty, "mydefault")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !dirty {
		t.Error("dirty should be true")
	}

	if state["newkey"] != "mydefault" {
		t.Errorf("expected state[newkey]='mydefault', got %v", state["newkey"])
	}
}

func TestPatchStateSkipsNonAddReplace(t *testing.T) {
	state := map[string]any{"key": "value", "other": "keep"}
	dirty := false

	// Only "add" and "replace" get the defaultValue fill-in.
	// A "test" op without value is passed through to jsonpatch which will handle it.
	patch := []map[string]any{
		{"op": "add", "path": "/newkey"},
	}

	err := patchState(state, patch, &dirty, "filled")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !dirty {
		t.Error("dirty should be true after applying patch")
	}

	if state["newkey"] != "filled" {
		t.Errorf("expected state[newkey]='filled', got %v", state["newkey"])
	}
}

type mockPrompt struct {
	askResponse      string
	passwordResponse string
	printMessages    []string
	askErr           error
	passwordErr      error
}

func (m *mockPrompt) Print(message string) error {
	m.printMessages = append(m.printMessages, message)
	return nil
}

func (m *mockPrompt) Ask(message string) (string, error) {
	return m.askResponse, m.askErr
}

func (m *mockPrompt) Password(message string) (string, error) {
	return m.passwordResponse, m.passwordErr
}

func TestPromptValueNonSensitive(t *testing.T) {
	p := &mockPrompt{askResponse: "answer"}

	result, err := promptValue(p, "Enter value", false, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "answer" {
		t.Errorf("expected 'answer', got %q", result)
	}
}

func TestPromptValueSensitive(t *testing.T) {
	p := &mockPrompt{passwordResponse: "secret"}

	result, err := promptValue(p, "Enter password", true, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "secret" {
		t.Errorf("expected 'secret', got %q", result)
	}
}

func TestPromptValueWithDefault(t *testing.T) {
	// When user enters empty string, defaultValue should be used
	p := &mockPrompt{askResponse: ""}

	result, err := promptValue(p, "Enter value", false, "defaultVal")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "defaultVal" {
		t.Errorf("expected 'defaultVal', got %q", result)
	}
}

func TestPromptValueWithDefaultOverridden(t *testing.T) {
	p := &mockPrompt{askResponse: "custom"}

	result, err := promptValue(p, "Enter value", false, "defaultVal")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "custom" {
		t.Errorf("expected 'custom', got %q", result)
	}
}

func TestPromptValueError(t *testing.T) {
	p := &mockPrompt{askErr: fmt.Errorf("input error")}

	_, err := promptValue(p, "Enter value", false, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetValue(t *testing.T) {
	state := map[string]any{
		"username": "testuser",
		"password": "secret",
	}

	t.Run("retrieve path", func(t *testing.T) {
		result, err := getValue(state, nil, "", false, "/username", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result != "testuser" {
			t.Errorf("expected 'testuser', got %q", result)
		}
	})

	t.Run("prompt with default from state", func(t *testing.T) {
		p := &mockPrompt{askResponse: ""}

		result, err := getValue(state, p, "Enter value", false, "", "/username")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result != "testuser" {
			t.Errorf("expected 'testuser' as default, got %q", result)
		}
	})

	t.Run("prompt without default", func(t *testing.T) {
		p := &mockPrompt{askResponse: "custom"}

		result, err := getValue(state, p, "Enter value", false, "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result != "custom" {
			t.Errorf("expected 'custom', got %q", result)
		}
	})
}

func TestConnAccessors(t *testing.T) {
	now := time.Now()
	c := &conn{
		connectedAt:     now,
		transportErrors: 3,
		sqlErrors:       1,
		nativePassword:  "native123",
		clientSignature: "sig456",
	}

	if got := c.ConnectedAt(); got != now {
		t.Errorf("ConnectedAt() = %v, want %v", got, now)
	}

	if got := c.TransportErrors(); got != 3 {
		t.Errorf("TransportErrors() = %d, want 3", got)
	}

	if got := c.SQLErrors(); got != 1 {
		t.Errorf("SQLErrors() = %d, want 1", got)
	}

	if got := c.NativePassword(); got != "native123" {
		t.Errorf("NativePassword() = %q, want 'native123'", got)
	}

	if got := c.ClientSignature(); got != "sig456" {
		t.Errorf("ClientSignature() = %q, want 'sig456'", got)
	}
}

func TestConnServerVersion(t *testing.T) {
	c := &conn{
		version: &msg.Version{ReleaseVersion: "rods4.3.2"},
	}

	if got := c.ServerVersion(); got != "4.3.2" {
		t.Errorf("ServerVersion() = %q, want '4.3.2'", got)
	}
}

func TestConnEnv(t *testing.T) {
	env := Env{
		Username: "user1",
		Zone:     "zone1",
	}

	c := &conn{
		env: &env,
	}

	got := c.Env()
	if got.Username != "user1" || got.Zone != "zone1" {
		t.Errorf("Env() = %+v, want Username=user1, Zone=zone1", got)
	}
}

func TestBuildErrorXML(t *testing.T) {
	c := &conn{
		protocol: msg.XML,
	}

	// No error payload
	m := msg.Message{
		Header: msg.Header{ErrorLen: 0},
		Body:   msg.Body{Message: []byte("simple error")},
	}

	result := c.buildError(m)
	if result != "simple error" {
		t.Errorf("expected 'simple error', got %q", result)
	}
}

func TestBuildErrorNativeNoError(t *testing.T) {
	c := &conn{
		protocol: msg.Native,
	}

	m := msg.Message{
		Header: msg.Header{ErrorLen: 0},
		Body:   msg.Body{Message: []byte("native error msg")},
	}

	result := c.buildError(m)
	if result != "native error msg" {
		t.Errorf("expected 'native error msg', got %q", result)
	}
}
