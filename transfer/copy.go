package transfer

import (
	"io"

	"golang.org/x/sync/errgroup"
)

func Copy(w io.Writer, r io.Reader, size int64, progress Progress) error {
	worker := New(progress, nil)

	worker.Copy(w, r, size)

	return worker.Wait()
}

func CopyN(w RangeWriter, r RangeReader, size int64, threads int, progress Progress) error {
	worker := New(progress, nil)

	worker.CopyN(w, r, size, threads)

	return worker.Wait()
}

type Worker struct {
	progress     Progress
	errorHandler func(error) error
	wg           errgroup.Group
}

func New(progess Progress, errorHandler func(error) error) *Worker {
	if progess == nil {
		progess = NullProgress{}
	}

	if errorHandler == nil {
		errorHandler = func(err error) error {
			return err
		}
	}

	return &Worker{
		progress:     progess,
		errorHandler: errorHandler,
	}
}

func (worker *Worker) Copy(w io.Writer, r io.Reader, size int64) {
	worker.progress.AddTotalFiles(1)
	worker.progress.AddTotalBytes(size)

	worker.wg.Go(func() error {
		if err := copyBuffer(w, r, worker.progress); err != nil {
			return worker.errorHandler(err)
		}

		worker.progress.AddTransferredFiles(1)

		return nil
	})
}

func (worker *Worker) CopyN(w RangeWriter, r RangeReader, size int64, threads int) {
	rangeSize := calculateRangeSize(size, threads)

	var wg errgroup.Group

	start := make(chan struct{})

	for offset := int64(0); offset < size; offset += rangeSize {
		rr := r.Range(offset, rangeSize)
		ww := w.Range(offset, rangeSize)

		wg.Go(func() error {
			<-start

			return copyBuffer(ww, rr, worker.progress)
		})
	}

	close(start)

	worker.progress.AddTotalFiles(1)
	worker.progress.AddTotalBytes(size)

	worker.wg.Go(func() error {
		if err := wg.Wait(); err != nil {
			return worker.errorHandler(err)
		}

		worker.progress.AddTransferredFiles(1)

		return nil
	})
}

func (worker *Worker) Wait() error {
	return worker.wg.Wait()
}

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

func copyBuffer(w io.Writer, r io.Reader, progress Progress) error {
	pw := &progressWriter{
		Progress: progress,
	}

	buffer := make([]byte, BufferSize)

	_, err := io.CopyBuffer(io.MultiWriter(w, pw), r, buffer)

	return err
}

type progressWriter struct {
	Progress Progress
}

func (p *progressWriter) Write(b []byte) (int, error) {
	p.Progress.AddTransferredBytes(int64(len(b)))

	return len(b), nil
}
