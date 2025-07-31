package transfer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	// MaxQueued indicates the maximum number of queued files
	// when uploading or downloading a directory
	MaxQueued int
	// VerifyChecksums indicates whether checksums should be verified
	// to compare an existing file when syncing (UploadDir, DownloadDir)
	VerifyChecksums bool
	// Output will, if set, display a progress bar and occurring errors
	// If ErrorHandler or ProgressHandler is set, this option is ignored
	Output io.Writer
	// Progress handler, can be used to track the progress of the transfers
	ProgressHandler func(progress Progress)
	// Error handler, called when an error occurs
	// If this callback is not set or returns an error, the worker will stop and Wait() will return the error
	ErrorHandler func(local, remote string, err error) error
}

type Worker struct {
	IndexPool    *api.API
	TransferPool *api.API

	// Options
	options Options

	// Internal waitgroup
	wg errgroup.Group

	// Hooks for Wait() function
	onwait func()
	closer func() error
}

func New(indexPool, transferPool *api.API, options Options) *Worker {
	var (
		onwait func()
		closer func() error
	)

	if options.Output != nil && options.ProgressHandler == nil && options.ErrorHandler == nil {
		p := ProgressBar(options.Output)

		options.ProgressHandler = p.Handler
		options.ErrorHandler = p.ErrorHandler
		onwait = p.ScanCompleted
		closer = p.Close
	}

	if options.ErrorHandler == nil {
		options.ErrorHandler = func(local, _ string, err error) error {
			return fmt.Errorf("%s: %w", local, err)
		}
	}

	if options.ProgressHandler == nil {
		options.ProgressHandler = func(progress Progress) {
			// Ignore
		}
	}

	if options.MaxThreads <= 0 {
		options.MaxThreads = 1
	}

	return &Worker{
		IndexPool:    indexPool,
		TransferPool: transferPool,
		options:      options,
		onwait:       onwait,
		closer:       closer,
	}
}

type Progress struct {
	Label       string
	Size        int64
	Transferred int64
	Increment   int
	StartedAt   time.Time
	FinishedAt  time.Time
}

type progressWriter struct {
	progress Progress
	handler  func(progress Progress)
	sync.Mutex
}

func (pw *progressWriter) Write(buf []byte) (int, error) {
	pw.Lock()
	defer pw.Unlock()

	if n := len(buf); n > 0 {
		pw.progress.Transferred += int64(n)
		pw.progress.Increment = n

		pw.handler(pw.progress)
	}

	return len(buf), nil
}

func (pw *progressWriter) Close() error {
	pw.Lock()
	defer pw.Unlock()

	pw.progress.FinishedAt = time.Now()
	pw.progress.Increment = 0

	pw.handler(pw.progress)

	return nil
}

// Wait for all transfers to finish
func (worker *Worker) Wait() error {
	if worker.onwait != nil {
		worker.onwait()
	}

	err := worker.wg.Wait()

	if worker.closer != nil {
		err = multierr.Append(err, worker.closer())
	}

	return err
}

// Upload schedules the upload of a local file to the iRODS server using parallel transfers.
// The local file refers to the local file system. The remote file refers to an iRODS path.
// The call blocks until the transfer of all chunks has started.
func (worker *Worker) Upload(ctx context.Context, local, remote string) {
	r, err := os.Open(local)
	if err != nil {
		worker.Error(local, remote, err)

		return
	}

	stat, err := r.Stat()
	if err != nil {
		worker.Error(local, remote, multierr.Append(err, r.Close()))

		return
	}

	worker.FromReader(ctx, &fileReader{
		name: local,
		stat: stat,
		File: r,
	}, remote)
}

type Reader interface {
	Name() string
	Size() int64
	ModTime() time.Time
	io.ReaderAt
	io.Closer
}

type fileReader struct {
	name string
	stat os.FileInfo
	*os.File
}

func (r fileReader) Name() string {
	return r.name
}

func (r fileReader) Size() int64 {
	return r.stat.Size()
}

func (r fileReader) ModTime() time.Time {
	return r.stat.ModTime()
}

// FromReader schedules the upload of a reader to the iRODS server using parallel transfers.
// The remote file refers to an iRODS path.
// The call blocks until the transfer of all chunks has started.
func (worker *Worker) FromReader(ctx context.Context, r Reader, remote string) { //nolint:funlen
	mode := api.O_CREAT | api.O_WRONLY | api.O_TRUNC

	if worker.options.Exclusive {
		mode |= api.O_EXCL
	}

	w, err := worker.TransferPool.OpenDataObject(ctx, remote, mode)
	if code, ok := api.ErrorCode(err); ok && code == msg.HIERARCHY_ERROR {
		if err = worker.IndexPool.RenameDataObject(ctx, remote, remote+".bad"); err == nil {
			w, err = worker.TransferPool.OpenDataObject(ctx, remote, mode|api.O_EXCL)
		}
	}

	if err != nil {
		worker.Error(r.Name(), remote, multierr.Append(err, r.Close()))

		return
	}

	// Schedule the upload
	pw := &progressWriter{
		progress: Progress{
			Label:     r.Name(),
			Size:      r.Size(),
			StartedAt: time.Now(),
		},
		handler: worker.options.ProgressHandler,
	}

	pw.handler(pw.progress)

	rr := &ReaderAtRangeReader{ReaderAt: r}

	ww := &ReopenRangeWriter{
		WriteSeekCloser: w,
		Reopen: func() (WriteSeekCloser, error) {
			return w.Reopen(nil, api.O_WRONLY)
		},
	}

	var wg errgroup.Group

	rangeSize := calculateRangeSize(r.Size(), worker.options.MaxThreads)

	for offset := int64(0); offset < r.Size(); offset += rangeSize {
		dst := ww.Range(offset, rangeSize)
		src := rr.Range(offset, rangeSize)

		wg.Go(func() error {
			return copyBuffer(dst, src, pw)
		})
	}

	worker.wg.Go(func() error {
		defer pw.Close()

		err := wg.Wait()

		err = multierr.Append(err, ww.Close())
		if err == nil && worker.options.SyncModTime {
			err = w.Touch(r.ModTime())
		}

		err = multierr.Append(err, w.Close())

		err = multierr.Append(err, r.Close())
		if err != nil {
			err = multierr.Append(err, worker.IndexPool.DeleteDataObject(ctx, remote, true))

			return worker.options.ErrorHandler(r.Name(), remote, err)
		}

		return nil
	})
}

// Download schedules the download of a remote file from the iRODS server using parallel transfers.
// The local file refers to the local file system. The remote file refers to an iRODS path.
// The call blocks until the transfer of all chunks has started.
func (worker *Worker) Download(ctx context.Context, local, remote string) {
	mode := os.O_CREATE | os.O_WRONLY | os.O_TRUNC

	if worker.options.Exclusive {
		mode |= os.O_EXCL
	}

	w, err := os.OpenFile(local, mode, 0o600)
	if err != nil {
		worker.Error(local, remote, err)

		return
	}

	worker.ToWriter(ctx, &fileWriter{
		name: local,
		File: w,
	}, remote)
}

type Writer interface {
	Name() string
	io.WriterAt
	io.Closer
	Remove() error
	Touch(mtime time.Time) error
}

type fileWriter struct {
	name string
	*os.File
}

func (w fileWriter) Name() string {
	return w.name
}

func (w fileWriter) Remove() error {
	return os.Remove(w.name)
}

func (w fileWriter) Touch(mtime time.Time) error {
	return os.Chtimes(w.name, time.Time{}, mtime)
}

// Download schedules the download of a remote file from the iRODS server using parallel transfers.
// The remote file refers to an iRODS path.
// The call blocks until the transfer of all chunks has started.
func (worker *Worker) ToWriter(ctx context.Context, w Writer, remote string) { //nolint:funlen
	r, err := worker.TransferPool.OpenDataObject(ctx, remote, api.O_RDONLY)
	if err != nil {
		err = multierr.Append(err, w.Close())
		err = multierr.Append(err, w.Remove())

		worker.Error(w.Name(), remote, err)

		return
	}

	size, err := findSize(r)
	if err != nil {
		err = multierr.Append(err, w.Close())
		err = multierr.Append(err, w.Remove())

		worker.Error(w.Name(), remote, err)

		return
	}

	// Schedule the download
	pw := &progressWriter{
		progress: Progress{
			Label:     w.Name(),
			Size:      size,
			StartedAt: time.Now(),
		},
		handler: worker.options.ProgressHandler,
	}

	pw.handler(pw.progress)

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
		dst := ww.Range(offset, rangeSize)
		src := rr.Range(offset, rangeSize)

		wg.Go(func() error {
			return copyBuffer(dst, src, pw)
		})
	}

	worker.wg.Go(func() error {
		defer pw.Close()

		err := wg.Wait()
		err = multierr.Append(err, rr.Close())
		err = multierr.Append(err, r.Close())

		err = multierr.Append(err, w.Close())
		if err != nil {
			err = multierr.Append(err, w.Remove())

			return worker.options.ErrorHandler(w.Name(), remote, err)
		}

		if !worker.options.SyncModTime {
			return nil
		}

		obj, err := worker.IndexPool.GetDataObject(ctx, remote)
		if err != nil {
			return worker.options.ErrorHandler(w.Name(), remote, err)
		}

		err = w.Touch(obj.ModTime())
		if err != nil {
			return worker.options.ErrorHandler(w.Name(), remote, err)
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

type pathRecord struct {
	path   string
	record api.Record
}

type upload struct {
	local, remote string
}

// UploadDir uploads a local directory to the iRODS server using parallel transfers.
// The local file refers to the local file system. The remote file refers to an iRODS path.
// The call blocks until the source directory has been completely scanned.
func (worker *Worker) UploadDir(ctx context.Context, local, remote string) {
	if err := worker.IndexPool.CreateCollectionAll(ctx, remote); err != nil {
		worker.Error(local, remote, err)

		return
	}

	queue := make(chan upload, worker.options.MaxQueued)

	ch := make(chan *pathRecord)

	// Execute the uploads
	worker.wg.Go(func() error {
		for u := range queue {
			worker.Upload(ctx, u.local, u.remote)
		}

		return nil
	})

	// Scan the remote directory
	worker.wg.Go(func() error {
		defer close(ch)

		return worker.IndexPool.Walk(ctx, remote, func(irodsPath string, record api.Record, err error) error {
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
	defer func() {
		for range ch {
			// skip
		}
	}()

	defer close(queue)

	if err := worker.uploadWalk(ctx, local, remote, ch, queue); err != nil {
		worker.wg.Go(func() error {
			return err
		})
	}
}

func (worker *Worker) uploadWalk(ctx context.Context, local, remote string, ch <-chan *pathRecord, queue chan<- upload) error {
	var (
		remoteRecord *pathRecord // Keeps a record of the last remote path. We'll iterate the remote paths simultaneously
		ok           bool
	)

	return filepath.Walk(local, func(path string, info os.FileInfo, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		relpath, relErr := filepath.Rel(local, path)
		if relErr != nil {
			return relErr
		}

		irodsPath := toIrodsPath(remote, relpath)

		if err != nil {
			return worker.options.ErrorHandler(path, irodsPath, err)
		}

		// Iterate until we find the remote path
		for remoteRecord == nil || remoteRecord.path < irodsPath {
			remoteRecord, ok = <-ch
			if !ok {
				break
			}
		}

		if remoteRecord != nil && remoteRecord.path == irodsPath {
			return worker.upload(ctx, path, info, irodsPath, remoteRecord.record, queue)
		}

		return worker.upload(ctx, path, info, irodsPath, nil, queue)
	})
}

func toIrodsPath(base, path string) string {
	if path == "" || path == "." {
		return base
	}

	return base + "/" + strings.Join(strings.Split(path, string(os.PathSeparator)), "/")
}

func (worker *Worker) upload(ctx context.Context, path string, info os.FileInfo, irodsPath string, remoteInfo api.Record, queue chan<- upload) error {
	if info.IsDir() {
		if remoteInfo != nil {
			return nil
		}

		if err := worker.IndexPool.CreateCollection(ctx, irodsPath); err != nil {
			return worker.options.ErrorHandler(path, irodsPath, err)
		}

		return nil
	}

	switch {
	case remoteInfo == nil:
		// file does not exist
	case worker.options.Exclusive:
		return nil // file already exists, don't overwrite
	case remoteInfo.Size() != info.Size():
		// size does not match
	case worker.options.VerifyChecksums:
		err := Verify(ctx, worker.IndexPool, path, irodsPath)
		if err == nil {
			return nil
		} else if !errors.Is(err, ErrChecksumMismatch) {
			return worker.options.ErrorHandler(path, irodsPath, err)
		}
	case remoteInfo.ModTime().Truncate(time.Second).Equal(info.ModTime().Truncate(time.Second)):
		return nil
	default:
	}

	if worker.options.ProgressHandler != nil {
		worker.options.ProgressHandler(Progress{
			Label: path,
			Size:  info.Size(),
		})
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case queue <- upload{path, irodsPath}:
	}

	return nil
}

type download struct {
	local, remote string
}

// DownloadDir downloads a remote directory from the iRODS server using parallel transfers.
// The local file refers to the local file system. The remote file refers to an iRODS path.
// The call blocks until the source directory has been completely scanned.
func (worker *Worker) DownloadDir(ctx context.Context, local, remote string) {
	queue := make(chan download, worker.options.MaxQueued)

	// Execute the downloads
	worker.wg.Go(func() error {
		for d := range queue {
			worker.Download(ctx, d.local, d.remote)
		}

		return nil
	})

	defer close(queue)

	err := worker.IndexPool.Walk(ctx, remote, func(irodsPath string, record api.Record, err error) error {
		path := toLocalPath(local, strings.TrimPrefix(irodsPath, remote))

		if err != nil {
			return worker.options.ErrorHandler(path, irodsPath, err)
		}

		fi, err := os.Stat(path)
		if !os.IsNotExist(err) && err != nil {
			return worker.options.ErrorHandler(path, irodsPath, err)
		}

		return worker.download(ctx, irodsPath, record, path, fi, queue)
	})
	if err != nil {
		worker.wg.Go(func() error {
			return err
		})
	}
}

func toLocalPath(base, path string) string {
	if path == "" {
		return base
	}

	return base + strings.Join(strings.Split(path, "/"), string(os.PathSeparator))
}

func (worker *Worker) download(ctx context.Context, irodsPath string, remoteInfo api.Record, path string, info os.FileInfo, queue chan<- download) error {
	if remoteInfo.IsDir() {
		if info != nil {
			return nil
		}

		if err := os.MkdirAll(path, 0o755); err != nil {
			return worker.options.ErrorHandler(path, irodsPath, err)
		}

		return nil
	}

	switch {
	case info == nil:
	// file does not exist
	case worker.options.Exclusive:
		return nil // file already exists, don't overwrite
	case info.Size() != remoteInfo.Size():
		// size does not match
	case worker.options.VerifyChecksums:
		if err := Verify(ctx, worker.IndexPool, path, irodsPath); err == nil {
			return nil
		} else if !errors.Is(err, ErrChecksumMismatch) {
			return worker.options.ErrorHandler(path, irodsPath, err)
		}
	case remoteInfo.ModTime().Truncate(time.Second).Equal(info.ModTime().Truncate(time.Second)):
		return nil
	}

	if worker.options.ProgressHandler != nil {
		worker.options.ProgressHandler(Progress{
			Label: path,
			Size:  remoteInfo.Size(),
		})
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case queue <- download{path, irodsPath}:
	}

	return nil
}

// Error schedules an error
func (worker *Worker) Error(local, remote string, err error) {
	worker.wg.Go(func() error {
		return worker.options.ErrorHandler(local, remote, err)
	})
}
