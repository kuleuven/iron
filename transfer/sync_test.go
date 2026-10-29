package transfer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kuleuven/iron/api"
)

func TestDownloadDirDryRunExistingTarget(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	for _, local := range []string{".", "./", dir, dir + "/"} {
		t.Run(local, func(t *testing.T) {
			writeIdenticalLocalFile(t)

			conn := &api.MockConn{}
			conn.AddResponses(responses)

			var output bytes.Buffer

			worker := New(mockTransferAPI(conn), nil, Options{
				DryRun:          true,
				Output:          &output,
				ProgressHandler: func(context.Context, Progress) {},
			})
			worker.DownloadDir(t.Context(), local, "/test", Flags{})

			if err := worker.Wait(); err != nil {
				t.Fatal(err)
			}

			if output.Len() != 0 {
				t.Fatalf("unchanged directory has dry-run actions: %q", output.String())
			}
		})
	}
}

func TestComputeLocalDirPaths(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := os.Mkdir("sub", 0o700); err != nil {
		t.Fatal(err)
	}

	for _, local := range []string{".", "./", dir, dir + "/", "./sub/.."} {
		for _, tc := range []struct {
			name     string
			existing bool
			upload   bool
		}{
			{name: "download missing"},
			{name: "download existing", existing: true},
			{name: "upload existing", existing: true, upload: true},
		} {
			t.Run(local+"/"+tc.name, func(t *testing.T) {
				testComputeLocalDirPaths(t, local, tc.existing, tc.upload)
			})
		}
	}
}

func testComputeLocalDirPaths(t *testing.T, local string, existing, upload bool) {
	t.Helper()

	if existing {
		writeIdenticalLocalFile(t)
	}

	conn := &api.MockConn{}
	conn.AddResponses(responses)

	worker := &Worker{
		IndexPool: mockTransferAPI(conn),
		options: Options{
			ErrorHandler: func(_ context.Context, _, _ string, err error) error {
				return err
			},
		},
	}
	queue := make(chan Task, 16)
	flags := Flags{Delete: true, IgnorePatterns: []string{"sub"}}

	var err error

	if upload {
		err = worker.ComputeUploadDir(t.Context(), local, "/test", queue, flags)
	} else {
		err = worker.ComputeDownloadDir(t.Context(), "/test", local, queue, flags)
	}

	if err != nil {
		t.Fatal(err)
	}

	if existing {
		if len(queue) != 0 {
			t.Fatalf("identical file scheduled for modification: %+v", <-queue)
		}

		return
	}

	if len(queue) != 1 {
		t.Fatalf("got %d tasks, want one download", len(queue))
	}

	task := <-queue
	if task.Action != TransferFile || task.Source != testFile1 ||
		task.Target != filepath.Join(local, "file1") || task.Size != int64(len(testStr)) {
		t.Errorf("unexpected download task: %+v", task)
	}
}

func writeIdenticalLocalFile(t *testing.T) {
	t.Helper()

	if err := os.WriteFile("file1", []byte(testStr), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := os.Remove("file1"); err != nil {
			t.Error(err)
		}
	})

	modTime := time.Unix(10000, 0)
	if err := os.Chtimes("file1", modTime, modTime); err != nil {
		t.Fatal(err)
	}
}
