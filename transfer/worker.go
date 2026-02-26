package transfer

import (
	"bytes"
	"context"
	"encoding/base64"
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
	// OnlyIfNewer indicates whether files should only be transferred
	// when syncing (UploadDir, DownloadDir, CopyDir) if the source file is newer than the destination file
	OnlyIfNewer bool
	// CompareChecksums indicates whether checksums should be verified
	// to compare an existing file when syncing (UploadDir, DownloadDir, CopyDir)
	CompareChecksums bool
	// IntegrityChecksums indicates whether checksums should be computed before
	// and after the transfer to verify the integrity of the transfer
	IntegrityChecksums bool
	// DryRun will only print actions for directory operations (UploadDir, DownloadDir, RemoveDir, CopyDir)
	// It will not apply to file operations (Upload, Download, ToStream, FromStream)!
	DryRun bool
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

type ChecksumReader interface {
	Reader
	Checksum(ctx context.Context) ([]byte, error)
}

type fileReader struct {
	name string
	stat os.FileInfo
	*os.File
	checksum []byte
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

func (r fileReader) Checksum(ctx context.Context) ([]byte, error) {
	if len(r.checksum) > 0 {
		return r.checksum, nil
	}

	return Sha256Checksum(ctx, r.name)
}

// FromReader schedules the upload of a reader to the iRODS server using parallel transfers.
// The remote file refers to an iRODS path.
// The call blocks until the transfer of all chunks has started.
func (worker *Worker) FromReader(ctx context.Context, r Reader, remote string) { //nolint:funlen
	mode := api.O_CREAT | api.O_WRONLY | api.O_TRUNC

	if worker.options.Exclusive {
		mode |= api.O_EXCL
	}

	w, err := worker.tryOpenDataObject(ctx, remote, mode)
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

		if cr, ok := r.(ChecksumReader); ok {
			err = multierr.Append(err, worker.verifyChecksumAndClose(ctx, cr.Checksum, w))
		} else {
			err = multierr.Append(err, w.Close())
		}

		err = multierr.Append(err, r.Close())
		if err != nil {
			err = multierr.Append(err, worker.IndexPool.DeleteDataObject(ctx, remote, true))

			return worker.options.ErrorHandler(r.Name(), remote, err)
		}

		return nil
	})
}

func (worker *Worker) verifyChecksumAndClose(ctx context.Context, callback func(ctx context.Context) ([]byte, error), remote api.File) error {
	conn, err := remote.CloseReturnConnection()
	if err != nil {
		return multierr.Append(err, conn.Close())
	}

	if !worker.options.IntegrityChecksums {
		return conn.Close()
	}

	request := msg.DataObjectRequest{
		Path: remote.Name(),
	}

	var irodsChecksum msg.String

	err = conn.Request(ctx, msg.DATA_OBJ_CHKSUM_AN, request, &irodsChecksum)

	err = multierr.Append(err, conn.Close())
	if err != nil {
		return err
	}

	remoteChecksum, err := api.ParseIrodsChecksum(irodsChecksum.String)
	if err != nil {
		return err
	}

	localChecksum, err := callback(ctx)
	if err != nil {
		return err
	}

	if !bytes.Equal(localChecksum, remoteChecksum) {
		return fmt.Errorf("%w: local: %s remote: %s", ErrChecksumMismatch, base64.StdEncoding.EncodeToString(localChecksum), base64.StdEncoding.EncodeToString(remoteChecksum))
	}

	return nil
}

func (worker *Worker) tryOpenDataObject(ctx context.Context, remote string, mode int) (api.File, error) {
	w, err := worker.TransferPool.OpenDataObject(ctx, remote, mode)
	if err == nil {
		return w, nil
	}

	code, ok := api.ErrorCode(err)
	if !ok {
		return nil, err
	}

	if code == msg.HIERARCHY_ERROR {
		if err2 := worker.IndexPool.RenameDataObject(ctx, remote, remote+".bad"); err2 == nil {
			return worker.TransferPool.OpenDataObject(ctx, remote, mode|api.O_EXCL)
		}
	}

	if code == -510017 && mode&api.O_EXCL != 0 {
		return nil, fmt.Errorf("cannot upload exclusively: %s", os.ErrExist)
	}

	return nil, err
}

// FromStream schedules the upload of a io.Reader to the iRODS server using parallel transfers.
// In contrast to FromReader, FromStream will block until the full file has been uploaded.
// The remote file refers to an iRODS path.
func (worker *Worker) FromStream(ctx context.Context, name string, r io.Reader, remote string, appendToFile bool) {
	mode := api.O_CREAT | api.O_WRONLY | api.O_TRUNC

	if appendToFile {
		mode = api.O_CREAT | api.O_WRONLY | api.O_APPEND
	}

	if worker.options.Exclusive {
		mode |= api.O_EXCL
	}

	w, err := worker.tryOpenDataObject(ctx, remote, mode)
	if err != nil {
		worker.Error(name, remote, err)

		return
	}

	// Schedule the upload
	pw := &progressWriter{
		progress: Progress{
			Action:    TransferFile,
			Label:     name,
			Size:      0, // Size is unknown
			StartedAt: time.Now(),
		},
		handler: worker.options.ProgressHandler,
	}

	ww := &CircularWriter{
		WriteSeekCloser: w,
		MaxThreads:      worker.options.MaxThreads,
		Reopen: func() (WriteSeekCloser, error) {
			return w.Reopen(nil, api.O_WRONLY)
		},
	}

	err = multierr.Append(copyBuffer(ww, r, pw), ww.Close())
	if err != nil {
		err = multierr.Append(err, worker.IndexPool.DeleteDataObject(ctx, remote, true))

		worker.Error(name, remote, err)
	}
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
	if errors.Is(err, os.ErrExist) && worker.options.Exclusive {
		worker.Error(local, remote, fmt.Errorf("cannot download exclusively: %w", os.ErrExist))

		return
	} else if err != nil {
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

type ChecksumWriter interface {
	Writer
	Checksum(ctx context.Context) ([]byte, error)
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

func (w fileWriter) Checksum(ctx context.Context) ([]byte, error) {
	return Sha256Checksum(ctx, w.name)
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
		err = multierr.Append(err, w.Close())

		if cw, ok := w.(ChecksumWriter); ok {
			err = multierr.Append(err, worker.verifyChecksumAndClose(ctx, cw.Checksum, r))
		} else {
			err = multierr.Append(err, r.Close())
		}

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

// ToStream schedules the download into a io.Writer from the iRODS server.
// The remote file refers to an iRODS path.
// The call blocks until the transfer has started.
func (worker *Worker) ToStream(ctx context.Context, name string, w io.Writer, remote string) {
	r, err := worker.TransferPool.OpenDataObject(ctx, remote, api.O_RDONLY)
	if err != nil {
		worker.Error(name, remote, err)

		return
	}

	size, err := findSize(r)
	if err != nil {
		worker.Error(name, remote, err)

		return
	}

	// Schedule the download
	pw := &progressWriter{
		progress: Progress{
			Action:    TransferFile,
			Label:     name,
			Size:      size,
			StartedAt: time.Now(),
		},
		handler: worker.options.ProgressHandler,
	}

	pw.handler(pw.progress)

	rr := &CircularReader{
		ReadSeekCloser: r,
		MaxThreads:     worker.options.MaxThreads,
		Reopen: func() (io.ReadSeekCloser, error) {
			return r.Reopen(nil, api.O_RDONLY)
		},
		Size: size,
	}

	worker.wg.Go(func() error {
		err = multierr.Append(copyBuffer(w, rr, pw), rr.Close())
		if err != nil {
			return worker.options.ErrorHandler(name, remote, err)
		}

		return nil
	})
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
	worker.wg.Go(func() error { //nolint:dupl
		for u := range queue {
			if ctx.Err() != nil {
				continue
			}

			switch u.Action {
			case ComputeChecksum: // This is a special case for the progress handler, it doesn't correspond to an actual task

			case SetModificationTime:
				worker.action(u, func() error { return worker.TransferPool.ModifyModificationTime(ctx, u.IrodsPath, u.ModTime) })

			case TransferFile:
				worker.uploadAction(ctx, u)

			case RemoveFile:
				worker.action(u, func() error { return worker.TransferPool.DeleteDataObject(ctx, u.IrodsPath, worker.options.SkipTrash) })

			case RemoveDirectory:
				worker.action(u, func() error { return worker.TransferPool.DeleteCollection(ctx, u.IrodsPath, worker.options.SkipTrash) })

			case CreateDirectory:
				worker.action(u, func() error { return worker.TransferPool.CreateCollection(ctx, u.IrodsPath) })
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

func (worker *Worker) uploadAction(ctx context.Context, u Task) {
	if worker.options.DryRun {
		worker.log(u)

		return
	}

	r, err := os.Open(u.Path)
	if err != nil {
		worker.Error(u.Path, u.IrodsPath, err)

		return
	}

	worker.FromReader(ctx, &taskReader{
		task: u,
		File: r,
	}, u.IrodsPath)
}

type taskReader struct {
	task Task
	*os.File
}

func (tr *taskReader) Name() string {
	return tr.task.Path
}

func (tr *taskReader) Size() int64 {
	return tr.task.Size
}

func (tr *taskReader) ModTime() time.Time {
	return tr.task.ModTime
}

func (tr *taskReader) Checksum(ctx context.Context) ([]byte, error) {
	if len(tr.task.Checksum) > 0 {
		return tr.task.Checksum, nil
	}

	return Sha256Checksum(ctx, tr.task.Path)
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
			case ComputeChecksum: // This is a special case for the progress handler, it doesn't correspond to an actual task

			case SetModificationTime:
				worker.action(u, func() error { return os.Chtimes(u.Path, time.Time{}, u.ModTime) })

			case TransferFile:
				worker.downloadAction(ctx, u)

			case RemoveFile, RemoveDirectory:
				worker.action(u, func() error { return os.Remove(u.Path) })

			case CreateDirectory:
				worker.action(u, func() error { return os.Mkdir(u.Path, 0o755) })
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

func (worker *Worker) downloadAction(ctx context.Context, u Task) {
	if worker.options.DryRun {
		worker.log(u)

		return
	}

	worker.Download(ctx, u.Path, u.IrodsPath)
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
	ComputeChecksum
	SetModificationTime
)

func (a Action) Format(label string) string {
	switch a {
	case ComputeChecksum:
		return fmt.Sprintf("\x1B[36mc %s\x1B[0m", label)

	case SetModificationTime:
		return fmt.Sprintf("\x1B[35mt %s\x1B[0m", label)

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
	Checksum        []byte
	ModTime         time.Time
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
			return worker.merge(ctx, rch, lch, queue, mergeOptions{opts, VerifyRemoteToLocal(worker.IndexPool, worker.options.ProgressHandler)})
		}

		return worker.merge(ctx, lch, rch, queue, mergeOptions{opts, VerifyLocalToRemote(worker.IndexPool, worker.options.ProgressHandler)})
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
		return worker.merge(ctx, lch, rch, queue, mergeOptions{opts, VerifyRemoteToRemote(worker.IndexPool, worker.options.ProgressHandler)})
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

// Verify checks the checksums of the local and remote file and returns nil if they match,
// ErrChecksumMismatch if they don't match, or an error if the verification failed.
// The checksum of the source file is returned
type checksumVerifyFunction func(ctx context.Context, source, target string, sourceInfo, targetInfo os.FileInfo) ([]byte, []byte, error)

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

		case !hasLeft, hasRight && api.ComparePaths(leftObject.irodsPath, rightObject.irodsPath) > 0:
			if worker.options.Delete {
				rightObject, hasRight = worker.removeAll(right, rightObject, queue)
			} else {
				rightObject, hasRight = skipAll(right, rightObject)
			}

		case !hasRight, api.ComparePaths(leftObject.irodsPath, rightObject.irodsPath) < 0:
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

func (worker *Worker) compareAndTransfer(ctx context.Context, left, right *object, queue chan<- Task, opts mergeOptions) error { //nolint:funlen
	var checksum []byte

	modTimeCompare := left.info.ModTime().Truncate(time.Second).Compare(right.info.ModTime().Truncate(time.Second))

	switch {
	case left.info.IsDir(), !left.info.Mode().IsRegular():
		return nil

	case worker.options.OnlyIfNewer && modTimeCompare < 0:
		return nil

	case left.info.Size() != right.info.Size():
		// Retransfer

	case worker.options.CompareChecksums:
		var err error

		checksum, _, err = opts.ChecksumVerify(ctx, left.path, left.irodsPath, left.info, right.info)
		if err == nil {
			if !worker.options.SyncModTime || modTimeCompare == 0 {
				return nil
			}

			// Still set metadata in sync
			queue <- Task{
				Action:    SetModificationTime,
				Path:      left.path,
				IrodsPath: left.irodsPath,
				ModTime:   left.info.ModTime(),
			}

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

	case modTimeCompare == 0:
		return nil
	}

	if opts.DisableUpdateInPlace {
		worker.Progress(Progress{
			Action: RemoveFile,
			Label:  ProgressLabel(left.path, left.irodsPath),
		})

		queue <- Task{
			Action:    RemoveFile,
			Path:      left.path,
			IrodsPath: left.irodsPath,
		}
	}

	worker.Progress(Progress{
		Action: TransferFile,
		Label:  ProgressLabel(left.path, left.irodsPath),
		Size:   left.info.Size(),
	})

	queue <- Task{
		Action:    TransferFile,
		Path:      left.path,
		IrodsPath: left.irodsPath,
		Size:      left.info.Size(),
		ModTime:   left.info.ModTime(),
		Checksum:  checksum,
	}

	return nil
}

func (worker *Worker) removeAll(ch <-chan *object, obj *object, queue chan<- Task) (*object, bool) {
	if obj.info.IsDir() {
		next, ok := <-ch

		for ok && strings.HasPrefix(next.irodsPath, obj.irodsPath+"/") {
			next, ok = worker.removeAll(ch, next, queue)
		}

		worker.Progress(Progress{
			Action: RemoveDirectory,
			Label:  ProgressLabel(obj.path, obj.irodsPath),
		})

		queue <- Task{
			Action:    RemoveDirectory,
			Path:      obj.path,
			IrodsPath: obj.irodsPath,
		}

		return next, ok
	}

	worker.Progress(Progress{
		Action: RemoveFile,
		Label:  ProgressLabel(obj.path, obj.irodsPath),
	})

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
		worker.Progress(Progress{
			Action: CreateDirectory,
			Label:  ProgressLabel(obj.path, obj.irodsPath),
		})

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

	// Ignore files from ignore globs

	worker.Progress(Progress{
		Action: TransferFile,
		Label:  ProgressLabel(obj.path, obj.irodsPath),
		Size:   obj.info.Size(),
	})

	queue <- Task{
		Action:    TransferFile,
		Path:      obj.path,
		IrodsPath: obj.irodsPath,
		Size:      obj.info.Size(),
		ModTime:   obj.info.ModTime(),
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
				worker.action(u, func() error { return worker.TransferPool.DeleteDataObject(ctx, u.IrodsPath, worker.options.SkipTrash) })

			case RemoveDirectory:
				worker.action(u, func() error { return worker.TransferPool.DeleteCollection(ctx, u.IrodsPath, worker.options.SkipTrash) })
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

// ComputeChecksums computes the checksums of all files in a directory on the iRODS server.
// It handles the recursion client side, but individual files are processed server-side only.
// The call blocks until the source directory has been completely scanned.
func (worker *Worker) ComputeChecksums(ctx context.Context, remote string) {
	queue := make(chan Task, worker.options.MaxQueued)

	// Execute the tasks
	worker.wg.Go(func() error {
		for u := range queue {
			if ctx.Err() != nil {
				continue
			}

			switch u.Action { //nolint:exhaustive
			case ComputeChecksum:
				worker.action(u, func() error {
					_, err := worker.TransferPool.Checksum(ctx, u.IrodsPath, true)

					return err
				})

			case TransferFile: // Actually we do VerifyChecksums here, but we want to report the progress as a transfer
				worker.action(u, func() error {
					return worker.TransferPool.VerifyChecksum(ctx, u.IrodsPath)
				})
			}
		}

		return ctx.Err()
	})

	// Walk the directory
	defer close(queue)

	err := worker.IndexPool.Walk(ctx, remote, func(irodsPath string, record api.Record, err error) error {
		if err != nil {
			return worker.options.ErrorHandler("", irodsPath, err)
		}

		worker.computeChecksum(irodsPath, record, queue)

		return nil
	}, api.NoSkip)
	if err != nil {
		worker.wg.Go(func() error {
			return err
		})
	}
}

func (worker *Worker) computeChecksum(irodsPath string, record api.Record, queue chan<- Task) {
	if record.IsDir() {
		return
	}

	if _, hasChecksum := parseChecksum(record); hasChecksum && worker.options.CompareChecksums {
		queue <- Task{
			Action:    TransferFile, // TransferFile = verify checksum
			IrodsPath: irodsPath,
		}

		return
	} else if hasChecksum || !worker.options.IntegrityChecksums {
		return
	}

	queue <- Task{
		Action:    ComputeChecksum,
		IrodsPath: irodsPath,
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
	worker.wg.Go(func() error { //nolint:dupl
		for u := range queue {
			if ctx.Err() != nil {
				continue
			}

			switch u.Action {
			case ComputeChecksum: // This is a special case for the progress handler, it doesn't correspond to an actual task

			case SetModificationTime:
				worker.action(u, func() error { return worker.TransferPool.ModifyModificationTime(ctx, u.IrodsPath, u.ModTime) })

			case TransferFile:
				worker.copyAction(ctx, u)

			case RemoveFile:
				worker.action(u, func() error { return worker.TransferPool.DeleteDataObject(ctx, u.IrodsPath, worker.options.SkipTrash) })

			case RemoveDirectory:
				worker.action(u, func() error { return worker.TransferPool.DeleteCollection(ctx, u.IrodsPath, worker.options.SkipTrash) })

			case CreateDirectory:
				worker.action(u, func() error { return worker.TransferPool.CreateCollection(ctx, u.IrodsPath) })
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

func (worker *Worker) copyAction(ctx context.Context, u Task) {
	if worker.options.DryRun {
		worker.log(u)

		return
	}

	remote1 := u.Path
	remote2 := u.IrodsPath

	conn, err := worker.TransferPool.Connect(ctx)
	if err != nil {
		worker.Error(remote1, remote2, err)

		return
	}

	startTime := time.Now()

	worker.Progress(Progress{
		Action:    TransferFile,
		Label:     ProgressLabel(remote1, remote2),
		Size:      u.Size,
		StartedAt: startTime,
	})

	worker.wg.Go(func() error {
		connAPI := *worker.TransferPool
		connAPI.Connect = func(ctx context.Context) (api.Conn, error) { return conn, nil }

		if err := connAPI.CopyDataObject(ctx, remote1, remote2); err != nil {
			return worker.options.ErrorHandler(remote1, remote2, err)
		}

		worker.Progress(Progress{
			Action:      TransferFile,
			Label:       ProgressLabel(remote1, remote2),
			Size:        u.Size,
			Increment:   u.Size,
			Transferred: u.Size,
			StartedAt:   startTime,
			FinishedAt:  time.Now(),
		})

		// Verify the checksum after copying if integrity checksums are enabled
		if worker.options.IntegrityChecksums {
			_, _, err := VerifyRemoteToRemote(worker.TransferPool, worker.options.ProgressHandler)(ctx, remote1, remote2, nil, nil)
			if err != nil {
				return worker.options.ErrorHandler(remote1, remote2, err)
			}
		}

		return nil
	})
}

// log logs a task without performing it, for dry-run mode.
func (worker *Worker) log(u Task) {
	fmt.Printf("\rwould %s\n", u.Action.Format(ProgressLabel(u.Path, u.IrodsPath)))
}

// action runs a simple action and schedules an error
func (worker *Worker) action(u Task, callback func() error) {
	if worker.options.DryRun {
		worker.log(u)

		return
	}

	startTime := time.Now()

	worker.Progress(Progress{
		Action:    u.Action,
		Label:     ProgressLabel(u.Path, u.IrodsPath),
		StartedAt: startTime,
	})

	if err := callback(); err != nil {
		worker.Error(u.Path, u.IrodsPath, err)

		return
	}

	worker.Progress(Progress{
		Action:     u.Action,
		Label:      ProgressLabel(u.Path, u.IrodsPath),
		StartedAt:  startTime,
		FinishedAt: time.Now(),
	})
}

// Error schedules an error
func (worker *Worker) Error(local, remote string, err error) {
	worker.wg.Go(func() error {
		return worker.options.ErrorHandler(local, remote, err)
	})
}

// Progress triggers the progress handler with the given progress.
func (worker *Worker) Progress(progress Progress) {
	if worker.options.ProgressHandler != nil {
		worker.options.ProgressHandler(progress)
	}
}
