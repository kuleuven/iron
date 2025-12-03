package transfer

import (
	"errors"
	"io"
	"sync"
	"time"

	"go.uber.org/multierr"
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

	// Prevent using WriteTo here, we ensure that the buffer is used
	r = &ioReader{r}

	_, err := io.CopyBuffer(io.MultiWriter(w, pw), r, buffer)

	return err
}

type ioReader struct {
	reader io.Reader
}

func (r *ioReader) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

type CircularWriter struct {
	WriteSeekCloser WriteSeekCloser
	MaxThreads      int
	Reopen          func() (WriteSeekCloser, error)

	workers         []*writeThread
	offset          int64
	nextWorkerIndex int
}

type writeThread struct {
	writer WriteSeekCloser
	buffer []byte
	offset int64
	length int
	write  chan struct{}
	done   chan error
}

func (w *CircularWriter) Write(data []byte) (int, error) {
	if len(w.workers) < w.MaxThreads {
		writer := w.WriteSeekCloser

		if len(w.workers) > 0 {
			var err error

			writer, err = w.Reopen()
			if err != nil {
				return 0, err
			}
		}

		thread := &writeThread{
			writer: writer,
			buffer: make([]byte, BufferSize),
			write:  make(chan struct{}),
			done:   make(chan error, 1),
		}

		w.workers = append(w.workers, thread)

		go thread.Run()
	}

	worker := w.workers[w.nextWorkerIndex]

	// Wait for worker to be ready
	if err := <-worker.done; err != nil {
		return 0, err
	}

	if len(worker.buffer) < len(data) {
		worker.buffer = make([]byte, len(data))
	}

	copy(worker.buffer, data)
	worker.offset = w.offset
	worker.length = len(data)

	worker.write <- struct{}{}

	w.nextWorkerIndex = (w.nextWorkerIndex + 1) % w.MaxThreads
	w.offset += int64(len(data))

	return len(data), nil
}

func (wt *writeThread) Run() {
	defer close(wt.done)

	if CopyBufferDelay > 0 {
		time.Sleep(CopyBufferDelay)
	}

	// We start ready
	wt.done <- nil

	for range wt.write {
		_, err := wt.writer.Seek(wt.offset, io.SeekStart)
		if err != nil {
			wt.done <- err

			continue
		}

		_, err = wt.writer.Write(wt.buffer[:wt.length])

		wt.done <- err
	}

	wt.done <- wt.writer.Close()
}

func (w *CircularWriter) Close() error {
	allErrors := make(chan error, 2*len(w.workers))

	var wg sync.WaitGroup

	for _, worker := range w.workers {
		close(worker.write)

		wg.Go(func() {
			for e := range worker.done {
				allErrors <- e
			}
		})
	}

	wg.Wait()

	close(allErrors)

	var err error

	for e := range allErrors {
		err = multierr.Append(err, e)
	}

	return err
}

type CircularReader struct {
	ReadSeekCloser io.ReadSeekCloser
	MaxThreads     int
	Reopen         func() (io.ReadSeekCloser, error)
	Size           int64

	workers         []*readThread
	offset          int64
	consumed        int64
	nextWorkerIndex int
}

type readThread struct {
	reader io.ReadSeekCloser
	buffer []byte
	offset int64
	length int
	read   chan struct{}
	done   chan error
}

func (r *CircularReader) Read(data []byte) (int, error) {
	for len(r.workers) < r.MaxThreads && r.offset < r.Size {
		reader := r.ReadSeekCloser

		if len(r.workers) > 0 {
			var err error

			reader, err = r.Reopen()
			if err != nil {
				return 0, err
			}
		}

		thread := &readThread{
			reader: reader,
			buffer: make([]byte, BufferSize),
			offset: r.offset,
			read:   make(chan struct{}, 1),
			done:   make(chan error, 1),
		}

		go thread.Run()

		r.offset += int64(len(thread.buffer))
		r.workers = append(r.workers, thread)
	}

	worker := r.workers[r.nextWorkerIndex]

	// Wait for worker to be ready
	if err := <-worker.done; err != nil {
		return 0, err
	}

	start := int(r.consumed - worker.offset)

	n := copy(data, worker.buffer[start:worker.length])

	r.consumed += int64(n)

	if n < worker.length-start {
		// Need to read more from the same buffer
		worker.done <- nil

		return n, nil
	}

	if r.consumed >= r.Size {
		return n, io.EOF
	}

	if r.offset < r.Size {
		// Prefetch next batch
		worker.offset = r.offset
		worker.read <- struct{}{}

		r.offset += int64(len(worker.buffer))
	}

	r.nextWorkerIndex = (r.nextWorkerIndex + 1) % r.MaxThreads

	return n, nil
}

func (rt *readThread) Run() {
	defer close(rt.done)

	if CopyBufferDelay > 0 {
		time.Sleep(CopyBufferDelay)
	}

	// Start with a read
	rt.doRead()

	for range rt.read {
		rt.doRead()
	}

	rt.done <- rt.reader.Close()
}

// read reads a single chunk from the reader, and signals when the end of the file is reached.
// If an error occurs, it is signaled through the done channel.
func (rt *readThread) doRead() {
	rt.length = 0

	_, err := rt.reader.Seek(rt.offset, io.SeekStart)
	if errors.Is(err, io.EOF) {
		rt.done <- nil

		return
	} else if err != nil {
		rt.done <- err

		return
	}

	rt.length, err = rt.reader.Read(rt.buffer)
	if errors.Is(err, io.EOF) {
		err = nil
	}

	rt.done <- err
}

func (r *CircularReader) Close() error {
	allErrors := make(chan error, 2*len(r.workers))

	var wg sync.WaitGroup

	for _, worker := range r.workers {
		close(worker.read)

		wg.Go(func() {
			for e := range worker.done {
				allErrors <- e
			}
		})
	}

	wg.Wait()

	close(allErrors)

	var err error

	for e := range allErrors {
		err = multierr.Append(err, e)
	}

	return err
}
