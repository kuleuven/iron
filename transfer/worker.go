package transfer

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/kuleuven/iron/api"
	"github.com/kuleuven/iron/msg"
	"go.uber.org/multierr"
	"golang.org/x/sync/errgroup"
)

type Options struct {
	// Do not overwrite existing files
	Exclusive bool
	// Sync modification time
	SyncModTime bool
	// MaxThreads indicates the maximum threads per transferred file
	MaxThreads int
	// Error handler
	ErrorHandler func(err error) error
}

type Worker struct {
	IndexPool    *api.API
	TransferPool *api.API

	// Options
	options Options

	// Internal waitgroup
	wg errgroup.Group
}

func New(indexPool, transferPool *api.API, options Options) *Worker {
	if options.ErrorHandler == nil {
		options.ErrorHandler = func(err error) error {
			return err
		}
	}

	if options.MaxThreads <= 0 {
		options.MaxThreads = 1
	}

	return &Worker{
		IndexPool:    indexPool,
		TransferPool: transferPool,
		options:      options,
	}
}

type progress struct {
	Label       string
	Size        int64
	Transferred int64
	StartedAt   time.Time
	FinishedAt  time.Time
	sync.Mutex
}

func (p *progress) Write(buf []byte) (int, error) {
	p.Lock()
	defer p.Unlock()

	p.Transferred += int64(len(buf))

	return len(buf), nil
}

func (p *progress) Close() error {
	p.Lock()
	defer p.Unlock()

	p.FinishedAt = time.Now()

	return nil
}

// Copy a generic io.Reader to an io.Writer
func (worker *Worker) Copy(w io.Writer, r io.Reader, size int64, callback func(err error) error) {
	if callback == nil {
		callback = func(err error) error {
			return err
		}
	}

	pw := &progress{
		Label:     "",
		Size:      size,
		StartedAt: time.Now(),
	}

	worker.wg.Go(func() error {
		defer pw.Close()

		return callback(copyBuffer(w, r, pw))
	})
}

// CopyN copies a generic io.Reader to an io.Writer using multiple threads
func (worker *Worker) CopyN(w RangeWriter, r RangeReader, size int64, threads int) {
	pw := &progress{
		Label:     "",
		Size:      size,
		StartedAt: time.Now(),
	}

	rangeSize := calculateRangeSize(size, threads)

	var wg errgroup.Group

	for offset := int64(0); offset < size; offset += rangeSize {
		wg.Go(func() error {
			rr := r.Range(offset, rangeSize)
			ww := w.Range(offset, rangeSize)

			return copyBuffer(ww, rr, pw)
		})
	}

	worker.wg.Go(func() error {
		defer pw.Close()

		return wg.Wait()
	})
}

// Wait for all transfers to finish
func (worker *Worker) Wait() error {
	return worker.wg.Wait()
}

// Upload schedules the upload of a local file to the iRODS server using parallel transfers.
// The local file refers to the local file system. The remote file refers to an iRODS path.
func (worker *Worker) Upload(ctx context.Context, local, remote string) { //nolint:funlen
	mode := api.O_CREAT | api.O_WRONLY | api.O_TRUNC

	if worker.options.Exclusive {
		mode |= api.O_EXCL
	}

	r, err := os.Open(local)
	if err != nil {
		worker.Error(err)

		return
	}

	stat, err := r.Stat()
	if err != nil {
		worker.Error(multierr.Append(err, r.Close()))

		return
	}

	w, err := worker.TransferPool.OpenDataObject(ctx, remote, mode)
	if code, ok := api.ErrorCode(err); ok && code == msg.HIERARCHY_ERROR {
		if err = worker.IndexPool.RenameDataObject(ctx, remote, remote+".bad"); err == nil {
			w, err = worker.TransferPool.OpenDataObject(ctx, remote, mode|api.O_EXCL)
		}
	}

	if err != nil {
		worker.Error(multierr.Append(err, r.Close()))

		return
	}

	// Schedule the upload
	pw := &progress{
		Label:     local,
		Size:      stat.Size(),
		StartedAt: time.Now(),
	}

	rr := &ReaderAtRangeReader{ReaderAt: r}

	ww := &ReopenRangeWriter{
		WriteSeekCloser: w,
		Reopen: func() (WriteSeekCloser, error) {
			return w.Reopen(nil, api.O_WRONLY)
		},
	}

	var wg errgroup.Group

	rangeSize := calculateRangeSize(stat.Size(), worker.options.MaxThreads)

	for offset := int64(0); offset < stat.Size(); offset += rangeSize {
		wg.Go(func() error {
			return copyBuffer(ww.Range(offset, rangeSize), rr.Range(offset, rangeSize), pw)
		})
	}

	worker.wg.Go(func() error {
		defer r.Close()
		defer pw.Close()

		err := wg.Wait()
		err = multierr.Append(err, ww.Close())
		if err == nil && worker.options.SyncModTime {
			err = w.Touch(stat.ModTime())
		}

		err = multierr.Append(err, w.Close())
		if err != nil {
			fmt.Print(err)
			err = multierr.Append(err, worker.IndexPool.DeleteDataObject(ctx, remote, true))

			return worker.options.ErrorHandler(err)
		}

		return nil
	})
}

// Download schedules the download of a remote file from the iRODS server using parallel transfers.
// The local file refers to the local file system. The remote file refers to an iRODS path.
func (worker *Worker) Download(ctx context.Context, local, remote string) { //nolint:funlen
	mode := os.O_CREATE | os.O_WRONLY | os.O_TRUNC

	if worker.options.Exclusive {
		mode |= os.O_EXCL
	}

	r, err := worker.TransferPool.OpenDataObject(ctx, remote, api.O_RDONLY)
	if err != nil {
		worker.Error(err)

		return
	}

	size, err := findSize(r)
	if err != nil {
		worker.Error(multierr.Append(err, r.Close()))

		return
	}

	w, err := os.OpenFile(local, mode, 0o600)
	if err != nil {
		worker.Error(multierr.Append(err, r.Close()))

		return
	}

	// Schedule the download
	pw := &progress{
		Label:     local,
		Size:      size,
		StartedAt: time.Now(),
	}

	ww := &WriterAtRangeWriter{WriterAt: w}

	rr := &ReopenRangeReader{
		ReadSeekCloser: r,
		Reopen: func() (io.ReadSeekCloser, error) {
			return r.Reopen(nil, api.O_RDONLY)
		},
	}

	var wg errgroup.Group

	rangeSize := calculateRangeSize(size, worker.options.MaxThreads)

	for offset := int64(0); offset < size; offset += rangeSize {
		wg.Go(func() error {
			return copyBuffer(ww.Range(offset, rangeSize), rr.Range(offset, rangeSize), pw)
		})
	}

	worker.wg.Go(func() error {
		defer w.Close()
		defer pw.Close()

		err := wg.Wait()
		err = multierr.Append(err, rr.Close())
		err = multierr.Append(err, r.Close())
		if err != nil {
			err = multierr.Append(err, os.Remove(local))

			return worker.options.ErrorHandler(err)
		}

		if !worker.options.SyncModTime {
			return nil
		}

		obj, err := worker.IndexPool.GetDataObject(ctx, remote)
		if err != nil {
			return worker.options.ErrorHandler(err)
		}

		err = os.Chtimes(local, time.Time{}, obj.ModTime())
		if err != nil {
			return worker.options.ErrorHandler(err)
		}

		return nil
	})
}

func findSize(r io.Seeker) (int64, error) {
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}

	_, err = r.Seek(0, io.SeekStart)
	if err != nil {
		return 0, err
	}

	return size, nil
}

/*
// UploadDir uploads a local directory to the iRODS server using parallel transfers.
// The local file refers to the local file system. The remote file refers to an iRODS path.
func (c *Client) UploadDir(ctx context.Context, local, remote string, opts Options) error {
	if opts.Threads+1 > c.defaultPool.maxConns {
		return fmt.Errorf("%w: need at least %d connections, %d available", ErrNoConnectionsAvailable, opts.Threads+1, c.defaultPool.maxConns)
	}

	// Use a dedicated pool for the actual uploads
	pool, err := c.defaultPool.Pool(opts.Threads)
	if err != nil {
		return err
	}

	worker := transfer.New(opts.Progress)

	wg, ctx := errgroup.WithContext(ctx)

	type pathRecord struct {
		path   string
		record api.Record
	}

	ch := make(chan *pathRecord)

	// Scan the remote directory
	wg.Go(func() error {
		defer close(ch)

		return c.Walk(ctx, remote, func(irodsPath string, record api.Record, err error) error {
			if err != nil {
				return err
			}

			ch <- &pathRecord{
				path:   irodsPath,
				record: record,
			}

			return nil
		}, api.LexographicalOrder, api.NoSkip)
	})

	// Walk through the local directory
	wg.Go(func() error {
		var (
			remoteRecord *pathRecord // Keeps a record of the last remote path. We'll iterate the remote paths simultaneously
			ok           bool
		)

		defer func() {
			for range ch {
				// skip
			}
		}()

		return filepath.Walk(local, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			relpath, err := filepath.Rel(local, path)
			if err != nil {
				return err
			}

			irodsPath := toIrodsPath(remote, relpath)

			// Iterate until we find the remote path
			for remoteRecord == nil || remoteRecord.path < irodsPath {
				remoteRecord, ok = <-ch
				if !ok {
					break
				}
			}

			if info.IsDir() {
				if remoteRecord != nil && remoteRecord.path == irodsPath {
					return nil
				}

				return c.CreateCollection(ctx, irodsPath)
			}

			switch {
			case remoteRecord == nil || remoteRecord.path > irodsPath:
				// file does not exist
			case remoteRecord.record.Size() != info.Size():
				// size does not match
			case checksum:
			if err = c.Verify(ctx, path, irodsPath); !errors.Is(err, ErrChecksumMismatch) {
				return err
			}
			case remoteRecord.record.ModTime().Truncate(time.Second).Equal(info.ModTime().Truncate(time.Second)):
				return nil
			default:
			}

			if err = pool.Upload(ctx, path, irodsPath, remoteRecord == nil || remoteRecord.path > irodsPath, true, worker); err != nil {
				return err
			}

			return nil
		})
	})

	wg.Go(func() error {
		return worker.Wait()
	})

	return wg.Wait()
}

func toIrodsPath(base, path string) string {
	if path == "" || path == "." {
		return base
	}

	return base + "/" + strings.Join(strings.Split(path, string(os.PathSeparator)), "/")
}*/

/*
func toLocalPath(base, path string) string {
	if path == "" {
		return base
	}

	return base + strings.Join(strings.Split(path, "/"), string(os.PathSeparator))
}*/

// Error schedules an error
func (worker *Worker) Error(err error) {
	worker.wg.Go(func() error {
		return worker.options.ErrorHandler(err)
	})
}
