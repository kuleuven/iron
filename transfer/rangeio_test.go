package transfer

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
)

// Tests for RangeReader implementations

func TestReaderAtRangeReader(t *testing.T) {
	data := []byte("Hello, World!")
	reader := &ReaderAtRangeReader{strings.NewReader(string(data))}

	rangeReader := reader.Range(0, 5)
	buf := make([]byte, 5)

	n, err := rangeReader.Read(buf)
	if err != nil && err != io.EOF {
		t.Errorf("Unexpected error: %v", err)
	}

	if n != 5 {
		t.Errorf("Expected 5 bytes, got %d", n)
	}

	if string(buf) != "Hello" {
		t.Errorf("Expected 'Hello', got %q", string(buf))
	}
}

type nopCloser struct {
	io.ReadSeeker
	io.Closer
}

func TestReopenRangeReader(t *testing.T) {
	data := []byte("Hello, World!")

	reader := &ReopenRangeReader{
		ReadSeekCloser: &nopCloser{strings.NewReader(string(data)), io.NopCloser(nil)},
		Reopen: func() (io.ReadSeekCloser, error) {
			return &nopCloser{strings.NewReader(string(data)), io.NopCloser(nil)}, nil
		},
	}

	defer reader.Close()

	rangeReader := reader.Range(0, 5)
	buf := make([]byte, 5)

	n, err := rangeReader.Read(buf)
	if err != nil && err != io.EOF {
		t.Errorf("Unexpected error: %v", err)
	}

	if n != 5 {
		t.Errorf("Expected 5 bytes, got %d", n)
	}

	if string(buf) != "Hello" {
		t.Errorf("Expected 'Hello', got %q", string(buf))
	}

	rangeReader = reader.Range(7, 5)
	buf = make([]byte, 5)

	n, err = rangeReader.Read(buf)
	if err != nil && err != io.EOF {
		t.Errorf("Unexpected error: %v", err)
	}

	if n != 5 {
		t.Errorf("Expected 5 bytes, got %d", n)
	}

	if string(buf) != "World" {
		t.Errorf("Expected 'World', got %q", string(buf))
	}
}

func TestErrorReader(t *testing.T) {
	expectedErr := errors.New("test error")
	reader := errorReader{err: expectedErr}

	buf := make([]byte, 10)
	n, err := reader.Read(buf)

	if n != 0 {
		t.Errorf("Expected 0 bytes, got %d", n)
	}

	if err != expectedErr {
		t.Errorf("Expected %v, got %v", expectedErr, err)
	}
}

// Tests for RangeWriter implementations

func TestWriterAtRangeWriter(t *testing.T) {
	buf := make([]byte, 20)
	writer := &WriterAtRangeWriter{&bytesWriterAt{buf, 0}}

	rangeWriter := writer.Range(5, 5)
	data := []byte("Hallo")

	n, err := rangeWriter.Write(data)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if n != 5 {
		t.Errorf("Expected 5 bytes written, got %d", n)
	}

	if string(buf[5:10]) != "Hallo" {
		t.Errorf("Expected 'Hallo' at offset 5, got %q", string(buf[5:10]))
	}
}

func TestReopenRangeWriter(t *testing.T) {
	buf := make([]byte, 20)

	writer := &ReopenRangeWriter{
		WriteSeekCloser: &bytesWriterAt{buf, 0},
		Reopen: func() (WriteSeekCloser, error) {
			return &bytesWriterAt{buf, 0}, nil
		},
	}

	rangeWriter := writer.Range(6, 5)
	data := []byte("World")

	n, err := rangeWriter.Write(data)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if n != 5 {
		t.Errorf("Expected 5 bytes written, got %d", n)
	}

	if string(buf[6:11]) != "World" {
		t.Errorf("Expected 'World' at offset 6, got %q", string(buf[6:11]))
	}

	rangeWriter = writer.Range(0, 5)
	data = []byte("Grand")

	n, err = rangeWriter.Write(data)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if n != 5 {
		t.Errorf("Expected 5 bytes written, got %d", n)
	}

	if string(buf[0:5]) != "Grand" {
		t.Errorf("Expected 'Grand' at offset 0, got %q", string(buf[0:5]))
	}
}

func TestSectionWriter(t *testing.T) {
	buf := make([]byte, 20)
	wa := &bytesWriterAt{buf, 0}
	sw := &sectionWriter{WriterAt: wa, off: 5, len: 5}

	// Write within limits
	data := []byte("Hi")

	n, err := sw.Write(data)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if n != 2 {
		t.Errorf("Expected 2 bytes written, got %d", n)
	}

	// Write exceeding limits
	data = []byte("ToolongData")

	n, err = sw.Write(data)
	if err != io.ErrShortWrite {
		t.Errorf("Expected ErrShortWrite, got %v", err)
	}

	if n != 3 { // Only 3 bytes left in section
		t.Errorf("Expected 3 bytes written, got %d", n)
	}

	// Write when section is full
	n, err = sw.Write([]byte("X"))
	if err != io.EOF {
		t.Errorf("Expected EOF, got %v", err)
	}

	if n != 0 {
		t.Errorf("Expected 0 bytes written, got %d", n)
	}
}

func TestLimitWriter(t *testing.T) {
	buf := &bytes.Buffer{}
	lw := &limitWriter{Writer: buf, limit: 5}

	// Write within limit
	data := []byte("Hi")

	n, err := lw.Write(data)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if n != 2 {
		t.Errorf("Expected 2 bytes written, got %d", n)
	}

	// Write exceeding limit
	data = []byte("ToolongData")

	n, err = lw.Write(data)
	if err != io.ErrShortWrite {
		t.Errorf("Expected ErrShortWrite, got %v", err)
	}

	if n != 3 { // Only 3 bytes left
		t.Errorf("Expected 3 bytes written, got %d", n)
	}

	// Write when limit reached
	n, err = lw.Write([]byte("X"))
	if err != io.EOF {
		t.Errorf("Expected EOF, got %v", err)
	}

	if n != 0 {
		t.Errorf("Expected 0 bytes written, got %d", n)
	}
}

func TestErrorWriter(t *testing.T) {
	expectedErr := errors.New("test error")
	writer := errorWriter{err: expectedErr}

	n, err := writer.Write([]byte("test"))

	if n != 0 {
		t.Errorf("Expected 0 bytes written, got %d", n)
	}

	if err != expectedErr {
		t.Errorf("Expected %v, got %v", expectedErr, err)
	}
}

// Helper type for testing WriterAt
type bytesWriterAt struct {
	buf    []byte
	offset int64
}

func (w *bytesWriterAt) WriteAt(p []byte, off int64) (int, error) {
	if off < 0 || off >= int64(len(w.buf)) {
		return 0, errors.New("offset out of range")
	}

	n := copy(w.buf[off:], p)
	if n < len(p) {
		return n, io.ErrShortWrite
	}

	return n, nil
}

func (w *bytesWriterAt) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		w.offset = offset
	case io.SeekCurrent:
		w.offset += offset
	case io.SeekEnd:
		w.offset = int64(len(w.buf)) + offset
	}

	return w.offset, nil
}

func (w *bytesWriterAt) Write(p []byte) (int, error) {
	n := copy(w.buf[w.offset:], p)

	w.offset += int64(n)

	return n, nil
}

func (w *bytesWriterAt) Close() error {
	return nil
}

// Benchmark tests

func BenchmarkCopy(b *testing.B) {
	data := strings.Repeat("x", 1024*1024) // 1MB

	b.ResetTimer()

	for range b.N {
		reader := strings.NewReader(data)
		writer := &bytes.Buffer{}
		Copy(writer, reader, int64(len(data)), nil)
	}
}

func BenchmarkCopyN(b *testing.B) {
	data := strings.Repeat("x", 1024*1024) // 1MB

	b.ResetTimer()

	for range b.N {
		reader := &mockRangeReader{data: []byte(data)}
		writer := newMockRangeWriter(int64(len(data)))
		CopyN(writer, reader, int64(len(data)), 4, nil)
	}
}

// Concurrent safety tests

func TestProgressConcurrentSafety(t *testing.T) { //nolint:funlen
	progress := &mockProgress{}

	var wg sync.WaitGroup

	numGoroutines := 10
	numOperations := 100

	wg.Add(numGoroutines * 4) // 4 operations per goroutine

	// Test concurrent access to all progress methods
	for range numGoroutines {
		go func() {
			defer wg.Done()

			for range numOperations {
				progress.AddTotalFiles(1)
			}
		}()

		go func() {
			defer wg.Done()

			for range numOperations {
				progress.AddTransferredFiles(1)
			}
		}()

		go func() {
			defer wg.Done()

			for range numOperations {
				progress.AddTotalBytes(100)
			}
		}()

		go func() {
			defer wg.Done()

			for range numOperations {
				progress.AddTransferredBytes(50)
			}
		}()
	}

	wg.Wait()

	totalFiles, transferredFiles, totalBytes, transferredBytes := progress.GetStats()
	expectedFiles := numGoroutines * numOperations
	expectedBytes := int64(numGoroutines * numOperations)

	if totalFiles != expectedFiles {
		t.Errorf("Expected totalFiles=%d, got %d", expectedFiles, totalFiles)
	}

	if transferredFiles != expectedFiles {
		t.Errorf("Expected transferredFiles=%d, got %d", expectedFiles, transferredFiles)
	}

	if totalBytes != expectedBytes*100 {
		t.Errorf("Expected totalBytes=%d, got %d", expectedBytes*100, totalBytes)
	}

	if transferredBytes != expectedBytes*50 {
		t.Errorf("Expected transferredBytes=%d, got %d", expectedBytes*50, transferredBytes)
	}
}

func TestWorkerConcurrentCopies(t *testing.T) {
	progress := &mockProgress{}
	worker := New(progress, nil)

	numCopies := 5
	dataSize := 1000

	for range numCopies {
		data := strings.Repeat("x", dataSize)
		reader := strings.NewReader(data)
		writer := &bytes.Buffer{}
		worker.Copy(writer, reader, int64(len(data)))
	}

	err := worker.Wait()
	if err != nil {
		t.Fatalf("Worker.Wait() failed: %v", err)
	}

	totalFiles, transferredFiles, totalBytes, transferredBytes := progress.GetStats()
	if totalFiles != numCopies {
		t.Errorf("Expected totalFiles=%d, got %d", numCopies, totalFiles)
	}

	if transferredFiles != numCopies {
		t.Errorf("Expected transferredFiles=%d, got %d", numCopies, transferredFiles)
	}

	expectedBytes := int64(numCopies * dataSize)
	if totalBytes != expectedBytes {
		t.Errorf("Expected totalBytes=%d, got %d", expectedBytes, totalBytes)
	}

	if transferredBytes != expectedBytes {
		t.Errorf("Expected transferredBytes=%d, got %d", expectedBytes, transferredBytes)
	}
}
