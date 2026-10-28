package transfer

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kuleuven/iron/api"
	"github.com/kuleuven/iron/msg"
)

func TestDrainKeepsWorkerAndProgressOpen(t *testing.T) {
	var out bytes.Buffer

	worker := New(nil, nil, Options{Output: &out})
	started := make(chan struct{})
	release := make(chan struct{})
	worker.wg.Go(func() error {
		close(started)
		<-release

		return nil
	})
	<-started

	drained := make(chan error, 1)

	go func() { drained <- worker.Drain() }()

	select {
	case <-drained:
		t.Fatal("Drain returned before outstanding work finished")
	default:
	}

	close(release)

	if err := <-drained; err != nil {
		t.Fatal(err)
	}

	worker.Action(t.Context(), Task{Action: TransferFile, Target: testRemoteTxt}, func() error { return nil })

	if err := worker.Drain(); err != nil {
		t.Fatal(err)
	}

	if err := worker.Wait(); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), testRemoteTxt) {
		t.Fatalf("shared progress lost action: %q", out.String())
	}
}

type callbackContextKey struct{}

func TestSubmissionErrorReportingIsAsync(t *testing.T) {
	ctx := context.WithValue(t.Context(), callbackContextKey{}, testStr)
	release := make(chan struct{})
	received := make(chan context.Context, 1)
	submitted := make(chan struct{})
	source := filepath.Join(t.TempDir(), "missing")
	worker := New(nil, nil, Options{
		ErrorHandler: func(callbackCtx context.Context, _, _ string, err error) error {
			received <- callbackCtx

			<-release

			return err
		},
	})

	t.Cleanup(func() {
		close(release)

		<-submitted

		if err := worker.Wait(); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("lost submission error: %v", err)
		}
	})

	go func() {
		worker.Upload(ctx, source, testCopyDest, Flags{})
		close(submitted)
	}()

	select {
	case <-submitted:
	case <-time.After(5 * time.Second):
		t.Fatal("submission blocked on the error handler")
	}

	select {
	case got := <-received:
		if got != ctx {
			t.Fatalf("lost callback context: %v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("error handler was not called")
	}
}

func TestErrorCallbackContext(t *testing.T) {
	sourceErr := errors.New("source failure")
	conn := &api.MockConn{}
	conn.AddResponse(sourceErr)

	ctx := context.WithValue(t.Context(), callbackContextKey{}, testStr)
	received := make(chan context.Context, 1)
	worker := New(nil, mockTransferAPI(conn), Options{
		ErrorHandler: func(ctx context.Context, _, _ string, err error) error {
			received <- ctx

			return err
		},
	})
	worker.Copy(ctx, testCopySrc, testCopyDest, Flags{})

	err := worker.Wait()
	if !errors.Is(err, sourceErr) {
		t.Fatalf("lost failure: %v", err)
	}

	if got := <-received; got != ctx {
		t.Fatalf("lost callback context: %v", got)
	}
}

func TestActionCancellationAndDryRun(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		worker := New(nil, nil, Options{DryRun: dryRun, Output: &bytes.Buffer{}})
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		called := false
		worker.Action(ctx, Task{}, func() error { called = true; return nil })

		err := worker.Wait()
		if called || err == nil {
			t.Fatalf("cancelled action executed or succeeded: called=%v err=%v", called, err)
		}
	}
}

type admissionConn struct {
	*api.MockConn
	release func()
}

func (conn *admissionConn) Close() error {
	conn.release()

	return nil
}

func TestWorkerStagesMultipleCommandsWithBackpressure(t *testing.T) {
	capacity := make(chan struct{}, 2)
	pool := &api.API{Connect: func(ctx context.Context) (api.Conn, error) {
		select {
		case capacity <- struct{}{}:
			return &admissionConn{MockConn: &api.MockConn{}, release: func() { <-capacity }}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
	worker := New(nil, pool, Options{})
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	run := func(*api.API) {
		started <- struct{}{}

		<-release
	}

	for range 2 {
		worker.deferredAction(t.Context(), Task{Action: TransferFile}, func(a *api.API) error {
			run(a)

			return nil
		})
	}

	for range 2 {
		<-started
	}

	ctx, cancel := context.WithCancel(t.Context())
	blocked := make(chan error, 1)

	go func() {
		worker.deferredAction(ctx, Task{Action: TransferFile}, func(a *api.API) error {
			run(a)

			return nil
		})

		blocked <- ctx.Err()
	}()

	cancel()

	if err := <-blocked; !errors.Is(err, context.Canceled) {
		t.Fatalf("blocked submission ignored cancellation: %v", err)
	}

	close(release)

	if err := worker.Wait(); !errors.Is(err, context.Canceled) {
		t.Fatalf("lost blocked submission error: %v", err)
	}

	if len(capacity) != 0 {
		t.Fatal("transfer connections leaked")
	}
}

func TestProgressCallbacksCarrySubmissionContext(t *testing.T) {
	var (
		mu       sync.Mutex
		contexts []context.Context
	)

	worker := New(nil, nil, Options{ProgressHandler: func(ctx context.Context, _ Progress) {
		mu.Lock()
		defer mu.Unlock()

		contexts = append(contexts, ctx)
	}})
	firstCtx := context.WithValue(t.Context(), callbackContextKey{}, "first")
	secondCtx := context.WithValue(t.Context(), callbackContextKey{}, "second")
	first := worker.transferProgress(firstCtx, testRemoteTxt, 1)
	second := worker.transferProgress(secondCtx, testRemoteTxt, 1)

	if _, err := first.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}

	if err := second.Close(); err != nil {
		t.Fatal(err)
	}

	if contexts[0] != firstCtx || contexts[1] != secondCtx || contexts[2] != firstCtx || contexts[3] != secondCtx {
		t.Fatalf("callbacks lost submission contexts: %v", contexts)
	}
}

func TestDryDownloadFailurePreservesDestination(t *testing.T) {
	file := filepath.Join(t.TempDir(), testLocalTxt)
	if err := os.WriteFile(file, []byte(testStr), 0o600); err != nil {
		t.Fatal(err)
	}

	failure := errors.New("remote unavailable")
	conn := &api.MockConn{}
	conn.AddResponse(failure)

	worker := New(nil, mockTransferAPI(conn), Options{DryRun: true})
	worker.Download(t.Context(), file, testRemoteTxt, Flags{})

	if err := worker.Wait(); !errors.Is(err, failure) {
		t.Fatalf("lost download error: %v", err)
	}

	data, err := os.ReadFile(file)
	if err != nil || string(data) != testStr {
		t.Fatalf("dry run modified destination: %q, %v", data, err)
	}
}

func TestDeferredCallbacksKeepContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.WithValue(t.Context(), callbackContextKey{}, testStr))
	defer cancel()

	callbackErr := errors.New("deferred failure")
	progressContexts := make(chan context.Context, 2)
	errorContexts := make(chan context.Context, 1)
	release := make(chan struct{})
	worker := New(nil, mockTransferAPI(&api.MockConn{}), Options{
		ProgressHandler: func(ctx context.Context, _ Progress) { progressContexts <- ctx },
		ErrorHandler: func(ctx context.Context, _, _ string, err error) error {
			errorContexts <- ctx

			return err
		},
	})
	worker.deferredAction(ctx, Task{Action: TransferFile, Target: testRemoteTxt}, func(*api.API) error {
		<-release

		return callbackErr
	})
	cancel()
	close(release)

	if err := worker.Wait(); !errors.Is(err, callbackErr) {
		t.Fatal(err)
	}

	for range 2 {
		if got := <-progressContexts; got != ctx || got.Err() != context.Canceled {
			t.Fatalf("progress lost cancelled submission context: %v", got)
		}
	}

	if got := <-errorContexts; got != ctx || got.Value(callbackContextKey{}) != testStr {
		t.Fatalf("error lost submission context: %v", got)
	}
}

func TestRegistrationContext(t *testing.T) {
	ctx := context.WithValue(t.Context(), callbackContextKey{}, testStr)
	tasks := make(chan Task, 1)
	contexts := make(chan context.Context, 1)
	input := registerProgress(ctx, tasks, func(ctx context.Context, _ Progress) { contexts <- ctx })

	input <- Task{Action: TransferFile, Target: testRemoteTxt}

	close(input)

	for range tasks {
	}

	if got := <-contexts; got != ctx {
		t.Fatalf("registration lost submission context: %v", got)
	}
}

func TestChecksumCallbackContext(t *testing.T) {
	ctx := context.WithValue(t.Context(), callbackContextKey{}, testStr)
	conn := &api.MockConn{}
	conn.AddResponse(msg.String{String: testSha})

	var received []context.Context

	_, err := parseOrComputeChecksum(ctx, mockTransferAPI(conn), func(ctx context.Context, _ Progress) {
		received = append(received, ctx)
	}, testRemoteTxt, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(received) != 2 || received[0] != ctx || received[1] != ctx {
		t.Fatalf("checksum callbacks lost context: %v", received)
	}
}

func TestSynchronizeErrorContext(t *testing.T) {
	ctx := context.WithValue(t.Context(), callbackContextKey{}, testStr)
	failure := errors.New("comparison failed")
	received := make(chan context.Context, 1)
	opts := SynchronizeOptions{
		CompareChecksums: true,
		ChecksumVerify: func(context.Context, string, string, os.FileInfo, os.FileInfo) ([]byte, []byte, error) {
			return nil, nil, failure
		},
		ErrorHandler: func(callbackCtx context.Context, _, _ string, err error) error {
			received <- callbackCtx

			return err
		},
	}
	left := &object{source: testLocalTxt, target: testRemoteTxt, info: &fakeFileInfo{}}
	right := &object{source: testRemoteTxt, target: testRemoteTxt, info: &fakeFileInfo{}}
	err := compareAndTransferObject(ctx, left, right, make(chan Task, 1), opts)

	if !errors.Is(err, failure) {
		t.Fatalf("comparison lost error: %v", err)
	}

	if got := <-received; got != ctx {
		t.Fatalf("comparison lost context: %v", got)
	}
}
