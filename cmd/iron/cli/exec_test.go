package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kuleuven/iron"
	"github.com/kuleuven/iron/msg"
	"github.com/kuleuven/iron/transfer"
)

func scriptTestApp(t *testing.T) *mockApp {
	t.Helper()

	app := testApp(t)
	t.Cleanup(func() { _ = app.Close() })

	if err := app.Client.Close(); err != nil {
		t.Fatal(err)
	}

	var err error

	app.Client, err = iron.New(t.Context(), iron.Env{Zone: testTestZoneName}, iron.Option{
		MaxConns: 8,
		HandshakeFunc: func(context.Context) (iron.Conn, error) {
			return app.mockConn, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	return app
}

func TestRunScriptBarriersAndFlagIsolation(t *testing.T) {
	app := scriptTestApp(t)
	for range 3 {
		app.AddResponse(msg.EmptyResponse{})
	}

	input := "# comment\r\nmkdir -p '/first path'\r\nwait\r\nmkdir /second\nwait\nmkdir /third"

	var output bytes.Buffer

	if err := app.execFile(t.Context(), transfer.Options{MaxThreads: 1, Output: &output}, "", "commands.iron", strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}

	if len(app.Dialog) != 0 {
		t.Fatalf("unconsumed operations: %d", len(app.Dialog))
	}

	for _, path := range []string{"/first path", "/second", "/third"} {
		if !strings.Contains(output.String(), path) {
			t.Fatalf("missing action %s in %q", path, output.String())
		}
	}
}

func TestRunScriptErrors(t *testing.T) {
	for _, line := range []string{
		"wait extra", "cp --exclusive /one /two", "upload --threads 2 /one /two",
		"auth", "run nested.iron", "mkdir '/unterminated", "mkdir /one; mkdir /two",
		"mkdir /one | cat", "cd /one",
	} {
		t.Run(line, func(t *testing.T) {
			app := scriptTestApp(t)
			err := app.execFile(t.Context(), transfer.Options{MaxThreads: 1}, "", "bad.iron", strings.NewReader("# header\n"+line))

			if err == nil || !strings.Contains(err.Error(), "bad.iron:2") {
				t.Fatalf("expected attributed error, got %v", err)
			}
		})
	}
}

func TestRunDryRunAndLateFailure(t *testing.T) {
	app := scriptTestApp(t)

	var out bytes.Buffer

	err := app.execFile(t.Context(), transfer.Options{MaxThreads: 1, DryRun: true, Output: &out}, "", "preview.iron",
		strings.NewReader("mkdir -p /new\nmv /old /new\nwait\nunsupported"))

	if err == nil || !strings.Contains(err.Error(), "preview.iron:4") {
		t.Fatalf("expected late parse error: %v", err)
	}

	if len(app.Dialog) != 0 || !strings.Contains(out.String(), "would") {
		t.Fatalf("dry run made requests or omitted preview: %q", out.String())
	}
}

func TestRunUploadsShareWorker(t *testing.T) {
	app := scriptTestApp(t)
	file := filepath.Join(t.TempDir(), "with space.txt")

	if err := os.WriteFile(file, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}

	var scopes []string

	err := app.execFile(t.Context(), transfer.Options{
		MaxThreads: 1, DryRun: true,
		ProgressHandler: func(ctx context.Context, _ transfer.Progress) {
			line := transfer.Label(ctx)
			scopes = append(scopes, line)
		},
	}, "", "upload.iron", strings.NewReader("put '"+file+"' /one\nwait\nupload '"+file+"' /two\n"))
	if err != nil {
		t.Fatal(err)
	}

	if len(scopes) != 2 || scopes[0] != "upload.iron:1" || scopes[1] != "upload.iron:3" {
		t.Fatalf("lost submission scopes: %v", scopes)
	}
}

type failedScriptReader struct{}

func (failedScriptReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestRunReadError(t *testing.T) {
	app := scriptTestApp(t)
	err := app.execFile(t.Context(), transfer.Options{MaxThreads: 1}, "", "read.iron", failedScriptReader{})

	if !errors.Is(err, io.ErrUnexpectedEOF) || !strings.Contains(err.Error(), "read.iron:1") {
		t.Fatalf("lost input error: %v", err)
	}
}

func TestExecCommandAndEmbedding(t *testing.T) {
	app := scriptTestApp(t)
	file := filepath.Join(t.TempDir(), "commands.iron")

	if err := os.WriteFile(file, []byte("mkdir /preview\nwait\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer

	if err := app.Exec(t.Context(), &output, &output, "exec", "--dry-run", "--threads", "1", file); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output.String(), "/preview") {
		t.Fatalf("output not routed to embedding writer: %q", output.String())
	}
}

func TestRunCopyWaitThenMove(t *testing.T) {
	app := scriptTestApp(t)
	record := scriptCopyRecord()
	app.AddResponse(record)
	app.AddResponse(record)
	app.AddResponse(msg.EmptyResponse{})
	app.AddResponse(record)
	app.AddResponse(msg.EmptyResponse{})

	var output bytes.Buffer

	err := app.execFile(t.Context(), transfer.Options{MaxThreads: 1, Output: &output}, "", "copy.iron",
		strings.NewReader("cp /source /copy\nwait\nmv /copy /final\n"))
	if err != nil {
		t.Fatal(err)
	}

	if len(app.Dialog) != 0 || !strings.Contains(output.String(), "/copy") || !strings.Contains(output.String(), "/final") {
		t.Fatalf("copy/move operations missing: %d responses, %q", len(app.Dialog), output.String())
	}
}

func scriptCopyRecord() msg.QueryResponse {
	return msg.QueryResponse{
		RowCount: 1, AttributeCount: 14,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 401, Values: []string{"1"}},
			{AttributeIndex: 500, Values: []string{"1"}},
			{AttributeIndex: 406, Values: []string{testGeneric}},
			{AttributeIndex: 404, Values: []string{"0"}},
			{AttributeIndex: 407, Values: []string{"4"}},
			{AttributeIndex: 411, Values: []string{testRods}},
			{AttributeIndex: 412, Values: []string{testTestZoneName}},
			{AttributeIndex: 415, Values: []string{testChecksum}},
			{AttributeIndex: 413, Values: []string{""}},
			{AttributeIndex: 409, Values: []string{testDemoResc}},
			{AttributeIndex: 410, Values: []string{testPath1}},
			{AttributeIndex: 422, Values: []string{testDemoResc}},
			{AttributeIndex: 419, Values: []string{"10000"}},
			{AttributeIndex: 420, Values: []string{"10000"}},
		},
	}
}

func TestScriptMixedCopyDestinations(t *testing.T) {
	app := scriptTestApp(t)

	responses := []any{
		scriptCopyRecord(),
		scriptCopyRecord(),
		scriptCopyRecord(),
		msg.FileDescriptor(1),
		msg.SeekResponse{Offset: 4},
		msg.SeekResponse{},
		msg.EmptyResponse{},
		scriptCopyRecord(),
		scriptCopyRecord(),
	}
	for _, response := range responses {
		app.AddResponse(response)
	}

	var (
		labels []string
		output bytes.Buffer
	)

	worker := transfer.New(app.Client.API, app.Client.API, transfer.Options{
		MaxThreads: 1, DryRun: true, Output: &output,
		ProgressHandler: func(_ context.Context, progress transfer.Progress) {
			labels = append(labels, progress.Label)
		},
	})
	input := "cp /source first\nwait\ncp --remote /source second\nwait\ncp /source third\nwait"

	err := app.readCommands(t.Context(), worker, "mixed.iron", strings.NewReader(input), "/archive", true)
	waitErr := worker.Wait()

	if err != nil || waitErr != nil {
		t.Fatalf("mixed copies failed: %v, %v", err, waitErr)
	}

	if len(labels) != 1 || !strings.HasSuffix(labels[0], "/archive/second") {
		t.Fatalf("incorrect remote copy destination: %v", labels)
	}

	for _, target := range []string{app.Path("first"), app.Path("third")} {
		if !strings.Contains(output.String(), target) {
			t.Fatalf("incorrect local copy destination: %q", output.String())
		}
	}

	if len(app.Dialog) != 0 {
		t.Fatalf("unconsumed copy requests: %d", len(app.Dialog))
	}
}

func TestScriptRemoteCopyRequiresEnvironment(t *testing.T) {
	app := scriptTestApp(t)
	err := app.execFile(t.Context(), transfer.Options{MaxThreads: 1}, "", "remote.iron",
		strings.NewReader("cp --remote /source /target"))

	if err == nil || !strings.Contains(err.Error(), "remote.iron:1") ||
		!strings.Contains(err.Error(), "--target-environment") {
		t.Fatalf("expected attributed missing-environment error: %v", err)
	}

	if len(app.Dialog) != 0 {
		t.Fatal("invalid remote copy made requests")
	}
}

func TestRunLongLine(t *testing.T) {
	app := scriptTestApp(t)

	err := app.execFile(t.Context(), transfer.Options{MaxThreads: 1, Output: io.Discard, DryRun: true}, "", "long.iron",
		strings.NewReader("mkdir /"+strings.Repeat("x", 70_000)))
	if err != nil {
		t.Fatal(err)
	}
}

func TestRunErrorHandlerReceivesLineContext(t *testing.T) {
	app := scriptTestApp(t)
	failure := errors.New("mkdir failed")
	app.AddResponse(failure)

	lines := make(chan string, 1)

	err := app.execFile(t.Context(), transfer.Options{
		MaxThreads: 1,
		ErrorHandler: func(ctx context.Context, _, _ string, err error) error {
			line := transfer.Label(ctx)
			lines <- line

			return fmt.Errorf("%s: %w", line, err)
		},
	}, "", "failure.iron", strings.NewReader("# header\nmkdir /one\nwait\nmkdir /two"))
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), "failure.iron:2") {
		t.Fatalf("lost script error attribution: %v", err)
	}

	if line := <-lines; line != "failure.iron:2" {
		t.Fatalf("error handler lost line context: %q", line)
	}
}
