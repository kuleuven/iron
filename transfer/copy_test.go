package transfer

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
)

// Mock implementations for testing

type mockProgress struct {
	totalFiles       int
	transferredFiles int
	totalBytes       int64
	transferredBytes int64
	mu               sync.Mutex
}

func (m *mockProgress) AddTotalFiles(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalFiles += n
}

func (m *mockProgress) AddTransferredFiles(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.transferredFiles += n
}

func (m *mockProgress) AddTotalBytes(n int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalBytes += n
}

func (m *mockProgress) AddTransferredBytes(n int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.transferredBytes += n
}

func (m *mockProgress) GetStats() (int, int, int64, int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.totalFiles, m.transferredFiles, m.totalBytes, m.transferredBytes
}

type mockRangeReader struct {
	data []byte
}

func (m *mockRangeReader) Range(offset, length int64) io.Reader {
	if offset >= int64(len(m.data)) {
		return strings.NewReader("")
	}

	end := offset + length
	if end > int64(len(m.data)) {
		end = int64(len(m.data))
	}

	return strings.NewReader(string(m.data[offset:end]))
}

type mockRangeWriter struct {
	data   []byte
	writes []writeOp
	mu     sync.Mutex
}

type writeOp struct {
	offset int64
	data   []byte
}

func newMockRangeWriter(size int64) *mockRangeWriter {
	return &mockRangeWriter{
		data: make([]byte, size),
	}
}

func (m *mockRangeWriter) Range(offset, length int64) io.Writer {
	return &mockSectionWriter{
		parent: m,
		offset: offset,
		length: length,
	}
}

func (m *mockRangeWriter) recordWrite(offset int64, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.writes = append(m.writes, writeOp{offset: offset, data: append([]byte(nil), data...)})

	copy(m.data[offset:], data)
}

func (m *mockRangeWriter) GetData() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]byte(nil), m.data...)
}

type mockSectionWriter struct {
	parent *mockRangeWriter
	offset int64
	length int64
	pos    int64
}

func (w *mockSectionWriter) Write(b []byte) (int, error) {
	if w.pos >= w.length {
		return 0, io.EOF
	}

	available := w.length - w.pos
	if int64(len(b)) > available {
		b = b[:available]
	}

	w.parent.recordWrite(w.offset+w.pos, b)
	w.pos += int64(len(b))

	return len(b), nil
}

// Tests for Worker and Copy functions

func TestCopy(t *testing.T) {
	data := "Hello, World! This is test data for copying."
	reader := strings.NewReader(data)
	writer := &bytes.Buffer{}
	progress := &mockProgress{}

	err := Copy(writer, reader, int64(len(data)), progress)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if writer.String() != data {
		t.Errorf("Expected %q, got %q", data, writer.String())
	}

	totalFiles, transferredFiles, totalBytes, transferredBytes := progress.GetStats()
	if totalFiles != 1 {
		t.Errorf("Expected totalFiles=1, got %d", totalFiles)
	}

	if transferredFiles != 1 {
		t.Errorf("Expected transferredFiles=1, got %d", transferredFiles)
	}

	if totalBytes != int64(len(data)) {
		t.Errorf("Expected totalBytes=%d, got %d", len(data), totalBytes)
	}

	if transferredBytes != int64(len(data)) {
		t.Errorf("Expected transferredBytes=%d, got %d", len(data), transferredBytes)
	}
}

func TestCopyWithNilProgress(t *testing.T) {
	data := "Test data"
	reader := strings.NewReader(data)
	writer := &bytes.Buffer{}

	err := Copy(writer, reader, int64(len(data)), nil)
	if err != nil {
		t.Fatalf("Copy with nil progress failed: %v", err)
	}

	if writer.String() != data {
		t.Errorf("Expected %q, got %q", data, writer.String())
	}
}

func TestCopyWithErrorReader(t *testing.T) {
	expectedErr := errors.New("read error")
	reader := errorReader{err: expectedErr}
	writer := &bytes.Buffer{}
	progress := &mockProgress{}

	err := Copy(writer, reader, 100, progress)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

func TestCopyN(t *testing.T) {
	data := strings.Repeat("Hello, World! ", 1000) // Create larger data for multi-threading
	reader := &mockRangeReader{data: []byte(data)}
	writer := newMockRangeWriter(int64(len(data)))
	progress := &mockProgress{}

	err := CopyN(writer, reader, int64(len(data)), 4, progress)
	if err != nil {
		t.Fatalf("CopyN failed: %v", err)
	}

	result := writer.GetData()
	if string(result) != data {
		t.Errorf("Data mismatch. Expected length %d, got %d", len(data), len(result))
	}

	totalFiles, transferredFiles, totalBytes, transferredBytes := progress.GetStats()
	if totalFiles != 1 {
		t.Errorf("Expected totalFiles=1, got %d", totalFiles)
	}

	if transferredFiles != 1 {
		t.Errorf("Expected transferredFiles=1, got %d", transferredFiles)
	}

	if totalBytes != int64(len(data)) {
		t.Errorf("Expected totalBytes=%d, got %d", len(data), totalBytes)
	}

	if transferredBytes != int64(len(data)) {
		t.Errorf("Expected transferredBytes=%d, got %d", len(data), transferredBytes)
	}
}

func TestWorkerNew(t *testing.T) {
	progress := &mockProgress{}
	errorHandler := func(err error) error {
		return errors.New("handled: " + err.Error())
	}

	worker := New(progress, errorHandler)
	if worker.progress != progress {
		t.Error("Progress not set correctly")
	}

	if worker.errorHandler == nil {
		t.Error("Error handler not set correctly")
	}
}

func TestWorkerNewWithNils(t *testing.T) {
	worker := New(nil, nil)

	// Should use NullProgress when progress is nil
	if worker.progress == nil {
		t.Error("Expected NullProgress, got nil")
	}

	// Should have default error handler
	if worker.errorHandler == nil {
		t.Error("Expected default error handler, got nil")
	}

	// Test default error handler
	testErr := errors.New("test error")
	if worker.errorHandler(testErr) != testErr {
		t.Error("Default error handler should return the same error")
	}
}

func TestWorkerWithErrorHandler(t *testing.T) {
	progress := &mockProgress{}
	handledErr := errors.New("handled error")
	errorHandler := func(err error) error {
		return handledErr
	}

	worker := New(progress, errorHandler)

	// Test with an error reader
	originalErr := errors.New("read error")
	reader := errorReader{err: originalErr}
	writer := &bytes.Buffer{}

	worker.Copy(writer, reader, 100)

	err := worker.Wait()
	if err != handledErr {
		t.Errorf("Expected handled error %v, got %v", handledErr, err)
	}
}

// Tests for utility functions

func TestCalculateRangeSize(t *testing.T) {
	tests := []struct {
		size     int64
		threads  int
		expected int64
	}{
		{100 * 1024 * 1024, 4, 32 * 1024 * 1024},     // 100MB / 4 threads
		{10 * 1024 * 1024, 4, 32 * 1024 * 1024},      // Small size, should use minimum
		{200 * 1024 * 1024, 2, 13 * 8 * 1024 * 1024}, // 200MB / 2 threads, aligned
	}

	for _, test := range tests {
		result := calculateRangeSize(test.size, test.threads)
		if result != test.expected {
			t.Errorf("calculateRangeSize(%d, %d) = %d, expected %d",
				test.size, test.threads, result, test.expected)
		}
	}
}

func TestProgressWriter(t *testing.T) {
	progress := &mockProgress{}
	pw := &progressWriter{Progress: progress}

	data := []byte("test data")

	n, err := pw.Write(data)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if n != len(data) {
		t.Errorf("Expected %d bytes written, got %d", len(data), n)
	}

	_, _, _, transferredBytes := progress.GetStats() //nolint:dogsled

	if transferredBytes != int64(len(data)) {
		t.Errorf("Expected %d transferred bytes, got %d", len(data), transferredBytes)
	}
}
