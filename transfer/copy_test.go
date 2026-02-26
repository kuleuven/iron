package transfer

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestCalculateRangeSize(t *testing.T) { //nolint:funlen
	tests := []struct {
		name    string
		size    int64
		threads int
		check   func(t *testing.T, result int64)
	}{
		{
			name:    "small file single thread",
			size:    1024,
			threads: 1,
			check: func(t *testing.T, result int64) {
				if result < MinimumRangeSize {
					t.Errorf("expected at least %d, got %d", MinimumRangeSize, result)
				}
			},
		},
		{
			name:    "minimum range size enforced",
			size:    100,
			threads: 4,
			check: func(t *testing.T, result int64) {
				if result < MinimumRangeSize {
					t.Errorf("expected at least minimum range size %d, got %d", MinimumRangeSize, result)
				}
			},
		},
		{
			name:    "aligned to buffer size",
			size:    100 * 1024 * 1024,
			threads: 2,
			check: func(t *testing.T, result int64) {
				if result%BufferSize != 0 {
					t.Errorf("expected result to be aligned to buffer size %d, got %d (remainder %d)", BufferSize, result, result%BufferSize)
				}
			},
		},
		{
			name:    "covers full file",
			size:    200 * 1024 * 1024,
			threads: 3,
			check: func(t *testing.T, result int64) {
				if result*int64(3) < 200*1024*1024 {
					t.Errorf("range size %d * 3 threads = %d, which is less than file size %d", result, result*3, 200*1024*1024)
				}
			},
		},
		{
			name:    "single thread large file",
			size:    1024 * 1024 * 1024,
			threads: 1,
			check: func(t *testing.T, result int64) {
				if result < 1024*1024*1024 {
					t.Errorf("single thread range should cover entire file, got %d", result)
				}
			},
		},
		{
			name:    "many threads",
			size:    1024 * 1024 * 1024,
			threads: 16,
			check: func(t *testing.T, result int64) {
				if result%BufferSize != 0 {
					t.Errorf("expected alignment to buffer size, got remainder %d", result%BufferSize)
				}

				if result*16 < 1024*1024*1024 {
					t.Errorf("range*threads should cover file size")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateRangeSize(tt.size, tt.threads)
			tt.check(t, result)
		})
	}
}

func TestCopyBuffer(t *testing.T) {
	data := []byte(strings.Repeat("hello world\n", 1000))
	reader := bytes.NewReader(data)
	writer := &bytes.Buffer{}
	pw := &progressWriter{
		handler: func(progress Progress) {},
	}

	err := copyBuffer(writer, reader, pw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Equal(writer.Bytes(), data) {
		t.Errorf("copied data doesn't match: got %d bytes, want %d bytes", writer.Len(), len(data))
	}

	if pw.progress.Transferred != int64(len(data)) {
		t.Errorf("progress writer shows %d bytes transferred, want %d", pw.progress.Transferred, len(data))
	}
}

func TestCopyBufferEmpty(t *testing.T) {
	reader := bytes.NewReader(nil)
	writer := &bytes.Buffer{}
	pw := &progressWriter{
		handler: func(progress Progress) {},
	}

	err := copyBuffer(writer, reader, pw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if writer.Len() != 0 {
		t.Errorf("expected empty output, got %d bytes", writer.Len())
	}
}

type memWriteSeekCloser struct {
	buf    bytes.Buffer
	offset int64
	closed bool
}

func (m *memWriteSeekCloser) Write(p []byte) (int, error) {
	// Extend buffer if needed
	for int64(m.buf.Len()) < m.offset+int64(len(p)) {
		m.buf.Write([]byte{0})
	}

	b := m.buf.Bytes()
	copy(b[m.offset:], p)
	m.offset += int64(len(p))

	return len(p), nil
}

func (m *memWriteSeekCloser) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		m.offset = offset
	case io.SeekCurrent:
		m.offset += offset
	case io.SeekEnd:
		m.offset = int64(m.buf.Len()) + offset
	}

	return m.offset, nil
}

func (m *memWriteSeekCloser) Close() error {
	m.closed = true
	return nil
}

func TestCircularWriterSingleThread(t *testing.T) {
	mem := &memWriteSeekCloser{}

	cw := &CircularWriter{
		WriteSeekCloser: mem,
		MaxThreads:      1,
	}

	data := []byte("hello world!")

	n, err := cw.Write(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n != len(data) {
		t.Errorf("expected %d bytes written, got %d", len(data), n)
	}

	if err := cw.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}

	if !mem.closed {
		t.Error("expected writer to be closed")
	}
}

func TestCircularWriterNoWorkers(t *testing.T) {
	mem := &memWriteSeekCloser{}

	cw := &CircularWriter{
		WriteSeekCloser: mem,
		MaxThreads:      1,
	}

	// Close without writing
	if err := cw.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}

	if !mem.closed {
		t.Error("expected writer to be closed even without writes")
	}
}

func TestCircularReaderEmpty(t *testing.T) {
	cr := &CircularReader{
		ReadSeekCloser: nopReadSeekCloser{bytes.NewReader(nil)},
		MaxThreads:     1,
		Size:           0,
	}

	buf := make([]byte, 10)

	_, err := cr.Read(buf)
	if err != io.EOF {
		t.Errorf("expected io.EOF for empty reader, got %v", err)
	}
}

type nopReadSeekCloser struct {
	*bytes.Reader
}

func (n nopReadSeekCloser) Close() error { return nil }

func TestIoReader(t *testing.T) {
	data := []byte("test data")
	r := &ioReader{reader: bytes.NewReader(data)}

	buf := make([]byte, len(data))

	n, err := r.Read(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n != len(data) {
		t.Errorf("expected %d bytes, got %d", len(data), n)
	}

	if !bytes.Equal(buf, data) {
		t.Errorf("data mismatch")
	}
}
