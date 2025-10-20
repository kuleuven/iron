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
	// Delete unrecognized files at the target when downloading or uploading a directory
	Delete bool
	// Don't update files in place. If set, a data object will be removed
	// and a new data object will be created, instead of updating the original object
	DisableUpdateInPlace bool
	// SkipTrash indicates whether files should be moved to the trash or not
	SkipTrash bool
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
	Action      Action
	Label       string
	Size        int64
	Transferred int64
	Increment   int64
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
		pw.progress.Increment = int64(n)

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
			Action:    TransferFile,
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
			Action:    TransferFile,
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

// UploadDir uploads a local directory to the iRODS server using parallel transfers.
// The local file refers to the local file system. The remote file refers to an iRODS path.
// The call blocks until the source directory has been completely scanned.
func (worker *Worker) UploadDir(ctx context.Context, local, remote string) {
	if err := worker.IndexPool.CreateCollectionAll(ctx, remote); err != nil {
		worker.Error(local, remote, err)

		return
	}

	queue := make(chan Task, worker.options.MaxQueued)

	// Execute the uploads
	worker.wg.Go(func() error {
		for u := range queue {
			if ctx.Err() != nil {
				continue
			}

			switch u.Action {
			case TransferFile:
				worker.Upload(ctx, u.Path, u.IrodsPath)

			case RemoveFile:
				worker.action(u.Path, u.IrodsPath, RemoveFile, func() error { return worker.TransferPool.DeleteDataObject(ctx, u.IrodsPath, worker.options.SkipTrash) })

			case RemoveDirectory:
				worker.action(u.Path, u.IrodsPath, RemoveDirectory, func() error { return worker.TransferPool.DeleteCollection(ctx, u.IrodsPath, worker.options.SkipTrash) })

			case CreateDirectory:
				worker.action(u.Path, u.IrodsPath, CreateDirectory, func() error { return worker.TransferPool.CreateCollection(ctx, u.IrodsPath) })
			}
		}

		return ctx.Err()
	})

	defer close(queue)

	if err := worker.SynchronizeDir(ctx, local, remote, LocalToRemote, queue, SynchronizeOptions{DisableUpdateInPlace: worker.options.DisableUpdateInPlace}); err != nil {
		worker.wg.Go(func() error {
			return err
		})
	}
}

// DownloadDir downloads a remote directory from the iRODS server using parallel transfers.
// The local file refers to the local file system. The remote file refers to an iRODS path.
// The call blocks until the source directory has been completely scanned.
func (worker *Worker) DownloadDir(ctx context.Context, local, remote string) {
	if err := os.MkdirAll(local, 0o755); err != nil {
		worker.Error(local, remote, err)

		return
	}

	queue := make(chan Task, worker.options.MaxQueued)

	// Execute the uploads
	worker.wg.Go(func() error {
		for u := range queue {
			if ctx.Err() != nil {
				continue
			}

			switch u.Action {
			case TransferFile:
				worker.Download(ctx, u.Path, u.IrodsPath)

			case RemoveFile, RemoveDirectory:
				worker.action(u.Path, u.IrodsPath, u.Action, func() error { return os.Remove(u.Path) })

			case CreateDirectory:
				worker.action(u.Path, u.IrodsPath, CreateDirectory, func() error { return os.Mkdir(u.Path, 0o755) })
			}
		}

		return ctx.Err()
	})

	defer close(queue)

	if err := worker.SynchronizeDir(ctx, local, remote, RemoteToLocal, queue, SynchronizeOptions{DisableUpdateInPlace: worker.options.DisableUpdateInPlace}); err != nil {
		worker.wg.Go(func() error {
			return err
		})
	}
}

type Direction int

const (
	LocalToRemote Direction = iota
	RemoteToLocal
)

type Action int

const (
	CreateDirectory Action = iota
	TransferFile
	RemoveFile
	RemoveDirectory
)

func (a Action) Format(label string) string {
	switch a {
	case CreateDirectory:
		return fmt.Sprintf("\x1B[34m+ %s/\x1B[0m", label)

	case TransferFile:
		return fmt.Sprintf("+ %s", label)

	case RemoveFile:
		return fmt.Sprintf("\x1B[31m- %s\x1B[0m", label)

	case RemoveDirectory:
		return fmt.Sprintf("\x1B[33m- %s/\x1B[0m", label)

	default:
		return label
	}
}

type Task struct {
	Action          Action
	Path, IrodsPath string
	Size            int64
}

type object struct {
	path      string
	irodsPath string
	info      os.FileInfo
}

type SynchronizeOptions struct {
	DisableUpdateInPlace bool
}

// SynchronizeDir schedules tasks to synchronize a local directory to or from a remote
// collection on the iRODS server. All individual actions are appended to the queue.
// The local file refers to the local file system. The remote file refers to an iRODS path.
// The Direction dictates the order of deletes and transfers. The deleteFirst parameter
// indicates whether to delete files first before retransferring, or whether files might
// be updated in place.
// The call blocks until the source directory has been completely scanned.
func (worker *Worker) SynchronizeDir(ctx context.Context, local, remote string, direction Direction, queue chan<- Task, opts SynchronizeOptions) error {
	lch := make(chan *object)
	rch := make(chan *object)

	wg, ctx := errgroup.WithContext(ctx)

	// Walk through the remote directory
	wg.Go(func() error {
		defer close(rch)

		return worker.IndexPool.Walk(ctx, remote, func(irodsPath string, record api.Record, err error) error {
			path := toLocalPath(local, strings.TrimPrefix(irodsPath, remote))

			if err != nil {
				return worker.options.ErrorHandler(path, irodsPath, err)
			}

			rch <- &object{path, irodsPath, record}

			return nil
		}, api.LexographicalOrder, api.NoSkip)
	})

	// Walk through the local directory
	wg.Go(func() error {
		defer close(lch)

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

			lch <- &object{path, irodsPath, info}

			return nil
		})
	})

	// Process the records
	wg.Go(func() error {
		if direction == RemoteToLocal {
			return worker.merge(ctx, rch, lch, queue, mergeOptions{opts, Verify})
		}

		return worker.merge(ctx, lch, rch, queue, mergeOptions{opts, Verify})
	})

	return wg.Wait()
}

// SynchronizeRemoteDir schedules tasks to synchronize a remote collection to another remote
// collection on the iRODS server. All individual actions are appended to the queue.
// The deleteFirst parameter indicates whether to delete files first before retransferring,
// or whether files might be updated in place.
// The call blocks until the source directory has been completely scanned.
func (worker *Worker) SynchronizeRemoteDir(ctx context.Context, remote1, remote2 string, queue chan<- Task, opts SynchronizeOptions) error {
	lch := make(chan *object)
	rch := make(chan *object)

	wg, ctx := errgroup.WithContext(ctx)

	// Walk through the "local" directory
	wg.Go(func() error {
		defer close(lch)

		return worker.IndexPool.Walk(ctx, remote1, func(path string, record api.Record, err error) error {
			irodsPath := remote2 + strings.TrimPrefix(path, remote1)

			if err != nil {
				return worker.options.ErrorHandler(path, irodsPath, err)
			}

			lch <- &object{path, irodsPath, record}

			return nil
		}, api.LexographicalOrder, api.NoSkip)
	})

	// Walk through the "remote" directory
	wg.Go(func() error {
		defer close(rch)

		return worker.IndexPool.Walk(ctx, remote2, func(irodsPath string, record api.Record, err error) error {
			path := remote1 + strings.TrimPrefix(irodsPath, remote2)

			if err != nil {
				return worker.options.ErrorHandler(path, irodsPath, err)
			}

			rch <- &object{path, irodsPath, record}

			return nil
		}, api.LexographicalOrder, api.NoSkip)
	})

	// Process the records
	wg.Go(func() error {
		return worker.merge(ctx, lch, rch, queue, mergeOptions{opts, VerifyRemote})
	})

	return wg.Wait()
}

func toIrodsPath(base, path string) string {
	if path == "" || path == "." {
		return base
	}

	return base + "/" + strings.Join(strings.Split(path, string(os.PathSeparator)), "/")
}

func toLocalPath(base, path string) string {
	if path == "" {
		return base
	}

	return base + strings.Join(strings.Split(path, "/"), string(os.PathSeparator))
}

type checksumVerifyFunction func(ctx context.Context, a *api.API, local, remote string) error

type mergeOptions struct {
	SynchronizeOptions
	ChecksumVerify checksumVerifyFunction
}

func (worker *Worker) merge(ctx context.Context, left, right chan *object, queue chan<- Task, opts mergeOptions) error {
	leftObject, hasLeft := <-left
	rightObject, hasRight := <-right

	for {
		switch {
		case !hasLeft && !hasRight:
			return nil

		case !hasLeft, hasRight && leftObject.irodsPath > rightObject.irodsPath:
			if worker.options.Delete {
				rightObject, hasRight = worker.removeAll(right, rightObject, queue)
			} else {
				rightObject, hasRight = skipAll(right, rightObject)
			}

		case !hasRight, leftObject.irodsPath < rightObject.irodsPath:
			worker.transfer(leftObject, queue)

			leftObject, hasLeft = <-left

		// Now we have the same iRODS path
		case leftObject.irodsPath != rightObject.irodsPath:
			panic(fmt.Errorf("expected same iRODS path: %s != %s", leftObject.irodsPath, rightObject.irodsPath))

		case leftObject.info.IsDir() && rightObject.info.IsDir():
			leftObject, hasLeft = <-left
			rightObject, hasRight = <-right

		case worker.options.Exclusive:
			leftObject, hasLeft = skipAll(left, leftObject)
			rightObject, hasRight = skipAll(right, rightObject)

		case leftObject.info.Mode()&os.ModeType != rightObject.info.Mode()&os.ModeType:
			rightObject, hasRight = worker.removeAll(right, rightObject, queue)

		default:
			if err := worker.compareAndTransfer(ctx, leftObject, rightObject, queue, opts); err != nil {
				return err
			}

			leftObject, hasLeft = <-left
			rightObject, hasRight = <-right
		}
	}
}

func (worker *Worker) compareAndTransfer(ctx context.Context, left, right *object, queue chan<- Task, opts mergeOptions) error {
	if left.info.IsDir() {
		return nil
	}

	switch {
	case !left.info.Mode().IsRegular(), left.info.Size() != right.info.Size():
		// Retransfer

	case worker.options.VerifyChecksums:
		err := opts.ChecksumVerify(ctx, worker.IndexPool, left.path, left.irodsPath)
		if err == nil {
			return nil
		}

		if errors.Is(err, ErrChecksumMismatch) {
			break
		}

		err = worker.options.ErrorHandler(left.path, left.irodsPath, err)
		if err == nil {
			break
		}

		return err
	default:
		if right.info.ModTime().Truncate(time.Second).Equal(left.info.ModTime().Truncate(time.Second)) {
			return nil
		}
	}

	if opts.DisableUpdateInPlace {
		if worker.options.ProgressHandler != nil {
			worker.options.ProgressHandler(Progress{
				Action: RemoveFile,
				Label:  ProgressLabel(left.path, left.irodsPath),
			})
		}

		queue <- Task{
			Action:    RemoveFile,
			Path:      left.path,
			IrodsPath: left.irodsPath,
		}
	}

	worker.transfer(left, queue)

	return nil
}

func (worker *Worker) removeAll(ch <-chan *object, obj *object, queue chan<- Task) (*object, bool) {
	if obj.info.IsDir() {
		next, ok := <-ch

		for ok && strings.HasPrefix(next.irodsPath, obj.irodsPath+"/") {
			next, ok = worker.removeAll(ch, next, queue)
		}

		if worker.options.ProgressHandler != nil {
			worker.options.ProgressHandler(Progress{
				Action: RemoveDirectory,
				Label:  ProgressLabel(obj.path, obj.irodsPath),
			})
		}

		queue <- Task{
			Action:    RemoveDirectory,
			Path:      obj.path,
			IrodsPath: obj.irodsPath,
		}

		return next, ok
	}

	if worker.options.ProgressHandler != nil {
		worker.options.ProgressHandler(Progress{
			Action: RemoveFile,
			Label:  ProgressLabel(obj.path, obj.irodsPath),
		})
	}

	queue <- Task{
		Action:    RemoveFile,
		Path:      obj.path,
		IrodsPath: obj.irodsPath,
	}

	obj, ok := <-ch

	return obj, ok
}

func skipAll(ch <-chan *object, obj *object) (*object, bool) {
	next, ok := <-ch

	if obj.info.IsDir() {
		for ok && strings.HasPrefix(next.irodsPath, obj.irodsPath+"/") {
			next, ok = <-ch
		}
	}

	return next, ok
}

func (worker *Worker) transfer(obj *object, queue chan<- Task) {
	if obj.info.IsDir() {
		if worker.options.ProgressHandler != nil {
			worker.options.ProgressHandler(Progress{
				Action: CreateDirectory,
				Label:  ProgressLabel(obj.path, obj.irodsPath),
			})
		}

		queue <- Task{
			Action:    CreateDirectory,
			Path:      obj.path,
			IrodsPath: obj.irodsPath,
		}

		return
	}

	// Ignore non-regular files
	if !obj.info.Mode().IsRegular() {
		return
	}

	if worker.options.ProgressHandler != nil {
		worker.options.ProgressHandler(Progress{
			Action: TransferFile,
			Label:  ProgressLabel(obj.path, obj.irodsPath),
			Size:   obj.info.Size(),
		})
	}

	queue <- Task{
		Action:    TransferFile,
		Path:      obj.path,
		IrodsPath: obj.irodsPath,
		Size:      obj.info.Size(),
	}
}

// RemoveDir removes a directory from the iRODS server entirely,
// but instead of calling the recursive RemoveCollection API,
// it handles the recursion client side. This allows the caller
// to track the deletion progress better.
// The call blocks until the source directory has been completely scanned.
func (worker *Worker) RemoveDir(ctx context.Context, remote string) {
	queue := make(chan Task, worker.options.MaxQueued)

	// Execute the deletions
	worker.wg.Go(func() error {
		for u := range queue {
			if ctx.Err() != nil {
				continue
			}

			switch u.Action { //nolint:exhaustive
			case RemoveFile:
				worker.action(u.Path, u.IrodsPath, RemoveFile, func() error { return worker.TransferPool.DeleteDataObject(ctx, u.IrodsPath, worker.options.SkipTrash) })

			case RemoveDirectory:
				worker.action(u.Path, u.IrodsPath, RemoveDirectory, func() error { return worker.TransferPool.DeleteCollection(ctx, u.IrodsPath, worker.options.SkipTrash) })
			}
		}

		return ctx.Err()
	})

	ch := make(chan *object)

	// Walk the directory
	worker.wg.Go(func() error {
		defer close(ch)

		return worker.IndexPool.Walk(ctx, remote, func(irodsPath string, record api.Record, err error) error {
			if err != nil {
				return worker.options.ErrorHandler("", irodsPath, err)
			}

			ch <- &object{
				irodsPath: irodsPath,
				info:      record,
			}

			return nil
		}, api.LexographicalOrder, api.NoSkip)
	})

	defer close(queue)

	// Process the objects
	obj, ok := <-ch

	for ok {
		obj, ok = worker.removeAll(ch, obj, queue)
	}
}

// CopyDir copies one directory to another on the iRODS server.
// It handles the recursion client side, but individual files are copied server-side only.
// The call blocks until the source directory has been completely scanned.
func (worker *Worker) CopyDir(ctx context.Context, remote1, remote2 string) {
	if err := worker.IndexPool.CreateCollectionAll(ctx, remote2); err != nil {
		worker.Error(remote1, remote2, err)

		return
	}

	queue := make(chan Task, worker.options.MaxQueued)

	// Execute the uploads
	worker.wg.Go(func() error {
		for u := range queue {
			if ctx.Err() != nil {
				continue
			}

			switch u.Action {
			case TransferFile:
				worker.copy(ctx, u.Path, u.IrodsPath, u.Size)

			case RemoveFile:
				worker.action(u.Path, u.IrodsPath, RemoveFile, func() error { return worker.TransferPool.DeleteDataObject(ctx, u.IrodsPath, worker.options.SkipTrash) })

			case RemoveDirectory:
				worker.action(u.Path, u.IrodsPath, RemoveDirectory, func() error { return worker.TransferPool.DeleteCollection(ctx, u.IrodsPath, worker.options.SkipTrash) })

			case CreateDirectory:
				worker.action(u.Path, u.IrodsPath, CreateDirectory, func() error { return worker.TransferPool.CreateCollection(ctx, u.IrodsPath) })
			}
		}

		return ctx.Err()
	})

	defer close(queue)

	if err := worker.SynchronizeRemoteDir(ctx, remote1, remote2, queue, SynchronizeOptions{DisableUpdateInPlace: true}); err != nil {
		worker.wg.Go(func() error {
			return err
		})
	}
}

func (worker *Worker) copy(ctx context.Context, remote1, remote2 string, size int64) {
	conn, err := worker.TransferPool.Connect(ctx)
	if err != nil {
		worker.Error(remote1, remote2, err)

		return
	}

	startTime := time.Now()

	if worker.options.ProgressHandler != nil {
		worker.options.ProgressHandler(Progress{
			Action:    TransferFile,
			Label:     ProgressLabel(remote1, remote2),
			Size:      size,
			StartedAt: startTime,
		})
	}

	worker.wg.Go(func() error {
		connAPI := *worker.TransferPool
		connAPI.Connect = func(ctx context.Context) (api.Conn, error) { return conn, nil }

		if err := connAPI.CopyDataObject(ctx, remote1, remote2); err != nil {
			return worker.options.ErrorHandler(remote1, remote2, err)
		}

		if worker.options.ProgressHandler != nil {
			worker.options.ProgressHandler(Progress{
				Action:      TransferFile,
				Label:       ProgressLabel(remote1, remote2),
				Size:        size,
				Increment:   size,
				Transferred: size,
				StartedAt:   startTime,
				FinishedAt:  time.Now(),
			})
		}

		return nil
	})
}

// action runs a simple action and schedules an error
func (worker *Worker) action(local, remote string, action Action, callback func() error) {
	startTime := time.Now()

	if worker.options.ProgressHandler != nil {
		worker.options.ProgressHandler(Progress{
			Action:    action,
			Label:     ProgressLabel(local, remote),
			StartedAt: startTime,
		})
	}

	if err := callback(); err != nil {
		worker.Error(local, remote, err)

		return
	}

	if worker.options.ProgressHandler != nil {
		worker.options.ProgressHandler(Progress{
			Action:     action,
			Label:      ProgressLabel(local, remote),
			StartedAt:  startTime,
			FinishedAt: time.Now(),
		})
	}
}

// Error schedules an error
func (worker *Worker) Error(local, remote string, err error) {
	worker.wg.Go(func() error {
		return worker.options.ErrorHandler(local, remote, err)
	})
}
