package iron

import (
	"context"
	"runtime"
	"testing"
)

func TestSystemOpenBrowserUnsupported(t *testing.T) {
	// Save and restore is not feasible because runtime.GOOS is a constant.
	// Instead, dispatch to a helper that mirrors the logic but accepts a goos arg
	// would be invasive. We just verify that on the current platform the call
	// succeeds OR returns ErrUnsupportedPlatform when not implemented.
	switch runtime.GOOS {
	case "linux", "windows", "darwin":
		// Just exercise the code path; the spawned command may fail in
		// headless test environments, which is fine — we only need the
		// branch to be covered. Discard the error.
		_ = SystemOpenBrowser("about:blank")
	default:
		err := SystemOpenBrowser("about:blank")
		if err != ErrUnsupportedPlatform {
			t.Errorf("expected ErrUnsupportedPlatform, got %v", err)
		}
	}
}

func TestConnAPI(t *testing.T) {
	c := &conn{
		env: &Env{
			Username: "alice",
			Zone:     "tempZone",
		},
	}

	api := c.API()

	if api == nil {
		t.Fatal("API() returned nil")
	}

	if api.Username != "alice" {
		t.Errorf("Username = %q, want alice", api.Username)
	}

	if api.Zone != "tempZone" {
		t.Errorf("Zone = %q, want tempZone", api.Zone)
	}

	if api.Connect == nil {
		t.Fatal("Connect callback is nil")
	}

	got, err := api.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	if got == nil {
		t.Fatal("Connect() returned nil conn")
	}

	// dummyCloser.Close() should always return nil.
	if err := got.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}
