package transfer

import (
	"io"
	"time"
)

var BufferSize int64 = 8 * 1024 * 1024

var MinimumRangeSize int64 = 32 * 1024 * 1024

func calculateRangeSize(size int64, threads int) int64 {
	rangeSize := size / int64(threads)

	// Align rangeSize to a multiple of BufferSize
	if rangeSize%BufferSize != 0 {
		rangeSize += BufferSize - rangeSize%BufferSize
	}

	if rangeSize < MinimumRangeSize {
		rangeSize = MinimumRangeSize
	}

	for rangeSize*int64(threads) < size {
		rangeSize += BufferSize
	}

	return rangeSize
}

var CopyBufferDelay time.Duration

func copyBuffer(w io.Writer, r io.Reader, pw *progressWriter) error {
	if CopyBufferDelay > 0 {
		time.Sleep(CopyBufferDelay)
	}

	buffer := make([]byte, BufferSize)

	_, err := io.CopyBuffer(io.MultiWriter(w, pw), r, buffer)

	return err
}
