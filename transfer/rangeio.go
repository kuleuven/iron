package transfer

import (
	"io"
	"sync"

	"golang.org/x/sync/errgroup"
)

type RangeReader interface {
	Range(offset, length int64) io.Reader
}

type ReaderAtRangeReader struct {
	io.ReaderAt
}

func (r *ReaderAtRangeReader) Range(offset, length int64) io.Reader {
	return io.NewSectionReader(r, offset, length)
}

type ReopenRangeReader struct {
	io.ReadSeekCloser
	Reopen func() (io.ReadSeekCloser, error)

	// Unexported fields
	needsClose []io.Closer
	sync.Mutex
}

func (r *ReopenRangeReader) Range(offset, length int64) io.Reader {
	r.Lock()
	defer r.Unlock()

	var f io.ReadSeekCloser

	if r.ReadSeekCloser != nil {
		f = r.ReadSeekCloser
		r.ReadSeekCloser = nil
	} else {
		var err error

		f, err = r.Reopen()
		if err != nil {
			return errorReader{err}
		}

		r.needsClose = append(r.needsClose, f)
	}

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return errorReader{err}
	}

	return io.LimitReader(f, length)
}

func (r *ReopenRangeReader) Close() error {
	var wg errgroup.Group

	for _, c := range r.needsClose {
		wg.Go(func() error {
			return c.Close()
		})
	}

	return wg.Wait()
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}

type RangeWriter interface {
	Range(offset, length int64) io.Writer
}

type WriterAtRangeWriter struct {
	io.WriterAt
}

func (r *WriterAtRangeWriter) Range(offset, length int64) io.Writer {
	return &sectionWriter{r.WriterAt, offset, length}
}

type sectionWriter struct {
	io.WriterAt
	off int64
	len int64
}

func (w *sectionWriter) Write(b []byte) (int, error) {
	if w.len == 0 {
		return 0, io.EOF
	}

	var defErr error

	if len(b) > int(w.len) {
		b = b[:w.len]

		defErr = io.ErrShortWrite
	}

	n, err := w.WriteAt(b, w.off)
	if err == nil {
		err = defErr
	}

	w.off += int64(n)
	w.len -= int64(n)

	return n, err
}

type ReopenRangeWriter struct {
	WriteSeekCloser
	Reopen func() (WriteSeekCloser, error)

	// Unexported fields
	needsClose []io.Closer
	sync.Mutex
}

type WriteSeekCloser interface {
	io.WriteSeeker
	io.Closer
}

func (r *ReopenRangeWriter) Range(offset, length int64) io.Writer {
	r.Lock()
	defer r.Unlock()

	var f WriteSeekCloser

	if r.WriteSeekCloser != nil {
		f = r.WriteSeekCloser
		r.WriteSeekCloser = nil
	} else {
		var err error

		f, err = r.Reopen()
		if err != nil {
			return errorWriter{err}
		}

		r.needsClose = append(r.needsClose, f)
	}

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return errorWriter{err}
	}

	return &limitWriter{f, length}
}

func (r *ReopenRangeWriter) Close() error {
	var wg errgroup.Group

	for _, c := range r.needsClose {
		wg.Go(func() error {
			return c.Close()
		})
	}

	return wg.Wait()
}

type limitWriter struct {
	io.Writer
	limit int64
}

func (w *limitWriter) Write(b []byte) (int, error) {
	if w.limit == 0 {
		return 0, io.EOF
	}

	var defErr error

	if len(b) > int(w.limit) {
		b = b[:w.limit]

		defErr = io.ErrShortWrite
	}

	n, err := w.Writer.Write(b)
	if err == nil {
		err = defErr
	}

	w.limit -= int64(n)

	return n, err
}

type errorWriter struct {
	err error
}

func (r errorWriter) Write([]byte) (int, error) {
	return 0, r.err
}
