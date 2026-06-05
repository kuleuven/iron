//nolint:goconst
package transfer

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileReaderAccessors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.bin")

	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}

	r := fileReader{
		name: path,
		stat: stat,
		File: f,
	}

	if r.Name() != path {
		t.Errorf("Name() = %q, want %q", r.Name(), path)
	}

	if r.Size() != int64(len("hello")) {
		t.Errorf("Size() = %d, want %d", r.Size(), len("hello"))
	}

	if r.ModTime().IsZero() {
		t.Error("ModTime() returned zero time")
	}

	// Checksum uses cached value when set.
	cached := fileReader{checksum: []byte("cached")}

	got, err := cached.Checksum(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != "cached" {
		t.Errorf("Checksum() = %q, want %q", got, "cached")
	}

	// Without cache, Checksum reads the underlying file.
	got, err = r.Checksum(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if len(got) == 0 {
		t.Error("expected non-empty checksum")
	}
}

func TestFileWriterAccessors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.bin")

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := f.WriteString("payload"); err != nil {
		t.Fatal(err)
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	w := fileWriter{name: path, File: f}

	if w.Name() != path {
		t.Errorf("Name() = %q, want %q", w.Name(), path)
	}

	got, err := w.Checksum(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if len(got) == 0 {
		t.Error("expected non-empty checksum")
	}

	mtime := time.Unix(1_600_000_000, 0)
	if err := w.Touch(mtime); err != nil {
		t.Fatal(err)
	}

	stat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if !stat.ModTime().Equal(mtime) {
		t.Errorf("Touch did not set mtime: got %v, want %v", stat.ModTime(), mtime)
	}

	if err := w.Remove(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected file removed, got %v", err)
	}
}

func TestTaskReaderAccessors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "task.bin")

	if err := os.WriteFile(path, []byte("xyz"), 0o600); err != nil {
		t.Fatal(err)
	}

	mtime := time.Unix(1_700_000_000, 0)
	tr := &taskReader{
		task: Task{
			Path:    path,
			Size:    3,
			ModTime: mtime,
		},
	}

	if tr.Name() != path {
		t.Errorf("Name() = %q, want %q", tr.Name(), path)
	}

	if tr.Size() != 3 {
		t.Errorf("Size() = %d, want 3", tr.Size())
	}

	if !tr.ModTime().Equal(mtime) {
		t.Errorf("ModTime() = %v, want %v", tr.ModTime(), mtime)
	}

	got, err := tr.Checksum(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if len(got) == 0 {
		t.Error("expected non-empty checksum")
	}

	// When pre-supplied, cached checksum is returned.
	tr.task.Checksum = []byte("cache")

	got, err = tr.Checksum(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != "cache" {
		t.Errorf("Checksum() = %q, want %q", got, "cache")
	}
}

func TestWorkerErrorAndLog(t *testing.T) {
	errCh := make(chan error, 1)
	worker := New(nil, nil, Options{
		MaxThreads: 1,
		Output:     io.Discard,
		ErrorHandler: func(_, _ string, err error) error {
			errCh <- err
			return nil
		},
	})

	want := errors.New("scheduled")
	worker.Error("/local", "/remote", want)

	if err := worker.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}

	select {
	case got := <-errCh:
		if !errors.Is(got, want) {
			t.Errorf("error handler got %v, want %v", got, want)
		}
	default:
		t.Error("error handler not invoked")
	}

	// log shouldn't panic.
	worker.log(Task{Path: "/a", IrodsPath: "/b"})
}

func TestWorkerProgressNoopHandler(t *testing.T) {
	worker := New(nil, nil, Options{})

	// Should not panic with default no-op handler.
	worker.Progress(Progress{Label: "test"})
}
