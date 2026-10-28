package iron

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kuleuven/iron/transfer"
)

func TestRunWorkerDrainsAndCleansUp(t *testing.T) {
	client := newTestClient(3)
	t.Cleanup(func() { _ = client.Close() })

	callbackErr := errors.New("bad script line")
	finished := false
	err := client.RunWorker(transfer.Options{MaxThreads: 1}, func(worker *transfer.Worker) {
		worker.Action(t.Context(), transfer.Task{Action: transfer.CreateDirectory}, func() error {
			finished = true

			return nil
		})

		worker.Error(t.Context(), "", "", callbackErr)
	})

	if !finished || !errors.Is(err, callbackErr) {
		t.Fatalf("RunWorker returned %v, action finished=%v", err, finished)
	}

	if client.defaultPool.maxConns != 3 {
		t.Fatal("transfer connections were not returned to the parent pool")
	}

	for _, child := range client.defaultPool.children {
		if !child.closed {
			t.Fatal("transfer pool was not closed")
		}
	}
}

func TestRunWorkerPreservesCustomErrorHandler(t *testing.T) {
	client := newTestClient(3)
	t.Cleanup(func() { _ = client.Close() })

	actionErr := errors.New("action failed")

	var handled bool

	err := client.RunWorker(transfer.Options{
		MaxThreads: 1,
		ErrorHandler: func(_ context.Context, _, _ string, err error) error {
			handled = errors.Is(err, actionErr)

			return nil
		},
	}, func(worker *transfer.Worker) {
		worker.Error(t.Context(), "", "", actionErr)
	})

	if err != nil || !handled {
		t.Fatalf("custom error handling changed: handled=%v, error=%v", handled, err)
	}
}

func TestRunWorkerErrorRetainsOperationLabel(t *testing.T) {
	client := newTestClient(3)
	t.Cleanup(func() { _ = client.Close() })

	failure := errors.New("operation failed")

	err := client.RunWorker(transfer.Options{MaxThreads: 1}, func(worker *transfer.Worker) {
		worker.Error(transfer.WithLabel(t.Context(), "operation:2"), "", "", failure)
	})
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), "operation:2") {
		t.Fatalf("lost labeled operation error: %v", err)
	}
}

func TestPoolConnectCancellation(t *testing.T) {
	client := newTestClient(1)
	t.Cleanup(func() { _ = client.Close() })

	conn, err := client.Connect(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)

	go func() {
		_, err := client.Connect(ctx)
		done <- err
	}()

	waitForPoolWaiters(t, client.defaultPool, 1)
	cancel()

	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Connect: %v", err)
	}

	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	conn, err = client.Connect(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
}

func waitForPoolWaiters(t *testing.T, pool *Pool, want int) {
	t.Helper()

	ticker := time.NewTicker(time.Millisecond)

	defer ticker.Stop()

	timeout := time.NewTimer(time.Second)

	defer timeout.Stop()

	for {
		pool.lock.Lock()

		waiting := pool.waiting
		pool.lock.Unlock()

		if waiting == want {
			return
		}

		select {
		case <-ticker.C:
		case <-timeout.C:
			t.Fatalf("waiting connections = %d, want %d", waiting, want)
		}
	}
}

func TestPoolCancelledWaiterPreservesOtherWaiters(t *testing.T) {
	client := newTestClient(1)
	t.Cleanup(func() { _ = client.Close() })

	conn, err := client.Connect(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancelled := make(chan error, 1)

	go func() {
		_, err := client.Connect(ctx)
		cancelled <- err
	}()

	resumed := make(chan error, 1)

	go func() {
		next, err := client.Connect(t.Context())
		if err == nil {
			err = next.Close()
		}

		resumed <- err
	}()

	waitForPoolWaiters(t, client.defaultPool, 2)
	cancel()

	if err := <-cancelled; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled waiter: %v", err)
	}

	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	if err := <-resumed; err != nil {
		t.Fatalf("other waiter: %v", err)
	}
}
