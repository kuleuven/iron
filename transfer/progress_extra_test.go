package transfer

import (
	"bytes"
	"testing"
	"time"
)

func TestProgressLabel(t *testing.T) {
	tests := []struct {
		local, remote, expected string
	}{
		{"local.txt", "remote.txt", "local.txt"},
		{"", "remote.txt", "remote.txt"},
		{"", "", ""},
		{"local.txt", "", "local.txt"},
	}

	for _, tt := range tests {
		result := ProgressLabel(tt.local, tt.remote)
		if result != tt.expected {
			t.Errorf("ProgressLabel(%q, %q) = %q, want %q", tt.local, tt.remote, result, tt.expected)
		}
	}
}

func TestBar(t *testing.T) {
	tests := []struct {
		percent float64
		hashes  int
	}{
		{0, 0},
		{25, 5},
		{50, 10},
		{100, 20},
		{12.5, 2},
	}

	for _, tt := range tests {
		result := bar(tt.percent)
		if len(result) != 20 {
			t.Errorf("bar(%v) length = %d, want 20", tt.percent, len(result))
		}

		hashCount := 0
		for _, c := range result {
			if c == '#' {
				hashCount++
			}
		}

		if hashCount != tt.hashes {
			t.Errorf("bar(%v) has %d hashes, want %d", tt.percent, hashCount, tt.hashes)
		}
	}
}

func TestPBHandlerRegistration(t *testing.T) {
	pb := &PB{
		actual:       map[string]Progress{},
		done:         make(chan struct{}),
		wait:         make(chan struct{}),
		started:      time.Now(),
		outputBuffer: &bytes.Buffer{},
		w:            &bytes.Buffer{},
	}

	// Register a file
	pb.Handler(Progress{
		Label: "test.txt",
		Size:  1000,
	})

	if pb.bytesTotal != 1000 {
		t.Errorf("expected bytesTotal=1000, got %d", pb.bytesTotal)
	}

	// Update with re-registration of same label with different size
	pb.Handler(Progress{
		Label: "test.txt",
		Size:  2000,
	})

	if pb.bytesTotal != 2000 {
		t.Errorf("expected bytesTotal=2000 after re-registration, got %d", pb.bytesTotal)
	}
}

func TestPBHandlerTransferOngoing(t *testing.T) {
	pb := &PB{
		actual:       map[string]Progress{},
		done:         make(chan struct{}),
		wait:         make(chan struct{}),
		started:      time.Now(),
		outputBuffer: &bytes.Buffer{},
		w:            &bytes.Buffer{},
	}

	// Register
	pb.Handler(Progress{
		Label: "test.txt",
		Size:  1000,
	})

	// Transfer ongoing
	pb.Handler(Progress{
		Label:       "test.txt",
		Size:        1000,
		StartedAt:   time.Now(),
		Transferred: 500,
		Increment:   500,
	})

	if pb.bytesTransferred != 500 {
		t.Errorf("expected bytesTransferred=500, got %d", pb.bytesTransferred)
	}
}

func TestPBHandlerTransferComplete(t *testing.T) {
	buf := &bytes.Buffer{}
	pb := &PB{
		actual:       map[string]Progress{},
		done:         make(chan struct{}),
		wait:         make(chan struct{}),
		started:      time.Now(),
		outputBuffer: buf,
		w:            &bytes.Buffer{},
	}

	// Register
	pb.Handler(Progress{
		Label: "test.txt",
		Size:  1000,
	})

	// Complete
	pb.Handler(Progress{
		Label:       "test.txt",
		Size:        1000,
		StartedAt:   time.Now(),
		FinishedAt:  time.Now(),
		Transferred: 1000,
		Increment:   1000,
		Action:      TransferFile,
	})

	if pb.bytesTransferred != 1000 {
		t.Errorf("expected bytesTransferred=1000, got %d", pb.bytesTransferred)
	}

	// Check that transfer is complete (removed from actual)
	if _, exists := pb.actual["test.txt"]; exists {
		t.Error("expected transfer to be removed from actual map after completion")
	}

	// Check that something was written to output buffer
	if buf.Len() == 0 {
		t.Error("expected output buffer to contain completion message")
	}
}

func TestPBHandlerComputeChecksum(t *testing.T) {
	buf := &bytes.Buffer{}
	pb := &PB{
		actual:       map[string]Progress{},
		done:         make(chan struct{}),
		wait:         make(chan struct{}),
		started:      time.Now(),
		outputBuffer: buf,
		w:            &bytes.Buffer{},
	}

	pb.Handler(Progress{
		Action: ComputeChecksum,
		Label:  "test.txt",
	})

	if buf.Len() == 0 {
		t.Error("expected output buffer to contain checksum message")
	}
}

func TestPBHandlerFromStream(t *testing.T) {
	pb := &PB{
		actual:       map[string]Progress{},
		done:         make(chan struct{}),
		wait:         make(chan struct{}),
		started:      time.Now(),
		outputBuffer: &bytes.Buffer{},
		w:            &bytes.Buffer{},
	}

	// FromStream: started but size unknown (Size=0)
	pb.Handler(Progress{
		Label:       "stream.txt",
		Size:        0,
		StartedAt:   time.Now(),
		Increment:   256,
		Transferred: 256,
	})

	if pb.bytesTransferred != 256 {
		t.Errorf("expected bytesTransferred=256, got %d", pb.bytesTransferred)
	}

	if pb.bytesTotal != 256 {
		t.Errorf("expected bytesTotal=256 for stream, got %d", pb.bytesTotal)
	}
}

func TestPBErrorHandler(t *testing.T) {
	buf := &bytes.Buffer{}
	pb := &PB{
		actual:       map[string]Progress{},
		done:         make(chan struct{}),
		wait:         make(chan struct{}),
		started:      time.Now(),
		outputBuffer: buf,
		w:            &bytes.Buffer{},
	}

	err := pb.ErrorHandler("local.txt", "remote.txt", bytes.ErrTooLarge)
	if err != nil {
		t.Errorf("expected nil error from ErrorHandler, got %v", err)
	}

	if pb.errors != 1 {
		t.Errorf("expected 1 error, got %d", pb.errors)
	}

	if buf.Len() == 0 {
		t.Error("expected error message in output buffer")
	}
}

func TestPBWrite(t *testing.T) {
	buf := &bytes.Buffer{}
	pb := &PB{
		actual:       map[string]Progress{},
		done:         make(chan struct{}),
		wait:         make(chan struct{}),
		started:      time.Now(),
		outputBuffer: buf,
		w:            &bytes.Buffer{},
	}

	n, err := pb.Write([]byte("test output"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n != 11 {
		t.Errorf("expected 11 bytes written, got %d", n)
	}

	if buf.String() != "test output" {
		t.Errorf("expected 'test output', got %q", buf.String())
	}
}

func TestPBScanCompleted(t *testing.T) {
	pb := &PB{
		actual:       map[string]Progress{},
		done:         make(chan struct{}),
		wait:         make(chan struct{}),
		started:      time.Now(),
		outputBuffer: &bytes.Buffer{},
		w:            &bytes.Buffer{},
	}

	if pb.scanCompleted {
		t.Error("expected scanCompleted to be false initially")
	}

	pb.ScanCompleted()

	if !pb.scanCompleted {
		t.Error("expected scanCompleted to be true after ScanCompleted()")
	}
}

func TestPBElapsed(t *testing.T) {
	pb := &PB{
		actual:       map[string]Progress{},
		done:         make(chan struct{}),
		wait:         make(chan struct{}),
		started:      time.Now().Add(-5 * time.Second),
		outputBuffer: &bytes.Buffer{},
		w:            &bytes.Buffer{},
	}

	elapsed := pb.Elapsed()
	if elapsed < 4*time.Second || elapsed > 10*time.Second {
		t.Errorf("expected elapsed around 5s, got %v", elapsed)
	}
}

func TestPBCloseNoErrors(t *testing.T) {
	w := &bytes.Buffer{}
	pb := ProgressBar(w)

	err := pb.Close()
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestPBCloseWithErrors(t *testing.T) {
	w := &bytes.Buffer{}
	pb := ProgressBar(w)

	pb.ErrorHandler("test.txt", "remote.txt", bytes.ErrTooLarge)

	err := pb.Close()
	if err == nil {
		t.Error("expected error from Close when errors occurred")
	}
}

func TestProgressBarOutput(t *testing.T) {
	w := &bytes.Buffer{}
	pb := ProgressBar(w)

	// Register a file
	pb.Handler(Progress{
		Label: "test.txt",
		Size:  1000,
	})

	// Complete it
	pb.Handler(Progress{
		Label:       "test.txt",
		Size:        1000,
		StartedAt:   time.Now(),
		FinishedAt:  time.Now(),
		Transferred: 1000,
		Increment:   1000,
		Action:      TransferFile,
	})

	// Wait briefly to let ticker fire
	time.Sleep(600 * time.Millisecond)

	err := pb.Close()
	if err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}

	// Output should contain the completion message
	if w.Len() == 0 {
		t.Error("expected output from progress bar")
	}
}

func TestActionFormat(t *testing.T) {
	tests := []struct {
		action   Action
		label    string
		contains string
	}{
		{ComputeChecksum, "file.txt", "c file.txt"},
		{SetModificationTime, "file.txt", "t file.txt"},
		{CreateDirectory, "dir", "+ dir/"},
		{TransferFile, "file.txt", "+ file.txt"},
		{RemoveFile, "file.txt", "- file.txt"},
		{RemoveDirectory, "dir", "- dir/"},
		{Action(99), "unknown", "unknown"},
	}

	for _, tt := range tests {
		result := tt.action.Format(tt.label)
		if result == "" {
			t.Errorf("Action(%d).Format(%q) returned empty string", tt.action, tt.label)
		}

		// Strip ANSI codes and check content
		stripped := stripANSI(result)
		if stripped != tt.contains {
			t.Errorf("Action(%d).Format(%q) = %q (stripped: %q), want %q", tt.action, tt.label, result, stripped, tt.contains)
		}
	}
}

func stripANSI(s string) string {
	var result []byte

	i := 0
	for i < len(s) {
		if s[i] == '\x1B' {
			// Skip until 'm'
			for i < len(s) && s[i] != 'm' {
				i++
			}

			i++ // skip 'm'
		} else {
			result = append(result, s[i])
			i++
		}
	}

	return string(result)
}
