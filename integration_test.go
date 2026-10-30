package iron_test

import (
	"context"
	"testing"

	"github.com/kuleuven/iron"
	"github.com/kuleuven/iron/tests"
	"github.com/kuleuven/iron/transfer"
)

// TestIntegrationClientConnection tests establishing a connection to an iRODS server.
// This is a basic sanity check to verify that the Client can connect.
func TestIntegrationClientConnection(t *testing.T) {
	tests.WithClients(t, func(t *testing.T, client *iron.Client, workdir string) {
		conn, err := client.Connect(context.Background())
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}

		defer func() {
			if err := conn.Close(); err != nil {
				t.Logf("warning: failed to close connection: %v", err)
			}
		}()
	})
}

// TestIntegrationClientWorker tests the RunWorker method with a basic callback.
// This verifies that the worker can be created and used with a connection pool.
func TestIntegrationClientWorker(t *testing.T) {
	tests.WithClients(t, func(t *testing.T, client *iron.Client, workdir string) {
		workerRan := false

		err := client.RunWorker(transfer.Options{}, func(worker *transfer.Worker) {
			workerRan = true
		})
		if err != nil {
			t.Fatalf("failed to run worker: %v", err)
		}

		if !workerRan {
			t.Error("worker callback was not executed")
		}
	})
}
