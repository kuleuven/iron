package cli

import (
	"context"
	"testing"
)

func TestNew(t *testing.T) {
	app := New(context.Background())

	cmd := app.Command()

	if cmd == nil {
		t.Fatal("expected command")
	}
}
